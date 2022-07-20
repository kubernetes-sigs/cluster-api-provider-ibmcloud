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
	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMVPCClusterReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name         string
		vpcCluster   *infrav1beta1.IBMVPCCluster
		ownerCluster *capiv1beta1.Cluster
		expectError  bool
	}{
		{
			name: "Should fail Reconcile if owner cluster not found",
			vpcCluster: &infrav1beta1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vpc-test-",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test",
							UID:        "1",
						}}},
				Spec: infrav1beta1.IBMVPCClusterSpec{}},
			expectError: true,
		},
		{
			name: "Should not reconcile if owner reference is not set",
			vpcCluster: &infrav1beta1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vpc-test-"},
				Spec: infrav1beta1.IBMVPCClusterSpec{}},
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
				Log:    klogr.New(),
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
						APIVersion: capiv1beta1.GroupVersion.String(),
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
			Log:    klogr.New(),
		}
		clusterScope = &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Logger:       klogr.New(),
			IBMVPCCluster: &infrav1beta1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-cluster",
				},
				Spec: infrav1beta1.IBMVPCClusterSpec{
					VPC: "capi-vpc",
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
			_, err := reconciler.reconcile(clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		clusterScope.IBMVPCCluster.Finalizers = []string{infrav1beta1.ClusterFinalizer}
		listVpcsOptions := &vpcv1.ListVpcsOptions{}
		response := &core.DetailedResponse{}
		vpclist := &vpcv1.VPCCollection{}
		t.Run("Should fail to reconcile IBMVPCCluster", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1beta1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, errors.New("failed to list VPCs"))
			_, err := reconciler.reconcile(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		vpclist.Vpcs = []vpcv1.VPC{
			{
				Name: pointer.String("capi-vpc"),
				ID:   pointer.String("capi-vpc-id"),
			},
		}
		listFloatingIpsOptions := &vpcv1.ListFloatingIpsOptions{}
		fips := &vpcv1.FloatingIPCollection{}
		t.Run("Should fail to reserve FIP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1beta1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListFloatingIps(listFloatingIpsOptions).Return(fips, response, errors.New("failed to list the FIPs"))
			_, err := reconciler.reconcile(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		fips.FloatingIps = []vpcv1.FloatingIP{
			{
				Name:    pointer.String("vpc-cluster-control-plane"),
				Address: pointer.String("192.98.98.45"),
				ID:      pointer.String("capi-fip-id"),
			},
		}
		options := &vpcv1.ListSubnetsOptions{}
		subnets := &vpcv1.SubnetCollection{}
		t.Run("Should fail to create subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1beta1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListFloatingIps(listFloatingIpsOptions).Return(fips, response, nil)
			mockvpc.EXPECT().ListSubnets(options).Return(subnets, response, errors.New("Failed to list the subnets"))
			_, err := reconciler.reconcile(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		subnets.Subnets = []vpcv1.Subnet{
			{
				ID:   pointer.String("capi-subnet-id"),
				Name: pointer.String("vpc-cluster-subnet"),
				Zone: &vpcv1.ZoneReference{
					Name: pointer.String("foo"),
				},
			},
		}
		t.Run("Should successfully reconcile IBMVPCCluster and set cluster status as Ready", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			clusterScope.IBMVPCCluster.Finalizers = []string{infrav1beta1.ClusterFinalizer}
			mockvpc.EXPECT().ListVpcs(listVpcsOptions).Return(vpclist, response, nil)
			mockvpc.EXPECT().ListFloatingIps(listFloatingIpsOptions).Return(fips, response, nil)
			mockvpc.EXPECT().ListSubnets(options).Return(subnets, response, nil)
			_, err := reconciler.reconcile(clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
			g.Expect(clusterScope.IBMVPCCluster.Status.Ready).To(Equal(true))
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
			Log:    klogr.New(),
		}
		clusterScope = &scope.ClusterScope{
			IBMVPCClient: mockvpc,
			Logger:       klogr.New(),
			IBMVPCCluster: &infrav1beta1.IBMVPCCluster{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1beta1.ClusterFinalizer},
				},
				Status: infrav1beta1.IBMVPCClusterStatus{
					VPCEndpoint: infrav1beta1.VPCEndpoint{
						FIPID: pointer.String("capi-fip-id"),
					},
					Subnet: infrav1beta1.Subnet{
						ID: pointer.String("capi-subnet-id"),
					},
					VPC: infrav1beta1.VPC{
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
		VPCID: pointer.String("capi-vpc-id"),
	}
	response := &core.DetailedResponse{}
	instancelist := &vpcv1.InstanceCollection{}
	t.Run("Reconciling deleting IBMVPCCluster", func(t *testing.T) {
		t.Run("Should fail to list instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, errors.New("Failed to list the VSIs"))
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		t.Run("Should skip deleting other resources if instances are still running", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			instancelist.TotalCount = pointer.Int64(2)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		getPGWOptions := &vpcv1.GetSubnetPublicGatewayOptions{ID: pointer.String("capi-subnet-id")}
		pgw := &vpcv1.PublicGateway{ID: pointer.String("capi-pgw-id")}
		unsetPGWOptions := &vpcv1.UnsetSubnetPublicGatewayOptions{ID: pointer.String("capi-subnet-id")}
		deleteSubnetOptions := &vpcv1.DeleteSubnetOptions{ID: pointer.String("capi-subnet-id")}
		deletePGWOptions := &vpcv1.DeletePublicGatewayOptions{ID: pgw.ID}
		instancelist.TotalCount = pointer.Int64(0)
		t.Run("Should fail deleting the subnet", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, errors.New("failed to delete subnet"))
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		deleteFIPOptions := &vpcv1.DeleteFloatingIPOptions{ID: pointer.String("capi-fip-id")}
		t.Run("Should fail deleting the floating IP", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteFloatingIP(deleteFIPOptions).Return(response, errors.New("failed to  delete floating IP"))
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		deleteVpcOptions := &vpcv1.DeleteVPCOptions{ID: pointer.String("capi-vpc-id")}
		t.Run("Should fail deleting the VPC", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteFloatingIP(deleteFIPOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteVPC(deleteVpcOptions).Return(response, errors.New("failed to delete VPC"))
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(ContainElement(infrav1beta1.ClusterFinalizer))
		})
		t.Run("Should successfully delete IBMVPCCluster and remove the finalizer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().ListInstances(listVSIOpts).Return(instancelist, response, nil)
			mockvpc.EXPECT().GetSubnetPublicGateway(getPGWOptions).Return(pgw, response, nil)
			mockvpc.EXPECT().UnsetSubnetPublicGateway(unsetPGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeletePublicGateway(deletePGWOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteSubnet(deleteSubnetOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteFloatingIP(deleteFIPOptions).Return(response, nil)
			mockvpc.EXPECT().DeleteVPC(deleteVpcOptions).Return(response, nil)
			_, err := reconciler.reconcileDelete(clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(clusterScope.IBMVPCCluster.Finalizers).To(Not(ContainElement(infrav1beta1.ClusterFinalizer)))
		})
	})
}

func createVPCCluster(g *WithT, vpcCluster *infrav1beta1.IBMVPCCluster, namespace string) {
	if vpcCluster != nil {
		vpcCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, vpcCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1beta1.IBMVPCCluster{}
			key := client.ObjectKey{
				Name:      vpcCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupVPCCluster(g *WithT, vpcCluster *infrav1beta1.IBMVPCCluster, namespace *corev1.Namespace) {
	if vpcCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vpcCluster, namespace)
	}
}
