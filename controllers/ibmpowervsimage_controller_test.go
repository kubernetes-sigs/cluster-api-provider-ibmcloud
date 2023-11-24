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

	"github.com/IBM-Cloud/power-go-client/power/models"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSImageReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name           string
		powervsCluster *infrav1beta2.IBMPowerVSCluster
		powervsImage   *infrav1beta2.IBMPowerVSImage
		expectError    bool
	}{
		{
			name:        "Should Reconcile successfully if IBMPowerVSImage is not found",
			expectError: false,
		},
		{
			name: "Should not Reconcile if failed to find IBMPowerVSCluster",
			powervsImage: &infrav1beta2.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image",
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      pointer.String("capi-image.ova.gz"),
					Region:      pointer.String("us-south"),
					Bucket:      pointer.String("capi-bucket"),
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSImageReconciler{
				Client: testEnv.Client,
			}

			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())

			createObject(g, tc.powervsImage, ns.Name)
			defer cleanupObject(g, tc.powervsImage)

			if tc.powervsImage != nil {
				g.Eventually(func() bool {
					machine := &infrav1beta2.IBMPowerVSImage{}
					key := client.ObjectKey{
						Name:      tc.powervsImage.Name,
						Namespace: ns.Name,
					}
					err = testEnv.Get(ctx, key, machine)
					return err == nil
				}, 10*time.Second).Should(Equal(true))

				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: tc.powervsImage.Namespace,
						Name:      tc.powervsImage.Name,
					},
				})
				if tc.expectError {
					g.Expect(err).ToNot(BeNil())
				} else {
					g.Expect(err).To(BeNil())
				}
			} else {
				_, err = reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: "default",
						Name:      "test",
					},
				})
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestIBMPowerVSImageReconciler_reconcile(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
		reconciler  IBMPowerVSImageReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
		recorder := record.NewFakeRecorder(2)
		reconciler = IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: recorder,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Reconciling IBMPowerVSImage ", func(t *testing.T) {
		t.Run("Should reconcile by setting the owner reference", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			powervsCluster := &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-powervs-cluster"},
			}
			imageScope := &scope.PowerVSImageScope{
				Logger: klogr.New(),
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-image",
					},
					Spec: infrav1beta2.IBMPowerVSImageSpec{
						ClusterName: "capi-powervs-cluster",
						Object:      pointer.String("capi-image.ova.gz"),
						Region:      pointer.String("us-south"),
						Bucket:      pointer.String("capi-bucket"),
					},
				},
			}
			_, err := reconciler.reconcile(powervsCluster, imageScope)
			g.Expect(err).To(BeNil())
		})
		t.Run("Reconciling an image import job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			const jobID = "job-1"
			powervsCluster := &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-powervs-cluster",
					UID:  "1",
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstanceID: "service-instance-1",
				},
			}
			powervsImage := &infrav1beta2.IBMPowerVSImage{
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
					Finalizers: []string{infrav1beta2.IBMPowerVSImageFinalizer},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      pointer.String("capi-image.ova.gz"),
					Region:      pointer.String("us-south"),
					Bucket:      pointer.String("capi-bucket"),
				},
			}

			mockclient := fake.NewClientBuilder().WithObjects([]client.Object{powervsCluster, powervsImage}...).Build()
			imageScope := &scope.PowerVSImageScope{
				Logger:           klogr.New(),
				Client:           mockclient,
				IBMPowerVSImage:  powervsImage,
				IBMPowerVSClient: mockpowervs,
			}

			imageScope.IBMPowerVSImage.Status.JobID = jobID
			t.Run("When failed to get the import job using jobID", func(t *testing.T) {
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf(jobID)).Return(nil, errors.New("Error finding the job"))
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(Not(BeNil()))
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
			})
			job := &models.Job{
				ID: pointer.String(jobID),
				Status: &models.Status{
					State: pointer.String("queued"),
				},
			}
			t.Run("When import job status is queued", func(t *testing.T) {
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf(jobID)).Return(job, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(BeNil())
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(false))
				g.Expect(imageScope.IBMPowerVSImage.Status.ImageState).To(BeEquivalentTo(infrav1beta2.PowerVSImageStateQue))
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{infrav1beta2.ImageImportedCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityInfo, string(infrav1beta2.PowerVSImageStateQue)}})
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			})
			t.Run("When importing image is still in progress", func(t *testing.T) {
				job.Status.State = pointer.String("")
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(BeNil())
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(false))
				g.Expect(imageScope.IBMPowerVSImage.Status.ImageState).To(BeEquivalentTo(infrav1beta2.PowerVSImageStateImporting))
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{infrav1beta2.ImageImportedCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityInfo, *job.Status.State}})
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			})
			t.Run("When import job status is failed", func(t *testing.T) {
				job.Status.State = pointer.String("failed")
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(Not(BeNil()))
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(false))
				g.Expect(imageScope.IBMPowerVSImage.Status.ImageState).To(BeEquivalentTo(infrav1beta2.PowerVSImageStateFailed))
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{infrav1beta2.ImageImportedCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityError, infrav1beta2.ImageImportFailedReason}})
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			})
			job.Status.State = pointer.String("completed")
			images := &models.Images{
				Images: []*models.ImageReference{
					{
						Name:    pointer.String("capi-image"),
						ImageID: pointer.String("capi-image-id"),
					},
				},
			}
			t.Run("When import job status is completed and fails to get the image details", func(t *testing.T) {
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				mockpowervs.EXPECT().GetAllImage().Return(images, nil)
				mockpowervs.EXPECT().GetImage(gomock.AssignableToTypeOf("capi-image-id")).Return(nil, errors.New("Failed to the image details"))
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(Not(BeNil()))
				g.Expect(result.RequeueAfter).To(BeZero())
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{conditionType: infrav1beta2.ImageImportedCondition, status: corev1.ConditionTrue}})
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
			})
			image := &models.Image{
				Name:    pointer.String("capi-image"),
				ImageID: pointer.String("capi-image-id"),
				State:   "queued",
			}
			t.Run("When import job status is completed and image state is queued", func(t *testing.T) {
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				mockpowervs.EXPECT().GetAllImage().Return(images, nil)
				mockpowervs.EXPECT().GetImage(gomock.AssignableToTypeOf("capi-image-id")).Return(image, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(BeNil())
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(false))
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{infrav1beta2.ImageReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityWarning, infrav1beta2.ImageNotReadyReason}})
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			})
			t.Run("When import job status is completed and image state is undefined", func(t *testing.T) {
				image.State = "unknown"
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				mockpowervs.EXPECT().GetAllImage().Return(images, nil)
				mockpowervs.EXPECT().GetImage(gomock.AssignableToTypeOf("capi-image-id")).Return(image, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(BeNil())
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{infrav1beta2.ImageReadyCondition, corev1.ConditionUnknown, "", ""}})
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(false))
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			})
			t.Run("When import job status is completed and image state is active", func(t *testing.T) {
				image.State = "active"
				mockpowervs.EXPECT().GetJob(gomock.AssignableToTypeOf("job-1")).Return(job, nil)
				mockpowervs.EXPECT().GetAllImage().Return(images, nil)
				mockpowervs.EXPECT().GetImage(gomock.AssignableToTypeOf("capi-image-id")).Return(image, nil)
				result, err := reconciler.reconcile(powervsCluster, imageScope)
				g.Expect(err).To(BeNil())
				expectConditionsImage(g, imageScope.IBMPowerVSImage, []conditionAssertion{{conditionType: infrav1beta2.ImageReadyCondition, status: corev1.ConditionTrue}})
				g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
				g.Expect(imageScope.IBMPowerVSImage.Status.Ready).To(Equal(true))
				g.Expect(result.RequeueAfter).To(BeZero())
			})
		})
	})
}

func TestIBMPowerVSImageReconciler_delete(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
		reconciler  IBMPowerVSImageReconciler
		imageScope  *scope.PowerVSImageScope
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
		recorder := record.NewFakeRecorder(2)
		reconciler = IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: recorder,
		}
		imageScope = &scope.PowerVSImageScope{
			Logger:           klogr.New(),
			IBMPowerVSImage:  &infrav1beta2.IBMPowerVSImage{},
			IBMPowerVSClient: mockpowervs,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Reconcile deleting IBMPowerVSImage ", func(t *testing.T) {
		t.Run("Should not delete IBMPowerVSImage is neither job ID nor image ID are set", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageScope.IBMPowerVSImage.Finalizers = []string{infrav1beta2.IBMPowerVSImageFinalizer}
			_, err := reconciler.reconcileDelete(imageScope)
			g.Expect(err).To(BeNil())
			g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(Not(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer)))
		})
		t.Run("Should fail to delete the import image job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageScope.IBMPowerVSImage.Status.JobID = "job-1"
			imageScope.IBMPowerVSImage.Finalizers = []string{infrav1beta2.IBMPowerVSImageFinalizer}
			mockpowervs.EXPECT().DeleteJob(gomock.AssignableToTypeOf("job-1")).Return(errors.New("Failed to deleted the import job"))
			_, err := reconciler.reconcileDelete(imageScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
		})
		t.Run("Should delete the import image job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageScope.IBMPowerVSImage.Status.JobID = "job-1"
			imageScope.IBMPowerVSImage.Finalizers = []string{infrav1beta2.IBMPowerVSImageFinalizer}
			mockpowervs.EXPECT().DeleteJob(gomock.AssignableToTypeOf("job-1")).Return(nil)
			_, err := reconciler.reconcileDelete(imageScope)
			g.Expect(err).To(BeNil())
			g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(Not(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer)))
		})
		t.Run("Should fail to delete the image using ID when delete policy is not to retain it", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageScope.IBMPowerVSImage.Status.ImageID = "capi-image-id"
			imageScope.IBMPowerVSImage.Finalizers = []string{infrav1beta2.IBMPowerVSImageFinalizer}
			mockpowervs.EXPECT().DeleteImage(gomock.AssignableToTypeOf("capi-image-id")).Return(errors.New("Failed to delete the image"))
			_, err := reconciler.reconcileDelete(imageScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer))
		})
		t.Run("Should not delete the image using ID when delete policy is to retain it", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageScope.IBMPowerVSImage.Status.ImageID = "capi-image-id"
			imageScope.IBMPowerVSImage.Finalizers = []string{infrav1beta2.IBMPowerVSImageFinalizer}
			imageScope.IBMPowerVSImage.Spec.DeletePolicy = "retain"
			_, err := reconciler.reconcileDelete(imageScope)
			g.Expect(err).To(BeNil())
			g.Expect(imageScope.IBMPowerVSImage.Finalizers).To(Not(ContainElement(infrav1beta2.IBMPowerVSImageFinalizer)))
		})
	})
}

func expectConditionsImage(g *WithT, m *infrav1beta2.IBMPowerVSImage, expected []conditionAssertion) {
	g.Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)))
	for _, c := range expected {
		actual := conditions.Get(m, c.conditionType)
		g.Expect(actual).To(Not(BeNil()))
		g.Expect(actual.Type).To(Equal(c.conditionType))
		g.Expect(actual.Status).To(Equal(c.status))
		g.Expect(actual.Severity).To(Equal(c.severity))
		g.Expect(actual.Reason).To(Equal(c.reason))
	}
}
