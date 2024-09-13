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
	"path"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	resourcecontrollermock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/options"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"
)

const (
	region = "us-south"
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
		Logger:            klog.Background(),
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
		ServerName: ptr.To(name),
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
			ID: ptr.To(serverID),
			Network: &models.DHCPServerNetwork{
				ID: ptr.To(networkID),
			},
		},
	}
}

func newDHCPServerDetails(serverID, leaseIP, instanceMac string) *models.DHCPServerDetail {
	return &models.DHCPServerDetail{
		ID: ptr.To(serverID),
		Leases: []*models.DHCPServerLeases{
			{
				InstanceIP:         ptr.To(leaseIP),
				InstanceMacAddress: ptr.To(instanceMac),
			},
		},
	}
}

func TestAPIServerPort(t *testing.T) {
	testcases := []struct {
		name               string
		expectedPortNumber int32
		machineScope       PowerVSMachineScope
	}{
		{
			name:               "Assigned port number",
			expectedPortNumber: int32(6445),
			machineScope: PowerVSMachineScope{
				Cluster: &capiv1beta1.Cluster{
					Spec: capiv1beta1.ClusterSpec{
						ClusterNetwork: &capiv1beta1.ClusterNetwork{
							APIServerPort: ptr.To(int32(6445)),
						},
					},
				},
			},
		}, {
			name:               "Default api server port",
			expectedPortNumber: infrav1beta2.DefaultAPIServerPort,
			machineScope: PowerVSMachineScope{
				Cluster: &capiv1beta1.Cluster{
					Spec: capiv1beta1.ClusterSpec{
						ClusterNetwork: nil,
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			port := tc.machineScope.APIServerPort()
			g.Expect(port).To(Equal(tc.expectedPortNumber))
		})
	}
}

func TestBucketName(t *testing.T) {
	testcases := []struct {
		name               string
		expectedBucketName string
		machineScope       PowerVSMachineScope
	}{
		{
			name:               "Bucket exists in cos instance",
			expectedBucketName: "foo-bucket",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						CosInstance: &infrav1beta2.CosInstance{
							BucketName: "foo-bucket",
						},
					},
				},
			},
		}, {
			name:               "Cos bucket from PowerVS cluster name",
			expectedBucketName: fmt.Sprintf("%s-%s", "foo-cluster", "cosbucket"),
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo-cluster",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			bucketName := tc.machineScope.bucketName()
			g.Expect(bucketName).To(Equal(tc.expectedBucketName))
		})
	}
}

func TestBucketRegion(t *testing.T) {
	testcases := []struct {
		name               string
		expectedportRegion string
		machineScope       PowerVSMachineScope
	}{
		{
			name:               "test - get region from cos instance",
			expectedportRegion: region,
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						CosInstance: &infrav1beta2.CosInstance{
							BucketRegion: region,
						},
					},
				},
			},
		}, {
			name:               "test - get region from vpc",
			expectedportRegion: region,
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						VPC: &infrav1beta2.VPCResourceReference{
							Region: ptr.To(region),
						},
					},
				},
			},
		}, {
			name:               "test - empty region (both cos instance and vpc source empty)",
			expectedportRegion: "",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			region := tc.machineScope.bucketRegion()
			g.Expect(region).To(Equal(tc.expectedportRegion))
		})
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
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewPowerVSMachineScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestGetServiceInstanceID(t *testing.T) {
	testcases := []struct {
		name                      string
		expectedServiceInstanceID string
		machineScope              PowerVSMachineScope
	}{
		{
			name:                      "get service instance ID from powervsClusterStatus",
			expectedServiceInstanceID: "service-instance-0",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1beta2.ResourceReference{
							ID: ptr.To("service-instance-0"),
						},
					},
				},
			},
		}, {
			name:                      "get service instance ID from powervsClusterSpec",
			expectedServiceInstanceID: "service-instance-1",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
			},
		}, {
			name:                      "get service instance ID from powervsClusterSpec's serviceInstance",
			expectedServiceInstanceID: "service-instance-2",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
							ID: ptr.To("service-instance-2"),
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			serviceInstanceID := tc.machineScope.GetServiceInstanceID()
			g.Expect(serviceInstanceID).To(Equal(tc.expectedServiceInstanceID))
		})
	}
}

func TestSetReady(t *testing.T) {
	t.Run("SetReady", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{},
			},
		}
		machineScope.SetReady()
		g.Expect(machineScope.IsReady()).To(Equal(true))
	})
}

func TestSetNotReady(t *testing.T) {
	t.Run("SetNotReady", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{
					Ready: true,
				},
			},
		}
		machineScope.SetNotReady()
		g.Expect(machineScope.IsReady()).To(Equal(false))
	})
}

func TestGetRegion(t *testing.T) {
	t.Run("Get region", func(t *testing.T) {
		t.Run("Set region", func(t *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
			}
			expectedRegion := region
			machineScope.SetRegion(expectedRegion)
			g.Expect(machineScope.GetRegion()).To(Equal(expectedRegion))
		})
		t.Run("Region not set", func(t *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
			}
			g.Expect(machineScope.GetRegion()).To(Equal(""))
		})
	})
}

func TestGetZone(t *testing.T) {
	t.Run("Get zone", func(t *testing.T) {
		t.Run("Set zone", func(t *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
			}
			expectedZone := "us-south-1"
			machineScope.SetZone(expectedZone)
			g.Expect(machineScope.GetZone()).To(Equal(expectedZone))
		})
		t.Run("Zone not set", func(t *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
			}
			g.Expect(machineScope.GetZone()).To(Equal(""))
		})
	})
}

func TestGetInstanceState(t *testing.T) {
	t.Run("Get instance state", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{},
			},
		}
		expectedState := "ready"
		machineScope.SetInstanceState(ptr.To(expectedState))
		g.Expect(machineScope.GetInstanceState()).To(Equal(infrav1beta2.PowerVSInstanceState(expectedState)))
	})
}

func TestGetIgnitionVersion(t *testing.T) {
	t.Run("Get Ignition version", func(t *testing.T) {
		t.Run("Ignition version is nil", func(t *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			}
			g.Expect(getIgnitionVersion(&machineScope)).To(Equal(infrav1beta2.DefaultIgnitionVersion))
		})
		t.Run("Custom Ignition Version", func(t *testing.T) {
			g := NewWithT(t)
			expectedIgnitionVersion := "3.4"
			machineScope := PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						Ignition: &infrav1beta2.Ignition{
							Version: expectedIgnitionVersion,
						},
					},
				},
			}
			g.Expect(getIgnitionVersion(&machineScope)).To(Equal(expectedIgnitionVersion))
		})
	})
}

func TestBootstrapDataKey(t *testing.T) {
	testcases := []struct {
		name                     string
		machineLabel             string
		machineName              string
		expectedBootstrapDataKey string
	}{
		{
			name:                     "BootstrapDataKey - control plane",
			machineLabel:             capiv1beta1.MachineControlPlaneLabel,
			machineName:              "foo-machine-0",
			expectedBootstrapDataKey: path.Join("control-plane", "foo-machine-0"),
		},
		{
			name:                     "BootstrapDataKey - worker node",
			machineName:              "foo-machine-1",
			machineLabel:             "foo",
			expectedBootstrapDataKey: path.Join("node", "foo-machine-1"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			machineScope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.machineName,
					},
				},
				Machine: &capiv1beta1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							tc.machineLabel: "",
						},
					},
				},
			}
			g.Expect(tc.expectedBootstrapDataKey).To(Equal(machineScope.bootstrapDataKey()))
		})
	}
}

func TestGetNetworkID(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	const networkID = "foo-network-id"
	t.Run("Get Network ID", func(t *testing.T) {
		t.Run("Fetch from Network spec's ID", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{}
			expectedNetworkID := networkID
			networkResource := infrav1beta2.IBMPowerVSResourceReference{
				ID: core.StringPtr(expectedNetworkID),
			}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(*result).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})
		t.Run("Fetch network ID from PowerVS Machine scope", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			networkName := "foo-network-name"
			expectedNetworkID := networkID
			networkReferences := []*models.NetworkReference{
				{
					Name:      core.StringPtr(networkName),
					NetworkID: core.StringPtr(expectedNetworkID),
				},
			}
			networks := &models.Networks{
				Networks: networkReferences,
			}
			networkResource := infrav1beta2.IBMPowerVSResourceReference{
				Name: core.StringPtr(networkName),
			}

			mockResourceController := mock.NewMockPowerVS(gomock.NewController(t))
			mockResourceController.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockResourceController,
			}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(*result).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})

		t.Run("Failed to find network ID", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedNetworkIName := "foo-network"
			differentNetworkName := "diff-network-name"
			networkReferences := []*models.NetworkReference{
				{
					Name: core.StringPtr(differentNetworkName),
				},
			}
			networks := &models.Networks{
				Networks: networkReferences,
			}
			networkResource := infrav1beta2.IBMPowerVSResourceReference{
				Name: core.StringPtr(expectedNetworkIName),
			}

			mockResourceController := mock.NewMockPowerVS(gomock.NewController(t))
			mockResourceController.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockResourceController,
			}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal(fmt.Sprintf("failed to find a network ID with name %s", expectedNetworkIName)))
		})

		t.Run("Fetch network ID with matching regex", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			networkName := "550e8400-e29b-41d4-a716-446655440000"
			expectedNetworkID := "foo-id"
			networkReferences := []*models.NetworkReference{
				{
					Name:      core.StringPtr(networkName),
					NetworkID: core.StringPtr(expectedNetworkID),
				},
			}
			networks := &models.Networks{
				Networks: networkReferences,
			}
			networkResource := infrav1beta2.IBMPowerVSResourceReference{
				RegEx: core.StringPtr("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"),
			}

			mockResourceController := mock.NewMockPowerVS(gomock.NewController(t))
			mockResourceController.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockResourceController,
			}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(*result).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})

		t.Run("Failed to fetch network ID with matching regex", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedNetworkID := "foo-netID"
			regex := "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
			networkReferences := []*models.NetworkReference{
				{
					Name: core.StringPtr(expectedNetworkID),
				},
			}
			networks := &models.Networks{
				Networks: networkReferences,
			}
			networkResource := infrav1beta2.IBMPowerVSResourceReference{
				RegEx: core.StringPtr(regex),
			}

			mockResourceController := mock.NewMockPowerVS(gomock.NewController(t))
			mockResourceController.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockResourceController,
			}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal(fmt.Sprintf("failed to find a network ID with RegEx %s", regex)))
		})

		t.Run("ID name and regex are all nil", func(t *testing.T) {
			g := NewWithT(t)
			networkResource := infrav1beta2.IBMPowerVSResourceReference{}
			scope := PowerVSMachineScope{}
			result, err := getNetworkID(networkResource, &scope)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("ID, Name and RegEx can't be nil"))
		})
	})
}

func TestGetMachineInternalIP(t *testing.T) {
	t.Run("Get Machine Internal IP", func(t *testing.T) {
		t.Run("Address type - Node Internal IP", func(t *testing.T) {
			g := NewWithT(t)
			expectedAddress := "10.0.0.1"
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: expectedAddress,
							},
						},
					},
				},
			}
			g.Expect(expectedAddress).To(Equal(scope.GetMachineInternalIP()))
		})

		t.Run("Address type = node external IP", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeExternalIP,
								Address: "198.0.0.1",
							},
						},
					},
				},
			}
			g.Expect("").To(Equal(scope.GetMachineInternalIP()))
		})
	})
}

func TestSetProviderID(t *testing.T) {
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

	t.Run("Set Provider ID", func(t *testing.T) {
		providerID := core.StringPtr("foo-provider-id")
		t.Run("Set Provider ID in v2 format", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			options.ProviderIDFormat = string(options.ProviderIDFormatV2)
			scope.SetZone("us-south-1")
			scope.SetRegion(region)
			scope.IBMPowerVSCluster.Spec.ServiceInstanceID = "service-instance-1"
			scope.SetProviderID(providerID)
			expectedProviderID := ptr.To(fmt.Sprintf("ibmpowervs://%s/%s/%s/%s", scope.GetRegion(), scope.GetZone(), scope.GetServiceInstanceID(), *providerID))
			g.Expect(*scope.IBMPowerVSMachine.Spec.ProviderID).To(Equal(*expectedProviderID))
		})

		t.Run("Provider ID is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			options.ProviderIDFormat = string(options.ProviderIDFormatV2)
			scope.SetProviderID(nil)
			g.Expect(scope.IBMPowerVSMachine.Spec.ProviderID).To(BeNil())
		})

		t.Run("Set Provider ID in v1 format", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			options.ProviderIDFormat = string(options.ProviderIDFormatV1)
			scope.SetProviderID(providerID)
			expectedProviderID := ptr.To(fmt.Sprintf("ibmpowervs://%s/%s", scope.Machine.Spec.ClusterName, scope.IBMPowerVSMachine.Name))
			g.Expect(*scope.IBMPowerVSMachine.Spec.ProviderID).To(Equal(*expectedProviderID))
		})
	})
}

func TestCreateCOSClient(t *testing.T) {
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

	t.Run("Create COS client", func(t *testing.T) {
		mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		t.Run("Create ignition data - error getting instance", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(nil, errors.New("error listing cos instances"))
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient()
			g.Expect(result).To(BeNil())
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("Create ignition data - empty service instance", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(nil, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient()
			g.Expect(result).To(BeNil())
			g.Expect(err).To(BeNil())
		})

		t.Run("Create ignition data - service instance is not in active state", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			serviceInstance := new(resourcecontrollerv2.ResourceInstance)
			state := string(infrav1beta2.ServiceInstanceStateProvisioning)
			serviceInstance.State = &state
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient()
			expectedError := fmt.Sprintf("COS service instance is not in active state, current state: %s", infrav1beta2.ServiceInstanceStateProvisioning)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(ContainSubstring(expectedError))
		})

		t.Run("Create ignition data - bucket region not set", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			serviceInstance := new(resourcecontrollerv2.ResourceInstance)
			state := string(infrav1beta2.ServiceInstanceStateActive)
			serviceInstance.State = &state
			scope.SetRegion(region)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient()
			expectedError := "failed to determine COS bucket region, both bucket region and VPC region not set"
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(ContainSubstring(expectedError))
		})
		t.Run("Create ignition data - success", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			serviceInstance := new(resourcecontrollerv2.ResourceInstance)
			state := string(infrav1beta2.ServiceInstanceStateActive)
			serviceInstance.State = &state
			guid := "foo-guid"
			serviceInstance.GUID = &guid
			scope.SetRegion(region)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			expectedBucketRegion := region
			scope.IBMPowerVSCluster.Spec.CosInstance = &infrav1beta2.CosInstance{BucketRegion: expectedBucketRegion}
			_, err := scope.createCOSClient()
			g.Expect(err).To(BeNil())
		})
	})
}

func TestClose(t *testing.T) {
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

	t.Run("Test Close", func(t *testing.T) {
		t.Run("IBMPowerVSMachine is not nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			patchHelper, _ := patch.NewHelper(scope.IBMPowerVSMachine, scope.Client)
			scope.patchHelper = patchHelper
			err := scope.Close()
			g.Expect(err).To(BeNil())
		})
		t.Run("IBMPowerVSMachine is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			patchHelper, _ := patch.NewHelper(scope.IBMPowerVSMachine, scope.Client)
			scope.patchHelper = patchHelper
			scope.IBMPowerVSMachine = nil
			err := scope.Close()
			g.Expect(err).ToNot(BeNil())
		})
	})
}

func TestSetInstanceID(t *testing.T) {
	t.Run("Test Close", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{},
			},
		}
		instanceID := "foo-instance-id"
		scope.SetInstanceID(ptr.To(instanceID))
		g.Expect(scope.GetInstanceID()).To(Equal(instanceID))
	})
}

func TestSetFailureReason(t *testing.T) {
	t.Run("Test SetFailureReason", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetFailureReason(capierrors.InvalidConfigurationMachineError)
		g.Expect(*scope.IBMPowerVSMachine.Status.FailureReason).To(Equal(capierrors.InvalidConfigurationMachineError))
	})
}

func TestSetHealth(t *testing.T) {
	t.Run("Test SetHealth", func(t *testing.T) {
		t.Run("Test SetHealth - status healthy", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{},
				},
			}
			healthStatus := &models.PVMInstanceHealth{
				Status: "healthy",
			}
			scope.SetHealth(healthStatus)
			g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal(healthStatus.Status))
		})
		t.Run("Test SetHealth - nil health status", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{},
				},
			}
			scope.SetHealth(nil)
			g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal(""))
		})
	})
}

func TestSetFailureMessage(t *testing.T) {
	t.Run("Test SetFailureMessage", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
				Status: infrav1beta2.IBMPowerVSMachineStatus{},
			},
		}
		failureMessage := "invalid configuration provided"
		scope.SetFailureMessage(failureMessage)
		g.Expect(*scope.IBMPowerVSMachine.Status.FailureMessage).To(Equal(failureMessage))
	})
}
func TestDeleteMachineIgnition(t *testing.T) {
	t.Run("Delete machine ignition", func(t *testing.T) {
		t.Run("Failed to retrieve bootstrap data: linked Machine's bootstrap.dataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				Machine: &capiv1beta1.Machine{
					Spec: capiv1beta1.MachineSpec{
						Bootstrap: capiv1beta1.Bootstrap{
							DataSecretName: nil,
						},
					},
				},
			}
			err := scope.DeleteMachineIgnition()
			g.Expect(err).ToNot(BeNil())
		})
		t.Run("Machine is not using user data of type ignition", func(t *testing.T) {
			g := NewWithT(t)
			bootstrapSecret := newBootstrapSecret(clusterName, machineName)
			initObjects := []client.Object{
				bootstrapSecret,
			}
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
			scope := PowerVSMachineScope{
				Client: client,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{},
				},
				Machine: &capiv1beta1.Machine{
					Spec: capiv1beta1.MachineSpec{
						Bootstrap: capiv1beta1.Bootstrap{
							DataSecretName: core.StringPtr(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			err := scope.DeleteMachineIgnition()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error creating COS client", func(t *testing.T) {
			g := NewWithT(t)
			bootstrapSecret := newBootstrapSecret(clusterName, machineName)
			initObjects := []client.Object{
				bootstrapSecret,
			}
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
			mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
			cosInstanceName := fmt.Sprintf("%s-%s", clusterName, "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(nil, errors.New("error listing cos instances"))
			scope := PowerVSMachineScope{
				Client: client,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						Ignition: &infrav1beta2.Ignition{
							Version: "3.1",
						},
					},
				},
				ResourceClient: mockResourceController,
				Machine: &capiv1beta1.Machine{
					Spec: capiv1beta1.MachineSpec{
						Bootstrap: capiv1beta1.Bootstrap{
							DataSecretName: core.StringPtr(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			err := scope.DeleteMachineIgnition()
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("Test creating COS client", func(t *testing.T) {
			g := NewWithT(t)
			bootstrapSecret := newBootstrapSecret(clusterName, machineName)
			initObjects := []client.Object{
				bootstrapSecret,
			}
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
			mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
			cosInstanceName := fmt.Sprintf("%s-%s", clusterName, "cosinstance")
			serviceInstance := new(resourcecontrollerv2.ResourceInstance)
			state := string(infrav1beta2.ServiceInstanceStateActive)
			serviceInstance.State = &state
			guid := "foo-guid"
			serviceInstance.GUID = &guid
			expectedBucketRegion := region
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope := PowerVSMachineScope{
				Client: client,
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{},
				},
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						Ignition: &infrav1beta2.Ignition{
							Version: "3.1",
						},
						CosInstance: &infrav1beta2.CosInstance{
							BucketRegion: expectedBucketRegion,
						},
					},
				},
				ResourceClient: mockResourceController,
				Machine: &capiv1beta1.Machine{
					Spec: capiv1beta1.MachineSpec{
						Bootstrap: capiv1beta1.Bootstrap{
							DataSecretName: core.StringPtr(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			scope.SetRegion(region)
			err := scope.DeleteMachineIgnition()
			g.Expect(err).To(BeNil())
		})
	})
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
					ImageID: core.StringPtr(pvsImage + idSuffix),
				},
			},
		}
		networks := &models.Networks{
			Networks: []*models.NetworkReference{
				{
					Name:      core.StringPtr(pvsNetwork),
					NetworkID: core.StringPtr(pvsNetwork + idSuffix),
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
			g.Expect(out.ServerName).To(Equal(expectedOutput.ServerName))
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
			g.Expect(out).To(Equal(expectedOutput))
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
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestCreateVPCLoadBalancerPoolMemberPowerVSMachine(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	nodeAddress := "10.0.0.1"
	loadBalancerID := "xyz-xyz-xyz"
	t.Run("Create VPC Load Balancer Pool Member", func(t *testing.T) {
		t.Run("No load balancer present", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: nil,
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("failed to find VPC load balancer ID"))
		})

		t.Run("Error getting load balancers from VPC Client", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockClient := vpcmock.NewMockVpc(mockCtrl)
			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: core.StringPtr(loadBalancerID)}).Return(nil, nil, errors.New("Error getting load balancer"))
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: "load-balancer-0",
								ID:   core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"load-balancer-0": {
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(result).To(BeNil())
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("VPC load balancer is not in active state", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancers := &vpcv1.LoadBalancer{
				ProvisioningStatus: (*string)(&infrav1beta2.VPCLoadBalancerStateCreatePending),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: core.StringPtr(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: "load-balancer-0",
								ID:   core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"load-balancer-0": {
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("VPC load balancer is not in active state"))
		})

		t.Run("No pools exist for the VPC load balancer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancers := &vpcv1.LoadBalancer{
				ProvisioningStatus: (*string)(&infrav1beta2.VPCLoadBalancerStateActive),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)
			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: core.StringPtr(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: "load-balancer-0",
								ID:   core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"load-balancer-0": {
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("no pools exist for the VPC load balancer"))
		})

		t.Run("Created load balancer pool member", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancerName := "load-balancer-0"
			targetPort := 3430
			loadBalancers := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr(loadBalancerID),
				Name:               core.StringPtr(loadBalancerName),
				ProvisioningStatus: (*string)(&infrav1beta2.VPCLoadBalancerStateActive),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   core.StringPtr("pool-id-0"),
						Name: core.StringPtr("externallyCreatedPool"),
					}, {
						ID:   core.StringPtr("pool-id-1"),
						Name: core.StringPtr("no-target-port-pool"),
					}, {
						ID:   core.StringPtr("pool-id-2"),
						Name: core.StringPtr(fmt.Sprintf("pool-2-%d", targetPort)),
					},
				},
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: nodeAddress,
								Type:    corev1.NodeInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
				},
			}

			mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
			mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
			expectedLoadBalancerPoolMemberID := "pool-member-2"
			expectedLoadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{ID: core.StringPtr(expectedLoadBalancerPoolMemberID)}
			mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember, nil, nil).AnyTimes()
			result, err := scope.CreateVPCLoadBalancerPoolMember()

			g.Expect(err).To(BeNil())
			g.Expect(*result.ID).To(Equal(expectedLoadBalancerPoolMemberID))
		})

		t.Run("Failed to find VPC load balancer ID", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{},
					},
				},
			}
			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(err.Error()).To(Equal("failed to find VPC load balancer ID"))
			g.Expect(result).To(BeNil())
		})

		t.Run("Created load balancer pool member", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancerName := "load-balancer-0"

			targetPort := 3430
			loadBalancers := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr(loadBalancerID),
				Name:               core.StringPtr(loadBalancerName),
				ProvisioningStatus: (*string)(&infrav1beta2.VPCLoadBalancerStateActive),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   core.StringPtr("pool-id-2"),
						Name: core.StringPtr(fmt.Sprintf("pool-2-%d", targetPort)),
					},
				},
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: nodeAddress,
								Type:    corev1.NodeInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   core.StringPtr(loadBalancerID),
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: core.StringPtr(loadBalancerID),
							},
						},
					},
				},
			}

			mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
			loadBalancerPoolMemberCollection := &vpcv1.LoadBalancerPoolMemberCollection{
				Members: []vpcv1.LoadBalancerPoolMember{
					{
						Port: core.Int64Ptr(3040),
						Target: &vpcv1.LoadBalancerPoolMemberTarget{
							Address: core.StringPtr(nodeAddress),
						},
					},
				},
			}
			mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, nil, nil).AnyTimes()
			expectedLoadBalancerPoolMemberID := "pool-member-2"
			expectedLoadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{ID: core.StringPtr(expectedLoadBalancerPoolMemberID)}
			mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember, nil, nil).AnyTimes()
			result, err := scope.CreateVPCLoadBalancerPoolMember()
			g.Expect(result).To(BeNil())
			g.Expect(err).To(BeNil())
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
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
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
				ServerName: ptr.To(instanceName),
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
				ServerName: ptr.To(instanceName),
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
				ServerName: ptr.To(instanceName),
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
				ServerName: ptr.To(instanceName),
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
							NetworkID: ptr.To("test-ID"),
							Name:      ptr.To("test-name"),
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
			scope := setupPowerVSMachineScope("test-cluster", "test-machine-0", ptr.To("test-image-ID"), &networkID, tc.setNetworkID, mockPowerVSClient)
			scope.DHCPIPCacheStore = tc.dhcpCacheStoreFunc()
			scope.SetAddresses(tc.pvmInstance)
			g.Expect(scope.IBMPowerVSMachine.Status.Addresses).To(Equal(tc.expectedNodeAddress))
		})
	}
}
