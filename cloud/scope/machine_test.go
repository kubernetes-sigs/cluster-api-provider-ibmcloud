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
	"context"
	"errors"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func newVPCMachine(clusterName, machineName string) *infrav1beta2.IBMVPCMachine {
	return &infrav1beta2.IBMVPCMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiv1beta1.ClusterNameLabel: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
	}
}

func setupMachineScope(clusterName string, machineName string, mockvpc *mock.MockVpc) *MachineScope {
	cluster := newCluster(clusterName)
	machine := newMachine(machineName)
	secret := newBootstrapSecret(clusterName, machineName)
	vpcMachine := newVPCMachine(clusterName, machineName)
	vpcCluster := newVPCCluster(clusterName)

	initObjects := []client.Object{
		cluster, machine, secret, vpcCluster, vpcMachine,
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &MachineScope{
		Client:        client,
		Logger:        klog.Background(),
		IBMVPCClient:  mockvpc,
		Cluster:       cluster,
		Machine:       machine,
		IBMVPCCluster: vpcCluster,
		IBMVPCMachine: vpcMachine,
	}
}

func TestNewMachineScope(t *testing.T) {
	testCases := []struct {
		name   string
		params MachineScopeParams
	}{
		{
			name: "Error when Machine in nil",
			params: MachineScopeParams{
				Machine: nil,
			},
		},
		{
			name: "Error when IBMVPCMachine in nil",
			params: MachineScopeParams{
				Machine:       newMachine(machineName),
				IBMVPCMachine: nil,
			},
		},
		{
			name: "Failed to create IBM VPC session",
			params: MachineScopeParams{
				Machine:       newMachine(machineName),
				IBMVPCMachine: newVPCMachine(clusterName, machineName),
				IBMVPCCluster: newVPCCluster(clusterName),
				Client:        testEnv.Client,
			},
		},
	}
	for _, tc := range testCases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			_, err := NewMachineScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateMachine(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcMachine := infrav1beta2.IBMVPCMachine{
		Spec: infrav1beta2.IBMVPCMachineSpec{
			SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
				{
					ID: core.StringPtr("foo-ssh-key-id"),
				},
			},
			Image: &infrav1beta2.IBMVPCResourceReference{
				ID: core.StringPtr("foo-image-id"),
			},
		},
	}

	t.Run("Create Machine", func(t *testing.T) {
		t.Run("Should create Machine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			expectedOutput := &vpcv1.Instance{
				Name: core.StringPtr("foo-machine"),
			}
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			instance := &vpcv1.Instance{
				Name: &scope.Machine.Name,
			}
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(&vpcv1.CreateInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return existing Machine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			expectedOutput := &vpcv1.Instance{
				Name: core.StringPtr("foo-machine-1"),
			}
			scope := setupMachineScope(clusterName, "foo-machine-1", mockvpc)
			instanceCollection := &vpcv1.InstanceCollection{
				Instances: []vpcv1.Instance{
					{
						Name: core.StringPtr("foo-machine-1"),
					},
				},
			}
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instanceCollection, &core.DetailedResponse{}, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing Instances", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, errors.New("Error when listing instances"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.Machine.Spec.Bootstrap.DataSecretName = core.StringPtr("foo-secret-temp")
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to retrieve bootstrap data, secret value key is missing", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capiv1beta1.ClusterNameLabel: clusterName,
					},
					Name:      machineName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"val": []byte("user data"),
				}}
			g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to create instance", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(&vpcv1.CreateInstanceOptions{})).Return(nil, &core.DetailedResponse{}, errors.New("Failed when creating instance"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})
	})

	t.Run("Error when both SSHKeys ID and Name are nil", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
					{},
				},
				Image: &infrav1beta2.IBMVPCResourceReference{
					ID: core.StringPtr("foo-image-id"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Error when listing SSHKeys", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
					{
						Name: core.StringPtr("foo-ssh-key"),
					},
				},
				Image: &infrav1beta2.IBMVPCResourceReference{
					ID: core.StringPtr("foo-image-id"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListKeys(gomock.AssignableToTypeOf(&vpcv1.ListKeysOptions{})).Return(nil, &core.DetailedResponse{}, errors.New("Failed when creating instance"))
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Error when SSHKey does not exist", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		keyCollection := &vpcv1.KeyCollection{
			Keys: []vpcv1.Key{
				{
					Name: core.StringPtr("foo-ssh-key-1"),
					ID:   core.StringPtr("foo-ssh-key-id"),
				},
			},
		}
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
					{
						Name: core.StringPtr("foo-ssh-key"),
					},
				},
				Image: &infrav1beta2.IBMVPCResourceReference{
					ID: core.StringPtr("foo-image-id"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListKeys(gomock.AssignableToTypeOf(&vpcv1.ListKeysOptions{})).Return(keyCollection, &core.DetailedResponse{}, nil)
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Should create Machine with SSHKeys and Image (Name)", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		expectedOutput := &vpcv1.Instance{
			Name: core.StringPtr("foo-machine"),
		}
		keyCollection := &vpcv1.KeyCollection{
			Keys: []vpcv1.Key{
				{
					Name: core.StringPtr("foo-ssh-key"),
					ID:   core.StringPtr("foo-ssh-key-id"),
				},
			},
		}
		imageCollection := &vpcv1.ImageCollection{
			Images: []vpcv1.Image{
				{
					Name: core.StringPtr("foo-image"),
					ID:   core.StringPtr("foo-image-id"),
				},
			},
		}
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
					{
						Name: core.StringPtr("foo-ssh-key"),
					},
				},
				Image: &infrav1beta2.IBMVPCResourceReference{
					Name: core.StringPtr("foo-image"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		instance := &vpcv1.Instance{
			Name: &scope.Machine.Name,
		}
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListImages(gomock.AssignableToTypeOf(&vpcv1.ListImagesOptions{})).Return(imageCollection, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListKeys(gomock.AssignableToTypeOf(&vpcv1.ListKeysOptions{})).Return(keyCollection, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(&vpcv1.CreateInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
		out, err := scope.CreateMachine()
		g.Expect(err).To(BeNil())
		require.Equal(t, expectedOutput, out)
	})

	t.Run("Error when both Image ID and Name are nil", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				Image: &infrav1beta2.IBMVPCResourceReference{},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Error when listing Images", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				Image: &infrav1beta2.IBMVPCResourceReference{
					Name: core.StringPtr("foo-image"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListImages(gomock.AssignableToTypeOf(&vpcv1.ListImagesOptions{})).Return(nil, &core.DetailedResponse{}, errors.New("Failed when listing Images"))
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Error when Image does not exist", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		imageCollection := &vpcv1.ImageCollection{
			Images: []vpcv1.Image{
				{
					Name: core.StringPtr("foo-image-1"),
					ID:   core.StringPtr("foo-image-id"),
				},
			},
		}
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				Image: &infrav1beta2.IBMVPCResourceReference{
					Name: core.StringPtr("foo-image"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListImages(gomock.AssignableToTypeOf(&vpcv1.ListImagesOptions{})).Return(imageCollection, &core.DetailedResponse{}, nil)
		_, err := scope.CreateMachine()
		g.Expect(err).To(Not(BeNil()))
	})

	t.Run("Should create machine when both Image/SSHKey ID and Name are defined with ID taking higher precedence", func(t *testing.T) {
		g := NewWithT(t)
		mockController, mockvpc := setup(t)
		t.Cleanup(mockController.Finish)
		scope := setupMachineScope(clusterName, machineName, mockvpc)
		expectedOutput := &vpcv1.Instance{
			Name: core.StringPtr("foo-machine"),
		}
		vpcMachine := infrav1beta2.IBMVPCMachine{
			Spec: infrav1beta2.IBMVPCMachineSpec{
				SSHKeys: []*infrav1beta2.IBMVPCResourceReference{
					{
						Name: core.StringPtr("foo-ssh-key"),
						ID:   core.StringPtr("foo-ssh-key-id"),
					},
				},
				Image: &infrav1beta2.IBMVPCResourceReference{
					Name: core.StringPtr("foo-image"),
					ID:   core.StringPtr("foo-image-id"),
				},
			},
		}
		scope.IBMVPCMachine.Spec = vpcMachine.Spec
		instance := &vpcv1.Instance{
			Name: &scope.Machine.Name,
		}
		mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(&vpcv1.InstanceCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(&vpcv1.CreateInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
		out, err := scope.CreateMachine()
		g.Expect(err).To(BeNil())
		require.Equal(t, expectedOutput, out)
	})
}

func TestDeleteMachine(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcMachine := infrav1beta2.IBMVPCMachine{
		Spec: infrav1beta2.IBMVPCMachineSpec{
			Name: "foo-machine",
		},
		Status: infrav1beta2.IBMVPCMachineStatus{
			InstanceID: "foo-instance-id",
		},
	}

	t.Run("Delete Machine", func(t *testing.T) {
		t.Run("Should delete Machine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(&vpcv1.DeleteInstanceOptions{})).Return(&core.DetailedResponse{}, nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(&vpcv1.DeleteInstanceOptions{})).Return(&core.DetailedResponse{}, errors.New("Failed instance deletion"))
			err := scope.DeleteMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Empty InstanceID", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Status.InstanceID = ""
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})
	})
}

func TestCreateVPCLoadBalancerPoolMember(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcMachine := infrav1beta2.IBMVPCMachine{
		Spec: infrav1beta2.IBMVPCMachineSpec{
			Name: "foo-machine",
		},
		Status: infrav1beta2.IBMVPCMachineStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.1.1",
				},
			},
		},
	}

	t.Run("Create VPCLoadBalancerPoolMember", func(t *testing.T) {
		loadBalancer := &vpcv1.LoadBalancer{
			ID:                 core.StringPtr("foo-load-balancer-id"),
			ProvisioningStatus: core.StringPtr("active"),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID: core.StringPtr("foo-load-balancer-pool-id"),
				},
			},
		}

		t.Run("Error when fetching LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(&vpcv1.LoadBalancer{}, &core.DetailedResponse{}, errors.New("Could not fetch LoadBalancer"))
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when LoadBalancer is not active", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			loadBalancer := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr("foo-load-balancer-id"),
				ProvisioningStatus: core.StringPtr("pending"),
			}
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when no pool exist", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			loadBalancer := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr("foo-load-balancer-id"),
				ProvisioningStatus: core.StringPtr("active"),
				Pools:              []vpcv1.LoadBalancerPoolReference{},
			}
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when listing LoadBalancerPoolMembers", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, errors.New("Failed to list LoadBalancerPoolMembers"))
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("PoolMember already exist", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			loadBalancerPoolMemberCollection := &vpcv1.LoadBalancerPoolMemberCollection{
				Members: []vpcv1.LoadBalancerPoolMember{
					{
						Port: core.Int64Ptr(int64(infrav1beta2.DefaultAPIServerPort)),
						Target: &vpcv1.LoadBalancerPoolMemberTarget{
							Address: core.StringPtr("192.168.1.1"),
						},
					},
				},
			}
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, &core.DetailedResponse{}, nil)
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(BeNil())
		})
		t.Run("Error when creating LoadBalancerPoolMember", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(&vpcv1.LoadBalancerPoolMember{}, &core.DetailedResponse{}, errors.New("Failed to create LoadBalancerPoolMember"))
			_, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(64))
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Should create VPCLoadBalancerPoolMember", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			expectedOutput := &vpcv1.LoadBalancerPoolMember{
				ID:   core.StringPtr("foo-load-balancer-pool-member-id"),
				Port: core.Int64Ptr(int64(infrav1beta2.DefaultAPIServerPort)),
			}
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			loadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{
				ID:   core.StringPtr("foo-load-balancer-pool-member-id"),
				Port: core.Int64Ptr(int64(infrav1beta2.DefaultAPIServerPort)),
			}
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(loadBalancerPoolMember, &core.DetailedResponse{}, nil)
			out, err := scope.CreateVPCLoadBalancerPoolMember(&scope.IBMVPCMachine.Status.Addresses[0].Address, int64(infrav1beta2.DefaultAPIServerPort))
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})
	})
}

func TestDeleteVPCLoadBalancerPoolMember(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc) {
		t.Helper()
		return gomock.NewController(t), mock.NewMockVpc(gomock.NewController(t))
	}

	vpcMachine := infrav1beta2.IBMVPCMachine{
		Spec: infrav1beta2.IBMVPCMachineSpec{
			Name: "foo-machine",
		},
		Status: infrav1beta2.IBMVPCMachineStatus{
			InstanceID: "foo-instance-id",
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.1.1",
				},
			},
		},
	}

	t.Run("Delete VPCLoadBalancerPoolMember", func(t *testing.T) {
		loadBalancer := &vpcv1.LoadBalancer{
			ID:                 core.StringPtr("foo-load-balancer-id"),
			ProvisioningStatus: core.StringPtr("active"),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID: core.StringPtr("foo-load-balancer-pool-id"),
				},
			},
		}
		instance := &vpcv1.Instance{
			PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
				PrimaryIP: &vpcv1.ReservedIPReference{
					Address: core.StringPtr("192.168.1.1"),
				},
			},
		}
		loadBalancerPoolMemberCollection := &vpcv1.LoadBalancerPoolMemberCollection{
			Members: []vpcv1.LoadBalancerPoolMember{
				{
					ID:   core.StringPtr("foo-lb-pool-member-id"),
					Port: core.Int64Ptr(int64(infrav1beta2.DefaultAPIServerPort)),
					Target: &vpcv1.LoadBalancerPoolMemberTarget{
						Address: core.StringPtr("192.168.1.1"),
					},
				},
			},
		}

		t.Run("Error when fetching LoadBalancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(&vpcv1.LoadBalancer{}, &core.DetailedResponse{}, errors.New("Could not fetch LoadBalancer"))
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("No pools associated with load balancer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(&vpcv1.LoadBalancer{}, &core.DetailedResponse{}, nil)
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(BeNil())
		})
		t.Run("Error when fetching Instance", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(&vpcv1.Instance{}, &core.DetailedResponse{}, errors.New("Failed to fetch Instance"))
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when listing LoadBalancerPoolMembers", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(&vpcv1.Instance{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, errors.New("Failed to list LoadBalancerPoolMembers"))
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("No members in load balancer pool", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(&vpcv1.Instance{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(BeNil())
		})
		t.Run("Error when load balancer is not in active state", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			loadBalancer := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr("foo-load-balancer-id"),
				ProvisioningStatus: core.StringPtr("pending"),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID: core.StringPtr("foo-load-balancer-pool-id"),
					},
				},
			}
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, &core.DetailedResponse{}, nil)
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Error when deleting load balancer pool member", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.DeleteLoadBalancerPoolMemberOptions{})).Return(&core.DetailedResponse{}, errors.New("Failed to delete LoadBalancerPoolMember"))
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(Not(BeNil()))
		})
		t.Run("Should delete load balancer pool", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc := setup(t)
			t.Cleanup(mockController.Finish)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			scope.IBMVPCMachine.Status = vpcMachine.Status
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(instance, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.DeleteLoadBalancerPoolMemberOptions{})).Return(&core.DetailedResponse{}, nil)
			err := scope.DeleteVPCLoadBalancerPoolMember()
			g.Expect(err).To(BeNil())
		})
	})
}
