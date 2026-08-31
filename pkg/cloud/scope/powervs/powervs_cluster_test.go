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
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	resourcemanagerv2 "github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	mockcos "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos/mock"
	mockP "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	mockRM "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager/mock"
	tgmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

const (
	region           = "us-south"
	testLB1          = "lb1"
	testLB2          = "lb2"
	testLBName       = "test-lb"
	testLoadBalancer = "loadbalancer"
	testClusterName  = "ClusterName"
	testRegion       = "test-region"
)

func TestNewPowerVSClusterScope(t *testing.T) {
	testCases := []struct {
		name        string
		params      ClusterScopeParams
		expectError bool
	}{
		{
			name: "Error when Client in nil",
			params: ClusterScopeParams{
				Client: nil,
			},
			expectError: true,
		},
		{
			name: "Error when Cluster in nil",
			params: ClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: nil,
			},
			expectError: true,
		},
		{
			name: "Error when IBMPowerVSCluster is nil",
			params: ClusterScopeParams{
				Client:            testEnv.Client,
				Cluster:           newCluster(clusterName),
				IBMPowerVSCluster: nil,
			},
			expectError: true,
		},
		{
			name: "Successfully create cluster scope when create infra annotation is not set",
			params: ClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: newCluster(clusterName),
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "powervs-test-",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: clusterv1.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       "capi-test",
								UID:        "1",
							}}},
					Spec: infrav1.IBMPowerVSClusterSpec{Zone: "zone"},
				},
				ClientBuilder: stubClientBuilder{},
			},
			expectError: false,
		},
		{
			name: "Successfully create cluster scope when create infra annotation is set",
			params: ClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: newCluster(clusterName),
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Annotations:  map[string]string{"powervs.cluster.x-k8s.io/create-infra": "true"},
						GenerateName: "powervs-test-",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: clusterv1.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       "capi-test",
								UID:        "1",
							}}},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Topology: infrav1.PowerVSLoadBalancerTopology,
						Zone:     "zone",
						VPC: infrav1.VPCSource{
							Type:   infrav1.SourceTypeProvision,
							Region: "eu-gb",
						},
					},
				},
				ClientBuilder: stubClientBuilder{},
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewPowerVSClusterScope(context.Background(), tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			if tc.expectError {
				g.Expect(err).To(Not(BeNil()))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestGetDHCPServerID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   string
		clusterScope ClusterScope
	}{
		{
			name: "DHCP server ID is not set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedID: "",
		},
		{
			name: "DHCP server ID is set in status",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						Network: infrav1.NetworkStatus{
							DHCPServer: infrav1.ResourceReference{
								ID: "dhcpserverid",
							},
						},
					},
				},
			},
			expectedID: "dhcpserverid",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			dhcpServerID := tc.clusterScope.IBMPowerVSCluster.Status.Network.DHCPServer.ID
			g.Expect(dhcpServerID).To(Equal(tc.expectedID))
		})
	}
}

func TestGetLoadBalancerID(t *testing.T) {
	testCases := []struct {
		name         string
		lbName       string
		expectedID   string
		clusterScope ClusterScope
	}{
		{
			name: "LoadBalancer status is not set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "LoadBalancer status is empty",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{},
					},
				},
			},
		},
		{
			name: "empty LoadBalancer name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{Name: "lb", ID: "lb-1"},
						},
					},
				},
			},
		},
		{
			name: "invalid LoadBalancer name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{Name: "lb", ID: "lb-1"},
						},
					},
				},
			},
			lbName: testLB2,
		},
		{
			name: "valid LoadBalancer name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{Name: "lb", ID: "lb-1"},
						},
					},
				},
			},
			lbName:     "lb",
			expectedID: "lb-1",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			lbID := tc.clusterScope.GetLoadBalancerID(tc.lbName)
			g.Expect(lbID).To(Equal(tc.expectedID))
		})
	}
}

func TestGetPublicLoadBalancerHostName(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Load balancer status is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("Load balancer name is not set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: ""},
				Spec:       infrav1.IBMPowerVSClusterSpec{},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "-loadbalancer-public", Hostname: "lb-hostname"},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb-hostname")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Invalid load balancer name is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "lb",
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: testLoadBalancer, Hostname: "lb-hostname"},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("Valid load balancer name is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: testLoadBalancer,
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: testLoadBalancer, Hostname: "lb-hostname"},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb-hostname")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Both public and private load balancer name is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: testLB1,
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: testLB2,
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: testLB1, Hostname: "lb1-hostname"},
						{Name: testLB2, Hostname: "lb2-hostname"},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb2-hostname")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Valid referenced load balancer ID is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "loadbalancer-id",
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: testLoadBalancer, Hostname: "lb-hostname"},
					},
				},
			},
		}
		lb := &vpcv1.LoadBalancer{
			Name: ptr.To(testLoadBalancer),
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, &core.DetailedResponse{}, nil)

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb-hostname")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Invalid referenced load balancer ID is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "loadbalancer-id1",
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: testLoadBalancer, Hostname: "lb-hostname"},
					},
				},
			},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{}, errors.New("failed to get the load balancer"))

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestGetResourceGroupID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   string
		clusterScope ClusterScope
	}{
		{
			name: "Resource group ID is not set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "Resource group ID is set in spec",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: infrav1.ResourceGroupSource{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "rgID",
							},
						},
					},
				},
			},
			expectedID: "rgID",
		},
		{
			name: "Resource group ID is set in status",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: infrav1.ResourceReference{
							ID: "rgID",
						},
					},
				},
			},
			expectedID: "rgID",
		},
		{
			name: "spec Resource group ID takes precedence over status Resource group ID",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: infrav1.ResourceGroupSource{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "rgID",
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: infrav1.ResourceReference{
							ID: "rgID1",
						},
					},
				},
			},
			expectedID: "rgID",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			rgID := tc.clusterScope.GetResourceGroupID()
			g.Expect(rgID).To(Equal(tc.expectedID))
		})
	}
}

func TestIsDHCPServerActive(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When GetDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReference{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("GetDHCPServer returns error"))
		isActive, err := clusterScope.isDHCPServerActive(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isActive).To(BeFalse())
	})
	t.Run("When checkDHCPServerStatus returns error state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1.DHCPServerStateError))}
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReference{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(dhcpServer, nil)

		isActive, err := clusterScope.isDHCPServerActive(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isActive).To(BeFalse())
	})
	t.Run("When checkDHCPServerStatus returns active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1.DHCPServerStateActive))}
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReference{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(dhcpServer, nil)

		isActive, err := clusterScope.isDHCPServerActive(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(isActive).To(BeTrue())
	})
}

func TestCheckDHCPServerStatus(t *testing.T) {
	testCases := []struct {
		name           string
		dhcpServer     models.DHCPServerDetail
		expectedStatus bool
	}{
		{
			name:           "DHCP server is in build state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDBuild"), Status: ptr.To(string(infrav1.DHCPServerStateBuild))},
			expectedStatus: false,
		},
		{
			name:           "DHCP server is in active state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDActive"), Status: ptr.To(string(infrav1.DHCPServerStateActive))},
			expectedStatus: true,
		},
		{
			name:           "DHCP server is in error state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDError"), Status: ptr.To(string(infrav1.DHCPServerStateError))},
			expectedStatus: false,
		},
		{
			name:           "DHCP server is in invalid state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDError"), Status: ptr.To("InvalidState")},
			expectedStatus: false,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		clusterScope := ClusterScope{}
		t.Run(tc.name, func(_ *testing.T) {
			status, _ := clusterScope.checkDHCPServerStatus(ctx, tc.dhcpServer)
			g.Expect(status).To(Equal(tc.expectedStatus))
		})
	}
}

func TestPowerVSDeleteLoadBalancer(t *testing.T) {
	var (
		mockVpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVpc = mock.NewMockVpc(mockCtrl)
	}

	teardown := func() {
		mockCtrl.Finish()
	}
	powervsClusterScope := func() *ClusterScope {
		return &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-powervs-cluster",
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{
							Name: "capi-powervs-cluster-loadbalancer-public",
							ID:   "lb-id",
						},
					},
				},
			},
		}
	}

	t.Run("When load balancer is not found", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DeleteLoadBalancer returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete load balancer"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When load balancer deletion is in pending state", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateDeletePending)),
		}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When GetLoadBalancer returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed to get loadbalancer"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteLoadBalancer successfully deletes load balancer in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When one load balancer is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: testLB1,
				},
			},
		}
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{
				Name: testLB1,
				ID:   "lb-id",
			},
		}
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To(testLB1),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When deleting multiple load balancer", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: testLB1,
				},
			},
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: testLB2,
				},
			},
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: "lb3",
				},
			},
		}
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{
				Name: testLB1,
				ID:   "lb-id-1",
			},
			{
				Name: testLB2,
				ID:   "lb-id-2",
			},
			{
				Name: "lb3",
				ID:   "lb-id-3",
			},
		}
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id-1"),
			Name:               ptr.To(testLB1),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id-2"),
			Name:               ptr.To(testLB2),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id-3"),
			Name:               ptr.To("lb3"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, nil).Times(3)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
}

func TestDeleteVPCSecurityGroups(t *testing.T) {
	var (
		mockVpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	// scopeWithProvisionSG returns a ClusterScope with one Provision-type SG named "sc"
	// in the spec (so deletion is triggered) and the matching status entry.
	scopeWithProvisionSG := func() *ClusterScope {
		return &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.VPCSecurityGroupProvision{Name: "sc"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupStatus{
						{ID: "sc-id", Name: "sc"},
					},
				},
			},
		}
	}

	t.Run("When security group is not found (404), skip deletion", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := scopeWithProvisionSG()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When GetSecurityGroup returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := scopeWithProvisionSG()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("When DeleteSecurityGroup returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := scopeWithProvisionSG()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete security group"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("When DeleteSecurityGroup successfully deletes security group", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := scopeWithProvisionSG()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When deleting multiple managed SecurityGroups", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: "sc1"}},
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: "sc2"}},
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: "sc3"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupStatus{
						{ID: "sc1-id", Name: "sc1"},
						{ID: "sc2-id", Name: "sc2"},
						{ID: "sc3-id", Name: "sc3"},
					},
				},
			},
		}
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil).Times(3)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, nil).Times(3)
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When one security group is referenced (not managed), skip it", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						// sc1 is managed (Provision), sc2 is referenced — only sc1 should be deleted
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: "sc1"}},
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{Name: "sc2"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupStatus{
						{ID: "sc1-id", Name: "sc1"},
						{ID: "sc2-id", Name: "sc2"},
					},
				},
			},
		}
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc1-id"),
			Name: ptr.To("sc1"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When no security groups are managed by controller, no deletion occurs", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "sc-id"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupStatus{
						{ID: "sc-id", Name: "sc"},
					},
				},
			},
		}
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestPowerVSDeleteVPC(t *testing.T) {
	var (
		mockVpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVpc = mock.NewMockVpc(mockCtrl)
	}

	teardown := func() {
		mockCtrl.Finish()
	}
	powervsClusterScope := func() *ClusterScope {
		return &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{
						ID:   "vpcid",
						Name: "vpcName",
					},
				},
			},
		}
	}

	t.Run("When VPC is not found", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is nil", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPC.ID = ""
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DeleteVPC returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Name: ptr.To("vpcName"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteVPC(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete vpc"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When vpc deletion is in pending state", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Status: ptr.To(string(infrav1.VPCStateDeleting))}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When GetVPC returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, errors.New("failed to get subnet"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteVPC successfully deletes VPC in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Name: ptr.To("vpcName"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteVPC(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When VPC is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		// Set VPC type to Reference to indicate it's not managed by controller
		clusterScope.IBMPowerVSCluster.Spec.VPC.Type = infrav1.SourceTypeReference
		clusterScope.IBMPowerVSCluster.Status.VPC = infrav1.VPCStatus{ID: "vpcid", Name: "vpcName"}
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestDeleteTransitGateway(t *testing.T) {
	var (
		mockCtrl *gomock.Controller
		mockTG   *tgmock.MockTransitGateway
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTG = tgmock.NewMockTransitGateway(mockCtrl)
	}

	teardown := func() {
		mockCtrl.Finish()
	}
	powervsClusterScope := func() *ClusterScope {
		return &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						ID: "transitgatewayID",
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "connectionID",
						},
						VPCConnection: infrav1.ResourceConnectionStatus{
							ID: "connectionID",
						},
					},
				},
			},
		}
	}

	t.Run("When transit gateway is nil", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status = infrav1.IBMPowerVSClusterStatus{}
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DeleteTransitGateway returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeProvision,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitGatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "powervsConnectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "vpcConnectionID",
			},
		}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("connection not found")).Times(2)
		mockTG.EXPECT().DeleteTransitGateway(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete transit gateway"))
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When transit gateway is not found", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		clusterScope := powervsClusterScope()
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When GetTransitGateway returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		clusterScope := powervsClusterScope()
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, errors.New("failed to get transit gateway"))
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When TransitGateway deletion is in pending state", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateDeletePending))}
		clusterScope := powervsClusterScope()
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When DeleteTransitGateway successfully deletes transit gateway in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeProvision,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitGatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "powervsConnectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "vpcConnectionID",
			},
		}
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("connection not found")).Times(2)
		mockTG.EXPECT().DeleteTransitGateway(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When deleteTransitGatewayConnections returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeProvision,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitGatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "powervsConnectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "vpcConnectionID",
			},
		}
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{}, errors.New("failed to get transit gateway connections"))
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When deleteTransitGatewayConnections returns requeue as true", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeProvision,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitGatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "powervsConnectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "vpcConnectionID",
			},
		}
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateDeleting))}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{}, nil)
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When transit gateway is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		clusterScope := powervsClusterScope()
		// Set TransitGateway as Reference type - controller should not delete it
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeReference,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeReference,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeReference,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitgatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
		}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		clusterScope.TransitGatewayClient = mockTG
		requeue, err := clusterScope.DeleteTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestDeleteCOSInstance(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When COS instance status ID is empty, skip deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance type is Reference, skip deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeReference},
			},
			Status: infrav1.IBMPowerVSClusterStatus{
				COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
			},
		}}
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance state is pending_reclamation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
				},
			},
			ResourceClient: mockResourceController,
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To("pending_reclamation")}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance is not found (404)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
				},
			},
			ResourceClient: mockResourceController,
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, fmt.Errorf("error getting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When GetResourceInstance returns non-404 error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
				},
			},
			ResourceClient: mockResourceController,
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error getting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When COS instance is active and DeleteResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
				},
			},
			ResourceClient: mockResourceController,
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To(string(infrav1.WorkspaceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, nil)
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance is active and DeleteResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cosInstanceID"},
				},
			},
			ResourceClient: mockResourceController,
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To(string(infrav1.WorkspaceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, fmt.Errorf("error deleting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).NotTo(BeNil())
	})
}

func TestDeleteDHCPServer(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When DHCP Server resource is not created by controller", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When PowerVS service instance is created by controller", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				Workspace: infrav1.WorkspaceSource{
					Type:      infrav1.SourceTypeProvision,
					Provision: infrav1.WorkspaceProvisionConfig{},
				},
			},
			Status: infrav1.IBMPowerVSClusterStatus{
				Network: infrav1.NetworkStatus{
					DHCPServer: infrav1.ResourceReference{
						ID: "dhcpServerID",
					},
				},
			},
		}}
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When DHCP server ID is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				Network: infrav1.NetworkStatus{},
			},
		}}
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When the DHCP server is not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						DHCPServer: infrav1.ResourceReference{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("dhcp server does not exist"))
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When GetDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						DHCPServer: infrav1.ResourceReference{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error getting dhcp server"))
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("error getting dhcp server")))
	})
	t.Run("When DeleteDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						DHCPServer: infrav1.ResourceReference{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpServerID")}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(dhcpServer, nil)
		mockPowerVS.EXPECT().DeleteDHCPServer(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error deleting dhcp server"))
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err.Error()).To(Equal("failed to delete DHCP server: error deleting dhcp server"))
	})
	t.Run("When DHCP server deletion is successful", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						DHCPServer: infrav1.ResourceReference{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpServerID")}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(dhcpServer, nil)
		mockPowerVS.EXPECT().DeleteDHCPServer(gomock.Any(), gomock.Any()).Return(nil)
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestDeleteTransitGatewayConnections(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When PowerVS connection of transit gateway is in deleting state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "pvs-connID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateDeleting))}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When DeleteTransitGatewayConnection for PowerVS connection returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "pvs-connID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, fmt.Errorf("error deleting transit gateway connection"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err.Error()).To(Equal("failed to delete transit gateway connection: error deleting transit gateway connection"))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteTransitGatewayConnection for PowerVS connection succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "pvs-connID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When GetTransitGatewayConnection for PowerVS connection returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "powerVStgID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 400}, fmt.Errorf("error getting transit gateway connection"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err.Error()).To(Equal("failed to get transit gateway connection: error getting transit gateway connection"))
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When PowerVS connection is not found and VPC connection of transit gateway is deleted successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
						VPCConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{
							ID: "powerVStgID",
						},
						VPCConnection: infrav1.ResourceConnectionStatus{
							ID: "vpctgID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, errors.New("connection not found")).Times(1)
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil).Times(1)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, nil).Times(1)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
	t.Run("When GetTransitGatewayConnection for VPC connection returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						VPCConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{},
						VPCConnection: infrav1.ResourceConnectionStatus{
							ID: "vpctgID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 500}, fmt.Errorf("error getting transit gateway connection"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err.Error()).To(Equal("failed to get transit gateway connection: error getting transit gateway connection"))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteTransitGatewayConnection for VPC connection succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						VPCConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{},
						VPCConnection: infrav1.ResourceConnectionStatus{
							ID: "vpctgID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When VPC connection of transit gateway is not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						VPCConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						PowerVSConnection: infrav1.ResourceConnectionStatus{},
						VPCConnection: infrav1.ResourceConnectionStatus{
							ID: "vpctgID",
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, errors.New("connection not found"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}
func TestReconcileCOSInstance(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When COSInstance.Type is empty, reconcile is a no-op", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{},
				},
			},
		}
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When status ID is already set, verify state of instance", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type:         infrav1.SourceTypeProvision,
						BucketRegion: "test-region",
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "existing-cos-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("existing-cos"),
			State: ptr.To(string(infrav1.WorkspaceStateActive)),
		}, nil, nil)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil()) // fails because mock returns instance with no CRN
		g.Expect(err.Error()).To(ContainSubstring("has no CRN"))
	})

	t.Run("When status ID is set but GetResourceInstance errors, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "existing-cos-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When type is unknown, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: "Unknown",
					},
				},
			},
		}
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("unknown COS instance source type"))
	})
}

func TestReconcileCOSReference(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When reference has neither ID nor Name, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{})
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("ID or Name"))
	})

	t.Run("When reference ID is set and GetResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{ID: ptr.To("cos-id"), Name: ptr.To("cos-name")}, nil, nil,
		)
		inst, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{ID: "cos-id"})
		g.Expect(err).To(BeNil())
		g.Expect(inst.ID).To(Equal(ptr.To("cos-id")))
	})

	t.Run("When reference ID is set and GetResourceInstance errors", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{ID: "cos-id"})
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When reference Name is set and GetResourceInstanceByFilter succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{Name: ptr.To("cos-name"), GUID: ptr.To("cos-guid")}, nil,
		)
		inst, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{Name: "cos-name"})
		g.Expect(err).To(BeNil())
		g.Expect(inst.Name).To(Equal(ptr.To("cos-name")))
	})

	t.Run("When reference Name is set but not found in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		_, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{Name: "cos-name"})
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("not found"))
	})
}

func TestReconcileCOSProvision(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When instance already exists by name, return it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{Name: ptr.To("my-cos"), GUID: ptr.To("existing-guid")}, nil,
		)
		inst, err := clusterScope.reconcileCOSProvision(ctx, "my-cos")
		g.Expect(err).To(BeNil())
		g.Expect(inst.GUID).To(Equal(ptr.To("existing-guid")))
	})

	t.Run("When instance not found and resource group is empty, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		_, err := clusterScope.reconcileCOSProvision(ctx, "my-cos")
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("resource group ID is empty"))
	})

	t.Run("When instance not found, resource group set and CreateResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{ID: ptr.To("new-id"), GUID: ptr.To("new-guid"), Name: ptr.To("my-cos")}, nil, nil,
		)
		inst, err := clusterScope.reconcileCOSProvision(ctx, "my-cos")
		g.Expect(err).To(BeNil())
		g.Expect(inst.GUID).To(Equal(ptr.To("new-guid")))
	})

	t.Run("When CreateResourceInstance fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("creation failed"))
		_, err := clusterScope.reconcileCOSProvision(ctx, "my-cos")
		g.Expect(err).ToNot(BeNil())
	})
}

func TestReconcileCOSBucket(t *testing.T) {
	var (
		mockCOSController *mockcos.MockCos
		mockCtrl          *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOSController = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When bucket already exists, return nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSController.EXPECT().GetBucketByName("test-bucket").Return(nil, nil)
		err := clusterScope.reconcileCOSBucket(ctx, "test-bucket")
		g.Expect(err).To(BeNil())
	})

	t.Run("When bucket does not exist (NoSuchBucket), create it successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSController.EXPECT().GetBucketByName("test-bucket").Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "not found", nil))
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(&s3.CreateBucketOutput{}, nil)
		err := clusterScope.reconcileCOSBucket(ctx, "test-bucket")
		g.Expect(err).To(BeNil())
	})

	t.Run("When bucket does not exist and CreateBucket returns BucketAlreadyOwnedByYou, return nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSController.EXPECT().GetBucketByName("test-bucket").Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "not found", nil))
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "owned by you", nil))
		err := clusterScope.reconcileCOSBucket(ctx, "test-bucket")
		g.Expect(err).To(BeNil())
	})

	t.Run("When GetBucketByName returns unexpected error, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSController.EXPECT().GetBucketByName("test-bucket").Return(nil, awserr.New("UnexpectedError", "unexpected", nil))
		err := clusterScope.reconcileCOSBucket(ctx, "test-bucket")
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("unexpected error"))
	})

	t.Run("When CreateBucket fails with unexpected error, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSController.EXPECT().GetBucketByName("test-bucket").Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "not found", nil))
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, fmt.Errorf("bucket API failed"))
		err := clusterScope.reconcileCOSBucket(ctx, "test-bucket")
		g.Expect(err).ToNot(BeNil())
	})
}

func TestReconcileTransitGateway(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockVPC                *mock.MockVpc
		mockTransitGateway     *tgmock.MockTransitGateway
		mockCtrl               *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
		mockVPC = mock.NewMockVpc(mockCtrl)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("when TransitGatewayID is set in status and returns error getting TransitGateway", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						ID: "transitGatewayID",
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(nil, nil, errors.New("failed to get transit gateway"))
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGateway ID is set in status and already exists but returns error when getting TransitGateway connections", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						ID: "transitGatewayID",
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(nil, nil, errors.New("failed to get transitGateway connections"))
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGatewayID is set in status and TransitGateway not in available state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{
						ID: "transitGatewayID",
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStatePending))}, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When TransitGatewayID is set in spec already exists in cloud and is in available state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "transitGatewayID",
						},
						PowerVSConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
						VPCConnection: infrav1.TransitGatewayConnectionSource{
							Type: infrav1.SourceTypeProvision,
						},
					},
					VPC: infrav1.VPCSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "vpcID",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "workspaceID",
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "workspaceID",
					},
					VPC: infrav1.VPCStatus{
						ID: "vpcID",
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc-conn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending))}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID"), Name: ptr.To("pvs-conn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending))}, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When TransitGatewayID is set in spec and returns error while getting TransitGateway details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "transitGatewayID",
						},
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(nil, nil, errors.New("failed to get transit gateway"))
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGatewayID is not set in spec and fetching using name returns with transit gateway in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "transitGatewayName",
						},
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(&tgapiv1.TransitGateway{Name: ptr.To("transitGatewayName"), ID: ptr.To("transitGatewayID"), Status: ptr.To(string(infrav1.TransitGatewayStateFailed))}, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGatewayID is not set in spec and fetching using name returns with transit gateway in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "transitGatewayName",
						},
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStatePending))}, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("Creates TransitGateway and transitGatewayConnections successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{},
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.Name).To(BeEquivalentTo("transitGatewayName"))
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When PowerVS service Instance and VPC details are not set in status and fails to create transit gateway", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestCheckAndUpdateTransitGatewayConnections(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockVPC                *mock.MockVpc
		mockTransitGateway     *tgmock.MockTransitGateway
		mockCtrl               *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
		mockVPC = mock.NewMockVpc(mockCtrl)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("Returns error when getting VPC details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, errors.New("failed to get vpc"))
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Returns error when getting PowerVS service Instance details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get serviceInstance"))
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGateway connections doesn't exist and creates connections", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
			ID:     ptr.To("vpc-connID"),
			Name:   ptr.To("vpc-conn"),
			Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending)),
		}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
			ID:     ptr.To("pvs-connID"),
			Name:   ptr.To("pvs-conn"),
			Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending)),
		}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When TransitGateway connections doesn't exist and return error while creating PowerVSConnection", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(nil, nil, errors.New("error while creating connections"))
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGateway connections exist and both are in attached state already", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), ID: ptr.To("vpc-connID"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		conn = append(conn, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("pvs"), ID: ptr.To("pvs-connID"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})

	t.Run("WHen PowerVSConnection exist and is in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		conn = append(conn, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID"), Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})

	t.Run("When VPCConnection exist and is in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
			ID:     ptr.To("pvs-connID"),
			Name:   ptr.To("pvs"),
			Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending)),
		}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When VPCConnection status exist and is in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateFailed))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When PowerVSConnection status exist and is in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		conn = append(conn, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID"), Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateFailed))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When PowerVSConnection doesn't exist and creates it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
			ID:     ptr.To("pvs-connID"),
			Name:   ptr.To("pvs-conn"),
			Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending)),
		}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When PowerVSConnection doesn't exist and returns error while creating it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID"), Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(nil, nil, errors.New("failed to create transit gateway connection"))
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When VPCConnection doesn't exist and creates it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID"), Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
			ID:     ptr.To("vpc-connID"),
			Name:   ptr.To("vpc-conn"),
			Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending)),
		}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When VPCConnection doesn't exist and returns error while creating it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(nil, nil, errors.New("failed to create transit gateway connection"))
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestCreateTransitGateway(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockVPC                *mock.MockVpc
		mockTransitGateway     *tgmock.MockTransitGateway
		mockCtrl               *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
		mockVPC = mock.NewMockVpc(mockCtrl)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("when PowerVS serviceInstance ID and VPC ID is not set in Status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Fails to get TransitGateway location and routing", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "zone-ID",
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Return error while creating TransitGateway", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName", Region: "region"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(nil, nil, errors.New("failed to create transit Gateway"))
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Creates TransitGateway but return error when getting VPC details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To("pending")}, nil, nil)
		tg, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(tg).ToNot(BeNil())
		g.Expect(tg.ID).To(Equal(ptr.To("transitGatewayID")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Creates TransitGateway but return error while getting PowerVS details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		tg, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(tg).ToNot(BeNil())
		g.Expect(tg.ID).To(Equal(ptr.To("transitGatewayID")))
		g.Expect(err).To(BeNil())
	})

	t.Run("When PowerVSConnection creation is completed but fails to create VPCConnection", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		tg, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(tg).ToNot(BeNil())
		g.Expect(tg.ID).To(Equal(ptr.To("transitGatewayID")))
		g.Expect(err).To(BeNil())
	})

	t.Run("When local routing is configured but global routing is required", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							GlobalRouting: infrav1.TransitGatewayRoutingLocal,
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "us-east-1",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to create transit gateway"))
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When global routing is set to true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							GlobalRouting: infrav1.TransitGatewayRoutingGlobal,
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
					Zone:          "zone-ID",
					VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "region"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{
						ID: "serviceInstanceID",
					},
					VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		tg, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(tg).ToNot(BeNil())
		g.Expect(tg.ID).To(Equal(ptr.To("transitGatewayID")))
		g.Expect(err).To(BeNil())
	})
}

func makePowerVSClusterScope(mockTransitGateway *tgmock.MockTransitGateway, mockVPC *mock.MockVpc, mockResourceController *mockRC.MockResourceController) ClusterScope {
	clusterScope := ClusterScope{
		TransitGatewayClient: mockTransitGateway,
		IBMVPCClient:         mockVPC,
		ResourceClient:       mockResourceController,
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				TransitGateway: infrav1.TransitGatewaySource{
					PowerVSConnection: infrav1.TransitGatewayConnectionSource{
						Type: infrav1.SourceTypeProvision,
					},
					VPCConnection: infrav1.TransitGatewayConnectionSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
			Status: infrav1.IBMPowerVSClusterStatus{
				TransitGateway: infrav1.TransitGatewayStatus{
					ID: "transitGatewayID",
				},
				Workspace: infrav1.ResourceReference{
					ID: "serviceInstanceID",
				},
				VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
			},
		},
	}

	return clusterScope
}

func TestReconcileVPCSecurityGroups(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	securityGroupID := "securityGroupID"
	securityGroupName := "securityGroupName"

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When Reference SG ID is set and GetSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: securityGroupID}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Reference SG ID is set and SecurityGroup exists", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: securityGroupID}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When Reference SG Name is set and GetSecurityGroupByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{Name: securityGroupName}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Provision SG Name is set and CreateSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: securityGroupName}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Provision SG Name is set and SecurityGroup is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: securityGroupName}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When Provision SG already exists", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSecurityGroupProvision{Name: securityGroupName}},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID}, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When Provision SG is created but CreateSecurityGroupRule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.VPCSecurityGroupProvision{
								Name: securityGroupName,
								Rules: []infrav1.VPCSecurityGroupRule{
									{
										Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
										Source: infrav1.VPCSecurityGroupRulePrototype{
											Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
											Remotes: []infrav1.VPCSecurityGroupRuleRemote{
												{Address: "192.168.0.1", RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to create security group rule"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When unknown source type is provided", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{Type: "unknown"},
					},
				},
			},
		}
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Provision SG inbound rule has empty protocol (zero-value source)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.VPCSecurityGroupProvision{
								Name: securityGroupName,
								Rules: []infrav1.VPCSecurityGroupRule{
									{
										Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
										// Source.Protocol intentionally left empty to simulate a zero-value struct
										Source: infrav1.VPCSecurityGroupRulePrototype{
											Remotes: []infrav1.VPCSecurityGroupRuleRemote{
												{RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("empty protocol"))
	})

	t.Run("When Provision SG outbound rule has no remotes (zero-value destination)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.VPCSecurityGroupProvision{
								Name: securityGroupName,
								Rules: []infrav1.VPCSecurityGroupRule{
									{
										Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
										// Destination.Remotes intentionally left nil
										Destination: infrav1.VPCSecurityGroupRulePrototype{
											Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To(securityGroupID), Name: ptr.To(securityGroupName)}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("no remotes"))
	})
}

func TestCreateVPCSecurityGroupRule(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	securityGroupID := "securityGroupID"
	var portMax int64 = 65535
	var portMin int64 = 1
	protocol := "tcp"
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Creates rule of remoteType Address successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			Address:    "192.168.0.1/24",
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleProtocolIcmptcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "outbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(Equal("ruleID"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Creates rule of remoteType CIDR successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: "CIDRSubnetName",
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: ptr.To("192.168.1.1/24")}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "outbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(Equal("ruleID"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Rule of remoteType CIDR returns error when getting VPC subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: "CIDRSubnetName",
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "outbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Creates rule of remoteType Any successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleProtocolIcmptcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "outbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(Equal("ruleID"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Creates rule of remoteType SG successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: "securityGroupName",
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{CRN: ptr.To("crn"), Name: ptr.To("securityGroupName")}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp{Direction: ptr.To("inbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "inbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(Equal("ruleID"))
		g.Expect(err).To(BeNil())
	})

	t.Run("Rule of remoteType SG returns error while getting securityGroup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: "securityGroupName",
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get security group"))
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "inbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Rule of remoteType SG returns error when SecurityGroup doesn't exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: "securityGroupName",
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, securityGroupID, "inbound", protocol, portMin, portMax, remote)
		g.Expect(ruleID).To(BeEmpty())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestCreateVPCSecurityGroupRules(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Outbound rule with Address remote created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
				Destination: infrav1.VPCSecurityGroupRulePrototype{
					Remotes:   []infrav1.VPCSecurityGroupRuleRemote{{Address: "192.168.0.1/24", RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress}},
					Protocol:  infrav1.VPCSecurityGroupRuleProtocolTCP,
					PortRange: infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535, MinimumPort: 1},
				},
			},
		}
		clusterScope := ClusterScope{IBMVPCClient: mockVPC, IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "securityGroupID")
		g.Expect(ruleIDs).To(Equal([]string{"ruleID"}))
		g.Expect(err).To(BeNil())
	})

	t.Run("Outbound rule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
				Destination: infrav1.VPCSecurityGroupRulePrototype{
					Remotes:  []infrav1.VPCSecurityGroupRuleRemote{{CIDRSubnetName: "CIDRSubnetName", RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeCIDR}},
					Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				},
			},
		}
		clusterScope := ClusterScope{IBMVPCClient: mockVPC, IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "securityGroupID")
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Inbound rule with Address remote created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
				Source: infrav1.VPCSecurityGroupRulePrototype{
					Remotes:   []infrav1.VPCSecurityGroupRuleRemote{{Address: "192.168.0.1/24", RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress}},
					Protocol:  infrav1.VPCSecurityGroupRuleProtocolTCP,
					PortRange: infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535, MinimumPort: 1},
				},
			},
		}
		clusterScope := ClusterScope{IBMVPCClient: mockVPC, IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("inbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "securityGroupID")
		g.Expect(ruleIDs).To(Equal([]string{"ruleID"}))
		g.Expect(err).To(BeNil())
	})

	t.Run("Inbound rule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
				Source: infrav1.VPCSecurityGroupRulePrototype{
					Remotes:  []infrav1.VPCSecurityGroupRuleRemote{{CIDRSubnetName: "CIDRSubnetName", RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeCIDR}},
					Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				},
			},
		}
		clusterScope := ClusterScope{IBMVPCClient: mockVPC, IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "securityGroupID")
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestCreateVPCSecurityGroup(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("SecurityGroup is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID")}, nil, nil)
		sgID, err := clusterScope.createVPCSecurityGroup(ctx, "securityGroupName")
		g.Expect(ptr.Deref(sgID, "")).To(Equal("securityGroupID"))
		g.Expect(err).To(BeNil())
	})

	t.Run("CreateSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create SecurityGroup"))
		sgID, err := clusterScope.createVPCSecurityGroup(ctx, "securityGroupName")
		g.Expect(sgID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestFetchBucketRegionCluster(t *testing.T) {
	testRegion := region
	vpcRegion := "us-east"

	testcases := []struct {
		name                 string
		expectedBucketRegion string
		cos                  infrav1.COSInstanceSource
		vpc                  infrav1.VPCStatus
	}{
		{
			name:                 "Returns bucket region from COS instance when set",
			expectedBucketRegion: testRegion,
			cos:                  infrav1.COSInstanceSource{BucketRegion: testRegion},
			vpc:                  infrav1.VPCStatus{Region: vpcRegion},
		},
		{
			name:                 "Returns VPC region when COS bucket region is not set",
			expectedBucketRegion: vpcRegion,
			cos:                  infrav1.COSInstanceSource{},
			vpc:                  infrav1.VPCStatus{Region: vpcRegion},
		},
		{
			name:                 "Returns empty string when both COS bucket region and VPC region are not set",
			expectedBucketRegion: "",
			cos:                  infrav1.COSInstanceSource{},
			vpc:                  infrav1.VPCStatus{},
		},
		{
			name:                 "Prioritizes COS bucket region over VPC region",
			expectedBucketRegion: testRegion,
			cos:                  infrav1.COSInstanceSource{BucketRegion: testRegion},
			vpc:                  infrav1.VPCStatus{Region: vpcRegion},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			r := fetchBucketRegion(tc.cos, tc.vpc)
			g.Expect(r).To(Equal(tc.expectedBucketRegion))
		})
	}
}

func TestReconcileCOSHMACKey(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Idempotent: Secret already exists, skip CreateResourceKey", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-cos-hmac",
				Namespace: "default",
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existingSecret).Build()

		clusterScope := ClusterScope{
			Client:         fakeClient,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{HMACSecretName: "test-cluster-cos-hmac"},
				},
			},
		}
		// CreateResourceKey must NOT be called
		err := clusterScope.reconcileCOSHMACKey(ctx, "crn:v1:bluemix:public:cloud-object-storage:global::::")
		g.Expect(err).To(BeNil())
	})

	t.Run("CreateResourceKey fails, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		clusterScope := ClusterScope{
			Client:         fakeClient,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
		}
		mockResourceController.EXPECT().CreateResourceKey(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		err := clusterScope.reconcileCOSHMACKey(ctx, "crn:v1:test")
		g.Expect(err).To(MatchError(ContainSubstring("failed to create COS HMAC resource key")))
	})

	t.Run("CreateResourceKey returns key without cos_hmac_keys, return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		clusterScope := ClusterScope{
			Client:         fakeClient,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
		}
		creds := &resourcecontrollerv2.Credentials{}
		rk := &resourcecontrollerv2.ResourceKey{Credentials: creds}
		mockResourceController.EXPECT().CreateResourceKey(gomock.Any()).Return(rk, nil, nil)
		err := clusterScope.reconcileCOSHMACKey(ctx, "crn:v1:test")
		g.Expect(err).To(MatchError(ContainSubstring("cos_hmac_keys property is missing")))
	})

	t.Run("CreateResourceKey succeeds, Secret created, status updated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		clusterScope := ClusterScope{
			Client:         fakeClient,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
		}
		creds := &resourcecontrollerv2.Credentials{}
		creds.SetProperty("cos_hmac_keys", map[string]interface{}{
			"access_key_id":     "AKID123",
			"secret_access_key": "SECRET456",
		})
		rk := &resourcecontrollerv2.ResourceKey{Credentials: creds}
		mockResourceController.EXPECT().CreateResourceKey(gomock.Any()).Return(rk, nil, nil)

		err := clusterScope.reconcileCOSHMACKey(ctx, "crn:v1:test")
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.HMACSecretName).To(Equal("test-cluster-cos-hmac"))

		// Verify the Secret was written
		stored := &corev1.Secret{}
		g.Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-cluster-cos-hmac"}, stored)).To(Succeed())
		g.Expect(string(stored.Data["access_key_id"])).To(Equal("AKID123"))
		g.Expect(string(stored.Data["secret_access_key"])).To(Equal("SECRET456"))
	})
}

func TestDeleteCOSInstanceHMACSecret(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("HMAC Secret is deleted before instance deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster-cos-hmac",
				Namespace: "default",
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existingSecret).Build()

		clusterScope := ClusterScope{
			Client:         fakeClient,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ //nolint:gosec
						ID:             "cosInstanceID",
						HMACSecretName: "my-cluster-cos-hmac",
					},
				},
			},
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{
			ID:    ptr.To("cosInstanceID"),
			State: ptr.To(string(infrav1.WorkspaceStateActive)),
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, nil)

		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())

		// Secret must be gone
		deleted := &corev1.Secret{}
		getErr := fakeClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "my-cluster-cos-hmac"}, deleted)
		g.Expect(getErr).ToNot(BeNil()) // not found
	})
}

func TestName(t *testing.T) {
	testCases := []struct {
		name         string
		clusterScope ClusterScope
		expectedName string
	}{
		{
			name: "Returns cluster name",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: testClusterName},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedName: testClusterName,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.clusterScope.Name()).To(Equal(tc.expectedName))
		})
	}
}

func TestInfraCluster(t *testing.T) {
	testCases := []struct {
		name         string
		clusterScope ClusterScope
		expectedName string
	}{
		{
			name: "Returns IBMPowerVSCluster name",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pvs-cluster"},
				},
			},
			expectedName: "my-pvs-cluster",
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.clusterScope.InfraCluster()).To(Equal(tc.expectedName))
		})
	}
}

func TestAPIServerPort(t *testing.T) {
	testCases := []struct {
		name         string
		clusterScope ClusterScope
		expectedPort int32
	}{
		{
			name: "Returns default API server port when ClusterNetwork not set",
			clusterScope: ClusterScope{
				Cluster:           &clusterv1.Cluster{},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedPort: infrav1.DefaultAPIServerPort,
		},
		{
			name: "Returns custom API server port from ClusterNetwork spec",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						ClusterNetwork: clusterv1.ClusterNetwork{
							APIServerPort: 9443,
						},
					},
				},
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedPort: 9443,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.clusterScope.APIServerPort()).To(Equal(tc.expectedPort))
		})
	}
}

func TestResourceGroupName(t *testing.T) {
	testCases := []struct {
		name         string
		clusterScope ClusterScope
		expectedName string
	}{
		{
			name: "Returns name from status when set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: infrav1.ResourceReference{Name: "status-rg"},
					},
				},
			},
			expectedName: "status-rg",
		},
		{
			name: "Falls back to spec name when status is empty",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: infrav1.ResourceGroupSource{
							Reference: infrav1.ResourceIdentifier{Name: "spec-rg"},
						},
					},
				},
			},
			expectedName: "spec-rg",
		},
		{
			name: "Status name takes precedence over spec name",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: infrav1.ResourceGroupSource{
							Reference: infrav1.ResourceIdentifier{Name: "spec-rg"},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: infrav1.ResourceReference{Name: "status-rg"},
					},
				},
			},
			expectedName: "status-rg",
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			g.Expect(tc.clusterScope.ResourceGroupName()).To(Equal(tc.expectedName))
		})
	}
}

func TestValidateZoneSupportsPER(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Returns error when zone is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: ""}},
		}
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("PowerVS zone is required")))
	})

	t.Run("Returns error when GetDatacenterDetails fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: "dal10"}},
		}
		mockPowerVS.EXPECT().GetDatacenterDetails(gomock.Any(), "dal10").Return(nil, fmt.Errorf("API error"))
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to get datacenter details")))
	})

	t.Run("Returns error when datacenter details is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: "dal10"}},
		}
		mockPowerVS.EXPECT().GetDatacenterDetails(gomock.Any(), "dal10").Return(nil, nil)
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to get datacenter details for zone")))
	})

	t.Run("Returns error when PER capability not in datacenter", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: "dal10"}},
		}
		mockPowerVS.EXPECT().GetDatacenterDetails(gomock.Any(), "dal10").Return(&models.Datacenter{
			Capabilities: map[string]bool{"some-other-cap": true},
		}, nil)
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("capability unknown")))
	})

	t.Run("Returns error when PER is not available for zone", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: "dal10"}},
		}
		mockPowerVS.EXPECT().GetDatacenterDetails(gomock.Any(), "dal10").Return(&models.Datacenter{
			Capabilities: map[string]bool{powerEdgeRouter: false},
		}, nil)
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("is not available for zone")))
	})

	t.Run("Returns nil when PER is available for zone", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Zone: "dal10"}},
		}
		mockPowerVS.EXPECT().GetDatacenterDetails(gomock.Any(), "dal10").Return(&models.Datacenter{
			Capabilities: map[string]bool{powerEdgeRouter: true},
		}, nil)
		err := clusterScope.ValidateZoneSupportsPER(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestReconcileResourceGroup(t *testing.T) {
	var (
		mockResourceManager *mockRM.MockResourceManager
		mockCtrl            *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceManager = mockRM.NewMockResourceManager(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When resource group ID is set in status and GetResourceGroup succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
				},
			},
		}
		mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(
			&resourcemanagerv2.ResourceGroup{ID: ptr.To("rg-id"), Name: ptr.To("rg-name")}, nil, nil,
		)
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.ResourceGroup.ID).To(Equal("rg-id"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.ResourceGroup.Name).To(Equal("rg-name"))
	})

	t.Run("When resource group ID is set in spec and GetResourceGroup succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(
			&resourcemanagerv2.ResourceGroup{ID: ptr.To("rg-id"), Name: ptr.To("rg-name")}, nil, nil,
		)
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.ResourceGroup.ID).To(Equal("rg-id"))
	})

	t.Run("When resource group ID is set and GetResourceGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
				},
			},
		}
		mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch resource group")))
	})

	t.Run("When resource group ID is set and GetResourceGroup returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
				},
			},
		}
		mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(nil, nil, nil)
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("resource group not found")))
	})

	t.Run("When no ID or Name set, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster:     &infrav1.IBMPowerVSCluster{},
		}
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("resource group name is not set")))
	})
}

func TestReconcileWorkspace(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When workspace ID is set in status and is in active state, no requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{ID: ptr.To("ws-id"), Name: ptr.To("ws-name"), State: ptr.To(string(infrav1.WorkspaceStateActive))}, nil, nil,
		)
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace ID is set in status and is in provisioning state, requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{ID: ptr.To("ws-id"), Name: ptr.To("ws-name"), State: ptr.To(string(infrav1.WorkspaceStateProvisioning))}, nil, nil,
		)
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When workspace ID is set in status and GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch workspace")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: "unknown"},
				},
			},
		}
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown workspace source type")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace type is Reference and GetResourceInstanceByFilter succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "ws-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("ws-guid"), Name: ptr.To("ws-name")}, nil,
		)
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Workspace.ID).To(Equal("ws-guid"))
	})

	t.Run("When workspace type is Reference and workspace not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "my-ws"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not found in IBM Cloud")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace type is Provision and idempotency check finds existing workspace", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.WorkspaceProvisionConfig{
							Name: "my-workspace",
						},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("ws-guid"), Name: ptr.To("my-workspace")}, nil,
		)
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Workspace.ID).To(Equal("ws-guid"))
	})

	t.Run("When workspace type is Provision and CreateResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.WorkspaceProvisionConfig{
							Name: "my-workspace",
						},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.ReconcileWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to provision workspace")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestCheckWorkspaceState(t *testing.T) {
	testCases := []struct {
		name           string
		workspaceState string
		expectedReq    bool
		expectError    bool
	}{
		{
			name:           "Active state returns no requeue",
			workspaceState: string(infrav1.WorkspaceStateActive),
			expectedReq:    false,
			expectError:    false,
		},
		{
			name:           "Provisioning state returns requeue",
			workspaceState: string(infrav1.WorkspaceStateProvisioning),
			expectedReq:    true,
			expectError:    false,
		},
		{
			name:           "Failed state returns error",
			workspaceState: string(infrav1.WorkspaceStateFailed),
			expectedReq:    false,
			expectError:    true,
		},
		{
			name:           "Unknown state returns error",
			workspaceState: "weird-state",
			expectedReq:    false,
			expectError:    true,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
			ws := resourcecontrollerv2.ResourceInstance{
				Name:  ptr.To("ws"),
				State: ptr.To(tc.workspaceState),
			}
			requeue, err := clusterScope.checkWorkspaceState(ctx, ws)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).To(BeNil())
			}
			g.Expect(requeue).To(Equal(tc.expectedReq))
		})
	}
}

func TestCreateWorkspace(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.createWorkspace(ctx, "my-workspace")
		g.Expect(err).To(MatchError(ContainSubstring("ID is empty")))
	})

	t.Run("When zone is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		_, err := clusterScope.createWorkspace(ctx, "my-workspace")
		g.Expect(err).To(MatchError(ContainSubstring("PowerVS zone is not set")))
	})

	t.Run("When CreateResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("new-ws-guid"), Name: ptr.To("my-workspace")}, nil, nil,
		)
		ws, err := clusterScope.createWorkspace(ctx, "my-workspace")
		g.Expect(err).To(BeNil())
		g.Expect(ws.GUID).To(Equal(ptr.To("new-ws-guid")))
	})

	t.Run("When CreateResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("creation failed"))
		_, err := clusterScope.createWorkspace(ctx, "my-workspace")
		g.Expect(err).To(MatchError(ContainSubstring("failed to create worksapce")))
	})
}

func TestReconcileNetwork(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When network ID is set in status and GetNetworkByID succeeds (Reference type, no DHCP)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeReference},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When network ID is set but GetNetworkByID returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeReference},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch network by ID")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When network ID is set in status with Provision type and DHCP server ID is missing, requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When network type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: "unknown"},
				},
			},
		}
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown network source type")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When network type is Reference and reference has ID, GetNetworkByID succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "net-id"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal("net-id"))
	})

	t.Run("When network type is Reference and reference has Name, GetNetworkByName succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "net-name"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any(), "net-name").Return(
			&models.NetworkReference{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal("net-id"))
	})

	t.Run("When network type is Reference with no ID or Name, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
		}
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("network reference must contain either an ID or a Name")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When network type is Provision and ListDHCPServers returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}
		mockPowerVS.EXPECT().ListDHCPServers(gomock.Any()).Return(nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch existing DHCP servers")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestCreateDHCPServer(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When CreateDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("create error"))
		_, _, err := clusterScope.createDHCPServer(ctx, "my-dhcp")
		g.Expect(err).To(MatchError(ContainSubstring("create error")))
	})

	t.Run("When CreateDHCPServer returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any(), gomock.Any()).Return(nil, nil)
		_, _, err := clusterScope.createDHCPServer(ctx, "my-dhcp")
		g.Expect(err).To(MatchError(ContainSubstring("DHCP server or its ID is nil")))
	})

	t.Run("When CreateDHCPServer succeeds with CIDR and DNSServer set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{
								CIDR:      "192.168.0.0/24",
								DNSServer: "8.8.8.8",
								Snat:      infrav1.DHCPSnatPolicyDisabled,
							},
						},
					},
				},
			},
		}
		dhcpServer := &models.DHCPServer{
			ID:      ptr.To("dhcp-id"),
			Network: &models.DHCPServerNetwork{ID: ptr.To("net-id"), Name: ptr.To("net-name")},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any(), gomock.Any()).Return(dhcpServer, nil)
		dhcpID, netID, err := clusterScope.createDHCPServer(ctx, "my-dhcp")
		g.Expect(err).To(BeNil())
		g.Expect(dhcpID).To(Equal("dhcp-id"))
		g.Expect(netID).To(Equal("net-id"))
	})

	t.Run("When CreateDHCPServer succeeds with CloudConnectionID set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		cloudConnID := "cloud-conn-uuid-1234"
		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{
								CloudConnectionID: cloudConnID,
							},
						},
					},
				},
			},
		}
		dhcpServer := &models.DHCPServer{
			ID:      ptr.To("dhcp-id-2"),
			Network: &models.DHCPServerNetwork{ID: ptr.To("net-id-2"), Name: ptr.To("net-name-2")},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any(), gomock.Cond(func(x *models.DHCPServerCreate) bool {
			return x != nil && x.CloudConnectionID != nil && *x.CloudConnectionID == cloudConnID
		})).Return(dhcpServer, nil)
		dhcpID, netID, err := clusterScope.createDHCPServer(ctx, "my-dhcp-2")
		g.Expect(err).To(BeNil())
		g.Expect(dhcpID).To(Equal("dhcp-id-2"))
		g.Expect(netID).To(Equal("net-id-2"))
	})
}

func TestReconcileVPC(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When VPC ID is set in status and is in pending state, requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpc-id"), Status: ptr.To(string(infrav1.VPCStatePending))}, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When VPC ID is set in status and is in available state, no requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpc-id"), Status: ptr.To("available")}, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and GetVPC returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch VPC")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Type: "unknown"},
				},
			},
		}
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown VPC source type")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC type is Reference with ID and GetVPC succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Region:    "us-south",
						Reference: infrav1.ResourceIdentifier{ID: "vpc-id"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpc-id"), Name: ptr.To("vpc-name")}, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal("vpc-id"))
	})

	t.Run("When VPC type is Reference with Name and GetVPCByName succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Region:    "us-south",
						Reference: infrav1.ResourceIdentifier{Name: "vpc-name"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("vpc-name").Return(&vpcv1.VPC{ID: ptr.To("vpc-id"), Name: ptr.To("vpc-name")}, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal("vpc-id"))
	})

	t.Run("When VPC type is Reference with no ID or Name, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
		}
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("VPC reference must have either ID or Name")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC type is Provision and VPC already exists by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeProvision,
						Region:    "us-south",
						Provision: infrav1.VPCProvision{Name: "my-vpc"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("my-vpc").Return(&vpcv1.VPC{ID: ptr.To("vpc-id"), Name: ptr.To("my-vpc")}, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal("vpc-id"))
	})

	t.Run("When VPC type is Provision and CreateVPC succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeProvision,
						Region:    "us-south",
						Provision: infrav1.VPCProvision{Name: "my-vpc"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("my-vpc").Return(nil, nil)
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(&vpcv1.VPC{
			ID:                   ptr.To("new-vpc-id"),
			Name:                 ptr.To("my-vpc"),
			DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("sg-id")},
		}, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal("new-vpc-id"))
	})
}

func TestReconcileVPCSubnets(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When topology is VirtualIP, subnets are skipped", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
				},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC region is missing from spec, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("VPC region is missing")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When subnet already tracked in status and GetSubnet verifies it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "my-subnet"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("my-subnet")}, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When Reference subnet by ID GetSubnet succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "my-subnet"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("my-subnet"), Zone: &vpcv1.ZoneReference{Name: ptr.To("us-south-1")}},
			nil, nil,
		)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnets).To(HaveLen(1))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnets[0].ID).To(Equal("subnet-id"))
	})

	t.Run("When unknown subnet type, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: "unknown", Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown VPC subnet source type")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileSubnetReference(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When ref has no ID or Name, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.reconcileSubnetReference(infrav1.ResourceIdentifier{})
		g.Expect(err).To(MatchError(ContainSubstring("ID or Name defined")))
	})

	t.Run("When ref has ID and GetSubnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.reconcileSubnetReference(infrav1.ResourceIdentifier{ID: "subnet-id"})
		g.Expect(err).To(MatchError(ContainSubstring("failed fetching referenced subnet")))
	})

	t.Run("When ref has ID and subnet found successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("my-subnet")}, nil, nil,
		)
		subnet, err := clusterScope.reconcileSubnetReference(infrav1.ResourceIdentifier{ID: "subnet-id"})
		g.Expect(err).To(BeNil())
		g.Expect(subnet.ID).To(Equal(ptr.To("subnet-id")))
	})

	t.Run("When ref has Name and GetVPCSubnetByName succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCSubnetByName("my-subnet").Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("my-subnet")}, nil,
		)
		subnet, err := clusterScope.reconcileSubnetReference(infrav1.ResourceIdentifier{Name: "my-subnet"})
		g.Expect(err).To(BeNil())
		g.Expect(subnet.Name).To(Equal(ptr.To("my-subnet")))
	})
}

func TestReconcileSubnetProvision(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When subnet already exists by name, return it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCSubnetByName("my-subnet").Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("my-subnet")}, nil,
		)
		subnet, err := clusterScope.reconcileSubnetProvision("my-subnet", "us-south-1")
		g.Expect(err).To(BeNil())
		g.Expect(subnet.ID).To(Equal(ptr.To("subnet-id")))
	})

	t.Run("When subnet not found, creates new subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPCSubnetByName("my-subnet").Return(nil, nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("new-subnet-id"), Name: ptr.To("my-subnet")}, nil, nil,
		)
		subnet, err := clusterScope.reconcileSubnetProvision("my-subnet", "us-south-1")
		g.Expect(err).To(BeNil())
		g.Expect(subnet.ID).To(Equal(ptr.To("new-subnet-id")))
	})
}

func TestUpdateSubnetStatusList(t *testing.T) {
	testCases := []struct {
		name           string
		existingStatus []infrav1.VPCSubnetStatus
		newStatus      infrav1.VPCSubnetStatus
		expectedLen    int
		expectedID     string
	}{
		{
			name:           "Adds new entry when not present",
			existingStatus: []infrav1.VPCSubnetStatus{},
			newStatus:      infrav1.VPCSubnetStatus{ID: "subnet-id", Name: "my-subnet"},
			expectedLen:    1,
			expectedID:     "subnet-id",
		},
		{
			name: "Updates existing entry by name",
			existingStatus: []infrav1.VPCSubnetStatus{
				{ID: "old-id", Name: "my-subnet"},
			},
			newStatus:   infrav1.VPCSubnetStatus{ID: "new-id", Name: "my-subnet"},
			expectedLen: 1,
			expectedID:  "new-id",
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnets: tc.existingStatus,
					},
				},
			}
			clusterScope.updateSubnetStatusList(tc.newStatus)
			g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnets).To(HaveLen(tc.expectedLen))
			g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnets[0].ID).To(Equal(tc.expectedID))
		})
	}
}

func TestSetLoadBalancerStatus(t *testing.T) {
	testCases := []struct {
		name         string
		existingLBs  []infrav1.LoadBalancerStatus
		newStatus    infrav1.LoadBalancerStatus
		expectedLen  int
		expectedName string
		expectedID   string
	}{
		{
			name:         "Appends new entry when nil slice",
			existingLBs:  nil,
			newStatus:    infrav1.LoadBalancerStatus{Name: "lb1", ID: "lb-id-1"},
			expectedLen:  1,
			expectedName: "lb1",
			expectedID:   "lb-id-1",
		},
		{
			name: "Updates existing entry by name",
			existingLBs: []infrav1.LoadBalancerStatus{
				{Name: "lb1", ID: "old-id"},
			},
			newStatus:    infrav1.LoadBalancerStatus{Name: "lb1", ID: "new-id"},
			expectedLen:  1,
			expectedName: "lb1",
			expectedID:   "new-id",
		},
		{
			name: "Appends new entry when name not found",
			existingLBs: []infrav1.LoadBalancerStatus{
				{Name: "lb1", ID: "lb-id-1"},
			},
			newStatus:    infrav1.LoadBalancerStatus{Name: "lb2", ID: "lb-id-2"},
			expectedLen:  2,
			expectedName: "lb2",
			expectedID:   "lb-id-2",
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: tc.existingLBs,
					},
				},
			}
			clusterScope.SetLoadBalancerStatus(ctx, tc.newStatus.Name, tc.newStatus)
			g.Expect(clusterScope.IBMPowerVSCluster.Status.LoadBalancers).To(HaveLen(tc.expectedLen))
			// Last entry should match the new status name/ID
			last := clusterScope.IBMPowerVSCluster.Status.LoadBalancers[tc.expectedLen-1]
			g.Expect(last.Name).To(Equal(tc.expectedName))
			g.Expect(last.ID).To(Equal(tc.expectedID))
		})
	}
}

func TestCheckLoadBalancerPort(t *testing.T) {
	testCases := []struct {
		name        string
		lbName      string
		prov        infrav1.LoadBalancerProvision
		cluster     *clusterv1.Cluster
		expectError bool
	}{
		{
			name:    "No additional listeners, no error",
			lbName:  "lb1",
			prov:    infrav1.LoadBalancerProvision{},
			cluster: &clusterv1.Cluster{},
		},
		{
			name:   "Additional listener uses different port, no error",
			lbName: "lb1",
			prov: infrav1.LoadBalancerProvision{
				AdditionalListeners: []infrav1.AdditionalListener{{Port: 9090}},
			},
			cluster:     &clusterv1.Cluster{},
			expectError: false,
		},
		{
			name:   "Additional listener uses API server port, returns error",
			lbName: "lb1",
			prov: infrav1.LoadBalancerProvision{
				AdditionalListeners: []infrav1.AdditionalListener{{Port: int64(infrav1.DefaultAPIServerPort)}},
			},
			cluster:     &clusterv1.Cluster{},
			expectError: true,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := ClusterScope{
				Cluster:           tc.cluster,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			}
			err := clusterScope.checkLoadBalancerPort(tc.lbName, tc.prov)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("cannot be used as an additional listener port"))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestCheckLoadBalancer(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When load balancer not found, returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("lb1").Return(nil, nil)
		status, err := clusterScope.checkLoadBalancer(ctx, "lb1")
		g.Expect(err).To(BeNil())
		g.Expect(status).To(BeNil())
	})

	t.Run("When GetLoadBalancerByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("lb1").Return(nil, fmt.Errorf("API error"))
		status, err := clusterScope.checkLoadBalancer(ctx, "lb1")
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch load balancer details")))
		g.Expect(status).To(BeNil())
	})

	t.Run("When load balancer found, returns status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("lb1").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb1"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		status, err := clusterScope.checkLoadBalancer(ctx, "lb1")
		g.Expect(err).To(BeNil())
		g.Expect(status).ToNot(BeNil())
		g.Expect(status.ID).To(Equal("lb-id"))
	})
}

func TestCheckLoadBalancerState(t *testing.T) {
	testCases := []struct {
		name            string
		lbStatus        string
		expectedIsReady bool
	}{
		{
			name:            "Active state returns true",
			lbStatus:        string(infrav1.LoadBalancerStateActive),
			expectedIsReady: true,
		},
		{
			name:            "CreatePending state returns false",
			lbStatus:        string(infrav1.LoadBalancerStateCreatePending),
			expectedIsReady: false,
		},
		{
			name:            "UpdatePending state returns false",
			lbStatus:        string(infrav1.LoadBalancerStateUpdatePending),
			expectedIsReady: false,
		},
		{
			name:            "Unknown state returns false",
			lbStatus:        "unknown",
			expectedIsReady: false,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
			lb := vpcv1.LoadBalancer{
				Name:               ptr.To("lb1"),
				ProvisioningStatus: ptr.To(tc.lbStatus),
			}
			g.Expect(clusterScope.checkLoadBalancerState(ctx, lb)).To(Equal(tc.expectedIsReady))
		})
	}
}

func TestCreateLoadBalancer(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.createLoadBalancer(ctx, "lb1", infrav1.LoadBalancerProvision{})
		g.Expect(err).To(MatchError(ContainSubstring("ID is empty")))
	})

	t.Run("When no VPC subnets in status, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		_, err := clusterScope.createLoadBalancer(ctx, "lb1", infrav1.LoadBalancerProvision{Type: infrav1.LoadBalancerTypePublic})
		g.Expect(err).To(MatchError(ContainSubstring("no VPC subnets are present")))
	})

	t.Run("When CreateLoadBalancer succeeds with private LB and additional listener", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		prov := infrav1.LoadBalancerProvision{
			Type:                infrav1.LoadBalancerTypePrivate,
			AdditionalListeners: []infrav1.AdditionalListener{{Port: 9090}},
		}
		mockVPC.EXPECT().CreateLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb1"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
		}, nil, nil)
		status, err := clusterScope.createLoadBalancer(ctx, "lb1", prov)
		g.Expect(err).To(BeNil())
		g.Expect(status.ID).To(Equal("lb-id"))
	})

	t.Run("When CreateLoadBalancer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().CreateLoadBalancer(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.createLoadBalancer(ctx, "lb1", infrav1.LoadBalancerProvision{})
		g.Expect(err).To(MatchError(ContainSubstring("failed to create load balancer")))
	})
}

func TestReconcileLoadBalancers(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When no LBs in spec, default public LB is provisioned and found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{{ID: "subnet-id", Name: "my-subnet"}},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("my-cluster-lb-public"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
	})

	t.Run("When LB ID is cached in status and LB is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "lb1", Type: infrav1.LoadBalancerTypePublic},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "lb1", ID: "lb-id"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb1"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
	})

	t.Run("When LB type is Reference by ID and LB is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "lb-id"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb1"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
	})

	t.Run("When LB type is Reference with no ID or Name, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{},
						},
					},
				},
			},
		}
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("must have either an ID or Name")))
		g.Expect(ready).To(BeFalse())
	})
}

func TestDeleteWorkspace(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When workspace type is not Provision, skip deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeReference},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace ID is empty, skip deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeProvision},
				},
			},
		}
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch PowerVS workspace")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace is already removed, no deletion needed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateRemoved))}, nil, nil,
		)
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace exists and DeleteResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateActive))}, nil, nil,
		)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When workspace exists and DeleteResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Workspace: infrav1.WorkspaceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateActive))}, nil, nil,
		)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, fmt.Errorf("delete error"))
		requeue, err := clusterScope.DeleteWorkspace(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to delete PowerVS workspace")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestDeleteVPCSubnets(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When no subnets in status, nothing to delete", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When subnet type is Reference (not managed), skip it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When subnet not found (404), treated as deleted", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, errors.New("not found"))
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When GetSubnet returns non-404 error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, errors.New("API error"))
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When subnet is in deleting state, requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Status: ptr.To(string(infrav1.VPCSubnetStateDeleting))}, nil, nil,
		)
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When DeleteSubnet succeeds, requeue to confirm deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Status: ptr.To("available")}, nil, nil,
		)
		mockVPC.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When spec VPC subnets are empty, all status subnets are treated as managed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "auto-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Status: ptr.To("available")}, nil, nil,
		)
		mockVPC.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		requeue, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
}

func TestFindConnectionByRef(t *testing.T) {
	conn1 := tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("conn-id-1"), Name: ptr.To("conn-name-1")}
	conn2 := tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("conn-id-2"), Name: ptr.To("conn-name-2")}
	conns := []tgapiv1.TransitGatewayConnectionCust{conn1, conn2}

	testCases := []struct {
		name        string
		ref         infrav1.ResourceIdentifier
		expectFound bool
		expectedID  string
	}{
		{
			name:        "Match by ID",
			ref:         infrav1.ResourceIdentifier{ID: "conn-id-1"},
			expectFound: true,
			expectedID:  "conn-id-1",
		},
		{
			name:        "Match by Name",
			ref:         infrav1.ResourceIdentifier{Name: "conn-name-2"},
			expectFound: true,
			expectedID:  "conn-id-2",
		},
		{
			name:        "No match",
			ref:         infrav1.ResourceIdentifier{ID: "no-such-id"},
			expectFound: false,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			result := findConnectionByRef(conns, tc.ref)
			if tc.expectFound {
				g.Expect(result).ToNot(BeNil())
				g.Expect(*result.ID).To(Equal(tc.expectedID))
			} else {
				g.Expect(result).To(BeNil())
			}
		})
	}
}

func TestReconcileConnectionReference(t *testing.T) {
	conn1 := tgapiv1.TransitGatewayConnectionCust{
		ID:        ptr.To("conn-id"),
		Name:      ptr.To("conn-name"),
		Status:    ptr.To(string(infrav1.TransitGatewayConnectionStateAttached)),
		NetworkID: ptr.To("net-crn"),
	}

	t.Run("When connection not found in existing conns, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		requeue, err := clusterScope.reconcileConnectionReference(
			ctx,
			infrav1.ResourceIdentifier{ID: "no-such"},
			ptr.To("net-crn"),
			vpcNetworkConnectionType,
			[]tgapiv1.TransitGatewayConnectionCust{conn1},
		)
		g.Expect(err).To(MatchError(ContainSubstring("not found on Transit Gateway")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When connection found but network ID doesn't match, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		requeue, err := clusterScope.reconcileConnectionReference(
			ctx,
			infrav1.ResourceIdentifier{ID: "conn-id"},
			ptr.To("wrong-crn"),
			vpcNetworkConnectionType,
			[]tgapiv1.TransitGatewayConnectionCust{conn1},
		)
		g.Expect(err).To(MatchError(ContainSubstring("wrong network CRN")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When connection found and network ID matches, returns status", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		requeue, err := clusterScope.reconcileConnectionReference(
			ctx,
			infrav1.ResourceIdentifier{ID: "conn-id"},
			ptr.To("net-crn"),
			vpcNetworkConnectionType,
			[]tgapiv1.TransitGatewayConnectionCust{conn1},
		)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse()) // attached state → no requeue
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(Equal("conn-id"))
	})
}

func TestResolveTransitGatewayReference(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Resolves by ID successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(
			&tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("tg-name")}, nil, nil,
		)
		tg, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{ID: "tg-id"})
		g.Expect(err).To(BeNil())
		g.Expect(tg.ID).To(Equal(ptr.To("tg-id")))
	})

	t.Run("Resolves by Name successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		mockTransitGateway.EXPECT().GetTransitGatewayByName("tg-name").Return(
			&tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("tg-name")}, nil,
		)
		tg, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{Name: "tg-name"})
		g.Expect(err).To(BeNil())
		g.Expect(tg.Name).To(Equal(ptr.To("tg-name")))
	})

	t.Run("Returns error when name not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		mockTransitGateway.EXPECT().GetTransitGatewayByName("tg-name").Return(nil, nil)
		_, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{Name: "tg-name"})
		g.Expect(err).To(MatchError(ContainSubstring("not found")))
	})

	t.Run("Returns error when ref has neither ID nor Name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{})
		g.Expect(err).To(MatchError(ContainSubstring("must have either ID or Name")))
	})
}

// Additional tests added to raise coverage toward ~91%.

func TestReconcileNetworkProvision(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When ListDHCPServers returns a server matching expected network name (idempotency recovery)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}
		// dhcpNetworkName("my-cluster-dhcp") → "DHCPSERVER<name>_Private"
		expectedNet := dhcpNetworkName(ResourceName("my-cluster", ResourceTypeDHCP, ""))
		mockPowerVS.EXPECT().ListDHCPServers(gomock.Any()).Return(models.DHCPServers{
			{
				ID: ptr.To("dhcp-id"),
				Network: &models.DHCPServerNetwork{
					ID:   ptr.To("net-id"),
					Name: ptr.To(expectedNet),
				},
				Status: ptr.To("ACTIVE"),
			},
		}, nil)
		requeue, err := clusterScope.reconcileNetworkProvision(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.DHCPServer.ID).To(Equal("dhcp-id"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal("net-id"))
	})

	t.Run("When ListDHCPServers returns a malformed record (nil ID), it is skipped and create is called", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{Name: "my-dhcp"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
					Workspace:     infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		expectedNet := dhcpNetworkName("my-dhcp")
		// Malformed record: Network.Name matches but Network.ID is nil → should be skipped
		mockPowerVS.EXPECT().ListDHCPServers(gomock.Any()).Return(models.DHCPServers{
			{
				ID: ptr.To("dhcp-id"),
				Network: &models.DHCPServerNetwork{
					ID:   nil, // malformed: Network.ID is nil
					Name: ptr.To(expectedNet),
				},
				Status: ptr.To("ACTIVE"),
			},
		}, nil)
		// After skipping, createDHCPServer is called
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any(), gomock.Any()).Return(&models.DHCPServer{
			ID: ptr.To("new-dhcp-id"),
			Network: &models.DHCPServerNetwork{
				ID:   ptr.To("new-net-id"),
				Name: ptr.To(expectedNet),
			},
		}, nil)
		requeue, err := clusterScope.reconcileNetworkProvision(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.DHCPServer.ID).To(Equal("new-dhcp-id"))
	})
}

func TestReconcileConnection(t *testing.T) {
	t.Run("When connection source type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		connSpec := infrav1.TransitGatewayConnectionSource{
			Type: "bogus-type",
		}
		networkID := ptr.To("net-id")
		requeue, err := clusterScope.reconcileConnection(ctx, nil, connSpec, networkID, vpcNetworkConnectionType, nil)
		g.Expect(err).To(MatchError(ContainSubstring("unknown connection source type")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestCheckTransitGatewayStatus(t *testing.T) {
	t.Run("When tg.Status is nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		tg := &tgapiv1.TransitGateway{Name: ptr.To("tg"), Status: nil}
		requeue, err := clusterScope.checkTransitGatewayStatus(ctx, tg)
		g.Expect(err).To(MatchError(ContainSubstring("missing Status")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When tg.Status is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		tg := &tgapiv1.TransitGateway{Name: ptr.To("tg"), Status: ptr.To("weird-state")}
		requeue, err := clusterScope.checkTransitGatewayStatus(ctx, tg)
		g.Expect(err).To(MatchError(ContainSubstring("unknown state")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestCheckTransitGatewayConnectionStatus(t *testing.T) {
	t.Run("When connection is in an unknown state, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		conn := &tgapiv1.TransitGatewayConnectionCust{
			Name:   ptr.To("conn"),
			Status: ptr.To("some-unknown-state"),
		}
		requeue, err := clusterScope.checkTransitGatewayConnectionStatus(ctx, conn)
		g.Expect(err).To(MatchError(ContainSubstring("unknown state")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When connection or status is nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		requeue, err := clusterScope.checkTransitGatewayConnectionStatus(ctx, nil)
		g.Expect(err).To(MatchError(ContainSubstring("nil")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileWorkspaceProvision(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When createWorkspace returns a workspace with nil GUID, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-east-1",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		// GetResourceInstanceByFilter returns nil → skip idempotency path
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		// CreateResourceInstance returns instance with nil GUID
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: nil}, nil, nil,
		)
		requeue, err := clusterScope.reconcileWorkspaceProvision(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("nil")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When workspace already exists via idempotency check, requeue with status set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-east-1",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}
		// GetResourceInstanceByFilter returns an existing workspace
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("existing-ws-guid")}, nil,
		)
		requeue, err := clusterScope.reconcileWorkspaceProvision(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Workspace.ID).To(Equal("existing-ws-guid"))
	})
}

func TestReconcileTransitGatewayAdditional(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When topology is VirtualIP, ReconcileTransitGateway is skipped", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
				},
			},
		}
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When TG type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: "unknown-type",
					},
				},
			},
		}
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown transit gateway source type")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When TG ID set in status but tg is nil from cloud, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: infrav1.TransitGatewayStatus{ID: "tg-id"},
				},
			},
		}
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not found")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileResourceGroupNameNil(t *testing.T) {
	var (
		mockResourceManager *mockRM.MockResourceManager
		mockCtrl            *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceManager = mockRM.NewMockResourceManager(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When resource group is found but Name is nil, ID is still saved to status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceManagerClient: mockResourceManager,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
				},
			},
		}
		// IBM Cloud returns a group with no Name
		mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(
			&resourcemanagerv2.ResourceGroup{ID: ptr.To("rg-id"), Name: nil}, nil, nil,
		)
		err := clusterScope.ReconcileResourceGroup(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.ResourceGroup.ID).To(Equal("rg-id"))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.ResourceGroup.Name).To(BeEmpty())
	})
}

func TestReconcileNetworkDHCPActivePath(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When network ID is set and Provision type, DHCP is active, no requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						ID: "net-id",
						DHCPServer: infrav1.ResourceReference{
							ID: "dhcp-id",
						},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		// GetDHCPServer returns active state
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), "dhcp-id").Return(
			&models.DHCPServerDetail{
				ID:     ptr.To("dhcp-id"),
				Status: ptr.To(string(infrav1.DHCPServerStateActive)),
			}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileVPCSubnetsAutoExpand(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When spec has no subnets, auto-expanded subnets are provisioned (anyPending=true)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					// VPCSubnets is intentionally empty
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
					VPC:           infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		// us-south has 3 zones; expect one CreateSubnet per zone
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil).AnyTimes()
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(&vpcv1.Subnet{
			ID:   ptr.To("subnet-id"),
			Name: ptr.To("subnet-name"),
			Zone: &vpcv1.ZoneReference{Name: ptr.To("us-south-1")},
		}, nil, nil).AnyTimes()
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue()) // anyPending because provision was triggered
	})

	t.Run("When status has tracked subnet but GetSubnet returns nil details, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "my-subnet"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		// GetSubnet returns nil details (subnet was deleted)
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not found in IBM Cloud")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileVPCSecurityGroupProvisionSGNil(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetSecurityGroupByName returns nil and createVPCSecurityGroup returns nil ID, GetSecurityGroup is called", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		// No existing SG found
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		// Create succeeds, returns an ID
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(
			&vpcv1.SecurityGroup{ID: ptr.To("sg-id"), Name: ptr.To("sg-name")}, nil, nil,
		)
		// GetSecurityGroup fetches the full object
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(
			&vpcv1.SecurityGroup{ID: ptr.To("sg-id"), Name: ptr.To("sg-name")}, nil, nil,
		)
		prov := infrav1.VPCSecurityGroupProvision{Name: "sg-name"}
		status, err := clusterScope.reconcileVPCSecurityGroupProvision(ctx, prov)
		g.Expect(err).To(BeNil())
		g.Expect(status).NotTo(BeNil())
		g.Expect(status.ID).To(Equal("sg-id"))
	})
}

func TestDeleteLoadBalancerByName(t *testing.T) {
	var (
		mockVpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When LB ID is not in status, falls back to GetLoadBalancerByName and deletes", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "my-lb"},
						},
					},
				},
				// Status.LoadBalancers is intentionally empty (no cached ID)
			},
		}
		mockVpc.EXPECT().GetLoadBalancerByName("my-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("my-lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When GetLoadBalancerByName returns nil (not found), nothing to delete", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "my-lb"},
						},
					},
				},
			},
		}
		mockVpc.EXPECT().GetLoadBalancerByName("my-lb").Return(nil, nil)
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When LB type is Reference, it is skipped during deletion", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "external-lb"},
						},
					},
				},
			},
		}
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestDeleteVPCNilDetails(t *testing.T) {
	var (
		mockVpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetVPC returns nil details (no error), VPC is considered gone", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		// Returns nil details with no error
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.DeleteVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(BeEmpty())
	})
}

func TestSetupCOSClientAlreadyInitialized(t *testing.T) {
	t.Run("When COSClient is already set, setupCOSClient is a no-op", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		mockCOS := mockcos.NewMockCos(mockCtrl)

		clusterScope := ClusterScope{
			COSClient:         mockCOS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		// No IBM Cloud calls expected
		err := clusterScope.setupCOSClient(ctx, "instance-id", "us-south")
		g.Expect(err).To(BeNil())
		// Client must still be the same mock (not replaced)
		g.Expect(clusterScope.COSClient).To(Equal(mockCOS))
	})
}

func TestReconcileCOSInstanceAdditional(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When status ID is set but instance is not active, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				Name:  ptr.To("my-cos"),
				State: ptr.To(string(infrav1.WorkspaceStateProvisioning)),
			}, nil, nil,
		)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not active")))
	})

	t.Run("When status ID is set but GetResourceInstance returns nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not found in cloud")))
	})

	t.Run("When BucketRegion and VPCRegion are both empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type:         infrav1.SourceTypeReference,
						BucketRegion: "", // no bucket region in spec
						Reference:    infrav1.ResourceIdentifier{ID: "cos-ref-id"},
					},
				},
				// Status.VPC.Region is also empty → should error on bucket region
			},
		}
		// reconcileCOSReference will call GetResourceInstance to fetch by ID
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("cos-guid"),
				Name:  ptr.To("my-cos"),
				State: ptr.To(string(infrav1.WorkspaceStateActive)),
				CRN:   ptr.To("crn:v1:bluemix:public:cloud-object-storage:global:a/abc:cos-guid::"),
			}, nil, nil,
		)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to determine COS bucket region")))
	})
}

func TestFetchVPCCRNNilCRN(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetVPC returns details with nil CRN, fetchVPCCRN still returns nil CRN", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		// CRN is nil in the response
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(
			&vpcv1.VPC{ID: ptr.To("vpc-id"), CRN: nil}, nil, nil,
		)
		crn, err := clusterScope.fetchVPCCRN()
		g.Expect(err).To(BeNil())
		g.Expect(crn).To(BeNil())
	})
}

func TestFetchPowerVSWorkspaceCRNNil(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetResourceInstance returns nil CRN, fetchPowerVSWorkspaceCRN returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{CRN: nil}, nil, nil,
		)
		crn, err := clusterScope.fetchPowerVSWorkspaceCRN()
		g.Expect(err).To(MatchError(ContainSubstring("CRN is empty")))
		g.Expect(crn).To(BeNil())
	})
}

func TestReconcileVPCProvisionError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetVPCByName returns an error, reconcileVPCProvision returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeProvision,
						Region:    "us-south",
						Provision: infrav1.VPCProvision{Name: "my-vpc"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("my-vpc").Return(nil, fmt.Errorf("API error"))
		requeue, err := clusterScope.reconcileVPCProvision(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to check if VPC exists")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileVPCReferenceNilDetails(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetVPCByName returns nil details, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "my-vpc"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("my-vpc").Return(nil, nil)
		requeue, err := clusterScope.reconcileVPCReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("nil")))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When GetVPC returns VPC with nil ID, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "vpc-id"},
					},
				},
			},
		}
		// Returns VPC with nil ID
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: nil, Name: nil}, nil, nil)
		requeue, err := clusterScope.reconcileVPCReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("nil")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileLoadBalancersByName(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When LB type is Reference by Name and LB is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "ref-lb"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("ref-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("ref-lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
	})

	t.Run("When LB ID is cached but GetLoadBalancer returns nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "my-lb"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "my-lb", ID: "lb-id"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("empty/nil response")))
		g.Expect(ready).To(BeFalse())
	})
}

func TestReconcileLoadBalancersCheckPath(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When provision LB has no cached ID but checkLoadBalancer finds it by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "found-lb", Type: infrav1.LoadBalancerTypePublic},
						},
					},
				},
				// No LB ID in Status so it will call checkLoadBalancer
			},
		}
		// GetLoadBalancerByName returns an existing LB (idempotency recovery path)
		mockVPC.EXPECT().GetLoadBalancerByName("found-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("found-lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.LoadBalancers).To(HaveLen(1))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.LoadBalancers[0].ID).To(Equal("lb-id"))
	})

	t.Run("When provision LB not found and is created, isAnyLoadBalancerNotReady returns false (not ready)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "new-lb", Type: infrav1.LoadBalancerTypePublic},
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{{ID: "subnet-id", Name: "my-subnet"}},
				},
			},
		}
		// checkLoadBalancer → not found
		mockVPC.EXPECT().GetLoadBalancerByName("new-lb").Return(nil, nil)
		// createLoadBalancer → succeeds
		mockVPC.EXPECT().CreateLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("new-lb-id"),
			Name:               ptr.To("new-lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
		}, nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		// isAnyLoadBalancerNotReady = true (just created), so returns false (not ready yet)
		g.Expect(ready).To(BeFalse())
	})
}

func TestReconcileVPCSecurityGroupProvisionGetSGError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When createVPCSecurityGroup succeeds but GetSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(
			&vpcv1.SecurityGroup{ID: ptr.To("sg-id"), Name: ptr.To("sg-name")}, nil, nil,
		)
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, fmt.Errorf("fetch error"))
		prov := infrav1.VPCSecurityGroupProvision{Name: "sg-name"}
		status, err := clusterScope.reconcileVPCSecurityGroupProvision(ctx, prov)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch newly created security group")))
		g.Expect(status).To(BeNil())
	})
}

func TestCreateVPCSubnetNilResponse(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When CreateSubnet returns nil subnetDetails, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(nil, nil, nil)
		subnet, err := clusterScope.createVPCSubnet("my-subnet", "us-south-1")
		g.Expect(err).To(MatchError(ContainSubstring("nil response")))
		g.Expect(subnet).To(BeNil())
	})

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		subnet, err := clusterScope.createVPCSubnet("my-subnet", "us-south-1")
		g.Expect(err).To(MatchError(ContainSubstring("resource group ID is empty")))
		g.Expect(subnet).To(BeNil())
	})
}

func TestReconcileConnectionViaReferenceType(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When connection type is Reference, routes to reconcileConnectionReference", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		networkID := ptr.To("net-crn")
		existingConns := []tgapiv1.TransitGatewayConnectionCust{
			{
				ID:        ptr.To("conn-id"),
				Name:      ptr.To("conn-name"),
				Status:    ptr.To(string(infrav1.TransitGatewayConnectionStateAttached)),
				NetworkID: networkID,
			},
		}
		connSpec := infrav1.TransitGatewayConnectionSource{
			Type:      infrav1.SourceTypeReference,
			Reference: infrav1.ResourceIdentifier{ID: "conn-id"},
		}
		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		requeue, err := clusterScope.reconcileConnection(ctx, nil, connSpec, networkID, vpcNetworkConnectionType, existingConns)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileLoadBalancersReferenceNilLB(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When LB type is Reference by Name but GetLoadBalancerByName returns nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "missing-lb"},
						},
					},
				},
			},
		}
		// Returns nil — means the LB was not found
		mockVPC.EXPECT().GetLoadBalancerByName("missing-lb").Return(nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch referenced load balancer")))
		g.Expect(ready).To(BeFalse())
	})
}

func TestReconcileSubnetProvisionError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetVPCSubnetByName returns error, reconcileSubnetProvision returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCSubnetByName("my-subnet").Return(nil, fmt.Errorf("API error"))
		subnet, err := clusterScope.reconcileSubnetProvision("my-subnet", "us-south-1")
		g.Expect(err).To(MatchError(ContainSubstring("failed checking subnet presence")))
		g.Expect(subnet).To(BeNil())
	})
}

func TestReconcileWorkspaceProvisionCreateError(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When createWorkspace returns an error, reconcileWorkspaceProvision returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-east-1",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(
			nil, nil, fmt.Errorf("creation failed"),
		)
		requeue, err := clusterScope.reconcileWorkspaceProvision(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to provision workspace")))
		g.Expect(requeue).To(BeFalse())
	})
}

func TestReconcileVPCSecurityGroupProvisionSGByNameError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetSecurityGroupByName returns a non-NotFound error, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		// Return a plain error (not SecurityGroupByNameNotFound) to hit the non-nil error return path
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, fmt.Errorf("unexpected API error"))
		prov := infrav1.VPCSecurityGroupProvision{Name: "sg-name"}
		status, err := clusterScope.reconcileVPCSecurityGroupProvision(ctx, prov)
		g.Expect(err).To(MatchError(ContainSubstring("failed to query VPC security group by name")))
		g.Expect(status).To(BeNil())
	})
}

func TestReconcileLoadBalancersCachedLBNotActive(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When LB ID cached but GetLoadBalancer returns not-active LB, isAnyLoadBalancerNotReady = true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "lb1"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "lb1", ID: "lb-id"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb1"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
		}, nil, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		// Not ready yet, so returns false
		g.Expect(ready).To(BeFalse())
	})

	t.Run("When LB auto-name is generated (empty provision name) and found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "auto-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "", // empty name → auto-generate
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
					},
				},
			},
		}
		// auto-generated name will be resolved via checkLoadBalancer → GetLoadBalancerByName
		mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("auto-lb-id"),
			Name:               ptr.To("auto-cluster-lb-public"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil)
		ready, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(ready).To(BeTrue())
	})
}

// TestReconcileResourceGroupMissingBranches covers the missing branches in ReconcileResourceGroup.
func TestReconcileResourceGroupMissingBranches(t *testing.T) {
	testCases := []struct {
		name        string
		setupScope  func(t *testing.T) *ClusterScope
		expectedErr string
	}{
		{
			name: "When resource group ID is set in Status and GetResourceGroup returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRM := mockRM.NewMockResourceManager(mockCtrl)
				mockRM.EXPECT().GetResourceGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get resource group"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Status: infrav1.IBMPowerVSClusterStatus{
							ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
						},
					},
					ResourceManagerClient: mockRM,
				}
			},
			expectedErr: "failed to fetch resource group (id: rg-id) details",
		},
		{
			name: "When resource group ID is set in Status and GetResourceGroup returns nil",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRM := mockRM.NewMockResourceManager(mockCtrl)
				mockRM.EXPECT().GetResourceGroup(gomock.Any()).Return(nil, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Status: infrav1.IBMPowerVSClusterStatus{
							ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
						},
					},
					ResourceManagerClient: mockRM,
				}
			},
			expectedErr: "resource group not found with ID: rg-id",
		},
		{
			name: "When resource group ID is set in Spec Reference and GetResourceGroup succeeds",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRM := mockRM.NewMockResourceManager(mockCtrl)
				mockRM.EXPECT().GetResourceGroup(gomock.Any()).Return(&resourcemanagerv2.ResourceGroup{
					ID:   ptr.To("rg-id"),
					Name: ptr.To("rg-name"),
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							ResourceGroup: infrav1.ResourceGroupSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
							},
						},
					},
					ResourceManagerClient: mockRM,
				}
			},
		},
		{
			name: "When resource group name is empty (no ID, no Name set)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							ResourceGroup: infrav1.ResourceGroupSource{},
						},
					},
				}
			},
			expectedErr: "resource group name is not set in the spec",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			err := scope.ReconcileResourceGroup(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

// TestReconcileWorkspaceProvisionMissingBranches covers the missing branches in reconcileWorkspaceProvision.
func TestReconcileWorkspaceProvisionMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedErr     string
		expectedRequeue bool
	}{
		{
			name: "When GetResourceInstanceByFilter returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, errors.New("filter error"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeProvision,
								Provision: infrav1.WorkspaceProvisionConfig{
									Name: "my-workspace",
								},
							},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "failed to check for existing workspace",
		},
		{
			name: "When workspace already exists (recovered from previous crash)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					GUID: ptr.To("workspace-guid"),
					Name: ptr.To("my-workspace"),
				}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Workspace: infrav1.WorkspaceSource{
								Type:      infrav1.SourceTypeProvision,
								Provision: infrav1.WorkspaceProvisionConfig{Name: "my-workspace"},
							},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedRequeue: true,
		},
		{
			name: "When provisioned workspace GUID is nil",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				// Filter returns nil (no existing workspace)
				mockRC.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
				// CreateResourceInstance returns workspace with nil GUID
				mockRC.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					GUID: nil,
					Name: ptr.To("my-workspace"),
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Zone: "dal10",
							ResourceGroup: infrav1.ResourceGroupSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
							},
							Workspace: infrav1.WorkspaceSource{
								Type:      infrav1.SourceTypeProvision,
								Provision: infrav1.WorkspaceProvisionConfig{Name: "my-workspace"},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "provisioned workspace or GUID is nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.reconcileWorkspaceProvision(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestReconcileNetworkMissingBranches covers uncovered branches in ReconcileNetwork.
func TestReconcileNetworkMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedErr     string
		expectedRequeue bool
	}{
		{
			name: "When networkID is set in status but DHCPServer ID is empty (recover state)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockPVS := mockP.NewMockPowerVS(mockCtrl)
				mockPVS.EXPECT().GetNetworkByID(gomock.Any(), gomock.Any()).Return(&models.Network{
					NetworkID: ptr.To("net-id"),
					Name:      ptr.To("my-net"),
				}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeProvision,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								ID: "net-id",
								// DHCPServer.ID is intentionally empty
							},
						},
					},
					IBMPowerVSClient: mockPVS,
				}
			},
			expectedRequeue: true,
		},
		{
			name: "When Network source type is unknown",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: "unknown-type",
							},
						},
					},
				}
			},
			expectedErr: "unknown network source type",
		},
		{
			name: "When reconcileNetworkReference called with neither ID nor Name",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{}, // neither ID nor Name
							},
						},
					},
				}
			},
			expectedErr: "network reference must contain either an ID or a Name",
		},
		{
			name: "When reconcileNetworkReference with name returns nil network",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockPVS := mockP.NewMockPowerVS(mockCtrl)
				mockPVS.EXPECT().GetNetworkByName(gomock.Any(), gomock.Any()).Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{Name: "my-network"},
							},
						},
					},
					IBMPowerVSClient: mockPVS,
				}
			},
			expectedErr: "invalid network payload received from IBM cloud for name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.ReconcileNetwork(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestReconcileVPCMissingBranches covers uncovered branches in ReconcileVPC and createVPC.
func TestReconcileVPCMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedErr     string
		expectedRequeue bool
	}{
		{
			name: "When VPC source type is unknown",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{Type: "unknown"},
						},
					},
				}
			},
			expectedErr: "unknown VPC source type",
		},
		{
			name: "When reconcileVPCReference called with neither ID nor Name",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{}, // neither ID nor Name
							},
						},
					},
				}
			},
			expectedErr: "VPC reference must have either ID or Name set",
		},
		{
			name: "When createVPC resource group ID is empty",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				// GetVPCByName returns nil → proceeds to create
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{
								Type: infrav1.SourceTypeProvision,
							},
							// ResourceGroup ID is empty → createVPC will fail
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
			expectedErr: "failed to fetch resource group ID",
		},
		{
			name: "When createVPC succeeds but no SecurityGroup exists (DefaultSecurityGroup is nil)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
				mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(&vpcv1.VPC{
					ID:   ptr.To("new-vpc-id"),
					Name: ptr.To("my-vpc"),
					// DefaultSecurityGroup is nil
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{
								Type: infrav1.SourceTypeProvision,
							},
							ResourceGroup: infrav1.ResourceGroupSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
						},
					},
					Cluster:      &clusterv1.Cluster{},
					IBMVPCClient: mockVPC,
				}
			},
			expectedRequeue: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.ReconcileVPC(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestReconcileLoadBalancersMissingBranches covers the uncovered branches in ReconcileLoadBalancers.
func TestReconcileLoadBalancersMissingBranches(t *testing.T) {
	testCases := []struct {
		name        string
		setupScope  func(t *testing.T) *ClusterScope
		wantReady   bool
		expectedErr string
	}{
		{
			name: "When referenced LB has neither ID nor Name",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{}, // no ID, no Name
								},
							},
						},
					},
					Cluster: &clusterv1.Cluster{},
				}
			},
			expectedErr: "referenced load balancer must have either an ID or Name",
		},
		{
			name: "When referenced LB by Name returns nil (not found) — treated as error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
								},
							},
						},
					},
					Cluster:      &clusterv1.Cluster{},
					IBMVPCClient: mockVPC,
				}
			},
			expectedErr: "failed to fetch referenced load balancer details",
		},
		{
			name: "When provisioned LB with empty name auto-generates and name not in status — uses checkLoadBalancer",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				// No ID in status, GetLoadBalancerByName returns nil → goes to create
				mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
				// checkLoadBalancerPort passes (no additionalListeners)
				// createLoadBalancer → but resource group ID is empty
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type: infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{
										Name: "", // auto-generate name
										Type: infrav1.LoadBalancerTypePublic,
									},
								},
							},
						},
					},
					Cluster:      &clusterv1.Cluster{},
					IBMVPCClient: mockVPC,
				}
			},
			expectedErr: "failed to fetch resource group ID",
		},
		{
			name: "When referenced LB by Name is active → loadBalancerReady = true",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("my-lb"),
					ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
				}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
								},
							},
						},
					},
					Cluster:      &clusterv1.Cluster{},
					IBMVPCClient: mockVPC,
				}
			},
			wantReady: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			ready, err := scope.ReconcileLoadBalancers(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ready).To(Equal(tc.wantReady))
			}
		})
	}
}

// TestReconcileCOSInstanceMissingBranches covers the uncovered branches in ReconcileCOSInstance.
func TestReconcileCOSInstanceMissingBranches(t *testing.T) {
	testCases := []struct {
		name        string
		setupScope  func(t *testing.T) *ClusterScope
		expectedErr string
	}{
		{
			name: "When COSInstance type is empty — returns nil immediately",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type: "", // opt-out
							},
						},
					},
				}
			},
		},
		{
			name: "When COSInstance ID set in Status and GetResourceInstance returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("instance fetch error"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type: infrav1.SourceTypeProvision,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "failed to fetch COS instance (id: cos-id) details",
		},
		{
			name: "When COSInstance ID set in Status but instance is nil",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type: infrav1.SourceTypeProvision,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "COS instance not found in cloud with ID: cos-id",
		},
		{
			name: "When COSInstance ID set in Status but instance is not active",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					State: ptr.To("inactive"),
					Name:  ptr.To("my-cos"),
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type: infrav1.SourceTypeProvision,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							COSInstance: infrav1.COSInstanceStatus{ID: "cos-id"},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "COS instance is not active",
		},
		{
			name: "When COSInstance source type is unknown",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type: "unknown-type",
							},
						},
					},
				}
			},
			expectedErr: "unknown COS instance source type",
		},
		{
			name: "When COSInstance bucket region is not set and VPC region is also not set",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					GUID:  ptr.To("cos-guid"),
					Name:  ptr.To("cos-instance"),
					State: ptr.To(string(infrav1.WorkspaceStateActive)),
					CRN:   ptr.To("crn:v1:bluemix:public:cloud-object-storage:global:a/abc:cos-guid::"),
				}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type:         infrav1.SourceTypeProvision,
								BucketRegion: "", // empty — triggers region lookup
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							// VPC.Region is empty → triggers the error
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "failed to determine COS bucket region: both bucket region and VPC region are unset",
		},
		{
			name: "When reconcileCOSReference called with neither ID nor Name",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{}, // neither ID nor Name
							},
						},
					},
				}
			},
			expectedErr: "COS reference must have either ID or Name set",
		},
		{
			name: "When reconcileCOSReference by ID returns nil instance",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{ID: "cos-ref-id"},
							},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "referenced COS instance ID cos-ref-id not found",
		},
		{
			name: "When reconcileCOSReference by Name returns nil instance",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockRC := mockRC.NewMockResourceController(mockCtrl)
				mockRC.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							COSInstance: infrav1.COSInstanceSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{Name: "cos-ref-name"},
							},
						},
					},
					ResourceClient: mockRC,
				}
			},
			expectedErr: "referenced COS instance Name cos-ref-name not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			err := scope.ReconcileCOSInstance(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

// TestDeleteDHCPServerMissingBranches covers uncovered branches in DeleteDHCPServer.
func TestDeleteDHCPServerMissingBranches(t *testing.T) {
	testCases := []struct {
		name        string
		setupScope  func(t *testing.T) *ClusterScope
		expectedErr string
	}{
		{
			name: "When network is in Reference mode — skip deletion",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeReference,
							},
						},
					},
				}
			},
		},
		{
			name: "When workspace is also provisioned — skip DHCP deletion (cascading delete)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeProvision,
							},
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeProvision,
							},
						},
					},
				}
			},
		},
		{
			name: "When DHCP server ID is empty in status — nothing to delete",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeProvision,
							},
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								DHCPServer: infrav1.ResourceReference{ID: ""},
							},
						},
					},
				}
			},
		},
		{
			name: "When GetDHCPServer returns 404-like error — DHCP server already gone",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockPVS := mockP.NewMockPowerVS(mockCtrl)
				mockPVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("%s: not found", string(DHCPServerNotFound)))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeProvision,
							},
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								DHCPServer: infrav1.ResourceReference{ID: "dhcp-id"},
							},
						},
					},
					IBMPowerVSClient: mockPVS,
				}
			},
		},
		{
			name: "When GetDHCPServer returns non-404 error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockPVS := mockP.NewMockPowerVS(mockCtrl)
				mockPVS.EXPECT().GetDHCPServer(gomock.Any(), gomock.Any()).Return(nil, errors.New("internal server error"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeProvision,
							},
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								DHCPServer: infrav1.ResourceReference{ID: "dhcp-id"},
							},
						},
					},
					IBMPowerVSClient: mockPVS,
				}
			},
			expectedErr: "failed to fetch DHCP server",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			err := scope.DeleteDHCPServer(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

// TestDeleteLoadBalancerMissingBranches covers the uncovered branches in DeleteLoadBalancer.
func TestDeleteLoadBalancerMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedRequeue bool
		expectedErr     string
	}{
		{
			name: "When load balancer type is Reference — skip deletion",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "lb-ref-id"},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "When LB ID in status and GetLoadBalancer returns 404 → isNotFound, no error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{Name: "lb-to-delete"},
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							LoadBalancers: []infrav1.LoadBalancerStatus{
								{Name: "lb-to-delete", ID: "lb-id"},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
		},
		{
			name: "When LB not in status and GetLoadBalancerByName returns nil → already gone",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{Name: "lb-gone"},
								},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
		},
		{
			name: "When LB ID in status but GetLoadBalancer returns unexpected error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 500}, errors.New("internal error"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{Name: "lb"},
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							LoadBalancers: []infrav1.LoadBalancerStatus{
								{Name: "lb", ID: "lb-id"},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
			expectedErr: "failed to fetch load balancer lb details",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.DeleteLoadBalancer(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestDeleteLoadBalancerAutoNameBranches covers auto-name generation edge cases in DeleteLoadBalancer.
func TestDeleteLoadBalancerAutoNameBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedRequeue bool
		expectedErr     string
	}{
		{
			name: "When provisioned private LB with empty name auto-generates private name and found by name",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				// private LB auto-name, no ID cached, found by name
				mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("private-lb-id"),
					Name:               ptr.To("test-cluster-loadbalancer-private"),
					ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateDeletePending)),
				}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type: infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{
										Name: "",
										Type: infrav1.LoadBalancerTypePrivate,
									},
								},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
			expectedRequeue: true,
		},
		{
			name: "When multiple provisioned LBs with empty names (i>0 gets qualifier suffix)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				// First LB (i=0): auto-name without qualifier, already gone
				mockVPC.EXPECT().GetLoadBalancerByName("test-cluster-loadbalancer-public").Return(nil, nil)
				// Second LB (i=1): auto-name with qualifier "1", already gone
				mockVPC.EXPECT().GetLoadBalancerByName("test-cluster-loadbalancer-public-1").Return(nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
						Spec: infrav1.IBMPowerVSClusterSpec{
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type: infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{
										Name: "",
										Type: infrav1.LoadBalancerTypePublic,
									},
								},
								{
									Type: infrav1.SourceTypeProvision,
									Provision: infrav1.LoadBalancerProvision{
										Name: "",
										Type: infrav1.LoadBalancerTypePublic,
									},
								},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.DeleteLoadBalancer(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestDeleteVPCSubnetsMissingBranches covers the uncovered branches in DeleteVPCSubnets.
func TestDeleteVPCSubnetsMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		expectedRequeue bool
		expectedErr     string
	}{
		{
			name: "When subnet fetch returns 404 → subnet deleted, skip it",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							// Empty spec → all status subnets are managed
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-1", Name: "subnet-name"},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
		},
		{
			name: "When spec VPCSubnets is empty but status has subnets — all treated as managed (auto-expanded)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockVPC := mock.NewMockVpc(mockCtrl)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{
					ID:     ptr.To("subnet-1"),
					Name:   ptr.To("subnet-name"),
					Status: ptr.To("available"),
				}, nil, nil)
				mockVPC.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPCSubnets: []infrav1.VPCSubnetSource{}, // empty spec
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-1", Name: "subnet-name"},
							},
						},
					},
					IBMVPCClient: mockVPC,
				}
			},
			expectedRequeue: true,
		},
		{
			name: "When referenced subnets (not managed) are skipped",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPCSubnets: []infrav1.VPCSubnetSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "subnet-ref", Name: "referenced-subnet"},
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-ref", Name: "referenced-subnet"},
							},
						},
					},
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.DeleteVPCSubnets(ctx)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestDeleteTransitGatewayConnectionsMissingBranches covers uncovered branches in deleteTransitGatewayConnections.
func TestDeleteTransitGatewayConnectionsMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		tg              *tgapiv1.TransitGateway
		expectedRequeue bool
		expectedErr     string
	}{
		{
			name: "When TG is nil — returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
				}
			},
			tg:          nil,
			expectedErr: "transit gateway or its ID is nil during connection deletion",
		},
		{
			name: "When TG ID is nil — returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
				}
			},
			tg:          &tgapiv1.TransitGateway{ID: nil},
			expectedErr: "transit gateway or its ID is nil during connection deletion",
		},
		{
			name: "When PowerVS connection is in deleting state — requeue",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockTG := tgmock.NewMockTransitGateway(mockCtrl)
				mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
					ID:     ptr.To("conn-id"),
					Status: ptr.To(string(infrav1.TransitGatewayConnectionStateDeleting)),
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							TransitGateway: infrav1.TransitGatewaySource{
								PowerVSConnection: infrav1.TransitGatewayConnectionSource{
									Type: infrav1.SourceTypeProvision,
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							TransitGateway: infrav1.TransitGatewayStatus{
								PowerVSConnection: infrav1.ResourceConnectionStatus{ID: "conn-id"},
							},
						},
					},
					TransitGatewayClient: mockTG,
				}
			},
			tg:              &tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("tg-name")},
			expectedRequeue: true,
		},
		{
			name: "When PowerVS connection GetTransitGatewayConnection returns non-404 error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockTG := tgmock.NewMockTransitGateway(mockCtrl)
				mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 500}, errors.New("internal error"))
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							TransitGateway: infrav1.TransitGatewaySource{
								PowerVSConnection: infrav1.TransitGatewayConnectionSource{
									Type: infrav1.SourceTypeProvision,
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							TransitGateway: infrav1.TransitGatewayStatus{
								PowerVSConnection: infrav1.ResourceConnectionStatus{ID: "conn-id"},
							},
						},
					},
					TransitGatewayClient: mockTG,
				}
			},
			tg:          &tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("tg-name")},
			expectedErr: "failed to get transit gateway connection",
		},
		{
			name: "When VPC connection is in deleting state — requeue (PowerVS connection not configured)",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				mockTG := tgmock.NewMockTransitGateway(mockCtrl)
				mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{
					ID:     ptr.To("vpc-conn-id"),
					Status: ptr.To(string(infrav1.TransitGatewayConnectionStateDeleting)),
				}, nil, nil)
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							TransitGateway: infrav1.TransitGatewaySource{
								// PowerVSConnection not set to Provision
								VPCConnection: infrav1.TransitGatewayConnectionSource{
									Type: infrav1.SourceTypeProvision,
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							TransitGateway: infrav1.TransitGatewayStatus{
								VPCConnection: infrav1.ResourceConnectionStatus{ID: "vpc-conn-id"},
							},
						},
					},
					TransitGatewayClient: mockTG,
				}
			},
			tg:              &tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("tg-name")},
			expectedRequeue: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			requeue, err := scope.deleteTransitGatewayConnections(ctx, tc.tg)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestReconcileConnectionProvisionMissingBranches covers uncovered branches in reconcileConnectionProvision.
func TestReconcileConnectionProvisionMissingBranches(t *testing.T) {
	testCases := []struct {
		name            string
		setupScope      func(t *testing.T) *ClusterScope
		existingConns   []tgapiv1.TransitGatewayConnectionCust
		expectedRequeue bool
		expectedErr     string
	}{
		{
			name: "When existing connection has nil fields — returns error",
			setupScope: func(t *testing.T) *ClusterScope {
				t.Helper()
				return &ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
				}
			},
			existingConns: []tgapiv1.TransitGatewayConnectionCust{
				{
					// NetworkType matches but ID/Name/Status are all nil
					NetworkType: ptr.To(string(vpcNetworkConnectionType)),
					NetworkID:   ptr.To("network-crn"),
					ID:          nil, // nil → triggers error
				},
			},
			expectedErr: "IBM cloud returned nil fields for existing connection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := tc.setupScope(t)
			tg := &tgapiv1.TransitGateway{
				ID:   ptr.To("tg-id"),
				Name: ptr.To("tg-name"),
			}
			provSpec := infrav1.TransitGatewayConnectionProvision{}
			requeue, err := scope.reconcileConnectionProvision(ctx, tg, provSpec, ptr.To("network-crn"), vpcNetworkConnectionType, tc.existingConns)
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requeue).To(Equal(tc.expectedRequeue))
			}
		})
	}
}

// TestGetPublicLoadBalancerHostNameProvisionedPrivate covers the private LB skip path.
func TestGetPublicLoadBalancerHostNameProvisionedPrivate(t *testing.T) {
	g := NewWithT(t)

	// Only private LBs in spec → should return nil hostname
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				LoadBalancers: []infrav1.LoadBalancerSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "my-private-lb",
							Type: infrav1.LoadBalancerTypePrivate,
						},
					},
				},
			},
			Status: infrav1.IBMPowerVSClusterStatus{
				LoadBalancers: []infrav1.LoadBalancerStatus{
					{Name: "my-private-lb", Hostname: "private.host"},
				},
			},
		},
	}

	hostName, err := clusterScope.GetPublicLoadBalancerHostName()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(hostName).To(BeNil())
}

// Coverage-gap tests (session 3): targeting 91.7% → ~93%+.

// TestReconcileNetworkUnknownType covers the default branch in ReconcileNetwork.
func TestReconcileNetworkUnknownType(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.NetworkSource{Type: "garbage"},
			},
		},
	}
	_, err := clusterScope.ReconcileNetwork(ctx)
	g.Expect(err).To(MatchError(ContainSubstring("unknown network source type")))
}

// TestReconcileNetworkDHCPIDMissing covers the recovery path where network ID is set
// but DHCP server ID is missing (returns requeue=true).
func TestReconcileNetworkDHCPIDMissing(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When network ID set, provision type, but DHCP server ID empty, returns requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
}

// TestReconcileNetworkReferenceNilPayload covers the nil-network-object error from
// reconcileNetworkReference when GetNetworkByID returns an incomplete payload.
func TestReconcileNetworkReferenceNilPayload(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("GetNetworkByID returns nil network, error returned", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "net-id"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(nil, nil)
		_, err := clusterScope.reconcileNetworkReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("invalid network payload")))
	})

	t.Run("GetNetworkByName returns nil network, error returned", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "net-name"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any(), "net-name").Return(nil, nil)
		_, err := clusterScope.reconcileNetworkReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("invalid network payload")))
	})

	t.Run("Returns error when reference has neither ID nor Name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
		}
		_, err := clusterScope.reconcileNetworkReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("must contain either an ID or a Name")))
	})
}

// TestReconcileWorkspaceReferenceNilGUID covers the GUID/name nil guard in
// reconcileWorkspaceReference.
func TestReconcileWorkspaceReferenceNilGUID(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When workspace found but GUID is nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-south-1",
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "ws-id"},
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: nil, Name: ptr.To("ws-name")}, nil,
		)
		_, err := clusterScope.reconcileWorkspaceReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("missing GUID or name")))
	})
}

// TestReconcileTransitGatewayReferenceNilIDOrName covers the guard in ReconcileTransitGateway
// when the Reference path returns a TG with nil ID or Name from IBM Cloud.
func TestReconcileTransitGatewayReferenceNilIDOrName(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When resolveTransitGatewayReference returns TG with nil ID, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "tg-id"},
					},
				},
			},
		}
		// Returns TG with nil ID
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(
			&tgapiv1.TransitGateway{ID: nil, Name: nil}, nil, nil,
		)
		_, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("nil ID or Name")))
	})
}

// TestResolveTransitGatewayReferenceGetByIDError covers the error case when
// GetTransitGateway returns an error in resolveTransitGatewayReference.
func TestResolveTransitGatewayReferenceGetByIDError(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("GetTransitGateway returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(
			nil, nil, fmt.Errorf("API error"),
		)
		_, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{ID: "tg-id"})
		g.Expect(err).To(MatchError(ContainSubstring("failed to get transit gateway by ID")))
	})

	t.Run("GetTransitGatewayByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
		}
		mockTransitGateway.EXPECT().GetTransitGatewayByName("tg-name").Return(
			nil, fmt.Errorf("API error"),
		)
		_, err := clusterScope.resolveTransitGatewayReference(ctx, infrav1.ResourceIdentifier{Name: "tg-name"})
		g.Expect(err).To(MatchError(ContainSubstring("failed to get transit gateway by name")))
	})
}

// TestProvisionTransitGatewayBranches covers the untested error paths in
// provisionTransitGateway.
func TestProvisionTransitGatewayBranches(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							Name: "my-tg",
						},
					},
					// No ResourceGroup → GetResourceGroupID returns ""
				},
			},
		}
		// idempotency check returns nil
		mockTransitGateway.EXPECT().GetTransitGatewayByName("my-tg").Return(nil, nil)
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("resource group ID")))
	})

	t.Run("When workspace ID or VPC ID missing, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							Name: "my-tg",
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				// Status.Workspace.ID and Status.VPC.ID are both empty
			},
		}
		mockTransitGateway.EXPECT().GetTransitGatewayByName("my-tg").Return(nil, nil)
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("PowerVS workspace or VPC reconciliation is not yet complete")))
	})
}

// TestReconcileVPCUnknownType covers the default branch in ReconcileVPC.
func TestReconcileVPCUnknownType(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				VPC: infrav1.VPCSource{Type: "unknown"},
			},
		},
	}
	_, err := clusterScope.ReconcileVPC(ctx)
	g.Expect(err).To(MatchError(ContainSubstring("unknown VPC source type")))
}

// TestReconcileVPCNilDetails covers ReconcileVPC when the idempotency path returns
// nil vpcDetails from IBM Cloud.
func TestReconcileVPCNilDetails(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When VPC ID in status but cloud returns nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, nil)
		_, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("vpc not found")))
	})
}

// TestReconcileVPCReferenceNilFields covers the path in reconcileVPCReference where
// GetVPC returns a VPC with nil ID or Name, and also the "no ID or Name" branch.
func TestReconcileVPCReferenceNilFields(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When reference has neither ID nor Name, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Type: infrav1.SourceTypeReference},
				},
			},
		}
		_, err := clusterScope.reconcileVPCReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("VPC reference must have either ID or Name set")))
	})

	t.Run("When GetVPCByName returns nil VPC, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "my-vpc"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetVPCByName("my-vpc").Return(nil, nil)
		_, err := clusterScope.reconcileVPCReference(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("referenced VPC not found")))
	})
}

// TestCreateVPCBranches covers the empty-resourceGroupID and CreateSecurityGroupRule
// error branches in createVPC.
func TestCreateVPCBranches(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.createVPC("my-vpc")
		g.Expect(err).To(MatchError(ContainSubstring("resource group ID")))
	})

	t.Run("When CreateSecurityGroupRule returns error, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			Cluster:      &clusterv1.Cluster{},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(
			&vpcv1.VPC{
				ID:   ptr.To("vpc-id"),
				Name: ptr.To("my-vpc"),
				DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
					ID: ptr.To("sg-id"),
				},
			}, nil, nil,
		)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(
			nil, nil, fmt.Errorf("sg rule creation failed"),
		)
		_, err := clusterScope.createVPC("my-vpc")
		g.Expect(err).To(MatchError(ContainSubstring("error creating security group rule")))
	})
}

// TestCreateVPCSubnetBranches covers the empty resourceGroupID and empty vpcID
// error branches in createVPCSubnet.
func TestCreateVPCSubnetBranches(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When resource group ID is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.createVPCSubnet("my-subnet", "us-south-1")
		g.Expect(err).To(MatchError(ContainSubstring("resource group ID is empty")))
	})

	t.Run("When VPC ID is empty in status, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				// Status.VPC.ID is empty
			},
		}
		_, err := clusterScope.createVPCSubnet("my-subnet", "us-south-1")
		g.Expect(err).To(MatchError(ContainSubstring("managing VPC ID is not found")))
	})
}

// TestReconcileSubnetReferenceNeitherIDNorName covers the "else" branch in
// reconcileSubnetReference when neither ID nor Name is set.
func TestReconcileSubnetReferenceNeitherIDNorName(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("Returns error when ref has neither ID nor Name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.reconcileSubnetReference(infrav1.ResourceIdentifier{})
		g.Expect(err).To(MatchError(ContainSubstring("must have either ID or Name defined")))
	})
}

// TestReconcileLoadBalancersBranches covers the reference-by-name, nil-loadbalancer,
// and reference-not-ready branches in ReconcileLoadBalancers.
func TestReconcileLoadBalancersBranches(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("Reference type: GetLoadBalancerByName succeeds, LB not active → isAnyNotReady=true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(
			&vpcv1.LoadBalancer{
				ID:                 ptr.To("lb-id"),
				Name:               ptr.To("my-lb"),
				ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
				Hostname:           ptr.To(""),
			}, nil,
		)
		requeue, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		// isAnyLoadBalancerNotReady=true → returns false, nil (not requeue)
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("Reference type: GetLoadBalancerByName returns nil loadbalancer, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(nil, nil)
		_, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch referenced load balancer")))
	})

	t.Run("Reference type: GetLoadBalancerByName returns error, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "my-lb"},
						},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(nil, fmt.Errorf("API error"))
		_, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch referenced load balancer")))
	})

	t.Run("Reference type: neither ID nor Name → returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{},
						},
					},
				},
			},
		}
		_, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("referenced load balancer must have either an ID or Name")))
	})
}

// TestCreateLoadBalancerNoSubnets covers the "no VPC subnets in status" guard
// in createLoadBalancer.
func TestCreateLoadBalancerNoSubnets(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When VPC subnets status is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
				// Status.VPCSubnets is empty
			},
		}
		_, err := clusterScope.createLoadBalancer(ctx, "my-lb", infrav1.LoadBalancerProvision{})
		g.Expect(err).To(MatchError(ContainSubstring("no VPC subnets are present")))
	})
}

// TestReconcileVPCSecurityGroupReferenceBranches covers the name-based lookup
// and the error+nil-sg branches in reconcileVPCSecurityGroupReference.
func TestReconcileVPCSecurityGroupReferenceBranches(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("Reference by Name succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName("my-sg").Return(
			&vpcv1.SecurityGroup{ID: ptr.To("sg-id"), Name: ptr.To("my-sg")}, nil,
		)
		status, err := clusterScope.reconcileVPCSecurityGroupReference(ctx, infrav1.ResourceIdentifier{Name: "my-sg"})
		g.Expect(err).To(BeNil())
		g.Expect(status.ID).To(Equal("sg-id"))
	})

	t.Run("Neither ID nor Name set, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.reconcileVPCSecurityGroupReference(ctx, infrav1.ResourceIdentifier{})
		g.Expect(err).To(MatchError(ContainSubstring("must have either ID or Name specified")))
	})

	t.Run("GetSecurityGroupByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName("my-sg").Return(nil, fmt.Errorf("API error"))
		_, err := clusterScope.reconcileVPCSecurityGroupReference(ctx, infrav1.ResourceIdentifier{Name: "my-sg"})
		g.Expect(err).To(MatchError(ContainSubstring("failed to find referenced VPC security group")))
	})
}

// TestReconcileVPCSecurityGroupProvisionAPIError covers the case where
// GetSecurityGroupByName returns a non-SecurityGroupByNameNotFound error.
func TestReconcileVPCSecurityGroupProvisionAPIError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetSecurityGroupByName returns a real API error (not NotFound), error is returned", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, fmt.Errorf("internal server error"))
		prov := infrav1.VPCSecurityGroupProvision{Name: "my-sg"}
		_, err := clusterScope.reconcileVPCSecurityGroupProvision(ctx, prov)
		g.Expect(err).To(MatchError(ContainSubstring("failed to query VPC security group by name")))
	})
}

// TestCreateVPCSecurityGroupRulesErrors covers the empty-protocol and empty-remotes
// error branches in createVPCSecurityGroupRules.
func TestCreateVPCSecurityGroupRulesErrors(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When inbound rule protocol is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
				Source: infrav1.VPCSecurityGroupRulePrototype{
					Protocol: "", // empty
					Remotes:  []infrav1.VPCSecurityGroupRuleRemote{{}},
				},
			},
		}
		_, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "sg-id")
		g.Expect(err).To(MatchError(ContainSubstring("empty protocol")))
	})

	t.Run("When inbound rule remotes are empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
				Source: infrav1.VPCSecurityGroupRulePrototype{
					Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
					Remotes:  []infrav1.VPCSecurityGroupRuleRemote{}, // empty
				},
			},
		}
		_, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "sg-id")
		g.Expect(err).To(MatchError(ContainSubstring("no remotes")))
	})

	t.Run("When direction is invalid, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		rules := []infrav1.VPCSecurityGroupRule{
			{
				Direction: "invalid-direction",
			},
		}
		_, err := clusterScope.createVPCSecurityGroupRules(ctx, rules, "sg-id")
		g.Expect(err).To(MatchError(ContainSubstring("invalid rule direction")))
	})
}

// TestReconcileCOSInstanceNotActive covers the path where COS ID is in status but
// instance is not active.
func TestReconcileCOSInstanceNotActive(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When COS instance ID in status but instance is not active, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{
						ID: "cos-id",
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("cos-id"),
				Name:  ptr.To("cos-name"),
				State: ptr.To("provisioning"),
			}, nil, nil,
		)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("not active")))
	})

	t.Run("When COS instance type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type: "garbage",
					},
				},
			},
		}
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown COS instance source type")))
	})
}

// TestReconcileCOSInstanceBucketRegionFallback covers the path where bucket region
// falls back to VPC region and succeeds.
func TestReconcileCOSInstanceBucketRegionFallback(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCOSClient          *mockcos.MockCos
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
		mockCOSClient = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When bucket region falls back to VPC region and COS client already set, succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		existingHMACSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-cos-hmac", Namespace: "default"},
		}
		clusterScope := ClusterScope{
			Client:         fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existingHMACSecret).Build(),
			ResourceClient: mockResourceController,
			COSClient:      mockCOSClient,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "default"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "cos-id"},
						// BucketRegion intentionally empty — will fall back to VPC region
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC:         infrav1.VPCStatus{Region: "us-south"},
					COSInstance: infrav1.COSInstanceStatus{HMACSecretName: "my-cluster-cos-hmac"}, //nolint:gosec
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("cos-id"),
				Name:  ptr.To("cos-name"),
				State: ptr.To(string(infrav1.WorkspaceStateActive)),
				CRN:   ptr.To("crn:v1:bluemix:public:cloud-object-storage:global:a/abc:cos-id::"),
			}, nil, nil,
		)
		mockCOSClient.EXPECT().GetBucketByName(gomock.Any()).Return(nil, nil)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When both bucket region and VPC region are unset, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			COSClient:      mockCOSClient,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					COSInstance: infrav1.COSInstanceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "cos-id"},
					},
				},
				// VPC.Region is empty
			},
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("cos-id"),
				Name:  ptr.To("cos-name"),
				State: ptr.To(string(infrav1.WorkspaceStateActive)),
				CRN:   ptr.To("crn:v1:bluemix:public:cloud-object-storage:global:a/abc:cos-id::"),
			}, nil, nil,
		)
		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to determine COS bucket region")))
	})
}

// TestReconcileCOSReferenceByName covers the Name-based lookup path in
// reconcileCOSReference as well as the "neither ID nor Name" error.
func TestReconcileCOSReferenceByName(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("Reference by Name succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{
				GUID: ptr.To("cos-guid"),
				Name: ptr.To("cos-name"),
			}, nil,
		)
		instance, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{Name: "cos-name"})
		g.Expect(err).To(BeNil())
		g.Expect(*instance.GUID).To(Equal("cos-guid"))
	})

	t.Run("Reference by Name: GetResourceInstanceByFilter returns nil, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		_, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{Name: "cos-name"})
		g.Expect(err).To(MatchError(ContainSubstring("not found")))
	})

	t.Run("Neither ID nor Name set, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.reconcileCOSReference(ctx, infrav1.ResourceIdentifier{})
		g.Expect(err).To(MatchError(ContainSubstring("must have either ID or Name set")))
	})
}

// TestReconcileCOSProvisionError covers the error path in reconcileCOSProvision.
func TestReconcileCOSProvisionError(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetResourceInstanceByFilter returns error, error propagated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient:    mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			nil, fmt.Errorf("API error"),
		)
		_, err := clusterScope.reconcileCOSProvision(ctx, "my-cos")
		g.Expect(err).To(MatchError(ContainSubstring("failed checking for existing COS instance")))
	})
}

// TestFetchVPCCRNBranches covers the empty-VPC-ID and GetVPC-error branches
// in fetchVPCCRN.
func TestFetchVPCCRNBranches(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When VPC ID in status is empty, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		_, err := clusterScope.fetchVPCCRN()
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch VPC ID")))
	})

	t.Run("When GetVPC returns error, error propagated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "vpc-id"},
				},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.fetchVPCCRN()
		g.Expect(err).To(MatchError(ContainSubstring("API error")))
	})
}

// TestDeleteVPCSecurityGroupsDeleteError covers the DeleteSecurityGroup error path
// in DeleteVPCSecurityGroups.
func TestDeleteVPCSecurityGroupsDeleteError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When DeleteSecurityGroup returns error, error returned", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.VPCSecurityGroupProvision{Name: "my-sg"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: []infrav1.VPCSecurityGroupStatus{
						{ID: "sg-id", Name: "my-sg"},
					},
				},
			},
		}
		// GetSecurityGroup succeeds
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(
			&vpcv1.SecurityGroup{ID: ptr.To("sg-id"), Name: ptr.To("my-sg")}, nil, nil,
		)
		// DeleteSecurityGroup fails
		mockVPC.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(
			nil, fmt.Errorf("delete failed"),
		)
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to execute DeleteSecurityGroup API")))
	})
}

// TestDeleteVPCSubnetsDeleteError covers the DeleteSubnet error path in DeleteVPCSubnets.
func TestDeleteVPCSubnetsDeleteError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When DeleteSubnet returns error, error is aggregated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: infrav1.SourceTypeProvision, Provision: infrav1.VPCSubnetProvision{Name: "my-subnet"}},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(
			&vpcv1.Subnet{ID: ptr.To("subnet-id"), Status: ptr.To("available")}, nil, nil,
		)
		mockVPC.EXPECT().DeleteSubnet(gomock.Any()).Return(nil, fmt.Errorf("delete failed"))
		_, err := clusterScope.DeleteVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to delete VPC subnet")))
	})
}

// TestReconcileCOSBucketDefaultError covers the default error case in reconcileCOSBucket
// when an unknown awserr code is returned.
func TestReconcileCOSBucketDefaultError(t *testing.T) {
	var (
		mockCOSClient *mockcos.MockCos
		mockCtrl      *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOSClient = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetBucketByName returns unknown awserr code, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSClient,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSClient.EXPECT().GetBucketByName(gomock.Any()).Return(
			nil, awserr.New("UnknownCode", "unknown aws error", nil),
		)
		err := clusterScope.reconcileCOSBucket(ctx, "my-bucket")
		g.Expect(err).To(MatchError(ContainSubstring("unexpected error checking bucket presence")))
	})

	t.Run("When GetBucketByName returns non-awserr, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSClient,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSClient.EXPECT().GetBucketByName(gomock.Any()).Return(
			nil, fmt.Errorf("non-aws error"),
		)
		err := clusterScope.reconcileCOSBucket(ctx, "my-bucket")
		g.Expect(err).To(MatchError(ContainSubstring("failed to check if COS bucket exists")))
	})
}

// TestReconcileCOSBucketAlreadyOwnedByYou covers the BucketAlreadyOwnedByYou
// edge case in reconcileCOSBucket (CreateBucket returns that error).
func TestReconcileCOSBucketAlreadyOwnedByYou(t *testing.T) {
	var (
		mockCOSClient *mockcos.MockCos
		mockCtrl      *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOSClient = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When CreateBucket returns BucketAlreadyOwnedByYou, returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:         mockCOSClient,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockCOSClient.EXPECT().GetBucketByName(gomock.Any()).Return(
			nil, awserr.New(s3.ErrCodeNoSuchBucket, "no such bucket", nil),
		)
		mockCOSClient.EXPECT().CreateBucket(gomock.Any()).Return(
			nil, awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "already owned", nil),
		)
		err := clusterScope.reconcileCOSBucket(ctx, "my-bucket")
		g.Expect(err).To(BeNil())
	})
}

// TestGetPublicLoadBalancerHostNameReferenceByID covers the Reference+ID path in
// GetPublicLoadBalancerHostName where we fall back to a live API call.
func TestGetPublicLoadBalancerHostNameReferenceByID(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("Reference with ID only: API fetch returns the name, hostname found in status", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "lb-id"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "my-lb", Hostname: "lb.example.com"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(
			&vpcv1.LoadBalancer{
				ID:   ptr.To("lb-id"),
				Name: ptr.To("my-lb"),
			}, nil, nil,
		)
		hostname, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(err).To(BeNil())
		g.Expect(hostname).NotTo(BeNil())
		g.Expect(*hostname).To(Equal("lb.example.com"))
	})

	t.Run("Reference with ID only: GetLoadBalancer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "lb-id"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "my-lb", Hostname: "lb.example.com"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch referenced load balancer")))
	})
}

// TestReconcileVPCSubnetsVirtualIPTopology covers the VirtualIP topology skip path
// in ReconcileVPCSubnets.
func TestReconcileVPCSubnetsVirtualIPTopology(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
			},
		},
	}
	requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
}

// TestCheckDHCPServerStatusUnknown covers the default (unknown state) branch
// in checkDHCPServerStatus.
func TestCheckDHCPServerStatusUnknown(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
	}
	_, err := clusterScope.checkDHCPServerStatus(ctx, models.DHCPServerDetail{
		Status: ptr.To("weird-state"),
	})
	g.Expect(err).To(MatchError(ContainSubstring("unknown state")))
}

// TestIsDHCPServerActiveError covers the GetDHCPServer error and nil-detail branches
// in isDHCPServerActive.
func TestIsDHCPServerActiveError(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("GetDHCPServer returns nil detail, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						DHCPServer: infrav1.ResourceReference{ID: "dhcp-id"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), "dhcp-id").Return(nil, nil)
		_, err := clusterScope.isDHCPServerActive(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("DHCP server details are nil")))
	})
}

// TestReconcileWorkspaceUnknownType covers the default branch in ReconcileWorkspace.
func TestReconcileWorkspaceUnknownType(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				Workspace: infrav1.WorkspaceSource{
					Type: "unknown-type",
				},
			},
		},
	}
	_, err := clusterScope.ReconcileWorkspace(ctx)
	g.Expect(err).To(MatchError(ContainSubstring("unknown workspace source type")))
}

// Coverage-gap tests (session 3 - part 2): targeting 92.8% → ~93.5%+.

// TestReconcileNetworkGetNetworkByIDError covers the GetNetworkByID error path
// when networkID is already in status.
func TestReconcileNetworkGetNetworkByIDError(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetNetworkByID returns error, error propagated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			nil, fmt.Errorf("network fetch failed"),
		)
		_, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch network by ID")))
	})

	t.Run("When network ID is set, Reference type, returns false nil (no DHCP check)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeReference},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{ID: "net-id"},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When network is set, provision type, DHCP server not active, requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Network: infrav1.NetworkSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Network: infrav1.NetworkStatus{
						ID:         "net-id",
						DHCPServer: infrav1.ResourceReference{ID: "dhcp-id"},
					},
				},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any(), "net-id").Return(
			&models.Network{NetworkID: ptr.To("net-id"), Name: ptr.To("net-name")}, nil,
		)
		// DHCP server is still building (not active)
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any(), "dhcp-id").Return(
			&models.DHCPServerDetail{
				ID:     ptr.To("dhcp-id"),
				Status: ptr.To(string(infrav1.DHCPServerStateBuild)),
			}, nil,
		)
		requeue, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
}

// TestReconcileWorkspaceProvisionGetByFilterError covers the error returned from
// GetResourceInstanceByFilter in reconcileWorkspaceProvision.
func TestReconcileWorkspaceProvisionGetByFilterError(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetResourceInstanceByFilter returns error, error propagated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-east-1",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(
			nil, fmt.Errorf("filter check failed"),
		)
		_, err := clusterScope.reconcileWorkspaceProvision(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to check for existing workspace")))
	})
}

// TestProvisionTransitGatewayCreateError covers the CreateTransitGateway error
// path in provisionTransitGateway.
func TestProvisionTransitGatewayCreateError(t *testing.T) {
	var (
		mockTransitGateway *tgmock.MockTransitGateway
		mockCtrl           *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockTransitGateway = tgmock.NewMockTransitGateway(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When CreateTransitGateway returns error, error propagated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							Name: "my-tg",
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
					VPC: infrav1.VPCSource{Region: "us-south"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
					VPC:       infrav1.VPCStatus{ID: "vpc-id", Region: "us-south"},
				},
			},
		}
		// Idempotency check returns nil
		mockTransitGateway.EXPECT().GetTransitGatewayByName("my-tg").Return(nil, nil)
		// CreateTransitGateway fails
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(
			nil, nil, fmt.Errorf("create TG failed"),
		)
		_, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("failed to create transit gateway")))
	})

	t.Run("When GlobalRouting is explicitly set to Global, globalRouting is true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "dal10",
					TransitGateway: infrav1.TransitGatewaySource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.TransitGatewayProvision{
							Name:          "my-tg",
							GlobalRouting: infrav1.TransitGatewayRoutingGlobal,
						},
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
					VPC: infrav1.VPCSource{Region: "us-south"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
					VPC:       infrav1.VPCStatus{ID: "vpc-id", Region: "us-south"},
				},
			},
		}
		mockTransitGateway.EXPECT().GetTransitGatewayByName("my-tg").Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(
			&tgapiv1.TransitGateway{ID: ptr.To("tg-id"), Name: ptr.To("my-tg")}, nil, nil,
		)
		tg, err := clusterScope.provisionTransitGateway(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(tg).NotTo(BeNil())
		g.Expect(*tg.ID).To(Equal("tg-id"))
	})
}

// TestValidateClusterScopeParamsClientBuilderNil covers the ClientBuilder nil
// validation branch in validate.
func TestValidateClusterScopeParamsClientBuilderNil(t *testing.T) {
	g := NewWithT(t)
	params := &ClusterScopeParams{
		Client:            testEnv.Client,
		Cluster:           newCluster(testClusterName),
		IBMPowerVSCluster: newPowerVSCluster(testClusterName),
		// ClientBuilder intentionally nil
	}
	err := params.validate()
	g.Expect(err).To(MatchError(ContainSubstring("ClientBuilder is nil")))
}

// TestReconcileVPCSubnetsUnknownType covers the default-type error branch
// in ReconcileVPCSubnets.
func TestReconcileVPCSubnetsUnknownType(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When subnet source type is unknown, returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{Type: "garbage"},
					},
				},
			},
		}
		_, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("unknown VPC subnet source type")))
	})
}

// TestReconcileLoadBalancersPrivateLBNaming covers the private LB naming path
// in ReconcileLoadBalancers where i > 0.
func TestReconcileLoadBalancersPrivateLBNaming(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When provision type private LB with no name and i>0, uses private naming with index", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			Cluster:      &clusterv1.Cluster{},
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{Name: "first-lb"},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Type: infrav1.LoadBalancerTypePrivate,
								// Name intentionally empty — will use auto-naming with index
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ResourceGroup: infrav1.ResourceReference{ID: "rg-id"},
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		// First LB: checkLoadBalancer finds nil (not existing), then createLoadBalancer
		mockVPC.EXPECT().GetLoadBalancerByName("first-lb").Return(nil, nil)
		mockVPC.EXPECT().CreateLoadBalancer(gomock.Any()).Return(
			&vpcv1.LoadBalancer{
				ID:                 ptr.To("lb1-id"),
				Name:               ptr.To("first-lb"),
				ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
				Hostname:           ptr.To(""),
			}, nil, nil,
		).Times(1)
		// Second LB (private): checkLoadBalancer finds nil, then createLoadBalancer
		mockVPC.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateLoadBalancer(gomock.Any()).Return(
			&vpcv1.LoadBalancer{
				ID:                 ptr.To("lb2-id"),
				Name:               ptr.To("my-cluster-lb-private-1"),
				ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateCreatePending)),
				Hostname:           ptr.To(""),
			}, nil, nil,
		)
		requeue, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

// TestReconcileWorkspaceUnknownTypeViaReconcileWorkspace covers the default branch
// in ReconcileWorkspace when workspace type is unknown.
func TestReconcileWorkspaceNilIDAndUnknownType(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Spec: infrav1.IBMPowerVSClusterSpec{
				Workspace: infrav1.WorkspaceSource{
					Type: "bad-type",
				},
			},
		},
	}
	_, err := clusterScope.ReconcileWorkspace(ctx)
	g.Expect(err).To(MatchError(ContainSubstring("unknown workspace source type")))
}

// TestCheckDHCPServerStatusNil covers the nil status pointer branch in checkDHCPServerStatus.
func TestCheckDHCPServerStatusNil(t *testing.T) {
	g := NewWithT(t)
	clusterScope := ClusterScope{
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
	}
	_, err := clusterScope.checkDHCPServerStatus(ctx, models.DHCPServerDetail{Status: nil})
	g.Expect(err).To(MatchError(ContainSubstring("DHCP server status is nil")))
}

// TestReconcileVPCSubnetsGetSubnetError covers the GetSubnet error branch when a
// subnet is already in the status (idempotency verification path).
func TestReconcileVPCSubnetsGetSubnetError(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When GetSubnet returns error for status-tracked subnet, error returned", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					VPCSubnets: []infrav1.VPCSubnetSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "my-subnet"},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnets: []infrav1.VPCSubnetStatus{
						{ID: "subnet-id", Name: "my-subnet"},
					},
				},
			},
		}
		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("API error"))
		_, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("error verifying active VPC subnet")))
	})
}

// TestReconcileWorkspaceProvisionSuccessPath covers the createWorkspace full-success path
// in reconcileWorkspaceProvision.
func TestReconcileWorkspaceProvisionSuccessPath(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("When createWorkspace succeeds with valid GUID, status is updated and requeue", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Zone: "us-east-1",
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeProvision,
					},
					ResourceGroup: infrav1.ResourceGroupSource{
						Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
					},
				},
			},
		}
		// Idempotency check returns nil
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, nil)
		// CreateResourceInstance returns success with GUID
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(
			&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("new-ws-guid")}, nil, nil,
		)
		requeue, err := clusterScope.reconcileWorkspaceProvision(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Workspace.ID).To(Equal("new-ws-guid"))
	})
}
