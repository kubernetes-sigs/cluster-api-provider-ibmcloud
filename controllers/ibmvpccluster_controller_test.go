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

package controllers

import (
	"errors"
	"fmt"

	"testing"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMVPCClusterReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name         string
		vpcCluster   *infrav1.IBMVPCCluster
		ownerCluster *clusterv1beta1.Cluster
		expectError  bool
	}{
		{
			name: "Should fail Reconcile if owner cluster not found",
			vpcCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vpc-test-",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test",
							UID:        "1",
						}}},
				Spec: infrav1.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: *core.StringPtr("vpc-load-balancer"),
					},
				}},
			expectError: true,
		},
		{
			name: "Should not reconcile if owner reference is not set",
			vpcCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vpc-test-"},
				Spec: infrav1.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: *core.StringPtr("vpc-load-balancer"),
					},
				}},
			expectError: false,
		},
		{
			name:        "Should Reconcile successfully if no IBMVPCCluster found",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMVPCClusterReconciler{
				Client: testEnv.Client,
				Log:    klog.Background(),
			}

			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())

			if tc.ownerCluster != nil {
				tc.ownerCluster.Namespace = ns.Name
				g.Expect(testEnv.Create(ctx, tc.ownerCluster)).To(Succeed())
				defer func(do ...client.Object) {
					g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
				}(tc.ownerCluster)
				tc.vpcCluster.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       tc.ownerCluster.Name,
						UID:        "1",
					},
				}
			}
			createVPCCluster(g, tc.vpcCluster, ns.Name)
			defer cleanupVPCCluster(g, tc.vpcCluster, ns)

			if tc.vpcCluster != nil {
				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: tc.vpcCluster.Namespace,
						Name:      tc.vpcCluster.Name,
					},
				})
				if tc.expectError {
					g.Expect(err).ToNot(BeNil())
				} else {
					g.Expect(err).To(BeNil())
				}
			} else {
				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: ns.Name,
						Name:      "test",
					},
				})
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestIBMVPCClusterReconciler_reconcile(t *testing.T) {
	var (
		mockvpc      *mock.MockVpc
		mockCtrl     *gomock.Controller
		clusterScope *scope.ClusterScope
		reconciler   IBMVPCClusterReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCClusterReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		clusterScope = &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Cluster:      &clusterv1.Cluster{},
			Logger:       klog.Background(),
			IBMVPCCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-cluster",
				},
				Spec: infrav1.IBMVPCClusterSpec{
					VPC: "capi-vpc",
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: *core.StringPtr("vpc-load-balancer"),
					},
				},
			},
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Reconciling creating IBMVPCCluster", func(t *testing.T) {
		t.Run("Should add finalizer if not present", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
		listVpcsOptions := &vpcv1.ListVpcsOptions{}
		response := &core.DetailedResponse{}
		vpclist := &vpcv1.VPCCollection{}
		t.Run("Should fail to reconcile IBMVPCCluster", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, errors.New("failed to list VPCs"))
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		vpclist.Vpcs = []vpcv1.VPC{
			{
				Name: ptr.To("capi-vpc"),
				ID:   ptr.To("capi-vpc-id"),
			},
		}
		subnetOptions := &vpcv1.ListSubnetsOptions{}
		subnets := &vpcv1.SubnetCollection{}
		loadBalancerOptions := &vpcv1.ListLoadBalancersOptions{}
		loadBalancers := &vpcv1.LoadBalancerCollection{
			LoadBalancers: []vpcv1.LoadBalancer{
				{
					Name:               core.StringPtr("vpc-load-balancer"),
					Hostname:           core.StringPtr("vpc-load-balancer-hostname"),
					ID:                 core.StringPtr("vpc-load-balancer-id"),
					ProvisioningStatus: core.StringPtr("active"),
				},
			},
		}
		t.Run("Should fail to create subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListSubnets(subnetOptions).Return(subnets, response, errors.New("Failed to list the subnets"))
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		subnets.Subnets = []vpcv1.Subnet{
			{
				ID:   ptr.To("capi-subnet-id"),
				Name: ptr.To("vpc-cluster-subnet"),
				Zone: &vpcv1.ZoneReference{
					Name: ptr.To("foo"),
				},
			},
		}
		t.Run("Should successfully reconcile IBMVPCCluster and set cluster status as Ready", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListSubnets(subnetOptions).Return(subnets, response, nil)
			mockvpc.EXPECT().ListLoadBalancers(loadBalancerOptions).Return(loadBalancers, response, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
		})
		t.Run("Should use the user supplied port for the apiserver", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			port := int32(412)
			clusterScope.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{APIServerPort: port}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListSubnets(subnetOptions).Return(subnets, response, nil)
			mockvpc.EXPECT().ListLoadBalancers(loadBalancerOptions).Return(loadBalancers, response, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
			g.Expect(clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(port))
		})
		t.Run("Should use the default port for the apiserver if not specified", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListSubnets(subnetOptions).Return(subnets, response, nil)
			mockvpc.EXPECT().ListLoadBalancers(loadBalancerOptions).Return(loadBalancers, response, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
			g.Expect(clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(infrav1.DefaultAPIServerPort))
		})
	})
}

func TestIBMVPCClusterLBReconciler_reconcile(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc, *scope.ClusterScope, IBMVPCClusterReconciler) {
		t.Helper()
		mockvpc := mock.NewMockVpc(gomock.NewController(t))
		reconciler := IBMVPCClusterReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		clusterScope := &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Cluster:      &clusterv1.Cluster{},
			Logger:       klog.Background(),
			IBMVPCCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-cluster",
				},
				Spec: infrav1.IBMVPCClusterSpec{
					VPC: "capi-vpc",
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: "vpc-load-balancer",
					},
				},
			},
		}
		return gomock.NewController(t), mockvpc, clusterScope, reconciler
	}

	t.Run("Reconciling creating IBMVPCCluster with LoadBalancer", func(t *testing.T) {
		vpclist := &vpcv1.VPCCollection{
			Vpcs: []vpcv1.VPC{
				{
					Name: ptr.To("capi-vpc"),
					ID:   ptr.To("capi-vpc-id"),
				},
			},
		}
		subnets := &vpcv1.SubnetCollection{
			Subnets: []vpcv1.Subnet{
				{
					ID:   ptr.To("capi-subnet-id"),
					Name: ptr.To("vpc-cluster-subnet"),
					Zone: &vpcv1.ZoneReference{
						Name: ptr.To("foo"),
					},
				},
			},
		}
		loadBalancerCollection := &vpcv1.LoadBalancerCollection{
			LoadBalancers: []vpcv1.LoadBalancer{
				{
					Name:               core.StringPtr("vpc-load-balancer"),
					ID:                 core.StringPtr("vpc-load-balancer-id"),
					ProvisioningStatus: core.StringPtr("active"),
					Hostname:           core.StringPtr("vpc-load-balancer-hostname"),
					OperatingStatus:    core.StringPtr("online"),
				},
			},
		}

		t.Run("Should fail to create LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, errors.New("Failed to list the LoadBalancers"))
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		t.Run("Should successfully reconcile IBMVPCCluster with default port for the apiserver and set cluster status as Ready when LoadBalancer is in active state", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
			g.Expect(clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(infrav1.DefaultAPIServerPort))
		})
		t.Run("Should successfully reconcile IBMVPCCluster with user supplied port for the apiserver and set cluster status as Ready when LoadBalancer is in active state", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			port := int32(412)
			clusterScope.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{APIServerPort: port}
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
			g.Expect(clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(port))
		})
		t.Run("Should successfully reconcile IBMVPCCluster and set cluster status as Ready when ControlPlaneEndpoint Host is set", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint = clusterv1beta1.APIEndpoint{
				Host: *core.StringPtr("vpc-load-balancer-hostname"),
			}
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
			g.Expect(clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(infrav1.DefaultAPIServerPort))
		})
		t.Run("Should successfully reconcile IBMVPCCluster and set cluster status as NotReady when LoadBalancer is create state", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			loadBalancerCollection.LoadBalancers[0].ProvisioningStatus = core.StringPtr("create_pending")
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(false))
		})
		t.Run("Should successfully reconcile IBMVPCCluster and set cluster status as NotReady when LoadBalancer is in undefined state", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1.ClusterFinalizer}
			loadBalancerCollection.LoadBalancers[0].ProvisioningStatus = core.StringPtr("update_pending")
			mockvpc.EXPECT().ListVpcs(&vpcv1.ListVpcsOptions{}).Return(vpclist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnets, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(&vpcv1.ListLoadBalancersOptions{}).Return(loadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcile(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(false))
		})
	})
}

func TestIBMVPCClusterReconciler_delete(t *testing.T) {
	var (
		mockvpc      *mock.MockVpc
		mockCtrl     *gomock.Controller
		clusterScope *scope.ClusterScope
		reconciler   IBMVPCClusterReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCClusterReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		clusterScope = &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Logger:       klog.Background(),
			IBMVPCCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1.ClusterFinalizer},
				},
				Status: infrav1.IBMVPCClusterStatus{
					Subnet: infrav1.Subnet{
						ID: ptr.To("capi-subnet-id"),
					},
					VPC: infrav1.VPC{
						ID: "capi-vpc-id",
					},
				},
			},
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	listVSIOpts := &vpcv1.ListInstancesOptions{
		VPCID: ptr.To("capi-vpc-id"),
	}
	response := &core.DetailedResponse{}
	instancelist := &vpcv1.InstanceCollection{}
	t.Run("Reconciling deleting IBMVPCCluster", func(t *testing.T) {
		t.Run("Should fail to list instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, errors.New("Failed to list the VSIs"))
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		t.Run("Should skip deleting other resources if instances are still running", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			instancelist.TotalCount = ptr.To(int64(2))
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		getPGWOptions := &vpcv1.GetSubnetPublicGatewayOptions{ID: ptr.To("capi-subnet-id")}
		subnet := &vpcv1.SubnetCollection{Subnets: []vpcv1.Subnet{{ID: core.StringPtr("capi-subnet-id")}}}
		pgw := &vpcv1.PublicGateway{ID: ptr.To("capi-pgw-id")}
		unsetPGWOptions := &vpcv1.UnsetSubnetPublicGatewayOptions{ID: ptr.To("capi-subnet-id")}
		deleteSubnetOptions := &vpcv1.DeleteSubnetOptions{ID: ptr.To("capi-subnet-id")}
		deletePGWOptions := &vpcv1.DeletePublicGatewayOptions{ID: pgw.ID}
		instancelist.TotalCount = ptr.To(int64(0))
		t.Run("Should fail deleting the subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnet, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, errors.New("failed to delete subnet"))
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		deleteVpcOptions := &vpcv1.DeleteVPCOptions{ID: ptr.To("capi-vpc-id")}
		t.Run("Should fail deleting the VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnet, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteVPC(deleteVpcOptions).Return(response, errors.New("failed to delete VPC"))
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		t.Run("Should successfully delete IBMVPCCluster and remove the finalizer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().ListSubnets(&vpcv1.ListSubnetsOptions{}).Return(subnet, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteVPC(deleteVpcOptions).Return(response, nil)
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(Not(ContainElement(infrav1.ClusterFinalizer)))
		})
	})
}

func TestIBMVPCClusterLBReconciler_delete(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc, *scope.ClusterScope, IBMVPCClusterReconciler) {
		t.Helper()
		mockvpc := mock.NewMockVpc(gomock.NewController(t))
		reconciler := IBMVPCClusterReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		clusterScope := &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Logger:       klog.Background(),
			IBMVPCCluster: &infrav1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1.ClusterFinalizer},
				},
				Spec: infrav1.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: "vpc-load-balancer",
					},
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "vpc-load-balancer-hostname",
					},
				},
				Status: infrav1.IBMVPCClusterStatus{
					VPCEndpoint: infrav1.VPCEndpoint{
						LBID: ptr.To("vpc-load-balancer-id"),
					},
					Subnet: infrav1.Subnet{
						ID: ptr.To("capi-subnet-id"),
					},
					VPC: infrav1.VPC{
						ID: "capi-vpc-id",
					},
				},
			},
		}
		return gomock.NewController(t), mockvpc, clusterScope, reconciler
	}

	t.Run("Reconciling deleting IBMVPCCluster with LoadBalancer", func(t *testing.T) {
		instancelist := &vpcv1.InstanceCollection{
			TotalCount: core.Int64Ptr(0),
		}

		t.Run("Should fail deleting the LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(&vpcv1.LoadBalancerCollection{}, &core.DetailedResponse{}, errors.New("failed to list LoadBalancers"))
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Should skip deleting other resources if LoadBalancer is still present", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			customloadBalancerCollection := &vpcv1.LoadBalancerCollection{
				LoadBalancers: []vpcv1.LoadBalancer{
					{
						Name:               core.StringPtr("vpc-load-balancer"),
						ID:                 core.StringPtr("vpc-load-balancer-id"),
						ProvisioningStatus: core.StringPtr("delete_pending"),
						Hostname:           core.StringPtr("vpc-load-balancer-hostname"),
					},
				},
			}
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(customloadBalancerCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancersOptions{})).Return(customloadBalancerCollection, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1.ClusterFinalizer))
		})
		t.Run("Should successfully delete IBMVPCCluster and remove the finalizer when ControlPlaneEndpoint Host is set", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, clusterScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			clusterScope.IBMVPCCluster.Spec.ControlPlaneLoadBalancer = nil
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			_, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(Not(ContainElement(infrav1.ClusterFinalizer)))
		})
	})
}

func createVPCCluster(g *WithT, vpcCluster *infrav1.IBMVPCCluster, namespace string) {
	if vpcCluster != nil {
		vpcCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, vpcCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1.IBMVPCCluster{}
			key := client.ObjectKey{
				Name:      vpcCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupVPCCluster(g *WithT, vpcCluster *infrav1.IBMVPCCluster, namespace *corev1.Namespace) {
	if vpcCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vpcCluster, namespace)
	}
}
