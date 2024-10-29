/*
Copyright 2021 The Kubernetes Authors.

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
	"sync"
	"testing"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/IBM/go-sdk-core/v5/core"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSClusterReconciler_Reconcile(t *testing.T) {
	t.Run("Should fail Reconcile if owner cluster not found", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})

		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Should not reconcile if owner reference is not set", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-"},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})

		g.Expect(err).To(BeNil())
	})

	t.Run("Should Reconcile successfully if no IBMPowerVSCluster found", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-cluster",
			},
		})

		g.Expect(err).To(BeNil())
	})

	t.Run("Error creating cluster scope", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Successfully reconcile", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
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
		}

		ownerCluster := &capiv1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-test",
				Namespace: ns.Name,
			},
		}

		g.Expect(testEnv.Create(ctx, ownerCluster)).To(Succeed())
		defer func(obj ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, obj...)).To(Succeed())
		}(ownerCluster)

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
			ClientFactory: scope.ClientFactory{
				PowerVSClientFactory: func() (powervs.PowerVS, error) {
					return nil, nil
				},
				AuthenticatorFactory: func() (core.Authenticator, error) {
					return nil, nil
				},
			},
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).To(BeNil())

		ibmPowerVSCluster := &infrav1beta2.IBMPowerVSCluster{}
		g.Eventually(func(gomega Gomega) {
			gomega.Expect(testEnv.Client.Get(ctx, client.ObjectKey{
				Name:      powerVSCluster.GetName(),
				Namespace: powerVSCluster.GetNamespace(),
			}, ibmPowerVSCluster)).To(Succeed())
			gomega.Expect(len(ibmPowerVSCluster.Finalizers)).To(Equal(1))
		}).Should(Succeed())
	})

	t.Run("Successfully call reconcile delete", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{"ibmpowervscluster.infrastructure.cluster.x-k8s.io"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{Zone: ptr.To("zone")},
		}

		ownerCluster := &capiv1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-test",
				Namespace: ns.Name,
			},
		}

		g.Expect(testEnv.Create(ctx, ownerCluster)).To(Succeed())
		defer func(obj ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, obj...)).To(Succeed())
		}(ownerCluster)

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		g.Expect(testEnv.Delete(ctx, powerVSCluster)).To(Succeed())
		ibmPowerVSCluster := &infrav1beta2.IBMPowerVSCluster{}

		g.Eventually(func() bool {
			err := testEnv.Client.Get(ctx, client.ObjectKey{
				Name:      powerVSCluster.GetName(),
				Namespace: powerVSCluster.GetNamespace(),
			}, ibmPowerVSCluster)
			g.Expect(err).To(BeNil())
			return ibmPowerVSCluster.DeletionTimestamp.IsZero() == false
		}, 10*time.Second).Should(BeTrue(), "Eventually failed while checking delete timestamp")

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
			ClientFactory: scope.ClientFactory{
				PowerVSClientFactory: func() (powervs.PowerVS, error) {
					return nil, nil
				},
				AuthenticatorFactory: func() (core.Authenticator, error) {
					return nil, nil
				},
			},
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).To(BeNil())
	})
}

func TestIBMPowerVSClusterReconciler_reconcile(t *testing.T) {
	testCases := []struct {
		name                string
		powervsClusterScope *scope.PowerVSClusterScope
		clusterStatus       bool
	}{
		{
			name: "Should add finalizer and reconcile IBMPowerVSCluster",
			powervsClusterScope: &scope.PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			},
			clusterStatus: false,
		},
		{
			name: "Should reconcile IBMPowerVSCluster status as Ready",
			powervsClusterScope: &scope.PowerVSClusterScope{
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{infrav1beta2.IBMPowerVSClusterFinalizer},
					},
				},
			},
			clusterStatus: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}
			_, _ = reconciler.reconcile(tc.powervsClusterScope)
			g.Expect(tc.powervsClusterScope.IBMPowerVSCluster.Status.Ready).To(Equal(tc.clusterStatus))
			g.Expect(tc.powervsClusterScope.IBMPowerVSCluster.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSClusterFinalizer))
		})
	}
}

func TestIBMPowerVSClusterReconciler_delete(t *testing.T) {
	var (
		reconciler   IBMPowerVSClusterReconciler
		clusterScope *scope.PowerVSClusterScope
	)
	reconciler = IBMPowerVSClusterReconciler{
		Client: testEnv.Client,
	}
	t.Run("Reconciling delete IBMPowerVSCluster", func(t *testing.T) {
		t.Run("Should reconcile successfully if no descendants are found", func(t *testing.T) {
			g := NewWithT(t)
			clusterScope = &scope.PowerVSClusterScope{
				Logger: klog.Background(),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
			}
			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(BeZero())
		})
		t.Run("Should reconcile with requeue by deleting the cluster descendants", func(t *testing.T) {
			g := NewWithT(t)
			clusterScope = &scope.PowerVSClusterScope{
				Logger: klog.Background(),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
			}
			powervsImage1 := &infrav1beta2.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1beta2.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{capiv1beta1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      ptr.To("capi-image.ova.gz"),
					Region:      ptr.To("us-south"),
					Bucket:      ptr.To("capi-bucket"),
				},
			}
			powervsImage2 := &infrav1beta2.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1beta2.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{capiv1beta1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      ptr.To("capi-image2.ova.gz"),
					Region:      ptr.To("us-south"),
					Bucket:      ptr.To("capi-bucket"),
				},
			}
			createObject(g, powervsImage1, "default")
			defer cleanupObject(g, powervsImage1)
			createObject(g, powervsImage2, "default")
			defer cleanupObject(g, powervsImage2)

			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			// Updating the object should fail as it doesn't exist
			g.Expect(clusterScope.Client.Update(ctx, powervsImage1)).To(Not(Succeed()))
			g.Expect(clusterScope.Client.Update(ctx, powervsImage2)).To(Not(Succeed()))
		})
	})
}

func TestReconcileVPCResources(t *testing.T) {
	testCases := []struct {
		name                    string
		powerVSClusterScopeFunc func() *scope.PowerVSClusterScope
		reconcileResult         reconcileResult
		conditions              capiv1beta1.Conditions
	}{
		{
			name: "when ReconcileVPC returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("vpc not found"),
			},
			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCReconciliationFailedReason,
					Message:            "vpc not found",
				},
			},
		},
		{
			name: "when ReconcileVPC returns requeue",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("pending")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					Requeue: true,
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("vpc subnet not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("vpc subnet not found"),
			},

			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCSubnetReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCSubnetReconciliationFailedReason,
					Message:            "vpc subnet not found",
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns requeue",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("rg-id"),
							},
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
				mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), gomock.Any()).Return("cidr", nil)
				mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(&vpcv1.Subnet{Status: ptr.To("active")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					Requeue: true,
				},
			},
			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
			},
		},
		{
			name: "when Reconciling VPC security group returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							VPCSecurityGroups: []infrav1beta2.VPCSecurityGroup{
								{
									Name: ptr.To("security-group"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("vpc security group not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: fmt.Errorf("failed to validate existing security group: vpc security group not found"),
			},

			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCSecurityGroupReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCSecurityGroupReconciliationFailedReason,
					Message:            "failed to validate existing security group: vpc security group not found",
				},
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
								{
									ID: ptr.To("lb-id"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("loadbalancer not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: fmt.Errorf("loadbalancer not found"),
			},

			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.LoadBalancerReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.LoadBalancerReconciliationFailedReason,
					Message:            "loadbalancer not found",
				},
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer and loadbalancer is ready",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
								{
									ID: ptr.To("lb-id"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := mock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To("active"),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			conditions: capiv1beta1.Conditions{
				getVPCLBReadyCondition(),
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}
			clusterScope := tc.powerVSClusterScopeFunc()
			ch := make(chan reconcileResult, 1)
			pvsCluster := &powerVSCluster{
				cluster: clusterScope.IBMPowerVSCluster,
			}
			wg := &sync.WaitGroup{}
			wg.Add(1)
			reconciler.reconcileVPCResources(clusterScope, pvsCluster, ch, wg)
			wg.Wait()
			close(ch)
			g.Expect(<-ch).To(Equal(tc.reconcileResult))
			if len(tc.conditions) > 0 {
				ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
					return metav1.Time{}
				})
				g.Expect(pvsCluster.cluster.GetConditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
			}
		})
	}
}

func getVPCReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCReadyCondition,
		Status: "True",
	}
}

func getVPCSubnetReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCSubnetReadyCondition,
		Status: "True",
	}
}

func getVPCSGReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCSecurityGroupReadyCondition,
		Status: "True",
	}
}

func getVPCLBReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.LoadBalancerReadyCondition,
		Status: "True",
	}
}

func createCluster(g *WithT, powervsCluster *infrav1beta2.IBMPowerVSCluster, namespace string) {
	if powervsCluster != nil {
		powervsCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, powervsCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1beta2.IBMPowerVSCluster{}
			key := client.ObjectKey{
				Name:      powervsCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupCluster(g *WithT, powervsCluster *infrav1beta2.IBMPowerVSCluster, namespace *corev1.Namespace) {
	if powervsCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(powervsCluster, namespace)
	}
}
