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
	"errors"
	"fmt"
	"os"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/pointer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos"
	mockcos "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	mockP "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway"
	tgmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
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
				ClientFactory: ClientFactory{
					AuthenticatorFactory: func() (core.Authenticator, error) {
						return nil, nil
					},
					PowerVSClientFactory: func() (powervs.PowerVS, error) {
						return nil, nil
					},
				},
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
				ClientFactory: ClientFactory{
					AuthenticatorFactory: func() (core.Authenticator, error) {
						return nil, nil
					},
					PowerVSClientFactory: func() (powervs.PowerVS, error) {
						return nil, nil
					},
					VPCClientFactory: func() (vpc.Vpc, error) {
						return nil, nil
					},
					TransitGatewayFactory: func() (transitgateway.TransitGateway, error) {
						return nil, nil
					},
					ResourceControllerFactory: func() (resourcecontroller.ResourceController, error) {
						return nil, nil
					},
					ResourceManagerFactory: func() (resourcemanager.ResourceManager, error) {
						return nil, nil
					},
				},
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewPowerVSClusterScope(tc.params)
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
							DHCPServer: infrav1.ResourceReferenceV1Beta3{
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

func TestVPCSecurityGroupByName(t *testing.T) {
	testCases := []struct {
		name         string
		sgName       string
		expectedID   *string
		clusterScope ClusterScope
	}{
		{
			name: "VPC SG status is not set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
		},
		{
			name: "invalid SG name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
			sgName: "sg2",
		},
		{
			name: "valid SG name is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
			sgName:     "sg",
			expectedID: ptr.To("sg-1"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			sgID, _, _ := tc.clusterScope.GetVPCSecurityGroupByName(tc.sgName)
			g.Expect(pointer.Dereference(sgID)).To(Equal(pointer.Dereference(tc.expectedID)))
		})
	}
}

func TestVPCSecurityGroupByID(t *testing.T) {
	testCases := []struct {
		name         string
		sgID         string
		expectedID   *string
		clusterScope ClusterScope
	}{
		{
			name: "VPC SG status is not set",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG ID is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
		},
		{
			name: "invalid SG ID is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
			sgID: "sg2",
		},
		{
			name: "valid SG ID is passed",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
							"sg": {
								ID: ptr.To("sg-1"),
							},
						},
					},
				},
			},
			sgID:       "sg-1",
			expectedID: ptr.To("sg-1"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			sgID, _, _ := tc.clusterScope.GetVPCSecurityGroupByID(tc.sgID)
			g.Expect(pointer.Dereference(sgID)).To(Equal(pointer.Dereference(tc.expectedID)))
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
						{Name: "-loadbalancer", Hostname: "lb-hostname"},
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
						ResourceGroup: infrav1.ResourceReferenceV1Beta3{
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
						ResourceGroup: infrav1.ResourceReferenceV1Beta3{
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

func TestGetServiceName(t *testing.T) {
	testCases := []struct {
		name         string
		resourceType infrav1.ResourceType
		expectedName *string
		clusterScope ClusterScope
	}{
		{
			name:         "Resource type is cos instance and cos instance is nil",
			resourceType: infrav1.ResourceTypeCOSInstance,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: testClusterName}},
			},
			expectedName: ptr.To("ClusterName-cosinstance"),
		},
		{
			name:         "Resource type is cos instance and cos instance is not nil",
			resourceType: infrav1.ResourceTypeCOSInstance,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{CosInstance: &infrav1.CosInstance{Name: "CosInstanceName"}}},
			},
			expectedName: ptr.To("CosInstanceName"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is nil",
			resourceType: infrav1.ResourceTypeCOSBucket,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: testClusterName}},
			},
			expectedName: ptr.To("ClusterName-cosbucket"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is not nil",
			resourceType: infrav1.ResourceTypeCOSBucket,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{CosInstance: &infrav1.CosInstance{BucketName: "CosBucketName"}}},
			},
			expectedName: ptr.To("CosBucketName"),
		},
		{
			name: "Resource type is invalid",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			expectedName: nil,
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			rgID := tc.clusterScope.GetServiceName(tc.resourceType)
			g.Expect(rgID).To(Equal(tc.expectedName))
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReferenceV1Beta3{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("GetDHCPServer returns error"))
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReferenceV1Beta3{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)

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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: infrav1.NetworkStatus{DHCPServer: infrav1.ResourceReferenceV1Beta3{ID: "dhcpID"}}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)

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
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "serviceInstanceID",
					},
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{
							Name: "capi-powervs-cluster-lb-public",
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
	powervsClusterScope := func() *ClusterScope {
		return &ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: map[string]infrav1.VPCSecurityGroupStatus{
						"sc": {
							ID:                ptr.To("sc-id"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
		}
	}

	t.Run("When security group is not found", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When DeleteSecurityGroup returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete security group"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("When GetSecurityGroup returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("When DeleteSecurityGroup successfully deletes security group in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When deleting multiple SecurityGroup", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSecurityGroups = map[string]infrav1.VPCSecurityGroupStatus{
			"sc1": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
			},
			"sc2": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
			},
			"sc3": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
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

	t.Run("When one security group is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSecurityGroups = map[string]infrav1.VPCSecurityGroupStatus{
			"sc1": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
			},
			"sc2": {
				ID: ptr.To("sc-id"),
			},
		}
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When security group is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSecurityGroups = map[string]infrav1.VPCSecurityGroupStatus{
			"sc": {
				ID: ptr.To("sc-id"),
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
func TestIsResourceCreatedByController(t *testing.T) {
	testCases := []struct {
		name           string
		resourceType   infrav1.ResourceType
		clusterScope   ClusterScope
		expectedResult bool
	}{
		{},
		{
			name: "When resourceType is COSInstance and COSInstance status is nil",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeCOSInstance,
			expectedResult: false,
		},
		{
			name: "When resourceType is COSInstance and COSInstance status is not nil",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						COSInstance: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			resourceType:   infrav1.ResourceTypeCOSInstance,
			expectedResult: true,
		},
		{
			name: "When resourceType is not valid",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypePublicGateway,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			isResourceCreated := tc.clusterScope.isResourceCreatedByController(tc.resourceType)
			g.Expect(isResourceCreated).To(Equal(tc.expectedResult))
		})
	}
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
	t.Run("When COS instance resource is not created by controller", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance ID is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				COSInstance: &infrav1.ResourceReference{},
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
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: &infrav1.ResourceReference{
						ID:                ptr.To("cosInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To("pending_reclamation")}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance is not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: &infrav1.ResourceReference{
						ID:                ptr.To("cosInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, fmt.Errorf("error getting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: &infrav1.ResourceReference{
						ID:                ptr.To("cosInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error getting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err.Error()).To(Equal("failed to fetch COS service instance: error getting resource instance"))
	})
	t.Run("When COS instance state is active and DeleteResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: &infrav1.ResourceReference{
						ID:                ptr.To("cosInstanceID"),
						ControllerCreated: ptr.To(true),
					},
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

	t.Run("When COS instance state is active and DeleteResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: &infrav1.ResourceReference{
						ID:                ptr.To("cosInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To(string(infrav1.WorkspaceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, fmt.Errorf("error deleting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(Equal(fmt.Errorf("error deleting resource instance")))
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
					DHCPServer: infrav1.ResourceReferenceV1Beta3{
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
						DHCPServer: infrav1.ResourceReferenceV1Beta3{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("dhcp server does not exist"))
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
						DHCPServer: infrav1.ResourceReferenceV1Beta3{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("error getting dhcp server"))
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
						DHCPServer: infrav1.ResourceReferenceV1Beta3{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpServerID")}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		mockPowerVS.EXPECT().DeleteDHCPServer(gomock.Any()).Return(fmt.Errorf("error deleting dhcp server"))
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
						DHCPServer: infrav1.ResourceReferenceV1Beta3{
							ID: "dhcpServerID",
						},
					},
				},
			},
			IBMPowerVSClient: mockPowerVS,
		}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpServerID")}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		mockPowerVS.EXPECT().DeleteDHCPServer(gomock.Any()).Return(nil)
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
		mockCOSController      *mockcos.MockCos
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockCOSController = mockcos.NewMockCos(mockCtrl)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When fetch for COS service instance fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: testRegion,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, fmt.Errorf("error fetching instance by name"))

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance).To(BeNil())
	})

	t.Run("When COS service instance is found in IBM Cloud and cluster status is updated", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: testRegion,
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
					Zone:          "test-zone",
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.WorkspaceStateActive)),
			GUID:  ptr.To("test-cos-instance-guid"),
		}, nil)

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-cos-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(false)))
	})

	t.Run("When COS service instance is not found in IBM Cloud and hence creates COS instance in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: testRegion,
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
					Zone:          "test-zone",
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID:   ptr.To("test-resource-instance-id"),
			GUID: ptr.To("test-resource-instance-guid"),
			Name: ptr.To("test-resource-instance-name"),
		}, nil, nil)

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-resource-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(true)))
	})

	t.Run("When COS service instance is not found in IBM Cloud and hence creates COS instance in cloud but fails during the creation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: testRegion,
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
					Zone:          "test-zone",
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create COS service instance"))

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance).To(BeNil())
	})

	t.Run("When fetch for API_KEY fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: testRegion,
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.WorkspaceStateActive)),
			GUID:  ptr.To("test-cos-instance-guid"),
		}, nil)

		err := clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-cos-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(false)))
	})

	t.Run("When COS bucket region is failed to be determined", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance:   &infrav1.CosInstance{},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID:   ptr.To("test-resource-instance-id"),
			GUID: ptr.To("test-resource-instance-guid"),
			Name: ptr.To("test-resource-instance-name"),
		}, nil, nil)

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-resource-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(bool(true))))
	})

	t.Run("When checkCOSBucket fails to determine whether bucket exist in cloud due to an unexpected error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID:   ptr.To("test-resource-instance-id"),
			GUID: ptr.To("test-resource-instance-guid"),
			Name: ptr.To("test-resource-instance-name"),
		}, nil, nil)

		mockCOSController.EXPECT().GetBucketByName(gomock.Any()).Return(nil, fmt.Errorf("failed to get bucket by name"))

		cos.NewServiceFunc = func(_ cos.ServiceOptions, _, _ string) (cos.Cos, error) {
			return mockCOSController, nil
		}

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-resource-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(true)))
	})
	t.Run("When create COS bucket fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID:   ptr.To("test-resource-instance-id"),
			GUID: ptr.To("test-resource-instance-guid"),
			Name: ptr.To("test-resource-instance-name"),
		}, nil, nil)

		mockCOSController.EXPECT().GetBucketByName(gomock.Any()).Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "bucket does not exist", nil))
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, fmt.Errorf("failed to create bucket"))

		cos.NewServiceFunc = func(_ cos.ServiceOptions, _, _ string) (cos.Cos, error) {
			return mockCOSController, nil
		}

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-resource-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(true)))
	})

	t.Run("When create COS bucket succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		err := os.Setenv("IBMCLOUD_APIKEY", "test-api-key")
		g.Expect(err).To(BeNil())
		defer os.Unsetenv("IBMCLOUD_APIKEY")

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resource-group-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}
		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID:   ptr.To("test-resource-instance-id"),
			GUID: ptr.To("test-resource-instance-guid"),
			Name: ptr.To("test-resource-instance-name"),
		}, nil, nil)

		mockCOSController.EXPECT().GetBucketByName(gomock.Any()).Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "bucket does not exist", nil))
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, nil)

		cos.NewServiceFunc = func(_ cos.ServiceOptions, _, _ string) (cos.Cos, error) {
			return mockCOSController, nil
		}

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ID).To(Equal(ptr.To("test-resource-instance-guid")))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance.ControllerCreated).To(Equal(ptr.To(true)))
	})
}

func TestCheckCOSServiceInstance(t *testing.T) {
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

	t.Run("When fetching of cos service instance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, fmt.Errorf("error listing COS instances"))

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When COS service instance is not found in IBM Cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(nil, nil)

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("When COS service instance exists but state is not active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To("failed"),
		}, nil)

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).ToNot(BeNil())
		g.Expect(err).To(MatchError(fmt.Errorf("COS service instance is not in active state, current state: %s", "failed")))
	})
	t.Run("When COS service instance exists and state is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstanceByFilter(gomock.AssignableToTypeOf(resourcecontroller.InstanceFilter{})).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.WorkspaceStateActive)),
		}, nil)

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance.Name).To(Equal(ptr.To("test-cos-resource-name")))
		g.Expect(cosResourceInstance.State).To(Equal(ptr.To(string(infrav1.WorkspaceStateActive))))
		g.Expect(err).To(BeNil())
	})
}

func TestCreateCOSBucket(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCOSController      *mockcos.MockCos
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
		mockCOSController = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When COS bucket creation fails in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, fmt.Errorf("failed to create COS bucket"))
		err := clusterScope.createCOSBucket()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When COS bucket already exists and is owned by the user", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "Bucket already owned by user", nil))

		err := clusterScope.createCOSBucket()
		g.Expect(err).To(BeNil())
	})

	t.Run("When COS bucket already exists but is owned by someone else", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}
		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, awserr.New(s3.ErrCodeBucketAlreadyExists, "Bucket already exists", nil))
		err := clusterScope.createCOSBucket()
		g.Expect(err).To(BeNil())
	})

	t.Run("When an unexpected error occurs during COS bucket creation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(nil, awserr.New("UnexpectedError", "An unexpected error occurred", nil))

		err := clusterScope.createCOSBucket()
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to create COS bucket"))
	})

	t.Run("When COS bucket is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockCOSController.EXPECT().CreateBucket(gomock.Any()).Return(&s3.CreateBucketOutput{}, nil)

		err := clusterScope.createCOSBucket()
		g.Expect(err).To(BeNil())
	})
}

func TestCheckCOSBucket(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCOSController      *mockcos.MockCos
		mockCtrl               *gomock.Controller
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
		mockCOSController = mockcos.NewMockCos(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When checking if COS bucket exists in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		testScenarios := []struct {
			name         string
			mockError    error
			bucketExists bool
			expectErr    bool
		}{
			{
				name:         "NoSuchBucket error",
				mockError:    awserr.New(s3.ErrCodeNoSuchBucket, "NoSuchBucket", nil),
				bucketExists: false,
				expectErr:    false,
			},
			{
				name:         "Forbidden error",
				mockError:    awserr.New("Forbidden", "Forbidden", nil),
				bucketExists: false,
				expectErr:    false,
			},
			{
				name:         "NotFound error",
				mockError:    awserr.New("NotFound", "NotFound", nil),
				bucketExists: false,
				expectErr:    false,
			},
			{
				name:         "Other aws error",
				mockError:    awserr.New("OtherAWSError", "OtherAWSError", nil),
				bucketExists: false,
				expectErr:    true,
			},
			{
				name:         "Bucket exists",
				mockError:    nil,
				bucketExists: true,
				expectErr:    false,
			},
		}

		for _, scenario := range testScenarios {
			t.Run(scenario.name, func(_ *testing.T) {
				mockCOSController.EXPECT().GetBucketByName(gomock.Any()).Return(nil, scenario.mockError)
				exists, err := clusterScope.checkCOSBucket()
				g.Expect(exists).To(Equal(scenario.bucketExists))
				if scenario.expectErr {
					g.Expect(err).ToNot(BeNil())
				} else {
					g.Expect(err).To(BeNil())
				}
			})
		}
	})
}

func TestCreateCOSServiceInstance(t *testing.T) {
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

	t.Run("When creating COS resource instance fails due to missing resource group id", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		cosResourceInstance, err := clusterScope.createCOSServiceInstance()
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When creating COS resource instance fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resourcegroup-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error creating resource instance"))

		cosResourceInstance, err := clusterScope.createCOSServiceInstance()
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When COS resource instance creation is successful", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := ClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "test-resourcegroup-id"}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "test-serviceinstance-id",
					},
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			ID: ptr.To("new-resource-instance-id"),
		}, nil, nil)

		cosResourceInstance, err := clusterScope.createCOSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(cosResourceInstance.ID).To(Equal(ptr.To("new-resource-instance-id")))
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
					Workspace: infrav1.ResourceReferenceV1Beta3{
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
				Workspace: infrav1.ResourceReferenceV1Beta3{
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

	t.Run("When SecurityGroup ID is set and returns error while getting SecurityGroup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						ID: ptr.To("securityGroupID"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When SecurityGroup Name is set and returns error while creating SecurityGroup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup Name is set and creates SecurityGroup successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID")}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroup Name is set and SecurityGroup already exists", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When SecurityGroup ID is set and SecurityGroup already exists", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						ID: ptr.To("securityGroupID"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroup Name is set and GetSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroupStatus := make(map[string]infrav1.VPCSecurityGroupStatus)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: vpcSecurityGroupStatus,
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When SecurityGroup Name is set  and returns error while getting SecurityGroupRules", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroupStatus := make(map[string]infrav1.VPCSecurityGroupStatus)
		vpcSecurityGroupStatus["securityGroupName"] = infrav1.VPCSecurityGroupStatus{
			ID:      ptr.To("securityGroupID"),
			RuleIDs: []*string{ptr.To("ruleID")},
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: vpcSecurityGroupStatus,
					VPC:               infrav1.VPCStatus{ID: "vpcID"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID")}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to get security group rule"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup is created successfully but returns error while creating SecurityGroupRules", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.0.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID")}, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to create security group rule"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})
}

func TestValidateVPCSecurityGroup(t *testing.T) {
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

	t.Run("When SecurityGroup by name exists and SecurityGroupRule matches", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID: ptr.To("ruleID"),
		}
		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(sg).To(BeEquivalentTo(securityGroupDetails))
		g.Expect(err).To(BeNil())
	})

	t.Run("When SecurityGroup by id exists and SecurityGroupRule matches", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID: ptr.To("ruleID"),
		}
		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(securityGroupDetails, nil, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(sg).To(BeEquivalentTo(securityGroupDetails))
		g.Expect(err).To(BeNil())
	})

	t.Run("When SecurityGroup by name doesn't exist and return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name: ptr.To("securityGroupNamw"),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get SecurityGroup"))
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup by id doesn't exist and return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID: ptr.To("securityGroupID"),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get SecurityGroup"))
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup by name exists but is not attached to VPC", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name: ptr.To("securityGroupName"),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("sgID")}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeEmpty())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup by name exists but VPC id not matching", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name: ptr.To("securityGroupName"),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("sgID"), VPC: &vpcv1.VPCReference{ID: ptr.To("vpcID")}}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroup by id exists but VPC id not matching", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID: ptr.To("securityGroupID"),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("sgID"), VPC: &vpcv1.VPCReference{ID: ptr.To("vpcID")}}
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(securityGroupDetails, nil, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When it returns error while validating SecurityGroupRules", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					CIDRSubnetName: ptr.To("CIDRSubnetName"),
					RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID:      ptr.To("ruleID"),
			PortMax: ptr.To(int64(65535)),
			PortMin: ptr.To(int64(1)),
		}
		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}
		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(securityGroupDetails, nil, nil)
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When SecurityGroupRule doesn't exist and creates it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.1.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{Protocol: ptr.To("tcp"), ID: ptr.To("ruleID")}, nil, nil)
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("When SecurityGroupRule doesn't match and return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroupStatus := make(map[string]infrav1.VPCSecurityGroupStatus)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: vpcSecurityGroupStatus,
					VPC:               infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to create security group rule"))
		sg, ruleIDs, err := clusterScope.validateVPCSecurityGroup(ctx, vpcSecurityGroup)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestValidateVPCSecurityGroupRule(t *testing.T) {
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

	t.Run("When it matches SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolTcpudp", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID:      ptr.To("ruleID"),
			PortMax: ptr.To(int64(65535)),
			PortMin: ptr.To(int64(1)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(*ruleID).To(BeEquivalentTo("ruleID"))
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolTcpudp", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID:      ptr.To("ruleID"),
			PortMax: ptr.To(int64(65535)),
			PortMin: ptr.To(int64(1)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolTcpudp returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID:      ptr.To("ruleID"),
			PortMax: ptr.To(int64(65535)),
			PortMin: ptr.To(int64(1)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When it matches SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolAll", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := make([]vpcv1.SecurityGroupRuleIntf, 0, 1)
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(*ruleID).To(BeEquivalentTo("ruleID"))
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolAll", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			Address:    ptr.To("192.168.0.1/24"),
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				Address: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := make([]vpcv1.SecurityGroupRuleIntf, 0, 1)
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolAll returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When it matches SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolIcmp", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				ICMPCode: ptr.To(int64(12)),
				ICMPType: ptr.To(int64(3)),
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CRN: ptr.To("crn"),
			},
			ID:   ptr.To("ruleID"),
			Code: ptr.To(int64(12)),
			Type: ptr.To(int64(3)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: ptr.To("crn"), CRN: ptr.To("crn")}, nil)
		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(*ruleID).To(BeEquivalentTo("ruleID"))
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolIcmp", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				ICMPCode: ptr.To(int64(12)),
				ICMPType: ptr.To(int64(3)),
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CRN: ptr.To("crn"),
			},
			ID:   ptr.To("ruleID"),
			Code: ptr.To(int64(12)),
			Type: ptr.To(int64(3)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: ptr.To("crn"), CRN: ptr.To("CRN")}, nil)
		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of protocolType SecurityGroupRuleSecurityGroupRuleProtocolIcmp returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				ICMPCode: ptr.To(int64(12)),
				ICMPType: ptr.To(int64(3)),
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CRN: ptr.To("crn"),
			},
			ID:   ptr.To("ruleID"),
			Code: ptr.To(int64(12)),
			Type: ptr.To(int64(3)),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get securityGroup"))
		ruleID, match, err := clusterScope.validateSecurityGroupRule(vpcSecurityGroupRules, rules.Direction, rules.Destination, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestValidateVPCSecurityGroupRules(t *testing.T) {
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

	t.Run("When SecurityGroupRule of Direction Inbound matches", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("inbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleIDs, match, err := clusterScope.validateVPCSecurityGroupRules(vpcSecurityGroupRules, vpcSecurityGroup.Rules)
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Inbound doesn't match", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("inbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "securityGroupID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleIDs, match, err := clusterScope.validateVPCSecurityGroupRules(vpcSecurityGroupRules, vpcSecurityGroup.Rules)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Inbound returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("inbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}
		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		ruleIDs, match, err := clusterScope.validateVPCSecurityGroupRules(vpcSecurityGroupRules, vpcSecurityGroup.Rules)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Outbound matches", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("0.0.0.0/0"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleIDs, match, err := clusterScope.validateVPCSecurityGroupRules(vpcSecurityGroupRules, vpcSecurityGroup.Rules)
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Outbound doesn't match", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: ptr.To("outbound"),
			Protocol:  ptr.To("tcp"),
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: ptr.To("192.168.1.1/24"),
			},
			ID: ptr.To("ruleID"),
		}

		vpcSecurityGroupRules := append([]vpcv1.SecurityGroupRuleIntf{}, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		ruleIDs, match, err := clusterScope.validateVPCSecurityGroupRules(vpcSecurityGroupRules, vpcSecurityGroup.Rules)
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
}

func TestValidateVPCSecurityGroupRuleRemote(t *testing.T) {
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
	t.Run("When it matches the remoteType Address", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			Address:    ptr.To("192.168.0.1/24"),
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{Address: ptr.To("192.168.0.1/24")}, remote)
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match the remoteType Address", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			Address:    ptr.To("192.168.0.1/24"),
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{Address: ptr.To("192.168.1.1/24")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it matches the remoteType Any", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CIDRBlock: ptr.To("0.0.0.0/0")}, remote)
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match the remoteType Any", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CIDRBlock: ptr.To("192.168.1.1/24")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it matches the remoteType CIDR", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: ptr.To("192.168.1.1/24")}, nil)
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CIDRBlock: ptr.To("192.168.1.1/24")}, remote)
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match the remoteType CIDR", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: ptr.To("192.168.0.1/24")}, nil)
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CIDRBlock: ptr.To("192.168.1.1/24")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When the remoteType CIDR and it returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CIDRBlock: ptr.To("192.168.1.1/24")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When it matches the remoteType SG", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: ptr.To("192.168.1.1/24"), CRN: ptr.To("crn")}, nil)
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CRN: ptr.To("crn")}, remote)
		g.Expect(match).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When it doesn't match the remoteType SG", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: ptr.To("192.168.1.1/24"), CRN: ptr.To("CRN")}, nil)
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CRN: ptr.To("crn")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When the remoteType SG and it returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get security group"))
		match, err := clusterScope.validateVPCSecurityGroupRuleRemote(&vpcv1.SecurityGroupRuleRemote{CRN: ptr.To("crn")}, remote)
		g.Expect(match).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
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
	var protocol = "tcp"
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Creates SecurityGroupRule of remoteType Address successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			Address:    ptr.To("192.168.0.1/24"),
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("outbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeEquivalentTo(ptr.To("ruleID")))
		g.Expect(err).To(BeNil())
	})
	t.Run("Creates SecurityGroupRule of remoteType CIDR successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: ptr.To("192.168.1.1/24")}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("outbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeEquivalentTo(ptr.To("ruleID")))
		g.Expect(err).To(BeNil())
	})
	t.Run("SecurityGroupRule of remoteType CIDR returns error when getting VPC subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: ptr.To("CIDRSubnetName"),
			RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get VPC subnet"))
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("outbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("Creates SecurityGroupRule of remoteType Any successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("outbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeEquivalentTo(ptr.To("ruleID")))
		g.Expect(err).To(BeNil())
	})
	t.Run("Creates SecurityGroupRule of remoteType SG successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{CRN: ptr.To("crn"), Name: ptr.To("securityGroupName")}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp{Direction: ptr.To("inbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("inbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeEquivalentTo(ptr.To("ruleID")))
		g.Expect(err).To(BeNil())
	})
	t.Run("SecurityGroupRule of remoteType SG returns error while getting securityGroup", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to get security group"))
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("inbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("SecurityGroupRule of remoteType SG returns error when SecurityGroup doesn't exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1.VPCSecurityGroupRuleRemote{
			SecurityGroupName: ptr.To("securityGroupName"),
			RemoteType:        infrav1.VPCSecurityGroupRuleRemoteTypeSG,
		}
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name:  ptr.To("securityGroupName"),
						Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
					}),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		ruleID, err := clusterScope.createVPCSecurityGroupRule(ctx, &securityGroupID, ptr.To("inbound"), &protocol, &portMin, &portMax, remote)
		g.Expect(ruleID).To(BeNil())
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

	t.Run("When SecurityGroupRule of Direction Outbound created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.0.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"))
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Outbound returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					CIDRSubnetName: ptr.To("CIDRSubnetName"),
					RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"))
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Inbound created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.0.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{Direction: ptr.To("inbound"), ID: ptr.To("ruleID")}, nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"))
		g.Expect(ruleIDs).To(BeEquivalentTo([]*string{ptr.To("ruleID")}))
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroupRule of Direction Inbound returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					CIDRSubnetName: ptr.To("CIDRSubnetName"),
					RemoteType:     infrav1.VPCSecurityGroupRuleRemoteTypeCIDR,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}

		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: infrav1.VPCStatus{ID: "VPCID", Name: "vpcName"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		ruleIDs, err := clusterScope.createVPCSecurityGroupRules(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"))
		g.Expect(ruleIDs).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestCreateVPCSecurityGroupRulesAndSetStatus(t *testing.T) {
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
	t.Run("When SecurityGroupRule is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.0.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{Direction: ptr.To("outbound"), ID: ptr.To("ruleID")}, nil, nil)
		err := clusterScope.createVPCSecurityGroupRulesAndSetStatus(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"), ptr.To("securityGroupName"))
		g.Expect(err).To(BeNil())
	})
	t.Run("When CreateSecurityGroupRule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		rules := infrav1.VPCSecurityGroupRule{
			Direction: infrav1.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1.VPCSecurityGroupRulePrototype{
				Remotes: append([]infrav1.VPCSecurityGroupRuleRemote{}, infrav1.VPCSecurityGroupRuleRemote{
					Address:    ptr.To("192.168.0.1/24"),
					RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAddress,
				}),
				Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
			},
		}
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to create securityGroupRules"))
		err := clusterScope.createVPCSecurityGroupRulesAndSetStatus(ctx, vpcSecurityGroup.Rules, ptr.To("securityGroupID"), ptr.To("securityGroupName"))
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
	t.Run("When SecurityGroup is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID")}, nil, nil)
		sg, err := clusterScope.createVPCSecurityGroup(ctx, clusterScope.IBMPowerVSCluster.Spec.VPCSecurityGroups[0])
		g.Expect(*sg).To(BeEquivalentTo("securityGroupID"))
		g.Expect(err).To(BeNil())
	})
	t.Run("When SecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := ClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "resourceGroupID"}},
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create SecurityGroup"))
		sg, err := clusterScope.createVPCSecurityGroup(ctx, clusterScope.IBMPowerVSCluster.Spec.VPCSecurityGroups[0])
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestClusterScope_BucketRegion(t *testing.T) {
	testRegion := region
	vpcRegion := "us-east"

	testcases := []struct {
		name                 string
		expectedBucketRegion string
		clusterScope         ClusterScope
	}{
		{
			name:                 "Returns bucket region from COS instance when set",
			expectedBucketRegion: testRegion,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{
							BucketRegion: testRegion,
						},
						VPC: infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: vpcRegion},
					},
				},
			},
		},
		{
			name:                 "Returns VPC region when COS bucket region is not set",
			expectedBucketRegion: vpcRegion,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{},
						VPC:         infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: vpcRegion},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						VPC: infrav1.VPCStatus{Region: vpcRegion},
					},
				},
			},
		},
		{
			name:                 "Returns VPC region when COS instance is nil",
			expectedBucketRegion: vpcRegion,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						VPC: infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: vpcRegion},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						VPC: infrav1.VPCStatus{Region: vpcRegion},
					},
				},
			},
		},
		{
			name:                 "Returns empty string when both COS bucket region and VPC region are not set",
			expectedBucketRegion: "",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{},
						VPC:         infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-east"},
					},
				},
			},
		},
		{
			name:                 "Returns empty string when COS instance is nil and VPC region is not set",
			expectedBucketRegion: "",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						VPC: infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-east"},
					},
				},
			},
		},
		{
			name:                 "Returns empty string when both COS instance and VPC are nil",
			expectedBucketRegion: "",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{},
				},
			},
		},
		{
			name:                 "Prioritizes COS bucket region over VPC region",
			expectedBucketRegion: testRegion,
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{
							BucketRegion: testRegion,
						},
						VPC: infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: vpcRegion},
					},
				},
			},
		},
		{
			name:                 "Returns empty string when COS bucket region is empty and VPC is nil",
			expectedBucketRegion: "",
			clusterScope: ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						CosInstance: &infrav1.CosInstance{
							BucketRegion: "",
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			region := tc.clusterScope.bucketRegion()
			g.Expect(region).To(Equal(tc.expectedBucketRegion))
		})
	}
}
