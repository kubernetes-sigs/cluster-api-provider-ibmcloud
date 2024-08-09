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
			name: "Service Instance ID is set in spec.ServiceInstanceID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "specServiceInstanceID",
					},
				},
			},
			expectedID: "specServiceInstanceID",
		},
		{
			name: "Service Instance ID is set in spec.ServiceInstance.ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstance: &infrav1beta2.IBMPowerVSResourceReference{
							ID: ptr.To("specServiceInstanceID"),
						},
					},
				},
			},
			expectedID: "specServiceInstanceID",
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
		{
			name: "Spec Service Instance ID takes precedence over status Service Instance ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "specServiceInstanceID",
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1beta2.ResourceReference{
							ID: ptr.To("statusServiceInstanceID"),
						},
					},
				},
			},
			expectedID: "specServiceInstanceID",
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
			name: "DHCP server ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						DHCPServer: &infrav1beta2.DHCPServer{ID: ptr.To("dhcpserverid")},
					},
				},
			},
			expectedID: ptr.To("dhcpserverid"),
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
		{
			name: "Spec DHCP server ID takes precedence over status DHCP Server ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						DHCPServer: &infrav1beta2.DHCPServer{ID: ptr.To("dhcpserverid")},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						DHCPServer: &infrav1beta2.ResourceReference{
							ID: ptr.To("dhcpserveridstatus"),
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
			name: "VPC ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						VPC: &infrav1beta2.VPCResourceReference{ID: ptr.To("vpcID")},
					},
				},
			},
			expectedID: ptr.To("vpcID"),
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
		{
			name: "spec VPC ID takes precedence over status VPC ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						VPC: &infrav1beta2.VPCResourceReference{ID: ptr.To("vpcID")},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						VPC: &infrav1beta2.ResourceReference{
							ID: ptr.To("vpcID1"),
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
			name: "VPC subnet id is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						VPCSubnets: []infrav1beta2.Subnet{
							{
								ID: ptr.To("subnet1"),
							},
							{
								ID: ptr.To("subnet2"),
							},
						},
					},
				},
			},
			expectedIDs: []*string{ptr.To("subnet1"), ptr.To("subnet2")},
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
	testCases := []struct {
		name             string
		lbName           string
		expectedHostName *string
		clusterScope     PowerVSClusterScope
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
								Hostname: ptr.To("hostname"),
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
								Hostname: ptr.To("hostname"),
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
								Hostname: ptr.To("hostname"),
							},
						},
					},
				},
			},
			lbName:           "lb",
			expectedHostName: ptr.To("hostname"),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			lbName := tc.clusterScope.GetLoadBalancerHostName(tc.lbName)
			g.Expect(utils.DereferencePointer(lbName)).To(Equal(utils.DereferencePointer(tc.expectedHostName)))
		})
	}
}

func TestPublicLoadBalancer(t *testing.T) {
	testCases := []struct {
		name         string
		expectedLB   *infrav1beta2.VPCLoadBalancerSpec
		clusterScope PowerVSClusterScope
	}{
		{
			name: "IBMPowerVSCluster spec is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
			expectedLB: &infrav1beta2.VPCLoadBalancerSpec{
				Name:   "-loadbalancer",
				Public: ptr.To(true),
			},
		},
		{
			name: "one public loadbalancer is configured",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name:   "lb",
								Public: ptr.To(true),
							},
						},
					},
				},
			},
			expectedLB: &infrav1beta2.VPCLoadBalancerSpec{
				Name:   "lb",
				Public: ptr.To(true),
			},
		},
		{
			name: "multiple public loadbalancer is configured",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name:   "lb",
								Public: ptr.To(true),
							},
							{
								Name:   "lb2",
								Public: ptr.To(true),
							},
						},
					},
				},
			},
			expectedLB: &infrav1beta2.VPCLoadBalancerSpec{
				Name:   "lb",
				Public: ptr.To(true),
			},
		},
		{
			name: "only private loadbalancer is configured",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name:   "lb",
								Public: ptr.To(false),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			lbSpec := tc.clusterScope.PublicLoadBalancer()
			g.Expect(lbSpec).To(Equal(tc.expectedLB))
		})
	}
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
	t.Run("When service instance id is set and GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ServiceInstanceID = serviceInstanceID

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set and GetResourceInstance returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ServiceInstanceID = serviceInstanceID

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set and instance in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			instance := &resourcecontrollerv2.ResourceInstance{
				Name:  ptr.To("test-instance"),
				State: ptr.To("failed"),
			}
			mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(instance, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ServiceInstanceID = serviceInstanceID

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set and instance in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			instance := &resourcecontrollerv2.ResourceInstance{
				Name:  ptr.To("test-instance"),
				State: ptr.To("active"),
			}
			mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(instance, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ServiceInstanceID = serviceInstanceID

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When isServiceInstanceExists returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to get service instance"))
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance exists in cloud and in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			instance := &resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("instance-GUID"),
				Name:  ptr.To("test-instance"),
				State: ptr.To("active"),
			}
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(instance, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ID).To(Equal("instance-GUID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.ServiceInstance.ControllerCreated).To(BeFalse())
	})

	t.Run("When create service instance return error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create resource instance"))
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ResourceGroup = &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When created service instance is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ResourceGroup = &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance()
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When successfully created new service instance", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			instance := &resourcecontrollerv2.ResourceInstance{
				GUID: ptr.To("instance-GUID"),
				Name: ptr.To("test-instance"),
			}

			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(instance, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.ResourceGroup = &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}

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
	t.Run("When get service instance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to get service instance"))
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When get service instance returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})
	t.Run("When checkServiceInstanceState returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("unknown")}, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When checkServiceInstanceState returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("unknown")}, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instanceID, requeue, err := clusterScope.isServiceInstanceExists()
		g.Expect(instanceID).To(Equal(""))
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When isServiceInstanceExists returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("guid"), Name: ptr.To("instance"), State: ptr.To("active")}, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

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

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.IBMPowerVSCluster.Spec.ResourceGroup = nil
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When zone is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())
		clusterScope.IBMPowerVSCluster.Spec.Zone = nil

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create resource instance"))
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScopeParams := getPowerVSClusterScopeParams()
		clusterScopeParams.ResourceControllerFactory = func() (resourcecontroller.ResourceController, error) {
			mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{}, nil, nil)
			return mockResourceController, nil
		}
		clusterScope, err := NewPowerVSClusterScope(clusterScopeParams)
		g.Expect(err).To(BeNil())

		instance, err := clusterScope.createServiceInstance()
		g.Expect(instance).ToNot(BeNil())
		g.Expect(err).To(BeNil())
	})
}

func getPowerVSClusterScopeParams() PowerVSClusterScopeParams {
	return PowerVSClusterScopeParams{
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
				Zone:          ptr.To("zone"),
				VPC:           &infrav1beta2.VPCResourceReference{Region: ptr.To("eu-gb")},
				ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{ID: ptr.To("rg-id")},
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
	}
}
