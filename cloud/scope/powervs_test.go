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
	"encoding/base64"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/IBM-Cloud/power-go-client/power/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	mockpowervs *mock.MockPowerVS
)

func newMachine(clusterName, machineName string) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: pstring(machineName),
			},
		},
	}
}

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{},
	}
}

func newBootstrapSecret(clusterName, machineName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("user data"),
		},
	}
}

func newPowerVSCluster(name string) *infrav1.IBMPowerVSCluster {
	return &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1.IBMPowerVSMachine {
	image := &infrav1.IBMPowerVSResourceReference{}
	network := infrav1.IBMPowerVSResourceReference{}

	if !isID {
		image.Name = imageRef
		network.Name = networkRef
	} else {
		image.ID = imageRef
		network.ID = networkRef
	}

	return &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			Memory:     "8",
			Processors: "0.25",
			Image:      image,
			Network:    network,
		},
	}
}

func pstring(name string) *string {
	return pointer.String(name)
}

func newPowervsImage(imageName string) *infrav1.IBMPowerVSImage {
	return &infrav1.IBMPowerVSImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageName,
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSImageSpec{
			ClusterName:       "test-cluster",
			ServiceInstanceID: "test-service-ID",
			Object:            pstring("sample-image.ova.gz"),
			Bucket:            pstring("sample-bucket"),
			Region:            pstring("us-south"),
		},
	}
}

func setupPowerVSImageScope(imageName string) (*PowerVSImageScope, error) {
	powervsImage := newPowervsImage(imageName)
	initObjects := []client.Object{powervsImage}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSImageScope{
		client:           client,
		Logger:           klogr.New(),
		IBMPowerVSClient: mockpowervs,
		IBMPowerVSImage:  powervsImage,
	}, nil
}

func setupPowerVSMachineScope(clusterName string, machineName string, imageID *string, networkID *string, isID bool) (*PowerVSMachineScope, error) {
	cluster := newCluster(clusterName)
	machine := newMachine(clusterName, machineName)
	secret := newBootstrapSecret(clusterName, machineName)
	powervsMachine := newPowerVSMachine(clusterName, machineName, imageID, networkID, isID)
	powervsCluster := newPowerVSCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, powervsCluster, powervsMachine,
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSMachineScope{
		client:            client,
		Logger:            klogr.New(),
		IBMPowerVSClient:  mockpowervs,
		Cluster:           cluster,
		Machine:           machine,
		IBMPowerVSCluster: powervsCluster,
		IBMPowerVSMachine: powervsMachine,
	}, nil
}

var _ = Describe("PowerVS machine and image creation", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockpowervs = mock.NewMockPowerVS(ctrl)

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Create an IBMPowerVSMachine", func() {
		var instances *models.PVMInstances

		BeforeEach(func() {
			instances = &models.PVMInstances{}
		})

		It("should not error and create a machine", func() {

			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-ID"), pstring("test-net-ID"), true)
			Expect(err).NotTo(HaveOccurred())

			instanceList := &models.PVMInstanceList{
				{
					PvmInstanceID: pstring("abcd-test-machine"),
				},
			}
			body := &models.PVMInstanceCreate{
				ServerName:  &scope.IBMPowerVSMachine.Name,
				Memory:      pointer.Float64(8),
				Processors:  pointer.Float64(0.25),
				ImageID:     pstring("test-image-ID"),
				KeyPairName: "dummy-key",
			}

			mockpowervs.EXPECT().GetAllInstance().Return(instances, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(body)).Return(instanceList, nil)

			_, err = scope.CreateMachine()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error with instance not getting created as bootstrap data is not available", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "", pstring("test-image-ID"), pstring("test-net-ID"), true)
			Expect(err).NotTo(HaveOccurred())

			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockpowervs.EXPECT().GetAllInstance().Return(instances, nil)

			_, err = scope.CreateMachine()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Delete IBMPowerVS machine", func() {
		var instance *models.PVMInstance

		BeforeEach(func() {
			instance = &models.PVMInstance{
				PvmInstanceID: pstring("abcd-test-machine"),
			}
		})

		It("should not error and delete the machine", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-ID"), pstring("test-net-ID"), true)
			Expect(err).NotTo(HaveOccurred())

			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(*instance.PvmInstanceID)).Return(nil)
			err = scope.DeleteMachine()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error with machine not being deleted", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-ID"), pstring("test-net-ID"), true)
			Expect(err).NotTo(HaveOccurred())

			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(*instance.PvmInstanceID)).Return(errors.New("Could not delete the macine"))
			err = scope.DeleteMachine()
			Expect(err).To(HaveOccurred())

		})
	})

	Context("Get image ID or network ID", func() {
		It("should not error and get imageID from name", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-name"), pstring("test-net-name"), false)
			Expect(err).NotTo(HaveOccurred())

			images := &models.Images{
				Images: []*models.ImageReference{
					{
						ImageID: pstring("test-image-ID"),
						Name:    pstring("test-image-name"),
					},
				},
			}
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)

			mspec := scope.IBMPowerVSMachine.Spec
			_, err = getImageID(mspec.Image, scope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error and not find the corresponding imageID", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-name"), pstring("test-net-name"), false)
			Expect(err).NotTo(HaveOccurred())

			images := &models.Images{
				Images: []*models.ImageReference{
					{
						ImageID: pstring("test-diff-image-ID"),
						Name:    pstring("test-diff-image-name"),
					},
				},
			}
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)

			mspec := scope.IBMPowerVSMachine.Spec
			_, err = getImageID(mspec.Image, scope)
			Expect(err).To(HaveOccurred())
		})

		It("should not error and get networkID from name", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-name"), pstring("test-net-name"), false)
			Expect(err).NotTo(HaveOccurred())

			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						NetworkID: pstring("test-net-ID"),
						Name:      pstring("test-net-name"),
					},
				},
			}
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)

			mspec := scope.IBMPowerVSMachine.Spec
			_, err = getNetworkID(mspec.Network, scope)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error and not find the corresponding networkID", func() {
			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-name"), pstring("test-net-name"), false)
			Expect(err).NotTo(HaveOccurred())

			networks := &models.Networks{
				Networks: []*models.NetworkReference{
					{
						NetworkID: pstring("test-diff-net-ID"),
						Name:      pstring("test-diff-net-name"),
					},
				},
			}
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)

			mspec := scope.IBMPowerVSMachine.Spec
			_, err = getNetworkID(mspec.Network, scope)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Get Bootstrap Data", func() {
		It("should not error and get base64 encoded bootstrap data", func() {

			scope, err := setupPowerVSMachineScope("test-cluster", "test-machine-0", pstring("test-image-ID"), pstring("test-net-ID"), true)
			Expect(err).NotTo(HaveOccurred())

			result, err := scope.GetBootstrapData()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			_, err = base64.StdEncoding.DecodeString(result)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Create or Delete IBMPowerVSImage", func() {
		It("should not error and create an image import job", func() {
			scope, err := setupPowerVSImageScope("sample-image")
			Expect(err).NotTo(HaveOccurred())
			spec := scope.IBMPowerVSImage.Spec

			images := &models.Images{}
			body := &models.CreateCosImageImportJob{
				ImageName:     &scope.IBMPowerVSImage.ObjectMeta.Name,
				BucketName:    spec.Bucket,
				BucketAccess:  pstring("public"),
				Region:        spec.Region,
				ImageFilename: spec.Object,
			}

			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetCosImages(scope.IBMPowerVSImage.Spec.ServiceInstanceID).Return(nil, nil)
			mockpowervs.EXPECT().CreateCosImage(body).Return(&models.JobReference{ID: pstring("test-job-ID")}, nil)

			_, jobRef, err := scope.CreateImageCOSBucket()
			Expect(err).NotTo(HaveOccurred())
			Expect(jobRef).ToNot(BeNil())
		})

		It("should not error and use the existing image", func() {
			scope, err := setupPowerVSImageScope("sample-image")
			Expect(err).NotTo(HaveOccurred())

			images := &models.Images{
				Images: []*models.ImageReference{
					{
						ImageID: pstring("sample-image-ID"),
						Name:    pstring("sample-image"),
					},
				},
			}
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)

			imageRef, _, err := scope.CreateImageCOSBucket()
			Expect(err).NotTo(HaveOccurred())
			Expect(imageRef).ToNot(BeNil())
		})

		It("should return as the previous job is not finished", func() {
			scope, err := setupPowerVSImageScope("sample-image")
			Expect(err).NotTo(HaveOccurred())

			images := &models.Images{}
			job := &models.Job{ID: pstring("test-job-ID"), Status: &models.Status{State: pstring("pending")}}
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetCosImages(scope.IBMPowerVSImage.Spec.ServiceInstanceID).Return(job, nil)

			_, _, err = scope.CreateImageCOSBucket()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not error and delete the image", func() {
			scope, err := setupPowerVSImageScope("sample-image")
			Expect(err).NotTo(HaveOccurred())

			mockpowervs.EXPECT().DeleteImage(gomock.AssignableToTypeOf("sample-image-ID")).Return(nil)

			err = scope.DeleteImage()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
