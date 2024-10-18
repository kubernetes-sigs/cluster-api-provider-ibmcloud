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
	"errors"
	"fmt"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestNewPowerVSClusterScope(t *testing.T) {
	testCases := []struct {
		name        string
		params      PowerVSClusterScopeParams
		expectError bool
	}{
		{
			name: "Error when Client in nil",
			params: PowerVSClusterScopeParams{
				Client: nil,
			},
			expectError: true,
		},
		{
			name: "Error when Cluster in nil",
			params: PowerVSClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: nil,
			},
			expectError: true,
		},
		{
			name: "Error when IBMPowerVSCluster is nil",
			params: PowerVSClusterScopeParams{
				Client:            testEnv.Client,
				Cluster:           newCluster(clusterName),
				IBMPowerVSCluster: nil,
			},
			expectError: true,
		},
		{
			name: "Successfully create cluster scope when create infra annotation is not set",
			params: PowerVSClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: newCluster(clusterName),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "powervs-test-",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: capiv1beta1.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       "capi-test",
								UID:        "1",
							}}},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{Zone: ptr.To("zone")},
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
			params: PowerVSClusterScopeParams{
				Client:  testEnv.Client,
				Cluster: newCluster(clusterName),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Annotations:  map[string]string{"powervs.cluster.x-k8s.io/create-infra": "true"},
						GenerateName: "powervs-test-",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: capiv1beta1.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       "capi-test",
								UID:        "1",
							}}},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						Zone: ptr.To("zone"),
						VPC:  &infrav1beta2.VPCResourceReference{Region: ptr.To("eu-gb")},
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

func TestGetServiceInstanceID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "Service Instance ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "Service Instance ID is set in status.ServiceInstanceID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1beta2.ResourceReference{
							ID: ptr.To("statusServiceInstanceID"),
						},
					},
				},
			},
			expectedID: "statusServiceInstanceID",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			serviceInstanceID := tc.clusterScope.GetServiceInstanceID()
			g.Expect(serviceInstanceID).To(Equal(tc.expectedID))
		})
	}
}

func TestGetDHCPServerID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "DHCP server ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "DHCP server ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						DHCPServer: &infrav1beta2.ResourceReference{
							ID: ptr.To("dhcpserverid"),
						},
					},
				},
			},
			expectedID: ptr.To("dhcpserverid"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			dhcpServerID := tc.clusterScope.GetDHCPServerID()
			g.Expect(utils.DereferencePointer(dhcpServerID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetVPCID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC server ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPC: &infrav1beta2.ResourceReference{
							ID: ptr.To("vpcID"),
						},
					},
				},
			},
			expectedID: ptr.To("vpcID"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			vpcID := tc.clusterScope.GetVPCID()
			g.Expect(utils.DereferencePointer(vpcID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetVPCSubnetID(t *testing.T) {
	testCases := []struct {
		name         string
		subnetName   string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC subnet status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC subnet status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: make(map[string]infrav1beta2.ResourceReference),
					},
				},
			},
		},
		{
			name: "empty subnet name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1beta2.ResourceReference{
							"us-south": {
								ID: ptr.To("us-south-1"),
							},
						},
					},
				},
			},
		},
		{
			name: "invalid subnet name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1beta2.ResourceReference{
							"us-south": {
								ID: ptr.To("us-south-1"),
							},
						},
					},
				},
			},
			subnetName: "us-north",
		},
		{
			name: "valid subnet name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1beta2.ResourceReference{
							"us-south": {
								ID: ptr.To("us-south-1"),
							},
						},
					},
				},
			},
			subnetName: "us-south",
			expectedID: ptr.To("us-south-1"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			subnetID := tc.clusterScope.GetVPCSubnetID(tc.subnetName)
			g.Expect(utils.DereferencePointer(subnetID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetVPCSubnetIDs(t *testing.T) {
	testCases := []struct {
		name         string
		expectedIDs  []*string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC subnet is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC subnet id is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1beta2.ResourceReference{
							"us-south":  {ID: ptr.To("subnet1")},
							"us-south2": {ID: ptr.To("subnet2")},
						},
					},
				},
			},
			expectedIDs: []*string{ptr.To("subnet1"), ptr.To("subnet2")},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			subnetIDs := tc.clusterScope.GetVPCSubnetIDs()
			if tc.expectedIDs == nil {
				g.Expect(subnetIDs).To(BeNil())
			} else {
				g.Expect(subnetIDs).Should(ContainElements(tc.expectedIDs))
			}
		})
	}
}

func TestVPCSecurityGroupByName(t *testing.T) {
	testCases := []struct {
		name         string
		sgName       string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC SG status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1beta2.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			g.Expect(utils.DereferencePointer(sgID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestVPCSecurityGroupByID(t *testing.T) {
	testCases := []struct {
		name         string
		sgID         string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC SG status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1beta2.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG ID is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSecurityGroups: map[string]infrav1beta2.VPCSecurityGroupStatus{
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
			g.Expect(utils.DereferencePointer(sgID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetTransitGatewayID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "TransitGateway ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "TransitGateway ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						TransitGateway: &infrav1beta2.TransitGateway{ID: ptr.To("tgID")},
					},
				},
			},
			expectedID: ptr.To(""),
		},
		{
			name: "TransitGateway ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						TransitGateway: &infrav1beta2.TransitGatewayStatus{
							ID: ptr.To("tgID"),
						},
					},
				},
			},
			expectedID: ptr.To("tgID"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			tgID := tc.clusterScope.GetTransitGatewayID()
			g.Expect(utils.DereferencePointer(tgID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetLoadBalancerID(t *testing.T) {
	testCases := []struct {
		name         string
		lbName       string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "LoadBalancer status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "LoadBalancer status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: make(map[string]infrav1beta2.VPCLoadBalancerStatus),
					},
				},
			},
		},
		{
			name: "empty LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								ID: ptr.To("lb-1"),
							},
						},
					},
				},
			},
		},
		{
			name: "invalid LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								ID: ptr.To("lb-1"),
							},
						},
					},
				},
			},
			lbName: "lb2",
		},
		{
			name: "valid LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								ID: ptr.To("lb-1"),
							},
						},
					},
				},
			},
			lbName:     "lb",
			expectedID: ptr.To("lb-1"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			lbID := tc.clusterScope.GetLoadBalancerID(tc.lbName)
			g.Expect(utils.DereferencePointer(lbID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
		})
	}
}

func TestGetLoadBalancerState(t *testing.T) {
	testCases := []struct {
		name          string
		lbName        string
		expectedState *infrav1beta2.VPCLoadBalancerState
		clusterScope  PowerVSClusterScope
	}{
		{
			name: "LoadBalancer status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "LoadBalancer status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: make(map[string]infrav1beta2.VPCLoadBalancerStatus),
					},
				},
			},
		},
		{
			name: "empty LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1beta2.VPCLoadBalancerStateActive,
							},
						},
					},
				},
			},
		},
		{
			name: "invalid LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1beta2.VPCLoadBalancerStateActive,
							},
						},
					},
				},
			},
			lbName: "lb2",
		},
		{
			name: "valid LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1beta2.VPCLoadBalancerStateActive,
							},
						},
					},
				},
			},
			lbName:        "lb",
			expectedState: ptr.To(infrav1beta2.VPCLoadBalancerStateActive),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			lbState := tc.clusterScope.GetLoadBalancerState(tc.lbName)
			if tc.expectedState == nil {
				g.Expect(lbState).To(BeNil())
			} else {
				g.Expect(*lbState).To(Equal(*tc.expectedState))
			}
		})
	}
}

func TestGetLoadBalancerHostName(t *testing.T) {
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"-loadbalancer": {
							Hostname: ptr.To("lb-hostname"),
						},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							Name:   "lb",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer": {
							Hostname: ptr.To("lb-hostname"),
						},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							Name:   "loadbalancer",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer": {
							Hostname: ptr.To("lb-hostname"),
						},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							Name:   "lb1",
							Public: core.BoolPtr(false),
						},
						{
							Name:   "lb2",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"lb1": {
							Hostname: ptr.To("lb1-hostname"),
						},
						"lb2": {
							Hostname: ptr.To("lb2-hostname"),
						},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb2-hostname")))
		g.Expect(err).To(BeNil())
	})
	t.Run("Multiple public load balancer names are set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							Name:   "lb1",
							Public: core.BoolPtr(true),
						},
						{
							Name:   "lb2",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"lb1": {
							Hostname: ptr.To("lb1-hostname"),
						},
						"lb2": {
							Hostname: ptr.To("lb2-hostname"),
						},
					},
				},
			},
		}

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb1-hostname")))
		g.Expect(err).To(BeNil())
	})
	t.Run("Valid load balancer ID is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("loadbalancer-id"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer": {
							Hostname: ptr.To("lb-hostname"),
						},
					},
				},
			},
		}
		lb := &vpcv1.LoadBalancer{
			Name: ptr.To("loadbalancer"),
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, &core.DetailedResponse{}, nil)

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb-hostname")))
		g.Expect(err).To(BeNil())
	})
	t.Run("Invalid load balancer ID is set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("loadbalancer-id1"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer": {
							Hostname: ptr.To("lb-hostname"),
						},
					},
				},
			},
		}

		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{}, errors.New("failed to get the load balancer"))

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("Multiple public load balancer IDs are set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("lb1"),
							Public: core.BoolPtr(true),
						},
						{
							ID:     ptr.To("lb2"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer1": {
							Hostname: ptr.To("lb1-hostname"),
						},
						"loadbalancer2": {
							Hostname: ptr.To("lb2-hostname"),
						},
					},
				},
			},
		}

		lb := &vpcv1.LoadBalancer{
			Name: ptr.To("loadbalancer1"),
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, &core.DetailedResponse{}, nil)

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb1-hostname")))
		g.Expect(err).To(BeNil())
	})

	t.Run("Both private and public load balancer IDs are set in IBMPowerVSCluster spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("lb1"),
							Public: core.BoolPtr(false),
						},
						{
							ID:     ptr.To("lb2"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"loadbalancer1": {
							Hostname: ptr.To("lb1-hostname"),
						},
						"loadbalancer2": {
							Hostname: ptr.To("lb2-hostname"),
						},
					},
				},
			},
		}

		lb := &vpcv1.LoadBalancer{
			Name: ptr.To("loadbalancer2"),
		}
		mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, &core.DetailedResponse{}, nil)

		hostName, err := clusterScope.GetPublicLoadBalancerHostName()
		g.Expect(hostName).To(Equal(ptr.To("lb2-hostname")))
		g.Expect(err).To(BeNil())
	})
}

func TestGetResourceGroupID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "Resource group ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
		},
		{
			name: "Resource group ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("rgID")},
					},
				},
			},
			expectedID: "rgID",
		},
		{
			name: "Resource group ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						ResourceGroup: &infrav1beta2.ResourceReference{
							ID: ptr.To("rgID"),
						},
					},
				},
			},
			expectedID: "rgID",
		},
		{
			name: "spec Resource group ID takes precedence over status Resource group ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("rgID")},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						ResourceGroup: &infrav1beta2.ResourceReference{
							ID: ptr.To("rgID1"),
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

func TestReconcilePowerVSServiceInstance(t *testing.T) {
	var (
		mockResourceController *mockRC.MockResourceController
		mockCtrl               *gomock.Controller
	)
	serviceInstanceID := "serviceInstanceID"
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockResourceController = mockRC.NewMockResourceController(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When service instance id is set in status and GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1beta2.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and GetResourceInstance returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1beta2.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and instance is in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1beta2.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		instance := &resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-instance"),
			State: ptr.To("failed"),
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(instance, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and instance is in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1beta2.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		instance := &resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-instance"),
			State: ptr.To("active"),
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(instance, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in spec and instance does not exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in spec and instance exists in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		instance := &resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-instance"),
			State: ptr.To("active"),
			GUID:  ptr.To("instance-GUID"),
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(instance, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ID).To(Equal("instance-GUID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ControllerCreated).To(BeFalse())
	})

	t.Run("When service instance id is set in both spec and status, ID from status takes precedence", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("serviceInstanceIDSpec"),
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1beta2.ResourceReference{
						ID: ptr.To("serviceInstanceIDStatus"),
					},
				},
			},
		}

		resource := &resourcecontrollerv2.GetResourceInstanceOptions{
			ID: clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ID,
		}

		instance := &resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-instance"),
			State: ptr.To("active"),
		}
		mockResourceController.EXPECT().GetResourceInstance(resource).Return(instance, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When create service instance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When created service instance is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When successfully created a new service instance", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		instance := &resourcecontrollerv2.ResourceInstance{
			GUID: ptr.To("instance-GUID"),
			Name: ptr.To("test-instance"),
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(instance, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ID).To(Equal("instance-GUID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ControllerCreated).To(BeTrue())
	})
}

func TestCheckServiceInstanceState(t *testing.T) {
	testCases := []struct {
		name        string
		requeue     bool
		expectedErr error
		instance    resourcecontrollerv2.ResourceInstance
	}{
		{
			name:     "Service instance is in active state",
			instance: resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("active")},
		},
		{
			name:     "Service instance is in provisioning state",
			instance: resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("provisioning")},
			requeue:  true,
		},
		{
			name:        "Service instance is in failed state",
			instance:    resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("failed")},
			expectedErr: fmt.Errorf("PowerVS service instance is in failed state"),
		},
		{
			name:        "Service instance is in unknown state",
			instance:    resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("unknown")},
			expectedErr: fmt.Errorf("PowerVS service instance is in unknown state"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			clusterScope := PowerVSClusterScope{}
			requeue, err := clusterScope.checkServiceInstanceState(tc.instance)
			g.Expect(requeue).To(Equal(tc.requeue))
			if tc.expectedErr != nil {
				g.Expect(err).To(Equal(tc.expectedErr))
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestIsServiceInstanceExists(t *testing.T) {
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

	t.Run("When ServiceInstanceID is set in spec and GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstanceID: "instance-id",
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When ServiceInstance.ID is set in spec and GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("instance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When ServiceInstance.Name is set in spec and GetResourceInstance returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						Name: ptr.To("instance-name"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})

	t.Run("When checkServiceInstanceState returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						Name: ptr.To("instance"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("unknown")}, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When isServiceInstanceExists returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
						Name: ptr.To("instance"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("guid"), Name: ptr.To("instance"), State: ptr.To("active")}, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal("guid"))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
}

func TestCreateServiceInstance(t *testing.T) {
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
	t.Run("When resource group is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{},
			},
		}

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When zone is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
				},
			},
		}

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create resource instance"))

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{}, nil, nil)

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).ToNot(BeNil())
		g.Expect(err).To(BeNil())
	})
}

func TestReconcileVPCSecurityGroups(t *testing.T) {
	var (
		mockVPC  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	securityGroupID := "securityGroupID"
	securityGroupName := "securityGroupName"
	ruleID := "ruleID"
	protocol := "tcp"
	cidrSubnetName := "CIDRSubnetName"
	inBoundDirection := "inbound"
	outBoundDirection := "outbound"
	address := "192.168.0.1/24"
	cidrBlock := "192.168.0.1/24"
	ipv4CIDRBlock := "192.168.1.1/24"

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockVPC = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("When Security group ID is set and GetSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			ID: ptr.To(securityGroupID),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group ID is set and GetSecurityGroup returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			ID: ptr.To(securityGroupID),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, nil)

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group ID is not set and GetSecurityGroupByName returns error security group not found by name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name: ptr.To(securityGroupName),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("failed to find security group by name securityGroupName"))

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group ID is not set and CreateSecurityGroup returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name: ptr.To(securityGroupName),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create resource instance"))

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group Name is set and CreateSecurityGroup creates vpc security group successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name: ptr.To(securityGroupName),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: &securityGroupID}, nil, nil)

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).To(BeNil())
	})

	t.Run("When Security group name is set and GetSecurityGroupByName returns security group successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name: ptr.To(securityGroupName),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil)

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).To(BeNil())
	})

	t.Run("When Security group Name is set  and GetSecurityGroupRule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			Address:    &address,
			RemoteType: infrav1beta2.VPCSecurityGroupRuleRemoteTypeAny,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name:  ptr.To(securityGroupName),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		iscreated := true
		vpcSecurityGroupStatus := make(map[string]infrav1beta2.VPCSecurityGroupStatus)
		vpcSecurityGroupStatus[securityGroupName] = infrav1beta2.VPCSecurityGroupStatus{ID: &securityGroupID, RuleIDs: []*string{&ruleID}, ControllerCreated: &iscreated}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPCSecurityGroups: vpcSecurityGroupStatus,
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, nil)
		mockVPC.EXPECT().GetSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to get security group rule"))

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group ID is set and GetSecurityGroup returns security group with security group rules", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			Address:    &address,
			RemoteType: infrav1beta2.VPCSecurityGroupRuleRemoteTypeAddress,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		securityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp{
			Direction: &inBoundDirection,
			PortMax:   &rules.Source.PortRange.MaximumPort,
			PortMin:   &rules.Source.PortRange.MinimumPort,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				Address: &address,
			},
		}

		securityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		securityGroupRules = append(securityGroupRules, &securityGroupRule)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			ID:    ptr.To(securityGroupID),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: securityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil, nil)

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).To(BeNil())
	})

	t.Run("When Security group ID is set and GetVPCSubnetByName returns error  ", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: &cidrSubnetName,
			RemoteType:     infrav1beta2.VPCSecurityGroupRuleRemoteTypeCIDR,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &inBoundDirection,
			Protocol:  &protocol,
			Remote:    &vpcv1.SecurityGroupRuleRemote{},
		}
		vpcSecurityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		v := infrav1beta2.VPCSecurityGroup{
			ID:    ptr.To(securityGroupID),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, v),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil, nil)
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get vpc subnet"))

		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group rule of type CIDR doesn't exist and GetVPCSubnetByName returns error when creating security group rule", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: &cidrSubnetName,
			RemoteType:     infrav1beta2.VPCSecurityGroupRuleRemoteTypeCIDR,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		securityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &inBoundDirection,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: &address,
			},
		}
		securityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		securityGroupRules = append(securityGroupRules, &securityGroupRule)

		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name:  ptr.To(securityGroupName),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: securityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil).AnyTimes()
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: &ipv4CIDRBlock}, nil)
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("failed to get vpc subnet"))
		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group rule of type CIDR doesn't exist and GetVPCSubnetByName returns nil when creating security group rule", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: &cidrSubnetName,
			RemoteType:     infrav1beta2.VPCSecurityGroupRuleRemoteTypeCIDR,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		securityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &inBoundDirection,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: &address,
			},
		}
		securityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		securityGroupRules = append(securityGroupRules, &securityGroupRule)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name:  ptr.To(securityGroupName),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: securityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil).AnyTimes()
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: &ipv4CIDRBlock}, nil)
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group rule of type CIDR doesn't exist and CreateSecurityGroupRule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			CIDRSubnetName: &cidrSubnetName,
			RemoteType:     infrav1beta2.VPCSecurityGroupRuleRemoteTypeCIDR,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionInbound,
			Source: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}

		securityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &inBoundDirection,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: &cidrBlock,
			},
		}
		securityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		securityGroupRules = append(securityGroupRules, &securityGroupRule)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name:  ptr.To(securityGroupName),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: securityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil).AnyTimes()
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: &ipv4CIDRBlock}, nil)
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(&vpcv1.Subnet{Ipv4CIDRBlock: &ipv4CIDRBlock}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, errors.New("failed to create security group rule"))
		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When Security group rule of type address doesn't exist and CreateSecurityGroupRule creates rule successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		remote := infrav1beta2.VPCSecurityGroupRuleRemote{
			Address:    &address,
			RemoteType: infrav1beta2.VPCSecurityGroupRuleRemoteTypeAddress,
		}

		securityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &outBoundDirection,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: &address,
			},
			ID: &ruleID,
		}

		rules := infrav1beta2.VPCSecurityGroupRule{
			Direction: infrav1beta2.VPCSecurityGroupRuleDirectionOutbound,
			Destination: &infrav1beta2.VPCSecurityGroupRulePrototype{
				Remotes:  append([]infrav1beta2.VPCSecurityGroupRuleRemote{}, remote),
				Protocol: infrav1beta2.VPCSecurityGroupRuleProtocolTCP,
				PortRange: &infrav1beta2.VPCSecurityGroupPortRange{MaximumPort: 65535,
					MinimumPort: 1,
				},
			},
		}
		vpcSecurityGroupRule := vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll{
			Direction: &outBoundDirection,
			Protocol:  &protocol,
			Remote: &vpcv1.SecurityGroupRuleRemote{
				CIDRBlock: &address,
			},
		}
		vpcSecurityGroupRules := []vpcv1.SecurityGroupRuleIntf{}
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1beta2.VPCSecurityGroup{
			Name:  ptr.To(securityGroupName),
			Rules: append([]*infrav1beta2.VPCSecurityGroupRule{}, &rules),
		}

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{
						ID: &securityGroupID,
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1beta2.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(&vpcv1.SecurityGroup{Name: &securityGroupName, ID: &securityGroupID, Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To(securityGroupID)}}, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&securityGroupRule, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: &securityGroupID}, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(&securityGroupRule, nil, nil)
		err := clusterScope.ReconcileVPCSecurityGroups()
		g.Expect(err).To(BeNil())
	})
}
