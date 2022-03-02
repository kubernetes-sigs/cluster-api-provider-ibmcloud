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
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/pointer"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	utils "github.com/ppc64le-cloud/powervs-utils"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_p_vm_instances"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	servicesutils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

// PowerVSMachineScopeParams defines the input parameters used to create a new PowerVSMachineScope.
type PowerVSMachineScopeParams struct {
	Logger            logr.Logger
	Client            client.Client
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	IBMPowerVSCluster *v1beta1.IBMPowerVSCluster
	IBMPowerVSMachine *v1beta1.IBMPowerVSMachine
	IBMPowerVSImage   *v1beta1.IBMPowerVSImage
}

// PowerVSMachineScope defines a scope defined around a Power VS Machine.
type PowerVSMachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	IBMPowerVSClient  powervs.PowerVS
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	IBMPowerVSCluster *v1beta1.IBMPowerVSCluster
	IBMPowerVSMachine *v1beta1.IBMPowerVSMachine
	IBMPowerVSImage   *v1beta1.IBMPowerVSImage
}

// NewPowerVSMachineScope creates a new PowerVSMachineScope from the supplied parameters.
func NewPowerVSMachineScope(params PowerVSMachineScopeParams) (scope *PowerVSMachineScope, err error) {
	scope = &PowerVSMachineScope{}

	if params.Client == nil {
		err = errors.New("client is required when creating a MachineScope")
		return
	}
	scope.client = params.Client

	if params.Machine == nil {
		err = errors.New("machine is required when creating a MachineScope")
		return
	}
	scope.Machine = params.Machine

	if params.Cluster == nil {
		err = errors.New("cluster is required when creating a MachineScope")
		return
	}
	scope.Cluster = params.Cluster

	if params.IBMPowerVSMachine == nil {
		err = errors.New("PowerVS machine is required when creating a MachineScope")
		return
	}
	scope.IBMPowerVSMachine = params.IBMPowerVSMachine
	scope.IBMPowerVSCluster = params.IBMPowerVSCluster
	scope.IBMPowerVSImage = params.IBMPowerVSImage

	if params.Logger == (logr.Logger{}) {
		params.Logger = klogr.New()
	}
	scope.Logger = params.Logger

	helper, err := patch.NewHelper(params.IBMPowerVSMachine, params.Client)
	if err != nil {
		err = errors.Wrap(err, "failed to init patch helper")
		return
	}
	scope.patchHelper = helper

	m := params.IBMPowerVSMachine

	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		err = errors.Wrap(err, "failed to get authenticator")
		return
	}

	account, err := servicesutils.GetAccount(auth)
	if err != nil {
		err = errors.Wrap(err, "failed to get account")
		return
	}

	rc, err := resourcecontroller.NewService(resourcecontroller.ServiceOptions{})
	if err != nil {
		return
	}

	res, _, err := rc.GetResourceInstance(
		&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: core.StringPtr(m.Spec.ServiceInstanceID),
		})
	if err != nil {
		err = errors.Wrap(err, "failed to get resource instance")
		return
	}

	region, err := utils.GetRegion(*res.RegionID)
	if err != nil {
		err = errors.Wrap(err, "failed to get region")
		return
	}

	options := powervs.ServiceOptions{
		IBMPIOptions: &ibmpisession.IBMPIOptions{
			Debug:       params.Logger.V(DEBUGLEVEL).Enabled(),
			UserAccount: account,
			Region:      region,
			Zone:        *res.RegionID,
		},
		CloudInstanceID: m.Spec.ServiceInstanceID,
	}
	c, err := powervs.NewService(options)
	if err != nil {
		err = fmt.Errorf("failed to create PowerVS service")
		return
	}
	scope.IBMPowerVSClient = c

	return
}

func (m *PowerVSMachineScope) ensureInstanceUnique(instanceName string) (*models.PVMInstanceReference, error) {
	instances, err := m.IBMPowerVSClient.GetAllInstance()
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

// CreateMachine creates a power vs machine
func (m *PowerVSMachineScope) CreateMachine() (*models.PVMInstanceReference, error) {
	s := m.IBMPowerVSMachine.Spec

	instanceReply, err := m.ensureInstanceUnique(m.IBMPowerVSMachine.Name)
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

	memory, err := strconv.ParseFloat(s.Memory, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert memory(%s) to float64", s.Memory)
	}
	cores, err := strconv.ParseFloat(s.Processors, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Processors(%s) to float64", s.Processors)
	}

	var imageID *string
	if m.IBMPowerVSImage != nil {
		imageID = &m.IBMPowerVSImage.Status.ImageID
	} else {
		imageID, err = getImageID(s.Image, m)
		if err != nil {
			return nil, fmt.Errorf("error getting image ID: %v", err)
		}
	}

	networkID, err := getNetworkID(s.Network, m)
	if err != nil {
		return nil, fmt.Errorf("error getting network ID: %v", err)
	}

	params := &p_cloud_p_vm_instances.PcloudPvminstancesPostParams{
		Body: &models.PVMInstanceCreate{
			ImageID:     imageID,
			KeyPairName: s.SSHKey,
			Networks: []*models.PVMInstanceAddNetwork{
				{
					NetworkID: networkID,
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
	_, err = m.IBMPowerVSClient.CreateInstance(params.Body)
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

// DeleteMachine deletes the power vs machine associated with machine instance id and service instance id.
func (m *PowerVSMachineScope) DeleteMachine() error {
	return m.IBMPowerVSClient.DeleteInstance(m.IBMPowerVSMachine.Status.InstanceID)
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

func getImageID(image *v1beta1.IBMPowerVSResourceReference, m *PowerVSMachineScope) (*string, error) {
	if image.ID != nil {
		return image.ID, nil
	} else if image.Name != nil {
		images, err := m.GetImages()
		if err != nil {
			m.Logger.Error(err, "failed to get images")
			return nil, err
		}
		for _, img := range images.Images {
			if *image.Name == *img.Name {
				m.Logger.Info("image found with ID", "Image", *image.Name, "ID", *img.ImageID)
				return img.ImageID, nil
			}
		}
	} else {
		return nil, fmt.Errorf("both ID and Name can't be nil")
	}
	return nil, fmt.Errorf("failed to find an image ID")
}

func (m *PowerVSMachineScope) GetImages() (*models.Images, error) {
	return m.IBMPowerVSClient.GetAllImage()
}

func getNetworkID(network v1beta1.IBMPowerVSResourceReference, m *PowerVSMachineScope) (*string, error) {
	if network.ID != nil {
		return network.ID, nil
	} else if network.Name != nil {
		networks, err := m.GetNetworks()
		if err != nil {
			m.Logger.Error(err, "failed to get networks")
			return nil, err
		}
		for _, nw := range networks.Networks {
			if *network.Name == *nw.Name {
				m.Logger.Info("network found with ID", "Network", *network.Name, "ID", *nw.NetworkID)
				return nw.NetworkID, nil
			}
		}
	} else {
		return nil, fmt.Errorf("both ID and Name can't be nil")
	}

	return nil, fmt.Errorf("failed to find a network ID")
}

func (m *PowerVSMachineScope) GetNetworks() (*models.Networks, error) {
	return m.IBMPowerVSClient.GetAllNetwork()
}

func (m *PowerVSMachineScope) SetReady() {
	m.IBMPowerVSMachine.Status.Ready = true
}

func (m *PowerVSMachineScope) SetNotReady() {
	m.IBMPowerVSMachine.Status.Ready = false
}

func (m *PowerVSMachineScope) IsReady() bool {
	return m.IBMPowerVSMachine.Status.Ready
}

func (m *PowerVSMachineScope) SetInstanceID(id *string) {
	if id != nil {
		m.IBMPowerVSMachine.Status.InstanceID = *id
	}
}

func (m *PowerVSMachineScope) GetInstanceID() string {
	return m.IBMPowerVSMachine.Status.InstanceID
}

func (m *PowerVSMachineScope) SetHealth(health *models.PVMInstanceHealth) {
	if health != nil {
		m.IBMPowerVSMachine.Status.Health = health.Status
	}
}

func (m *PowerVSMachineScope) SetAddresses(instance *models.PVMInstance) {
	var addresses []corev1.NodeAddress
	// Setting the name of the vm to the InternalDNS and Hostname as the vm uses that as hostname
	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalDNS,
		Address: *instance.ServerName,
	})
	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeHostName,
		Address: *instance.ServerName,
	})
	for _, network := range instance.Networks {
		if strings.TrimSpace(network.IPAddress) != "" {
			addresses = append(addresses, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: strings.TrimSpace(network.IPAddress),
			})
		}
		if strings.TrimSpace(network.ExternalIP) != "" {
			addresses = append(addresses, corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: strings.TrimSpace(network.ExternalIP),
			})
		}
	}
	m.IBMPowerVSMachine.Status.Addresses = addresses
}

func (m *PowerVSMachineScope) SetInstanceState(status *string) {
	m.IBMPowerVSMachine.Status.InstanceState = v1beta1.PowerVSInstanceState(*status)
}

func (m *PowerVSMachineScope) GetInstanceState() v1beta1.PowerVSInstanceState {
	return m.IBMPowerVSMachine.Status.InstanceState
}

func (m *PowerVSMachineScope) SetProviderID() {
	m.IBMPowerVSMachine.Spec.ProviderID = pointer.StringPtr(fmt.Sprintf("ibmpowervs://%s/%s", m.Machine.Spec.ClusterName, m.IBMPowerVSMachine.Name))
}
