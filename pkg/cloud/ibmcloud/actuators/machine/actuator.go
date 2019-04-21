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

	"github.com/softlayer/softlayer-go/datatypes"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud"
	ibmcloudclients "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/clients"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	ProviderName = "ibmcloud"
)

// Actuator is responsible for performing machine reconciliation
type IbmCloudClient struct {
	params ibmcloud.ActuatorParams
}

// NewActuator creates a new Actuator
func NewActuator(params ibmcloud.ActuatorParams) (*IbmCloudClient, error) {
	return &IbmCloudClient{
		params: params,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Creating machine %v for cluster %v.", machine.Name, cluster.Name)
	return fmt.Errorf("TODO: Not yet implemented")
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
	log.Printf("Getting IP of machine %v for cluster %v.", master.Name, cluster.Name)
	return "", fmt.Errorf("TODO: Not yet implemented")
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
