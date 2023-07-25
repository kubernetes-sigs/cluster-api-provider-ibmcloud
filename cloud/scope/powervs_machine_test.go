/*
Copyright 2022 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"

	. "github.com/onsi/gomega"
)

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1beta2.IBMPowerVSMachine {
	image := &infrav1beta2.IBMPowerVSResourceReference{}
	network := infrav1beta2.IBMPowerVSResourceReference{}

	if !isID {
		image.Name = imageRef
		network.Name = networkRef
	} else {
		image.ID = imageRef
		network.ID = networkRef
	}

	return &infrav1beta2.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiv1beta1.ClusterNameLabel: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Spec: infrav1beta2.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromInt(1),
			Image:      image,
			Network:    network,
		},
	}
}

func setupPowerVSMachineScope(clusterName string, machineName string, imageID *string, networkID *string, isID bool, mockpowervs *mock.MockPowerVS) *PowerVSMachineScope {
	cluster := newCluster(clusterName)
	machine := newMachine(machineName)
	secret := newBootstrapSecret(clusterName, machineName)
	powervsMachine := newPowerVSMachine(clusterName, machineName, imageID, networkID, isID)
	powervsCluster := newPowerVSCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, powervsCluster, powervsMachine,
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSMachineScope{
		Client:            client,
		Logger:            klogr.New(),
		IBMPowerVSClient:  mockpowervs,
		Cluster:           cluster,
		Machine:           machine,
		IBMPowerVSCluster: powervsCluster,
		IBMPowerVSMachine: powervsMachine,
		DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
	}
}

func newPowerVSInstance(name, networkID, mac string) *models.PVMInstance {
	return &models.PVMInstance{
		ServerName: pointer.String(name),
		Networks: []*models.PVMInstanceNetwork{
			{
				NetworkID:  networkID,
				MacAddress: mac,
			},
		},
	}
}

func newDHCPServer(serverID, networkID string) models.DHCPServers {
	return models.DHCPServers{
		&models.DHCPServer{
			ID: pointer.String(serverID),
			Network: &models.DHCPServerNetwork{
				ID: pointer.String(networkID),
			},
		},
	}
}

func newDHCPServerDetails(serverID, leaseIP, instanceMac string) *models.DHCPServerDetail {
	return &models.DHCPServerDetail{
		ID: pointer.String(serverID),
		Leases: []*models.DHCPServerLeases{
			{
				InstanceIP:         pointer.String(leaseIP),
				InstanceMacAddress: pointer.String(instanceMac),
			},
		},
	}
}

func TestNewPowerVSMachineScope(t *testing.T) {
	testCases := []struct {
		name   string
		params PowerVSMachineScopeParams
	}{
		{
			name: "Error when Client in nil",
			params: PowerVSMachineScopeParams{
				Client: nil,
			},
		},
		{
			name: "Error when Machine in nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: nil,
			},
		},
		{
			name: "Error when Cluster is nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: newMachine(machineName),
				Cluster: nil,
			},
		},
		{
			name: "Error when IBMPowerVSMachine is nil",
			params: PowerVSMachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: nil,
			},
		},
		{
			name: "Failed to get authenticator",
			params: PowerVSMachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true),
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPowerVSMachineScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateMachinePVS(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Create Machine", func(t *testing.T) {
		pvmInstances := &models.PVMInstances{
			PvmInstances: []*models.PVMInstanceReference{
				{
					ServerName: core.StringPtr("foo-machine-1"),
				},
			},
		}
		images := &models.Images{
			Images: []*models.ImageReference{
				{
					Name:    core.StringPtr(pvsImage),
					ImageID: core.StringPtr(pvsImage + "-id"),
				},
			},
		}
		networks := &models.Networks{
			Networks: []*models.NetworkReference{
				{
					Name:      core.StringPtr(pvsNetwork),
					NetworkID: core.StringPtr(pvsNetwork + "-id"),
				},
			},
		}
		pvmInstanceList := &models.PVMInstanceList{}
		pvmInstanceCreate := &models.PVMInstanceCreate{}

		t.Run("Should create Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Return exsisting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedOutput := models.PVMInstanceReference{
				ServerName: core.StringPtr("foo-machine-1"),
			}
			scope := setupPowerVSMachineScope(clusterName, *core.StringPtr("foo-machine-1"), core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput.ServerName, out.ServerName)
		})

		t.Run("Return NIL when Machine is not present in the Instance list and Machine state is unknown", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedOutput := (*models.PVMInstanceReference)(nil)
			scope := setupPowerVSMachineScope(clusterName, *core.StringPtr("foo-machine-2"), core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.Conditions = append(scope.IBMPowerVSMachine.Status.Conditions, capiv1beta1.Condition{
				Type:   infrav1beta2.InstanceReadyCondition,
				Status: corev1.ConditionUnknown,
			})
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Eror while getting instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, errors.New("Error when getting list of instances"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = core.StringPtr("foo-secret-temp")
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to retrieve bootstrap data, secret value key is missing", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capiv1beta1.ClusterNameLabel: clusterName,
					},
					Name:      machineName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"val": []byte("user data"),
				}}
			g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Invalid processors value", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Spec.Processors = intstr.FromString("invalid")
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("IBMPowerVSImage is not nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSImage = &infrav1beta2.IBMPowerVSImage{
				Status: infrav1beta2.IBMPowerVSImageStatus{
					ImageID: "foo-image",
				},
			}
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((BeNil()))
		})

		t.Run("Image and Network name is set", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((BeNil()))
		})

		t.Run("Error when both Image id and name are nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when both Network id and name are nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), nil, true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when Image id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage+"-temp"), core.StringPtr(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when Network id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork+"-temp"), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error while creating machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, errors.New("Failed to create machine"))
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})
	})
}

func TestDeleteMachinePVS(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Delete Machine", func(t *testing.T) {
		var id string
		t.Run("Should delete Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + "-id"
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + "-id"
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(errors.New("Failed to delete machine"))
			err := scope.DeleteMachine()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestSetAddresses(t *testing.T) {
	instanceName := "test_vm"
	networkID := "test-net-ID"
	leaseIP := "192.168.0.10"
	instanceMac := "ff:11:33:dd:00:22"
	dhcpServerID := "test-server-id"
	defaultExpectedMachineAddress := []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalDNS,
			Address: instanceName,
		},
		{
			Type:    corev1.NodeHostName,
			Address: instanceName,
		},
	}

	defaultDhcpCacheStoreFunc := func() cache.Store {
		return cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)
	}

	testCases := []struct {
		testcase            string
		powerVSClientFunc   func(*gomock.Controller) *mock.MockPowerVS
		pvmInstance         *models.PVMInstance
		expectedNodeAddress []corev1.NodeAddress
		expectedError       error
		dhcpCacheStoreFunc  func() cache.Store
		setNetworkID        bool
	}{
		{
			testcase: "should set external IP address from instance network",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				Networks: []*models.PVMInstanceNetwork{
					{
						ExternalIP: "10.11.2.3",
					},
				},
				ServerName: pointer.String(instanceName),
			},
			expectedNodeAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: "10.11.2.3",
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "should set internal IP address from instance network",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "192.168.10.3",
					},
				},
				ServerName: pointer.String(instanceName),
			},
			expectedNodeAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: "192.168.10.3",
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "should set both internal and external IP address from instance network",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "192.168.10.3",
						ExternalIP: "10.11.2.3",
					},
				},
				ServerName: pointer.String(instanceName),
			},
			expectedNodeAddress: append(defaultExpectedMachineAddress, []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.10.3",
				},
				{
					Type:    corev1.NodeExternalIP,
					Address: "10.11.2.3",
				},
			}...),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "error while getting network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllNetwork().Return(nil, fmt.Errorf("intentional error"))
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.String(instanceName),
			},
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "no network id associated with network name",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				networks := &models.Networks{
					Networks: []*models.NetworkReference{
						{
							NetworkID: pointer.String("test-ID"),
							Name:      pointer.String("test-name"),
						},
					},
				}
				mockPowerVSClient.EXPECT().GetAllNetwork().Return(networks, nil)
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "provided network id not attached to vm",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, "test-net", instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
			setNetworkID:        true,
		},
		{
			testcase: "error while getting DHCP servers",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(nil, fmt.Errorf("intentional error"))
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
			setNetworkID:        true,
		},
		{
			testcase: "dhcp server details not found associated to network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(newDHCPServer(dhcpServerID, "test-network"), nil)
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
			setNetworkID:        true,
		},
		{
			testcase: "error on getting DHCP server details",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(dhcpServerID).Return(nil, fmt.Errorf("intentnional error"))
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
			setNetworkID:        true,
		},
		{
			testcase: "dhcp server lease does not have lease for instance",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, "ff:11:33:dd:00:33"), nil)
				return mockPowerVSClient
			},
			pvmInstance:         newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:  defaultDhcpCacheStoreFunc,
			setNetworkID:        true,
		},
		{
			testcase: "success in getting ip address from dhcp server",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, instanceMac), nil)
				return mockPowerVSClient
			},
			pvmInstance: newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
			setNetworkID:       true,
		},
		{
			testcase: "ip stored in cache expired, fetch from dhcp server",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetAllDHCPServers().Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, instanceMac), nil)
				return mockPowerVSClient
			},
			pvmInstance: newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
			dhcpCacheStoreFunc: func() cache.Store {
				cacheStore := cache.NewTTLStore(powervs.CacheKeyFunc, time.Millisecond)
				_ = cacheStore.Add(powervs.VMip{
					Name: instanceName,
					IP:   "192.168.99.98",
				})
				time.Sleep(time.Millisecond)
				return cacheStore
			},
			setNetworkID: true,
		},
		{
			testcase: "success in fetching DHCP IP from cache",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
			dhcpCacheStoreFunc: func() cache.Store {
				cacheStore := cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)
				_ = cacheStore.Add(powervs.VMip{
					Name: instanceName,
					IP:   leaseIP,
				})
				return cacheStore
			},
			setNetworkID: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			g := NewWithT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPowerVSClient := tc.powerVSClientFunc(ctrl)
			scope := setupPowerVSMachineScope("test-cluster", "test-machine-0", pointer.String("test-image-ID"), &networkID, tc.setNetworkID, mockPowerVSClient)
			scope.DHCPIPCacheStore = tc.dhcpCacheStoreFunc()
			scope.SetAddresses(tc.pvmInstance)
			g.Expect(scope.IBMPowerVSMachine.Status.Addresses).To(Equal(tc.expectedNodeAddress))
		})
	}
}
