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
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
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
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			Region:        "foo-region",
			ResourceGroup: "foo-resource-group",
			VPC:           "foo-vpc",
			Zone:          "foo-zone",
		},
	}

	t.Run("Create VPC", func(t *testing.T) {
		vpc := &vpcv1.VPC{
			DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
				ID: core.StringPtr("foo-security-group"),
			},
			Name: core.StringPtr("foo-vpc"),
		}

		t.Run("Should create VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			var securityGroupRuleIntf vpcv1.SecurityGroupRuleIntf
			scope := setupClusterScope(clusterName, mockvpc)
			expectedOutput := &vpcv1.VPC{
				Name: core.StringPtr("foo-vpc"),
				DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
					ID: core.StringPtr("foo-security-group"),
				}}
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(&vpcv1.ListVpcsOptions{})).Return(&vpcv1.VPCCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(&vpcv1.CreateVPCOptions{})).Return(vpc, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSecurityGroupRule(gomock.AssignableToTypeOf(&vpcv1.CreateSecurityGroupRuleOptions{})).Return(securityGroupRuleIntf, &core.DetailedResponse{}, nil)
			out, err := scope.CreateVPC()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			vpcClusterCustom := infrav1beta2.IBMVPCCluster{
				Spec: infrav1beta2.IBMVPCClusterSpec{
					Region:        "foo-region-1",
					ResourceGroup: "foo-resource-group-1",
					VPC:           "foo-vpc-1",
					Zone:          "foo-zone-1",
				}}
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
			expectedOutput := &vpcv1.VPC{
				Name: core.StringPtr("foo-vpc-1"),
				ID:   core.StringPtr("foo-vpc-1-id"),
				DefaultSecurityGroup: &vpcv1.SecurityGroupReference{
					ID: core.StringPtr("foo-security-group-1"),
				}}
			scope.IBMVPCCluster.Spec = vpcClusterCustom.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(&vpcv1.ListVpcsOptions{})).Return(vpcCollection, &core.DetailedResponse{}, nil)
			out, err := scope.CreateVPC()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(&vpcv1.ListVpcsOptions{})).Return(&vpcv1.VPCCollection{}, &core.DetailedResponse{}, errors.New("Failed to list VPC"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(&vpcv1.ListVpcsOptions{})).Return(&vpcv1.VPCCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(&vpcv1.CreateVPCOptions{})).Return(&vpcv1.VPC{}, &core.DetailedResponse{}, errors.New("Failed to create VPC"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating security group rule", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			var securityGroupRuleIntf vpcv1.SecurityGroupRuleIntf
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			mockvpc.EXPECT().ListVpcs(gomock.AssignableToTypeOf(&vpcv1.ListVpcsOptions{})).Return(&vpcv1.VPCCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateVPC(gomock.AssignableToTypeOf(&vpcv1.CreateVPCOptions{})).Return(vpc, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSecurityGroupRule(gomock.AssignableToTypeOf(&vpcv1.CreateSecurityGroupRuleOptions{})).Return(securityGroupRuleIntf, &core.DetailedResponse{}, errors.New("Failed security group rule creation"))
			_, err := scope.CreateVPC()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteVPC(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			VPC: "foo-vpc",
		},
		Status: infrav1beta2.IBMVPCClusterStatus{
			VPC: infrav1beta2.VPC{
				ID: "foo-vpc",
			},
		},
	}

	t.Run("Delete VPC", func(t *testing.T) {
		t.Run("Should delete VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteVPC(gomock.AssignableToTypeOf(&vpcv1.DeleteVPCOptions{})).Return(&core.DetailedResponse{}, nil)
			err := scope.DeleteVPC()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting VPC", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().DeleteVPC(gomock.AssignableToTypeOf(&vpcv1.DeleteVPCOptions{})).Return(&core.DetailedResponse{}, errors.New("Could not delete VPC"))
			err := scope.DeleteVPC()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestCreateSubnet(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			Region:        "foo-region",
			ResourceGroup: "foo-resource-group",
			VPC:           "foo-vpc",
			Zone:          "foo-zone",
		},
		Status: infrav1beta2.IBMVPCClusterStatus{
			VPC: infrav1beta2.VPC{
				ID: *core.StringPtr("foo-vpc"),
			},
		},
	}

	t.Run("Create Subnet", func(t *testing.T) {
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
		publicGateway := &vpcv1.PublicGateway{Name: core.StringPtr("foo-public-gateway"), ID: core.StringPtr("foo-public-gateway-id")}

		t.Run("Should create Subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			expectedOutput := &vpcv1.Subnet{
				Name: core.StringPtr("foo-cluster-subnet"),
				ID:   core.StringPtr("foo-cluster-subnet-id"),
			}
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status

			subnet := &vpcv1.Subnet{
				Name: core.StringPtr(scope.IBMVPCCluster.Name + subnetSuffix),
				ID:   core.StringPtr(scope.IBMVPCCluster.Name + "-subnet-id"),
			}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(addressPrefixCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(&vpcv1.CreateSubnetOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(&vpcv1.CreatePublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().SetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.SetSubnetPublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			out, err := scope.CreateSubnet()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting Subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope("foo-cluster-1", mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			subnetCollection := &vpcv1.SubnetCollection{
				Subnets: []vpcv1.Subnet{
					{
						Name: core.StringPtr("foo-cluster-1-subnet"),
						ID:   core.StringPtr("foo-cluster-1-subnet-id"),
					},
				},
			}
			expectedOutput := &vpcv1.Subnet{
				Name: core.StringPtr("foo-cluster-1-subnet"),
				ID:   core.StringPtr("foo-cluster-1-subnet-id"),
			}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(subnetCollection, &core.DetailedResponse{}, nil)
			out, err := scope.CreateSubnet()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing Subnets", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, errors.New("Error when listing subnets"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when listing VPC AddressPerfixes", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(&vpcv1.AddressPrefixCollection{}, &core.DetailedResponse{}, errors.New("Error when listing VPC AddressPrefixes"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error not found a valid CIDR for VPC in zone", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			scope.IBMVPCCluster.Spec.Zone = "foo-zone-temp"
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(&vpcv1.AddressPrefixCollection{}, &core.DetailedResponse{}, nil)
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating PublicGateWay", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			subnet := &vpcv1.Subnet{}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(addressPrefixCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(&vpcv1.CreateSubnetOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(&vpcv1.CreatePublicGatewayOptions{})).Return(&vpcv1.PublicGateway{}, &core.DetailedResponse{}, errors.New("Error when creating PublicGateWay"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when attaching PublicGateWay", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			subnet := &vpcv1.Subnet{
				Name: core.StringPtr(scope.IBMVPCCluster.Name + subnetSuffix),
				ID:   core.StringPtr(scope.IBMVPCCluster.Name + "-subnet-id"),
			}
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(addressPrefixCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(&vpcv1.CreateSubnetOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreatePublicGateway(gomock.AssignableToTypeOf(&vpcv1.CreatePublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().SetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.SetSubnetPublicGatewayOptions{})).Return(&vpcv1.PublicGateway{}, &core.DetailedResponse{}, errors.New("Error when setting SubnetPublicGateWay"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when creating Subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListVPCAddressPrefixes(gomock.AssignableToTypeOf(&vpcv1.ListVPCAddressPrefixesOptions{})).Return(addressPrefixCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateSubnet(gomock.AssignableToTypeOf(&vpcv1.CreateSubnetOptions{})).Return(nil, &core.DetailedResponse{}, errors.New("Error when creating Subnet"))
			_, err := scope.CreateSubnet()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteSubnet(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			VPC: "foo-vpc",
		},
		Status: infrav1beta2.IBMVPCClusterStatus{
			Subnet: infrav1beta2.Subnet{
				ID: core.StringPtr("foo-vpc-subnet-id"),
			},
		},
	}

	t.Run("Delete Subnet", func(t *testing.T) {
		publicGateway := &vpcv1.PublicGateway{
			ID: core.StringPtr("foo-public-gateway-id"),
		}
		subnet := &vpcv1.SubnetCollection{
			Subnets: []vpcv1.Subnet{
				{ID: core.StringPtr("foo-vpc-subnet-id")},
			},
		}

		t.Run("Should delete subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.GetSubnetPublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.UnsetSubnetPublicGatewayOptions{})).Return(&core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(&vpcv1.DeletePublicGatewayOptions{})).Return(&core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteSubnet(gomock.AssignableToTypeOf(&vpcv1.DeleteSubnetOptions{})).Return(&core.DetailedResponse{}, nil)
			err := scope.DeleteSubnet()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when unsetting publicgateway for subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.GetSubnetPublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.UnsetSubnetPublicGatewayOptions{})).Return(&core.DetailedResponse{}, errors.New("Error when unsetting publicgateway for subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when deleting publicgateway for subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.GetSubnetPublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.UnsetSubnetPublicGatewayOptions{})).Return(&core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(&vpcv1.DeletePublicGatewayOptions{})).Return(&core.DetailedResponse{}, errors.New("Error when deleting publicgateway for subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when deleting subnet", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(subnet, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.GetSubnetPublicGatewayOptions{})).Return(publicGateway, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(gomock.AssignableToTypeOf(&vpcv1.UnsetSubnetPublicGatewayOptions{})).Return(&core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeletePublicGateway(gomock.AssignableToTypeOf(&vpcv1.DeletePublicGatewayOptions{})).Return(&core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteSubnet(gomock.AssignableToTypeOf(&vpcv1.DeleteSubnetOptions{})).Return(&core.DetailedResponse{}, errors.New("Error when deleting subnet"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error listing subnets", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(nil, &core.DetailedResponse{}, errors.New("Error listing subnets"))
			err := scope.DeleteSubnet()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Subnet doesn't exist", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListSubnets(gomock.AssignableToTypeOf(&vpcv1.ListSubnetsOptions{})).Return(&vpcv1.SubnetCollection{Subnets: []vpcv1.Subnet{}}, &core.DetailedResponse{}, nil)
			err := scope.DeleteSubnet()
			g.Expect(err).To(BeNil())
		})
	})
}

func TestCreateLoadBalancer(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			ControlPlaneLoadBalancer: &infrav1beta2.VPCLoadBalancerSpec{
				Name: "foo-load-balancer",
			},
		},
		Status: infrav1beta2.IBMVPCClusterStatus{
			Subnet: infrav1beta2.Subnet{
				ID: core.StringPtr("foo-subnet-id"),
			},
		},
	}

	t.Run("Create LoadBalancer", func(t *testing.T) {
		t.Run("Error when listing LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, errors.New("Failed to list LoadBalancer"))
			_, err := scope.CreateLoadBalancer()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Return exsisting LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			vpcClusterCustom := infrav1beta2.IBMVPCCluster{
				Spec: infrav1beta2.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1beta2.VPCLoadBalancerSpec{
						Name: "foo-load-balancer-1",
					},
				},
				Status: infrav1beta2.IBMVPCClusterStatus{
					Subnet: infrav1beta2.Subnet{
						ID: core.StringPtr("foo-subnet-id"),
					},
				},
			}
			loadBalancerCollection := &vpcv1.LoadBalancerCollection{
				LoadBalancers: []vpcv1.LoadBalancer{
					{
						Name: core.StringPtr("foo-load-balancer-1"),
					},
				},
			}
			expectedOutput := &vpcv1.LoadBalancer{
				Name: core.StringPtr("foo-load-balancer-1"),
			}
			scope.IBMVPCCluster.Spec = vpcClusterCustom.Spec
			scope.IBMVPCCluster.Status = vpcClusterCustom.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			out, err := scope.CreateLoadBalancer()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})
		t.Run("Error when listing LoadBalancer (GetLoadBalancerByHostname)", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, errors.New("Failed to list LoadBalancer"))
			_, err := scope.GetLoadBalancerByHostname("foo-load-balancer-hostname")
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Return LoadBalancer by Hostname", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			loadBalancerCollection := &vpcv1.LoadBalancerCollection{
				LoadBalancers: []vpcv1.LoadBalancer{
					{
						Name:     core.StringPtr("foo-load-balancer"),
						Hostname: core.StringPtr("foo-load-balancer-hostname"),
					},
				},
			}
			expectedOutput := &vpcv1.LoadBalancer{
				Name:     core.StringPtr("foo-load-balancer"),
				Hostname: core.StringPtr("foo-load-balancer-hostname"),
			}
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			out, err := scope.GetLoadBalancerByHostname("foo-load-balancer-hostname")
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})
		t.Run("Error when subnet is nil", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			scope.IBMVPCCluster.Status.Subnet.ID = nil
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, nil)
			_, err := scope.CreateLoadBalancer()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when creating LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerOptions{})).Return(&vpcv1.LoadBalancer{}, &core.DetailedResponse{}, errors.New("Failed to create LoadBalancer"))
			_, err := scope.CreateLoadBalancer()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Should create LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			loadBalancer := &vpcv1.LoadBalancer{
				Name: core.StringPtr("foo-load-balancer"),
			}
			expectedOutput := &vpcv1.LoadBalancer{
				Name: core.StringPtr("foo-load-balancer"),
			}
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			out, err := scope.CreateLoadBalancer()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})
	})
}

func TestDeleteLoadBalancer(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcCluster := infrav1beta2.IBMVPCCluster{
		Spec: infrav1beta2.IBMVPCClusterSpec{
			ControlPlaneLoadBalancer: &infrav1beta2.VPCLoadBalancerSpec{
				Name: "foo-load-balancer",
			},
		},
		Status: infrav1beta2.IBMVPCClusterStatus{
			VPCEndpoint: infrav1beta2.VPCEndpoint{
				LBID: core.StringPtr("foo-load-balancer-id"),
			},
		},
	}

	t.Run("Delete LoadBalancer", func(t *testing.T) {
		loadBalancerCollection := &vpcv1.LoadBalancerCollection{
			LoadBalancers: []vpcv1.LoadBalancer{
				{
					ID:                 core.StringPtr("foo-load-balancer-id"),
					ProvisioningStatus: core.StringPtr("active"),
				},
			},
		}

		t.Run("Error when listing LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, errors.New("Failed to list LoadBalancer"))
			_, err := scope.DeleteLoadBalancer()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error while deleting LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.DeleteLoadBalancerOptions{})).Return(&core.DetailedResponse{}, errors.New("Could not delete LoadBalancer"))
			_, err := scope.DeleteLoadBalancer()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Should delete LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupClusterScope(clusterName, mockvpc)
			scope.IBMVPCCluster.Spec = vpcCluster.Spec
			scope.IBMVPCCluster.Status = vpcCluster.Status
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.DeleteLoadBalancerOptions{})).Return(&core.DetailedResponse{}, nil)
			_, err := scope.DeleteLoadBalancer()
			g.Expect(err).To(BeNil())
		})
	})
}
