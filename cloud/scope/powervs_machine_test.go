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
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcecontrollermock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/options"

	. "github.com/onsi/gomega"
)

const (
	region = "us-south"
)

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1.IBMPowerVSMachine {
	image := &infrav1.IBMPowerVSResourceReference{}
	network := infrav1.IBMPowerVSResourceReference{}

	if !isID {
		image.Name = imageRef
		network.Name = networkRef
	} else {
		image.ID = imageRef
		network.ID = networkRef
	}

	return &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
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
	powerVSMachine := newPowerVSMachine(clusterName, machineName, imageID, networkID, isID)
	powerVSCluster := newPowerVSCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, powerVSCluster, powerVSMachine,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSMachineScope{
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

func TestAPIServerPort(t *testing.T) {
	testcases := []struct {
		name               string
		expectedPortNumber int32
		machineScope       PowerVSMachineScope
	}{
		{
			name:               "Returns assigned port number",
			expectedPortNumber: int32(6445),
			machineScope: PowerVSMachineScope{
				Cluster: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						ClusterNetwork: clusterv1.ClusterNetwork{
							APIServerPort: int32(6445),
						},
					},
				},
			},
		}, {
			name:               "Returns DefaultAPIServerPort when machineScope.Cluster.Spec.ClusterNetwork is nil",
			expectedPortNumber: infrav1.DefaultAPIServerPort,
			machineScope: PowerVSMachineScope{
				Cluster: &clusterv1.Cluster{},
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
			name:               "Bucket exists in COS instance",
			expectedBucketName: "foo-bucket",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{
							BucketName: "foo-bucket",
						},
					},
				},
			},
		}, {
			name:               "Deriving COS bucket name from PowerVS cluster name",
			expectedBucketName: fmt.Sprintf("%s-%s", "foo-cluster", "cosbucket"),
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
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
		name                 string
		expectedBucketRegion string
		machineScope         PowerVSMachineScope
	}{
		{
			name:                 "Get bucket region from COS instance",
			expectedBucketRegion: region,
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{
							BucketRegion: region,
						},
					},
				},
			},
		}, {
			name:                 "Get bucket region from VPC region set in spec",
			expectedBucketRegion: region,
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						VPC: &infrav1.VPCResourceReference{
							Region: ptr.To(region),
						},
					},
				},
			},
		}, {
			name: "Returns empty region when both COS instance and VPC source spec are empty",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			region := tc.machineScope.bucketRegion()
			g.Expect(region).To(Equal(tc.expectedBucketRegion))
		})
	}
}

func TestNewPowerVSMachineScope(t *testing.T) {
	testCases := []struct {
		name   string
		params PowerVSMachineScopeParams
	}{
		{
			name: "Returns error when controller runtime client in nil",
			params: PowerVSMachineScopeParams{
				Client: nil,
			},
		},
		{
			name: "Returns error when Machine in nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: nil,
			},
		},
		{
			name: "Returns error when Cluster is nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: newMachine(machineName),
				Cluster: nil,
			},
		},
		{
			name: "Returns error when IBMPowerVSMachine is nil",
			params: PowerVSMachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: nil,
			},
		},
		{
			name: "Error initialising authenticator",
			params: PowerVSMachineScopeParams{
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
			_, err := NewPowerVSMachineScope(tc.params)
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
		machineScope              PowerVSMachineScope
	}{
		{
			name:                      "Returns service instance ID set in IBMPowerVSCluster.Status.ServiceInstance.ID",
			expectedServiceInstanceID: "service-instance-0",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1.ResourceReference{
							ID: ptr.To("service-instance-0"),
						},
					},
				},
			},
		}, {
			name:                      "get service instance ID from powervsClusterSpec",
			expectedServiceInstanceID: "service-instance-1",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
			},
		}, {
			name:                      "get service instance ID from powervsClusterSpec's serviceInstance",
			expectedServiceInstanceID: "service-instance-2",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ServiceInstance: &infrav1.IBMPowerVSResourceReference{
							ID: ptr.To("service-instance-2"),
						},
					},
				},
			},
		}, {
			name:                      "get service instance ID with serviceInstanceID present in both IBMPowerVSCluster Status and Spec ",
			expectedServiceInstanceID: "service-instance-in-status",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1.ResourceReference{
							ID: ptr.To("service-instance-in-status"),
						},
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-in-spec",
					},
				},
			},
		}, {
			name: "Failed to find service instance id",
			machineScope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ServiceInstance: &infrav1.IBMPowerVSResourceReference{},
					},
				},
			},
			expectedError: fmt.Errorf("failed to find service instance id as both name and id are not set"),
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			serviceInstanceID, err := tc.machineScope.GetServiceInstanceID()
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
		scope := PowerVSMachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("foo-cluster"),
					},
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Zone: ptr.To("us-south-1"),
				},
			},
		}
		mockResourceController.EXPECT().GetServiceInstance("", "foo-cluster", gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("foo-id")}, nil)
		scope.ResourceClient = mockResourceController
		serviceInstanceID, err := scope.GetServiceInstanceID()
		g.Expect(serviceInstanceID).To(Equal("foo-id"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Failed to get Power VS service instance id", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		scope := PowerVSMachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("foo-cluster"),
					},
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Zone: ptr.To("us-south-1"),
				},
			},
		}
		mockResourceController.EXPECT().GetServiceInstance("", "foo-cluster", gomock.Any()).Return(nil, fmt.Errorf("failed to list instance id"))
		scope.ResourceClient = mockResourceController
		serviceInstanceID, err := scope.GetServiceInstanceID()
		g.Expect(serviceInstanceID).To(Equal(""))
		g.Expect(err).ToNot(BeNil())
	})
}

func TestSetReady(t *testing.T) {
	t.Run("Set Machine status to ready", func(t *testing.T) {
		g := NewWithT(t)
		machineScope := PowerVSMachineScope{
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
		machineScope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Ready: true,
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
		scope          PowerVSMachineScope
		expectedRegion string
	}{
		{
			name: "Returns region set in spec",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Region: ptr.To(region),
					},
				},
			},
			expectedRegion: region,
		}, {
			name: "Return empty string when region is not set in spec",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Region: nil,
					},
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
		scope          PowerVSMachineScope
		expectedRegion string
	}{
		{
			name: "Set region to us-east in IBMPowerVSMachine status",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedRegion: "us-east",
		}, {
			name: "Set region to empty value in IBMPowerVSMachine status",
			scope: PowerVSMachineScope{
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
		scope        PowerVSMachineScope
		expectedZone string
	}{
		{
			name: "Machine's zone is set",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Zone: ptr.To("us-south-1"),
					},
				},
			},
			expectedZone: "us-south-1",
		}, {
			name: "Machine's zone is nil",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Zone: nil,
					},
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
		scope        PowerVSMachineScope
		expectedZone string
	}{
		{
			name: "Set machine's zone to us-east-1",
			scope: PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			},
			expectedZone: "us-east-1",
		}, {
			name: "Set machine's zone to an empty value",
			scope: PowerVSMachineScope{
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
		machineScope := PowerVSMachineScope{
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
		scope                   PowerVSMachineScope
	}{
		{
			name:                    "Ignition version is nil",
			expectedIgnitionVersion: infrav1.DefaultIgnitionVersion,
			scope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		}, {
			name:                    "Custom Ignition Version is set",
			expectedIgnitionVersion: "3.4",
			scope: PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						Ignition: &infrav1.Ignition{
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
			g.Expect(getIgnitionVersion(&tc.scope)).To(Equal(tc.expectedIgnitionVersion))
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
			machineScope := PowerVSMachineScope{
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
			scope := PowerVSMachineScope{}
			expectedNetworkID := networkID
			networkResource := infrav1.IBMPowerVSResourceReference{
				ID: ptr.To(expectedNetworkID),
			}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(*networkID).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})
		t.Run("Returns network ID from PowerVS Machine scope", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			networkName := "foo-network-name"
			expectedNetworkID := networkID
			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						Name:      ptr.To(networkName),
						NetworkID: ptr.To(expectedNetworkID),
					},
				},
			}
			networkResource := infrav1.IBMPowerVSResourceReference{
				Name: ptr.To(networkName),
			}

			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(*networkID).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})

		t.Run("Failed to find network ID", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedNetworkIName := "foo-network"
			differentNetworkName := "diff-network-name"

			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						Name: ptr.To(differentNetworkName),
					},
				},
			}
			networkResource := infrav1.IBMPowerVSResourceReference{
				Name: ptr.To(expectedNetworkIName),
			}

			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(networkID).To(BeNil())
			g.Expect(err.Error()).To(Equal(fmt.Sprintf("failed to find a network ID with name %s", expectedNetworkIName)))
		})

		t.Run("Fetch network ID with matching regex", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			networkName := "550e8400-e29b-41d4-a716-446655440000"
			expectedNetworkID := "foo-id"
			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						Name:      ptr.To(networkName),
						NetworkID: ptr.To(expectedNetworkID),
					},
				},
			}
			networkResource := infrav1.IBMPowerVSResourceReference{
				RegEx: ptr.To("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"),
			}

			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(*networkID).To(Equal(expectedNetworkID))
			g.Expect(err).To(BeNil())
		})

		t.Run("Failed to fetch network ID with matching regex", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedNetworkID := "foo-netID"
			regex := "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						Name: ptr.To(expectedNetworkID),
					},
				},
			}
			networkResource := infrav1.IBMPowerVSResourceReference{
				RegEx: ptr.To(regex),
			}

			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			scope := PowerVSMachineScope{
				IBMPowerVSClient: mockpowervs,
			}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(networkID).To(BeNil())
			g.Expect(err.Error()).To(Equal(fmt.Sprintf("failed to find a network ID with RegEx %s", regex)))
		})

		t.Run("When ID, name and regex are all nil", func(t *testing.T) {
			g := NewWithT(t)
			networkResource := infrav1.IBMPowerVSResourceReference{}
			scope := PowerVSMachineScope{}
			networkID, err := getNetworkID(networkResource, &scope)
			g.Expect(networkID).To(BeNil())
			g.Expect(err.Error()).To(Equal("ID, Name and RegEx can't be nil"))
		})
	})
}

func TestGetMachineInternalIP(t *testing.T) {
	t.Run("Get Machine Internal IP", func(t *testing.T) {
		t.Run("Returns machine IP for address type - Node Internal IP", func(t *testing.T) {
			g := NewWithT(t)
			expectedAddress := "10.0.0.1"
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
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

		t.Run("Returns empty IP for address type - node external IP", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
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

		t.Run("Returns empty IP if powervsmachineStatus in nil", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
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
		scope := PowerVSMachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{},
				},
			},
		}
		options.ProviderIDFormat = string(options.ProviderIDFormatV2)
		err := scope.SetProviderID(providerID)
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("Set Provider ID in v2 format", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("foo-service-instance-id"),
					},
				},
			},
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{},
		}
		options.ProviderIDFormat = string(options.ProviderIDFormatV2)
		scope.SetZone("us-south-1")
		scope.SetRegion(region)
		err := scope.SetProviderID(providerID)
		expectedProviderID := ptr.To(fmt.Sprintf("ibmpowervs://%s/%s/%s/%s", scope.GetRegion(), scope.GetZone(), "foo-service-instance-id", providerID))
		g.Expect(*scope.IBMPowerVSMachine.Spec.ProviderID).To(Equal(*expectedProviderID))
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
		mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
		t.Run("Error getting COS service instance", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(nil, errors.New("error listing COS instances"))
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("COS service instance is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(nil, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient(ctx)
			g.Expect(result).To(BeNil())
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("COS service instance is not in active state", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			serviceInstance := &resourcecontrollerv2.ResourceInstance{
				State: ptr.To(string(infrav1.ServiceInstanceStateProvisioning)),
			}
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient(ctx)
			expectedError := fmt.Sprintf("COS service instance is not in active state, current state: %s", infrav1.ServiceInstanceStateProvisioning)
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(ContainSubstring(expectedError))
		})

		t.Run("Failed to determine COS bucket region", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			serviceInstance := &resourcecontrollerv2.ResourceInstance{
				State: ptr.To(string(infrav1.ServiceInstanceStateActive)),
			}
			scope.SetRegion(region)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			result, err := scope.createCOSClient(ctx)
			expectedError := "failed to determine COS bucket region, both bucket region and VPC region not set"
			g.Expect(result).To(BeNil())
			g.Expect(err.Error()).To(ContainSubstring(expectedError))
		})
		t.Run("Creates COS client successfully", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			serviceInstance := &resourcecontrollerv2.ResourceInstance{
				State: ptr.To(string(infrav1.ServiceInstanceStateActive)),
				GUID:  ptr.To("foo-guid"),
			}
			scope.SetRegion(region)
			cosInstanceName := fmt.Sprintf("%s-%s", scope.IBMPowerVSCluster.GetName(), "cosinstance")
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope.ResourceClient = mockResourceController
			expectedBucketRegion := region
			scope.IBMPowerVSCluster.Spec.CosInstance = &infrav1.CosInstance{BucketRegion: expectedBucketRegion}
			_, err := scope.createCOSClient(ctx)
			g.Expect(err).To(BeNil())
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
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			}
			scope.SetInstanceID(tc.instanceID)
			g.Expect(scope.GetInstanceID()).To(Equal(tc.expectedInstanceID))
		})
	}
}

func TestSetFailureReason(t *testing.T) {
	t.Run("Set failure reason to InvalidConfiguration", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		scope.SetFailureReason(infrav1.UpdateMachineError)
		//nolint:staticcheck
		g.Expect(*scope.IBMPowerVSMachine.Status.FailureReason).To(Equal(infrav1.UpdateMachineError))
	})
}

func TestSetHealth(t *testing.T) {
	t.Run("Test SetHealth", func(t *testing.T) {
		t.Run("Set PVMInstance status to healthy", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
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
			scope := PowerVSMachineScope{
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
			}
			scope.SetHealth(nil)
			g.Expect(scope.IBMPowerVSMachine.Status.Health).To(Equal(""))
		})
	})
}

func TestSetFailureMessage(t *testing.T) {
	t.Run("Set failure message for PowerVSMachine status", func(t *testing.T) {
		g := NewWithT(t)
		scope := PowerVSMachineScope{
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{},
			},
		}
		failureMessage := "invalid configuration provided"
		scope.SetFailureMessage(failureMessage)
		g.Expect(*scope.IBMPowerVSMachine.Status.FailureMessage).To(Equal(failureMessage)) //nolint:staticcheck
	})
}
func TestDeleteMachineIgnition(t *testing.T) {
	t.Run("Delete machine ignition", func(t *testing.T) {
		t.Run("Fails to retrieve bootstrap data: linked Machine's bootstrap.dataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			scope := PowerVSMachineScope{
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: nil,
						},
					},
				},
			}
			err := scope.DeleteMachineIgnition(ctx)
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{},
				},
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			err := scope.DeleteMachineIgnition(ctx)
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Ignition: &infrav1.Ignition{
							Version: "3.1",
						},
					},
				},
				ResourceClient: mockResourceController,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			err := scope.DeleteMachineIgnition(ctx)
			g.Expect(err).ToNot(BeNil())
		})

		t.Run("Successful DeleteMachineIgnition", func(t *testing.T) {
			g := NewWithT(t)
			bootstrapSecret := newBootstrapSecret(clusterName, machineName)
			initObjects := []client.Object{
				bootstrapSecret,
			}
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
			mockResourceController := resourcecontrollermock.NewMockResourceController(gomock.NewController(t))
			cosInstanceName := fmt.Sprintf("%s-%s", clusterName, "cosinstance")
			serviceInstance := new(resourcecontrollerv2.ResourceInstance)
			state := string(infrav1.ServiceInstanceStateActive)
			serviceInstance.State = &state
			guid := "foo-guid"
			serviceInstance.GUID = &guid
			expectedBucketRegion := region
			mockResourceController.EXPECT().GetInstanceByName(cosInstanceName, resourcecontroller.CosResourceID, resourcecontroller.CosResourcePlanID).Return(serviceInstance, nil)
			scope := PowerVSMachineScope{
				Client: client,
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Ignition: &infrav1.Ignition{
							Version: "3.1",
						},
						CosInstance: &infrav1.CosInstance{
							BucketRegion: expectedBucketRegion,
						},
					},
				},
				ResourceClient: mockResourceController,
				Machine: &clusterv1.Machine{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To(machineName),
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
				},
			}
			scope.SetRegion(region)
			err := scope.DeleteMachineIgnition(ctx)
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
					ServerName: ptr.To("foo-machine-1"),
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
		networks := &models.Networks{
			Networks: []*models.NetworkReference{
				{
					Name:      ptr.To(pvsNetwork),
					NetworkID: ptr.To(pvsNetwork + idSuffix),
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
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
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
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
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
			scope.IBMPowerVSMachine.Status.Conditions = append(scope.IBMPowerVSMachine.Status.Conditions, clusterv1beta1.Condition{
				Type:   infrav1.InstanceReadyCondition,
				Status: corev1.ConditionUnknown,
			})
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			out, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
			g.Expect(out).To(Equal(expectedOutput))
		})

		t.Run("Eror while getting instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, errors.New("error when getting list of instances"))
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("foo-secret-temp")
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
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
					Namespace: "default",
				},
				Data: map[string][]byte{
					"val": []byte("user data"),
				}}
			g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Invalid processors value", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Spec.Processors = intstr.FromString("invalid")
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
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
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Image and Network name is set", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when both Image id and name are nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when Image id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage+"-temp"), ptr.To(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when Network id does not exsist", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork+"-temp"), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			_, err := scope.CreateMachine(ctx)
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error while creating machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, errors.New("failed to create machine"))
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

	nodeAddress := "10.0.0.1"
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
			ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
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

		scope := PowerVSMachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"listener-selector": "port-22",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: loadBalancerName,
							ID:   ptr.To(loadBalancerID),
							AdditionalListeners: []infrav1.AdditionalListenerSpec{
								{
									Port: 23,
									Selector: metav1.LabelSelector{
										MatchLabels: map[string]string{
											"listener-selector": "port-23",
										},
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
						loadBalancerName: {
							ID: ptr.To(loadBalancerID),
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
			ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
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

		scope := PowerVSMachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"listener-selector": "port-22",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: loadBalancerName,
							ID:   ptr.To(loadBalancerID),
							AdditionalListeners: []infrav1.AdditionalListenerSpec{
								{
									Port: 22,
									Selector: metav1.LabelSelector{
										MatchLabels: map[string]string{
											"listener-selector": "port-22",
										},
									},
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
						loadBalancerName: {
							ID: ptr.To(loadBalancerID),
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
			ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
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

		scope := PowerVSMachineScope{
			Machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			IBMVPCClient: mockClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"listener-selector": "port-6443",
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: loadBalancerName,
							ID:   ptr.To(loadBalancerID),
							AdditionalListeners: []infrav1.AdditionalListenerSpec{
								{
									Port: 6443,
								},
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
						loadBalancerName: {
							ID: ptr.To(loadBalancerID),
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
			ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
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

		scope := PowerVSMachineScope{
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
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: loadBalancerName,
							ID:   ptr.To(loadBalancerID),
							AdditionalListeners: []infrav1.AdditionalListenerSpec{
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
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
						loadBalancerName: {
							ID: ptr.To(loadBalancerID),
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
			scope := PowerVSMachineScope{
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
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: ptr.To(loadBalancerID),
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
				ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateCreatePending),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: ptr.To(loadBalancerID),
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
				ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)
			mockClient.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To(loadBalancerID)}).Return(loadBalancers, nil, nil)
			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: ptr.To(loadBalancerID),
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
				ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
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

			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: nodeAddress,
								Type:    corev1.NodeInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: ptr.To(loadBalancerID),
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
			scope := PowerVSMachineScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								ID: ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{},
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
				ProvisioningStatus: (*string)(&infrav1.VPCLoadBalancerStateActive),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   ptr.To("pool-id-2"),
						Name: ptr.To(fmt.Sprintf("pool-2-%d", targetPort)),
					},
				},
			}
			mockClient := vpcmock.NewMockVpc(mockCtrl)

			scope := PowerVSMachineScope{
				IBMVPCClient: mockClient,
				IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
					Status: infrav1.IBMPowerVSMachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: nodeAddress,
								Type:    corev1.NodeInternalIP,
							},
						},
					},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{
								Name: loadBalancerName,
								ID:   ptr.To(loadBalancerID),
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							loadBalancerName: {
								ID: ptr.To(loadBalancerID),
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
							Address: ptr.To(nodeAddress),
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
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSMachineScope(clusterName, machineName, ptr.To(pvsImage), ptr.To(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Status.InstanceID = machineName + idSuffix
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(errors.New("failed to delete machine"))
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
			scope.SetAddresses(ctx, tc.pvmInstance)
			g.Expect(scope.IBMPowerVSMachine.Status.Addresses).To(Equal(tc.expectedNodeAddress))
		})
	}
}
