/*
Copyright 2019 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clients

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/filter"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
	"github.com/softlayer/softlayer-go/sl"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	ibmcloudv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/options"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	CloudsYamlFile = "/etc/ibmcloud/clouds.yaml"

	WaitReadyRetryInterval = 5 * time.Second
)

type GuestService struct {
	sess *session.Session
}

func NewGuestService(sess *session.Session) *GuestService {
	return &GuestService{sess: sess}
}

func NewInstanceServiceFromMachine(kubeClient kubernetes.Interface, machine *clusterv1.Machine) (*GuestService, error) {
	// AuthConfig is mounted into controller pod for clouds authentication
	fileName := CloudsYamlFile
	if _, err := os.Stat(fileName); err != nil {
		return nil, fmt.Errorf("Cannot stat %q: %v", fileName, err)
	}
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Cannot read %q: %v", fileName, err)
	}

	config := cloud.Config{}
	yaml.Unmarshal(bytes, &config)
	authConfig := config.Clouds.IBMCloud.Auth

	if authConfig.APIUserName == "" || authConfig.AuthenticationKey == "" {
		return nil, fmt.Errorf("Failed getting IBM Cloud config API Username %q, Authentication Key %q", authConfig.APIUserName, authConfig.AuthenticationKey)
	}

	sess := session.New(authConfig.APIUserName, authConfig.AuthenticationKey)
	return NewGuestService(sess), nil
}

func (gs *GuestService) waitGuestReady(Id int) error {
	// Wait for transactions to finish
	klog.Info("Waiting for transactions to complete.")
	s := services.GetVirtualGuestService(gs.sess).Id(Id)

	// Delay to allow transactions to be registered
	time.Sleep(WaitReadyRetryInterval)
	err := waitTransactionDone(&s)
	if err != nil {
		return err
	}

	klog.Info("Waiting for transactions done.")
	return nil
}

func (gs *GuestService) CreateGuest(clusterName, hostName string, machineSpec *ibmcloudv1.IBMCloudMachineProviderSpec, userScript string) {
	s := services.GetVirtualGuestService(gs.sess)

	keyId := getSshKey(gs.sess, machineSpec.SshKeyName)
	if keyId == 0 {
		klog.Infof("Cannot retrieving specific SSH key %q. Continue creating VM instance.", machineSpec.SshKeyName)
	}

	sshKeys := []datatypes.Security_Ssh_Key{
		{
			Id: sl.Int(keyId),
		},
	}

	userData := []datatypes.Virtual_Guest_Attribute{
		{
			Value: sl.String(userScript),
			Guest: nil,
			Type: &datatypes.Virtual_Guest_Attribute_Type{
				Keyname: sl.String("USER_DATA"),
				Name:    sl.String("user data"),
			},
		},
	}

	var options datatypes.Virtual_Guest_SupplementalCreateObjectOptions
	options.FlavorKeyName = sl.String(machineSpec.Flavor)

	// Create a Virtual_Guest instance from a template
	// TODO: create instance from spcified subnetwork to avoid CIDR confliction with the pod CIDR and service CIDR
	// of the provisioned cluster, see:https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues/153
	var vGuestTemplate datatypes.Virtual_Guest
	if keyId == 0 {
		vGuestTemplate = datatypes.Virtual_Guest{
			Hostname:                        sl.String(hostName),
			Domain:                          sl.String(machineSpec.Domain),
			SupplementalCreateObjectOptions: &options,
			Datacenter:                      &datatypes.Location{Name: sl.String(machineSpec.Datacenter)},
			OperatingSystemReferenceCode:    sl.String(machineSpec.OSReferenceCode),
			HourlyBillingFlag:               sl.Bool(machineSpec.HourlyBillingFlag),
			SshKeyCount:                     sl.Uint(1),
			SshKeys:                         sshKeys,
			UserData:                        userData,
		}
	} else {
		vGuestTemplate = datatypes.Virtual_Guest{
			Hostname:                        sl.String(hostName),
			Domain:                          sl.String(machineSpec.Domain),
			SupplementalCreateObjectOptions: &options,
			Datacenter:                      &datatypes.Location{Name: sl.String(machineSpec.Datacenter)},
			OperatingSystemReferenceCode:    sl.String(machineSpec.OSReferenceCode),
			HourlyBillingFlag:               sl.Bool(machineSpec.HourlyBillingFlag),
			UserData:                        userData,
		}
	}

	vGuest, err := s.Mask("id;domain").CreateObject(&vGuestTemplate)
	if err != nil {
		klog.Errorf("Failed creating virtual guest: %v", err)
		return
	}
	klog.Infof("New Virtual Guest created with ID %d in domain %q", *vGuest.Id, *vGuest.Domain)

	// Wait for transactions to finish
	err = gs.waitGuestReady(*vGuest.Id)
	if err != nil {
		klog.Errorf("Failed to wait guest ready: %v", err)
		return
	}
}

func (gs *GuestService) DeleteGuest(Id int) error {
	s := services.GetVirtualGuestService(gs.sess).Id(Id)

	success, err := s.DeleteObject()
	if err != nil {
		klog.Errorf("Failed deleting the virtual guest with ID %d: %v", Id, err)
		return err
	} else if success == false {
		return fmt.Errorf("Failed deleting the virtual guest with ID %d", Id)
	}

	err = waitTransactionDone(&s)
	if err == nil {
		klog.Infof("Virtual Guest deleted successfully")
	}
	return err
}

func (gs *GuestService) GetGuest(name, domain string) (*datatypes.Virtual_Guest, error) {
	s := services.GetAccountService(gs.sess)

	hostFilter := filter.Build(
		filter.Path("virtualGuests.hostname").Eq(name),
		filter.Path("virtualGuests.domain").Eq(domain),
	)

	guests, err := s.Filter(hostFilter).GetVirtualGuests()
	if err != nil {
		klog.Errorf("Error getting virtual guests by filter (hostname=%s, domain=%s): %v", name, domain, err)
		return nil, err
	}

	if len(guests) == 0 {
		return nil, nil
	}

	if len(guests) > 1 {
		// I noticed that IBM Cloud can use same name for 2 machines.
		// It is bad for our case. Print a message to make it to be noticed.
		klog.Errorf("Getting more than one virtual guests by filter (hostname=%s, domain=%s). The first one with id %q is used.",
			name, domain, *guests[0].Id)
	}

	return &guests[0], nil
}

func getSshKey(sess *session.Session, name string) int {
	service := services.GetAccountService(sess)

	sshKeyFilter := filter.Build(
		filter.Path("sshKeys.label").Eq(name),
	)

	keys, err := service.Filter(sshKeyFilter).GetSshKeys()
	if err != nil {
		klog.Errorf("Error retrieving ssh keys by filter (key=%s): %v", name, err)
		return 0
	}

	if len(keys) == 0 || keys[0].Id == nil {
		return 0
	}

	if len(keys) > 1 {
		klog.Errorf("Getting more than one ssh keys by filter (key=%s). The first one with id %q is used.",
			name, *keys[0].Id)
	}

	return *keys[0].Id

}

func waitTransactionDone(s *services.Virtual_Guest) error {
	if klog.V(3) {
		// Enable debug to show messages from IBM Cloud during node provision
		s.Session.Debug = true
	}

	klog.V(4).Infof("Waiting to get active transactions in %ds", options.WaitTransactionsTimeout/time.Second)

	sum := WaitReadyRetryInterval
	for transactions, _ := s.GetActiveTransactions(); len(transactions) > 0; {
		time.Sleep(WaitReadyRetryInterval)
		sum += WaitReadyRetryInterval
		if sum > options.WaitTransactionsTimeout {
			// Now the guest failed to reach timeout
			return fmt.Errorf("Waiting for guest %d ready time out", *s.Options.Id)
		}
		transactions, _ = s.GetActiveTransactions()
	}
	s.Session.Debug = false

	return nil
}
