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
	"fmt"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
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
				g.Expect(subnetIDs).To(Equal(tc.expectedIDs))
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
			subnetID, _, _ := tc.clusterScope.GetVPCSecurityGroupByName(tc.sgName)
			g.Expect(utils.DereferencePointer(subnetID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
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
			subnetID, _, _ := tc.clusterScope.GetVPCSecurityGroupByID(tc.sgID)
			g.Expect(utils.DereferencePointer(subnetID)).To(Equal(utils.DereferencePointer(tc.expectedID)))
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
			expectedID: ptr.To("tgID"),
		},
		{
			name: "TransitGateway ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						TransitGateway: &infrav1beta2.ResourceReference{
							ID: ptr.To("tgID"),
						},
					},
				},
			},
			expectedID: ptr.To("tgID"),
		},
		{
			name: "spec TransitGateway ID takes precedence over status TransitGateway ID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						TransitGateway: &infrav1beta2.TransitGateway{ID: ptr.To("tgID")},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						TransitGateway: &infrav1beta2.ResourceReference{
							ID: ptr.To("tgID1"),
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
			fmt.Println(lbID)
			fmt.Println(tc.expectedID)
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
			fmt.Println(lbState)
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
			lbID := tc.clusterScope.GetLoadBalancerHostName(tc.lbName)
			g.Expect(utils.DereferencePointer(lbID)).To(Equal(utils.DereferencePointer(tc.expectedHostName)))
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
			vpcID := tc.clusterScope.GetResourceGroupID()
			g.Expect(vpcID).To(Equal(tc.expectedID))
		})
	}
}
