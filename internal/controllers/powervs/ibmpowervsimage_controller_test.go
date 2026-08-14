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

package powervs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	powervsscope "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/scope/powervs"
	powervssvc "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"

	. "github.com/onsi/gomega"
)

const (
	testImageCluster = "capi-powervs-cluster"
	testImageName    = "capi-image"
	testJobID        = "job-1"
	testImageID      = "capi-image-id"
)

func baseImage() *infrav1.IBMPowerVSImage {
	return &infrav1.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testImageName,
			Finalizers: []string{infrav1.IBMPowerVSImageFinalizer},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       ibmPowerVSClusterKind,
					Name:       testImageCluster,
					UID:        "1",
				},
			},
		},
		Spec: infrav1.IBMPowerVSImageSpec{
			ClusterName: testImageCluster,
			Object:      "capi-image.ova.gz",
			Region:      "us-south",
			Bucket:      "capi-bucket",
		},
	}
}

func baseCluster() *infrav1.IBMPowerVSCluster {
	return &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{Name: testImageCluster},
		Spec: infrav1.IBMPowerVSClusterSpec{
			Zone:     "us-south-1",
			Topology: infrav1.PowerVSVirtualIPTopology,
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "workspace-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "network-id"},
			},
		},
	}
}

func newImageReconciler() IBMPowerVSImageReconciler {
	return IBMPowerVSImageReconciler{
		Client:        testEnv.Client,
		Recorder:      record.NewFakeRecorder(10),
		ClientBuilder: stubClientBuilder{},
	}
}

func newImageScope(image *infrav1.IBMPowerVSImage, mockPVS *mock.MockPowerVS) *powervsscope.ImageScope {
	c := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(image).
		Build()
	return &powervsscope.ImageScope{
		Client:           c,
		IBMPowerVSImage:  image,
		IBMPowerVSClient: mockPVS,
	}
}

// mockClientBuilder is a test-only ClientBuilder that injects pre-built mock clients.
// It embeds stubClientBuilder for all methods not overridden here.
type mockClientBuilder struct {
	stubClientBuilder
	pvs *mock.MockPowerVS
	rc  *mockRC.MockResourceController
}

func (b mockClientBuilder) GetPowerVSClient(_ context.Context, _ powervsscope.ClientOptions) (powervssvc.PowerVS, error) {
	return b.pvs, nil
}

func (b mockClientBuilder) GetResourceControllerClient(_ context.Context, _ powervsscope.ClientOptions) (resourcecontroller.ResourceController, error) {
	return b.rc, nil
}

func assertConditionV1Beta2(t *testing.T, image *infrav1.IBMPowerVSImage, condType clusterv1.ConditionType, status corev1.ConditionStatus, severity clusterv1.ConditionSeverity, reason string) {
	t.Helper()
	g := NewWithT(t)
	actual := deprecatedv1beta1conditions.Get(image, condType)
	g.Expect(actual).NotTo(BeNil(), "expected condition %s to be set", condType)
	g.Expect(actual.Status).To(Equal(status))
	if severity != "" {
		g.Expect(actual.Severity).To(Equal(severity))
	}
	if reason != "" {
		g.Expect(actual.Reason).To(Equal(reason))
	}
}

func assertConditionV1Beta3(t *testing.T, image *infrav1.IBMPowerVSImage, condType string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	g := NewWithT(t)
	actual := conditions.Get(image, condType)
	g.Expect(actual).NotTo(BeNil(), "expected condition %s to be set", condType)
	g.Expect(actual.Status).To(Equal(status))
	if reason != "" {
		g.Expect(actual.Reason).To(Equal(reason))
	}
}

func TestIBMPowerVSImageReconciler_Reconcile(t *testing.T) {
	t.Run("returns nil when IBMPowerVSImage is not found", func(t *testing.T) {
		g := NewWithT(t)
		r := newImageReconciler()
		result, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: "default", Name: "does-not-exist"},
		})
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("adds finalizer on first reconcile", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Name: testImageName, Namespace: ns.Name},
			Spec: infrav1.IBMPowerVSImageSpec{
				ClusterName: testImageCluster,
				Object:      "capi-image.ova.gz",
				Region:      "us-south",
				Bucket:      "capi-bucket",
			},
		}
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		// Wait for the object to be visible in the API server before reconciling.
		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		r := newImageReconciler()
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).To(BeNil())

		// EnsureFinalizer writes the finalizer via a Patch. Assert with Eventually to
		// allow the envtest API server cache to reflect the write before we read back.
		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			if err := testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got); err != nil {
				return false
			}
			for _, f := range got.Finalizers {
				if f == infrav1.IBMPowerVSImageFinalizer {
					return true
				}
			}
			return false
		}, 10*time.Second).Should(BeTrue(), "finalizer should be present after first reconcile")
	})

	t.Run("returns error when IBMPowerVSCluster is not found", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		image := baseImage()
		image.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		r := newImageReconciler()
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).NotTo(BeNil())
	})

	t.Run("sets IBMPowerVSImageReady=Unknown when scope creation fails", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		image := baseImage()
		image.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		r := IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
			// nil ClientBuilder causes NewPowerVSImageScope to fail
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).NotTo(BeNil())

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition, metav1.ConditionUnknown, infrav1.IBMPowerVSImageReadyUnknownReason)
	})

	t.Run("returns error when Get returns unexpected error", func(t *testing.T) {
		g := NewWithT(t)

		// Build a fake client that returns a server-error on Get.
		errClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return apierrors.NewInternalError(errors.New("etcd unavailable"))
				},
			}).
			Build()

		r := IBMPowerVSImageReconciler{
			Client:        errClient,
			Recorder:      record.NewFakeRecorder(10),
			ClientBuilder: stubClientBuilder{},
		}
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: "default", Name: "any-image"},
		})
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to get IBMPowerVSImage"))
	})

	t.Run("sets WorkspaceReady=False when scope creation fails due to inactive workspace", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		// Image with no Workspace.ID forces name-based resolution via the RC client.
		image := baseImage()
		image.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		inactiveState := "inactive"
		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("ws-guid"),
				State: &inactiveState,
			}, nil)

		r := IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
			ClientBuilder: mockClientBuilder{
				rc: rcMock,
			},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("workspace is not in active state"))

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		assertConditionV1Beta3(t, got, infrav1.WorkspaceReadyCondition, metav1.ConditionFalse, infrav1.WorkspaceNotReadyReason)
		// patchIBMPowerVSImage SetSummaryCondition rolls up WorkspaceReadyCondition (False) into the
		// IBMPowerVSImageReadyCondition, so the final persisted state is False, not Unknown.
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition, metav1.ConditionFalse, infrav1.IBMPowerVSImageNotReadyReason)
	})

	t.Run("calls reconcileDelete when DeletionTimestamp is set and scope creation succeeds", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		// Image with Workspace.ID set so resolveWorkspace goes directly to GetResourceInstance.
		image := baseImage()
		image.Namespace = ns.Name
		image.Spec.Workspace.ID = "ws-id"
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		// Trigger deletion — object stays (finalizer present), DeletionTimestamp is set.
		g.Expect(testEnv.Delete(ctx, image)).To(Succeed())
		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			_ = testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)
			return !got.DeletionTimestamp.IsZero()
		}, 10*time.Second).Should(BeTrue())

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().
			GetResourceInstance(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				RegionID: ptr.To("us-south"),
			}, nil, nil)

		pvsMock := mock.NewMockPowerVS(mockCtrl)

		r := IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
			ClientBuilder: mockClientBuilder{
				rc:  rcMock,
				pvs: pvsMock,
			},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).To(BeNil())

		// After reconcileDelete removes the finalizer and the patch is applied, the API server
		// garbage-collects the object (DeletionTimestamp was already set). Expect it to be gone.
		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			err := testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)
			return apierrors.IsNotFound(err)
		}, 10*time.Second).Should(BeTrue(), "image should be fully deleted after finalizer removal")
	})

	t.Run("calls reconcile when image is active and scope creation succeeds", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		// Image with Workspace.ID and an existing ImageID so GetOrImportImage returns it quickly.
		image := baseImage()
		image.Namespace = ns.Name
		image.Spec.Workspace.ID = "ws-id"
		image.Status.ImageID = testImageID
		image.Status.ImageState = infrav1.PowerVSImageStateACTIVE
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().
			GetResourceInstance(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				RegionID: ptr.To("us-south"),
			}, nil, nil)

		pvsMock := mock.NewMockPowerVS(mockCtrl)
		// GetOrImportImage will call ListImages; return the existing image.
		pvsMock.EXPECT().
			ListImages(gomock.Any()).
			Return(&models.Images{
				Images: []*models.ImageReference{{
					Name:    ptr.To(testImageName),
					ImageID: ptr.To(testImageID),
				}},
			}, nil)
		// GetImage is called after the image reference is found.
		pvsMock.EXPECT().
			GetImage(gomock.Any(), testImageID).
			Return(&models.Image{
				Name:  ptr.To(testImageName),
				State: string(infrav1.PowerVSImageStateACTIVE),
			}, nil)

		r := IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
			ClientBuilder: mockClientBuilder{
				rc:  rcMock,
				pvs: pvsMock,
			},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).To(BeNil())

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition, metav1.ConditionTrue, infrav1.IBMPowerVSImageReadyReason)
	})

	t.Run("patchIBMPowerVSImage sets ImageReady=False when condition absent and image not active", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		// Image with no owner reference causes shouldAdopt=true → reconcile returns early
		// without setting IBMPowerVSImageReadyCondition → patchIBMPowerVSImage takes the c==nil branch.
		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:       testImageName,
				Namespace:  ns.Name,
				Finalizers: []string{infrav1.IBMPowerVSImageFinalizer},
				// No OwnerReferences → shouldAdopt returns true
			},
			Spec: infrav1.IBMPowerVSImageSpec{
				ClusterName: testImageCluster,
				Workspace:   infrav1.ResourceIdentifier{ID: "ws-id"},
				Object:      "capi-image.ova.gz",
				Region:      "us-south",
				Bucket:      "capi-bucket",
			},
			// Status.ImageState not ACTIVE → patchIBMPowerVSImage sets False
		}
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().GetResourceInstance(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{RegionID: ptr.To("us-south")}, nil, nil)
		pvsMock := mock.NewMockPowerVS(mockCtrl)

		r := IBMPowerVSImageReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(10),
			ClientBuilder: mockClientBuilder{rc: rcMock, pvs: pvsMock},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).To(BeNil())

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		// patchIBMPowerVSImage enters the c==nil branch (lines 312-325) and sets False,
		// but SetSummaryCondition then re-computes from ForConditionTypes (WorkspaceReadyCondition is
		// absent → Unknown), overriding to Unknown/IBMPowerVSImageReadyUnknownReason.
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition, metav1.ConditionUnknown, infrav1.IBMPowerVSImageReadyUnknownReason)
	})

	t.Run("patchIBMPowerVSImage sets ImageReady=True when condition absent and image is active", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		// Same as above but with ImageState=ACTIVE → patchIBMPowerVSImage sets True.
		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:       testImageName,
				Namespace:  ns.Name,
				Finalizers: []string{infrav1.IBMPowerVSImageFinalizer},
			},
			Spec: infrav1.IBMPowerVSImageSpec{
				ClusterName: testImageCluster,
				Workspace:   infrav1.ResourceIdentifier{ID: "ws-id"},
				Object:      "capi-image.ova.gz",
				Region:      "us-south",
				Bucket:      "capi-bucket",
			},
		}
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		// Status is a subresource — must be updated separately.
		image.Status.ImageState = infrav1.PowerVSImageStateACTIVE
		g.Expect(testEnv.Status().Update(ctx, image)).To(Succeed())

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			if err := testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got); err != nil {
				return false
			}
			return got.Status.ImageState == infrav1.PowerVSImageStateACTIVE
		}, 10*time.Second).Should(BeTrue(), "image status should reflect ACTIVE state")

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().GetResourceInstance(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{RegionID: ptr.To("us-south")}, nil, nil)
		pvsMock := mock.NewMockPowerVS(mockCtrl)

		r := IBMPowerVSImageReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(10),
			ClientBuilder: mockClientBuilder{rc: rcMock, pvs: pvsMock},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).To(BeNil())

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		// patchIBMPowerVSImage enters the c==nil branch (lines 313-318) and sets True,
		// but SetSummaryCondition then re-computes from ForConditionTypes (WorkspaceReadyCondition is
		// absent → Unknown), overriding the value.
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition, metav1.ConditionUnknown, infrav1.IBMPowerVSImageReadyUnknownReason)
	})
}

func TestIBMPowerVSImageReconciler_reconcile(t *testing.T) {
	var (
		mockPVS  *mock.MockPowerVS
		mockCtrl *gomock.Controller
		r        IBMPowerVSImageReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPVS = mock.NewMockPowerVS(mockCtrl)
		r = IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
		}
	}
	teardown := func() { mockCtrl.Finish() }

	cluster := baseCluster()

	t.Run("sets owner reference when image has no owner", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Name: testImageName},
			Spec:       infrav1.IBMPowerVSImageSpec{ClusterName: testImageCluster},
		}
		scope := newImageScope(image, mockPVS)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{}))
		g.Expect(scope.IBMPowerVSImage.OwnerReferences).To(HaveLen(1))
		g.Expect(scope.IBMPowerVSImage.OwnerReferences[0].Kind).To(Equal(ibmPowerVSClusterKind))
	})

	t.Run("sets cluster label when missing", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Labels = nil
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{Images: []*models.ImageReference{}}, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.Any()).Return(&models.JobReference{ID: ptr.To("new-job")}, nil)

		_, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Labels[clusterv1.ClusterNameLabel]).To(Equal(testImageCluster))
	})

	t.Run("returns error and requeues when GetJob fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(nil, errors.New("api unavailable"))

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("api unavailable"))
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})

	t.Run("requeues when job status is nil", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(&models.Job{Status: nil}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})

	t.Run("requeues with ImageQueued condition when job state is queued", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(&models.Job{
			Status: &models.Status{State: ptr.To(string(infrav1.PowerVSImageStateQueued))},
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
		g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(infrav1.PowerVSImageStateQueued))
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageImportedV1Beta2Condition,
			corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, string(infrav1.PowerVSImageStateQueued))
	})

	t.Run("returns error and requeues with ImageImportFailed condition when job state is failed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(&models.Job{
			Status: &models.Status{State: ptr.To(string(infrav1.PowerVSImageStateFailed)), Message: "disk error"},
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("disk error"))
		g.Expect(result.RequeueAfter).NotTo(BeZero())
		g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(infrav1.PowerVSImageStateFailed))
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageImportedV1Beta2Condition,
			corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.ImageImportFailedV1Beta2Reason)
	})

	t.Run("requeues with importing condition when job state is in-progress", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(&models.Job{
			Status: &models.Status{State: ptr.To("in-progress")},
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
		g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(infrav1.PowerVSImageStateImporting))
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageImportedV1Beta2Condition,
			corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.ImageNotReadyV1Beta2Reason)
	})

	t.Run("stores new JobID when completed job triggers a new import", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := baseImage()
		image.Status.JobID = testJobID
		scope := newImageScope(image, mockPVS)

		mockPVS.EXPECT().GetJob(gomock.Any(), testJobID).Return(&models.Job{
			Status: &models.Status{State: ptr.To(string(infrav1.PowerVSImageStateCompleted))},
		}, nil)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To("other"), ImageID: ptr.To("other-id")}},
		}, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.Any()).Return(&models.JobReference{ID: ptr.To("new-job-2")}, nil)

		_, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Status.JobID).To(Equal("new-job-2"))
	})

	t.Run("returns error when GetOrImportImage fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(nil, errors.New("list failed"))

		_, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("list failed"))
	})

	t.Run("returns error when GetImage fails after image reference found", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To(testImageName), ImageID: ptr.To(testImageID)}},
		}, nil)
		mockPVS.EXPECT().GetImage(gomock.Any(), testImageID).Return(nil, errors.New("get image failed"))

		_, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("get image failed"))
	})

	t.Run("requeues with ImageReady=False when image state is queued", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To(testImageName), ImageID: ptr.To(testImageID)}},
		}, nil)
		mockPVS.EXPECT().GetImage(gomock.Any(), testImageID).Return(&models.Image{
			Name:    ptr.To(testImageName),
			ImageID: ptr.To(testImageID),
			State:   string(infrav1.PowerVSImageStateQueued),
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
		g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(infrav1.PowerVSImageStateQueued))
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageReadyV1Beta2Condition,
			corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1.ImageNotReadyV1Beta2Reason)
	})

	t.Run("marks image active and sets ImageReady=True when image state is active", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To(testImageName), ImageID: ptr.To(testImageID)}},
		}, nil)
		mockPVS.EXPECT().GetImage(gomock.Any(), testImageID).Return(&models.Image{
			Name:    ptr.To(testImageName),
			ImageID: ptr.To(testImageID),
			State:   string(infrav1.PowerVSImageStateACTIVE),
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).To(BeZero())
		g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(infrav1.PowerVSImageStateACTIVE))
		g.Expect(scope.IsImageActive()).To(BeTrue())
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageReadyV1Beta2Condition,
			corev1.ConditionTrue, "", "")
	})

	t.Run("requeues with ImageReady=Unknown when image state is undefined", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To(testImageName), ImageID: ptr.To(testImageID)}},
		}, nil)
		mockPVS.EXPECT().GetImage(gomock.Any(), testImageID).Return(&models.Image{
			Name:    ptr.To(testImageName),
			ImageID: ptr.To(testImageID),
			State:   "initializing",
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
		assertConditionV1Beta2(t, scope.IBMPowerVSImage, infrav1.ImageReadyV1Beta2Condition,
			corev1.ConditionUnknown, clusterv1.ConditionSeverityNone, infrav1.ImageStateUnknownV1Beta2Reason)
	})

	t.Run("requeues when import is in-progress on the remote side (nil img and nil jobRef)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(baseImage(), mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(&models.Images{
			Images: []*models.ImageReference{{Name: ptr.To("other"), ImageID: ptr.To("other-id")}},
		}, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(&models.Job{
			Status: &models.Status{State: ptr.To("in-progress")},
		}, nil)

		result, err := r.reconcile(ctx, cluster, scope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})
}

func TestIBMPowerVSImageReconciler_reconcileDelete(t *testing.T) {
	var (
		mockPVS  *mock.MockPowerVS
		mockCtrl *gomock.Controller
		r        IBMPowerVSImageReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPVS = mock.NewMockPowerVS(mockCtrl)
		r = IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
		}
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("removes finalizer when neither ImageID nor JobID are set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
		}
		scope := newImageScope(image, mockPVS)

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Finalizers).NotTo(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("removes finalizer when JobID is set and DeleteJob succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Status:     infrav1.IBMPowerVSImageStatus{JobID: testJobID},
		}
		scope := newImageScope(image, mockPVS)
		mockPVS.EXPECT().DeleteJob(gomock.Any(), testJobID).Return(nil)

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Finalizers).NotTo(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("retains finalizer when DeleteJob fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Status:     infrav1.IBMPowerVSImageStatus{JobID: testJobID},
		}
		scope := newImageScope(image, mockPVS)
		mockPVS.EXPECT().DeleteJob(gomock.Any(), testJobID).Return(errors.New("job delete failed"))

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("job delete failed"))
		g.Expect(scope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("removes finalizer when ImageID is set and DeleteImage succeeds", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Status:     infrav1.IBMPowerVSImageStatus{ImageID: testImageID},
		}
		scope := newImageScope(image, mockPVS)
		mockPVS.EXPECT().DeleteImage(gomock.Any(), testImageID).Return(nil)

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Finalizers).NotTo(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("retains finalizer when DeleteImage fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Status:     infrav1.IBMPowerVSImageStatus{ImageID: testImageID},
		}
		scope := newImageScope(image, mockPVS)
		mockPVS.EXPECT().DeleteImage(gomock.Any(), testImageID).Return(errors.New("delete failed"))

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("delete failed"))
		g.Expect(scope.IBMPowerVSImage.Finalizers).To(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("skips DeleteImage and removes finalizer when DeletePolicy is retain", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		// gomock will fail if DeleteImage is unexpectedly called.

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Spec:       infrav1.IBMPowerVSImageSpec{DeletePolicy: infrav1.PowerVSImageDeletePolicyRetain},
			Status:     infrav1.IBMPowerVSImageStatus{ImageID: testImageID},
		}
		scope := newImageScope(image, mockPVS)

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Finalizers).NotTo(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})

	t.Run("deletes image by ImageID even when JobID is also set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		image := &infrav1.IBMPowerVSImage{
			ObjectMeta: metav1.ObjectMeta{Finalizers: []string{infrav1.IBMPowerVSImageFinalizer}},
			Status:     infrav1.IBMPowerVSImageStatus{ImageID: testImageID, JobID: testJobID},
		}
		scope := newImageScope(image, mockPVS)
		mockPVS.EXPECT().DeleteImage(gomock.Any(), testImageID).Return(nil)

		_, err := r.reconcileDelete(ctx, scope)
		g.Expect(err).To(BeNil())
		g.Expect(scope.IBMPowerVSImage.Finalizers).NotTo(ContainElement(infrav1.IBMPowerVSImageFinalizer))
	})
}

func TestIBMPowerVSImageReconciler_Conditions(t *testing.T) {
	t.Run("IBMPowerVSImageReady=Unknown is set when scope creation fails", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster := baseCluster()
		cluster.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, cluster)).To(Succeed()) }()

		image := baseImage()
		image.Namespace = ns.Name
		g.Expect(testEnv.Create(ctx, image)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, image)).To(Succeed()) }()

		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSImage{}
			return testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got) == nil
		}, 10*time.Second).Should(BeTrue())

		r := IBMPowerVSImageReconciler{
			Client:   testEnv.Client,
			Recorder: record.NewFakeRecorder(10),
			// nil ClientBuilder → scope creation fails
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: testImageName},
		})
		g.Expect(err).NotTo(BeNil())

		got := &infrav1.IBMPowerVSImage{}
		g.Expect(testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: testImageName}, got)).To(Succeed())
		assertConditionV1Beta3(t, got, infrav1.IBMPowerVSImageReadyCondition,
			metav1.ConditionUnknown, infrav1.IBMPowerVSImageReadyUnknownReason)
	})
}
