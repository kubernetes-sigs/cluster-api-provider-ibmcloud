/*
Copyright 2021 The Kubernetes Authors.

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

package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	IBMVPCClients
	Client        client.Client
	Logger        logr.Logger
	Cluster       *clusterv1.Cluster
	Machine       *clusterv1.Machine
	IBMVPCCluster *infrav1.IBMVPCCluster
	IBMVPCMachine *infrav1.IBMVPCMachine
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	IBMVPCClients
	Cluster *clusterv1.Cluster
	Machine *clusterv1.Machine

	IBMVPCCluster *infrav1.IBMVPCCluster
	IBMVPCMachine *infrav1.IBMVPCMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
func NewMachineScope(params MachineScopeParams, authenticator core.Authenticator, svcEndpoint string) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.IBMVPCMachine == nil {
		return nil, errors.New("failed to generate new scope from nil IBMVPCCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.IBMVPCMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	vpcErr := params.IBMVPCClients.setIBMVPCService(authenticator, svcEndpoint)
	if vpcErr != nil {
		return nil, errors.Wrap(vpcErr, "failed to create IBM VPC session")
	}

	return &MachineScope{
		Logger:        params.Logger,
		client:        params.Client,
		IBMVPCClients: params.IBMVPCClients,
		Cluster:       params.Cluster,
		IBMVPCCluster: params.IBMVPCCluster,
		patchHelper:   helper,
		Machine:       params.Machine,
		IBMVPCMachine: params.IBMVPCMachine,
	}, nil
}

// CreateMachine creates a vpc machine
func (m *MachineScope) CreateMachine() (*vpcv1.Instance, error) {
	instanceReply, err := m.ensureInstanceUnique(m.IBMVPCMachine.Name)
	if err != nil {
		return nil, err
	} else if instanceReply != nil {
		//TODO need a reasonable wrapped error
		return instanceReply, nil
	}

	cloudInitData, err := m.GetBootstrapData()
	if err != nil {
		return nil, err
	}

	options := &vpcv1.CreateInstanceOptions{}
	instancePrototype := &vpcv1.InstancePrototype{
		Name: &m.IBMVPCMachine.Name,
		Image: &vpcv1.ImageIdentity{
			ID: &m.IBMVPCMachine.Spec.Image,
		},
		Profile: &vpcv1.InstanceProfileIdentity{
			Name: &m.IBMVPCMachine.Spec.Profile,
		},
		Zone: &vpcv1.ZoneIdentity{
			Name: &m.IBMVPCMachine.Spec.Zone,
		},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfacePrototype{
			Subnet: &vpcv1.SubnetIdentity{
				ID: &m.IBMVPCMachine.Spec.PrimaryNetworkInterface.Subnet,
			},
		},
		UserData: &cloudInitData,
	}

	if m.IBMVPCMachine.Spec.SSHKeys != nil {
		instancePrototype.Keys = []vpcv1.KeyIdentityIntf{}
		for _, sshKey := range m.IBMVPCMachine.Spec.SSHKeys {
			key := &vpcv1.KeyIdentity{
				ID: sshKey,
			}
			instancePrototype.Keys = append(instancePrototype.Keys, key)
		}

	}

	options.SetInstancePrototype(instancePrototype)
	instance, response, err := m.IBMVPCClients.VPCService.CreateInstance(options)
	fmt.Printf("%v\n", response)
	return instance, err
}

// DeleteMachine deletes the vpc machine associated with machine instance id.
func (m *MachineScope) DeleteMachine() error {
	options := &vpcv1.DeleteInstanceOptions{}
	options.SetID(m.IBMVPCMachine.Status.InstanceID)
	_, err := m.IBMVPCClients.VPCService.DeleteInstance(options)
	return err
}

func (m *MachineScope) ensureInstanceUnique(instanceName string) (*vpcv1.Instance, error) {
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := m.IBMVPCClients.VPCService.ListInstances(options)

	if err != nil {
		return nil, err
	}
	for _, instance := range instances.Instances {
		if *instance.Name == instanceName {
			return &instance, nil
		}
	}
	return nil, nil
}

// GetMachine returns a machine associated with a machine instanceID
func (m *MachineScope) GetMachine(instanceID string) (*vpcv1.Instance, error) {
	options := &vpcv1.GetInstanceOptions{}
	options.SetID(instanceID)

	instance, _, err := m.IBMVPCClients.VPCService.GetInstance(options)
	return instance, err
}

// PatchObject persists the cluster configuration and status.
func (m *MachineScope) PatchObject() error {
	return m.patchHelper.Patch(context.TODO(), m.IBMVPCMachine)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close() error {
	return m.PatchObject()
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData() (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Machine.Namespace, Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for IBMVPCMachine %s/%s", m.Machine.Namespace, m.Machine.Name)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return string(value), nil
}
