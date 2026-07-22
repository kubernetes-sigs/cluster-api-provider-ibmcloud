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

package powervs

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
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcecontrollermock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

const (
	testListenerSelector = "listener-selector"
)

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1.IBMPowerVSMachine {
	var image infrav1.IBMPowerVSMachineImage
	network := infrav1.ResourceIdentifier{}

	if imageRef == nil {
		// No image reference supplied — caller is expected to set IBMPowerVSImage (Import path).
		image.Type = infrav1.ImageSourceTypeImport
	} else if !isID {
		image.Type = infrav1.ImageSourceTypeReference
		image.Reference.Name = *imageRef
	} else {
		image.Type = infrav1.ImageSourceTypeReference
		image.Reference.ID = *imageRef
	}

	if networkRef != nil {
		if isID {
			network.ID = *networkRef
		} else {
			network.Name = *networkRef
		}
	}

	return &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      machineName,
			Namespace: defaultNamespace,
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromInt(1),
			Image:      image,
			Network:    network,
		},
	}
}

func setupPowerVSMachineScope(clusterName string, machineName string, imageID *string, networkID *string, isID bool, mockpowervs *mock.MockPowerVS) *MachineScope {
	cluster := newCluster(clusterName)
	machine := newMachine(machineName)
	secret := newBootstrapSecret(clusterName, machineName)
	powerVSMachine := newPowerVSMachine(clusterName, machineName, imageID, networkID, isID)
	powerVSCluster := newPowerVSCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, powerVSCluster, powerVSMachine,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &MachineScope{
		Client:            fakeClient,
		IBMPowerVSClient:  mockpowervs,
		Cluster:           cluster,
		Machine:           machine,
		IBMPowerVSCluster: powerVSCluster,
		IBMPowerVSMachine: powerVSMachine,
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

func TestNewPowerVSMachineScope(t *testing.T) {
	testCases := []struct {
		name   string
		params MachineScopeParams
	}{
		{
			name: "Returns error when controller runtime client in nil",
			params: MachineScopeParams{
				Client: nil,
			},
		},
		{
			name: "Returns error when Machine in nil",
			params: MachineScopeParams{
				Client:  testEnv.Client,
				Machine: nil,
			},
		},
		{
			name: "Returns error when Cluster is nil",
			params: MachineScopeParams{
				Client:  testEnv.Client,
				Machine: newMachine(machineName),
				Cluster: nil,
			},
		},
		{
			name: "Returns error when IBMPowerVSMachine is nil",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: nil,
			},
		},
		{
			name: "Error initialising authenticator",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true),
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewMachineScope(context.Background(), tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestGetServiceInstanceIDForMachineScope(t *testing.T) {
	testcases := []struct {
		name                      string
		expectedServiceInstanceID string
		expectedError             error
		machineScope              MachineScope
	}{
		{
			name:                      "Returns service instance ID set in IBMPowerVSCluster.Status.ServiceInstance.ID",
			expectedServiceInstanceID: "service-instance-0",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReferenceV1Beta3{
							ID: "service-instance-0",
						},
					},
				},
			},
		}, {
			name:                      "get service instance ID from powervsClusterStatus when machine spec is empty",
			expectedServiceInstanceID: "service-instance-1",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReferenceV1Beta3{
							ID: "service-instance-1",
						},
					},
				},
			},
		}, {
			name:                      "get service instance ID with serviceInstanceID present in both IBMPowerVSCluster Status and Spec ",
			expectedServiceInstanceID: "service-instance-0",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReferenceV1Beta3{
							ID: "service-instance-0",
						},
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Workspace: infrav1.WorkspaceSource{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "service-instance-in-spec",
							},
						},
					},
				},
			},
		}, {
			name: "Failed to find service instance id",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						Workspace: infrav1.WorkspaceSource{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{},
						},
					},
				},
			},
			expectedError: fmt.Errorf("failed to find workspace ID: not specified in Machine spec and not yet populated in Cluster status"),
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			serviceInstanceID, err := tc.machineScope.GetWorkspaceID()
			g.Expect(serviceInstanceID).To(Equal(tc.expectedServiceInstanceID))
			if tc.expectedError != nil {
				g.Expect(err).To(Equal(tc.expectedError))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}

	// Running other test cases which need some mock calls to be defined
	var mockCtrl *gomock.Controller

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
	t.Run("Returns service instance ID successfully when name is set in spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-south-1",
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{
						Name: "foo-cluster",
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("foo-id")}, nil)
		scope.ResourceClient = mockResourceController
		serviceInstanceID, err := scope.GetWorkspaceID()
		g.Expect(serviceInstanceID).To(Equal("foo-id"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Failed to get Power VS service instance id", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-south-1",
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{
						Name: "foo-cluster",
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{Name: "foo-cluster"})).Return(nil, fmt.Errorf("failed to list instance id"))
		scope.ResourceClient = mockResourceController
		serviceInstanceID, err := scope.GetWorkspaceID()
		g.Expect(serviceInstanceID).To(Equal(""))
		g.Expect(err).ToNot(BeNil())
	})
}

func TestSetReady(t *testing.T) {
	t.Run("Set Machine status to ready", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		machineScope.SetReady()
		g.Expect(machineScope.IsReady()).To(Equal(true))
	})
}

func TestSetNotReady(t *testing.T) {
	t.Run("Set status of machine as not ready", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Initialization: infrav1.IBMPowerVSMachineInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
		}
		machineScope.SetNotReady()
		g.Expect(machineScope.IsReady()).To(Equal(false))
	})
}

func TestGetRegion(t *testing.T) {
	testcases := []struct {
		name           string
		scope          MachineScope
		expectedRegion string
	}{
		{
			name: "Returns region set in spec",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Region: region,
					},
				},
			},
			expectedRegion: region,
		}, {
			name: "Return empty string when region is not set in spec",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			g.Expect(tc.scope.GetRegion()).To(Equal(tc.expectedRegion))
		})
	}
}

func TestSetRegion(t *testing.T) {
	testcases := []struct {
		name           string
		scope          MachineScope
		expectedRegion string
	}{
		{
			name: "Set region to us-east in IBMPowerVSMachine status",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedRegion: "us-east",
		}, {
			name: "Set region to empty value in IBMPowerVSMachine status",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			tc.scope.SetRegion(tc.expectedRegion)
			g.Expect(tc.scope.GetRegion()).To(Equal(tc.expectedRegion))
		})
	}
}

func TestGetZone(t *testing.T) {
	testcases := []struct {
		name         string
		scope        MachineScope
		expectedZone string
	}{
		{
			name: "Machine's zone is set",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Zone: "us-south-1",
					},
				},
			},
			expectedZone: "us-south-1",
		}, {
			name: "Machine's zone is empty",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			g.Expect(tc.scope.GetZone()).To(Equal(tc.expectedZone))
		})
	}
}

func TestSetZone(t *testing.T) {
	testcases := []struct {
		name         string
		scope        MachineScope
		expectedZone string
	}{
		{
			name: "Set machine's zone to us-east-1",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedZone: "us-east-1",
		}, {
			name: "Set machine's zone to an empty value",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			tc.scope.SetZone(tc.expectedZone)
			g.Expect(tc.scope.GetZone()).To(Equal(tc.expectedZone))
		})
	}
}

func TestGetInstanceState(t *testing.T) {
	t.Run("Set PowerVS instance state to ready", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		machineScope.SetInstanceState(ptr.To("ready"))
		g.Expect(machineScope.GetInstanceState()).To(Equal(infrav1.PowerVSInstanceState("ready")))
	})
}

func TestGetIgnitionVersion(t *testing.T) {
	testcases := []struct {
		name                    string
		expectedIgnitionVersion string
		scope                   MachineScope
	}{
		{
			name:                    "Ignition version is nil",
			expectedIgnitionVersion: infrav1.DefaultIgnitionVersion,
			scope: MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		}, {
			name:                    "Custom Ignition Version is set",
			expectedIgnitionVersion: "3.4",
			scope: MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						Ignition: infrav1.Ignition{
							Version: "3.4",
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			g.Expect(tc.scope.getIgnitionVersion()).To(Equal(tc.expectedIgnitionVersion))
		})
	}
}

func TestBootstrapDataKey(t *testing.T) {
	testcases := []struct {
		name                     string
		machineLabel             string
		machineName              string
		expectedBootstrapDataKey string
	}{
		{
			name:                     "Returns BootstrapDataKey for a machine in control plane",
			machineLabel:             clusterv1.MachineControlPlaneLabel,
			machineName:              "foo-machine-0",
			expectedBootstrapDataKey: path.Join("control-plane", "foo-machine-0"),
		},
		{
			name:                     "Returns BootstrapDataKey for a worker node",
			machineName:              "foo-machine-1",
			machineLabel:             "foo",
			expectedBootstrapDataKey: path.Join("node", "foo-machine-1"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(_ *testing.T) {
			g := NewWithT(t)
			machineScope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.machineName,
					},
				},
				Machine: &clusterv1.Machine{
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
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	const networkID = "foo-network-id"
	t.Run("Get Network ID", func(t *testing.T) {
		t.Run("Returns networkID from Network spec's ID", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{}
			expectedNetworkID := networkID
			networkResource := infrav1.ResourceIdentifier{
				ID: expectedNetworkID,
			}
			networkID, err := scope.getNetworkID(context.Background(), networkResource)
			g.Expect(*networkID).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})
		t.Run("Returns network ID from PowerVS Machine scope", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			networkName := "foo-network-name"
			expectedNetworkID := networkID
			networkResource := infrav1.ResourceIdentifier{
				Name: networkName,
			}

			mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), networkName).Return(&models.NetworkReference{
				NetworkID: ptr.To(expectedNetworkID),
				Name:      ptr.To(networkName),
			}, nil)
			scope := MachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := scope.getNetworkID(context.Background(), networkResource)
			g.Expect(*networkID).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})

		t.Run("Failed to find network ID", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedNetworkIName := "foo-network"

			networkResource := infrav1.ResourceIdentifier{
				Name: expectedNetworkIName,
			}

			mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), expectedNetworkIName).Return(nil, nil)
			scope := MachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := scope.getNetworkID(context.Background(), networkResource)
			g.Expect(networkID).To(BeNil())
			g.Expect(err.Error()).To(Equal(fmt.Sprintf("network with name %q not found", expectedNetworkIName)))
		})

		t.Run("When ID and name are both empty", func(t *testing.T) {
			g := NewWithT(t)
			networkResource := infrav1.ResourceIdentifier{}
			scope := MachineScope{}
			networkID, err := scope.getNetworkID(context.Background(), networkResource)
			g.Expect(networkID).To(BeNil())
			g.Expect(err.Error()).To(Equal("network identifier must contain either an ID or a Name"))
		})
	})
}

func TestGetMachineInternalIP(t *testing.T) {
	t.Run("Get Machine Internal IP", func(t *testing.T) {
		t.Run("Returns machine IP for address type - Node Internal IP", func(t *testing.T) {
			g := NewWithT(t)
			expectedAddress := "10.0.0.1"
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []clusterv1.MachineAddress{
							{
								Type:    clusterv1.MachineInternalIP,
								Address: expectedAddress,
							},
						},
					},
				},
			}
			g.Expect(expectedAddress).To(Equal(scope.GetMachineInternalIP()))
		})

		t.Run("Returns empty IP for address type - node external IP", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []clusterv1.MachineAddress{
							{
								Type:    clusterv1.MachineExternalIP,
								Address: "198.0.0.1",
							},
						},
					},
				},
			}
			g.Expect("").To(Equal(scope.GetMachineInternalIP()))
		})

		t.Run("Returns empty IP if powervsmachineStatus in nil", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
			}
			g.Expect("").To(Equal(scope.GetMachineInternalIP()))
		})
	})
}

func TestSetProviderID(t *testing.T) {
	providerID := "foo-provider-id"

	t.Run("Set Provider ID in invalid format", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		options.ProviderIDFormat = "v1"
		err := scope.SetProviderID(providerID)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("failed to get service instance ID", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
		}
		options.ProviderIDFormat = string(options.ProviderIDFormatV2)
		err := scope.SetProviderID(providerID)
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("Set Provider ID in v2 format", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "foo-service-instance-id",
					},
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
		}
		options.ProviderIDFormat = string(options.ProviderIDFormatV2)
		scope.SetZone("us-south-1")
		scope.SetRegion(region)
		err := scope.SetProviderID(providerID)
		expectedProviderID := fmt.Sprintf("ibmpowervs://%s/%s/%s/%s", scope.GetRegion(), scope.GetZone(), "foo-service-instance-id", providerID)
		g.Expect(scope.IBMPowerVSMachine.Spec.ProviderID).To(Equal(expectedProviderID))
		g.Expect(err).To(BeNil())
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
		t.Run("Returns error when COS instance ID is not in cluster status", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			// Status.COSInstance.ID is empty by default
			result, err := scope.createCOSClient(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err).To(MatchError(ContainSubstring("COS instance ID is not yet populated in cluster status")))
		})

		t.Run("Returns error when COS bucket region is not set in spec", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSCluster.Status.COSInstance = infrav1.COSInstanceStatus{ID: "cos-instance-id"}
			// Spec.COSInstance.BucketRegion is empty — should fail after API key check
			result, err := scope.createCOSClient(ctx)
			g.Expect(result).To(BeNil())
			// Will fail at API key or bucket region; either is acceptable
			g.Expect(err).ToNot(BeNil())
		})
	})
}

func TestSetInstanceID(t *testing.T) {
	testcases := []struct {
		name               string
		instanceID         *string
		expectedInstanceID string
	}{
		{
			name:               "Set instance ID with value",
			instanceID:         ptr.To("foo-instance-id"),
			expectedInstanceID: "foo-instance-id",
		}, {
			name:       "Set instance ID to nil",
			instanceID: nil,
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			}
			scope.SetInstanceID(tc.instanceID)
			g.Expect(scope.GetInstanceID()).To(Equal(tc.expectedInstanceID))
		})
	}
}

func TestSetHealth(t *testing.T) {
	t.Run("Test SetHealth", func(t *testing.T) {
		t.Run("Set PVMInstance status to healthy", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			}
			healthStatus := &models.PVMInstanceHealth{
				Status: "healthy",
			}
			scope.SetHealth(healthStatus)
			g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal(healthStatus.Status))
		})
		t.Run("Set PVMInstance status to nil", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			}
			scope.SetHealth(nil)
			g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal(""))
		})
	})
}

func TestDeleteMachineIgnition(t *testing.T) {
	t.Run("Delete machine ignition", func(t *testing.T) {
		t.Run("Skips when COSInstance type is not set (Ignition not configured)", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{},
				},
				Machine: &clusterv1.Machine{},
			}
			err := scope.DeleteMachineIgnition(ctx)
			g.Expect(err).To(BeNil())
		})
		t.Run("Skips when bucket name not yet populated in cluster status", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						COSInstance: infrav1.COSInstanceSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
					// Status.COSInstance.BucketName is empty → deletion skipped
				},
				Machine: &clusterv1.Machine{},
			}
			err := scope.DeleteMachineIgnition(ctx)
			g.Expect(err).To(BeNil())
		})
		t.Run("Error creating COS client when COS instance ID not in status", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						COSInstance: infrav1.COSInstanceSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						COSInstance: infrav1.COSInstanceStatus{
							BucketName: "test-bucket",
							// ID is empty → createCOSClient will fail
						},
					},
				},
				Machine: &clusterv1.Machine{},
			}
			err := scope.DeleteMachineIgnition(ctx)
			g.Expect(err).To(MatchError(ContainSubstring("COS instance ID is not yet populated in cluster status")))
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
		const idSuffix = "-id"
		pvmInstances := &models.PVMInstances{
			PvmInstances: []*models.PVMInstanceReference{
				{
					ServerName:    ptr.To("foo-machine-1"),
					PvmInstanceID: ptr.To("foo-machine-1-id"),
				},
			},
		}
		images := &models.Images{
			Images: []*models.ImageReference{
				{
					Name:    ptr.To(pvsImage),
					ImageID: ptr.To(pvsImage + idSuffix),
				},
			},
		}
		pvmInstanceList := &models.PVMInstanceList{}
		pvmInstanceCreate := &models.PVMInstanceCreate{}

		t.Run("Should create Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Return exsisting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedOutput := models.PVMInstanceReference{
				ServerName: ptr.To("foo-machine-1"),
			}
			scope := setupPowerVSMachineScope(clusterName, "foo-machine-1", ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			out, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
			g.Expect(out.ServerName).To(Equal(expectedOutput.ServerName))
		})

		t.Run("Return NIL when Machine is not present in the Instance list and Machine state is unknown", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedOutput := (*models.PVMInstanceReference)(nil)
			scope := setupPowerVSMachineScope(clusterName, "foo-machine-2", ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.Conditions = append(scope.IBMPowerVSMachine.Status.Conditions, metav1.Condition{
				Type:   infrav1.InstanceReadyCondition,
				Status: metav1.ConditionUnknown,
			})
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			out, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
			g.Expect(out).To(Equal(expectedOutput))
		})

		t.Run("Eror while getting instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, errors.New("error when getting list of instances"))
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("foo-secret-temp")
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to retrieve bootstrap data, secret value key is missing", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: clusterName,
					},
					Name:      machineName,
					Namespace: defaultNamespace,
				},
				Data: map[string][]byte{
					"val": []byte("user data"),
				}}
			g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Invalid processors value", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Spec.Processors = intstr.FromString("invalid")
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("IBMPowerVSImage is not nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSImage = &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{
					ImageID: "foo-image",
				},
			}
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Image and Network name is set", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().ListImages(gomock.Any()).Return(images, nil)
			mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), pvsNetwork).Return(&models.NetworkReference{
				NetworkID: ptr.To(pvsNetwork + "-id"),
				Name:      ptr.To(pvsNetwork),
			}, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when both Image id and name are nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when Image id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage+"-temp"), ptr.To(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().ListImages(gomock.Any()).Return(images, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when Network id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork+"-temp"), false, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().ListImages(gomock.Any()).Return(images, nil)
			mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), pvsNetwork+"-temp").Return(nil, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error while creating machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, errors.New("failed to create machine"))
			_, err := scope.CreateMachine(ctx)
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

	machineAddress := "10.0.0.1"
	loadBalancerID := "xyz-xyz-xyz"
	loadBalancerName := "load-balancer-0"
	t.Run("Skip adding listener if the machine label and listener label doesnot match", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		loadBalancerName := loadBalancerName
		loadBalancers := &vpcv1.LoadBalancer{
			ID:                 ptr.To(loadBalancerID),
			Name:               ptr.To(loadBalancerName),
			ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID:   ptr.To("pool-id-23"),
					Name: ptr.To("pool-23"),
				},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{
				{
					ID: ptr.To("pool-id-23"),
				},
			},
		}
		loadBalancerListener := &vpcv1.LoadBalancerListener{
			DefaultPool: &vpcv1.LoadBalancerPoolReference{
				Name: ptr.To("pool-23"),
			},
			ID:       ptr.To("pool-id-23"),
			Port:     ptr.To(int64(23)),
			Protocol: ptr.To("tcp"),
		}
		mockClient := vpcmock.NewMockVpc(mockCtrl)

		scope := MachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testListenerSelector: "port-22",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID:   loadBalancerID,
								Name: loadBalancerName,
							},
							Provision: infrav1.LoadBalancerProvision{
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port: 23,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												testListenerSelector: "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{
							Name: loadBalancerName,
							ID:   loadBalancerID,
						},
					},
				},
			},
		}

		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
		mockClient.EXPECT().GetLoadBalancerListener(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerListenerOptions{})).Return(loadBalancerListener, nil, nil).AnyTimes()
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
		result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(result).To(BeNil())
	})

	t.Run("Add listener if the machine label and listener label matches", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		loadBalancerName := loadBalancerName
		loadBalancers := &vpcv1.LoadBalancer{
			ID:                 ptr.To(loadBalancerID),
			Name:               ptr.To(loadBalancerName),
			ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID:   ptr.To("pool-id-22"),
					Name: ptr.To("pool-22"),
				},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{
				{
					ID: ptr.To("pool-id-22"),
				},
				{
					ID: ptr.To("pool-id-23"),
				},
			},
		}
		loadBalancerListener := &vpcv1.LoadBalancerListener{
			DefaultPool: &vpcv1.LoadBalancerPoolReference{
				Name: ptr.To("pool-22"),
			},
			ID:       ptr.To("pool-id-22"),
			Port:     ptr.To(int64(22)),
			Protocol: ptr.To("tcp"),
		}
		mockClient := vpcmock.NewMockVpc(mockCtrl)

		scope := MachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testListenerSelector: "port-22",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName},
							Provision: infrav1.LoadBalancerProvision{
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port: 22,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												testListenerSelector: "port-22",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{

							Name: loadBalancerName,

							ID: loadBalancerID,
						},
					},
				},
			},
		}

		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
		mockClient.EXPECT().GetLoadBalancerListener(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerListenerOptions{})).Return(loadBalancerListener, nil, nil).AnyTimes()
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
		expectedLoadBalancerPoolMemberID := "pool-member-3"
		expectedLoadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{ID: ptr.To(expectedLoadBalancerPoolMemberID)}
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember, nil, nil).AnyTimes()
		result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(*result.ID).To(Equal(expectedLoadBalancerPoolMemberID))
	})

	t.Run("Skip adding non control plane nodes if there is no selector", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		loadBalancerName := loadBalancerName
		loadBalancers := &vpcv1.LoadBalancer{
			ID:                 ptr.To(loadBalancerID),
			Name:               ptr.To(loadBalancerName),
			ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID:   ptr.To("pool-id-6443"),
					Name: ptr.To("pool-6443"),
				},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{
				{
					ID: ptr.To("pool-id-6443"),
				},
				{
					ID: ptr.To("pool-id-1"),
				},
			},
		}
		loadBalancerListener := &vpcv1.LoadBalancerListener{
			DefaultPool: &vpcv1.LoadBalancerPoolReference{
				Name: ptr.To("pool-6443"),
			},
			ID:       ptr.To("pool-id-6443"),
			Port:     ptr.To(int64(6443)),
			Protocol: ptr.To("tcp"),
		}
		mockClient := vpcmock.NewMockVpc(mockCtrl)

		scope := MachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testListenerSelector: "port-6443",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName},
							Provision: infrav1.LoadBalancerProvision{
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port: 6443,
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{

							Name: loadBalancerName,

							ID: loadBalancerID,
						},
					},
				},
			},
		}

		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
		mockClient.EXPECT().GetLoadBalancerListener(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerListenerOptions{})).Return(loadBalancerListener, nil, nil).AnyTimes()
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
		result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(result).To(BeNil())
	})
	t.Run("Adding control plane nodes even if there is no selector", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		loadBalancerName := loadBalancerName
		loadBalancers := &vpcv1.LoadBalancer{
			ID:                 ptr.To(loadBalancerID),
			Name:               ptr.To(loadBalancerName),
			ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID:   ptr.To("pool-id-6443"),
					Name: ptr.To("pool-6443"),
				},
				{
					ID:   ptr.To("pool-id-24"),
					Name: ptr.To("pool-24"),
				},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{
				{
					ID: ptr.To("pool-id-6443"),
				},
				{
					ID: ptr.To("pool-id-24"),
				},
			},
		}
		loadBalancerListener6443 := &vpcv1.LoadBalancerListener{
			DefaultPool: &vpcv1.LoadBalancerPoolReference{
				Name: ptr.To("pool-6443"),
			},
			ID:       ptr.To("pool-id-6443"),
			Port:     ptr.To(int64(6443)),
			Protocol: ptr.To("tcp"),
		}
		loadBalancerListener24 := &vpcv1.LoadBalancerListener{
			DefaultPool: &vpcv1.LoadBalancerPoolReference{
				Name: ptr.To("pool-24"),
			},
			ID:       ptr.To("pool-id-24"),
			Port:     ptr.To(int64(24)),
			Protocol: ptr.To("tcp"),
		}
		mockClient := vpcmock.NewMockVpc(mockCtrl)

		scope := MachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/control-plane": "true",
					},
				},
			},
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName},
							Provision: infrav1.LoadBalancerProvision{
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port: 6443,
									},
									{
										Port: 24,
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{

							Name: loadBalancerName,

							ID: loadBalancerID,
						},
					},
				},
			},
		}

		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
		mockClient.EXPECT().GetLoadBalancerListener(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerListenerOptions{LoadBalancerID: ptr.To(loadBalancerID), ID: ptr.To("pool-id-6443")})).Return(loadBalancerListener6443, nil, nil).AnyTimes()
		mockClient.EXPECT().GetLoadBalancerListener(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerListenerOptions{LoadBalancerID: ptr.To(loadBalancerID), ID: ptr.To("pool-id-24")})).Return(loadBalancerListener24, nil, nil).AnyTimes()
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
		expectedLoadBalancerPoolMemberID6443 := "pool-member-6443"
		expectedLoadBalancerPoolMember6443 := &vpcv1.LoadBalancerPoolMember{ID: ptr.To(expectedLoadBalancerPoolMemberID6443)}
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember6443, nil, nil).Times(1)
		result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(*result.ID).To(Equal(expectedLoadBalancerPoolMemberID6443))

		expectedLoadBalancerPoolMemberID24 := "pool-member-24"
		expectedLoadBalancerPoolMember24 := &vpcv1.LoadBalancerPoolMember{ID: ptr.To(expectedLoadBalancerPoolMemberID24)}
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember24, nil, nil).Times(1)
		result1, err1 := scope.CreateVPCLoadBalancerPoolMember(ctx)

		g.Expect(err1).To(BeNil())
		g.Expect(*result1.ID).To(Equal(expectedLoadBalancerPoolMemberID24))
	})
	t.Run("Create VPC Load Balancer Pool Member", func(t *testing.T) {
		t.Run("No load balancers present in status", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: nil,
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("failed to find VPC load balancer ID"))
		})

		t.Run("Error getting load balancers from VPC Client", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockClient := vpcmock.NewMockVpc(mockCtrl)
			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(loadBalancerID)}).Return(nil, nil, errors.New("error getting load balancer"))
			scope := MachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{

								Name: loadBalancerName,

								ID: loadBalancerID,
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("VPC load balancer is not in active state", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancers := &vpcv1.LoadBalancer{
				ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateCreatePending),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := MachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{

								Name: loadBalancerName,

								ID: loadBalancerID,
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(ContainSubstring("VPC load balancer is not in active state"))
		})

		t.Run("No pools exist for the VPC load balancer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			loadBalancers := &vpcv1.LoadBalancer{
				ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)
			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := MachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{

								Name: loadBalancerName,

								ID: loadBalancerID,
							},
						},
					},
				},
			}

			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(Equal("no pools exist for the VPC load balancer load-balancer-0"))
		})

		t.Run("Created load balancer pool member (when there are no members in the load balancer pool)", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			targetPort := 3430
			loadBalancers := &vpcv1.LoadBalancer{
				ID:                 ptr.To(loadBalancerID),
				Name:               ptr.To(loadBalancerName),
				ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   ptr.To("pool-id-0"),
						Name: ptr.To("externallyCreatedPool"),
					}, {
						ID:   ptr.To("pool-id-1"),
						Name: ptr.To("no-target-port-pool"),
					}, {
						ID:   ptr.To("pool-id-2"),
						Name: ptr.To(fmt.Sprintf("pool-2-%d", targetPort)),
					},
				},
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			scope := MachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []clusterv1.MachineAddress{
							{
								Address: machineAddress,
								Type:    clusterv1.MachineInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{

								Name: loadBalancerName,

								ID: loadBalancerID,
							},
						},
					},
				},
			}

			mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancers, nil, nil).AnyTimes()
			mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).AnyTimes()
			expectedLoadBalancerPoolMemberID := "pool-member-2"
			expectedLoadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{ID: ptr.To(expectedLoadBalancerPoolMemberID)}
			mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember, nil, nil).AnyTimes()
			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(err).To(BeNil())
			g.Expect(*result.ID).To(Equal(expectedLoadBalancerPoolMemberID))
		})

		t.Run("Failed to find VPC load balancer ID", func(t *testing.T) {
			g := NewWithT(t)
			scope := MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{},
					},
				},
			}
			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
			g.Expect(err.Error()).To(Equal("failed to find VPC load balancer ID"))
			g.Expect(result).To(BeNil())
		})

		t.Run("Created load balancer pool member (when target IP is already configured for pool)", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)

			targetPort := 3430
			loadBalancers := &vpcv1.LoadBalancer{
				ID:                 ptr.To(loadBalancerID),
				Name:               ptr.To(loadBalancerName),
				ProvisioningStatus: (*string)(&infrav1.LoadBalancerStateActive),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   ptr.To("pool-id-2"),
						Name: ptr.To(fmt.Sprintf("pool-2-%d", targetPort)),
					},
				},
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			scope := MachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []clusterv1.MachineAddress{
							{
								Address: machineAddress,
								Type:    clusterv1.MachineInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: loadBalancerID, Name: loadBalancerName}},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{

								Name: loadBalancerName,

								ID: loadBalancerID,
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
							Address: ptr.To(machineAddress),
						},
					},
				},
			}
			mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, nil, nil).AnyTimes()
			expectedLoadBalancerPoolMemberID := "pool-member-2"
			expectedLoadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{ID: ptr.To(expectedLoadBalancerPoolMemberID)}
			mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(expectedLoadBalancerPoolMember, nil, nil).AnyTimes()
			result, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
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
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
			mockpowervs.EXPECT().DeleteInstance(gomock.Any(), gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
			mockpowervs.EXPECT().DeleteInstance(gomock.Any(), gomock.AssignableToTypeOf(id)).Return(errors.New("failed to delete machine"))
			err := scope.DeleteMachine(ctx)
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
	defaultExpectedMachineAddress := []clusterv1.MachineAddress{
		{
			Type:    clusterv1.MachineInternalDNS,
			Address: instanceName,
		},
		{
			Type:    clusterv1.MachineHostName,
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
		expectedNodeAddress []clusterv1.MachineAddress
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
			expectedNodeAddress: append(defaultExpectedMachineAddress, clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
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
			expectedNodeAddress: append(defaultExpectedMachineAddress, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
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
			expectedNodeAddress: append(defaultExpectedMachineAddress, []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineInternalIP,
					Address: "192.168.10.3",
				},
				{
					Type:    clusterv1.MachineExternalIP,
					Address: "10.11.2.3",
				},
			}...),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "error while getting network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().GetNetworkByName(gomock.Any(), "test-net-ID").Return(nil, fmt.Errorf("intentional error"))
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
				mockPowerVSClient.EXPECT().GetNetworkByName(gomock.Any(), "test-net-ID").Return(nil, nil)
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
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(nil, fmt.Errorf("intentional error"))
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
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpServerID, "test-network"), nil)
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
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(gomock.Any(), dhcpServerID).Return(nil, fmt.Errorf("intentnional error"))
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
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(gomock.Any(), dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, "ff:11:33:dd:00:33"), nil)
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
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(gomock.Any(), dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, instanceMac), nil)
				return mockPowerVSClient
			},
			pvmInstance: newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: append(defaultExpectedMachineAddress, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: leaseIP,
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
			setNetworkID:       true,
		},
		{
			testcase: "ip stored in cache expired, fetch from dhcp server",
			powerVSClientFunc: func(ctrl *gomock.Controller) *mock.MockPowerVS {
				mockPowerVSClient := mock.NewMockPowerVS(ctrl)
				mockPowerVSClient.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpServerID, networkID), nil)
				mockPowerVSClient.EXPECT().GetDHCPServer(gomock.Any(), dhcpServerID).Return(newDHCPServerDetails(dhcpServerID, leaseIP, instanceMac), nil)
				return mockPowerVSClient
			},
			pvmInstance: newPowerVSInstance(instanceName, networkID, instanceMac),
			expectedNodeAddress: append(defaultExpectedMachineAddress, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
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
			expectedNodeAddress: append(defaultExpectedMachineAddress, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
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
			scope.SetAddresses(ctx, tc.pvmInstance)
			g.Expect(scope.IBMPowerVSMachine.Status.Addresses).To(Equal(tc.expectedNodeAddress))
		})
	}
}

func TestValidateSystemType(t *testing.T) {
	testCases := []struct {
		name              string
		systemType        string
		zone              string
		mockSetup         func(*mock.MockPowerVS)
		setupCache        func()
		expectedValid     bool
		expectedSupported []string
		expectError       bool
		errorContains     string
	}{
		{
			name:       "Valid system type with fresh cache",
			systemType: "s922",
			zone:       "us-south",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				sysCache.zonesMap["us-south"] = zoneCacheEntry{
					supportedTypes: []string{"e980", "s1022", "s922"},
					lastFetch:      time.Now(),
				}
			},
			expectedValid:     true,
			expectedSupported: []string{"e980", "s1022", "s922"},
			expectError:       false,
		},
		{
			name:       "Invalid system type with fresh cache",
			systemType: "invalid-type",
			zone:       "us-south",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				sysCache.zonesMap["us-south"] = zoneCacheEntry{
					supportedTypes: []string{"e980", "s1022", "s922"},
					lastFetch:      time.Now(),
				}
			},
			expectedValid:     false,
			expectedSupported: []string{"e980", "s1022", "s922"},
			expectError:       false,
		},
		{
			name:       "Empty system type",
			systemType: "",
			zone:       "us-south",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "us-south")
			},
			expectError:   true,
			errorContains: "systemType is not set",
		},
		{
			name:       "Valid system type with expired cache - fetches from API",
			systemType: "s1022",
			zone:       "us-east",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				// Set expired cache entry
				sysCache.zonesMap["us-east"] = zoneCacheEntry{
					supportedTypes: []string{"old-type"},
					lastFetch:      time.Now().Add(-7 * time.Hour), // Expired (TTL is 6 hours)
				}
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "us-east").Return(&models.Datacenter{
					CapabilitiesDetails: &models.CapabilitiesDetails{
						SupportedSystems: &models.SupportedSystems{
							General: []string{"e1080", "s1022", "s922"},
						},
					},
				}, nil)
			},
			expectedValid:     true,
			expectedSupported: []string{"e1080", "s1022", "s922"},
			expectError:       false,
		},
		{
			name:       "Cache miss - fetches from API successfully",
			systemType: "e1080",
			zone:       "eu-de",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "eu-de")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "eu-de").Return(&models.Datacenter{
					CapabilitiesDetails: &models.CapabilitiesDetails{
						SupportedSystems: &models.SupportedSystems{
							General: []string{"e1050", "e1080", "s922"},
						},
					},
				}, nil)
			},
			expectedValid:     true,
			expectedSupported: []string{"e1050", "e1080", "s922"},
			expectError:       false,
		},
		{
			name:       "API returns error",
			systemType: "s922",
			zone:       "jp-tok",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "jp-tok")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "jp-tok").Return(nil, errors.New("API error"))
			},
			expectError:   true,
			errorContains: "failed to get datacenter details",
		},
		{
			name:       "API returns nil datacenter",
			systemType: "s922",
			zone:       "ca-tor",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "ca-tor")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "ca-tor").Return(nil, nil)
			},
			expectError:   true,
			errorContains: "system capabilities details are missing",
		},
		{
			name:       "API returns datacenter with nil capabilities",
			systemType: "s922",
			zone:       "br-sao",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "br-sao")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "br-sao").Return(&models.Datacenter{
					CapabilitiesDetails: nil,
				}, nil)
			},
			expectError:   true,
			errorContains: "system capabilities details are missing",
		},
		{
			name:       "API returns empty system types list",
			systemType: "s922",
			zone:       "au-syd",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "au-syd")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "au-syd").Return(&models.Datacenter{
					CapabilitiesDetails: &models.CapabilitiesDetails{
						SupportedSystems: &models.SupportedSystems{
							General: []string{},
						},
					},
				}, nil)
			},
			expectError:   true,
			errorContains: "no general system types available",
		},
		{
			name:       "System types are sorted in cache",
			systemType: "s922",
			zone:       "test-zone",
			setupCache: func() {
				sysCache.mu.Lock()
				defer sysCache.mu.Unlock()
				delete(sysCache.zonesMap, "test-zone")
			},
			mockSetup: func(m *mock.MockPowerVS) {
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "test-zone").Return(&models.Datacenter{
					CapabilitiesDetails: &models.CapabilitiesDetails{
						SupportedSystems: &models.SupportedSystems{
							General: []string{"s922", "e980", "s1022", "e1050"}, // Unsorted
						},
					},
				}, nil)
			},
			expectedValid:     true,
			expectedSupported: []string{"e1050", "e980", "s1022", "s922"}, // Should be sorted
			expectError:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Setup cache
			if tc.setupCache != nil {
				tc.setupCache()
			}

			// Setup mock
			mockPowerVSClient := mock.NewMockPowerVS(ctrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockPowerVSClient)
			}

			// Create machine scope
			scope := setupPowerVSMachineScope("test-cluster", "test-machine", ptr.To("test-image"), ptr.To("test-network"), true, mockPowerVSClient)
			scope.IBMPowerVSMachine.Spec.SystemType = tc.systemType
			scope.SetZone(tc.zone)

			// Execute validation
			valid, supportedTypes, err := scope.validateSystemType(context.Background())

			// Assertions
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(valid).To(Equal(tc.expectedValid))
				g.Expect(supportedTypes).To(Equal(tc.expectedSupported))

				// Verify cache was updated correctly (if not error case)
				if !tc.expectError && tc.mockSetup != nil {
					sysCache.mu.RLock()
					entry, exists := sysCache.zonesMap[tc.zone]
					sysCache.mu.RUnlock()
					g.Expect(exists).To(BeTrue())
					g.Expect(entry.supportedTypes).To(Equal(tc.expectedSupported))
				}
			}
		})
	}
}
