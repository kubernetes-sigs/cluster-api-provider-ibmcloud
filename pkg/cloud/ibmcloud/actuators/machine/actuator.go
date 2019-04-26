/*
Copyright 2018 The Kubernetes authors.

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

package machine

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/softlayer/softlayer-go/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	tokenapi "k8s.io/cluster-bootstrap/token/api"
	tokenutil "k8s.io/cluster-bootstrap/token/util"

	ibmcloudv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	bootstrap "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/bootstrap"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud"
	ibmcloudclients "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/clients"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProviderName = "ibmcloud"
	UserDataKey  = "userData"

	TokenTTL = 60 * time.Minute
)

// Actuator is responsible for performing machine reconciliation
type IbmCloudClient struct {
	params ibmcloud.ActuatorParams
	client client.Client
}

// NewActuator creates a new Actuator
func NewActuator(params ibmcloud.ActuatorParams) (*IbmCloudClient, error) {
	return &IbmCloudClient{
		params: params,
		client: params.Client,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Creating machine %v for cluster %v.", machine.Name, cluster.Name)

	kubeClient := ic.params.KubeClient
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(kubeClient, machine)
	if err != nil {
		return err
	}

	providerSpec, err := ibmcloudv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	guest, err := ic.guestExists(machine)
	if err != nil {
		return err
	}
	if guest != nil {
		log.Printf("Skipped creating a VM that already exists.\n")
		return nil
	}

	var userData []byte
	if providerSpec.UserDataSecret != nil {
		namespace := providerSpec.UserDataSecret.Namespace
		if namespace == "" {
			namespace = machine.Namespace
		}

		if providerSpec.UserDataSecret.Name == "" {
			return fmt.Errorf("UserDataSecret name must be provided")
		}

		userDataSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(providerSpec.UserDataSecret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		var ok bool
		userData, ok = userDataSecret.Data[UserDataKey]
		if !ok {
			return fmt.Errorf("Machine's userdata secret %v in namespace %v did not contain key %v", providerSpec.UserDataSecret.Name, namespace, UserDataKey)
		}
	}

	var userScriptRendered string
	if len(userData) > 0 {
		if util.IsControlPlaneMachine(machine) {
			userScriptRendered, err = masterStartupScript(cluster, machine, string(userData))
		} else {
			token, err := ic.createBootstrapToken()
			if err != nil {
				return fmt.Errorf("Failed to create toke: %s", err)
			}
			userScriptRendered, err = nodeStartupScript(cluster, machine, token, string(userData))
		}
	}
	if err != nil {
		return err
	}

	machineService.GuestCreate(cluster.Name, machine.Name, providerSpec.SshKeyName, userScriptRendered)

	guest, err = ic.guestExists(machine)
	if err != nil {
		return err
	}

	return ic.updateAnnotation(machine, strconv.Itoa(*guest.Id))
}

// Delete deletes a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(ic.params.KubeClient, machine)
	if err != nil {
		return err
	}

	guestGet, err := ic.guestExists(machine)
	if err != nil {
		return err
	}

	if guestGet == nil {
		log.Printf("Skipped deleting %s that is already deleted.\n", machine.Name)
		return nil
	}

	err = machineService.GuestDelete(*guestGet.Id)
	if err != nil {
		return fmt.Errorf("Guest delete failed %s", err)
	}

	return nil
}

// Update updates a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Updating machine %v for cluster %v.", machine.Name, cluster.Name)
	return fmt.Errorf("TODO: Not yet implemented")
}

// Exists test for the existance of a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	guest, err := ic.guestExists(machine)
	if err != nil {
		return false, err
	}
	return guest != nil, err
}

// The Machine Actuator interface must implement GetIP and GetKubeConfig functions as a workaround for issues
// cluster-api#158 (https://github.com/kubernetes-sigs/cluster-api/issues/158) and cluster-api#160
// (https://github.com/kubernetes-sigs/cluster-api/issues/160).

// GetIP returns IP address of the machine in the cluster.
func (ic *IbmCloudClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(ic.params.KubeClient, machine)
	if err != nil {
		return "", err
	}

	guestGet, err := machineService.GuestGet(machine.Name)
	if err != nil {
		return "", err
	}
	return *guestGet.PrimaryIpAddress, nil
}

// GetKubeConfig gets a kubeconfig from the master.
func (ic *IbmCloudClient) GetKubeConfig(cluster *clusterv1.Cluster, master *clusterv1.Machine) (string, error) {
	ip, err := ic.GetIP(cluster, master)
	if err != nil {
		return "", err
	}

	homeDir, ok := os.LookupEnv("HOME")
	if !ok {
		return "", fmt.Errorf("unable to use HOME environment variable to find SSH key: %v", err)
	}

	// FIXME: use ssh user defined in machine spec name later
	sshUserName := "ubuntu"
	// FIXME: use other predefined ssh keyname or make this global definition
	privateKey := "cluster-api-provider-ibmcloud"

	result := strings.TrimSpace(util.ExecCommand(
		"ssh", "-i", homeDir+"/.ssh/"+privateKey,
		"-o", "StrictHostKeyChecking no",
		"-o", "UserKnownHostsFile /dev/null",
		"-o", "BatchMode=yes",
		fmt.Sprintf("%s@%s", sshUserName, ip),
		"echo STARTFILE; sudo cat /etc/kubernetes/admin.conf"))
	parts := strings.Split(result, "STARTFILE")
	if len(parts) != 2 {
		return "", nil
	}
	return strings.TrimSpace(parts[1]), nil
}

func (ic *IbmCloudClient) guestExists(machine *clusterv1.Machine) (guest *datatypes.Virtual_Guest, err error) {
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(ic.params.KubeClient, machine)
	if err != nil {
		return nil, err
	}

	guestGet, err := machineService.GuestGet(machine.Name)
	if err != nil {
		return nil, err
	}
	return guestGet, nil
}

func (ic *IbmCloudClient) updateAnnotation(machine *clusterv1.Machine, id string) error {
	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	machine.ObjectMeta.Annotations[ibmcloud.IBMCloudIdAnnotationKey] = id

	ip, err := ic.GetIP(nil, machine)
	if err != nil {
		return err
	}
	machine.ObjectMeta.Annotations[ibmcloud.IBMCloudIPAnnotationKey] = ip

	if err := ic.params.Client.Update(nil, machine); err != nil {
		return err
	}

	return nil
}

func (ic *IbmCloudClient) createBootstrapToken() (string, error) {
	token, err := tokenutil.GenerateBootstrapToken()
	if err != nil {
		return "", err
	}

	expiration := time.Now().UTC().Add(TokenTTL)
	tokenSecret, err := bootstrap.GenerateTokenSecret(token, expiration)
	if err != nil {
		panic(fmt.Sprintf("unable to create token. there might be a bug somwhere: %v", err))
	}

	err = ic.client.Create(context.TODO(), tokenSecret)
	if err != nil {
		return "", err
	}

	return tokenutil.TokenFromIDAndSecret(
		string(tokenSecret.Data[tokenapi.BootstrapTokenIDKey]),
		string(tokenSecret.Data[tokenapi.BootstrapTokenSecretKey]),
	), nil
}
