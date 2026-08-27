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
	"testing"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"go.uber.org/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"

	. "github.com/onsi/gomega"
)

func newIBMPowerVSImage(name string) *infrav1.IBMPowerVSImage {
	return &infrav1.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec: infrav1.IBMPowerVSImageSpec{
			ClusterName: "test-cluster",
			Workspace:   infrav1.ResourceIdentifier{ID: "test-workspace-id"},
			StorageType: infrav1.PowerVSStorageTypeTier1,
			Object:      "rhcos.ova.gz",
			Bucket:      "my-cos-bucket",
			Region:      "us-south",
		},
	}
}

// errAuthBuilder is a ClientBuilder whose GetAuthenticator always fails.
type errAuthBuilder struct{ stubClientBuilder }

func (e errAuthBuilder) GetAuthenticator(_ context.Context) (core.Authenticator, error) {
	return nil, errors.New("authenticator error")
}

// errRCBuilder is a ClientBuilder whose GetResourceControllerClient always fails.
type errRCBuilder struct{ stubClientBuilder }

func (e errRCBuilder) GetResourceControllerClient(_ context.Context, _ ClientOptions) (resourcecontroller.ResourceController, error) {
	return nil, errors.New("rc client error")
}

// errPVSBuilder wraps a working RC client but fails on GetPowerVSClient.
type errPVSBuilder struct {
	stubClientBuilder
	rc resourcecontroller.ResourceController
}

func (e errPVSBuilder) GetResourceControllerClient(_ context.Context, _ ClientOptions) (resourcecontroller.ResourceController, error) {
	return e.rc, nil
}
func (e errPVSBuilder) GetPowerVSClient(_ context.Context, _ ClientOptions) (powervs.PowerVS, error) {
	return nil, errors.New("pvs client error")
}

func newImageScope(imageName string, mockPVS *mock.MockPowerVS) *ImageScope {
	img := newIBMPowerVSImage(imageName)
	c := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects([]client.Object{img}...).
		Build()
	return &ImageScope{
		Client:           c,
		IBMPowerVSClient: mockPVS,
		IBMPowerVSImage:  img,
		Recorder:         record.NewFakeRecorder(1000),
	}
}

func TestNewPowerVSImageScope(t *testing.T) {
	testCases := []struct {
		name        string
		params      ImageScopeParams
		expectError bool
	}{
		{
			name: "error when Client is nil",
			params: ImageScopeParams{
				Client: nil,
			},
			expectError: true,
		},
		{
			name: "error when IBMPowerVSImage is nil",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: nil,
			},
			expectError: true,
		},
		{
			name: "error when ClientBuilder is nil",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder:   nil,
			},
			expectError: true,
		},
		{
			name: "error when GetAuthenticator fails",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder:   errAuthBuilder{},
			},
			expectError: true,
		},
		{
			name: "error when GetResourceControllerClient fails",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder:   errRCBuilder{},
			},
			expectError: true,
		},
		// resolveWorkspace: workspace ID provided — GetResourceInstance error paths
		{
			name: "error when GetResourceInstance fails (workspace ID set)",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(nil, &core.DetailedResponse{}, errors.New("get resource instance failed"))
						return m
					}(),
				},
			},
			expectError: true,
		},
		{
			name: "error when resource instance has nil RegionID (workspace ID set)",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:     ptr.To("test-workspace-id"),
								RegionID: nil,
							}, &core.DetailedResponse{}, nil)
						return m
					}(),
				},
			},
			expectError: true,
		},
		{
			name: "successfully creates scope with workspace ID resolved via ResourceClient",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:     ptr.To("test-workspace-id"),
								RegionID: ptr.To("us-south"),
							}, &core.DetailedResponse{}, nil)
						return m
					}(),
				},
			},
			expectError: false,
		},
		// resolveWorkspace: workspace Name lookup paths (Workspace.ID is empty)
		{
			name: "error when GetResourceInstanceByFilter fails (workspace name lookup)",
			params: ImageScopeParams{
				Client: testEnv.Client,
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{Name: pvsImage, Namespace: defaultNamespace},
					Spec: infrav1.IBMPowerVSImageSpec{
						ClusterName: "test-cluster",
						Workspace:   infrav1.ResourceIdentifier{Name: "my-workspace"},
					},
				},
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstanceByFilter(gomock.Any()).
							Return(nil, errors.New("filter lookup failed"))
						return m
					}(),
				},
			},
			expectError: true,
		},
		{
			name: "error when workspace not found by name (nil instance returned)",
			params: ImageScopeParams{
				Client: testEnv.Client,
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{Name: pvsImage, Namespace: defaultNamespace},
					Spec: infrav1.IBMPowerVSImageSpec{
						ClusterName: "test-cluster",
						Workspace:   infrav1.ResourceIdentifier{Name: "my-workspace"},
					},
				},
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstanceByFilter(gomock.Any()).
							Return(nil, nil)
						return m
					}(),
				},
			},
			expectError: true,
		},
		{
			name: "error when workspace found by name but not in active state",
			params: ImageScopeParams{
				Client: testEnv.Client,
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{Name: pvsImage, Namespace: defaultNamespace},
					Spec: infrav1.IBMPowerVSImageSpec{
						ClusterName: "test-cluster",
						Workspace:   infrav1.ResourceIdentifier{Name: "my-workspace"},
					},
				},
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstanceByFilter(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:  ptr.To("ws-guid"),
								State: ptr.To("provisioning"),
							}, nil)
						return m
					}(),
				},
			},
			expectError: true,
		},
		{
			name: "successfully creates scope using workspace name lookup",
			params: ImageScopeParams{
				Client: testEnv.Client,
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{Name: pvsImage, Namespace: defaultNamespace},
					Spec: infrav1.IBMPowerVSImageSpec{
						ClusterName: "test-cluster",
						Workspace:   infrav1.ResourceIdentifier{Name: "my-workspace"},
					},
				},
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstanceByFilter(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:  ptr.To("ws-guid"),
								State: ptr.To(string(infrav1.WorkspaceStateActive)),
							}, nil)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:     ptr.To("ws-guid"),
								RegionID: ptr.To("us-south"),
							}, &core.DetailedResponse{}, nil)
						return m
					}(),
				},
			},
			expectError: false,
		},
		{
			name: "successfully creates scope using cluster-derived workspace name",
			params: ImageScopeParams{
				Client: testEnv.Client,
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					ObjectMeta: metav1.ObjectMeta{Name: pvsImage, Namespace: defaultNamespace},
					Spec: infrav1.IBMPowerVSImageSpec{
						ClusterName: "test-cluster",
						// Workspace.ID and Workspace.Name both empty → name derived as "test-cluster-workspace"
					},
				},
				ClientBuilder: stubClientBuilder{
					rcClient: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstanceByFilter(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:  ptr.To("ws-guid"),
								State: ptr.To(string(infrav1.WorkspaceStateActive)),
							}, nil)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:     ptr.To("ws-guid"),
								RegionID: ptr.To("us-south"),
							}, &core.DetailedResponse{}, nil)
						return m
					}(),
				},
			},
			expectError: false,
		},
		{
			name: "error when GetPowerVSClient fails",
			params: ImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newIBMPowerVSImage(pvsImage),
				ClientBuilder: errPVSBuilder{
					rc: func() *mockRC.MockResourceController {
						ctrl := gomock.NewController(t)
						m := mockRC.NewMockResourceController(ctrl)
						m.EXPECT().
							GetResourceInstance(gomock.Any()).
							Return(&resourcecontrollerv2.ResourceInstance{
								GUID:     ptr.To("test-workspace-id"),
								RegionID: ptr.To("us-south"),
							}, &core.DetailedResponse{}, nil)
						return m
					}(),
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewPowerVSImageScope(ctx, tc.params)
			if tc.expectError {
				g.Expect(err).NotTo(BeNil())
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestGetOrImportImage(t *testing.T) {
	var (
		mockPVS  *mock.MockPowerVS
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPVS = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	existingImages := &models.Images{
		Images: []*models.ImageReference{
			{Name: ptr.To("existing-image"), ImageID: ptr.To("existing-image-id")},
		},
	}
	otherImages := &models.Images{
		Images: []*models.ImageReference{
			{Name: ptr.To("other-image"), ImageID: ptr.To("other-image-id")},
		},
	}
	completedJob := &models.Job{
		Status: &models.Status{State: ptr.To(string(infrav1.PowerVSImageStateCompleted))},
	}
	failedJob := &models.Job{
		Status: &models.Status{State: ptr.To(string(infrav1.PowerVSImageStateFailed))},
	}
	inProgressJob := &models.Job{
		Status: &models.Status{State: ptr.To("in-progress")},
	}
	jobRef := &models.JobReference{ID: ptr.To("new-job-id")}
	cosBody := &models.CreateCosImageImportJob{}

	t.Run("returns existing image when name already present in workspace", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope("existing-image", mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(existingImages, nil)

		imgRef, jobRefOut, err := scope.GetOrImportImage(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(jobRefOut).To(BeNil())
		g.Expect(imgRef).NotTo(BeNil())
		g.Expect(*imgRef.Name).To(Equal("existing-image"))
	})

	t.Run("error when ListImages fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(nil, errors.New("list images failed"))

		_, _, err := scope.GetOrImportImage(ctx)

		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("list images failed"))
	})

	t.Run("returns nil-nil-nil when previous import job is still in-progress", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(inProgressJob, nil)

		imgRef, jobRefOut, err := scope.GetOrImportImage(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(imgRef).To(BeNil())
		g.Expect(jobRefOut).To(BeNil())
	})

	t.Run("error when GetCosImages fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(nil, errors.New("get cos images failed"))

		_, _, err := scope.GetOrImportImage(ctx)

		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("get cos images failed"))
	})

	t.Run("triggers new import job when previous job completed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(completedJob, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).Return(jobRef, nil)

		imgRef, jobRefOut, err := scope.GetOrImportImage(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(imgRef).To(BeNil())
		g.Expect(jobRefOut).NotTo(BeNil())
		g.Expect(*jobRefOut.ID).To(Equal("new-job-id"))
	})

	t.Run("triggers new import job when previous job failed", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(failedJob, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).Return(jobRef, nil)

		imgRef, jobRefOut, err := scope.GetOrImportImage(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(imgRef).To(BeNil())
		g.Expect(jobRefOut).NotTo(BeNil())
	})

	t.Run("triggers new import job when no previous job exists (nil job returned)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).Return(jobRef, nil)

		imgRef, jobRefOut, err := scope.GetOrImportImage(ctx)

		g.Expect(err).To(BeNil())
		g.Expect(imgRef).To(BeNil())
		g.Expect(jobRefOut).NotTo(BeNil())
	})

	t.Run("error when CreateCosImage fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(completedJob, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).
			Return(nil, errors.New("create cos image failed"))

		_, _, err := scope.GetOrImportImage(ctx)

		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("create cos image failed"))
	})

	t.Run("import body omits StorageType when spec field is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Spec.StorageType = ""

		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(completedJob, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).
			DoAndReturn(func(_ interface{}, body *models.CreateCosImageImportJob) (*models.JobReference, error) {
				g.Expect(body.StorageType).To(BeEmpty())
				return jobRef, nil
			})

		_, jobRefOut, err := scope.GetOrImportImage(ctx)
		g.Expect(err).To(BeNil())
		g.Expect(jobRefOut).NotTo(BeNil())
	})

	t.Run("import body carries StorageType when spec field is set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Spec.StorageType = infrav1.PowerVSStorageTypeTier3

		mockPVS.EXPECT().ListImages(gomock.Any()).Return(otherImages, nil)
		mockPVS.EXPECT().GetCosImages(gomock.Any(), gomock.Any()).Return(completedJob, nil)
		mockPVS.EXPECT().CreateCosImage(gomock.Any(), gomock.AssignableToTypeOf(cosBody)).
			DoAndReturn(func(_ interface{}, body *models.CreateCosImageImportJob) (*models.JobReference, error) {
				g.Expect(body.StorageType).To(Equal(string(infrav1.PowerVSStorageTypeTier3)))
				return jobRef, nil
			})

		_, _, err := scope.GetOrImportImage(ctx)
		g.Expect(err).To(BeNil())
	})
}

func TestDeleteImage(t *testing.T) {
	var (
		mockPVS  *mock.MockPowerVS
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPVS = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("no-op when ImageID is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)

		err := scope.DeleteImage(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("successfully deletes image when ImageID is set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Status.ImageID = "img-abc123"
		mockPVS.EXPECT().DeleteImage(gomock.Any(), "img-abc123").Return(nil)

		err := scope.DeleteImage(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("returns error when DeleteImage API fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Status.ImageID = "img-abc123"
		mockPVS.EXPECT().DeleteImage(gomock.Any(), "img-abc123").Return(errors.New("api error"))

		err := scope.DeleteImage(ctx)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("api error"))
	})
}

func TestDeleteImportJob(t *testing.T) {
	var (
		mockPVS  *mock.MockPowerVS
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockPVS = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	t.Run("no-op when JobID is empty", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)

		err := scope.DeleteImportJob(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("successfully deletes import job when JobID is set", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Status.JobID = "job-xyz789"
		mockPVS.EXPECT().DeleteJob(gomock.Any(), "job-xyz789").Return(nil)

		err := scope.DeleteImportJob(ctx)
		g.Expect(err).To(BeNil())
	})

	t.Run("returns error when DeleteJob API fails", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		scope := newImageScope(pvsImage, mockPVS)
		scope.IBMPowerVSImage.Status.JobID = "job-xyz789"
		mockPVS.EXPECT().DeleteJob(gomock.Any(), "job-xyz789").Return(errors.New("job delete failed"))

		err := scope.DeleteImportJob(ctx)
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("job delete failed"))
	})
}

func TestSetImageActive(t *testing.T) {
	testCases := []struct {
		name          string
		initialState  infrav1.PowerVSImageState
		expectedState infrav1.PowerVSImageState
	}{
		{
			name:          "sets state to ACTIVE from empty",
			initialState:  "",
			expectedState: infrav1.PowerVSImageStateACTIVE,
		},
		{
			name:          "sets state to ACTIVE from importing",
			initialState:  infrav1.PowerVSImageStateImporting,
			expectedState: infrav1.PowerVSImageStateACTIVE,
		},
		{
			name:          "idempotent when already ACTIVE",
			initialState:  infrav1.PowerVSImageStateACTIVE,
			expectedState: infrav1.PowerVSImageStateACTIVE,
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := &ImageScope{
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					Status: infrav1.IBMPowerVSImageStatus{ImageState: tc.initialState},
				},
			}
			scope.SetImageActive()
			g.Expect(scope.IBMPowerVSImage.Status.ImageState).To(Equal(tc.expectedState))
		})
	}
}

func TestIsImageActive(t *testing.T) {
	testCases := []struct {
		name     string
		state    infrav1.PowerVSImageState
		expected bool
	}{
		{
			name:     "returns true when state is ACTIVE",
			state:    infrav1.PowerVSImageStateACTIVE,
			expected: true,
		},
		{
			name:     "returns false when state is importing",
			state:    infrav1.PowerVSImageStateImporting,
			expected: false,
		},
		{
			name:     "returns false when state is queued",
			state:    infrav1.PowerVSImageStateQueued,
			expected: false,
		},
		{
			name:     "returns false when state is empty",
			state:    "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := &ImageScope{
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{
					Status: infrav1.IBMPowerVSImageStatus{ImageState: tc.state},
				},
			}
			g.Expect(scope.IsImageActive()).To(Equal(tc.expected))
		})
	}
}

func TestSetImageID(t *testing.T) {
	testCases := []struct {
		name       string
		input      *string
		expectedID string
	}{
		{
			name:       "sets ImageID from non-nil pointer",
			input:      ptr.To("img-abc"),
			expectedID: "img-abc",
		},
		{
			name:       "does not mutate ImageID when pointer is nil",
			input:      nil,
			expectedID: "",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := &ImageScope{
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{},
			}
			scope.SetImageID(tc.input)
			g.Expect(scope.GetImageID()).To(Equal(tc.expectedID))
		})
	}
}

func TestSetImageState(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected infrav1.PowerVSImageState
	}{
		{
			name:     "sets state to active",
			input:    "active",
			expected: infrav1.PowerVSImageStateACTIVE,
		},
		{
			name:     "sets state to importing",
			input:    "importing",
			expected: infrav1.PowerVSImageStateImporting,
		},
		{
			name:     "sets state to queued",
			input:    "queued",
			expected: infrav1.PowerVSImageStateQueued,
		},
		{
			name:     "sets state to failed",
			input:    "failed",
			expected: infrav1.PowerVSImageStateFailed,
		},
		{
			name:     "sets state to empty string",
			input:    "",
			expected: infrav1.PowerVSImageState(""),
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := &ImageScope{
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{},
			}
			scope.SetImageState(tc.input)
			g.Expect(scope.GetImageState()).To(Equal(tc.expected))
		})
	}
}

func TestSetJobID(t *testing.T) {
	testCases := []struct {
		name       string
		input      string
		expectedID string
	}{
		{
			name:       "sets JobID to a valid value",
			input:      "job-abc",
			expectedID: "job-abc",
		},
		{
			name:       "clears JobID with empty string",
			input:      "",
			expectedID: "",
		},
	}

	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			scope := &ImageScope{
				IBMPowerVSImage: &infrav1.IBMPowerVSImage{},
			}
			scope.SetJobID(tc.input)
			g.Expect(scope.GetJobID()).To(Equal(tc.expectedID))
		})
	}
}
