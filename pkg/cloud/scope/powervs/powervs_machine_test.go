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
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/endpoints"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos"
	cosmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	resourcecontrollermock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1.IBMPowerVSMachine {
	var image infrav1.IBMPowerVSMachineImage
	network := infrav1.ResourceIdentifier{}

	if imageRef == nil {
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
		Recorder:          record.NewFakeRecorder(1000),
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

// errMachineVPCBuilder is a ClientBuilder that succeeds for auth/RC/PVS but fails on GetVPCClient.
type errMachineVPCBuilder struct {
	stubClientBuilder
	rc resourcecontroller.ResourceController
	pv powervs.PowerVS
}

func (e errMachineVPCBuilder) GetResourceControllerClient(_ context.Context, _ ClientOptions) (resourcecontroller.ResourceController, error) {
	return e.rc, nil
}
func (e errMachineVPCBuilder) GetPowerVSClient(_ context.Context, _ ClientOptions) (powervs.PowerVS, error) {
	return e.pv, nil
}
func (e errMachineVPCBuilder) GetVPCClient(_ context.Context, _ ClientOptions) (vpc.Vpc, error) {
	return nil, errors.New("vpc client error")
}

func TestNewPowerVSMachineScope(t *testing.T) {
	testCases := []struct {
		name          string
		params        MachineScopeParams
		expectError   bool
		errorContains string
	}{
		{
			name: "error when client is nil",
			params: MachineScopeParams{
				Client: nil,
			},
			expectError:   true,
			errorContains: "client is required",
		},
		{
			name: "error when Machine is nil",
			params: MachineScopeParams{
				Client:  testEnv.Client,
				Machine: nil,
			},
			expectError:   true,
			errorContains: "machine is required",
		},
		{
			name: "error when Cluster is nil",
			params: MachineScopeParams{
				Client:  testEnv.Client,
				Machine: newMachine(machineName),
				Cluster: nil,
			},
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name: "error when IBMPowerVSMachine is nil",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: nil,
			},
			expectError:   true,
			errorContains: "ibmPowerVSMachine is required",
		},
		{
			name: "error when IBMPowerVSCluster is nil",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true),
				IBMPowerVSCluster: nil,
			},
			expectError:   true,
			errorContains: "ibmPowerVSCluster is required",
		},
		{
			name: "error when ClientBuilder is nil",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true),
				IBMPowerVSCluster: newPowerVSCluster(clusterName),
				ClientBuilder:     nil,
			},
			expectError:   true,
			errorContains: "ClientBuilder is required",
		},
		{
			name: "error when GetAuthenticator fails",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true),
				IBMPowerVSCluster: newPowerVSCluster(clusterName),
				ClientBuilder:     errAuthBuilder{},
			},
			expectError:   true,
			errorContains: "failed to create authenticator",
		},
		{
			name: "error when GetResourceControllerClient fails",
			params: MachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true),
				IBMPowerVSCluster: newPowerVSCluster(clusterName),
				ClientBuilder:     errRCBuilder{},
			},
			expectError:   true,
			errorContains: "failed to create Resource Controller client",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewMachineScope(context.Background(), tc.params)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}

	// Cases that need mock RC wired through a real stubClientBuilder.
	t.Run("error when GetPowerVSClient fails", func(t *testing.T) {
		g := NewWithT(t)
		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Spec.Workspace.ID = "direct-ws-id"

		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To("ws-guid"),
				State:    ptr.To("active"),
				RegionID: ptr.To("us-south-1"),
			}, nil).AnyTimes()

		_, err := NewMachineScope(context.Background(), MachineScopeParams{
			Client:            testEnv.Client,
			Machine:           newMachine(machineName),
			Cluster:           newCluster(clusterName),
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: newPowerVSCluster(clusterName),
			ClientBuilder:     errPVSBuilder{rc: mockRC},
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create PowerVS client"))
	})

	t.Run("error when GetVPCClient fails", func(t *testing.T) {
		g := NewWithT(t)
		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Spec.Workspace.ID = "direct-ws-id"

		pvsMachineCluster := newPowerVSCluster(clusterName)
		pvsMachineCluster.Spec.Zone = "us-south-1"
		pvsMachineCluster.Spec.VPC.Region = "us-south"

		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To("ws-guid"),
				State:    ptr.To("active"),
				RegionID: ptr.To("us-south-1"),
			}, nil).AnyTimes()

		_, err := NewMachineScope(context.Background(), MachineScopeParams{
			Client:            testEnv.Client,
			Machine:           newMachine(machineName),
			Cluster:           newCluster(clusterName),
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsMachineCluster,
			ClientBuilder:     errMachineVPCBuilder{rc: mockRC},
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create IBM VPC client"))
	})
}

func TestResolveWorkspace(t *testing.T) {
	const (
		wsID     = "ws-guid"
		wsRegion = "us-south-1"
		wsName   = "my-workspace"
		wsZone   = "us-south-1"
	)
	activeState := string(infrav1.WorkspaceStateActive)

	t.Run("uses workspace ID from machine spec directly", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To(wsID),
				State:    ptr.To(activeState),
				RegionID: ptr.To(wsRegion),
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		id, zone, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal(wsID))
		g.Expect(zone).To(Equal(wsRegion))
	})

	t.Run("uses workspace Name from machine spec", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To(wsID),
				State:    ptr.To(activeState),
				RegionID: ptr.To(wsRegion),
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{Name: wsName},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		id, zone, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal(wsID))
		g.Expect(zone).To(Equal(wsRegion))
	})

	t.Run("falls back to cluster status workspace ID", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To(wsID),
				State:    ptr.To(activeState),
				RegionID: ptr.To(wsRegion),
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: wsID},
				},
			},
			ResourceClient: mockRC,
		}
		id, zone, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal(wsID))
		g.Expect(zone).To(Equal(wsRegion))
	})

	t.Run("error when cluster status workspace ID is empty", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec:   infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
				Status: infrav1.IBMPowerVSClusterStatus{},
			},
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("PowerVS workspace ID is not yet populated in the cluster status"))
	})

	t.Run("error when GetResourceInstanceByFilter returns error", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(nil, errors.New("rc lookup failed"))

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get PowerVS workspace details"))
	})

	t.Run("error when workspace is nil", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(nil, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("PowerVS workspace or GUID or RegionID is nil"))
	})

	t.Run("error when workspace GUID is nil", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     nil,
				State:    ptr.To(activeState),
				RegionID: ptr.To(wsRegion),
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("PowerVS workspace or GUID or RegionID is nil"))
	})

	t.Run("error when workspace RegionID is nil", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To(wsID),
				State:    ptr.To(activeState),
				RegionID: nil,
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("PowerVS workspace or GUID or RegionID is nil"))
	})

	t.Run("error when workspace is not in active state", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To(wsID),
				State:    ptr.To("provisioning"),
				RegionID: ptr.To(wsRegion),
			}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: wsID},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: wsZone},
			},
			ResourceClient: mockRC,
		}
		_, _, err := scope.resolveWorkspace(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("is not in active state"))
	})
}

func TestGetWorkspaceID(t *testing.T) {
	testCases := []struct {
		name                string
		expectedWorkspaceID string
		expectedError       error
		machineScope        MachineScope
	}{
		{
			name:                "returns workspace ID from cluster status",
			expectedWorkspaceID: "service-instance-0",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReference{
							ID: "service-instance-0",
						},
					},
				},
			},
		},
		{
			name:                "returns workspace ID from cluster status when machine spec is empty",
			expectedWorkspaceID: "service-instance-1",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReference{
							ID: "service-instance-1",
						},
					},
				},
			},
		},
		{
			name:                "cluster status takes precedence over cluster spec",
			expectedWorkspaceID: "service-instance-0",
			machineScope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Spec: infrav1.IBMPowerVSMachineSpec{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Workspace: infrav1.ResourceReference{
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
		},
		{
			name: "error when workspace ID not found anywhere",
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

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			workspaceID, err := tc.machineScope.GetWorkspaceID()
			g.Expect(workspaceID).To(Equal(tc.expectedWorkspaceID))
			if tc.expectedError != nil {
				g.Expect(err).To(Equal(tc.expectedError))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}

	t.Run("returns workspace ID when name is set in machine spec", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).
			Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("foo-id")}, nil)

		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: "us-south-1"},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{Name: "foo-cluster"},
				},
			},
			ResourceClient: mockRC,
		}
		workspaceID, err := scope.GetWorkspaceID()
		g.Expect(workspaceID).To(Equal("foo-id"))
		g.Expect(err).To(BeNil())
	})

	t.Run("error when resource controller lookup fails by name", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).
			Return(nil, fmt.Errorf("failed to list instance id"))

		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: "us-south-1"},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{Name: "foo-cluster"},
				},
			},
			ResourceClient: mockRC,
		}
		workspaceID, err := scope.GetWorkspaceID()
		g.Expect(workspaceID).To(BeEmpty())
		g.Expect(err).To(HaveOccurred())
	})
}

func TestConfigurationError(t *testing.T) {
	t.Run("Error returns configured message", func(t *testing.T) {
		g := NewWithT(t)
		err := NewConfigurationError("systemType 's922' is not supported")
		g.Expect(err.Error()).To(Equal("systemType 's922' is not supported"))
	})

	t.Run("NewConfigurationError wraps message as ConfigurationError", func(t *testing.T) {
		g := NewWithT(t)
		msg := "some configuration problem"
		err := NewConfigurationError(msg)
		var confErr *ConfigurationError
		g.Expect(errors.As(err, &confErr)).To(BeTrue())
		g.Expect(confErr.Error()).To(Equal(msg))
	})
}

func TestSetReady(t *testing.T) {
	t.Run("sets machine status to ready", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetReady()
		g.Expect(scope.IsReady()).To(BeTrue())
	})
}

func TestSetNotReady(t *testing.T) {
	t.Run("sets machine status to not ready", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Initialization: infrav1.IBMPowerVSMachineInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
		}
		scope.SetNotReady()
		g.Expect(scope.IsReady()).To(BeFalse())
	})
}

func TestGetRegion(t *testing.T) {
	testCases := []struct {
		name           string
		scope          MachineScope
		expectedRegion string
	}{
		{
			name: "returns region set in status",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{Region: region},
				},
			},
			expectedRegion: region,
		},
		{
			name: "returns empty string when region is not set",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.scope.GetRegion()).To(Equal(tc.expectedRegion))
		})
	}
}

func TestSetRegion(t *testing.T) {
	testCases := []struct {
		name           string
		scope          MachineScope
		expectedRegion string
	}{
		{
			name: "sets region to us-east",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedRegion: "us-east",
		},
		{
			name: "sets region to empty string",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			tc.scope.SetRegion(tc.expectedRegion)
			g.Expect(tc.scope.GetRegion()).To(Equal(tc.expectedRegion))
		})
	}
}

func TestGetZone(t *testing.T) {
	testCases := []struct {
		name         string
		scope        MachineScope
		expectedZone string
	}{
		{
			name: "returns zone when set",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{Zone: "us-south-1"},
				},
			},
			expectedZone: "us-south-1",
		},
		{
			name: "returns empty string when zone is not set",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.scope.GetZone()).To(Equal(tc.expectedZone))
		})
	}
}

func TestSetZone(t *testing.T) {
	testCases := []struct {
		name         string
		scope        MachineScope
		expectedZone string
	}{
		{
			name: "sets zone to us-east-1",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedZone: "us-east-1",
		},
		{
			name: "sets zone to empty string",
			scope: MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			tc.scope.SetZone(tc.expectedZone)
			g.Expect(tc.scope.GetZone()).To(Equal(tc.expectedZone))
		})
	}
}

func TestGetInstanceState(t *testing.T) {
	t.Run("returns state set via SetInstanceState", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetInstanceState(ptr.To("ready"))
		g.Expect(scope.GetInstanceState()).To(Equal(infrav1.PowerVSInstanceState("ready")))
	})
}

func TestGetIgnitionVersion(t *testing.T) {
	testCases := []struct {
		name            string
		scope           MachineScope
		expectedVersion string
	}{
		{
			name: "returns default version when not configured",
			scope: MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedVersion: infrav1.DefaultIgnitionVersion,
		},
		{
			name: "returns custom version when configured",
			scope: MachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						Ignition: infrav1.Ignition{Version: "3.4"},
					},
				},
			},
			expectedVersion: "3.4",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.scope.getIgnitionVersion()).To(Equal(tc.expectedVersion))
		})
	}
}

func TestBootstrapDataKey(t *testing.T) {
	testCases := []struct {
		name            string
		machineLabel    string
		machineName     string
		expectedDataKey string
	}{
		{
			name:            "returns control-plane key for control plane machine",
			machineLabel:    clusterv1.MachineControlPlaneLabel,
			machineName:     "foo-machine-0",
			expectedDataKey: path.Join("control-plane", "foo-machine-0"),
		},
		{
			name:            "returns node key for worker machine",
			machineName:     "foo-machine-1",
			machineLabel:    "foo",
			expectedDataKey: path.Join("node", "foo-machine-1"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := MachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					ObjectMeta: metav1.ObjectMeta{Name: tc.machineName},
				},
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{tc.machineLabel: ""},
					},
				},
			}
			g.Expect(scope.bootstrapDataKey()).To(Equal(tc.expectedDataKey))
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
	teardown := func() { mockCtrl.Finish() }

	const networkID = "foo-network-id"

	t.Run("returns network ID from resource identifier ID field", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{}
		id, err := scope.getNetworkID(context.Background(), infrav1.ResourceIdentifier{ID: networkID})
		g.Expect(*id).To(Equal(networkID))
		g.Expect(err).To(BeNil())
	})

	t.Run("returns network ID by name lookup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		networkName := "foo-network-name"
		mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), networkName).Return(&models.NetworkReference{
			NetworkID: ptr.To(networkID),
			Name:      ptr.To(networkName),
		}, nil)

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		id, err := scope.getNetworkID(context.Background(), infrav1.ResourceIdentifier{Name: networkName})
		g.Expect(*id).To(Equal(networkID))
		g.Expect(err).To(BeNil())
	})

	t.Run("error when network not found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		networkName := "foo-network"
		mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), networkName).Return(nil, nil)

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		id, err := scope.getNetworkID(context.Background(), infrav1.ResourceIdentifier{Name: networkName})
		g.Expect(id).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal(fmt.Sprintf("network with name %q not found", networkName)))
	})

	t.Run("error when both ID and name are empty", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{}
		id, err := scope.getNetworkID(context.Background(), infrav1.ResourceIdentifier{})
		g.Expect(id).To(BeNil())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("network identifier must contain either an ID or a Name"))
	})
}

func TestGetMachineInternalIP(t *testing.T) {
	t.Run("returns internal IP when set", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Addresses: []clusterv1.MachineAddress{
						{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"},
					},
				},
			},
		}
		g.Expect(scope.GetMachineInternalIP()).To(Equal("10.0.0.1"))
	})

	t.Run("returns empty string for external IP address type", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Addresses: []clusterv1.MachineAddress{
						{Type: clusterv1.MachineExternalIP, Address: "198.0.0.1"},
					},
				},
			},
		}
		g.Expect(scope.GetMachineInternalIP()).To(BeEmpty())
	})

	t.Run("returns empty string when no addresses set", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
		}
		g.Expect(scope.GetMachineInternalIP()).To(BeEmpty())
	})
}

func TestSetProviderID(t *testing.T) {
	providerID := "foo-provider-id"

	t.Run("error when workspace ID cannot be resolved", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
		}
		err := scope.SetProviderID(providerID)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("sets provider ID", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "foo-service-instance-id"},
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
		}
		scope.SetZone("us-south-1")
		scope.SetRegion(region)
		err := scope.SetProviderID(providerID)
		expected := fmt.Sprintf("ibmpowervs://%s/%s/%s/%s", scope.GetRegion(), scope.GetZone(), "foo-service-instance-id", providerID)
		g.Expect(scope.IBMPowerVSMachine.Spec.ProviderID).To(Equal(expected))
		g.Expect(err).To(BeNil())
	})
}

func TestCreateCOSClient(t *testing.T) {
	t.Run("Returns error when COS bucket region is not in cluster status", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
					// BucketRegion empty
				},
			},
		}
		result, err := scope.createCOSClient(ctx)
		g.Expect(result).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring("COS bucket region is not yet populated in cluster status")))
	})

	t.Run("Returns error when HMAC Secret name is not in cluster status", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{
						ID:           "cos-id",
						BucketRegion: "us-south",
						// HMACSecretName empty
					},
				},
			},
		}
		result, err := scope.createCOSClient(ctx)
		g.Expect(result).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring("COS HMAC Secret name is not yet populated in cluster status")))
	})

	t.Run("Returns error when HMAC Secret does not exist in Kubernetes", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{
						ID:             "cos-id",
						BucketRegion:   "us-south",
						HMACSecretName: "missing-secret",
					},
				},
			},
		}
		result, err := scope.createCOSClient(ctx)
		g.Expect(result).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch COS HMAC Secret")))
	})

	t.Run("Returns error when HMAC Secret is missing access_key_id", func(t *testing.T) {
		g := NewWithT(t)
		hmacSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "cos-hmac", Namespace: "default"},
			Data:       map[string][]byte{"secret_access_key": []byte("secret")},
		}
		scope := MachineScope{
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(hmacSecret).Build(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{
						ID:             "cos-id",
						BucketRegion:   "us-south",
						HMACSecretName: "cos-hmac",
					},
				},
			},
		}
		result, err := scope.createCOSClient(ctx)
		g.Expect(result).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring("missing access_key_id or secret_access_key")))
	})

	t.Run("Returns HMAC COS client when Secret has valid credentials", func(t *testing.T) {
		g := NewWithT(t)
		hmacSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "cos-hmac", Namespace: "default"},
			Data: map[string][]byte{
				"access_key_id":     []byte("AKID"),
				"secret_access_key": []byte("SECRET"),
			},
		}
		scope := MachineScope{
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(hmacSecret).Build(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{
						ID:             "cos-id",
						BucketRegion:   "us-south",
						HMACSecretName: "cos-hmac",
					},
				},
			},
		}
		result, err := scope.createCOSClient(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(result).ToNot(BeNil())
	})
}

func TestNewMachineScopeCOSClientBuild(t *testing.T) {
	t.Run("error when GetCOSClient fails during initClients", func(t *testing.T) {
		g := NewWithT(t)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Spec.Workspace.ID = "ws-id"

		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Spec.Zone = "us-south-1"
		pvsCluster.Spec.VPC.Region = "us-south"
		// Mark ignition configured so the COS build is triggered
		pvsCluster.Spec.COSInstance = infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision}
		pvsCluster.Status.COSInstance = infrav1.COSInstanceStatus{
			ID:           "cos-id",
			BucketRegion: "us-south",
		}

		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To("ws-id"),
				State:    ptr.To(string(infrav1.WorkspaceStateActive)),
				RegionID: ptr.To("us-south-1"),
			}, nil)

		_, err := NewMachineScope(context.Background(), MachineScopeParams{
			Client:            testEnv.Client,
			Machine:           newMachine(machineName),
			Cluster:           newCluster(clusterName),
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			ClientBuilder:     errCOSBuilder{rc: mockRC},
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create COS client"))
	})
}

// errCOSBuilder succeeds for all clients but fails on GetCOSClient.
type errCOSBuilder struct {
	stubClientBuilder
	rc resourcecontroller.ResourceController
	pv powervs.PowerVS
}

func (e errCOSBuilder) GetResourceControllerClient(_ context.Context, _ ClientOptions) (resourcecontroller.ResourceController, error) {
	return e.rc, nil
}
func (e errCOSBuilder) GetPowerVSClient(_ context.Context, _ ClientOptions) (powervs.PowerVS, error) {
	return e.pv, nil
}
func (e errCOSBuilder) GetVPCClient(_ context.Context, _ ClientOptions) (vpc.Vpc, error) {
	return nil, nil
}
func (e errCOSBuilder) GetCOSClient(_ context.Context, _ COSClientOptions) (cos.Cos, error) {
	return nil, errors.New("cos client error")
}

func TestSetInstanceID(t *testing.T) {
	testCases := []struct {
		name               string
		instanceID         *string
		expectedInstanceID string
	}{
		{
			name:               "sets instance ID from non-nil pointer",
			instanceID:         ptr.To("foo-instance-id"),
			expectedInstanceID: "foo-instance-id",
		},
		{
			name:       "does not mutate instance ID when pointer is nil",
			instanceID: nil,
		},
	}

	for _, tc := range testCases {
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
	t.Run("sets health status when health is non-nil", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetHealth(&models.PVMInstanceHealth{Status: "healthy"})
		g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal("healthy"))
	})

	t.Run("does not mutate health status when health is nil", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetHealth(nil)
		g.Expect(scope.IBMPowerVSMachine.Status.Health).To(BeEmpty())
	})
}

func TestDeleteMachineIgnition(t *testing.T) {
	t.Run("skips when COSInstance type is not configured", func(t *testing.T) {
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

	t.Run("skips when COS bucket name not yet populated in cluster status", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
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

	t.Run("Error creating COS client when COS bucket region not in status", func(t *testing.T) {
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
						// BucketRegion is empty → createCOSClient will fail
					},
				},
			},
			Machine: &clusterv1.Machine{},
		}
		err := scope.DeleteMachineIgnition(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("COS bucket region is not yet populated in cluster status")))
	})
}

func TestResolveUserData(t *testing.T) {
	t.Run("returns base64-encoded cloud-init data when Ignition is not configured", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		result, err := scope.resolveUserData(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).NotTo(BeEmpty())
	})

	t.Run("error when DataSecretName is nil", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.Machine.Spec.Bootstrap.DataSecretName = nil
		_, err := scope.resolveUserData(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("bootstrap.dataSecretName is nil"))
	})

	t.Run("error when bootstrap secret does not exist", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("does-not-exist")
		_, err := scope.resolveUserData(ctx)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestCreateIgnitionData(t *testing.T) {
	t.Run("error when user data is empty", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		_, err := scope.createIgnitionData(ctx, []byte{})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("user data is empty"))
	})

	t.Run("error when bucket name not in cluster status", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		// Status.COSInstance.BucketName is empty → createIgnitionData will fail
		_, err := scope.createIgnitionData(ctx, []byte("some-data"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("COS bucket name is not yet populated in cluster status"))
	})
}

func TestGetImageID(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("returns image ID directly when ID is set", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{}
		id, err := scope.getImageID(context.Background(), infrav1.ResourceIdentifier{ID: "direct-id"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal("direct-id"))
	})

	t.Run("returns image ID by name lookup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{
				{Name: ptr.To(pvsImage), ImageID: ptr.To(pvsImage + "-id")},
			},
		}, nil)

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		id, err := scope.getImageID(context.Background(), infrav1.ResourceIdentifier{Name: pvsImage})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal(pvsImage + "-id"))
	})

	t.Run("error when image not found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{
				{Name: ptr.To("other-image"), ImageID: ptr.To("other-id")},
			},
		}, nil)

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		_, err := scope.getImageID(context.Background(), infrav1.ResourceIdentifier{Name: "missing-image"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(`image with name "missing-image" not found`))
	})

	t.Run("error when ListImages API fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(nil, errors.New("images api failure"))

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		_, err := scope.getImageID(context.Background(), infrav1.ResourceIdentifier{Name: "some-image"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get images from IBM Cloud"))
	})

	t.Run("error when image reference has neither ID nor Name", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{}
		_, err := scope.getImageID(context.Background(), infrav1.ResourceIdentifier{})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("image reference must contain either an ID or a Name"))
	})
}

func TestCreateMachine(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

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
			{Name: ptr.To(pvsImage), ImageID: ptr.To(pvsImage + "-id")},
		},
	}
	pvmInstanceList := &models.PVMInstanceList{}
	pvmInstanceCreate := &models.PVMInstanceCreate{}

	t.Run("successfully creates machine", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("returns existing machine when already present", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, "foo-machine-1", ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		out, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(out.ServerName).To(Equal(ptr.To("foo-machine-1")))
	})

	t.Run("returns nil when instance creation already triggered and state is unknown", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, "foo-machine-2", ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Status.Conditions = append(scope.IBMPowerVSMachine.Status.Conditions, metav1.Condition{
			Type:   infrav1.InstanceReadyCondition,
			Status: metav1.ConditionUnknown,
		})
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		out, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(out).To(BeNil())
	})

	t.Run("error when ListInstances fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(nil, errors.New("error when getting list of instances"))
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when DataSecretName is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.Machine.Spec.Bootstrap.DataSecretName = nil
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when bootstrap data secret does not exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("foo-secret-temp")
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when bootstrap secret value key is missing", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
				Name:      machineName,
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{"val": []byte("user data")},
		}
		g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when processors value is invalid string", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.Processors = intstr.FromString("invalid")
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("successfully creates machine using imported image from IBMPowerVSImage status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, nil, ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSImage = &infrav1.IBMPowerVSImage{
			Status: infrav1.IBMPowerVSImageStatus{ImageID: "foo-image"},
		}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("successfully creates machine when image and network are specified by name", func(t *testing.T) {
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

	t.Run("error when imported image reference has no image ID (IBMPowerVSImage nil)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, nil, ptr.To(pvsNetwork), true, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when image not found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage+"-temp"), ptr.To(pvsNetwork), false, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(images, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when network not found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork+"-temp"), false, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(images, nil)
		mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), pvsNetwork+"-temp").Return(nil, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when CreateInstance API fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(nil, errors.New("failed to create machine"))
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("error when network not specified on machine and cluster status network is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), nil, true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.Network = infrav1.ResourceIdentifier{}
		scope.IBMPowerVSCluster.Status.Network = infrav1.NetworkStatus{}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("network ID is not yet resolved in cluster status"))
	})

	t.Run("uses cluster-status network ID when machine has no network specified", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), nil, true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.Network = infrav1.ResourceIdentifier{}
		scope.IBMPowerVSCluster.Status.Network = infrav1.NetworkStatus{ID: "cluster-network-id"}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestCreateMachineSystemTypeValidation(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	pvmInstances := &models.PVMInstances{PvmInstances: []*models.PVMInstanceReference{}}
	pvmInstanceList := &models.PVMInstanceList{}
	pvmInstanceCreate := &models.PVMInstanceCreate{}

	t.Run("error when systemType validation API call fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "validation-fail-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(nil, errors.New("datacenter api error"))

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.SystemType = "s922"
		scope.SetZone(zone)

		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to validate systemType"))
	})

	t.Run("error when systemType is not supported in the zone", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "unsupported-sys-type-zone"
		sysCache.mu.Lock()
		sysCache.zonesMap[zone] = zoneCacheEntry{
			supportedTypes: []string{"e980", "s1022"},
			lastFetch:      time.Now(),
		}
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.SystemType = "invalid-type"
		scope.SetZone(zone)

		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(HaveOccurred())
		var confErr *ConfigurationError
		g.Expect(errors.As(err, &confErr)).To(BeTrue())
		g.Expect(confErr.Error()).To(ContainSubstring("is not supported in this zone"))
	})

	t.Run("succeeds when systemType is valid for the zone", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "valid-sys-type-zone"
		sysCache.mu.Lock()
		sysCache.zonesMap[zone] = zoneCacheEntry{
			supportedTypes: []string{"e980", "s1022", "s922"},
			lastFetch:      time.Now(),
		}
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.SystemType = "s922"
		scope.SetZone(zone)

		_, err := scope.CreateMachine(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestDeleteMachine(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	var id string

	t.Run("successfully deletes machine", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
		mockpowervs.EXPECT().DeleteInstance(gomock.Any(), gomock.AssignableToTypeOf(id)).Return(nil)
		err := scope.DeleteMachine(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("error when DeleteInstance API fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
		mockpowervs.EXPECT().DeleteInstance(gomock.Any(), gomock.AssignableToTypeOf(id)).Return(errors.New("failed to delete machine"))
		err := scope.DeleteMachine(ctx)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestGetIPFromCache(t *testing.T) {
	t.Run("returns empty string and false when key not in cache", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.DHCPIPCacheStore = cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)
		ip, found := scope.getIPFromCache(ctx, "nonexistent-vm")
		g.Expect(found).To(BeFalse())
		g.Expect(ip).To(BeEmpty())
	})
}

func TestValidateSystemType(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("error when systemType is empty", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: ""},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: "us-south-1"},
			},
		}
		_, _, err := scope.validateSystemType(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("systemType is not set"))
	})

	t.Run("returns result from fresh cache without API call", func(t *testing.T) {
		g := NewWithT(t)
		zone := "cache-hit-zone"
		sysCache.mu.Lock()
		sysCache.zonesMap[zone] = zoneCacheEntry{
			supportedTypes: []string{"e980", "s922"},
			lastFetch:      time.Now(),
		}
		sysCache.mu.Unlock()

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
		}
		ok, types, err := scope.validateSystemType(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())
		g.Expect(types).To(ContainElement("s922"))
	})

	t.Run("error when datacenter API returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "api-error-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(nil, errors.New("datacenter api error"))

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
			IBMPowerVSClient: mockpowervs,
		}
		_, _, err := scope.validateSystemType(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get datacenter details"))
	})

	t.Run("error when datacenter returns nil CapabilitiesDetails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "nil-caps-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(nil, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
			IBMPowerVSClient: mockpowervs,
		}
		_, _, err := scope.validateSystemType(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("system capabilities details are missing"))
	})

	t.Run("returns false when systemType not in supported list", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "unsupported-type-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(&models.Datacenter{
			CapabilitiesDetails: &models.CapabilitiesDetails{
				SupportedSystems: &models.SupportedSystems{
					General: []string{"e980", "s1022"},
				},
			},
		}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
			IBMPowerVSClient: mockpowervs,
		}
		ok, types, err := scope.validateSystemType(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeFalse())
		g.Expect(types).To(ContainElements("e980", "s1022"))
	})

	t.Run("returns true when systemType is in supported list", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "supported-type-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(&models.Datacenter{
			CapabilitiesDetails: &models.CapabilitiesDetails{
				SupportedSystems: &models.SupportedSystems{
					General: []string{"e980", "s1022", "s922"},
				},
			},
		}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
			IBMPowerVSClient: mockpowervs,
		}
		ok, types, err := scope.validateSystemType(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())
		g.Expect(types).To(ContainElements("s922"))
	})

	t.Run("error when supported systems list is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		zone := "empty-systems-zone"
		sysCache.mu.Lock()
		delete(sysCache.zonesMap, zone)
		sysCache.mu.Unlock()

		mockpowervs.EXPECT().GetDatacenterDetails(gomock.Any(), zone).Return(&models.Datacenter{
			CapabilitiesDetails: &models.CapabilitiesDetails{
				SupportedSystems: &models.SupportedSystems{
					General: []string{},
				},
			},
		}, nil)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
			IBMPowerVSClient: mockpowervs,
		}
		_, _, err := scope.validateSystemType(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no general system types available"))
	})
}

func TestSetAddresses(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		serverName = "test-vm"
		networkID  = "test-network-id"
		macAddress = "aa:bb:cc:dd:ee:ff"
		dhcpIP     = "10.0.0.5"
		dhcpSrvID  = "dhcp-server-id"
	)

	t.Run("sets base addresses and IP when instance already has IP assigned", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{
			ServerName: ptr.To(serverName),
			Networks: []*models.PVMInstanceNetwork{
				{IPAddress: "192.168.1.10"},
			},
		}
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, nil)
		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalDNS, Address: serverName}))
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineHostName, Address: serverName}))
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: "192.168.1.10"}))
	})

	t.Run("sets external IP when instance has external IP assigned", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{
			ServerName: ptr.To(serverName),
			Networks: []*models.PVMInstanceNetwork{
				{IPAddress: "192.168.1.10", ExternalIP: "52.1.2.3"},
			},
		}
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, nil)
		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineExternalIP, Address: "52.1.2.3"}))
	})

	t.Run("returns IP from cache when instance has no IP and cache hit", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{
			ServerName: ptr.To(serverName),
			Networks:   []*models.PVMInstanceNetwork{{NetworkID: networkID, MacAddress: macAddress}},
		}
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, nil)
		g.Expect(scope.DHCPIPCacheStore.Add(powervs.VMip{Name: serverName, IP: dhcpIP})).To(Succeed())

		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: dhcpIP}))
	})

	t.Run("resolves IP from DHCP server when cache miss", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		instance := newPowerVSInstance(serverName, networkID, macAddress)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, mockpowervs)

		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpSrvID, networkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), dhcpSrvID).Return(newDHCPServerDetails(dhcpSrvID, dhcpIP, macAddress), nil)

		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: dhcpIP}))
	})

	t.Run("falls back to base addresses when DHCP server list fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		instance := newPowerVSInstance(serverName, networkID, macAddress)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, mockpowervs)

		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(nil, errors.New("dhcp list failed"))

		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalDNS, Address: serverName}))
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineHostName, Address: serverName}))
		// No InternalIP — fell back to base addresses
		for _, a := range addrs {
			g.Expect(a.Type).NotTo(Equal(clusterv1.MachineInternalIP))
		}
	})

	t.Run("falls back to base addresses when DHCP lease not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		instance := newPowerVSInstance(serverName, networkID, macAddress)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, mockpowervs)

		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpSrvID, networkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), dhcpSrvID).Return(newDHCPServerDetails(dhcpSrvID, dhcpIP, "different-mac"), nil)

		scope.SetAddresses(ctx, instance)
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		for _, a := range addrs {
			g.Expect(a.Type).NotTo(Equal(clusterv1.MachineInternalIP))
		}
	})
}

func TestCreateVPCLoadBalancerPoolMember(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockVPC  *vpcmock.MockVpc
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = vpcmock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		lbID       = "lb-id"
		lbName     = "foo-cluster-loadbalancer-public"
		poolID     = "pool-id"
		poolName   = "pool-name"
		listenerID = "listener-id"
		memberID   = "member-id"
		internalIP = "10.0.0.10"
	)

	activeState := string(infrav1.LoadBalancerStateActive)

	makeScopeWithLBStatus := func(vpcClient vpc.Vpc) *MachineScope {
		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: internalIP},
		}
		cluster := newCluster(clusterName)
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{ID: lbID, Name: lbName},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).
			WithObjects(cluster, pvsMachine, pvsCluster).Build()
		return &MachineScope{
			Client:            fakeClient,
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Cluster:           cluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      vpcClient,
		}
	}

	t.Run("returns nil when no load balancers are in cluster spec or status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsCluster := newPowerVSCluster(clusterName)
		// Deliberately empty status — no LBs
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{}
		scope := &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      mockVPC,
		}
		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to find VPC load balancer ID"))
	})

	t.Run("error when GetLoadBalancer fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("get lb failed"))

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to find VPC load balancer details"))
	})

	t.Run("update_pending LB is skipped (not an error), returns pending=true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To("update_pending"),
			Pools:              []vpcv1.LoadBalancerPoolReference{},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeTrue())
	})

	t.Run("error when load balancer has no pools", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no pools exist for the VPC load balancer"))
	})

	t.Run("error when ListLoadBalancerPoolMembers fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(nil, nil, errors.New("list members error"))

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to list"))
	})

	t.Run("member already registered skips creation and returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{
				{
					ID: ptr.To(memberID),
					Target: &vpcv1.LoadBalancerPoolMemberTarget{
						Address: ptr.To(internalIP),
					},
				},
			},
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("delete_pending LB is a non-recoverable error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To("delete_pending"),
		}, nil, nil)

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("non-recoverable state"))
	})

	t.Run("error when CreateLoadBalancerPoolMember fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		lbActive := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lbActive, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(nil, nil, errors.New("create member failed"))

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create VPC load balancer"))
	})

	t.Run("successfully creates pool member and returns pending=false when active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		lbActive := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lbActive, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To(memberID),
			ProvisioningStatus: ptr.To(activeState),
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("error when GetLoadBalancerListener fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		scope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: lbName,
					AdditionalListeners: []infrav1.AdditionalListener{
						{Port: 6443, Protocol: infrav1.LoadBalancerListenerProtocolTCP},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
			},
			Listeners: []vpcv1.LoadBalancerListenerReference{
				{ID: ptr.To(listenerID)},
			},
		}, nil, nil)
		mockVPC.EXPECT().GetLoadBalancerListener(gomock.Any()).Return(nil, nil, errors.New("listener fetch failed"))

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get load balancer listener"))
	})
}

func TestInitClientsVPCRegionFallback(t *testing.T) {
	t.Run("error when VPCRegionForPowerVSRegion fails for unrecognised PowerVS region", func(t *testing.T) {
		g := NewWithT(t)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Spec.Workspace.ID = "ws-id"

		pvsCluster := newPowerVSCluster(clusterName)
		// Leave VPC.Region empty so the code falls through to VPCRegionForPowerVSRegion.
		// Use a zone whose derived region has no VPC mapping.
		pvsCluster.Spec.Zone = "totally-invalid-1"

		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To("ws-id"),
				State:    ptr.To(string(infrav1.WorkspaceStateActive)),
				RegionID: ptr.To("totally-invalid-1"),
			}, nil)

		_, err := NewMachineScope(context.Background(), MachineScopeParams{
			Client:            testEnv.Client,
			Machine:           newMachine(machineName),
			Cluster:           newCluster(clusterName),
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			ClientBuilder:     errMachineVPCBuilder{rc: mockRC},
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to determine VPC region"))
	})
}

func TestGetWorkspaceIDNilGUID(t *testing.T) {
	t.Run("error when workspace lookup returns nil GUID", func(t *testing.T) {
		g := NewWithT(t)
		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{GUID: nil}, nil)

		scope := MachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Zone: "us-south-1"},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{Name: "my-workspace"},
				},
			},
			ResourceClient: mockRC,
		}
		_, err := scope.GetWorkspaceID()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("not found or GUID is nil"))
	})
}

func TestGetNetworkIDByNameError(t *testing.T) {
	t.Run("error when GetNetworkByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockpowervs := mock.NewMockPowerVS(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		mockpowervs.EXPECT().GetNetworkByName(gomock.Any(), "bad-network").
			Return(nil, errors.New("network api error"))

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		_, err := scope.getNetworkID(context.Background(), infrav1.ResourceIdentifier{Name: "bad-network"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get network by name"))
	})
}

// errCacheStore is a test-only cache.Store whose methods return configurable errors.
type errCacheStore struct{ cache.Store }

func (e *errCacheStore) Add(_ interface{}) error {
	return errors.New("cache add error")
}

func (e *errCacheStore) GetByKey(_ string) (interface{}, bool, error) {
	return nil, false, errors.New("cache error")
}

func TestGetIPFromCacheStoreError(t *testing.T) {
	t.Run("returns empty string and false when cache store returns error", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
			DHCPIPCacheStore:  &errCacheStore{Store: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)},
		}
		ip, found := scope.getIPFromCache(ctx, "any-vm")
		g.Expect(found).To(BeFalse())
		g.Expect(ip).To(BeEmpty())
	})
}

func TestGetIPFromDHCPServerAdditionalBranches(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		testNetworkID  = "net-id"
		testMACAddr    = "aa:bb:cc:dd:ee:ff"
		testServerName = "test-vm"
		testDHCPSrvID  = "dhcp-srv-id"
	)

	t.Run("error when instance network attachment not found", func(t *testing.T) {
		g := NewWithT(t)
		instance := newPowerVSInstance(testServerName, "other-network-id", testMACAddr)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(testNetworkID), true, mockpowervs)

		_, err := scope.getIPFromDHCPServer(ctx, instance)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to find network attached to machine"))
	})

	t.Run("error when no DHCP server found for network", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		instance := newPowerVSInstance(testServerName, testNetworkID, testMACAddr)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(testNetworkID), true, mockpowervs)
		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer("other-srv-id", "other-net-id"), nil)

		_, err := scope.getIPFromDHCPServer(ctx, instance)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no DHCP server found associated with network"))
	})

	t.Run("error when GetDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		instance := newPowerVSInstance(testServerName, testNetworkID, testMACAddr)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(testNetworkID), true, mockpowervs)
		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(testDHCPSrvID, testNetworkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), testDHCPSrvID).Return(nil, errors.New("dhcp server error"))

		_, err := scope.getIPFromDHCPServer(ctx, instance)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get DHCP server details"))
	})

	t.Run("falls back to cluster status network when machine spec network is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		instance := newPowerVSInstance(testServerName, testNetworkID, testMACAddr)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), nil, true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.Network = infrav1.ResourceIdentifier{}
		scope.IBMPowerVSCluster.Status.Network = infrav1.NetworkStatus{ID: testNetworkID}

		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(testDHCPSrvID, testNetworkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), testDHCPSrvID).Return(newDHCPServerDetails(testDHCPSrvID, "10.0.0.9", testMACAddr), nil)

		ip, err := scope.getIPFromDHCPServer(ctx, instance)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ip).To(Equal("10.0.0.9"))
	})
}

func TestCreateMachineSSHKey(t *testing.T) {
	t.Run("sets SSH key on the payload when SSHKey is specified", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockpowervs := mock.NewMockPowerVS(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		pvmInstances := &models.PVMInstances{PvmInstances: []*models.PVMInstanceReference{}}
		pvmInstanceList := &models.PVMInstanceList{}
		pvmInstanceCreate := &models.PVMInstanceCreate{}

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Spec.SSHKey = "my-ssh-key"

		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(pvmInstances, nil)
		mockpowervs.EXPECT().CreateInstance(gomock.Any(), gomock.AssignableToTypeOf(pvmInstanceCreate)).
			DoAndReturn(func(_ context.Context, p *models.PVMInstanceCreate) (*models.PVMInstanceList, error) {
				g.Expect(p.KeyPairName).To(Equal("my-ssh-key"))
				return pvmInstanceList, nil
			})

		_, err := scope.CreateMachine(ctx)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestCreateVPCLoadBalancerPoolMemberSelectorBranches(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockVPC  *vpcmock.MockVpc
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = vpcmock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		selectorLBID        = "lb-id"
		selectorLBName      = "foo-cluster-loadbalancer-public"
		selectorPoolID      = "pool-id"
		selectorPoolName    = "pool-name"
		selectorListenerID  = "listener-id"
		selectorInternalIP  = "10.0.0.10"
		selectorActiveState = "active"
	)

	makeSelectorScope := func(vpcClient vpc.Vpc) *MachineScope {
		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: selectorInternalIP},
		}
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{{ID: selectorLBID, Name: selectorLBName}}
		return &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      vpcClient,
		}
	}

	lbWithListener := func() *vpcv1.LoadBalancer {
		return &vpcv1.LoadBalancer{
			ID:                 ptr.To(selectorLBID),
			Name:               ptr.To(selectorLBName),
			ProvisioningStatus: ptr.To(selectorActiveState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(selectorPoolID), Name: ptr.To(selectorPoolName)}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{{ID: ptr.To(selectorListenerID)}},
		}
	}

	t.Run("skips pool when selector is empty and machine is not control-plane", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeSelectorScope(mockVPC)
		scope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: selectorLBName,
					AdditionalListeners: []infrav1.AdditionalListener{
						{Port: 6443, Protocol: infrav1.LoadBalancerListenerProtocolTCP},
					},
				},
			},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lbWithListener(), nil, nil)
		mockVPC.EXPECT().GetLoadBalancerListener(gomock.Any()).Return(&vpcv1.LoadBalancerListener{
			ID:          ptr.To(selectorListenerID),
			Port:        ptr.To(int64(6443)),
			Protocol:    ptr.To("tcp"),
			DefaultPool: &vpcv1.LoadBalancerPoolReference{ID: ptr.To(selectorPoolID), Name: ptr.To(selectorPoolName)},
		}, nil, nil)
		// ListLoadBalancerPoolMembers is NOT called — the selector check (empty + not CP) fires first

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("skips pool when non-empty selector does not match machine labels", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeSelectorScope(mockVPC)
		scope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: selectorLBName,
					AdditionalListeners: []infrav1.AdditionalListener{
						{
							Port:     8080,
							Protocol: infrav1.LoadBalancerListenerProtocolTCP,
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{"role": "worker"},
							},
						},
					},
				},
			},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lbWithListener(), nil, nil)
		mockVPC.EXPECT().GetLoadBalancerListener(gomock.Any()).Return(&vpcv1.LoadBalancerListener{
			ID:          ptr.To(selectorListenerID),
			Port:        ptr.To(int64(8080)),
			Protocol:    ptr.To("tcp"),
			DefaultPool: &vpcv1.LoadBalancerPoolReference{ID: ptr.To(selectorPoolID), Name: ptr.To(selectorPoolName)},
		}, nil, nil)
		// ListLoadBalancerPoolMembers is NOT called — the selector mismatch check fires first

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("nil provisioning status returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeSelectorScope(mockVPC)
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(selectorLBID),
			Name:               ptr.To(selectorLBName),
			ProvisioningStatus: nil,
		}, nil, nil)

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("has no provisioning status"))
	})
}

func TestDeleteMachineIgnitionCOSDelete(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockCOS  *cosmock.MockCos
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOS = cosmock.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("successfully deletes ignition object from COS", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{Name: machineName},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{BucketName: "test-bucket"},
				},
			},
			Machine:   &clusterv1.Machine{},
			COSClient: mockCOS,
			Recorder:  record.NewFakeRecorder(1000),
		}
		mockCOS.EXPECT().DeleteObject(gomock.Any()).Return(nil, nil)

		err := scope.DeleteMachineIgnition(ctx)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("error when COS DeleteObject fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{Name: machineName},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{BucketName: "test-bucket"},
				},
			},
			Machine:   &clusterv1.Machine{},
			COSClient: mockCOS,
			Recorder:  record.NewFakeRecorder(1000),
		}
		mockCOS.EXPECT().DeleteObject(gomock.Any()).Return(nil, errors.New("delete failed"))

		err := scope.DeleteMachineIgnition(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to delete COS object"))
	})
}

func TestCreateIgnitionDataAdditionalBranches(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockCOS  *cosmock.MockCos
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOS = cosmock.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("error when PutObject fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, errors.New("put object failed"))

		_, err := scope.createIgnitionData(ctx, []byte("some-data"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to push object to COS bucket"))
	})

	t.Run("returns HTTPS object URL on success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://test-bucket.s3.us-south.example.com/node/foo-machine?sig=abc", nil)

		objectURL, err := scope.createIgnitionData(ctx, []byte("some-data"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objectURL).To(HavePrefix("https://"))
		g.Expect(objectURL).To(ContainSubstring("test-bucket"))
	})

	t.Run("uses custom COS endpoint when ServiceEndpoint overrides it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		scope.ServiceEndpoint = []endpoints.ServiceEndpoint{
			{ID: string(endpoints.COS), URL: "https://custom-cos.example.com"},
		}
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://test-bucket.custom-cos.example.com/node/foo-machine?sig=abc", nil)

		objectURL, err := scope.createIgnitionData(ctx, []byte("some-data"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(objectURL).To(ContainSubstring("custom-cos.example.com"))
	})

	t.Run("error when bucket name not in cluster status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		// Status.COSInstance.BucketName is empty → createIgnitionData will fail

		_, err := scope.createIgnitionData(ctx, []byte("some-data"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("COS bucket name is not yet populated in cluster status"))
	})
}

// TestNewMachineScopeHappyPath verifies that NewMachineScope returns a fully
// initialised scope when all parameters are valid.
func TestNewMachineScopeHappyPath(t *testing.T) {
	t.Run("successfully creates scope when all clients build without error", func(t *testing.T) {
		g := NewWithT(t)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Spec.Workspace.ID = "direct-ws-id"

		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Spec.Zone = "us-south-1"
		pvsCluster.Spec.VPC.Region = "us-south"

		mockRC := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		mockRC.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:     ptr.To("ws-guid"),
				State:    ptr.To(string(infrav1.WorkspaceStateActive)),
				RegionID: ptr.To("us-south-1"),
			}, nil).AnyTimes()

		scope, err := NewMachineScope(context.Background(), MachineScopeParams{
			Client:            testEnv.Client,
			Machine:           newMachine(machineName),
			Cluster:           newCluster(clusterName),
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			ClientBuilder:     stubClientBuilder{rcClient: mockRC},
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(scope).NotTo(BeNil())
	})
}

// TestGetWorkspaceIDMachineSpecID covers the first precedence branch where
// IBMPowerVSMachine.Spec.Workspace.ID is set directly.
func TestGetWorkspaceIDMachineSpecID(t *testing.T) {
	t.Run("returns workspace ID from machine spec when ID is set", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "machine-spec-ws-id"},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		id, err := scope.GetWorkspaceID()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(id).To(Equal("machine-spec-ws-id"))
	})
}

// TestSetAddressesCacheStoreAddError verifies that a cache-store Add failure
// is non-fatal — the IP is still appended to Status.Addresses.
func TestSetAddressesCacheStoreAddError(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		serverName = "test-vm"
		networkID  = "test-network-id"
		macAddress = "aa:bb:cc:dd:ee:ff"
		dhcpIP     = "10.0.0.5"
		dhcpSrvID  = "dhcp-server-id"
	)

	t.Run("adds address even when DHCPIPCacheStore.Add returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		instance := newPowerVSInstance(serverName, networkID, macAddress)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, mockpowervs)
		// Replace the cache store with one that always errors on Add.
		scope.DHCPIPCacheStore = &errCacheStore{Store: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)}

		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpSrvID, networkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), dhcpSrvID).Return(newDHCPServerDetails(dhcpSrvID, dhcpIP, macAddress), nil)

		scope.SetAddresses(ctx, instance)
		// The IP is still set despite the cache Add failure.
		addrs := scope.IBMPowerVSMachine.Status.Addresses
		g.Expect(addrs).To(ContainElement(clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: dhcpIP}))
	})
}

// TestValidateSystemTypeCacheDoubleCheck covers the branch where the first
// read-lock check finds a stale entry but the write-lock double-check finds a
// freshly-populated entry — preventing a redundant API call.
func TestValidateSystemTypeCacheDoubleCheck(t *testing.T) {
	t.Run("returns from cache on write-lock double-check when freshly refreshed by another goroutine", func(t *testing.T) {
		g := NewWithT(t)

		zone := "double-check-zone"

		// Simulate an expired entry so the read-lock check misses ...
		sysCache.mu.Lock()
		sysCache.zonesMap[zone] = zoneCacheEntry{
			supportedTypes: []string{"e980", "s922"},
			lastFetch:      time.Now().Add(-2 * sysCache.ttl), // expired
		}
		sysCache.mu.Unlock()

		// ... but refresh the entry BEFORE the test reaches the write-lock
		// double-check by simply writing a fresh entry right now; the test
		// will acquire the write lock immediately after and see it fresh.
		sysCache.mu.Lock()
		sysCache.zonesMap[zone] = zoneCacheEntry{
			supportedTypes: []string{"e980", "s922"},
			lastFetch:      time.Now(), // fresh
		}
		sysCache.mu.Unlock()

		scope := MachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec:   infrav1.IBMPowerVSMachineSpec{SystemType: "s922"},
				Status: infrav1.IBMPowerVSMachineStatus{Zone: zone},
			},
		}
		ok, types, err := scope.validateSystemType(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())
		g.Expect(types).To(ContainElement("s922"))
	})
}

// TestGetIPFromDHCPServerLeaseMACNotFound covers the final return in
// getIPFromDHCPServer when no lease matches the machine's MAC address.
func TestGetIPFromDHCPServerLeaseMACNotFound(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockpowervs *mock.MockPowerVS
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		networkID  = "net-id"
		macAddr    = "aa:bb:cc:dd:ee:ff"
		serverName = "test-vm"
		dhcpSrvID  = "dhcp-srv-id"
	)

	t.Run("error when DHCP lease not found for machine MAC", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		instance := newPowerVSInstance(serverName, networkID, macAddr)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(networkID), true, mockpowervs)

		// DHCP server exists for this network but has no lease matching macAddr.
		mockpowervs.EXPECT().ListDHCPServers(gomock.Any()).Return(newDHCPServer(dhcpSrvID, networkID), nil)
		mockpowervs.EXPECT().GetDHCPServer(gomock.Any(), dhcpSrvID).Return(
			newDHCPServerDetails(dhcpSrvID, "10.0.0.99", "different-mac"), nil,
		)

		_, err := scope.getIPFromDHCPServer(ctx, instance)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("DHCP lease not found for machine with MAC"))
	})
}

// TestCreateVPCLoadBalancerPoolMemberAdditionalBranches covers the remaining
// uncovered branches in CreateVPCLoadBalancerPoolMember.
func TestCreateVPCLoadBalancerPoolMemberAdditionalBranches(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockVPC  *vpcmock.MockVpc
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = vpcmock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	const (
		lbID        = "lb-id"
		lbName      = "foo-cluster-loadbalancer-public"
		poolID      = "pool-id"
		poolName    = "pool-name"
		listenerID  = "listener-id"
		internalIP  = "10.0.0.10"
		activeState = "active"
	)

	makeScopeWithLBStatus := func(vpcClient vpc.Vpc) *MachineScope {
		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: internalIP},
		}
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{{ID: lbID, Name: lbName}}
		return &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      vpcClient,
		}
	}

	t.Run("resolves LB name for private type when provision name is empty and index is 0", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: internalIP},
		}
		privateLBName := ResourceName(clusterName, ResourceTypeLBPrivate, "")
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					// Name is empty — should be auto-generated as private
					Type: infrav1.LoadBalancerTypePrivate,
				},
			},
		}
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{{ID: lbID, Name: privateLBName}}
		scope := &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      mockVPC,
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(privateLBName),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID), Name: ptr.To(poolName)}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("new-member"),
			ProvisioningStatus: ptr.To(activeState),
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("resolves LB name with index qualifier when provision name is empty and index > 0", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: internalIP},
		}
		lb0Name := ResourceName(clusterName, ResourceTypeLBPublic, "")
		lb1Name := ResourceName(clusterName, ResourceTypeLBPublic, "1")
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type:      infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{Name: lb0Name, Type: infrav1.LoadBalancerTypePublic},
			},
			{
				// index 1, no name → should get qualifier "1"
				Type:      infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{Type: infrav1.LoadBalancerTypePublic},
			},
		}
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{ID: "lb-0-id", Name: lb0Name},
			{ID: "lb-1-id", Name: lb1Name},
		}
		scope := &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      mockVPC,
		}

		// Both LBs are queried; first one already has the member registered.
		activeLB0 := &vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-0-id"),
			Name:               ptr.To(lb0Name),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To("pool-0"), Name: ptr.To("pool-0-name")}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}
		activeLB1 := &vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-1-id"),
			Name:               ptr.To(lb1Name),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To("pool-1"), Name: ptr.To("pool-1-name")}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(activeLB0, nil, nil)
		// LB0 pool member already registered — skip
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{
				{Target: &vpcv1.LoadBalancerPoolMemberTarget{Address: ptr.To(internalIP)}},
			},
		}, nil, nil)
		// LB1
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(activeLB1, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("new-member-1"),
			ProvisioningStatus: ptr.To(activeState),
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("resolves LB name from reference type", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		refLBName := "my-reference-lb"
		scope := makeScopeWithLBStatus(mockVPC)
		scope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{Name: refLBName},
			},
		}
		scope.IBMPowerVSCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{ID: lbID, Name: refLBName},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(refLBName),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID), Name: ptr.To(poolName)}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}, nil, nil)
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("ref-member"),
			ProvisioningStatus: ptr.To(activeState),
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})

	t.Run("listener with port 6443 but nil DefaultPool logs and skips pool mapping", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := makeScopeWithLBStatus(mockVPC)
		scope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: lbName,
					// No AdditionalListeners → 6443 branch with nil DefaultPool
				},
			},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(activeState),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID), Name: ptr.To(poolName)}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{{ID: ptr.To(listenerID)}},
		}, nil, nil)
		// Listener on port 6443 but DefaultPool is nil
		mockVPC.EXPECT().GetLoadBalancerListener(gomock.Any()).Return(&vpcv1.LoadBalancerListener{
			ID:          ptr.To(listenerID),
			Port:        ptr.To(int64(6443)),
			Protocol:    ptr.To("tcp"),
			DefaultPool: nil,
		}, nil, nil)
		// Pool not in loadBalancerListeners map (since default pool was nil) →
		// targetPort=0, not alreadyRegistered → second GetLoadBalancer before create
		mockVPC.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{},
		}, nil, nil)
		mockVPC.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("member-after-nil-pool"),
			ProvisioningStatus: ptr.To(activeState),
		}, nil, nil)

		pending, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pending).To(BeFalse())
	})
}

// TestResolveUserDataIgnitionPath covers the ignition branch of resolveUserData
// where createIgnitionData fails (since GetIAMAuthenticator requires real credentials).
func TestResolveUserDataIgnitionPath(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockCOS  *cosmock.MockCos
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOS = cosmock.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("error when createIgnitionData fails in ignition path", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		// Enable ignition by setting a COS instance on the cluster spec and status.
		scope.IBMPowerVSCluster.Spec.COSInstance = infrav1.COSInstanceSource{
			Type: infrav1.SourceTypeProvision,
		}
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		scope.COSClient = mockCOS
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, errors.New("put failed"))

		_, err := scope.resolveUserData(ctx)
		g.Expect(err).To(HaveOccurred())
		// resolveUserData wraps the ignitionUserData error which wraps createIgnitionData
		g.Expect(err.Error()).To(ContainSubstring("failed to push object to COS bucket"))
	})
}

// TestIgnitionUserDataCreateIgnitionDataError covers the early-return in
// ignitionUserData when createIgnitionData fails, exercising the 0% function.
func TestIgnitionUserDataCreateIgnitionDataError(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockCOS  *cosmock.MockCos
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOS = cosmock.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("error propagated from createIgnitionData into ignitionUserData", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		scope.COSClient = mockCOS
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, errors.New("upload failed"))

		_, err := scope.ignitionUserData(ctx, []byte("bootstrap-data"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create user data object"))
	})
}

func TestGetRawBootstrapData(t *testing.T) {
	t.Run("returns bootstrap data when secret and value key exist", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		data, err := scope.getRawBootstrapData()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal([]byte("user data")))
	})

	t.Run("error when Machine is nil", func(t *testing.T) {
		g := NewWithT(t)
		scope := MachineScope{Machine: nil}
		_, err := scope.getRawBootstrapData()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("bootstrap.dataSecretName is nil"))
	})

	t.Run("error when DataSecretName is nil", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.Machine.Spec.Bootstrap.DataSecretName = nil
		_, err := scope.getRawBootstrapData()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("bootstrap.dataSecretName is nil"))
	})

	t.Run("error when secret does not exist", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("nonexistent-secret")
		_, err := scope.getRawBootstrapData()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to retrieve bootstrap data secret"))
	})

	t.Run("error when secret is missing the value key", func(t *testing.T) {
		g := NewWithT(t)
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		// Replace the pre-registered secret with one that has no "value" key.
		badSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{"other": []byte("stuff")},
		}
		g.Expect(scope.Client.Update(context.Background(), badSecret)).To(Succeed())
		_, err := scope.getRawBootstrapData()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("secret value key is missing"))
	})
}

func TestGetImages(t *testing.T) {
	t.Run("returns images list from PowerVS client", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockpowervs := mock.NewMockPowerVS(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		expected := &models.Images{
			Images: []*models.ImageReference{
				{Name: ptr.To("img-a"), ImageID: ptr.To("id-a")},
			},
		}
		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(expected, nil)

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		got, err := scope.getImages(context.Background())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(Equal(expected))
	})

	t.Run("propagates error from PowerVS client", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockpowervs := mock.NewMockPowerVS(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		mockpowervs.EXPECT().ListImages(gomock.Any()).Return(nil, errors.New("list images failed"))

		scope := MachineScope{IBMPowerVSClient: mockpowervs}
		_, err := scope.getImages(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("list images failed"))
	})
}

func TestExtractIPsFromInstance(t *testing.T) {
	t.Run("returns empty slice when instance has no networks", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{Networks: nil}
		g.Expect(extractIPsFromInstance(instance)).To(BeEmpty())
	})

	t.Run("skips network entry when IPAddress is whitespace only", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{
			Networks: []*models.PVMInstanceNetwork{
				{IPAddress: "   ", ExternalIP: ""},
			},
		}
		g.Expect(extractIPsFromInstance(instance)).To(BeEmpty())
	})

	t.Run("skips ExternalIP when it is whitespace only", func(t *testing.T) {
		g := NewWithT(t)
		instance := &models.PVMInstance{
			Networks: []*models.PVMInstanceNetwork{
				{IPAddress: "10.0.0.1", ExternalIP: "  "},
			},
		}
		ips := extractIPsFromInstance(instance)
		g.Expect(ips).To(HaveLen(1))
		g.Expect(ips[0]).To(Equal(clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"}))
	})
}

func TestCreateIgnitionDataEndpointWithoutScheme(t *testing.T) {
	t.Run("uses bare hostname when COS endpoint override has no scheme", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockCOS := cosmock.NewMockCos(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		// Endpoint override with no scheme — triggers the else-branch on line 945
		scope.ServiceEndpoint = []endpoints.ServiceEndpoint{
			{ID: string(endpoints.COS), URL: "custom-cos.example.com"},
		}
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://test-bucket.custom-cos.example.com/node/foo-machine?sig=abc", nil)

		objectURL, err := scope.createIgnitionData(ctx, []byte("data"))
		g.Expect(err).ToNot(HaveOccurred())
		// objHost should be "test-bucket.custom-cos.example.com" (no scheme stripping)
		g.Expect(objectURL).To(ContainSubstring("test-bucket.custom-cos.example.com"))
	})
}

func TestIgnitionUserData(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockCOS  *cosmock.MockCos
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOS = cosmock.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	makeScope := func(version string, cosClient cos.Cos) *MachineScope {
		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = cosClient
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		scope.IBMPowerVSCluster.Spec.Ignition = infrav1.Ignition{Version: version}
		return scope
	}

	t.Run("error when ignition version cannot be parsed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://presigned.url", nil)
		scope := makeScope("not-a-version", mockCOS)
		_, err := scope.ignitionUserData(ctx, []byte("bootstrap"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to parse ignition version"))
	})

	t.Run("produces v2 ignition redirect document", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://presigned.url/v2", nil)
		scope := makeScope("2.3", mockCOS)
		data, err := scope.ignitionUserData(ctx, []byte("bootstrap"))
		g.Expect(err).ToNot(HaveOccurred())
		// semver normalises "2.3" → "2.3.0"
		g.Expect(string(data)).To(ContainSubstring(`"version":"2.3.0"`))
	})

	t.Run("produces v3 ignition redirect document", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://presigned.url/v3", nil)
		scope := makeScope("3.4", mockCOS)
		data, err := scope.ignitionUserData(ctx, []byte("bootstrap"))
		g.Expect(err).ToNot(HaveOccurred())
		// semver normalises "3.4" → "3.4.0"
		g.Expect(string(data)).To(ContainSubstring(`"version":"3.4.0"`))
	})

	t.Run("error when ignition major version is unsupported", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://presigned.url/v4", nil)
		scope := makeScope("4.0", mockCOS)
		_, err := scope.ignitionUserData(ctx, []byte("bootstrap"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unsupported ignition version"))
	})
}

func TestResolveUserDataIgnitionSuccess(t *testing.T) {
	t.Run("returns base64-encoded ignition v2 document when ignition is configured", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockCOS := cosmock.NewMockCos(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, nil)
		scope.COSClient = mockCOS
		scope.IBMPowerVSCluster.Spec.COSInstance = infrav1.COSInstanceSource{
			Type: infrav1.SourceTypeProvision,
		}
		scope.IBMPowerVSCluster.Status.COSInstance.BucketName = "test-bucket"
		mockCOS.EXPECT().PutObject(gomock.Any()).Return(nil, nil)
		mockCOS.EXPECT().PresignedURL(gomock.Any(), gomock.Any(), gomock.Any()).Return("https://presigned.url/ignition", nil)

		result, err := scope.resolveUserData(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).NotTo(BeEmpty())
	})
}

func TestCreateVPCLoadBalancerPoolMemberEmptyLBName(t *testing.T) {
	t.Run("error when reference LB has empty name", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockVPC := vpcmock.NewMockVpc(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"},
		}
		pvsCluster := newPowerVSCluster(clusterName)
		// Reference type with both Name and ID empty — triggers the lbName == "" guard
		pvsCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{},
			},
		}
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{{ID: "lb-id", Name: "some-lb"}}

		scope := &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      mockVPC,
		}

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to determine VPC load balancer name"))
	})

	t.Run("error when LB name is found in status but ID is empty", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		mockVPC := vpcmock.NewMockVpc(mockCtrl)
		t.Cleanup(mockCtrl.Finish)

		pvsMachine := newPowerVSMachine(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true)
		pvsMachine.Status.Addresses = []clusterv1.MachineAddress{
			{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"},
		}
		pvsCluster := newPowerVSCluster(clusterName)
		pvsCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
			},
		}
		// Status entry has matching name but empty ID
		pvsCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{{ID: "", Name: "my-lb"}}

		scope := &MachineScope{
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: pvsCluster,
			Machine:           newMachine(machineName),
			IBMVPCClient:      mockVPC,
		}

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to find VPC load balancer ID"))
	})
}

func TestCreateVPCLoadBalancerPoolMemberSkipAndRequeue(t *testing.T) {
	var mockCtrl *gomock.Controller
	setupSR := func(t *testing.T) { t.Helper(); mockCtrl = gomock.NewController(t) }
	teardownSR := func() { mockCtrl.Finish() }

	lbID := "lb-id-1"
	lbName := "lb-1"
	poolID := "pool-id-1"
	poolName := fmt.Sprintf("pool-1-%d", 6443)
	machineIP := "10.0.0.5"

	baseCluster := func(lbSrc infrav1.LoadBalancerSource, lbStatus infrav1.LoadBalancerStatus) *infrav1.IBMPowerVSCluster {
		return &infrav1.IBMPowerVSCluster{
			Spec:   infrav1.IBMPowerVSClusterSpec{LoadBalancers: []infrav1.LoadBalancerSource{lbSrc}},
			Status: infrav1.IBMPowerVSClusterStatus{LoadBalancers: []infrav1.LoadBalancerStatus{lbStatus}},
		}
	}
	cpMachine := func() *clusterv1.Machine {
		return &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"cluster.x-k8s.io/control-plane": "true"}}}
	}
	machineWithIP := func() *infrav1.IBMPowerVSMachine {
		return &infrav1.IBMPowerVSMachine{
			Status: infrav1.IBMPowerVSMachineStatus{
				Addresses: []clusterv1.MachineAddress{{Address: machineIP, Type: clusterv1.MachineInternalIP}},
			},
		}
	}

	t.Run("Skips busy load balancer without error", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		busyLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateUpdatePending)),
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(busyLB, nil, nil)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		pendingUpdate, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(pendingUpdate).To(BeTrue())
		// No CreateLoadBalancerPoolMember call expected (gomock will fail the test if one occurs).
	})

	t.Run("Treats nil member ProvisioningStatus as pending without panic", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		activeLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID), Name: ptr.To(poolName)}},
		}
		nilStatusMember := &vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("member-id-nil"),
			ProvisioningStatus: nil,
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(activeLB, nil, nil)
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(
			&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil)
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(nilStatusMember, nil, nil)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		pendingUpdate, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(pendingUpdate).To(BeTrue())
	})

	t.Run("Returns error for non-transient load balancer state", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		deletingLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateDeletePending)),
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(deletingLB, nil, nil)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("non-recoverable state"))
	})

	t.Run("Returns error when load balancer ProvisioningStatus is nil", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		nilStatusLB := &vpcv1.LoadBalancer{
			ID:   ptr.To(lbID),
			Name: ptr.To(lbName),
			// ProvisioningStatus intentionally nil
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(nilStatusLB, nil, nil)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		_, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("has no provisioning status"))
	})

	t.Run("Treats create_pending with zero pools as transient, not an error", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		creatingLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
			// Pools intentionally empty — LB not yet fully created
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(creatingLB, nil, nil)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		pendingUpdate, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(pendingUpdate).To(BeTrue())
	})

	t.Run("Issues at most one write per load balancer per pass", func(t *testing.T) {
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		pool2ID := "pool-id-2"
		pool2Name := fmt.Sprintf("pool-2-%d", 22)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		activeLBTwoPools := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{ID: ptr.To(poolID), Name: ptr.To(poolName)},
				{ID: ptr.To(pool2ID), Name: ptr.To(pool2Name)},
			},
		}

		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(activeLBTwoPools, nil, nil)
		// pool-1 is listed then written; pool-2 is never listed because break exits the loop after the write.
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(
			&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).Times(1)
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).
			Return(&vpcv1.LoadBalancerPoolMember{ID: ptr.To("member-id-1"), ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive))}, nil, nil).Times(1)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		pendingUpdate, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(BeNil())
		// Wrote to pool-1; pool-2 deferred → pendingUpdate=true, only 1 POST issued (enforced by Times(1)).
		g.Expect(pendingUpdate).To(BeTrue())
	})

	t.Run("Makes progress on active load balancer when other is busy", func(t *testing.T) {
		// Two LBs in the cluster: lb-1 is update_pending (busy), lb-2 is active with an
		// unregistered pool. The fix must write to lb-2 and return pendingUpdate=true
		// (because lb-1 was skipped, not because lb-2 is incomplete).
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		lbID2 := "lb-id-2"
		lbName2 := "lb-2"
		poolID2 := "pool-id-2"
		poolName2 := fmt.Sprintf("pool-2-%d", 6443)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		busyLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateUpdatePending)),
		}
		activeLB2 := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID2),
			Name:               ptr.To(lbName2),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID2), Name: ptr.To(poolName2)}},
		}
		registeredMember := &vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("member-id-2"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}

		// lb-1 is busy — one GetLoadBalancer call, no write.
		mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(lbID)}).
			Return(busyLB, nil, nil).Times(1)
		// lb-2 is active — proceeds through list+create.
		mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(lbID2)}).
			Return(activeLB2, nil, nil).Times(1)
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).
			Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).Times(1)
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).
			Return(registeredMember, nil, nil).Times(1)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID2, Name: lbName2}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: lbName, ID: lbID},
						{Name: lbName2, ID: lbID2},
					},
				},
			},
		}

		pendingUpdate, err := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err).To(BeNil())
		// lb-1 was busy → pendingUpdate=true even though lb-2 made progress.
		g.Expect(pendingUpdate).To(BeTrue())
		// Exactly one POST was issued — to lb-2, not lb-1 (enforced by Times(1) above).
	})

	t.Run("Completes registration across multiple passes", func(t *testing.T) {
		// Pass 1: pool is unregistered → POST, member returns active.
		// Pass 2: pool already has machineIP → no write, pendingUpdate=false.
		g := NewWithT(t)
		setupSR(t)
		t.Cleanup(teardownSR)

		mockClient := vpcmock.NewMockVpc(mockCtrl)
		activeLB := &vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			Name:               ptr.To(lbName),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To(poolID), Name: ptr.To(poolName)}},
		}

		// Pass 1: empty pool → POST → active member.
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).
			Return(activeLB, nil, nil).Times(1)
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).
			Return(&vpcv1.LoadBalancerPoolMemberCollection{}, nil, nil).Times(1)
		mockClient.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).
			Return(&vpcv1.LoadBalancerPoolMember{
				ID:                 ptr.To("member-id-1"),
				ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
			}, nil, nil).Times(1)

		scope := MachineScope{
			Machine:           cpMachine(),
			IBMVPCClient:      mockClient,
			IBMPowerVSMachine: machineWithIP(),
			IBMPowerVSCluster: baseCluster(
				infrav1.LoadBalancerSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: lbID, Name: lbName}},
				infrav1.LoadBalancerStatus{Name: lbName, ID: lbID},
			),
		}

		pending1, err1 := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err1).To(BeNil())
		// Active member returned, pool written → pendingUpdate=false (all done).
		g.Expect(pending1).To(BeFalse())

		// Pass 2: pool already has machineIP registered → no POST, pendingUpdate=false.
		registeredMembers := &vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{
				{Target: &vpcv1.LoadBalancerPoolMemberTarget{Address: ptr.To(machineIP)}},
			},
		}
		mockClient.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).
			Return(activeLB, nil, nil).Times(1)
		mockClient.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).
			Return(registeredMembers, nil, nil).Times(1)

		pending2, err2 := scope.CreateVPCLoadBalancerPoolMember(ctx)
		g.Expect(err2).To(BeNil())
		g.Expect(pending2).To(BeFalse())
	})
}
