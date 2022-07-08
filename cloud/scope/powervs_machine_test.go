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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newPowerVSMachine(clusterName, machineName string, imageRef *string, networkRef *string, isID bool) *infrav1beta1.IBMPowerVSMachine {
	image := &infrav1beta1.IBMPowerVSResourceReference{}
	network := infrav1beta1.IBMPowerVSResourceReference{}

	if !isID {
		image.Name = imageRef
		network.Name = networkRef
	} else {
		image.ID = imageRef
		network.ID = networkRef
	}

	return &infrav1beta1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiv1beta1.ClusterLabelName: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Spec: infrav1beta1.IBMPowerVSMachineSpec{
			Memory:     "8",
			Processors: "0.25",
			Image:      image,
			Network:    network,
		},
	}
}

func setupPowerVSMachineScope(clusterName string, machineName string, imageID *string, networkID *string, isID bool, mockpowervs *mock.MockPowerVS) *PowerVSMachineScope {
	cluster := newCluster(clusterName)
	machine := newMachine(machineName)
	secret := newBootstrapSecret(clusterName, machineName)
	powervsMachine := newPowerVSMachine(clusterName, machineName, imageID, networkID, isID)
	powervsCluster := newPowerVSCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, powervsCluster, powervsMachine,
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &PowerVSMachineScope{
		Client:            client,
		Logger:            klogr.New(),
		IBMPowerVSClient:  mockpowervs,
		Cluster:           cluster,
		Machine:           machine,
		IBMPowerVSCluster: powervsCluster,
		IBMPowerVSMachine: powervsMachine,
	}
}

func TestNewPowerVSMachineScope(t *testing.T) {
	testCases := []struct {
		name   string
		params PowerVSMachineScopeParams
	}{
		{
			name: "Error when Client in nil",
			params: PowerVSMachineScopeParams{
				Client: nil,
			},
		},
		{
			name: "Error when Machine in nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: nil,
			},
		},
		{
			name: "Error when Cluster is nil",
			params: PowerVSMachineScopeParams{
				Client:  testEnv.Client,
				Machine: newMachine(machineName),
				Cluster: nil,
			},
		},
		{
			name: "Error when IBMPowerVSMachine is nil",
			params: PowerVSMachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: nil,
			},
		},
		{
			name: "Failed to get authenticator",
			params: PowerVSMachineScopeParams{
				Client:            testEnv.Client,
				Machine:           newMachine(machineName),
				Cluster:           newCluster(clusterName),
				IBMPowerVSMachine: newPowerVSMachine(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true),
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPowerVSMachineScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateMachinePVS(t *testing.T) {
	mockctrl := gomock.NewController(GinkgoT())
	mockpowervs := mock.NewMockPowerVS(mockctrl)
	g := NewWithT(t)

	t.Run("Create Machine", func(t *testing.T) {
		pvmInstances := &models.PVMInstances{
			PvmInstances: []*models.PVMInstanceReference{
				{
					ServerName: core.StringPtr("foo-machine-1"),
				},
			},
		}
		images := &models.Images{
			Images: []*models.ImageReference{
				{
					Name:    core.StringPtr(pvsImage),
					ImageID: core.StringPtr(pvsImage + "-id"),
				},
			},
		}
		networks := &models.Networks{
			Networks: []*models.NetworkReference{
				{
					Name:      core.StringPtr(pvsNetwork),
					NetworkID: core.StringPtr(pvsNetwork + "-id"),
				},
			},
		}
		pvmInstanceList := &models.PVMInstanceList{}
		pvmInstanceCreate := &models.PVMInstanceCreate{}

		t.Run("Should create Machine", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capiv1beta1.ClusterLabelName: scope.Cluster.Name,
					},
					Name:      scope.Machine.Name,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"value": []byte("user data"),
				},
			}
			createObject(g, secret, "default")
			defer cleanupObject(g, secret)

			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Return exsisting Machine", func(t *testing.T) {
			expectedOutput := models.PVMInstanceReference{
				ServerName: core.StringPtr("foo-machine-1"),
			}

			scope := setupPowerVSMachineScope(clusterName, *core.StringPtr("foo-machine-1"), core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput.ServerName, out.ServerName)
		})

		t.Run("Eror while getting instances", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, errors.New("Error when getting list of instances"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.Machine.Spec.Bootstrap.DataSecretName = core.StringPtr("foo-secret-temp")
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Invalid memory value", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Spec.Memory = "illegal"
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Invalid core value", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSMachine.Spec.Processors = "illegal"
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("IBMPowerVSImage is not nil", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, core.StringPtr(pvsNetwork), true, mockpowervs)
			scope.IBMPowerVSImage = &infrav1beta1.IBMPowerVSImage{
				Status: infrav1beta1.IBMPowerVSImageStatus{
					ImageID: "foo-image",
				},
			}
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((BeNil()))
		})

		t.Run("Image and Network name is set", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((BeNil()))
		})

		t.Run("Error when both Image id and name are nil", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, nil, core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when both Network id and name are nil", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), nil, true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when Image id does not exsist", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage+"-temp"), core.StringPtr(pvsNetwork), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error when Network id does not exsist", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork+"-temp"), false, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})

		t.Run("Error while creating machine", func(t *testing.T) {
			scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
			mockpowervs.EXPECT().GetAllInstance().Return(pvmInstances, nil)
			mockpowervs.EXPECT().GetAllImage().Return(images, nil)
			mockpowervs.EXPECT().GetAllNetwork().Return(networks, nil)
			mockpowervs.EXPECT().CreateInstance(gomock.AssignableToTypeOf(pvmInstanceCreate)).Return(pvmInstanceList, errors.New("Failed to create machine"))
			_, err := scope.CreateMachine()
			g.Expect(err).To((Not(BeNil())))
		})
	})
}

func TestDeleteMachinePVS(t *testing.T) {
	mockctrl := gomock.NewController(GinkgoT())
	mockpowervs := mock.NewMockPowerVS(mockctrl)
	g := NewWithT(t)

	t.Run("Delete Machine", func(t *testing.T) {
		scope := setupPowerVSMachineScope(clusterName, machineName, core.StringPtr(pvsImage), core.StringPtr(pvsNetwork), true, mockpowervs)
		scope.IBMPowerVSMachine.Status.InstanceID = machineName + "-id"
		var id string

		t.Run("Should delete Machine", func(t *testing.T) {
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error while deleting Machine", func(t *testing.T) {
			mockpowervs.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(id)).Return(errors.New("Failed to delete machine"))
			err := scope.DeleteMachine()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}
