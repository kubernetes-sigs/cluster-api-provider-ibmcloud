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
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSClusterReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name           string
		powervsCluster *infrav1beta2.IBMPowerVSCluster
		ownerCluster   *capiv1beta1.Cluster
		expectError    bool
	}{
		{
			name: "Should fail Reconcile if owner cluster not found",
			powervsCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "powervs-test-",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test",
							UID:        "1",
						}}},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"}},
			expectError: true,
		},
		{
			name: "Should not reconcile if owner reference is not set",
			powervsCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "powervs-test-"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstanceID: "foo"}},
			expectError: false,
		},
		{
			name:        "Should Reconcile successfully if no IBMPowerVSCluster found",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}

			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())

			if tc.ownerCluster != nil {
				tc.ownerCluster.Namespace = ns.Name
				g.Expect(testEnv.Create(ctx, tc.ownerCluster)).To(Succeed())
				defer func(do ...client.Object) {
					g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
				}(tc.ownerCluster)
				tc.powervsCluster.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       tc.ownerCluster.Name,
						UID:        "1",
					},
				}
			}
			createCluster(g, tc.powervsCluster, ns.Name)
			defer cleanupCluster(g, tc.powervsCluster, ns)

			if tc.powervsCluster != nil {
				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: tc.powervsCluster.Namespace,
						Name:      tc.powervsCluster.Name,
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
			_ = reconciler.reconcile(tc.powervsClusterScope)
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
				Logger: klogr.New(),
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
				Logger: klogr.New(),
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
					Labels: map[string]string{capiv1beta1.ClusterLabelName: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      pointer.String("capi-image.ova.gz"),
					Region:      pointer.String("us-south"),
					Bucket:      pointer.String("capi-bucket"),
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
					Labels: map[string]string{capiv1beta1.ClusterLabelName: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      pointer.String("capi-image2.ova.gz"),
					Region:      pointer.String("us-south"),
					Bucket:      pointer.String("capi-bucket"),
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
