/*


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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha4"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IBMPowerVSClusterReconciler", func() {

	Context("Reconcile an IBMMPowerVSCluster", func() {
		It("should not error and not requeue the request", func() {
			ctx := context.Background()

			reconciler := &IBMPowerVSClusterReconciler{
				Client: k8sClient,
				Log:    klogr.New(),
			}

			instance := &v1alpha4.IBMPowerVSCluster{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

			// Create the IBMPowerVSCluster object and expect the Reconcile to be created
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			defer func() {
				err := k8sClient.Delete(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}()

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(result.Requeue).To(BeFalse())
		})
	})
})
