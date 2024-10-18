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

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	mockP "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
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
	t.Run("When VPC already exists in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID")}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(vpcOutput, nil)

		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal(vpcOutput.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPC.ControllerCreated).To(BeFalse())
	})
	t.Run("When GetVPCByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCByName error"))
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("Create new VPC when VPC doesnt exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			Cluster:      &capiv1beta1.Cluster{Spec: capiv1beta1.ClusterSpec{ClusterNetwork: nil}},
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal(vpcOutput.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPC.ControllerCreated).To(BeTrue())
	})
	t.Run("When CreateVPC returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("CreateVPC returns error"))

		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and exists in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				VPC: &infrav1beta2.VPCResourceReference{ID: ptr.To("VPCID")},
			}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID")}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)

		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPC.ID).To(Equal(vpcOutput.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPC.ControllerCreated).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and GetVPC returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("VPCID")}}},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("GetVPC returns error"))
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and GetVPC returns empty output", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("VPCID")}},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and VPC is in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID"), Status: ptr.To(string(infrav1beta2.VPCStatePending))}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("VPCID")}},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When VPC ID is set in status and VPC is in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID")}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("VPCID")}}},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		requeue, err := clusterScope.ReconcileVPC()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
}

func TestPowerVSScopeCreateVPC(t *testing.T) {
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
	t.Run("When resourceGroupID is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
		}

		vpcID, err := clusterScope.createVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(vpcID).To(BeNil())
	})
	t.Run("When resourceGroupID is set and create VPC is successful", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			Cluster:      &capiv1beta1.Cluster{Spec: capiv1beta1.ClusterSpec{ClusterNetwork: nil}},
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, nil)

		vpcID, err := clusterScope.createVPC()
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(vpcOutput.ID))
	})

	t.Run("When resourceGroupID is not nil and CreateSecurityGroupRule returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			Cluster:      &capiv1beta1.Cluster{Spec: capiv1beta1.ClusterSpec{ClusterNetwork: nil}},
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, fmt.Errorf("CreateSecurityGroupRule returns error"))
		vpcID, err := clusterScope.createVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(vpcID).To(BeNil())
	})
}

func TestGetServiceName(t *testing.T) {
	testCases := []struct {
		name         string
		resourceType infrav1beta2.ResourceType
		expectedName *string
		clusterScope PowerVSClusterScope
	}{
		{
			name:         "Resource type is service instance and ServiceInstance is nil",
			resourceType: infrav1beta2.ResourceTypeServiceInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-serviceInstance"),
		},
		{
			name:         "Resource type is service instance and ServiceInstance is not nil",
			resourceType: infrav1beta2.ResourceTypeServiceInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{Name: ptr.To("ServiceInstanceName")}}},
			},
			expectedName: ptr.To("ServiceInstanceName"),
		},
		{
			name:         "Resource type is network and Network is nil",
			resourceType: infrav1beta2.ResourceTypeNetwork,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("DHCPSERVERClusterName_Private"),
		},
		{
			name:         "Resource type is network and Network is not nil",
			resourceType: infrav1beta2.ResourceTypeNetwork,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{Network: infrav1beta2.IBMPowerVSResourceReference{Name: ptr.To("NetworkName")}}},
			},
			expectedName: ptr.To("NetworkName"),
		},
		{
			name:         "Resource type is vpc and VPC is nil",
			resourceType: infrav1beta2.ResourceTypeVPC,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-vpc"),
		},
		{
			name:         "Resource type is vpc and VPC is not nil",
			resourceType: infrav1beta2.ResourceTypeVPC,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{VPC: &infrav1beta2.VPCResourceReference{Name: ptr.To("VPCName")}}},
			},
			expectedName: ptr.To("VPCName"),
		},
		{
			name:         "Resource type is transit gateway and transitgateway is nil",
			resourceType: infrav1beta2.ResourceTypeTransitGateway,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-transitgateway"),
		},
		{
			name:         "Resource type is transit gateway and transitgateway is not nil",
			resourceType: infrav1beta2.ResourceTypeTransitGateway,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{TransitGateway: &infrav1beta2.TransitGateway{Name: ptr.To("TransitGatewayName")}}},
			},
			expectedName: ptr.To("TransitGatewayName"),
		},
		{
			name:         "Resource type is dhcp server and dhcpserver is nil",
			resourceType: infrav1beta2.ResourceTypeDHCPServer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName"),
		},
		{
			name:         "Resource type is dhcp server and dhcpserver is not nil",
			resourceType: infrav1beta2.ResourceTypeDHCPServer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{DHCPServer: &infrav1beta2.DHCPServer{Name: ptr.To("DHCPServerName")}}},
			},
			expectedName: ptr.To("DHCPServerName"),
		},
		{
			name:         "Resource type is cos instance and cos instance is nil",
			resourceType: infrav1beta2.ResourceTypeCOSInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-cosinstance"),
		},
		{
			name:         "Resource type is cos instance and cos instance is not nil",
			resourceType: infrav1beta2.ResourceTypeCOSInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{CosInstance: &infrav1beta2.CosInstance{Name: "CosInstanceName"}}},
			},
			expectedName: ptr.To("CosInstanceName"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is nil",
			resourceType: infrav1beta2.ResourceTypeCOSBucket,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-cosbucket"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is not nil",
			resourceType: infrav1beta2.ResourceTypeCOSBucket,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{CosInstance: &infrav1beta2.CosInstance{BucketName: "CosBucketName"}}},
			},
			expectedName: ptr.To("CosBucketName"),
		},
		{
			name:         "Resource type is subnet",
			resourceType: infrav1beta2.ResourceTypeSubnet,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-vpcsubnet"),
		},
		{
			name:         "Resource type is load balancer",
			resourceType: infrav1beta2.ResourceTypeLoadBalancer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-loadbalancer"),
		},
		{
			name: "Resource type is invalid",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
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

func TestGetVPCByName(t *testing.T) {
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
	t.Run("When GetVPCByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCByName returns error"))
		vpcResponse, err := clusterScope.getVPCByName()
		g.Expect(err).ToNot(BeNil())
		g.Expect(vpcResponse).To(BeNil())
	})
	t.Run("When GetVPCByName returns valid vpc details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(vpcOutput, nil)

		vpcResponse, err := clusterScope.getVPCByName()
		g.Expect(err).To(BeNil())
		g.Expect(vpcResponse.ID).To(Equal(vpcOutput.ID))
	})
}

func TestCheckVPC(t *testing.T) {
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
	t.Run("When GetVPC returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{VPC: &infrav1beta2.VPCResourceReference{ID: ptr.To("VPCID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		vpcID, err := clusterScope.checkVPC()
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(*vpcOutput.ID))
	})
	t.Run("When spec.VPC.ID is not set and GetVPCByName returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(vpcOutput, nil)

		vpcID, err := clusterScope.checkVPC()
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(*vpcOutput.ID))
	})

	t.Run("When spec.VPC.ID is not set and GetVPCByName returns empty vpcDetails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)

		vpcID, err := clusterScope.checkVPC()
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(""))
	})
	t.Run("When spec.VPC.ID is not set and GetVPCByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCByName returns error"))

		vpcID, err := clusterScope.checkVPC()
		g.Expect(err).ToNot(BeNil())
		g.Expect(vpcID).To(Equal(""))
	})
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
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Status: infrav1beta2.IBMPowerVSClusterStatus{DHCPServer: &infrav1beta2.ResourceReference{ID: ptr.To("dhcpID")}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("GetDHCPServer returns error"))
		isActive, err := clusterScope.isDHCPServerActive()
		g.Expect(err).ToNot(BeNil())
		g.Expect(isActive).To(BeFalse())
	})
	t.Run("When checkDHCPServerStatus returns error state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1beta2.DHCPServerStateError))}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Status: infrav1beta2.IBMPowerVSClusterStatus{DHCPServer: &infrav1beta2.ResourceReference{ID: ptr.To("dhcpID")}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)

		isActive, err := clusterScope.isDHCPServerActive()
		g.Expect(err).ToNot(BeNil())
		g.Expect(isActive).To(BeFalse())
	})
	t.Run("When checkDHCPServerStatus returns active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1beta2.DHCPServerStateActive))}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Status: infrav1beta2.IBMPowerVSClusterStatus{DHCPServer: &infrav1beta2.ResourceReference{ID: ptr.To("dhcpID")}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)

		isActive, err := clusterScope.isDHCPServerActive()
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
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDBuild"), Status: ptr.To(string(infrav1beta2.DHCPServerStateBuild))},
			expectedStatus: false,
		},
		{
			name:           "DHCP server is in active state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDActive"), Status: ptr.To(string(infrav1beta2.DHCPServerStateActive))},
			expectedStatus: true,
		},
		{
			name:           "DHCP server is in error state",
			dhcpServer:     models.DHCPServerDetail{ID: ptr.To("dhcpIDError"), Status: ptr.To(string(infrav1beta2.DHCPServerStateError))},
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
		clusterScope := PowerVSClusterScope{}
		t.Run(tc.name, func(_ *testing.T) {
			status, _ := clusterScope.checkDHCPServerStatus(tc.dhcpServer)
			g.Expect(status).To(Equal(tc.expectedStatus))
		})
	}
}

func TestCreateDHCPServer(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
		clusterName = "clusterName"
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When dhcpServerDetails is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpNetwork := &models.DHCPServerNetwork{ID: ptr.To("dhcpNetworkID")}
		dhcpServer := &models.DHCPServer{ID: ptr.To("dhcpID"), Network: dhcpNetwork}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer()
		g.Expect(dhcpID).To(Equal(dhcpServer.ID))
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(dhcpNetwork.ID))
	})

	t.Run("When dhcpServerDetails are all set but createDHCPServer returns server with no network", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServer := &models.DHCPServer{ID: ptr.To("dhcpID")}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{DHCPServer: &infrav1beta2.DHCPServer{
					ID:        ptr.To("dhcpID"),
					DNSServer: ptr.To("DNSServer"),
					Cidr:      ptr.To("10.10.1.10/24"),
					Snat:      ptr.To(true),
				}},
			},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer()
		g.Expect(dhcpID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When dhcpServerDetails has no dnsserver,cidr or snat set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpNetwork := &models.DHCPServerNetwork{ID: ptr.To("dhcpNetworkID")}
		dhcpServer := &models.DHCPServer{ID: ptr.To("dhcpID"), Network: dhcpNetwork}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer()
		g.Expect(dhcpID).To(Equal(dhcpServer.ID))
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(dhcpNetwork.ID))
	})

	t.Run("When CreateDHCPServer returns empty dhcp server", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, nil)
		dhcpID, err := clusterScope.createDHCPServer()
		g.Expect(dhcpID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When CreateDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("CreateDHCPServer returns error"))
		dhcpID, err := clusterScope.createDHCPServer()
		g.Expect(dhcpID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
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
	t.Run("When GetDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Status: infrav1beta2.IBMPowerVSClusterStatus{DHCPServer: &infrav1beta2.ResourceReference{ID: ptr.To("dhcpID")}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("GetDHCPServer error"))

		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DHCPServer exists and is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Status: infrav1beta2.IBMPowerVSClusterStatus{DHCPServer: &infrav1beta2.ResourceReference{ID: ptr.To("dhcpID")}}},
		}

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1beta2.DHCPServerStateActive))}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)

		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
	t.Run("When DHCPID is empty and GetNetworkByID returns error ", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				Network: infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("networkID")}}},
		}
		network := &models.Network{}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, fmt.Errorf("GetNetworkByID error"))

		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DHCPID is empty and networkID is not empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		network := &models.Network{NetworkID: ptr.To("networkID")}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				Network: infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("networkID")}}},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(network.NetworkID))
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
	t.Run("When network name is set in spec and DHCP server is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpNetwork := &models.DHCPServerNetwork{ID: ptr.To("dhcpNetworkID")}
		dhcpServer := &models.DHCPServer{ID: ptr.To("dhcpID"), Network: dhcpNetwork}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				Network: infrav1beta2.IBMPowerVSResourceReference{Name: ptr.To("networkName")}}},
		}
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any()).Return(nil, nil)
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpServer.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(dhcpNetwork.ID))
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When network name is set in spec and createDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{
				Network: infrav1beta2.IBMPowerVSResourceReference{Name: ptr.To("networkName")}}},
		}
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any()).Return(nil, nil)
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("CreateDHCPServer error"))
		requeue, err := clusterScope.ReconcileNetwork()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
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
	t.Run("When VPCSubnets are set in spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}, {ID: ptr.To("subnet2ID"), Name: ptr.To("subnet2Name")}}},
			},
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}
		subnet1Options := &vpcv1.GetSubnetOptions{ID: subnet1Details.ID}
		mockVPC.EXPECT().GetSubnet(subnet1Options).Return(subnet1Details, nil, nil)
		subnet2Details := &vpcv1.Subnet{ID: ptr.To("subnet2ID"), Name: ptr.To("subnet2Name")}
		subnet2Options := &vpcv1.GetSubnetOptions{ID: subnet2Details.ID}
		mockVPC.EXPECT().GetSubnet(subnet2Options).Return(subnet2Details, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet2Details.Name].ID).To(Equal(subnet2Details.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeNil())
	})

	t.Run("When VPCSubnets are not set in spec and subnet doesnot exist in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1beta2.VPCResourceReference{Region: ptr.To("eu-de")}},
			},
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), "eu-de-1").Return("10.240.20.0/18", nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(subnet1Details, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
		g.Expect(len(clusterScope.IBMPowerVSCluster.Status.VPCSubnet)).To(Equal(1))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeTrue())
	})
	t.Run("When VPCZonesForVPCRegion returns error on providing invalid region", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPC: &infrav1beta2.VPCResourceReference{Region: ptr.To("aa-dde")}},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When no vpcZones exists for a region", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPC: &infrav1beta2.VPCResourceReference{Region: ptr.To("")}},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When subnetID and subnetName is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{{}}},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1beta2.ResourceReference{
						"subnet1Name": {ID: ptr.To("subnet1ID"), ControllerCreated: ptr.To(true)},
					}},
			},
		}
		vpcSubnet1Name := fmt.Sprintf("%s-vpcsubnet-0", clusterScope.IBMPowerVSCluster.ObjectMeta.Name)
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: &vpcSubnet1Name}
		mockVPC.EXPECT().GetVPCSubnetByName(vpcSubnet1Name).Return(subnet1Details, nil)

		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeFalse())
	})

	t.Run("When GetSubnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{
						{
							ID:   ptr.To("subnet1ID"),
							Name: ptr.To("subnet1Name"),
						},
					}},
			},
		}

		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("GetSubnet returns error"))
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When GetSubnet returns empty subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{
						{
							ID:   ptr.To("subnet1ID"),
							Name: ptr.To("subnet1Name"),
						},
					}},
			},
		}

		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When GetVPCSubnetByName in checkVPCSubnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{{}}},
			},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCSubnetByName returns error"))
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When createVPCSubnet returns error as Resourcegroup is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					VPCSubnets: []infrav1beta2.Subnet{{}}},
			},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets()
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestSetVPCSubnetStatus(t *testing.T) {
	testCases := []struct {
		name         string
		subnetName   string
		resource     infrav1beta2.ResourceReference
		clusterScope PowerVSClusterScope
	}{
		{
			name:       "VPC subnet status is nil",
			subnetName: "subnet1Name",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
			resource: infrav1beta2.ResourceReference{ID: ptr.To("ID1")},
		},
		{
			name:       "VPC subnet status is not nil",
			subnetName: "subnet1Name",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1beta2.ResourceReference{
							"subnet1Name": {
								ControllerCreated: ptr.To(false),
							},
						},
					},
				},
			},
			resource: infrav1beta2.ResourceReference{ID: ptr.To("ID1"), ControllerCreated: ptr.To(true)},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			tc.clusterScope.SetVPCSubnetStatus(tc.subnetName, tc.resource)
			g.Expect(tc.clusterScope.IBMPowerVSCluster.Status.VPCSubnet[tc.subnetName].ID).To(Equal(tc.resource.ID))
			g.Expect(tc.clusterScope.IBMPowerVSCluster.Status.VPCSubnet[tc.subnetName].ControllerCreated).To(Equal(tc.resource.ControllerCreated))
		})
	}
}

func TestCheckVPCSubnet(t *testing.T) {
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
	t.Run("When GetVPCSubnetByName returns nil as vpc subnet does not exist in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		vpcSubnetID, err := clusterScope.checkVPCSubnet("subnet1Name")
		g.Expect(vpcSubnetID).To(Equal(""))
		g.Expect(err).To(BeNil())
	})
	t.Run("When GetVPCSubnetByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCSubnetByName returns error"))
		vpcSubnetID, err := clusterScope.checkVPCSubnet("subnet1Name")
		g.Expect(vpcSubnetID).To(Equal(""))
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When GetVPCSubnetByName returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(subnet1Details, nil)
		vpcSubnetID, err := clusterScope.checkVPCSubnet("subnet1Name")
		g.Expect(vpcSubnetID).To(Equal(*subnet1Details.ID))
		g.Expect(err).To(BeNil())
	})
}

func TestCreateVPCSubnet(t *testing.T) {
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
	t.Run("When zone is set and createVPCSubnet returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")}},
			},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}

		mockVPC.EXPECT().GetSubnetAddrPrefix(*clusterScope.IBMPowerVSCluster.Status.VPC.ID, *subnet.Zone).Return("10.240.20.0/18", nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(subnet1Details, nil, nil)
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(Equal(subnet1Details.ID))
		g.Expect(err).To(BeNil())
	})
	t.Run("When zone is not set and createVPCSubnet returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1beta2.VPCResourceReference{Region: ptr.To("eu-de")},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")}},
			},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}

		mockVPC.EXPECT().GetSubnetAddrPrefix(*clusterScope.IBMPowerVSCluster.Status.VPC.ID, "eu-de-1").Return("10.240.20.0/18", nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(subnet1Details, nil, nil)
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(Equal(subnet1Details.ID))
		g.Expect(err).To(BeNil())
	})

	t.Run("When resourceGroupID is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{Spec: infrav1beta2.IBMPowerVSClusterSpec{}},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When vpc is not set in Status and GetVPCID returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}, Status: infrav1beta2.IBMPowerVSClusterStatus{}},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When GetSubnetAddrPrefix returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec:   infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}},
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")}}},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("GetSubnetAddrPrefix returns error"))
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When CreateSubnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec:   infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}},
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")}}},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), gomock.Any()).Return("10.10.1.10/24", nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("GetSubnetAddrPrefix returns error"))
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When CreateSubnet returns empty subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec:   infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}},
				Status: infrav1beta2.IBMPowerVSClusterStatus{VPC: &infrav1beta2.ResourceReference{ID: ptr.To("vpcID")}}},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), gomock.Any()).Return("10.10.1.10/24", nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(nil, nil, nil)
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When VPCZonesForVPCRegion returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC: &infrav1beta2.VPCResourceReference{Region: ptr.To("aadd")}},
			},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When VPCZonesForVPCRegion returns zero zones for the region set in spec", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				Spec: infrav1beta2.IBMPowerVSClusterSpec{ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC: &infrav1beta2.VPCResourceReference{Region: ptr.To("")}},
			},
		}
		subnet := infrav1beta2.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}
