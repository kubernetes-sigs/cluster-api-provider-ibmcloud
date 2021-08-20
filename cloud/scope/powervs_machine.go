/*


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
	"encoding/base64"
	"fmt"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_p_vm_instances"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pkg"
	"github.com/pkg/errors"
	"github.com/ppc64le-cloud/powervs-utils"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PowerVSMachineScopeParams struct {
	Logger            logr.Logger
	Client            client.Client
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	IBMPowerVSCluster *v1alpha4.IBMPowerVSCluster
	IBMPowerVSMachine *v1alpha4.IBMPowerVSMachine
}

type PowerVSMachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	IBMPowerVSClient  *IBMPowerVSClient
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	IBMPowerVSCluster *v1alpha4.IBMPowerVSCluster
	IBMPowerVSMachine *v1alpha4.IBMPowerVSMachine
}

func NewPowerVSMachineScope(params PowerVSMachineScopeParams) (*PowerVSMachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachineScope")
	}
	if params.IBMPowerVSMachine == nil {
		return nil, errors.New("aws machine is required when creating a MachineScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	m := params.IBMPowerVSMachine
	client := pkg.NewClient()

	resource, err := client.ResourceClient.GetInstance(m.Spec.ServiceInstanceID)
	if err != nil {
		return nil, err
	}
	region, err := utils.GetRegion(resource.RegionID)
	if err != nil {
		return nil, err
	}
	zone := resource.RegionID

	c, err := NewIBMPowerVSClient(client.Config.IAMAccessToken, client.User.Account, m.Spec.ServiceInstanceID, region, zone, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create NewIBMPowerVSClient")
	}

	helper, err := patch.NewHelper(params.IBMPowerVSMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &PowerVSMachineScope{
		Logger:      params.Logger,
		client:      params.Client,
		patchHelper: helper,

		IBMPowerVSClient:  c,
		Cluster:           params.Cluster,
		Machine:           params.Machine,
		IBMPowerVSMachine: params.IBMPowerVSMachine,
		IBMPowerVSCluster: params.IBMPowerVSCluster,
	}, nil
}

func (m *PowerVSMachineScope) ensureInstanceUnique(instanceName string) (*models.PVMInstanceReference, error) {
	instances, err := m.IBMPowerVSClient.InstanceClient.GetAll(m.IBMPowerVSMachine.Spec.ServiceInstanceID, 60*time.Minute)
	if err != nil {
		return nil, err
	}
	for _, ins := range instances.PvmInstances {
		if *ins.ServerName == instanceName {
			return ins, nil
		}
	}
	return nil, nil
}

func (m *PowerVSMachineScope) CreateMachine() (*models.PVMInstanceReference, error) {
	s := m.IBMPowerVSMachine.Spec

	instanceReply, err := m.ensureInstanceUnique(m.IBMPowerVSMachine.Name)
	if err != nil {
		return nil, err
	} else {
		if instanceReply != nil {
			//TODO need a resonable wraped error
			return instanceReply, nil
		}
	}
	cloudInitData, err := m.GetBootstrapData()
	if err != nil {
		return nil, err
	}

	memory, err := strconv.ParseFloat(s.Memory, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert memory(%s) to float64", s.Memory)
	}
	cores, err := strconv.ParseFloat(s.Processors, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Processors(%s) to float64", s.Processors)
	}

	params := &p_cloud_p_vm_instances.PcloudPvminstancesPostParams{
		Body: &models.PVMInstanceCreate{
			ImageID:     &s.ImageID,
			KeyPairName: s.SSHKey,
			Networks: []*models.PVMInstanceAddNetwork{
				{
					NetworkID: &s.NetworkID,
					//IPAddress: address,
				},
			},
			ServerName: &m.IBMPowerVSMachine.Name,
			Memory:     &memory,
			Processors: &cores,
			ProcType:   &s.ProcType,
			SysType:    s.SysType,
			UserData:   cloudInitData,
		},
	}
	_, err = m.IBMPowerVSClient.InstanceClient.Create(params, s.ServiceInstanceID, time.Hour)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *PowerVSMachineScope) Close() error {
	return m.PatchObject()
}

// PatchObject persists the cluster configuration and status.
func (m *PowerVSMachineScope) PatchObject() error {
	return m.patchHelper.Patch(context.TODO(), m.IBMPowerVSMachine)
}

func (m *PowerVSMachineScope) DeleteMachine() error {
	return m.IBMPowerVSClient.InstanceClient.Delete(m.IBMPowerVSMachine.Status.InstanceID, m.IBMPowerVSMachine.Spec.ServiceInstanceID, time.Hour)
}

// GetBootstrapData returns the base64 encoded bootstrap data from the secret in the Machine's bootstrap.dataSecretName
func (m *PowerVSMachineScope) GetBootstrapData() (string, error) {
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

	return base64.StdEncoding.EncodeToString(value), nil
}
