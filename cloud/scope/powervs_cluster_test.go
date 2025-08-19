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
	regionUtil "github.com/ppc64le-cloud/powervs-utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
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
					Spec: infrav1.IBMPowerVSClusterSpec{Zone: ptr.To("zone")},
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
						Zone: ptr.To("zone"),
						VPC:  &infrav1.VPCResourceReference{Region: ptr.To("eu-gb")},
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "Service Instance ID is set in status.ServiceInstanceID",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1.ResourceReference{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "DHCP server ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						DHCPServer: &infrav1.ResourceReference{
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
			g.Expect(pointer.Dereference(dhcpServerID)).To(Equal(pointer.Dereference(tc.expectedID)))
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPC: &infrav1.ResourceReference{
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
			g.Expect(pointer.Dereference(vpcID)).To(Equal(pointer.Dereference(tc.expectedID)))
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC subnet status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: make(map[string]infrav1.ResourceReference),
					},
				},
			},
		},
		{
			name: "empty subnet name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1.ResourceReference{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1.ResourceReference{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1.ResourceReference{
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
			g.Expect(pointer.Dereference(subnetID)).To(Equal(pointer.Dereference(tc.expectedID)))
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC subnet id is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1.ResourceReference{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG name is passed",
			clusterScope: PowerVSClusterScope{
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
			clusterScope: PowerVSClusterScope{
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
			clusterScope: PowerVSClusterScope{
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
		clusterScope PowerVSClusterScope
	}{
		{
			name: "VPC SG status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "VPC SG status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSecurityGroups: make(map[string]infrav1.VPCSecurityGroupStatus),
					},
				},
			},
		},
		{
			name: "empty SG ID is passed",
			clusterScope: PowerVSClusterScope{
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
			clusterScope: PowerVSClusterScope{
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
			clusterScope: PowerVSClusterScope{
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

func TestGetTransitGatewayID(t *testing.T) {
	testCases := []struct {
		name         string
		expectedID   *string
		clusterScope PowerVSClusterScope
	}{
		{
			name: "TransitGateway ID is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "TransitGateway ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						TransitGateway: &infrav1.TransitGateway{ID: ptr.To("tgID")},
					},
				},
			},
			expectedID: ptr.To(""),
		},
		{
			name: "TransitGateway ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						TransitGateway: &infrav1.TransitGatewayStatus{
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
			g.Expect(pointer.Dereference(tgID)).To(Equal(pointer.Dereference(tc.expectedID)))
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "LoadBalancer status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: make(map[string]infrav1.VPCLoadBalancerStatus),
					},
				},
			},
		},
		{
			name: "empty LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			g.Expect(pointer.Dereference(lbID)).To(Equal(pointer.Dereference(tc.expectedID)))
		})
	}
}

func TestGetLoadBalancerState(t *testing.T) {
	testCases := []struct {
		name          string
		lbName        string
		expectedState *infrav1.VPCLoadBalancerState
		clusterScope  PowerVSClusterScope
	}{
		{
			name: "LoadBalancer status is not set",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "LoadBalancer status is empty",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: make(map[string]infrav1.VPCLoadBalancerStatus),
					},
				},
			},
		},
		{
			name: "empty LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1.VPCLoadBalancerStateActive,
							},
						},
					},
				},
			},
		},
		{
			name: "invalid LoadBalancer name is passed",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1.VPCLoadBalancerStateActive,
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
							"lb": {
								State: infrav1.VPCLoadBalancerStateActive,
							},
						},
					},
				},
			},
			lbName:        "lb",
			expectedState: ptr.To(infrav1.VPCLoadBalancerStateActive),
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name:   "lb",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name:   "loadbalancer",
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
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
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
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
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("loadbalancer-id"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID:     ptr.To("loadbalancer-id1"),
							Public: core.BoolPtr(true),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
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
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
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
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
		},
		{
			name: "Resource group ID is set in spec",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("rgID")},
					},
				},
			},
			expectedID: "rgID",
		},
		{
			name: "Resource group ID is set in status",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: &infrav1.ResourceReference{
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
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("rgID")},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						ResourceGroup: &infrav1.ResourceReference{
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

func TestReconcileLoadBalancers(t *testing.T) {
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

	t.Run("When LoadBalancer ID is set and GetLoadbalancer fails to fetch loadbalancer details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: ptr.To("test-lb-instanceid"),
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed to fetch VPC load balancer details"))

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(loadBalancerReady).To(BeFalse())
	})

	t.Run("When LoadBalancer ID is set and the checkLoadBalancerStatus returns status is not active, indicating that load balancer is still not ready", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: ptr.To("test-lb-instanceid"),
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test-lb-instanceid"),
			ProvisioningStatus: ptr.To("update_pending"),
			Name:               ptr.To("test-lb"),
		}, nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(loadBalancerReady).To(BeFalse())
	})

	t.Run("Reconcile should not requeue when one load balancer is ready but another is still initializing or inactive", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: ptr.To("test-active-lb-instanceid"),
						},
						{
							ID: ptr.To("test-inactive-lb-instanceid"),
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To("test-active-lb-instanceid")}).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test- active-lb-instanceid"),
			ProvisioningStatus: ptr.To("active"),
			Name:               ptr.To("test-active-lb"),
		}, nil, nil)

		mockVpc.EXPECT().GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{ID: ptr.To("test-inactive-lb-instanceid")}).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test-inactive-lb-instanceid"),
			ProvisioningStatus: ptr.To("update_pending"),
			Name:               ptr.To("test-inactive-lb"),
		}, nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(loadBalancerReady).To(BeFalse())
	})

	t.Run("When LoadBalancer ID is set, checkLoadBalancerStatus returns status active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: ptr.To("test-lb-instanceid"),
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test-lb-instanceid"),
			Hostname:           ptr.To("test-lb-hostname"),
			ProvisioningStatus: ptr.To("active"),
			Name:               ptr.To("test-lb"),
		}, nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeTrue())
		g.Expect(err).To(BeNil())

		loadBalancerStatus, ok := clusterScope.IBMPowerVSCluster.Status.LoadBalancers["test-lb"]
		g.Expect(ok).To(BeTrue())
		g.Expect(loadBalancerStatus.ID).To(Equal(ptr.To("test-lb-instanceid")))
		g.Expect(loadBalancerStatus.State).To(BeEquivalentTo(infrav1.VPCLoadBalancerStateActive))
		g.Expect(loadBalancerStatus.Hostname).To(Equal(ptr.To("test-lb-hostname")))
	})

	t.Run("When LoadBalancer ID is not set and checkLoadBalancer fails to fetch load balancer", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: nil,
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, errors.New("failed to get load balancer by name"))

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When LoadBalancer ID is not set, the checkLoadBalancer function returns nil, indicating that the load balancer does not exist in the cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: nil,
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When LoadBalancer ID is not set, the checkLoadBalancer function still returns a valid loadBalancerStatus, indicating that the load balancer exists in the cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ProvisioningStatus: ptr.To("active"),
			Hostname:           ptr.To("test-lb-hostname"),
			Name:               ptr.To("test-lb"),
			ID:                 ptr.To("test-lb-instanceid"),
		}, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeTrue())
		g.Expect(err).To(BeNil())

		loadBalancerStatus, ok := clusterScope.IBMPowerVSCluster.Status.LoadBalancers["test-lb"]
		g.Expect(ok).To(BeTrue())
		g.Expect(loadBalancerStatus.State).To(BeEquivalentTo(infrav1.VPCLoadBalancerStateActive))
		g.Expect(loadBalancerStatus.ID).To(Equal(ptr.To("test-lb-instanceid")))
		g.Expect(loadBalancerStatus.Hostname).To(Equal(ptr.To("test-lb-hostname")))
	})

	t.Run("when checkLoadBalancerPort returns an error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterNetworkAPIServerPort := int32(9090)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
							AdditionalListeners: []infrav1.AdditionalListenerSpec{
								{
									Port: 9090,
								},
							},
						},
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: clusterNetworkAPIServerPort,
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When createLoadBalancer fails to create load balancer", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterAPIServerPort := int32(9090)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{

					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-gid"),
					},
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
					VPCSubnets: []infrav1.Subnet{
						{
							Name: ptr.To("test-subnet"),
							ID:   ptr.To("test-subnetid"),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"test-subnet": {
							ID: ptr.To("test-resource-reference-id"),
						},
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: clusterAPIServerPort,
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().CreateLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed loadBalancer creation"))
		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When createLoadBalancer successfully creates load balancer", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterAPIServerPort := int32(9090)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{

					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-gid"),
					},
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
					VPCSubnets: []infrav1.Subnet{
						{
							Name: ptr.To("test-subnet"),
							ID:   ptr.To("test-subnetid"),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"test-subnet": {
							ID: ptr.To("test-resource-reference-id"),
						},
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: clusterAPIServerPort,
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().CreateLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test-lb-id"),
			ProvisioningStatus: ptr.To("active"),
			Hostname:           ptr.To("test-lb-hostname"),
		}, nil, nil)

		loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(ctx)
		g.Expect(loadBalancerReady).To(BeFalse())
		g.Expect(err).To(BeNil())

		loadBalancer, ok := clusterScope.IBMPowerVSCluster.Status.LoadBalancers["test-lb"]
		g.Expect(ok).To(BeTrue())
		g.Expect(loadBalancer.State).To(BeEquivalentTo(infrav1.VPCLoadBalancerStateActive))
		g.Expect(loadBalancer.ControllerCreated).To(Equal(ptr.To(true)))
		g.Expect(loadBalancer.Hostname).To(Equal(ptr.To("test-lb-hostname")))
	})
}

func TestCreateLoadbalancer(t *testing.T) {
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

	t.Run("When createLoadBalancer returns error as resource group id is empty", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
			AdditionalListeners: []infrav1.AdditionalListenerSpec{
				{
					Port: int64(9090),
				},
			},
		}

		loadBalancerStatus, err := clusterScope.createLoadBalancer(ctx, lb)
		g.Expect(loadBalancerStatus).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When createLoadBalancer returns error as no subnets present for load balancer creation", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-gid"),
					},
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
			AdditionalListeners: []infrav1.AdditionalListenerSpec{
				{
					Port: int64(9090),
				},
			},
		}

		loadbalancerStatus, err := clusterScope.createLoadBalancer(ctx, lb)
		g.Expect(loadbalancerStatus).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When IBMVPCClient client CreateLoadBalancer returns error due to failed load balancer creation in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterAPIServerPort := int32(9090)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{

					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-gid"),
					},
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
					VPCSubnets: []infrav1.Subnet{
						{
							Name: ptr.To("test-subnet"),
							ID:   ptr.To("test-subnetid"),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"test-subnet": {
							ID: ptr.To("test-resource-reference-id"),
						},
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: clusterAPIServerPort,
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
			AdditionalListeners: []infrav1.AdditionalListenerSpec{
				{
					Port: int64(9090),
				},
			},
		}

		mockVpc.EXPECT().CreateLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed loadBalancer creation"))
		loadBalancerStatus, err := clusterScope.createLoadBalancer(ctx, lb)
		g.Expect(loadBalancerStatus).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When IBMVPCClient client CreateLoadBalancer successfully creates load balancer in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterAPIServerPort := int32(9090)
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{

					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-gid"),
					},
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
					VPCSubnets: []infrav1.Subnet{
						{
							Name: ptr.To("test-subnet"),
							ID:   ptr.To("test-subnetid"),
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"test-subnet": {
							ID: ptr.To("test-resource-reference-id"),
						},
					},
				},
			},
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: clusterAPIServerPort,
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
			AdditionalListeners: []infrav1.AdditionalListenerSpec{
				{
					Port: int64(9090),
				},
			},
		}

		mockVpc.EXPECT().CreateLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("test-lb-id"),
			ProvisioningStatus: ptr.To("active"),
			Hostname:           ptr.To("test-lb-hostname"),
		}, nil, nil)

		loadBalancerStatus, err := clusterScope.createLoadBalancer(ctx, lb)
		g.Expect(err).To(BeNil())
		g.Expect(loadBalancerStatus.State).To(BeEquivalentTo(infrav1.VPCLoadBalancerStateActive))
		g.Expect(loadBalancerStatus.ControllerCreated).To(Equal(ptr.To(true)))
		g.Expect(loadBalancerStatus.Hostname).To(Equal(ptr.To("test-lb-hostname")))
	})
}

func TestCheckLoadBalancerPort(t *testing.T) {
	t.Run("When load balancer listener port and powerVS API server port are same", func(t *testing.T) {
		g := NewWithT(t)
		lbName := "test-loadbalancer"
		port := 9090
		expectedErr := fmt.Errorf("port %d for the %s load balancer cannot be used as an additional listener port, as it is already assigned to the API server", port, lbName)

		clusterScope := PowerVSClusterScope{
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: int32(port),
					},
				},
			},
		}

		loadBalancer := infrav1.VPCLoadBalancerSpec{Name: lbName, AdditionalListeners: []infrav1.AdditionalListenerSpec{
			{
				Port: int64(port),
			},
		}}

		err := clusterScope.checkLoadBalancerPort(loadBalancer)
		g.Expect(err).To(MatchError(expectedErr))
	})

	t.Run("When load balancer listener port and powerVS API server port are different", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope := PowerVSClusterScope{
			Cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						APIServerPort: int32(8080),
					},
				},
			},
		}

		loadBalancer := infrav1.VPCLoadBalancerSpec{Name: "test-loadbalancer", AdditionalListeners: []infrav1.AdditionalListenerSpec{
			{
				Port: int64(9090),
			},
		}}

		err := clusterScope.checkLoadBalancerPort(loadBalancer)
		g.Expect(err).To(BeNil())
	})
}
func TestCheckLoadBalancer(t *testing.T) {
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

	t.Run("When checkLoadBalancer returns error due to failure in fetching load balancer details from cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: nil,
						},
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, errors.New("failed to get load balancer by name"))
		loadBalancerStatus, err := clusterScope.checkLoadBalancer(ctx, lb)
		g.Expect(loadBalancerStatus).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When checkLoadBalancer fails to returns load balancer status, indicating that load balancer does not exist in the cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							ID: nil,
						},
					},
				},
			},
		}

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)

		loadBalancerStatus, err := clusterScope.checkLoadBalancer(ctx, lb)
		g.Expect(loadBalancerStatus).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("When checkLoadBalancer returns valid load balancer status, indicating that load balancer exists in the cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVpc,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.VPCLoadBalancerSpec{
						{
							Name: "test-lb",
							ID:   nil,
						},
					},
				},
			},
		}

		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ProvisioningStatus: ptr.To("active"),
			Hostname:           ptr.To("test-lb-hostname"),
			Name:               ptr.To("test-lb"),
			ID:                 ptr.To("test-lb-instanceid"),
		}, nil)

		lb := infrav1.VPCLoadBalancerSpec{
			Name: "test-lb",
		}

		loadBalancerStatus, err := clusterScope.checkLoadBalancer(ctx, lb)
		g.Expect(err).To(BeNil())
		g.Expect(loadBalancerStatus.ID).To(Equal(ptr.To("test-lb-instanceid")))
		g.Expect(loadBalancerStatus.State).To(Equal(infrav1.VPCLoadBalancerStateActive))
		g.Expect(loadBalancerStatus.Hostname).To(Equal(ptr.To("test-lb-hostname")))
	})
}

func TestCheckLoadBalancerStatus(t *testing.T) {
	testcases := []struct {
		name           string
		loadbalancer   vpcv1.LoadBalancer
		expectedStatus bool
	}{
		{
			name:           "VPC load balancer is in active state",
			loadbalancer:   vpcv1.LoadBalancer{Name: ptr.To("loadbalancer-active"), ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive))},
			expectedStatus: true,
		},
		{
			name:           "VPC load balancer creation is in pending state",
			loadbalancer:   vpcv1.LoadBalancer{Name: ptr.To("loadbalancer-createPending"), ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending))},
			expectedStatus: false,
		},
		{
			name:           "VPC load balancer is in updating state",
			loadbalancer:   vpcv1.LoadBalancer{Name: ptr.To("loadbalancer-updatePending"), ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateUpdatePending))},
			expectedStatus: false,
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		clusterScope := PowerVSClusterScope{}
		t.Run(tc.name, func(_ *testing.T) {
			isReady := clusterScope.checkLoadBalancerStatus(ctx, tc.loadbalancer)
			g.Expect(isReady).To(Equal(tc.expectedStatus))
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and GetResourceInstance returns nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and instance is in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
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

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in status and instance is in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
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

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in spec and instance does not exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To(serviceInstanceID),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When service instance id is set in spec and instance exists in active state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
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

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("serviceInstanceIDSpec"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
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

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When create service instance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When created service instance is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, nil)

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When successfully created a new service instance", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
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

		requeue, err := clusterScope.ReconcilePowerVSServiceInstance(ctx)
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
			requeue, err := clusterScope.checkServiceInstanceState(ctx, tc.instance)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstanceID: "instance-id",
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		instanceID, requeue, err := clusterScope.isServiceInstanceExists(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("instance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get resource instance"))

		instanceID, requeue, err := clusterScope.isServiceInstanceExists(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("instance-name"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("instance"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{Name: ptr.To("instance"), State: ptr.To("unknown")}, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("instance"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetServiceInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{GUID: ptr.To("guid"), Name: ptr.To("instance"), State: ptr.To("active")}, nil)

		instanceID, requeue, err := clusterScope.isServiceInstanceExists(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
			},
		}

		instance, err := clusterScope.createServiceInstance(ctx)
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When zone is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
				},
			},
		}

		instance, err := clusterScope.createServiceInstance(ctx)
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create resource instance"))

		instance, err := clusterScope.createServiceInstance(ctx)
		g.Expect(instance).To(BeNil())
		g.Expect(err).NotTo(BeNil())
	})
	t.Run("When CreateResourceInstance returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resource-group-id"),
					},
					Zone: ptr.To("zone1"),
				},
			},
		}

		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{}, nil, nil)

		instance, err := clusterScope.createServiceInstance(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID")}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(vpcOutput, nil)

		requeue, err := clusterScope.ReconcileVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCByName error"))
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("Create new VPC when VPC doesnt exist", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			Cluster:      &clusterv1.Cluster{},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		mockVPC.EXPECT().CreateSecurityGroupRule(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("CreateVPC returns error"))

		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and exists in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				VPC: &infrav1.VPCResourceReference{ID: ptr.To("VPCID")},
			}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID")}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)

		requeue, err := clusterScope.ReconcileVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("VPCID")}}},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, fmt.Errorf("GetVPC returns error"))
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and GetVPC returns empty output", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("VPCID")}},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When VPC ID is set in status and VPC is in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID"), Status: ptr.To(string(infrav1.VPCStatePending))}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("VPCID")}},
			},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("VPCID")}}},
		}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		requeue, err := clusterScope.ReconcileVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
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
			Cluster:      &clusterv1.Cluster{},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
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
			Cluster:      &clusterv1.Cluster{},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
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
		resourceType infrav1.ResourceType
		expectedName *string
		clusterScope PowerVSClusterScope
	}{
		{
			name:         "Resource type is service instance and ServiceInstance is nil",
			resourceType: infrav1.ResourceTypeServiceInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-serviceInstance"),
		},
		{
			name:         "Resource type is service instance and ServiceInstance is not nil",
			resourceType: infrav1.ResourceTypeServiceInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{ServiceInstance: &infrav1.IBMPowerVSResourceReference{Name: ptr.To("ServiceInstanceName")}}},
			},
			expectedName: ptr.To("ServiceInstanceName"),
		},
		{
			name:         "Resource type is vpc and VPC is nil",
			resourceType: infrav1.ResourceTypeVPC,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-vpc"),
		},
		{
			name:         "Resource type is vpc and VPC is not nil",
			resourceType: infrav1.ResourceTypeVPC,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{VPC: &infrav1.VPCResourceReference{Name: ptr.To("VPCName")}}},
			},
			expectedName: ptr.To("VPCName"),
		},
		{
			name:         "Resource type is transit gateway and transitgateway is nil",
			resourceType: infrav1.ResourceTypeTransitGateway,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-transitgateway"),
		},
		{
			name:         "Resource type is transit gateway and transitgateway is not nil",
			resourceType: infrav1.ResourceTypeTransitGateway,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{TransitGateway: &infrav1.TransitGateway{Name: ptr.To("TransitGatewayName")}}},
			},
			expectedName: ptr.To("TransitGatewayName"),
		},
		{
			name:         "Resource type is dhcp server and dhcpserver is nil",
			resourceType: infrav1.ResourceTypeDHCPServer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName"),
		},
		{
			name:         "Resource type is dhcp server and dhcpserver is not nil",
			resourceType: infrav1.ResourceTypeDHCPServer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{DHCPServer: &infrav1.DHCPServer{Name: ptr.To("DHCPServerName")}}},
			},
			expectedName: ptr.To("DHCPServerName"),
		},
		{
			name:         "Resource type is dhcp server and dhcpserver is not nil and network is not nil",
			resourceType: infrav1.ResourceTypeDHCPServer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{Network: infrav1.IBMPowerVSResourceReference{Name: ptr.To("NetworkName")}}},
			},
			expectedName: ptr.To("NetworkName"),
		},
		{
			name:         "Resource type is cos instance and cos instance is nil",
			resourceType: infrav1.ResourceTypeCOSInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-cosinstance"),
		},
		{
			name:         "Resource type is cos instance and cos instance is not nil",
			resourceType: infrav1.ResourceTypeCOSInstance,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{CosInstance: &infrav1.CosInstance{Name: "CosInstanceName"}}},
			},
			expectedName: ptr.To("CosInstanceName"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is nil",
			resourceType: infrav1.ResourceTypeCOSBucket,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-cosbucket"),
		},
		{
			name:         "Resource type is cos bucket and cos bucket is not nil",
			resourceType: infrav1.ResourceTypeCOSBucket,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{CosInstance: &infrav1.CosInstance{BucketName: "CosBucketName"}}},
			},
			expectedName: ptr.To("CosBucketName"),
		},
		{
			name:         "Resource type is subnet",
			resourceType: infrav1.ResourceTypeSubnet,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-vpcsubnet"),
		},
		{
			name:         "Resource type is load balancer",
			resourceType: infrav1.ResourceTypeLoadBalancer,
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
			},
			expectedName: ptr.To("ClusterName-loadbalancer"),
		},
		{
			name: "Resource type is invalid",
			clusterScope: PowerVSClusterScope{
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{VPC: &infrav1.VPCResourceReference{ID: ptr.To("VPCID")}}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("VPCID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(vpcOutput, nil, nil)
		vpcID, err := clusterScope.checkVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(*vpcOutput.ID))
	})
	t.Run("When spec.VPC.ID is not set and GetVPCByName returns success", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		vpcOutput := &vpcv1.VPC{Name: ptr.To("VPCName"), ID: ptr.To("vpcID"), DefaultSecurityGroup: &vpcv1.SecurityGroupReference{ID: ptr.To("DefaultSecurityGroupID")}}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(vpcOutput, nil)

		vpcID, err := clusterScope.checkVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(*vpcOutput.ID))
	})

	t.Run("When spec.VPC.ID is not set and GetVPCByName returns empty vpcDetails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, nil)

		vpcID, err := clusterScope.checkVPC(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(vpcID).To(Equal(""))
	})
	t.Run("When spec.VPC.ID is not set and GetVPCByName returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient:      mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"}},
		}
		mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCByName returns error"))

		vpcID, err := clusterScope.checkVPC(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{DHCPServer: &infrav1.ResourceReference{ID: ptr.To("dhcpID")}}},
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
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{DHCPServer: &infrav1.ResourceReference{ID: ptr.To("dhcpID")}}},
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
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{DHCPServer: &infrav1.ResourceReference{ID: ptr.To("dhcpID")}}},
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
		clusterScope := PowerVSClusterScope{}
		t.Run(tc.name, func(_ *testing.T) {
			status, _ := clusterScope.checkDHCPServerStatus(ctx, tc.dhcpServer)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName},
				Spec: infrav1.IBMPowerVSClusterSpec{DHCPServer: &infrav1.DHCPServer{
					ID:        ptr.To("dhcpID"),
					DNSServer: ptr.To("DNSServer"),
					Cidr:      ptr.To("10.10.1.10/24"),
					Snat:      ptr.To(true),
				}},
			},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		dhcpID, err := clusterScope.createDHCPServer(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, nil)
		dhcpID, err := clusterScope.createDHCPServer(ctx)
		g.Expect(dhcpID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When CreateDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("CreateDHCPServer returns error"))
		dhcpID, err := clusterScope.createDHCPServer(ctx)
		g.Expect(dhcpID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestReconcileNetwork(t *testing.T) {
	var (
		mockPowerVS *mockP.MockPowerVS
		mockCtrl    *gomock.Controller
	)
	const netID = "netID"
	const dhcpID = "dhcpID"
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPowerVS = mockP.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("When network is available in cloud during status reconciliation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: &infrav1.ResourceReference{ID: ptr.To("netID")}}},
		}

		network := &models.Network{NetworkID: ptr.To("netID")}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)

		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When GetNetworkByID returns error during status reconciliation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{Network: &infrav1.ResourceReference{ID: ptr.To("netID")}}},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(nil, fmt.Errorf("GetNetworkByID error"))

		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When both network and DHCP server is available in cloud during status reconciliation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{DHCPServer: &infrav1.ResourceReference{ID: ptr.To("dhcpID")}, Network: &infrav1.ResourceReference{ID: ptr.To("netID")}}},
		}

		dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1.DHCPServerStateActive))}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		network := &models.Network{NetworkID: ptr.To("netID")}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)

		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When network is available in cloud but DHCP server is not available during status reconciliation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Status: infrav1.IBMPowerVSClusterStatus{DHCPServer: &infrav1.ResourceReference{ID: ptr.To("dhcpID")}, Network: &infrav1.ResourceReference{ID: ptr.To("netID")}}},
		}

		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("GetDHCPServer error"))
		network := &models.Network{NetworkID: ptr.To("netID")}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)

		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When networkID is set via spec and GetNetworkByID returns error ", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.IBMPowerVSResourceReference{ID: ptr.To("networkID")}}},
		}
		network := &models.Network{}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, fmt.Errorf("GetNetworkByID error"))

		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When networkID is set via spec and exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		network := &models.Network{NetworkID: ptr.To(netID)}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.IBMPowerVSResourceReference{ID: ptr.To(netID)}}},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(nil, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(network.NetworkID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When network name is set and exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		netName := "networkName"
		network := &models.NetworkReference{Name: ptr.To(netName), NetworkID: ptr.To(netID)}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.IBMPowerVSResourceReference{Name: ptr.To(netName)}}},
		}
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(nil, nil)
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any()).Return(network, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(netID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When network and DHCP server ID is set via spec and exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		network := &models.Network{NetworkID: ptr.To(netID)}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To(dhcpID), Network: &models.DHCPServerNetwork{ID: ptr.To(netID)}}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Network: infrav1.IBMPowerVSResourceReference{ID: ptr.To(netID)}, DHCPServer: &infrav1.DHCPServer{ID: ptr.To(dhcpID)}},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(netID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When network and DHCP server ID is set via spec but network is not belong to given dhcp server", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		network := &models.Network{NetworkID: ptr.To(netID)}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To(dhcpID), Network: &models.DHCPServerNetwork{ID: ptr.To("netID2")}}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{Network: infrav1.IBMPowerVSResourceReference{ID: ptr.To(netID)}, DHCPServer: &infrav1.DHCPServer{ID: ptr.To(dhcpID)}},
			},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When only DHCP server ID is set via spec and exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		network := &models.Network{NetworkID: ptr.To(netID)}
		dhcpServer := &models.DHCPServerDetail{ID: ptr.To(dhcpID), Network: &models.DHCPServerNetwork{ID: ptr.To(netID)}}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				DHCPServer: &infrav1.DHCPServer{ID: ptr.To(dhcpID)}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(netID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When only DHCP server ID is set but not exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				DHCPServer: &infrav1.DHCPServer{ID: ptr.To("dhcpID")}}},
		}
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("dhcp server by ID not found"))
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When DHCP server name is set and exists in IBM cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpServerName := "dhcpServerName"
		netName := dhcpNetworkName(dhcpServerName)
		network := &models.Network{NetworkID: ptr.To(netID)}
		dhcpServers := models.DHCPServers{&models.DHCPServer{ID: ptr.To(dhcpID), Network: &models.DHCPServerNetwork{ID: ptr.To(netID), Name: ptr.To(netName)}}}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				DHCPServer: &infrav1.DHCPServer{Name: ptr.To(dhcpServerName)}}},
		}
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(dhcpServers, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(netID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When no network details set via spec but dhcp network exist with cluster name", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterName := "clusterName"
		netName := dhcpNetworkName(clusterName)
		network := &models.Network{NetworkID: ptr.To(netID)}
		dhcpServers := models.DHCPServers{&models.DHCPServer{ID: ptr.To(dhcpID), Network: &models.DHCPServerNetwork{ID: ptr.To(netID), Name: ptr.To(netName)}}}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient:  mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		}
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(dhcpServers, nil)
		mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(netID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ControllerCreated).To(Equal(ptr.To(false)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeTrue())
	})
	t.Run("When network name is set in spec and DHCP server is created successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		dhcpNetwork := &models.DHCPServerNetwork{ID: ptr.To("dhcpNetworkID")}
		dhcpServer := &models.DHCPServer{ID: ptr.To("dhcpID"), Network: dhcpNetwork}
		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.IBMPowerVSResourceReference{Name: ptr.To("networkName")}}},
		}
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(nil, nil)
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any()).Return(nil, nil)
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(dhcpServer, nil)
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ID).To(Equal(dhcpServer.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.DHCPServer.ControllerCreated).To(Equal(ptr.To(true)))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ID).To(Equal(dhcpNetwork.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.Network.ControllerCreated).To(Equal(ptr.To(true)))
		g.Expect(err).To(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
	})
	t.Run("When network name is set in spec and createDHCPServer returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSClient: mockPowerVS,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{
				Network: infrav1.IBMPowerVSResourceReference{Name: ptr.To("networkName")}}},
		}
		mockPowerVS.EXPECT().GetAllDHCPServers().Return(nil, nil)
		mockPowerVS.EXPECT().GetNetworkByName(gomock.Any()).Return(nil, nil)
		mockPowerVS.EXPECT().CreateDHCPServer(gomock.Any()).Return(nil, fmt.Errorf("CreateDHCPServer error"))
		isNetworkAvailable, err := clusterScope.ReconcileNetwork(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(isNetworkAvailable).To(BeFalse())
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC:        &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}, {ID: ptr.To("subnet2ID"), Name: ptr.To("subnet2Name")}}},
			},
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}
		subnet1Options := &vpcv1.GetSubnetOptions{ID: subnet1Details.ID}
		mockVPC.EXPECT().GetSubnet(subnet1Options).Return(subnet1Details, nil, nil)
		subnet2Details := &vpcv1.Subnet{ID: ptr.To("subnet2ID"), Name: ptr.To("subnet2Name")}
		subnet2Options := &vpcv1.GetSubnetOptions{ID: subnet2Details.ID}
		mockVPC.EXPECT().GetSubnet(subnet2Options).Return(subnet2Details, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet2Details.Name].ID).To(Equal(subnet2Details.ID))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet2Details.Name].ControllerCreated).To(BeNil())
	})

	t.Run("When more VPCSubnets are set in the spec than the available VPC zones with zone explicitly mentioned for few subnets", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		var subnetZone *string

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{
						{Name: ptr.To("subnet1Name"), Zone: ptr.To("eu-de-2")},
						{Name: ptr.To("subnet2Name")},
						{Name: ptr.To("subnet3Name")},
						{Name: ptr.To("subnet4Name")},
						{Name: ptr.To("subnet5Name")},
					}},
			},
		}

		vpcZones, err := regionUtil.VPCZonesForVPCRegion(*clusterScope.IBMPowerVSCluster.Spec.VPC.Region)
		g.Expect(err).To(BeNil())
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil).Times(len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets))
		for i := 0; i < len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets); i++ {
			subnet1Options := &vpcv1.CreateSubnetOptions{}
			if clusterScope.IBMPowerVSCluster.Spec.VPCSubnets[i].Zone != nil {
				subnetZone = clusterScope.IBMPowerVSCluster.Spec.VPCSubnets[i].Zone
			} else {
				subnetZone = &vpcZones[i%len(vpcZones)]
			}
			subnet1Options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
				IPVersion:             ptr.To("ipv4"),
				TotalIpv4AddressCount: ptr.To(vpcSubnetIPAddressCount),
				Name:                  clusterScope.IBMPowerVSCluster.Spec.VPCSubnets[i].Name,
				VPC: &vpcv1.VPCIdentity{
					ID: clusterScope.IBMPowerVSCluster.Status.VPC.ID,
				},
				Zone: &vpcv1.ZoneIdentity{
					Name: subnetZone,
				},
				ResourceGroup: &vpcv1.ResourceGroupIdentity{
					ID: clusterScope.IBMPowerVSCluster.Spec.ResourceGroup.ID,
				},
			})
			subnetDetails := &vpcv1.Subnet{ID: ptr.To(fmt.Sprintf("subnet%dID", i+1)), Name: ptr.To(fmt.Sprintf("subnet%dName", i+1))}
			mockVPC.EXPECT().CreateSubnet(subnet1Options).Return(subnetDetails, nil, nil)
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
		for i := 1; i <= len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets); i++ {
			g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[fmt.Sprintf("subnet%dName", i)].ID).To(Equal(fmt.Sprintf("subnet%dID", i)))
			g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[fmt.Sprintf("subnet%dName", i)].ControllerCreated).To(BeTrue())
		}
	})

	t.Run("When VPCSubnets are set in the spec along with the zones", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{
						{Name: ptr.To("subnet1Name"), Zone: ptr.To("eu-de-1")},
						{Name: ptr.To("subnet2Name"), Zone: ptr.To("eu-de-2")},
						{Name: ptr.To("subnet3Name"), Zone: ptr.To("eu-de-3")},
					}},
			},
		}

		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil).Times(len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets))
		for i := 0; i < len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets); i++ {
			subnet1Options := &vpcv1.CreateSubnetOptions{}
			subnet1Options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
				IPVersion:             ptr.To("ipv4"),
				TotalIpv4AddressCount: ptr.To(vpcSubnetIPAddressCount),
				Name:                  clusterScope.IBMPowerVSCluster.Spec.VPCSubnets[i].Name,
				VPC: &vpcv1.VPCIdentity{
					ID: clusterScope.IBMPowerVSCluster.Status.VPC.ID,
				},
				Zone: &vpcv1.ZoneIdentity{
					Name: clusterScope.IBMPowerVSCluster.Spec.VPCSubnets[i].Zone,
				},
				ResourceGroup: &vpcv1.ResourceGroupIdentity{
					ID: clusterScope.IBMPowerVSCluster.Spec.ResourceGroup.ID,
				},
			})
			subnetDetails := &vpcv1.Subnet{ID: ptr.To(fmt.Sprintf("subnet%dID", i+1)), Name: ptr.To(fmt.Sprintf("subnet%dName", i+1))}
			mockVPC.EXPECT().CreateSubnet(subnet1Options).Return(subnetDetails, nil, nil)
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
		for i := 1; i <= len(clusterScope.IBMPowerVSCluster.Spec.VPCSubnets); i++ {
			g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[fmt.Sprintf("subnet%dName", i)].ID).To(Equal(fmt.Sprintf("subnet%dID", i)))
			g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[fmt.Sprintf("subnet%dName", i)].ControllerCreated).To(BeTrue())
		}
	})
	t.Run("When VPCSubnets are not set in spec and subnet doesnot exist in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("eu-de")}},
			},
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil).Times(3)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(subnet1Details, nil, nil).Times(3)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
		g.Expect(len(clusterScope.IBMPowerVSCluster.Status.VPCSubnet)).To(Equal(3))
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeTrue())
	})

	t.Run("When VPCSubnets are set in spec but zone is not specified", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets:    []infrav1.Subnet{{Name: ptr.To("subnet1Name")}}},
			},
		}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("subnet1Name")}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(subnet1Details, nil, nil)

		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ID).To(Equal(subnet1Details.ID))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.VPCSubnet[*subnet1Details.Name].ControllerCreated).To(BeTrue())
	})
	t.Run("When VPCZonesForVPCRegion returns error on providing invalid region", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("aa-dde")}},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When no vpcZones exists for a region", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("")}},
			},
		}
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When subnetID and subnetName is nil and vpc subnet exists in cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC:        &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{{Zone: ptr.To("eu-de-1")}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"subnet1Name": {ID: ptr.To("subnet1ID"), ControllerCreated: ptr.To(true)},
					}},
			},
		}
		vpcSubnet1Name := fmt.Sprintf("%s-vpcsubnet-0", clusterScope.IBMPowerVSCluster.ObjectMeta.Name)
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: &vpcSubnet1Name}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(subnet1Details, nil)

		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{
						{
							ID:   ptr.To("subnet1ID"),
							Name: ptr.To("subnet1Name"),
						},
					}},
			},
		}

		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("GetSubnet returns error"))
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When GetSubnet returns empty subnet", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
					VPCSubnets: []infrav1.Subnet{
						{
							ID:   ptr.To("subnet1ID"),
							Name: ptr.To("subnet1Name"),
						},
					}},
			},
		}

		mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When GetVPCSubnetByName in checkVPCSubnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
				},
			},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, fmt.Errorf("GetVPCSubnetByName returns error"))
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When createVPCSubnet returns error as Resourcegroup is not set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "ClusterName"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
				},
			},
		}
		mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.ReconcileVPCSubnets(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})
}

func TestSetVPCSubnetStatus(t *testing.T) {
	testCases := []struct {
		name         string
		subnetName   string
		resource     infrav1.ResourceReference
		clusterScope PowerVSClusterScope
	}{
		{
			name:       "VPC subnet status is nil",
			subnetName: "subnet1Name",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resource: infrav1.ResourceReference{ID: ptr.To("ID1")},
		},
		{
			name:       "VPC subnet status is not nil",
			subnetName: "subnet1Name",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPCSubnet: map[string]infrav1.ResourceReference{
							"subnet1Name": {
								ControllerCreated: ptr.To(true),
							},
						},
					},
				},
			},
			resource: infrav1.ResourceReference{ID: ptr.To("ID1"), ControllerCreated: ptr.To(true)},
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			tc.clusterScope.SetVPCSubnetStatus(ctx, tc.subnetName, tc.resource)
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
		vpcSubnetID, err := clusterScope.checkVPCSubnet(ctx, "subnet1Name")
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
		vpcSubnetID, err := clusterScope.checkVPCSubnet(ctx, "subnet1Name")
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
		vpcSubnetID, err := clusterScope.checkVPCSubnet(ctx, "subnet1Name")
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")}},
			},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}

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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("eu-de")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")}},
			},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
		subnet1Details := &vpcv1.Subnet{ID: ptr.To("subnet1ID"), Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}

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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{Spec: infrav1.IBMPowerVSClusterSpec{}},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1")}
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}}},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec:   infrav1.IBMPowerVSClusterSpec{ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}},
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")}}},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(nil, nil, fmt.Errorf("error creating subnet"))
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
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec:   infrav1.IBMPowerVSClusterSpec{ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")}},
				Status: infrav1.IBMPowerVSClusterStatus{VPC: &infrav1.ResourceReference{ID: ptr.To("vpcID")}}},
		}
		subnet := infrav1.Subnet{Name: ptr.To("ClusterName-vpcsubnet-eu-de-1"), Zone: ptr.To("eu-de-1")}
		mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(nil, nil, nil)
		subnetID, err := clusterScope.createVPCSubnet(subnet)
		g.Expect(subnetID).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
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
	powervsClusterScope := func() *PowerVSClusterScope {
		return &PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					LoadBalancers: map[string]infrav1.VPCLoadBalancerStatus{
						"lb": {
							ID:                ptr.To("lb-id"),
							ControllerCreated: ptr.To(true),
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
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive)),
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
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateDeletePending)),
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
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When load balancer is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = map[string]infrav1.VPCLoadBalancerStatus{
			"lb": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(false),
			},
		}
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteLoadBalancer(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When one load balancer is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = map[string]infrav1.VPCLoadBalancerStatus{
			"lb1": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(true),
			},
			"lb2": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(false),
			},
		}
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive)),
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
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = map[string]infrav1.VPCLoadBalancerStatus{
			"lb1": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(true),
			},
			"lb2": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(true),
			},
			"lb3": {
				ID:                ptr.To("lb-id"),
				ControllerCreated: ptr.To(true),
			},
		}
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive)),
		}, nil, nil).Times(3)
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
	powervsClusterScope := func() *PowerVSClusterScope {
		return &PowerVSClusterScope{
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
				ControllerCreated: ptr.To(false),
			},
			"sc2": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
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
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(false),
			},
		}
		clusterScope.IBMVPCClient = mockVpc
		err := clusterScope.DeleteVPCSecurityGroups(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestDeleteVPCSubnet(t *testing.T) {
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
	powervsClusterScope := func() *PowerVSClusterScope {
		return &PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSubnet: map[string]infrav1.ResourceReference{
						"subent1": {
							ID:                ptr.To("subent1"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
		}
	}

	t.Run("When VPC Subnet is not found", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("not found"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When DeleteSubnet returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete subnet"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When subnet deletion is in pending state", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To(string(infrav1.VPCSubnetStateDeleting))}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When GetSubnet returns error", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(nil, nil, errors.New("failed to get subnet"))
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteVPCSubnet successfully deletes subnet in cloud", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When deleting multiple subnet", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSubnet = map[string]infrav1.ResourceReference{
			"subent1": {
				ID:                ptr.To("subentid"),
				ControllerCreated: ptr.To(true),
			},
			"subent2": {
				ID:                ptr.To("subentid"),
				ControllerCreated: ptr.To(true),
			},
			"subent3": {
				ID:                ptr.To("subentid"),
				ControllerCreated: ptr.To(true),
			},
		}
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnetid"), Status: ptr.To("active")}, nil, nil).Times(3)
		mockVpc.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil).Times(3)
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When one subnet is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSubnet = map[string]infrav1.ResourceReference{
			"subent1": {
				ID:                ptr.To("subentid"),
				ControllerCreated: ptr.To(false),
			},
			"subent2": {
				ID:                ptr.To("subentid"),
				ControllerCreated: ptr.To(true),
			},
		}
		clusterScope.IBMVPCClient = mockVpc
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnetid"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When subnet is not created by controller", func(*testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Status.VPCSubnet = map[string]infrav1.ResourceReference{
			"subent1": {
				ID:                ptr.To("subent1"),
				ControllerCreated: ptr.To(false),
			},
		}
		clusterScope.IBMVPCClient = mockVpc
		requeue, err := clusterScope.DeleteVPCSubnet(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
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
	powervsClusterScope := func() *PowerVSClusterScope {
		return &PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID:                ptr.To("vpcid"),
						ControllerCreated: ptr.To(true),
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
		clusterScope.IBMPowerVSCluster.Status.VPC.ID = nil
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
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Status: ptr.To("active")}, nil, nil)
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
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Status: ptr.To("active")}, nil, nil)
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
		clusterScope.IBMPowerVSCluster.Status.VPC = &infrav1.ResourceReference{
			ID:                ptr.To("vpcid"),
			ControllerCreated: ptr.To(false),
		}
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
	powervsClusterScope := func() *PowerVSClusterScope {
		return &PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						ID:                ptr.To("transitgatewayID"),
						ControllerCreated: ptr.To(true),
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
							ID:                ptr.To("connectionID"),
						},
						VPCConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
							ID:                ptr.To("connectionID"),
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
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, nil).Times(2)
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
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		mockTG.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTG.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, nil).Times(2)
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
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = &infrav1.TransitGatewayStatus{
			ID:                ptr.To("transitgatewayID"),
			ControllerCreated: ptr.To(false),
			PowerVSConnection: &infrav1.ResourceReference{
				ControllerCreated: ptr.To(false),
				ID:                ptr.To("connectionID"),
			},
			VPCConnection: &infrav1.ResourceReference{
				ControllerCreated: ptr.To(false),
				ID:                ptr.To("connectionID"),
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
		clusterScope   PowerVSClusterScope
		expectedResult bool
	}{
		{
			name: "When resourceType is VPC and VPC status is nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeVPC,
			expectedResult: false,
		},
		{
			name: "When resourceType is VPC and VPC status is not nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						VPC: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			resourceType:   infrav1.ResourceTypeVPC,
			expectedResult: true,
		},
		{
			name: "When resourceType is ServiceInstance and ServiceInstance status is nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeServiceInstance,
			expectedResult: false,
		},
		{
			name: "When resourceType is ServiceInstance and ServiceInstance status is not nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						ServiceInstance: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			resourceType:   infrav1.ResourceTypeServiceInstance,
			expectedResult: true,
		},
		{
			name: "When resourceType is TransitGateway and TransitGateway status is nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeTransitGateway,
			expectedResult: false,
		},
		{
			name: "When resourceType is TransitGateway and TransitGateway status is not nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						TransitGateway: &infrav1.TransitGatewayStatus{
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			resourceType:   infrav1.ResourceTypeTransitGateway,
			expectedResult: true,
		},
		{
			name: "When resourceType is DHCPServer and DHCPServer status is nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeDHCPServer,
			expectedResult: false,
		},
		{
			name: "When resourceType is DHCPServer and DHCPServer status is not nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Status: infrav1.IBMPowerVSClusterStatus{
						DHCPServer: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			resourceType:   infrav1.ResourceTypeDHCPServer,
			expectedResult: true,
		},
		{
			name: "When resourceType is COSInstance and COSInstance status is nil",
			clusterScope: PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			},
			resourceType:   infrav1.ResourceTypeCOSInstance,
			expectedResult: false,
		},
		{
			name: "When resourceType is COSInstance and COSInstance status is not nil",
			clusterScope: PowerVSClusterScope{
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
			clusterScope: PowerVSClusterScope{
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

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance ID is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				COSInstance: &infrav1.ResourceReference{
					ControllerCreated: ptr.To(true),
				},
			},
		}}
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When COS instance state is pending_reclamation", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
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

		clusterScope := PowerVSClusterScope{
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

		clusterScope := PowerVSClusterScope{
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

		clusterScope := PowerVSClusterScope{
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
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To(string(infrav1.ServiceInstanceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, nil)
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("When COS instance state is active and DeleteResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
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
		cosInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("cosInstanceID"), State: ptr.To(string(infrav1.ServiceInstanceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(cosInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, fmt.Errorf("error deleting resource instance"))
		err := clusterScope.DeleteCOSInstance(ctx)
		g.Expect(err).To(Equal(fmt.Errorf("error deleting resource instance")))
	})
}

func TestDeleteServiceInstance(t *testing.T) {
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
	t.Run("When PowerVS service instance resource is not created by controller", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When PowerVS service instance ID is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				ServiceInstance: &infrav1.ResourceReference{
					ControllerCreated: ptr.To(true),
				},
			},
		}}
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When PowerVS service instance is in removed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID:                ptr.To("serviceInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		serviceInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("serviceInstanceID"), State: ptr.To(string(infrav1.ServiceInstanceStateRemoved))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(serviceInstance, nil, nil)
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When GetResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID:                ptr.To("serviceInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error getting resource instance"))
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err.Error()).To(Equal("failed to fetch PowerVS service instance: error getting resource instance"))
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When PowerVS service instance state is active and DeleteResourceInstance succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID:                ptr.To("serviceInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		serviceInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("serviceInstanceID"), State: ptr.To(string(infrav1.ServiceInstanceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(serviceInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When PowerVS instance state is active and DeleteResourceInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID:                ptr.To("serviceInstanceID"),
						ControllerCreated: ptr.To(true),
					},
				},
			},
			ResourceClient: mockResourceController,
		}
		serviceInstance := &resourcecontrollerv2.ResourceInstance{ID: ptr.To("serviceInstanceID"), State: ptr.To(string(infrav1.ServiceInstanceStateActive))}
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(serviceInstance, nil, nil)
		mockResourceController.EXPECT().DeleteResourceInstance(gomock.Any()).Return(nil, fmt.Errorf("error deleting resource instance"))
		requeue, err := clusterScope.DeleteServiceInstance(ctx)
		g.Expect(err).To(MatchError(ContainSubstring("error deleting resource instance")))
		g.Expect(requeue).To(BeFalse())
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

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{}}
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When PowerVS service instance is created by controller", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				DHCPServer: &infrav1.ResourceReference{
					ControllerCreated: ptr.To(true),
				},
				ServiceInstance: &infrav1.ResourceReference{
					ControllerCreated: ptr.To(true),
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

		clusterScope := PowerVSClusterScope{IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				DHCPServer: &infrav1.ResourceReference{
					ControllerCreated: ptr.To(true),
				},
				ServiceInstance: &infrav1.ResourceReference{},
			},
		}}
		err := clusterScope.DeleteDHCPServer(ctx)
		g.Expect(err).To(BeNil())
	})
	t.Run("When the DHCP server is not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					DHCPServer: &infrav1.ResourceReference{
						ID:                ptr.To("dhcpServerID"),
						ControllerCreated: ptr.To(true),
					},
					ServiceInstance: &infrav1.ResourceReference{},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					DHCPServer: &infrav1.ResourceReference{
						ID:                ptr.To("dhcpServerID"),
						ControllerCreated: ptr.To(true),
					},
					ServiceInstance: &infrav1.ResourceReference{},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					DHCPServer: &infrav1.ResourceReference{
						ID:                ptr.To("dhcpServerID"),
						ControllerCreated: ptr.To(true),
					},
					ServiceInstance: &infrav1.ResourceReference{},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					DHCPServer: &infrav1.ResourceReference{
						ID:                ptr.To("dhcpServerID"),
						ControllerCreated: ptr.To(true),
					},
					ServiceInstance: &infrav1.ResourceReference{},
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(true),
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

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ID:                ptr.To("powerVStgID"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 400}, fmt.Errorf("error getting transit gateway connection"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err.Error()).To(Equal("failed to get transit gateway powervs connection: error getting transit gateway connection"))
		g.Expect(requeue).To(BeFalse())
	})
	t.Run("When PowerVS connection is not found and VPC connection of transit gateway is deleted successfully", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ID:                ptr.To("powerVStgID"),
							ControllerCreated: ptr.To(true),
						},
						VPCConnection: &infrav1.ResourceReference{
							ID:                ptr.To("vpctgID"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		powerVSTGOptions := &tgapiv1.GetTransitGatewayConnectionOptions{TransitGatewayID: tg.ID, ID: ptr.To("powerVStgID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(powerVSTGOptions).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, nil)
		vpcTGOptions := &tgapiv1.GetTransitGatewayConnectionOptions{TransitGatewayID: tg.ID, ID: ptr.To("vpctgID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(vpcTGOptions).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})
	t.Run("When GetTransitGatewayConnection for VPC connection returns error", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(false),
						},
						VPCConnection: &infrav1.ResourceReference{
							ID:                ptr.To("vpctgID"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		vpcTGOptions := &tgapiv1.GetTransitGatewayConnectionOptions{TransitGatewayID: tg.ID, ID: ptr.To("vpctgID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(vpcTGOptions).Return(nil, &core.DetailedResponse{StatusCode: 500}, fmt.Errorf("error getting transit gateway connection"))
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err.Error()).To(Equal("failed to get transit gateway powervs connection: error getting transit gateway connection"))
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("When DeleteTransitGatewayConnection for VPC connection succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(false),
						},
						VPCConnection: &infrav1.ResourceReference{
							ID:                ptr.To("vpctgID"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		tgResponse := &tgapiv1.TransitGatewayConnectionCust{Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))}
		vpcTGOptions := &tgapiv1.GetTransitGatewayConnectionOptions{TransitGatewayID: tg.ID, ID: ptr.To("vpctgID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(vpcTGOptions).Return(tgResponse, &core.DetailedResponse{StatusCode: 200}, nil)
		mockTransitGateway.EXPECT().DeleteTransitGatewayConnection(gomock.Any()).Return(nil, nil)
		requeue, err := clusterScope.deleteTransitGatewayConnections(ctx, tg)
		g.Expect(err).To(BeNil())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("When VPC connection of transit gateway is not found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						PowerVSConnection: &infrav1.ResourceReference{
							ControllerCreated: ptr.To(false),
						},
						VPCConnection: &infrav1.ResourceReference{
							ID:                ptr.To("vpctgID"),
							ControllerCreated: ptr.To(true),
						},
					},
				},
			},
			TransitGatewayClient: mockTransitGateway,
		}
		tg := &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID")}
		vpcTGOptions := &tgapiv1.GetTransitGatewayConnectionOptions{TransitGatewayID: tg.ID, ID: ptr.To("vpctgID")}
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(vpcTGOptions).Return(nil, &core.DetailedResponse{StatusCode: ResourceNotFoundCode}, nil)
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-region",
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error fetching instance by name"))

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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
					Zone: ptr.To("test-zone"),
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.ServiceInstanceStateActive)),
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
					Zone: ptr.To("test-zone"),
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
					Zone: ptr.To("test-zone"),
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockResourceController.EXPECT().CreateResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to create COS service instance"))

		err = clusterScope.ReconcileCOSInstance(ctx)
		g.Expect(err).ToNot(BeNil())
		g.Expect(clusterScope.IBMPowerVSCluster.Status.COSInstance).To(BeNil())
	})

	t.Run("When fetch for API_KEY fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-region",
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.ServiceInstanceStateActive)),
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}
		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}
		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}
		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					CosInstance: &infrav1.CosInstance{
						BucketRegion: "test-bucket-region",
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resource-group-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}
		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error listing COS instances"))

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When COS service instance is not found in IBM Cloud", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance).To(BeNil())
		g.Expect(err).To(BeNil())
	})

	t.Run("When COS service instance exists but state is not active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
					},
				},
			},
		}

		mockResourceController.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("test-cos-resource-name"),
			State: ptr.To(string(infrav1.ServiceInstanceStateActive)),
		}, nil)

		cosResourceInstance, err := clusterScope.checkCOSServiceInstance(ctx)
		g.Expect(cosResourceInstance.Name).To(Equal(ptr.To("test-cos-resource-name")))
		g.Expect(cosResourceInstance.State).To(Equal(ptr.To(string(infrav1.ServiceInstanceStateActive))))
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			COSClient:      mockCOSController,
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resourcegroup-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			ResourceClient: mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("test-resourcegroup-id"),
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("test-serviceinstance-id"),
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						ID: ptr.To("transitGatewayID"),
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						ID: ptr.To("transitGatewayID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(nil, nil, errors.New("failed to get transitGateway connections"))
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When TransitGatewayID is set in status and TransitGateway not in available state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					TransitGateway: &infrav1.TransitGatewayStatus{
						ID: ptr.To("transitGatewayID"),
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
		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: &infrav1.TransitGateway{
						ID: ptr.To("transitGatewayID"),
					},
					VPC: &infrav1.VPCResourceReference{
						ID: ptr.To("vpcID"),
					},
					ServiceInstance: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID")}, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeFalse())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeTrue())
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
	t.Run("When TransitGatewayID is set in spec and returns error while getting TransitGateway details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: &infrav1.TransitGateway{
						ID: ptr.To("transitGatewayID"),
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().GetTransitGatewayByName(gomock.Any()).Return(nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID")}, nil, nil)
		requeue, err := clusterScope.ReconcileTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeTrue())
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When PowerVS service Instance and VPC details are not set in status and fails to create transit gateway", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster:    &infrav1.IBMPowerVSCluster{},
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
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeTrue())
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
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeFalse())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeFalse())
		g.Expect(requeue).To(BeFalse())
		g.Expect(err).To(BeNil())
	})

	t.Run("WHen PowerVSConnection exist and is in pending state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		conn = append(conn, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
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

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStatePending))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When VPCConnection status exist and is in failed state", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateFailed))})
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

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		conn = append(conn, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateFailed))})
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

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(requeue).To(BeTrue())
		g.Expect(err).To(BeNil())
	})

	t.Run("When PowerVSConnection doesn't exist and returns error while creating it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		clusterScope := makePowerVSClusterScope(mockTransitGateway, mockVPC, mockResourceController)

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("vpc"), NetworkType: ptr.To("vpc"), NetworkID: ptr.To("vpc-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
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

		conn := append([]tgapiv1.TransitGatewayConnectionCust{}, tgapiv1.TransitGatewayConnectionCust{Name: ptr.To("pvs"), NetworkType: ptr.To("power_virtual_server"), NetworkID: ptr.To("pvs-crn"), Status: ptr.To(string(infrav1.TransitGatewayConnectionStateAttached))})
		mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{Connections: conn}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("vpc-crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("pvs-crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID")}, nil, nil)
		requeue, err := clusterScope.checkAndUpdateTransitGatewayConnections(ctx, &tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName")})
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeTrue())
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

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
			},
		}

		err := clusterScope.createTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Fails to get TransitGateway location and routing", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("zone-ID"),
					VPC:           &infrav1.VPCResourceReference{},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		err := clusterScope.createTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Return error while creating TransitGateway", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(nil, nil, errors.New("failed to create transit Gateway"))
		err := clusterScope.createTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Creates TransitGateway but return error when getting VPC details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To("pending")}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(nil, nil, errors.New("failed to get vpc"))
		err := clusterScope.createTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeTrue())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Creates TransitGateway but return error while getting PowerVS details", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("failed to get power vs instance"))
		err := clusterScope.createTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeTrue())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When PowerVSConnection creation is completed but fails to create VPCConnection", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(nil, nil, errors.New("failed to create transit Gateway connection"))
		err := clusterScope.createTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When local routing is configured but global routing is required", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: &infrav1.TransitGateway{
						GlobalRouting: ptr.To(false),
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("us-east-1"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		err := clusterScope.createTransitGateway(ctx)
		g.Expect(clusterScope.IBMPowerVSCluster.Status.TransitGateway).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("When global routing is set to true", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		clusterScope := PowerVSClusterScope{
			TransitGatewayClient: mockTransitGateway,
			IBMVPCClient:         mockVPC,
			ResourceClient:       mockResourceController,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					TransitGateway: &infrav1.TransitGateway{
						GlobalRouting: ptr.To(true),
					},
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{ID: ptr.To("resourceGroupID")},
					Zone:          ptr.To("zone-ID"),
					VPC:           &infrav1.VPCResourceReference{Region: ptr.To("region")},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					ServiceInstance: &infrav1.ResourceReference{
						ID: ptr.To("serviceInstanceID"),
					},
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("vpcID"),
					},
				},
			},
		}

		mockTransitGateway.EXPECT().CreateTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{ID: ptr.To("transitGatewayID"), Name: ptr.To("transitGatewayName"), Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}, nil, nil)
		mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{CRN: ptr.To("crn")}, nil, nil)
		mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{CRN: ptr.To("crn")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("pvs-connID")}, nil, nil)
		mockTransitGateway.EXPECT().CreateTransitGatewayConnection(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCust{ID: ptr.To("vpc-connID")}, nil, nil)
		err := clusterScope.createTransitGateway(ctx)
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ID).To(BeEquivalentTo("transitGatewayID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID).To(BeEquivalentTo("pvs-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated).To(BeTrue())
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID).To(BeEquivalentTo("vpc-connID"))
		g.Expect(*clusterScope.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated).To(BeTrue())
		g.Expect(err).To(BeNil())
	})
}

func makePowerVSClusterScope(mockTransitGateway *tgmock.MockTransitGateway, mockVPC *mock.MockVpc, mockResourceController *mockRC.MockResourceController) PowerVSClusterScope {
	clusterScope := PowerVSClusterScope{
		TransitGatewayClient: mockTransitGateway,
		IBMVPCClient:         mockVPC,
		ResourceClient:       mockResourceController,
		IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
			Status: infrav1.IBMPowerVSClusterStatus{
				TransitGateway: &infrav1.TransitGatewayStatus{
					ID: ptr.To("transitGatewayID"),
				},
				ServiceInstance: &infrav1.ResourceReference{
					ID: ptr.To("serviceInstanceID"),
				},
				VPC: &infrav1.ResourceReference{
					ID: ptr.To("vpcID"),
				},
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
		clusterScope := PowerVSClusterScope{
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
		clusterScope := PowerVSClusterScope{
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
		clusterScope := PowerVSClusterScope{
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		vpcSecurityGroupStatus[securityGroupName] = infrav1.VPCSecurityGroupStatus{ID: ptr.To("securityGroupID"), RuleIDs: []*string{ptr.To("ruleID")}, ControllerCreated: ptr.To(true)}
		clusterScope := PowerVSClusterScope{
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

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to get security group"))
		err := clusterScope.ReconcileVPCSecurityGroups(ctx)
		g.Expect(err).ToNot(BeNil())
	})
	t.Run("When SecurityGroup Name is set  and returns error while getting SecurityGroupRules", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		vpcSecurityGroupStatus := make(map[string]infrav1.VPCSecurityGroupStatus)
		vpcSecurityGroupStatus[securityGroupName] = infrav1.VPCSecurityGroupStatus{ID: &securityGroupID, RuleIDs: []*string{ptr.To("ruleID")}, ControllerCreated: ptr.To(true)}
		clusterScope := PowerVSClusterScope{
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

		mockVPC.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{ID: ptr.To("securityGroupID"), Name: ptr.To("securityGroupName")}, nil, nil)
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		vpcSecurityGroupStatus["securityGroupName"] = infrav1.VPCSecurityGroupStatus{ID: ptr.To("securityGroupID"), RuleIDs: []*string{}, ControllerCreated: ptr.To(false)}
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			Name:  ptr.To("securityGroupName"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPCSecurityGroups: vpcSecurityGroupStatus,
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, vpcSecurityGroup),
				},
			},
		}

		securityGroupDetails := &vpcv1.SecurityGroup{Name: ptr.To("securityGroupName"), ID: ptr.To("securityGroupID"), Rules: vpcSecurityGroupRules, VPC: &vpcv1.VPCReference{ID: ptr.To("VPCID")}}
		mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(securityGroupDetails, nil)
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		var vpcSecurityGroupRules []vpcv1.SecurityGroupRuleIntf
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		var vpcSecurityGroupRules []vpcv1.SecurityGroupRuleIntf
		vpcSecurityGroupRules = append(vpcSecurityGroupRules, &vpcSecurityGroupRule)
		vpcSecurityGroup := infrav1.VPCSecurityGroup{
			ID:    ptr.To("securityGroupID"),
			Rules: append([]*infrav1.VPCSecurityGroupRule{}, &rules),
		}
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("securityGroupID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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

		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					VPC: &infrav1.ResourceReference{
						ID: ptr.To("VPCID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resourceGroupID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resourceGroupID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resourceGroupID"),
					},
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
		clusterScope := PowerVSClusterScope{
			IBMVPCClient: mockVPC,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPCSecurityGroups: append([]infrav1.VPCSecurityGroup{}, infrav1.VPCSecurityGroup{
						Name: ptr.To("securityGroupName"),
					}),
					ResourceGroup: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("resourceGroupID"),
					},
				},
			},
		}

		mockVPC.EXPECT().CreateSecurityGroup(gomock.Any()).Return(nil, nil, errors.New("failed to create SecurityGroup"))
		sg, err := clusterScope.createVPCSecurityGroup(ctx, clusterScope.IBMPowerVSCluster.Spec.VPCSecurityGroups[0])
		g.Expect(sg).To(BeNil())
		g.Expect(err).ToNot(BeNil())
	})
}
