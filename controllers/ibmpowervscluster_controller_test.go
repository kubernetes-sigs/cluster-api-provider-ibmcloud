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

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
)

func TestIBMPowerVSClusterReconciler(t *testing.T) {
	testCases := []struct {
		name           string
		powervsCluster *infrav1beta1.IBMPowerVSCluster
		ownerCluster   *capiv1beta1.Cluster
		expectError    bool
	}{
		{
			name: "Should fail Reconcile if owner cluster not found",
			powervsCluster: &infrav1beta1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{GenerateName: "powervs-test-", OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: capiv1beta1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "capi-test",
					UID:        "1",
				}}},
				Spec: infrav1beta1.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"}},
			expectError: true,
		},
		{
			name:           "Should not reconcile if owner reference is not set",
			powervsCluster: &infrav1beta1.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{GenerateName: "powervs-test-"}, Spec: infrav1beta1.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"}},
			expectError:    false,
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

func createCluster(g *WithT, powervsCluster *infrav1beta1.IBMPowerVSCluster, namespace string) {
	if powervsCluster != nil {
		powervsCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, powervsCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1beta1.IBMPowerVSCluster{}
			key := client.ObjectKey{
				Name:      powervsCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupCluster(g *WithT, powervsCluster *infrav1beta1.IBMPowerVSCluster, namespace *corev1.Namespace) {
	if powervsCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(powervsCluster, namespace)
	}
}
