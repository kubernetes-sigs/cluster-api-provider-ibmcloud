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

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMVPCMachineReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name         string
		vpcMachine   *infrav1beta1.IBMVPCMachine
		ownerMachine *capiv1beta1.Machine
		vpcCluster   *infrav1beta1.IBMVPCCluster
		ownerCluster *capiv1beta1.Cluster
		expectError  bool
	}{
		{
			name:        "Should Reconcile successfully if no IBMVPCMachine found",
			expectError: false,
		},
		{
			name: "Should Reconcile if Owner Reference is not set",
			vpcMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-1"},
			},
			expectError: false,
		},
		{
			name: "Should fail Reconcile if no OwnerMachine found",
			vpcMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "Should not Reconcile if machine does not contain cluster label",
			vpcMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-3", OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				}},
			ownerMachine: &capiv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-machine"}},
			ownerCluster: &capiv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-1"}},
			expectError: false,
		},
		{
			name: "Should not Reconcile if IBMVPCCluster is not found",
			vpcMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-4", Labels: map[string]string{
						capiv1beta1.ClusterNameAnnotation: "capi-test-2"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test-2",
							UID:        "1",
						},
					},
				}},
			ownerMachine: &capiv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-machine"}},
			ownerCluster: &capiv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-2"},
				Spec: capiv1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "vpc-cluster"}}},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMVPCMachineReconciler{
				Client: testEnv.Client,
				Log:    klogr.New(),
			}
			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())
			defer func() {
				g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
			}()

			createObject(g, tc.ownerCluster, ns.Name)
			defer cleanupObject(g, tc.ownerCluster)

			createObject(g, tc.vpcCluster, ns.Name)
			defer cleanupObject(g, tc.vpcCluster)

			createObject(g, tc.ownerMachine, ns.Name)
			defer cleanupObject(g, tc.ownerMachine)

			createObject(g, tc.vpcMachine, ns.Name)
			defer cleanupObject(g, tc.vpcMachine)

			if tc.vpcMachine != nil {
				g.Eventually(func() bool {
					machine := &infrav1beta1.IBMVPCMachine{}
					key := client.ObjectKey{
						Name:      tc.vpcMachine.Name,
						Namespace: ns.Name,
					}
					err = testEnv.Get(ctx, key, machine)
					return err == nil
				}, 10*time.Second).Should(Equal(true))

				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: tc.vpcMachine.Namespace,
						Name:      tc.vpcMachine.Name,
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

func TestIBMVPCMachineReconciler_reconcile(t *testing.T) {
	var (
		mockvpc      *mock.MockVpc
		mockCtrl     *gomock.Controller
		machineScope *scope.MachineScope
		reconciler   IBMVPCMachineReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klogr.New(),
		}
		machineScope = &scope.MachineScope{
			Logger: klogr.New(),
			IBMVPCMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-machine",
					Labels: map[string]string{
						capiv1beta1.MachineControlPlaneLabelName: "capi-control-plane-machine",
					},
				},
			},
			Machine: &capiv1beta1.Machine{
				Spec: capiv1beta1.MachineSpec{
					ClusterName: "vpc-cluster",
				},
			},
			IBMVPCCluster: &infrav1beta1.IBMVPCCluster{},
			IBMVPCClient:  mockvpc,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Reconcile creating IBMVPCMachine", func(t *testing.T) {
		t.Run("Should fail to find bootstrap data secret reference", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta1.MachineFinalizer))
		})
		options := &vpcv1.ListInstancesOptions{}
		response := &core.DetailedResponse{}
		instancelist := &vpcv1.InstanceCollection{}
		t.Run("Should fail reconcile IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope.Machine.Spec.Bootstrap.DataSecretName = pointer.String("capi-machine")
			machineScope.IBMVPCCluster.Status.Subnet.ID = pointer.String("capi-subnet-id")
			mockvpc.EXPECT().ListInstances(options).Return(instancelist, response, errors.New("Failed to create or fetch instance"))
			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta1.MachineFinalizer))
		})
		instancelist.Instances = []vpcv1.Instance{
			{
				Name: pointer.String("capi-machine"),
				ID:   pointer.String("capi-machine-id"),
				PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
					PrimaryIP: &vpcv1.ReservedIPReference{
						Address: pointer.String("192.129.11.50"),
					},
					ID: pointer.String("capi-net"),
				},
			}}
		fipoptions := &vpcv1.AddInstanceNetworkInterfaceFloatingIPOptions{}
		fip := &vpcv1.FloatingIP{}
		t.Run("Should fail to bind floating IP to control plane", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope.Machine.Spec.Bootstrap.DataSecretName = pointer.String("capi-machine")
			machineScope.IBMVPCCluster.Status.Subnet.ID = pointer.String("capi-subnet-id")
			machineScope.IBMVPCCluster.Status.VPCEndpoint.FIPID = pointer.String("capi-fip-id")
			mockvpc.EXPECT().ListInstances(options).Return(instancelist, response, nil)
			mockvpc.EXPECT().AddInstanceNetworkInterfaceFloatingIP(gomock.AssignableToTypeOf(fipoptions)).Return(fip, response, errors.New("Failed to bind floating IP to control plane"))
			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta1.MachineFinalizer))
		})
		t.Run("Should successfully reconcile IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope.Machine.Spec.Bootstrap.DataSecretName = pointer.String("capi-machine")
			machineScope.IBMVPCCluster.Status.Subnet.ID = pointer.String("capi-subnet-id")
			machineScope.IBMVPCCluster.Status.VPCEndpoint.FIPID = pointer.String("capi-fip-id")
			fip.Address = pointer.String("192.129.11.52")
			mockvpc.EXPECT().ListInstances(options).Return(instancelist, response, nil)
			mockvpc.EXPECT().AddInstanceNetworkInterfaceFloatingIP(gomock.AssignableToTypeOf(fipoptions)).Return(fip, response, nil)
			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta1.MachineFinalizer))
			g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(true))
		})
	})
}
func TestIBMVPCMachineReconciler_Delete(t *testing.T) {
	var (
		mockvpc      *mock.MockVpc
		mockCtrl     *gomock.Controller
		machineScope *scope.MachineScope
		reconciler   IBMVPCMachineReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klogr.New(),
		}
		machineScope = &scope.MachineScope{
			Logger: klogr.New(),
			IBMVPCMachine: &infrav1beta1.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "capi-machine",
					Finalizers: []string{infrav1beta1.MachineFinalizer},
				},
				Status: infrav1beta1.IBMVPCMachineStatus{
					InstanceID: "capi-machine-id",
				},
			},
			IBMVPCClient: mockvpc,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	options := &vpcv1.DeleteInstanceOptions{ID: pointer.String("capi-instance-id")}
	t.Run("Reconciling deleting IBMVPCMachine", func(t *testing.T) {
		t.Run("Should fail to delete VPC machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(options)).Return(nil, errors.New("Failed to delete the VPC instance"))
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta1.MachineFinalizer))
		})
		t.Run("Should successfully delete VPC machine and remove the finalizer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			response := &core.DetailedResponse{}
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(options)).Return(response, nil)
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(Not(ContainElement(infrav1beta1.MachineFinalizer)))
		})
	})
}
