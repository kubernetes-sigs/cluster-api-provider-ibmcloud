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
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func setupClusterScope(clusterName string, mockvpc *mock.MockVpc) *ClusterScope {
	cluster := newCluster(clusterName)
	vpcCluster := newVPCCluster(clusterName)

	initObjects := []client.Object{
		cluster, vpcCluster,
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &ClusterScope{
		Client:        client,
		Logger:        klogr.New(),
		IBMVPCClient:  mockvpc,
		Cluster:       cluster,
		IBMVPCCluster: vpcCluster,
	}
}

func TestNewClusterScope(t *testing.T) {
	testCases := []struct {
		name   string
		params ClusterScopeParams
	}{
		{
			name: "Error when Cluster in nil",
			params: ClusterScopeParams{
				Cluster: nil,
			},
		},
		{
			name: "Error when IBMVPCCluster in nil",
			params: ClusterScopeParams{
				Cluster:       newCluster(clusterName),
				IBMVPCCluster: nil,
			},
		},
		{
			name: "Failed to create IBM VPC session",
			params: ClusterScopeParams{
				Cluster:       newCluster(clusterName),
				IBMVPCCluster: newVPCCluster(clusterName),
				Client:        testEnv.Client,
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClusterScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateVPC(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{
		Spec: infrav1beta1.IBMVPCClusterSpec{
			Region:        "foo-region",
			ResourceGroup: "foo-resource-group",
			VPC:           "foo-vpc",
			Zone:          "foo-zone",
		}}
	expectedOutput := &vpcv1.VPC{
		Name: core.StringPtr("foo-vpc"),
		DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
			ID: core.StringPtr("foo-security-group"),
		}}

	t.Run("Create VPC", func(t *testing.T) {
		listVpcsOptions := &vpcv1.ListVpcsOptions{}
		createVPCOptions := &vpcv1.CreateVPCOptions{}
		vpcCollection := &vpcv1.VPCCollection{
			Vpcs: []vpcv1.VPC{
				{
					Name: core.StringPtr("foo-vpc-1"),
					ID:   core.StringPtr("foo-vpc-1-id"),
					DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
						ID: core.StringPtr("foo-security-group-1"),
					},
				},
			},
		}
		vpc := &vpcv1.VPC{
			DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
				ID: core.StringPtr("foo-security-group"),
			},
			Name: core.StringPtr("foo-vpc"),
		}
		detailedResponse := &core.DetailedResponse{}
		securityGroupRuleOptions := &vpcv1.CreateSecurityGroupRuleOptions{}
		var securityGroupRuleIntf vpcv1.SecurityGroupRuleIntf

		t.Run("Should create VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(listVpcsOptions)).Return(vpcCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(createVPCOptions)).Return(vpc, detailedResponse, nil)
			mockvpc.EXPECT().CreateSecurityGroupRule(gomock.AssignableToTypeOf(securityGroupRuleOptions)).Return(securityGroupRuleIntf, detailedResponse, nil)
			out, err := scope.CreateVPC()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			vpcClusterCustom := infrav1beta1.IBMVPCCluster{
				Spec: infrav1beta1.IBMVPCClusterSpec{
					Region:        "foo-region-1",
					ResourceGroup: "foo-resource-group-1",
					VPC:           "foo-vpc-1",
					Zone:          "foo-zone-1",
				}}
			expectedOutput = &vpcv1.VPC{
				Name: core.StringPtr("foo-vpc-1"),
				ID:   core.StringPtr("foo-vpc-1-id"),
				DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
					ID: core.StringPtr("foo-security-group-1"),
				}}

			scope.IBMVPCCluster.Spec = vpcClusterCustom.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(listVpcsOptions)).Return(vpcCollection, detailedResponse, nil)
			out, err := scope.CreateVPC()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(listVpcsOptions)).Return(vpcCollection, detailedResponse, errors.New("Error when deleting subnet"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(listVpcsOptions)).Return(vpcCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(createVPCOptions)).Return(vpc, detailedResponse, errors.New("Error when deleting subnet"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating security group rule", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(listVpcsOptions)).Return(vpcCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(createVPCOptions)).Return(vpc, detailedResponse, nil)
			mockvpc.EXPECT().CreateSecurityGroupRule(gomock.AssignableToTypeOf(securityGroupRuleOptions)).Return(securityGroupRuleIntf, detailedResponse, errors.New("Failed security group rule creation"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteVPC(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{
		Spec: infrav1beta1.IBMVPCClusterSpec{
			VPC: "foo-vpc",
		},
		Status: infrav1beta1.IBMVPCClusterStatus{
			VPC: infrav1beta1.VPC{
				ID: "foo-vpc",
			},
		}}

	t.Run("Delete VPC", func(t *testing.T) {
		deleteVpcOptions := &vpcv1.DeleteVPCOptions{}
		detailedResponse := &core.DetailedResponse{}

		t.Run("Should delete VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteVPC(gomock.AssignableToTypeOf(deleteVpcOptions)).Return(detailedResponse, nil)
			err := scope.DeleteVPC()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteVPC(gomock.AssignableToTypeOf(deleteVpcOptions)).Return(detailedResponse, errors.New("Could not delete VPC"))
			err := scope.DeleteVPC()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestReserveFIP(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{
		Spec: infrav1beta1.IBMVPCClusterSpec{
			ResourceGroup: "foo-resource-group",
			VPC:           "foo-vpc",
			Zone:          "foo-zone",
		}}

	t.Run("Reserve FloatingIP", func(t *testing.T) {
		listFloatingIpsOptions := &vpcv1.ListFloatingIpsOptions{}
		floatingIPCollection := &vpcv1.FloatingIPCollection{
			FloatingIps: []vpcv1.FloatingIP{
				{
					Name: core.StringPtr("foo-cluster-1-control-plane"),
				},
			},
		}
		detailedResponse := &core.DetailedResponse{}
		floatingIPOptions := &vpcv1.CreateFloatingIPOptions{}

		t.Run("Should reserve FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			expectedOutput := &vpcv1.FloatingIP{
				Name: core.StringPtr("foo-cluster-control-plane"),
			}
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			floatingIP := &vpcv1.FloatingIP{Name: core.StringPtr("foo-cluster-control-plane")}
			mockvpc.EXPECT().ListFloatingIps(gomock.AssignableToTypeOf(listFloatingIpsOptions)).Return(floatingIPCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateFloatingIP(gomock.AssignableToTypeOf(floatingIPOptions)).Return(floatingIP, detailedResponse, nil)

			out, err := scope.ReserveFIP()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope("foo-cluster-1", mockvpc)
			expectedOutput := &vpcv1.FloatingIP{
				Name: core.StringPtr("foo-cluster-1-control-plane"),
			}

			mockvpc.EXPECT().ListFloatingIps(gomock.AssignableToTypeOf(listFloatingIpsOptions)).Return(floatingIPCollection, detailedResponse, nil)
			out, err := scope.ReserveFIP()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing FloatingIPs", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			mockvpc.EXPECT().ListFloatingIps(gomock.AssignableToTypeOf(listFloatingIpsOptions)).Return(floatingIPCollection, detailedResponse, errors.New("Error when listing FloatingIPs"))
			_, err := scope.ReserveFIP()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListFloatingIps(gomock.AssignableToTypeOf(listFloatingIpsOptions)).Return(floatingIPCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateFloatingIP(gomock.AssignableToTypeOf(floatingIPOptions)).Return(nil, detailedResponse, errors.New("Error when creating FloatingIP"))
			_, err := scope.ReserveFIP()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteFloatingIP(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{
		Status: infrav1beta1.IBMVPCClusterStatus{
			VPCEndpoint: infrav1beta1.VPCEndpoint{
				FIPID: core.StringPtr("foo-vpc"),
			},
		}}

	t.Run("Delete FloatingIP", func(t *testing.T) {
		deleteFIPOption := &vpcv1.DeleteFloatingIPOptions{}
		detailedResponse := &core.DetailedResponse{}

		t.Run("Should delete FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteFloatingIP(gomock.AssignableToTypeOf(deleteFIPOption)).Return(detailedResponse, nil)
			err := scope.DeleteFloatingIP()
			g.Expect(err).To(BeNil())
		})

		t.Run("Empty FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			vpcClusterCustom := infrav1beta1.IBMVPCCluster{
				Status: infrav1beta1.IBMVPCClusterStatus{
					VPCEndpoint: infrav1beta1.VPCEndpoint{
						FIPID: core.StringPtr(""),
					},
				}}
			scope.IBMVPCCluster.Status = vpcClusterCustom.Status
			err := scope.DeleteFloatingIP()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting FloatingIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteFloatingIP(gomock.AssignableToTypeOf(deleteFIPOption)).Return(detailedResponse, errors.New("Could not delete FloatingIP"))
			err := scope.DeleteFloatingIP()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestCreateSubnet(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{
		Spec: infrav1beta1.IBMVPCClusterSpec{
			Region:        "foo-region",
			ResourceGroup: "foo-resource-group",
			VPC:           "foo-vpc",
			Zone:          "foo-zone",
		},
		Status: infrav1beta1.IBMVPCClusterStatus{
			VPC: infrav1beta1.VPC{
				ID: *core.StringPtr("foo-vpc"),
			},
		}}

	t.Run("Create Subnet", func(t *testing.T) {
		listSubnetsOptions := &vpcv1.ListSubnetsOptions{}
		subnetCollection := &vpcv1.SubnetCollection{
			Subnets: []vpcv1.Subnet{
				{
					Name: core.StringPtr("foo-cluster-1-subnet"),
					ID:   core.StringPtr("foo-cluster-1-subnet-id"),
				},
			},
		}
		detailedResponse := &core.DetailedResponse{}
		listVPCAddressPrefixesOptions := &vpcv1.ListVPCAddressPrefixesOptions{}
		addressPrefixCollection := &vpcv1.AddressPrefixCollection{
			AddressPrefixes: []vpcv1.AddressPrefix{
				{
					CIDR: core.StringPtr("foo-vpc-cidr"),
					Zone: &vpcv1.ZoneReference{
						Name: core.StringPtr("foo-zone"),
					},
				},
			},
		}

		subnetOptions := &vpcv1.CreateSubnetOptions{}
		publicGatewayOptions := &vpcv1.CreatePublicGatewayOptions{}
		publicGateway := &vpcv1.PublicGateway{Name: core.StringPtr("foo-public-gateway"), ID: core.StringPtr("foo-public-gateway-id")}
		subnetPublicGatewayOptions := &vpcv1.SetSubnetPublicGatewayOptions{}

		t.Run("Should create Subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			expectedOutput := &vpcv1.Subnet{
				Name: core.StringPtr("foo-cluster-subnet"),
				ID:   core.StringPtr("foo-cluster-subnet-id"),
			}
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status

			subnet := &vpcv1.Subnet{
				Name: core.StringPtr(scope.IBMVPCCluster.Name + "-subnet"),
				ID:   core.StringPtr(scope.IBMVPCCluster.Name + "-subnet-id"),
			}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(subnetOptions)).Return(subnet, detailedResponse, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(publicGatewayOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().SetSubnetPublicGateway(gomock.AssignableToTypeOf(subnetPublicGatewayOptions)).Return(publicGateway, detailedResponse, nil)
			out, err := scope.CreateSubnet()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting Subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope("foo-cluster-1", mockvpc)
			expectedOutput := &vpcv1.Subnet{
				Name: core.StringPtr("foo-cluster-1-subnet"),
				ID:   core.StringPtr("foo-cluster-1-subnet-id"),
			}

			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			out, err := scope.CreateSubnet()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing Subnets", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, errors.New("Error when listing subnets"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when listing VPC AddressPerfixes", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, errors.New("Error when listing VPC AddressPrefixes"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error not found a valid CIDR for VPC in zone", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			scope.IBMVPCCluster.Spec.Zone = "foo-zone-temp"
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, nil)
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating PublicGateWay", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			subnet := &vpcv1.Subnet{}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(subnetOptions)).Return(subnet, detailedResponse, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(publicGatewayOptions)).Return(publicGateway, detailedResponse, errors.New("Error when creating PublicGateWay"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when attaching PublicGateWay", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			subnet := &vpcv1.Subnet{
				Name: core.StringPtr(scope.IBMVPCCluster.Name + "-subnet"),
				ID:   core.StringPtr(scope.IBMVPCCluster.Name + "-subnet-id"),
			}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(subnetOptions)).Return(subnet, detailedResponse, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(publicGatewayOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().SetSubnetPublicGateway(gomock.AssignableToTypeOf(subnetPublicGatewayOptions)).Return(publicGateway, detailedResponse, errors.New("Error when setting SubnetPublicGateWay"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating Subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(listSubnetsOptions)).Return(subnetCollection, detailedResponse, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(listVPCAddressPrefixesOptions)).Return(addressPrefixCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(subnetOptions)).Return(nil, detailedResponse, errors.New("Error when creating Subnet"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteSubnet(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcCluster := infrav1beta1.IBMVPCCluster{Spec: infrav1beta1.IBMVPCClusterSpec{
		VPC: "foo-vpc",
	}, Status: infrav1beta1.IBMVPCClusterStatus{
		Subnet: infrav1beta1.Subnet{
			ID: core.StringPtr("foo-vpc-subnet-id"),
		},
	}}

	t.Run("Delete Subnet", func(t *testing.T) {
		getPGWOptions := &vpcv1.GetSubnetPublicGatewayOptions{}
		detailedResponse := &core.DetailedResponse{}
		publicGateway := &vpcv1.PublicGateway{
			ID: core.StringPtr("foo-public-gateway-id"),
		}
		unsetPGWOption := &vpcv1.UnsetSubnetPublicGatewayOptions{}
		deletePGWOption := &vpcv1.DeletePublicGatewayOptions{
			ID: publicGateway.ID,
		}
		deleteSubnetOption := &vpcv1.DeleteSubnetOptions{}

		t.Run("Should delete subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(getPGWOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(unsetPGWOption)).Return(detailedResponse, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(deletePGWOption)).Return(detailedResponse, nil)
			mockvpc.EXPECT().DeleteSubnet(gomock.AssignableToTypeOf(deleteSubnetOption)).Return(detailedResponse, nil)
			err := scope.DeleteSubnet()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when unsetting publicgateway for subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(getPGWOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(unsetPGWOption)).Return(detailedResponse, errors.New("Error when unsetting publicgateway for subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when deleting publicgateway for subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(getPGWOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(unsetPGWOption)).Return(detailedResponse, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(deletePGWOption)).Return(detailedResponse, errors.New("Error when deleting publicgateway for subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when deleting subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(getPGWOptions)).Return(publicGateway, detailedResponse, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(unsetPGWOption)).Return(detailedResponse, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(deletePGWOption)).Return(detailedResponse, nil)
			mockvpc.EXPECT().DeleteSubnet(gomock.AssignableToTypeOf(deleteSubnetOption)).Return(detailedResponse, errors.New("Error when deleting subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}
