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

	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	mockVPC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	. "github.com/onsi/gomega"
)

func TestIBMPowerVSMachineReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name           string
		powervsMachine *infrav1beta2.IBMPowerVSMachine
		ownerMachine   *capiv1beta1.Machine
		powervsCluster *infrav1beta2.IBMPowerVSCluster
		ownerCluster   *capiv1beta1.Cluster
		expectError    bool
	}{
		{
			name:        "Should Reconcile successfully if no IBMPowerVSMachine found",
			expectError: false,
		},
		{
			name: "Should Reconcile if Owner Reference is not set",
			powervsMachine: &infrav1beta2.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-1"},
				Spec: infrav1beta2.IBMPowerVSMachineSpec{
					ServiceInstanceID: "service-instance-1",
					Image:             &infrav1beta2.IBMPowerVSResourceReference{}}},
			expectError: false,
		},
		{
			name: "Should fail Reconcile if no OwnerMachine found",
			powervsMachine: &infrav1beta2.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				},
				Spec: infrav1beta2.IBMPowerVSMachineSpec{
					ServiceInstanceID: "service-instance-1",
					Image:             &infrav1beta2.IBMPowerVSResourceReference{}},
			},
			expectError: true,
		},
		{
			name: "Should not Reconcile if machine does not contain cluster label",
			powervsMachine: &infrav1beta2.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-3", OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				}, Spec: infrav1beta2.IBMPowerVSMachineSpec{
					ServiceInstanceID: "service-instance-1",
					Image:             &infrav1beta2.IBMPowerVSResourceReference{}},
			},
			ownerMachine: &capiv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &capiv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-1"}},
			expectError: false,
		},
		{
			name: "Should not Reconcile if IBMPowerVSCluster is not found",
			powervsMachine: &infrav1beta2.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "powervs-test-4",
					Labels: map[string]string{capiv1beta1.ClusterNameAnnotation: "capi-test-2"},
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
				}, Spec: infrav1beta2.IBMPowerVSMachineSpec{
					ServiceInstanceID: "service-instance-1",
					Image:             &infrav1beta2.IBMPowerVSResourceReference{}},
			},
			ownerMachine: &capiv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &capiv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-2"},
				Spec: capiv1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Name: "powervs-cluster"}}},
			expectError: false,
		},
		{
			name: "Should not Reconcile if IBMPowerVSImage is not found",
			powervsMachine: &infrav1beta2.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "powervs-test-5",
					Labels: map[string]string{capiv1beta1.ClusterNameAnnotation: "capi-test-3"},
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
							Name:       "capi-test-3",
							UID:        "1",
						},
					},
				}, Spec: infrav1beta2.IBMPowerVSMachineSpec{
					ServiceInstanceID: "service-instance-1",
					ImageRef: &corev1.LocalObjectReference{
						Name: "capi-image",
					}},
			},
			ownerMachine: &capiv1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &capiv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-3"},
				Spec: capiv1beta1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{Name: "powervs-cluster"}}},
			powervsCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "powervs-cluster"},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					ServiceInstanceID: "service-instance-1"}},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSMachineReconciler{
				Client: testEnv.Client,
				Log:    klog.Background(),
			}
			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())
			defer func() {
				g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
			}()

			createObject(g, tc.ownerCluster, ns.Name)
			defer cleanupObject(g, tc.ownerCluster)

			createObject(g, tc.powervsCluster, ns.Name)
			defer cleanupObject(g, tc.powervsCluster)

			createObject(g, tc.ownerMachine, ns.Name)
			defer cleanupObject(g, tc.ownerMachine)

			createObject(g, tc.powervsMachine, ns.Name)
			defer cleanupObject(g, tc.powervsMachine)

			if tc.powervsMachine != nil {
				g.Eventually(func() bool {
					machine := &infrav1beta2.IBMPowerVSMachine{}
					key := client.ObjectKey{
						Name:      tc.powervsMachine.Name,
						Namespace: ns.Name,
					}
					err = testEnv.Get(ctx, key, machine)
					return err == nil
				}, 10*time.Second).Should(Equal(true))

				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: tc.powervsMachine.Namespace,
						Name:      tc.powervsMachine.Name,
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

func TestIBMPowerVSMachineReconciler_Delete(t *testing.T) {
	var (
		mockpowervs  *mock.MockPowerVS
		mockCtrl     *gomock.Controller
		machineScope *scope.PowerVSMachineScope
		reconciler   IBMPowerVSMachineReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
		recorder := record.NewFakeRecorder(2)
		reconciler = IBMPowerVSMachineReconciler{
			Client:   testEnv.Client,
			Log:      klog.Background(),
			Recorder: recorder,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	t.Run("Reconciling deleting IBMPowerVSMachine ", func(t *testing.T) {
		t.Run("Should not delete IBMPowerVSMachine if instance ID not found", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)

			pvsmachine := newIBMPowerVSMachine()

			machineScope = &scope.PowerVSMachineScope{
				Logger:            klog.Background(),
				IBMPowerVSClient:  mockpowervs,
				IBMPowerVSMachine: pvsmachine,
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			}
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(len(machineScope.IBMPowerVSMachine.Finalizers)).To(BeZero())
		})
		t.Run("Should fail to delete PowerVS instance", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope = &scope.PowerVSMachineScope{
				Logger:           klog.Background(),
				IBMPowerVSClient: mockpowervs,
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{infrav1beta2.IBMPowerVSMachineFinalizer},
					},
					Spec: infrav1beta2.IBMPowerVSMachineSpec{},
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						InstanceID: "powervs-instance-id",
					},
				},
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			}
			mockpowervs.EXPECT().DeleteInstance(machineScope.IBMPowerVSMachine.Status.InstanceID).Return(errors.New("Could not delete PowerVS instance"))
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
		})
		t.Run("Should successfully delete the PowerVS machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			secret := newSecret()
			machine := newMachine()

			mockClient := fake.NewClientBuilder().WithObjects([]client.Object{secret}...).Build()
			machineScope = &scope.PowerVSMachineScope{
				Client:           mockClient,
				Logger:           klog.Background(),
				IBMPowerVSClient: mockpowervs,
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{infrav1beta2.IBMPowerVSMachineFinalizer},
					},
					Spec: infrav1beta2.IBMPowerVSMachineSpec{},
					Status: infrav1beta2.IBMPowerVSMachineStatus{
						InstanceID: "powervs-instance-id",
					},
				},
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
				DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				Machine:           machine,
			}
			mockpowervs.EXPECT().DeleteInstance(machineScope.IBMPowerVSMachine.Status.InstanceID).Return(nil)
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(len(machineScope.IBMPowerVSMachine.Finalizers)).To(BeZero())
		})
	})
}

func TestIBMPowerVSMachineReconciler_ReconcileOperations(t *testing.T) {
	var (
		mockpowervs  *mock.MockPowerVS
		mockCtrl     *gomock.Controller
		machineScope *scope.PowerVSMachineScope
		reconciler   IBMPowerVSMachineReconciler
		mockvpc      *mockVPC.MockVpc
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
		mockvpc = mockVPC.NewMockVpc(mockCtrl)
		recorder := record.NewFakeRecorder(2)
		reconciler = IBMPowerVSMachineReconciler{
			Client:   testEnv.Client,
			Log:      klog.Background(),
			Recorder: recorder,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}
	t.Run("Reconciling creating IBMPowerVSMachine ", func(t *testing.T) {
		t.Run("Should requeue if Cluster infrastructure status is not ready", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: false,
					},
				},
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
			}
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityInfo, infrav1beta2.WaitingForClusterInfrastructureReason}})
		})
		t.Run("Should requeue if IBMPowerVSImage status is not ready", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: false,
					},
				},
			}
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityInfo, infrav1beta2.WaitingForIBMPowerVSImageReason}})
		})
		t.Run("Should requeue if boostrap data secret reference is not found", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				Machine:           &capiv1beta1.Machine{},
				IBMPowerVSMachine: &infrav1beta2.IBMPowerVSMachine{},
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: true,
					},
				},
			}
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityInfo, infrav1beta2.WaitingForBootstrapDataReason}})
		})
		t.Run("Should fail reconcile with create instance failure due to error in retrieving bootstrap data secret", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			var instances = &models.PVMInstances{}
			machine := newMachine()
			pvsMachine := newIBMPowerVSMachine()
			mockclient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build()
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Client: mockclient,
				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				Machine:           machine,
				IBMPowerVSMachine: pvsMachine,
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: true,
					},
				},
				IBMPowerVSClient: mockpowervs,
			}
			mockpowervs.EXPECT().GetAllInstance().Return(instances, nil)

			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(HaveOccurred())
			g.Expect(result.RequeueAfter).To(BeZero())
			g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityError, infrav1beta2.InstanceProvisionFailedReason}})
		})

		t.Run("Should fail reconcile if creation of the load balancer pool member is unsuccessful", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)

			secret := newSecret()
			pvsmachine := newIBMPowerVSMachine()
			machine := newMachine()
			machine.Labels = map[string]string{
				"cluster.x-k8s.io/control-plane": "true",
			}

			mockclient := fake.NewClientBuilder().WithObjects([]client.Object{secret, pvsmachine, machine}...).Build()
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Client: mockclient,

				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				Machine:           machine,
				IBMPowerVSMachine: pvsmachine,
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: true,
					},
				},
				IBMVPCClient:     mockvpc,
				IBMPowerVSClient: mockpowervs,
				DHCPIPCacheStore: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"powervs.cluster.x-k8s.io/create-infra": "true",
						},
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: "capi-test-lb",
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"capi-test-lb": {
								ID: ptr.To("capi-test-lb-id"),
							},
						},
					},
				},
			}

			instanceReferences := &models.PVMInstances{
				PvmInstances: []*models.PVMInstanceReference{
					{
						PvmInstanceID: ptr.To("capi-test-machine-id"),
						ServerName:    ptr.To("capi-test-machine"),
					},
				},
			}
			instance := &models.PVMInstance{
				PvmInstanceID: ptr.To("capi-test-machine-id"),
				ServerName:    ptr.To("capi-test-machine"),
				Status:        ptr.To("ACTIVE"),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "192.168.7.1",
					},
				},
			}

			loadBalancer := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr("capi-test-lb-id"),
				ProvisioningStatus: core.StringPtr("active"),
				Name:               core.StringPtr("capi-test-lb-name"),
			}

			mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
			mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).ToNot(BeNil())
			g.Expect(result.Requeue).To((BeFalse()))
			g.Expect(result.RequeueAfter).To(BeZero())
			g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}})
		})

		t.Run("Should requeue if the load balancer pool member is created successfully, but its provisioning status is not active", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)

			secret := newSecret()
			pvsmachine := newIBMPowerVSMachine()
			machine := newMachine()
			machine.Labels = map[string]string{
				"cluster.x-k8s.io/control-plane": "true",
			}

			mockclient := fake.NewClientBuilder().WithObjects([]client.Object{secret, pvsmachine, machine}...).Build()
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Client: mockclient,

				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				Machine:           machine,
				IBMPowerVSMachine: pvsmachine,
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: true,
					},
				},
				IBMVPCClient:     mockvpc,
				IBMPowerVSClient: mockpowervs,
				DHCPIPCacheStore: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"powervs.cluster.x-k8s.io/create-infra": "true",
						},
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
							{
								Name: "capi-test-lb",
							},
						},
					},
					Status: infrav1beta2.IBMPowerVSClusterStatus{
						LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
							"capi-test-lb": {
								ID: ptr.To("capi-test-lb-id"),
							},
						},
					},
				},
			}

			instanceReferences := &models.PVMInstances{
				PvmInstances: []*models.PVMInstanceReference{
					{
						PvmInstanceID: ptr.To("capi-test-machine-id"),
						ServerName:    ptr.To("capi-test-machine"),
					},
				},
			}
			instance := &models.PVMInstance{
				PvmInstanceID: ptr.To("capi-test-machine-id"),
				ServerName:    ptr.To("capi-test-machine"),
				Status:        ptr.To("ACTIVE"),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "192.168.7.1",
					},
				},
			}

			loadBalancer := &vpcv1.LoadBalancer{
				ID:                 core.StringPtr("capi-test-lb-id"),
				ProvisioningStatus: core.StringPtr("active"),
				Name:               core.StringPtr("capi-test-lb-name"),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   core.StringPtr("capi-test-lb-pool-id"),
						Name: core.StringPtr("capi-test-lb-pool-name"),
					},
				},
			}

			loadBalancerPoolMemberCollection := &vpcv1.LoadBalancerPoolMemberCollection{
				Members: []vpcv1.LoadBalancerPoolMember{
					{
						ID: core.StringPtr("capi-test-lb-pool-id"),
					},
				},
			}

			loadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{
				ID:                 core.StringPtr("capi-test-lb-pool-member-id"),
				ProvisioningStatus: core.StringPtr("update-pending"),
			}

			mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
			mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(loadBalancerPoolMemberCollection, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(loadBalancerPoolMember, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(true))
			g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}})
		})

		t.Run("Should reconcile IBMPowerVSMachine instance creation in different states", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)

			secret := newSecret()
			pvsmachine := newIBMPowerVSMachine()
			machine := newMachine()

			mockclient := fake.NewClientBuilder().WithObjects([]client.Object{secret, pvsmachine, machine}...).Build()
			machineScope = &scope.PowerVSMachineScope{
				Logger: klog.Background(),
				Client: mockclient,
				Cluster: &capiv1beta1.Cluster{
					Status: capiv1beta1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				Machine:           machine,
				IBMPowerVSMachine: pvsmachine,
				IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
					Status: infrav1beta2.IBMPowerVSImageStatus{
						Ready: true,
					},
				},
				IBMPowerVSClient:  mockpowervs,
				DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
			}

			instanceReferences := &models.PVMInstances{
				PvmInstances: []*models.PVMInstanceReference{
					{
						PvmInstanceID: ptr.To("capi-test-machine-id"),
						ServerName:    ptr.To("capi-test-machine"),
					},
				},
			}
			instance := &models.PVMInstance{
				PvmInstanceID: ptr.To("capi-test-machine-id"),
				ServerName:    ptr.To("capi-test-machine"),
				Status:        ptr.To("BUILD"),
			}

			mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
			mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
			result, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(false))
			g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
			expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityWarning, infrav1beta2.InstanceNotReadyReason}})

			t.Run("When PVM instance is in SHUTOFF state", func(_ *testing.T) {
				instance.Status = ptr.To("SHUTOFF")
				mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
				mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
				result, err = reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(BeZero())
				g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(false))
				g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
				expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityError, infrav1beta2.InstanceStoppedReason}})
			})
			t.Run("When PVM instance is in ACTIVE state", func(_ *testing.T) {
				instance.Status = ptr.To("ACTIVE")
				mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
				mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
				result, err = reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(BeZero())
				g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(true))
				g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
				expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{conditionType: infrav1beta2.InstanceReadyCondition, status: corev1.ConditionTrue}})
			})
			t.Run("When PVM instance is in ERROR state", func(_ *testing.T) {
				instance.Status = ptr.To("ERROR")
				instance.Fault = &models.PVMInstanceFault{Details: "Timeout creating instance"}
				mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
				mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
				result, err = reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(BeZero())
				g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(false))
				g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
				expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionFalse, capiv1beta1.ConditionSeverityError, infrav1beta2.InstanceErroredReason}})
			})
			t.Run("When PVM instance is in unknown state", func(_ *testing.T) {
				instance.Status = ptr.To("UNKNOWN")
				mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
				mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
				result, err = reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
				g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(false))
				g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
				expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{conditionType: infrav1beta2.InstanceReadyCondition, status: corev1.ConditionUnknown}})
			})
		})
	})

	t.Run("Should skip creation of loadbalancer pool member if not control plane machine", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)
		secret := newSecret()
		pvsmachine := newIBMPowerVSMachine()
		pvsmachine.ObjectMeta.Labels = map[string]string{
			"node-role.kubernetes.io/worker": "true",
		}
		machine := newMachine()

		mockclient := fake.NewClientBuilder().WithObjects([]client.Object{secret, pvsmachine, machine}...).Build()

		machineScope = &scope.PowerVSMachineScope{
			Logger: klog.Background(),
			Client: mockclient,

			Cluster: &capiv1beta1.Cluster{
				Status: capiv1beta1.ClusterStatus{
					InfrastructureReady: true,
				},
			},
			Machine:           machine,
			IBMPowerVSMachine: pvsmachine,
			IBMPowerVSImage: &infrav1beta2.IBMPowerVSImage{
				Status: infrav1beta2.IBMPowerVSImageStatus{
					Ready: true,
				},
			},
			IBMPowerVSClient: mockpowervs,
			DHCPIPCacheStore: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
			IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"powervs.cluster.x-k8s.io/create-infra": "true",
					},
				},
				Spec: infrav1beta2.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
						{
							Name: "capi-test-lb",
						},
					},
				},
				Status: infrav1beta2.IBMPowerVSClusterStatus{
					LoadBalancers: map[string]infrav1beta2.VPCLoadBalancerStatus{
						"capi-test-lb": {
							ID: ptr.To("capi-test-lb-id"),
						},
					},
				},
			},
		}

		instanceReferences := &models.PVMInstances{
			PvmInstances: []*models.PVMInstanceReference{
				{
					PvmInstanceID: ptr.To("capi-test-machine-id"),
					ServerName:    ptr.To("capi-test-machine"),
				},
			},
		}
		instance := &models.PVMInstance{
			PvmInstanceID: ptr.To("capi-test-machine-id"),
			ServerName:    ptr.To("capi-test-machine"),
			Status:        ptr.To("ACTIVE"),
			Networks: []*models.PVMInstanceNetwork{
				{
					IPAddress: "192.168.7.1",
				},
			},
		}

		mockpowervs.EXPECT().GetAllInstance().Return(instanceReferences, nil)
		mockpowervs.EXPECT().GetInstance(gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
		result, err := reconciler.reconcileNormal(machineScope)
		g.Expect(err).To(BeNil())
		g.Expect(result.Requeue).To(BeFalse())
		g.Expect(result.RequeueAfter).To(BeZero())
		g.Expect(machineScope.IBMPowerVSMachine.Status.Ready).To(Equal(true))
		g.Expect(machineScope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSMachineFinalizer))
		expectConditions(g, machineScope.IBMPowerVSMachine, []conditionAssertion{{infrav1beta2.InstanceReadyCondition, corev1.ConditionTrue, "", ""}})
	})
}

type conditionAssertion struct {
	conditionType capiv1beta1.ConditionType
	status        corev1.ConditionStatus
	severity      capiv1beta1.ConditionSeverity
	reason        string
}

func expectConditions(g *WithT, m *infrav1beta2.IBMPowerVSMachine, expected []conditionAssertion) {
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

func createObject(g *WithT, obj client.Object, namespace string) {
	if obj.DeepCopyObject() != nil {
		obj.SetNamespace(namespace)
		g.Expect(testEnv.Create(ctx, obj)).To(Succeed())
	}
}

func cleanupObject(g *WithT, obj client.Object) {
	if obj.DeepCopyObject() != nil {
		g.Expect(testEnv.Cleanup(ctx, obj)).To(Succeed())
	}
}

func newSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiv1beta1.ClusterNameLabel: "powervs-cluster",
			},
			Name:      "bootsecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("user data"),
		},
	}
}

func newIBMPowerVSMachine() *infrav1beta2.IBMPowerVSMachine {
	return &infrav1beta2.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:       *ptr.To("capi-test-machine"),
			Finalizers: []string{infrav1beta2.IBMPowerVSMachineFinalizer},
		},
		Spec: infrav1beta2.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromString("0.5"),
			Image: &infrav1beta2.IBMPowerVSResourceReference{
				ID: ptr.To("capi-image-id"),
			},
			Network: infrav1beta2.IBMPowerVSResourceReference{
				ID: ptr.To("capi-net-id"),
			},
			ServiceInstanceID: *ptr.To("service-instance-1"),
		},
	}
}

func newMachine() *capiv1beta1.Machine {
	return &capiv1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-machine",
			Namespace: "default",
		},
		Spec: capiv1beta1.MachineSpec{
			Bootstrap: capiv1beta1.Bootstrap{
				DataSecretName: ptr.To("bootsecret"),
			},
		},
	}
}
