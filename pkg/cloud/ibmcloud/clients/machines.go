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
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
	"github.com/softlayer/softlayer-go/sl"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	ibmcloudv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	CloudsYamlFile = "/etc/ibmcloud/clouds.yaml"

	// TODO: make the timeout to be configurable
	WaitReadyRetryInterval = 5 * time.Second
	WaitReadyTimeout       = 600 * time.Second
)

type GuestService struct {
	sess *session.Session
}

func NewGuestService(sess *session.Session) *GuestService {
	return &GuestService{sess: sess}
}

type IBMCloudConfig struct {
	UserName string `yaml:"userName,omitempty"`
	APIKey   string `yaml:"apiKey,omitempty"`
}

func NewInstanceServiceFromMachine(kubeClient kubernetes.Interface, machine *clusterv1.Machine) (*GuestService, error) {
	// clouds.yaml is mounted into controller pod for clouds authentication
	fileName := CloudsYamlFile
	if _, err := os.Stat(fileName); err != nil {
		return nil, fmt.Errorf("Cannot stat %q: %v", fileName, err)
	}
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Cannot read %q: %v", fileName, err)
	}

	config := IBMCloudConfig{}
	yaml.Unmarshal(bytes, &config)

	if config.UserName == "" || config.APIKey == "" {
		return nil, fmt.Errorf("Failed getting IBM Cloud config userName %q, apiKey %q", config.UserName, config.APIKey)
	}

	sess := session.New(config.UserName, config.APIKey)
	return NewGuestService(sess), nil
}

func (gs *GuestService) waitGuestReady(Id int) error {
	// Wait for transactions to finish
	klog.Info("Waiting for transactions to complete before destroying.")
	s := services.GetVirtualGuestService(gs.sess).Id(Id)

	// Delay to allow transactions to be registered
	time.Sleep(WaitReadyRetryInterval)

	// TODO: make it V(2) after initial development work done if necessary
	//if klog.V(2) {
	// Enable debug to show messages from IBM Cloud during node provision
	s.Session.Debug = true
	//}
	sum := WaitReadyRetryInterval
	for transactions, _ := s.GetActiveTransactions(); len(transactions) > 0; {
		time.Sleep(WaitReadyRetryInterval)
		sum += WaitReadyRetryInterval
		if sum > WaitReadyTimeout {
			// Now the guest failed to reach timeout
			return fmt.Errorf("Waiting for guest %d ready time out", Id)
		}
		transactions, _ = s.GetActiveTransactions()
	}
	s.Session.Debug = false

	klog.Info("Waiting for transactions done.")
	return nil
}

func (gs *GuestService) CreateGuest(clusterName, hostName string, machineSpec *ibmcloudv1.IbmcloudMachineProviderSpec, userScript string) {
	s := services.GetVirtualGuestService(gs.sess)

	keyId := getSshKey(gs.sess, machineSpec.SshKeyName)
	if keyId == 0 {
		klog.Infof("Cannot retrieving specific SSH key %q. Stop creating VM instance.", machineSpec.SshKeyName)
		return
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

	// Create a Virtual_Guest instance from a template
	vGuestTemplate := datatypes.Virtual_Guest{
		Hostname:                     sl.String(hostName),
		Domain:                       sl.String(machineSpec.Domain),
		MaxMemory:                    sl.Int(machineSpec.MaxMemory),
		StartCpus:                    sl.Int(machineSpec.StartCpus),
		Datacenter:                   &datatypes.Location{Name: sl.String(machineSpec.Datacenter)},
		OperatingSystemReferenceCode: sl.String(machineSpec.OSReferenceCode),
		LocalDiskFlag:                sl.Bool(machineSpec.LocalDiskFlag),
		HourlyBillingFlag:            sl.Bool(machineSpec.HourlyBillingFlag),
		SshKeyCount:                  sl.Uint(1),
		SshKeys:                      sshKeys,
		UserData:                     userData,
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
	} else if success == false {
		err = fmt.Errorf("Failed deleting the virtual guest with ID %d", Id)
	} else {
		klog.Infof("Virtual Guest deleted successfully")
	}
	return err
}

func (gs *GuestService) listGuest() ([]datatypes.Virtual_Guest, error) {
	s := services.GetAccountService(gs.sess)

	guests, err := s.GetVirtualGuests()
	if err != nil {
		klog.Errorf("Error listing virtual guest: %v", err)
		return nil, err
	}
	return guests, nil
}

// FIXME: use API layer query instead of query all then compare here
func (gs *GuestService) GetGuest(name string) (*datatypes.Virtual_Guest, error) {
	guests, err := gs.listGuest()
	if err != nil {
		return nil, err
	}

	for _, guest := range guests {
		// FIXME: how to unique identify one guest
		if *guest.Hostname == name {
			klog.Infof("Found guest with ID %d for %q", *guest.Id, name)
			return &guest, nil
		}
	}
	return nil, nil
}

// TODO: directly get ID of ssh key instead of list and search
func getSshKey(sess *session.Session, name string) int {
	id := 0

	service := services.GetAccountService(sess)
	keys, err := service.GetSshKeys()
	if err != nil {
		klog.Errorf("Error retrieving ssh keys: %v", err)
		return 0
	}

	for _, key := range keys {
		if *key.Label == name {
			id = *key.Id
			break
		}
	}

	return id

}
