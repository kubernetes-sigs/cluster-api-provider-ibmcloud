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

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"

	. "github.com/onsi/gomega"
)

func newPowervsImage(imageName string) *infrav1beta2.IBMPowerVSImage {
	return &infrav1beta2.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageName,
			Namespace: "default",
		},
		Spec: infrav1beta2.IBMPowerVSImageSpec{
			ClusterName:       "test-cluster",
			ServiceInstanceID: "test-service-ID",
			StorageType:       "foo-tier",
			Object:            core.StringPtr("foo-obj"),
			Bucket:            core.StringPtr("foo-bucket"),
			Region:            core.StringPtr("foo-zone"),
		},
	}
}

func setupPowerVSImageScope(imageName string, mockpowervs *mock.MockPowerVS) *PowerVSImageScope {
	powervsImage := newPowervsImage(imageName)
	initObjects := []client.Object{powervsImage}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSImageScope{
		Client:           client,
		Logger:           klogr.New(),
		IBMPowerVSClient: mockpowervs,
		IBMPowerVSImage:  powervsImage,
	}
}

func TestNewPowerVSImageScope(t *testing.T) {
	testCases := []struct {
		name   string
		params PowerVSImageScopeParams
	}{
		{
			name: "Error when Client in nil",
			params: PowerVSImageScopeParams{
				Client: nil,
			},
		},
		{
			name: "Error when IBMPowerVSImage is nil",
			params: PowerVSImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: nil,
			},
		},
		{
			name: "Failed to get authenticator",
			params: PowerVSImageScopeParams{
				Client:          testEnv.Client,
				IBMPowerVSImage: newPowervsImage(pvsImage),
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPowerVSImageScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateImageCOSBucket(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Create Import Job", func(t *testing.T) {
		images := &models.Images{
			Images: []*models.ImageReference{
				{
					Name: core.StringPtr("foo-image-1"),
				},
			},
		}
		var serviceInstanceID string
		job := &models.Job{
			Status: &models.Status{
				State: core.StringPtr("completed"),
			},
		}
		body := &models.CreateCosImageImportJob{}
		jobReference := &models.JobReference{
			ID: core.StringPtr("foo-jobref-id"),
		}

		t.Run("Should create image import job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetCosImages(gomock.AssignableToTypeOf(serviceInstanceID)).Return(job, nil)
			mockpowervs.EXPECT().CreateCosImage(gomock.AssignableToTypeOf(body)).Return(jobReference, nil)
			_, out, err := scope.CreateImageCOSBucket()
			g.Expect(err).To(BeNil())
			require.Equal(t, jobReference, out)
		})

		t.Run("Return exsisting Image", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			imageReference := &models.ImageReference{
				Name: core.StringPtr("foo-image-1"),
			}
			scope := setupPowerVSImageScope("foo-image-1", mockpowervs)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			out, _, err := scope.CreateImageCOSBucket()
			g.Expect(err).To(BeNil())
			require.Equal(t, imageReference, out)
		})

		t.Run("Error while listing images", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			mockpowervs.EXPECT().GetAllImage().Return(images, errors.New("Failed to list images"))
			_, _, err := scope.CreateImageCOSBucket()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Previous import job in-progress", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			job := &models.Job{
				Status: &models.Status{
					State: core.StringPtr("in-progress"),
				},
			}
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetCosImages(gomock.AssignableToTypeOf(serviceInstanceID)).Return(job, nil)
			_, _, err := scope.CreateImageCOSBucket()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while creating import job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetCosImages(gomock.AssignableToTypeOf(serviceInstanceID)).Return(job, nil)
			mockpowervs.EXPECT().CreateCosImage(gomock.AssignableToTypeOf(body)).Return(jobReference, errors.New("Failed to create image import job"))
			_, _, err := scope.CreateImageCOSBucket()
			g.Expect(err).To((Not(BeNil())))
		})
	})
}

func TestDeleteImage(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Delete Image", func(t *testing.T) {
		var id string
		t.Run("Should delete image", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			scope.IBMPowerVSImage.Status.ImageID = pvsImage + "-id"
			mockpowervs.EXPECT().DeleteImage(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteImage()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting image", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			scope.IBMPowerVSImage.Status.ImageID = pvsImage + "-id"
			mockpowervs.EXPECT().DeleteImage(gomock.AssignableToTypeOf(id)).Return(errors.New("Failed to delete image"))
			err := scope.DeleteImage()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteImportJob(t *testing.T) {
	var (
		mockpowervs *mock.MockPowerVS
		mockCtrl    *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Delete Import Job", func(t *testing.T) {
		var id string
		t.Run("Should delete image import job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			scope.IBMPowerVSImage.Status.JobID = "foo-job-id"
			mockpowervs.EXPECT().DeleteJob(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteImportJob()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting image import job", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupPowerVSImageScope(pvsImage, mockpowervs)
			scope.IBMPowerVSImage.Status.JobID = "foo-job-id"
			mockpowervs.EXPECT().DeleteJob(gomock.AssignableToTypeOf(id)).Return(errors.New("Failed to delete image import job"))
			err := scope.DeleteImportJob()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}
