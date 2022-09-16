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
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func newVPCMachine(clusterName, machineName string) *infrav1beta1.IBMVPCMachine {
	return &infrav1beta1.IBMVPCMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiv1beta1.ClusterLabelName: clusterName,
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
		Logger:        klogr.New(),
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
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewMachineScope(tc.params)
			// Note: only error/failure cases covered
			// TO-DO: cover success cases
			g.Expect(err).To(Not(BeNil()))
		})
	}
}

func TestCreateMachine(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	vpcMachine := infrav1beta1.IBMVPCMachine{
		Spec: infrav1beta1.IBMVPCMachineSpec{
			SSHKeys: []*string{
				core.StringPtr("foo-ssh-key"),
			},
		},
	}
	expectedOutput := &vpcv1.Instance{
		Name: core.StringPtr("foo-machine"),
	}

	t.Run("Create Machine", func(t *testing.T) {
		listInstancesOptions := &vpcv1.ListInstancesOptions{}
		detailedResponse := &core.DetailedResponse{}
		instanceCollection := &vpcv1.InstanceCollection{
			Instances: []vpcv1.Instance{
				{
					Name: core.StringPtr("foo-machine-1"),
				},
			},
		}

		createInstanceOptions := &vpcv1.CreateInstanceOptions{}

		t.Run("Should create Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Spec = vpcMachine.Spec
			instance := &vpcv1.Instance{
				Name: &scope.Machine.Name,
			}
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(createInstanceOptions)).Return(instance, detailedResponse, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Return exsisting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			expectedOutput = &vpcv1.Instance{
				Name: core.StringPtr("foo-machine-1"),
			}
			scope := setupMachineScope(clusterName, "foo-machine-1", mockvpc)
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			out, err := scope.CreateMachine()
			g.Expect(err).To(BeNil())
			require.Equal(t, expectedOutput, out)
		})

		t.Run("Error when listing Instances", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, errors.New("Error when listing instances"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Error when DataSecretName is nil", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.Machine.Spec.Bootstrap.DataSecretName = nil
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("failed to retrieve bootstrap data secret for IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.Machine.Spec.Bootstrap.DataSecretName = core.StringPtr("foo-secret-temp")
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to retrieve bootstrap data, secret value key is missing", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capiv1beta1.ClusterLabelName: clusterName,
					},
					Name:      machineName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"val": []byte("user data"),
				}}
			g.Expect(scope.Client.Update(context.Background(), secret)).To(Succeed())
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Failed to create instance", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(listInstancesOptions)).Return(instanceCollection, detailedResponse, nil)
			mockvpc.EXPECT().CreateInstance(gomock.AssignableToTypeOf(createInstanceOptions)).Return(nil, detailedResponse, errors.New("Failed when creating instance"))
			_, err := scope.CreateMachine()
			g.Expect(err).To(Not(BeNil()))
		})
	})
}

func TestDeleteMachine(t *testing.T) {
	var (
		mockvpc  *mock.MockVpc
		mockCtrl *gomock.Controller
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Delete Machine", func(t *testing.T) {
		deleteInstanceOptions := &vpcv1.DeleteInstanceOptions{}
		detailedResponse := &core.DetailedResponse{}

		t.Run("Should delete Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Status.InstanceID = "foo-instance-id"
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(deleteInstanceOptions)).Return(detailedResponse, nil)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})

		t.Run("Error when deleting Machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			scope.IBMVPCMachine.Status.InstanceID = "foo-instance-id"
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(deleteInstanceOptions)).Return(detailedResponse, errors.New("Failed instance deletion"))
			err := scope.DeleteMachine()
			g.Expect(err).To(Not(BeNil()))
		})

		t.Run("Empty InstanceID", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			scope := setupMachineScope(clusterName, machineName, mockvpc)
			err := scope.DeleteMachine()
			g.Expect(err).To(BeNil())
		})
	})
}
