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

	"go.uber.org/mock/gomock"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
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
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	powervsscope "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/scope/powervs"
	cosmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos/mock"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	mockRC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	mockVPC "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSMachineReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name           string
		powervsMachine *infrav1.IBMPowerVSMachine
		ownerMachine   *clusterv1.Machine
		powervsCluster *infrav1.IBMPowerVSCluster
		ownerCluster   *clusterv1.Cluster
		expectError    bool
	}{
		{
			name:        "Should Reconcile successfully if no IBMPowerVSMachine found",
			expectError: false,
		},
		{
			name: "Should Reconcile if Owner Reference is not set",
			powervsMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-1"},
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
					Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "service-instance-1"}}}},
			expectError: false,
		},
		{
			name: "Should fail Reconcile if no OwnerMachine found",
			powervsMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				},
				Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
					Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "service-instance-1"}}},
			},
			expectError: true,
		},
		{
			name: "Should not Reconcile if machine does not contain cluster label",
			powervsMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "powervs-test-3",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				}, Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
					Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "service-instance-1"}}},
			},
			ownerMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-1"}},
			expectError: false,
		},
		{
			name: "Should not Reconcile if IBMPowerVSCluster is not found",
			powervsMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "powervs-test-4",
					Labels: map[string]string{clusterv1.ClusterNameAnnotation: "capi-test-2"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test-2",
							UID:        "1",
						},
					},
				}, Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
					Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "service-instance-1"}}},
			},
			ownerMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-2"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Name: "powervs-cluster"}}},
			expectError: false,
		},
		{
			name: "Should not Reconcile if IBMPowerVSImage is not found",
			powervsMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "powervs-test-5",
					Labels: map[string]string{clusterv1.ClusterNameAnnotation: "capi-test-3"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test-3",
							UID:        "1",
						},
					},
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				}, Spec: infrav1.IBMPowerVSMachineSpec{
					Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
					Image: infrav1.IBMPowerVSMachineImage{
						Type:   infrav1.ImageSourceTypeImport,
						Import: infrav1.ImageReference{Name: "capi-image"},
					}},
			},
			ownerMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "capi-test-machine"}},
			ownerCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-test-3"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{Name: "powervs-cluster"}}},
			powervsCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "powervs-cluster"},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "service-instance-1",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "network-id",
						},
					}}},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSMachineReconciler{
				Client: testEnv.Client,
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
					machine := &infrav1.IBMPowerVSMachine{}
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

func TestIBMPowerVSMachineReconciler_reconcileDelete(t *testing.T) {
	testCases := []struct {
		name           string
		machine        *infrav1.IBMPowerVSMachine
		expect         func(m *mock.MockPowerVS)
		expectedError  string
		checkFinalizer bool
	}{
		{
			name: "Should remove finalizer when instance ID is not set",
			machine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-machine",
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				},
			},
			expect:         func(_ *mock.MockPowerVS) {},
			checkFinalizer: false,
		},
		{
			name: "Should fail to delete PowerVS instance",
			machine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-machine",
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				},
				Status: infrav1.IBMPowerVSMachineStatus{
					InstanceID: "powervs-instance-id",
				},
			},
			expect: func(m *mock.MockPowerVS) {
				m.EXPECT().DeleteInstance(gomock.Any(), "powervs-instance-id").Return(errors.New("could not delete PowerVS instance"))
			},
			expectedError:  "could not delete PowerVS instance",
			checkFinalizer: true,
		},
		{
			name: "Should successfully delete the PowerVS machine",
			machine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-machine",
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				},
				Status: infrav1.IBMPowerVSMachineStatus{
					InstanceID: "powervs-instance-id",
				},
			},
			expect: func(m *mock.MockPowerVS) {
				m.EXPECT().DeleteInstance(gomock.Any(), "powervs-instance-id").Return(nil)
			},
			checkFinalizer: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockPVS := mock.NewMockPowerVS(mockCtrl)
			if tc.expect != nil {
				tc.expect(mockPVS)
			}

			secret := newSecret()
			machine := newMachine()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secret).Build()

			reconciler := IBMPowerVSMachineReconciler{
				Client:   fakeClient,
				Recorder: record.NewFakeRecorder(2),
			}

			scope := &powervsscope.MachineScope{
				Client:            fakeClient,
				IBMPowerVSClient:  mockPVS,
				IBMPowerVSMachine: tc.machine,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
				DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				Machine:           machine,
				Recorder:          record.NewFakeRecorder(2),
			}

			_, err := reconciler.reconcileDelete(ctx, scope)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tc.checkFinalizer {
				g.Expect(scope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1.IBMPowerVSMachineFinalizer))
			} else {
				g.Expect(scope.IBMPowerVSMachine.Finalizers).To(BeEmpty())
			}
		})
	}
}

func TestIBMPowerVSMachineReconciler_reconcileNormal(t *testing.T) {
	testCases := []struct {
		name            string
		machine         *infrav1.IBMPowerVSMachine
		cluster         *clusterv1.Cluster
		pvsCluster      *infrav1.IBMPowerVSCluster
		image           *infrav1.IBMPowerVSImage
		ownerMachine    *clusterv1.Machine
		expect          func(m *mock.MockPowerVS, v *mockVPC.MockVpc)
		expectedError   string
		expectedRequeue bool
		checkCondition  func(g *WithT, m *infrav1.IBMPowerVSMachine)
	}{
		{
			name:    "Should requeue if Cluster infrastructure status is not ready",
			machine: newIBMPowerVSMachine(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{},
				},
			},
			expectedRequeue: true,
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.InstanceWaitingForClusterInfrastructureReadyReason}})
			},
		},
		{
			name:    "Should requeue if IBMPowerVSImage status is not ready",
			machine: newIBMPowerVSMachine(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			image:           &infrav1.IBMPowerVSImage{},
			expectedRequeue: true,
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.InstanceWaitingForImageReason}})
			},
		},
		{
			name:    "Should requeue if bootstrap data secret reference is not found",
			machine: newIBMPowerVSMachine(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			ownerMachine: &clusterv1.Machine{},
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{
					ImageState: infrav1.PowerVSImageStateACTIVE,
				},
			},
			expectedRequeue: false,
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.InstanceWaitingForControlPlaneInitializedReason}})
			},
		},
		{
			name:    "Should fail reconcile with create instance failure due to error in retrieving bootstrap data secret",
			machine: newIBMPowerVSMachine(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			ownerMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "owner-machine",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To("non-existent-secret"),
					},
				},
			},
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{
					ImageState: infrav1.PowerVSImageStateACTIVE,
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(&models.PVMInstances{}, nil)
			},
			expectedError: "failed to retrieve bootstrap data secret",
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.InstanceProvisionFailedReason}})
			},
		},
		{
			name:    "Should fail reconcile with InvalidMachineConfiguration when systemType is not supported",
			machine: newIBMPowerVSMachineWithInvalidConfiguration(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{
					ImageState: infrav1.PowerVSImageStateACTIVE,
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(&models.PVMInstances{}, nil)
				m.EXPECT().GetDatacenterDetails(gomock.Any(), "us-south").Return(&models.Datacenter{
					CapabilitiesDetails: &models.CapabilitiesDetails{
						SupportedSystems: &models.SupportedSystems{
							General: []string{"e980", "s1022", "s922"},
						},
					},
				}, nil)
			},
			expectedError: "is not supported in this zone",
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectV1Beta3Conditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.InvalidMachineConfigurationReason}})
			},
		},
		{
			name:    "Should reconcile IBMPowerVSMachine instance creation in BUILD state",
			machine: newIBMPowerVSMachine(),
			cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{
					ImageState: infrav1.PowerVSImageStateACTIVE,
				},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "serviceInstanceID",
						},
					},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
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
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceReferences, nil)
				m.EXPECT().GetInstance(gomock.Any(), "capi-test-machine-id").Return(instance, nil)
			},
			expectedRequeue: true,
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1.InstanceNotReadyReason}})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockPVS := mock.NewMockPowerVS(mockCtrl)
			mockVPCClient := mockVPC.NewMockVpc(mockCtrl)
			if tc.expect != nil {
				tc.expect(mockPVS, mockVPCClient)
			}

			objs := []client.Object{}
			secret := newSecret()
			objs = append(objs, secret)
			if tc.machine != nil {
				objs = append(objs, tc.machine)
			}
			if tc.ownerMachine != nil {
				objs = append(objs, tc.ownerMachine)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

			reconciler := IBMPowerVSMachineReconciler{
				Client:   fakeClient,
				Recorder: record.NewFakeRecorder(2),
			}

			scope := &powervsscope.MachineScope{
				Client:            fakeClient,
				IBMPowerVSClient:  mockPVS,
				IBMVPCClient:      mockVPCClient,
				IBMPowerVSMachine: tc.machine,
				Cluster:           tc.cluster,
				IBMPowerVSCluster: tc.pvsCluster,
				IBMPowerVSImage:   tc.image,
				Machine:           tc.ownerMachine,
				DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				ProviderIDFormat:  "v2",
			}
			if tc.pvsCluster == nil {
				scope.IBMPowerVSCluster = &infrav1.IBMPowerVSCluster{}
			}
			scope.SetZone("us-south")

			result, err := reconciler.reconcileNormal(ctx, scope)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tc.expectedRequeue {
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
			}

			if tc.checkCondition != nil {
				tc.checkCondition(g, scope.IBMPowerVSMachine)
			}
		})
	}
}

func TestIBMPowerVSMachineReconciler_markCondition(t *testing.T) {
	g := NewWithT(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	reconciler := IBMPowerVSMachineReconciler{Client: fakeClient}

	scope := &powervsscope.MachineScope{
		IBMPowerVSMachine: newIBMPowerVSMachine(),
	}

	reconciler.markCondition(scope, metav1.ConditionTrue, infrav1.InstanceReadyReason, "Machine ready")
	g.Expect(scope.IBMPowerVSMachine.Status.Conditions).ToNot(BeEmpty())

	reconciler.markCondition(scope, metav1.ConditionFalse, infrav1.InstanceNotReadyReason, "Machine not ready")
	g.Expect(scope.IBMPowerVSMachine.Status.Conditions).ToNot(BeEmpty())

	reconciler.markCondition(scope, metav1.ConditionUnknown, infrav1.InstanceStateUnknownReason, "Unknown state")
	g.Expect(scope.IBMPowerVSMachine.Status.Conditions).ToNot(BeEmpty())
}

func TestPatchIBMPowerVSMachine(t *testing.T) {
	g := NewWithT(t)
	machine := newIBMPowerVSMachine()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(machine).WithObjects(machine).Build()

	patchHelper, err := patch.NewHelper(machine, fakeClient)
	g.Expect(err).ToNot(HaveOccurred())

	err = patchIBMPowerVSMachine(ctx, patchHelper, machine)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestIBMPowerVSMachineReconciler_handleLoadBalancerPoolMemberConfiguration(t *testing.T) {
	testCases := []struct {
		name            string
		expect          func(v *mockVPC.MockVpc)
		expectedError   string
		expectedRequeue bool
	}{
		{
			name: "Should return error when CreateVPCLoadBalancerPoolMember fails",
			expect: func(v *mockVPC.MockVpc) {
				v.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{}, errors.New("lb error"))
			},
			expectedError: "failed to configure VPC load balancer pool member",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockVPCClient := mockVPC.NewMockVpc(mockCtrl)
			if tc.expect != nil {
				tc.expect(mockVPCClient)
			}

			reconciler := IBMPowerVSMachineReconciler{}
			scope := &powervsscope.MachineScope{
				IBMVPCClient: mockVPCClient,
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					Spec: infrav1.IBMPowerVSClusterSpec{
						VPC: infrav1.VPCSource{
							Type:   infrav1.SourceTypeReference,
							Region: "us-south",
						},
						LoadBalancers: []infrav1.LoadBalancerSource{
							{
								Type: infrav1.SourceTypeProvision,
								Provision: infrav1.LoadBalancerProvision{
									Name: "capi-test-lb",
								},
							},
						},
					},
					Status: infrav1.IBMPowerVSClusterStatus{
						LoadBalancers: []infrav1.LoadBalancerStatus{
							{
								Name: "capi-test-lb",
								ID:   "capi-test-lb-id",
							},
						},
					},
				},
			}

			res, err := reconciler.handleLoadBalancerPoolMemberConfiguration(ctx, scope)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tc.expectedRequeue {
				g.Expect(res.RequeueAfter).To(Not(BeZero()))
			}
		})
	}
}

func TestIBMPowerVSMachineReconciler_ibmPowerVSClusterToIBMPowerVSMachines(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	pvsCluster := &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "test-cluster",
					UID:        "12345",
				},
			},
		},
	}

	capiMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-machine-1",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				Name: "machine-1",
			},
		},
	}

	pvsMachine1 := &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(cluster, pvsCluster, capiMachine, pvsMachine1).Build()
	reconciler := IBMPowerVSMachineReconciler{Client: fakeClient}

	requests := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, pvsCluster)
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests[0].Name).To(Equal("machine-1"))
	g.Expect(requests[0].Namespace).To(Equal("default"))
}

type conditionAssertion struct {
	conditionType clusterv1.ConditionType
	status        corev1.ConditionStatus
	severity      clusterv1.ConditionSeverity
	reason        string
}

func expectConditions(g *WithT, m *infrav1.IBMPowerVSMachine, expected []conditionAssertion) {
	g.Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)))
	for _, c := range expected {
		actual := deprecatedv1beta1conditions.Get(m, c.conditionType)
		g.Expect(actual).To(Not(BeNil()))
		g.Expect(actual.Type).To(Equal(c.conditionType))
		g.Expect(actual.Status).To(Equal(c.status))
		g.Expect(actual.Severity).To(Equal(c.severity))
		g.Expect(actual.Reason).To(Equal(c.reason))
	}
}

func expectV1Beta3Conditions(g *WithT, m *infrav1.IBMPowerVSMachine, expected []conditionAssertion) {
	g.Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)))
	for _, c := range expected {
		var actual *metav1.Condition
		for i := range m.Status.Conditions {
			if m.Status.Conditions[i].Type == string(c.conditionType) {
				actual = &m.Status.Conditions[i]
				break
			}
		}
		g.Expect(actual).To(Not(BeNil()), "condition %s not found", c.conditionType)
		g.Expect(actual.Type).To(Equal(string(c.conditionType)))
		g.Expect(actual.Status).To(Equal(metav1.ConditionStatus(c.status)))
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
				clusterv1.ClusterNameLabel: "powervs-cluster",
			},
			Name:      "bootsecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("user data"),
		},
	}
}

func newIBMPowerVSMachine() *infrav1.IBMPowerVSMachine {
	return &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:       *ptr.To("capi-test-machine"),
			Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromString("0.5"),
			Image: infrav1.IBMPowerVSMachineImage{
				Type:      infrav1.ImageSourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-image-id"},
			},
			Network: infrav1.ResourceIdentifier{
				ID: "capi-net-id",
			},
			Workspace: infrav1.ResourceIdentifier{ID: "service-instance-1"},
		},
	}
}

func newIBMPowerVSMachineWithInvalidConfiguration() *infrav1.IBMPowerVSMachine {
	return &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:       *ptr.To("capi-test-machine-invalid-systype"),
			Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromString("0.5"),
			Image: infrav1.IBMPowerVSMachineImage{
				Type:      infrav1.ImageSourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-image-id-invalid-systype"},
			},
			SystemType: "Invalid",
			Network: infrav1.ResourceIdentifier{
				ID: "capi-net-id-invalid-systype",
			},
		},
	}
}

// TestHandleLoadBalancerPoolMemberConfiguration_PendingFlag verifies that the controller requeues at
// loadBalancerSettleRequeueInterval when CreateVPCLoadBalancerPoolMember reports pending work, does
// not requeue when registration is complete, and propagates hard errors (e.g. delete_pending) rather
// than swallowing them — the busy-vs-broken distinction end to end.
func TestHandleLoadBalancerPoolMemberConfiguration_PendingFlag(t *testing.T) {
	var (
		mockCtrl    *gomock.Controller
		mockvpc     *mockVPC.MockVpc
		mockpowervs *mock.MockPowerVS
	)
	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = mockVPC.NewMockVpc(mockCtrl)
		mockpowervs = mock.NewMockPowerVS(mockCtrl)
	}
	teardown := func() { mockCtrl.Finish() }

	// newScope builds a MachineScope wired for reconcileNormal with a single LB "capi-test-lb".
	newScope := func(mockclient client.Client) *powervsscope.MachineScope {
		secret := newSecret()
		pvsmachine := newIBMPowerVSMachine()
		machine := newMachine()
		machine.Labels = map[string]string{"cluster.x-k8s.io/control-plane": "true"}
		if mockclient == nil {
			mockclient = fake.NewClientBuilder().WithObjects(secret, pvsmachine, machine).Build()
		}
		return &powervsscope.MachineScope{
			Client: mockclient,
			Cluster: &clusterv1.Cluster{
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
					},
				},
			},
			Machine:           machine,
			IBMPowerVSMachine: pvsmachine,
			IBMPowerVSImage: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			IBMVPCClient:     mockvpc,
			IBMPowerVSClient: mockpowervs,
			DHCPIPCacheStore: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
			ProviderIDFormat: "v2",
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"powervs.cluster.x-k8s.io/create-infra": "true"},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "serviceInstanceID"},
					},
					VPC: infrav1.VPCSource{
						Type:   infrav1.SourceTypeReference,
						Region: "us-south",
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "capi-test-lb",
							},
						},
					},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{
						{Name: "capi-test-lb", ID: "capi-test-lb-id"},
					},
				},
			},
		}
	}

	instanceReferences := &models.PVMInstances{
		PvmInstances: []*models.PVMInstanceReference{
			{PvmInstanceID: ptr.To("capi-test-machine-id"), ServerName: ptr.To("capi-test-machine")},
		},
	}
	instance := &models.PVMInstance{
		PvmInstanceID: ptr.To("capi-test-machine-id"),
		ServerName:    ptr.To("capi-test-machine"),
		Status:        ptr.To("ACTIVE"),
		Networks:      []*models.PVMInstanceNetwork{{IPAddress: "192.168.7.1"}},
	}
	activeLB := &vpcv1.LoadBalancer{
		ID:                 core.StringPtr("capi-test-lb-id"),
		ProvisioningStatus: core.StringPtr("active"),
		Name:               core.StringPtr("capi-test-lb-name"),
		Pools: []vpcv1.LoadBalancerPoolReference{
			{ID: core.StringPtr("capi-test-pool-id"), Name: core.StringPtr("capi-test-pool-name")},
		},
	}

	t.Run("pending=true → RequeueAfter equals loadBalancerSettleRequeueInterval", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		reconciler := IBMPowerVSMachineReconciler{
			Client:   fake.NewClientBuilder().Build(),
			Recorder: record.NewFakeRecorder(10),
		}
		machineScope := newScope(nil)

		busyMember := &vpcv1.LoadBalancerPoolMember{
			ID:                 core.StringPtr("member-id"),
			ProvisioningStatus: core.StringPtr("update_pending"),
		}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(instanceReferences, nil)
		mockpowervs.EXPECT().GetInstance(gomock.Any(), gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
		mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(activeLB, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(busyMember, &core.DetailedResponse{}, nil)

		result, err := reconciler.reconcileNormal(ctx, machineScope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).To(Equal(loadBalancerSettleRequeueInterval))
	})

	t.Run("pending=false → empty Result (no requeue)", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		reconciler := IBMPowerVSMachineReconciler{
			Client:   fake.NewClientBuilder().Build(),
			Recorder: record.NewFakeRecorder(10),
		}
		machineScope := newScope(nil)

		activeMember := &vpcv1.LoadBalancerPoolMember{
			ID:                 core.StringPtr("member-id"),
			ProvisioningStatus: core.StringPtr("active"),
		}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(instanceReferences, nil)
		mockpowervs.EXPECT().GetInstance(gomock.Any(), gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
		mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(activeLB, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
		mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(activeMember, &core.DetailedResponse{}, nil)

		result, err := reconciler.reconcileNormal(ctx, machineScope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("scope error → reconciler propagates error, does not swallow it", func(t *testing.T) {
		g := NewWithT(t)
		setup(t)
		t.Cleanup(teardown)

		reconciler := IBMPowerVSMachineReconciler{
			Client:   fake.NewClientBuilder().Build(),
			Recorder: record.NewFakeRecorder(10),
		}
		machineScope := newScope(nil)

		// An LB in delete_pending is a non-transient error from CreateVPCLoadBalancerPoolMember.
		deletingLB := &vpcv1.LoadBalancer{
			ID:                 core.StringPtr("capi-test-lb-id"),
			Name:               core.StringPtr("capi-test-lb-name"),
			ProvisioningStatus: core.StringPtr("delete_pending"),
		}
		mockpowervs.EXPECT().ListInstances(gomock.Any()).Return(instanceReferences, nil)
		mockpowervs.EXPECT().GetInstance(gomock.Any(), gomock.AssignableToTypeOf("capi-test-machine-id")).Return(instance, nil)
		mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(deletingLB, &core.DetailedResponse{}, nil)

		_, err := reconciler.reconcileNormal(ctx, machineScope)
		g.Expect(err).ToNot(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to configure load balancer"))
	})
}

func newMachine() *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-machine",
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("bootsecret"),
			},
		},
	}
}

func TestIBMPowerVSMachineReconciler_reconcileDelete_ignitionError(t *testing.T) {
	t.Run("Should fail when DeleteMachineIgnition returns error", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockPVS := mock.NewMockPowerVS(mockCtrl)
		mockCOS := cosmock.NewMockCos(mockCtrl)

		// DeleteInstance succeeds
		mockPVS.EXPECT().DeleteInstance(gomock.Any(), "pvs-id").Return(nil)
		// DeleteObject (ignition cleanup) fails
		mockCOS.EXPECT().DeleteObject(gomock.Any()).Return(nil, errors.New("cos delete failed"))

		machine := newMachine()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(newSecret()).Build()

		reconciler := IBMPowerVSMachineReconciler{
			Client:   fakeClient,
			Recorder: record.NewFakeRecorder(2),
		}

		scope := &powervsscope.MachineScope{
			Client:           fakeClient,
			IBMPowerVSClient: mockPVS,
			COSClient:        mockCOS,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-machine",
					Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				},
				Status: infrav1.IBMPowerVSMachineStatus{
					InstanceID: "pvs-id",
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					// COSInstance.Type != "" triggers DeleteMachineIgnition
					COSInstance: infrav1.COSInstanceSource{Type: infrav1.SourceTypeProvision},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					COSInstance: infrav1.COSInstanceStatus{BucketName: "test-bucket"},
				},
			},
			DHCPIPCacheStore: cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
			Machine:          machine,
			Recorder:         record.NewFakeRecorder(2),
		}

		_, err := reconciler.reconcileDelete(ctx, scope)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to delete COS object"))
		// finalizer must still be present on error
		g.Expect(scope.IBMPowerVSMachine.Finalizers).To(ContainElement(infrav1.IBMPowerVSMachineFinalizer))
	})
}

func TestIBMPowerVSMachineReconciler_reconcileNormal_additionalBranches(t *testing.T) {
	// shared instance references used by multiple cases
	instanceRef := &models.PVMInstances{
		PvmInstances: []*models.PVMInstanceReference{
			{PvmInstanceID: ptr.To("inst-id"), ServerName: ptr.To("capi-test-machine")},
		},
	}

	activeInstance := func(extra ...func(*models.PVMInstance)) *models.PVMInstance {
		i := &models.PVMInstance{
			PvmInstanceID: ptr.To("inst-id"),
			ServerName:    ptr.To("capi-test-machine"),
			Status:        ptr.To("ACTIVE"),
			Networks:      []*models.PVMInstanceNetwork{{IPAddress: "192.168.1.10"}},
		}
		for _, f := range extra {
			f(i)
		}
		return i
	}

	testCases := []struct {
		name            string
		machine         *infrav1.IBMPowerVSMachine
		ownerMachine    *clusterv1.Machine
		pvsCluster      *infrav1.IBMPowerVSCluster
		image           *infrav1.IBMPowerVSImage
		expect          func(m *mock.MockPowerVS, v *mockVPC.MockVpc)
		expectedError   string
		expectedRequeue bool
		checkScope      func(g *WithT, scope *powervsscope.MachineScope)
		checkCondition  func(g *WithT, m *infrav1.IBMPowerVSMachine)
	}{
		// ── bootstrap gates ──────────────────────────────────────────────────
		{
			name:    "requeues with InstanceWaitingForBootstrapDataReason for cp machine with nil DataSecretName",
			machine: newIBMPowerVSMachine(),
			ownerMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{clusterv1.MachineControlPlaneLabel: ""},
				},
			},
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityInfo, infrav1.InstanceWaitingForBootstrapDataReason,
				}})
			},
		},
		// ── machine == nil (already triggered, re-queued) ────────────────────
		{
			name: "returns not-ready when CreateMachine returns nil with no error",
			// Pre-set InstanceReadyCondition=Unknown so CreateMachine returns nil, nil (line 327-329)
			machine: func() *infrav1.IBMPowerVSMachine {
				m := newIBMPowerVSMachine()
				conditions.Set(m, metav1.Condition{
					Type:   infrav1.InstanceReadyCondition,
					Status: metav1.ConditionUnknown,
					Reason: infrav1.InstanceStateUnknownReason,
				})
				return m
			}(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				// No matching instance found — ensureInstanceUnique returns nil
				m.EXPECT().ListInstances(gomock.Any()).Return(&models.PVMInstances{
					PvmInstances: []*models.PVMInstanceReference{},
				}, nil)
				// No GetInstance call expected — CreateMachine returns nil, nil
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				// InstanceStateUnknownReason is set (line 302)
				cond := conditions.Get(m, infrav1.InstanceReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionUnknown))
			},
		},
		// ── GetInstance error ────────────────────────────────────────────────
		{
			name:         "error when GetInstance fails after CreateMachine",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(nil, errors.New("get instance failed"))
			},
			expectedError: "get instance failed",
		},
		// ── SetProviderID error ──────────────────────────────────────────────
		{
			name:         "error when SetProviderID fails",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(activeInstance(), nil)
			},
			// Override ProviderIDFormat to something invalid so SetProviderID fails
			checkScope: func(_ *WithT, s *powervsscope.MachineScope) {
				s.ProviderIDFormat = "v1" // invalid → SetProviderID returns error
			},
			expectedError: "failed to set provider ID",
		},
		// ── SHUTOFF state ────────────────────────────────────────────────────
		{
			name:         "returns not-ready when instance is SHUTOFF",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				inst := activeInstance(func(i *models.PVMInstance) { i.Status = ptr.To("SHUTOFF") })
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(inst, nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityError, infrav1.InstanceStoppedReason,
				}})
			},
		},
		// ── ACTIVE — no VPC region (skip LB) ─────────────────────────────────
		{
			name:         "marks ready and skips LB when VPC region is not set",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
				// VPC.Region deliberately left empty
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(activeInstance(), nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionTrue, "", "",
				}})
			},
		},
		// ── ACTIVE — VPC set but no internal IP yet ───────────────────────────
		{
			name:         "waits for network address when internal IP is empty",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				// instance has no IPAddress → GetMachineInternalIP returns ""
				inst := &models.PVMInstance{
					PvmInstanceID: ptr.To("inst-id"),
					ServerName:    ptr.To("capi-test-machine"),
					Status:        ptr.To("ACTIVE"),
					Networks:      []*models.PVMInstanceNetwork{},
				}
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(inst, nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityWarning, infrav1.InstanceWaitingForNetworkAddressReason,
				}})
			},
		},
		// ── ERROR state — nil fault ───────────────────────────────────────────
		{
			name:         "returns not-ready with unknown error when instance is in ERROR with nil fault",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				inst := activeInstance(func(i *models.PVMInstance) {
					i.Status = ptr.To("ERROR")
					i.Fault = nil
				})
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(inst, nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityError, infrav1.InstanceErroredReason,
				}})
			},
		},
		// ── ERROR state — with fault details ─────────────────────────────────
		{
			name:         "returns not-ready with fault details when instance is in ERROR with fault",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				inst := activeInstance(func(i *models.PVMInstance) {
					i.Status = ptr.To("ERROR")
					i.Fault = &models.PVMInstanceFault{Details: "Timeout provisioning"}
				})
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(inst, nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityError, infrav1.InstanceErroredReason,
				}})
			},
		},
		// ── default / UNKNOWN state ───────────────────────────────────────────
		{
			name:         "requeues with unknown condition for undefined instance state",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReference{ID: "ws-id"},
				},
			},
			expect: func(m *mock.MockPowerVS, _ *mockVPC.MockVpc) {
				inst := activeInstance(func(i *models.PVMInstance) { i.Status = ptr.To("UNDEFINED") })
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(inst, nil)
			},
			expectedRequeue: true,
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionUnknown,
					clusterv1.ConditionSeverityNone, infrav1.InstanceStateUnknownReason,
				}})
			},
		},
		// ── LB config error path ──────────────────────────────────────────────
		{
			name:         "marks LB config failed condition when handleLoadBalancer returns error",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					LoadBalancers: []infrav1.LoadBalancerSource{{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{Name: "test-lb"},
					}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace:     infrav1.ResourceReference{ID: "ws-id"},
					LoadBalancers: []infrav1.LoadBalancerStatus{{ID: "lb-id", Name: "test-lb"}},
				},
			},
			expect: func(m *mock.MockPowerVS, v *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(activeInstance(), nil)
				v.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, &core.DetailedResponse{}, errors.New("lb get failed"))
			},
			expectedError: "failed to configure load balancer",
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionFalse,
					clusterv1.ConditionSeverityWarning, infrav1.InstanceLoadBalancerConfigurationFailedReason,
				}})
			},
		},
		// ── Full happy path: ACTIVE + VPC + LB success ───────────────────────
		{
			name:         "marks ready when instance is ACTIVE and LB member is created successfully",
			machine:      newIBMPowerVSMachine(),
			ownerMachine: newMachine(),
			image: &infrav1.IBMPowerVSImage{
				Status: infrav1.IBMPowerVSImageStatus{ImageState: infrav1.PowerVSImageStateACTIVE},
			},
			pvsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					VPC: infrav1.VPCSource{Region: "us-south"},
					LoadBalancers: []infrav1.LoadBalancerSource{{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{Name: "test-lb"},
					}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace:     infrav1.ResourceReference{ID: "ws-id"},
					LoadBalancers: []infrav1.LoadBalancerStatus{{ID: "lb-id", Name: "test-lb"}},
				},
			},
			expect: func(m *mock.MockPowerVS, v *mockVPC.MockVpc) {
				m.EXPECT().ListInstances(gomock.Any()).Return(instanceRef, nil)
				m.EXPECT().GetInstance(gomock.Any(), "inst-id").Return(activeInstance(), nil)
				// GetLoadBalancer — active LB with one pool
				v.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("test-lb"),
					ProvisioningStatus: ptr.To("active"),
					Pools: []vpcv1.LoadBalancerPoolReference{
						{ID: ptr.To("pool-id"), Name: ptr.To("pool-name")},
					},
					Listeners: []vpcv1.LoadBalancerListenerReference{},
				}, nil, nil)
				v.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{},
				}, nil, nil)
				v.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
					ID:                 ptr.To("member-id"),
					ProvisioningStatus: ptr.To("active"),
				}, nil, nil)
			},
			checkCondition: func(g *WithT, m *infrav1.IBMPowerVSMachine) {
				expectConditions(g, m, []conditionAssertion{{
					infrav1.InstanceReadyCondition, corev1.ConditionTrue, "", "",
				}})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockPVS := mock.NewMockPowerVS(mockCtrl)
			mockVPCClient := mockVPC.NewMockVpc(mockCtrl)
			if tc.expect != nil {
				tc.expect(mockPVS, mockVPCClient)
			}

			objs := []client.Object{newSecret()}
			if tc.machine != nil {
				objs = append(objs, tc.machine)
			}
			if tc.ownerMachine != nil {
				objs = append(objs, tc.ownerMachine)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

			reconciler := IBMPowerVSMachineReconciler{
				Client:   fakeClient,
				Recorder: record.NewFakeRecorder(4),
			}

			scope := &powervsscope.MachineScope{
				Client:            fakeClient,
				IBMPowerVSClient:  mockPVS,
				IBMVPCClient:      mockVPCClient,
				IBMPowerVSMachine: tc.machine,
				Cluster: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						Initialization: clusterv1.ClusterInitializationStatus{
							InfrastructureProvisioned: ptr.To(true),
						},
					},
				},
				IBMPowerVSCluster: tc.pvsCluster,
				IBMPowerVSImage:   tc.image,
				Machine:           tc.ownerMachine,
				DHCPIPCacheStore:  cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL),
				ProviderIDFormat:  "v2",
			}
			if scope.IBMPowerVSCluster == nil {
				scope.IBMPowerVSCluster = &infrav1.IBMPowerVSCluster{}
			}
			scope.SetZone("us-south")

			// allow test to mutate scope before reconcile (e.g. ProviderIDFormat)
			if tc.checkScope != nil {
				tc.checkScope(g, scope)
			}

			result, err := reconciler.reconcileNormal(ctx, scope)

			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			if tc.expectedRequeue {
				g.Expect(result.RequeueAfter).NotTo(BeZero())
			}
			if tc.checkCondition != nil {
				tc.checkCondition(g, scope.IBMPowerVSMachine)
			}
		})
	}
}

func TestIBMPowerVSMachineReconciler_handleLB_additionalBranches(t *testing.T) {
	t.Run("requeues when pool member is created but provisioning is not active", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockVPCClient := mockVPC.NewMockVpc(mockCtrl)

		lb := &vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("test-lb"),
			ProvisioningStatus: ptr.To("active"),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To("p-id"), Name: ptr.To("p-name")}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}
		mockVPCClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, nil, nil)
		mockVPCClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(
			&vpcv1.LoadBalancerPoolMemberCollection{Members: []vpcv1.LoadBalancerPoolMember{}}, nil, nil)
		mockVPCClient.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMember{
			ID:                 ptr.To("m-id"),
			ProvisioningStatus: ptr.To("create_pending"),
		}, nil, nil)

		scope := &powervsscope.MachineScope{
			IBMVPCClient: mockVPCClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Addresses: []clusterv1.MachineAddress{
						{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"},
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{Name: "test-lb"},
					}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{{ID: "lb-id", Name: "test-lb"}},
				},
			},
			Machine: newMachine(),
		}

		result, err := (&IBMPowerVSMachineReconciler{}).handleLoadBalancerPoolMemberConfiguration(ctx, scope)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})

	t.Run("returns empty result when pool member is nil (already registered)", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockVPCClient := mockVPC.NewMockVpc(mockCtrl)

		lb := &vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("test-lb"),
			ProvisioningStatus: ptr.To("active"),
			Pools:              []vpcv1.LoadBalancerPoolReference{{ID: ptr.To("p-id"), Name: ptr.To("p-name")}},
			Listeners:          []vpcv1.LoadBalancerListenerReference{},
		}
		mockVPCClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(lb, nil, nil)
		// Member already registered with machine's IP → returns nil pool member
		mockVPCClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(
			&vpcv1.LoadBalancerPoolMemberCollection{
				Members: []vpcv1.LoadBalancerPoolMember{{
					Target: &vpcv1.LoadBalancerPoolMemberTarget{Address: ptr.To("10.0.0.1")},
				}},
			}, nil, nil)

		scope := &powervsscope.MachineScope{
			IBMVPCClient: mockVPCClient,
			IBMPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Status: infrav1.IBMPowerVSMachineStatus{
					Addresses: []clusterv1.MachineAddress{
						{Type: clusterv1.MachineInternalIP, Address: "10.0.0.1"},
					},
				},
			},
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{Name: "test-lb"},
					}},
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					LoadBalancers: []infrav1.LoadBalancerStatus{{ID: "lb-id", Name: "test-lb"}},
				},
			},
			Machine: newMachine(),
		}

		result, err := (&IBMPowerVSMachineReconciler{}).handleLoadBalancerPoolMemberConfiguration(ctx, scope)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})
}

func TestPatchIBMPowerVSMachine_provisioned(t *testing.T) {
	t.Run("sets InstanceReadyCondition=True when Provisioned is true and condition absent", func(t *testing.T) {
		g := NewWithT(t)
		machine := newIBMPowerVSMachine()
		machine.Status.Initialization.Provisioned = ptr.To(true)
		// no InstanceReadyCondition set yet

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithStatusSubresource(machine).
			WithObjects(machine).
			Build()
		patchHelper, err := patch.NewHelper(machine, fakeClient)
		g.Expect(err).ToNot(HaveOccurred())

		err = patchIBMPowerVSMachine(ctx, patchHelper, machine)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the condition was set on the in-memory object before patching
		cond := conditions.Get(machine, infrav1.InstanceReadyCondition)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))

		// Round-trip: read back from the fake API server to confirm the patch was persisted
		persisted := &infrav1.IBMPowerVSMachine{}
		g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machine), persisted)).To(Succeed())
		persistedCond := conditions.Get(persisted, infrav1.InstanceReadyCondition)
		g.Expect(persistedCond).NotTo(BeNil())
		g.Expect(persistedCond.Status).To(Equal(metav1.ConditionTrue))
	})
}

func TestIBMPowerVSMachineReconciler_ibmPowerVSClusterToIBMPowerVSMachines_additionalBranches(t *testing.T) {
	t.Run("returns nil when object is not an IBMPowerVSCluster", func(t *testing.T) {
		g := NewWithT(t)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		reconciler := IBMPowerVSMachineReconciler{Client: fakeClient}

		// Pass a Machine instead of IBMPowerVSCluster
		result := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, &clusterv1.Machine{})
		g.Expect(result).To(BeNil())
	})

	t.Run("returns empty slice when owning cluster is not found", func(t *testing.T) {
		g := NewWithT(t)
		// IBMPowerVSCluster with no owner reference → GetOwnerCluster returns nil cluster
		pvsCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-owner-cluster",
				Namespace: "default",
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pvsCluster).Build()
		reconciler := IBMPowerVSMachineReconciler{Client: fakeClient}

		result := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, pvsCluster)
		g.Expect(result).To(BeEmpty())
	})

	t.Run("skips machines with empty InfrastructureRef name", func(t *testing.T) {
		g := NewWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		}
		pvsCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "test-cluster",
					UID:        "uid-1",
				}},
			},
		}
		// Machine with empty InfrastructureRef.Name — should be skipped
		emptyRefMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-ref-machine",
				Namespace: "default",
				Labels:    map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
			},
			Spec: clusterv1.MachineSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{Name: ""},
			},
		}
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(cluster, pvsCluster, emptyRefMachine).
			Build()
		reconciler := IBMPowerVSMachineReconciler{Client: fakeClient}

		result := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, pvsCluster)
		// empty-ref machine must be skipped
		g.Expect(result).To(BeEmpty())
	})
}

// errCacheStore is a cache.Store that always returns an error on Delete.
// Used to exercise the non-fatal DHCP cache-delete error path.
type errCacheStore struct{ cache.Store }

func (e errCacheStore) Delete(_ interface{}) error {
	return errors.New("cache delete failed")
}

// TestIBMPowerVSMachineReconciler_Reconcile_fakeclient covers the Reconcile
// branches that are unreachable via testEnv (non-NotFound Get error,
// machine==nil, InfraRef not defined, paused, IBMPowerVSCluster not found)
// using a plain fake.Client.
func TestIBMPowerVSMachineReconciler_Reconcile_fakeclient(t *testing.T) {
	t.Run("returns error when Get returns non-NotFound error", func(t *testing.T) {
		g := NewWithT(t)
		errClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, c client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					_ = c
					return apierrors.NewInternalError(errors.New("etcd unavailable"))
				},
			}).
			Build()
		r := &IBMPowerVSMachineReconciler{Client: errClient, Recorder: record.NewFakeRecorder(4)}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get IBMPowerVSMachine"))
	})

	t.Run("returns nil when owner Machine is not yet set (machine==nil)", func(t *testing.T) {
		g := NewWithT(t)
		// Machine object with no owner references → GetOwnerMachine returns nil
		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Namespace:  "default",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
			},
			Spec: infrav1.IBMPowerVSMachineSpec{
				Workspace: infrav1.ResourceIdentifier{ID: "ws-id"},
				Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "img-id"}},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pvsMachine).Build()
		r := &IBMPowerVSMachineReconciler{Client: fakeClient, Recorder: record.NewFakeRecorder(4)}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("returns nil when cluster label is missing (cluster==nil)", func(t *testing.T) {
		g := NewWithT(t)
		ownerMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: "owner-machine", Namespace: "default", UID: "m-uid"},
		}
		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Namespace:  "default",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       "owner-machine",
					UID:        "m-uid",
				}},
				// No cluster label → GetClusterFromMetadata returns nil
			},
			Spec: infrav1.IBMPowerVSMachineSpec{
				Workspace: infrav1.ResourceIdentifier{ID: "ws-id"},
				Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "img-id"}},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pvsMachine, ownerMachine).Build()
		r := &IBMPowerVSMachineReconciler{Client: fakeClient, Recorder: record.NewFakeRecorder(4)}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("returns nil when cluster InfrastructureRef is not defined", func(t *testing.T) {
		g := NewWithT(t)
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default", UID: "c-uid"},
			// Spec.InfrastructureRef is zero-value → IsDefined() == false
		}
		ownerMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-machine",
				Namespace: "default",
				UID:       "m-uid",
				Labels:    map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
			},
		}
		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Namespace:  "default",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				Labels:     map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       "owner-machine",
					UID:        "m-uid",
				}},
			},
			Spec: infrav1.IBMPowerVSMachineSpec{
				Workspace: infrav1.ResourceIdentifier{ID: "ws-id"},
				Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "img-id"}},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pvsMachine, ownerMachine, cluster).Build()
		r := &IBMPowerVSMachineReconciler{Client: fakeClient, Recorder: record.NewFakeRecorder(4)}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("returns nil when cluster is paused", func(t *testing.T) {
		g := NewWithT(t)
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default", UID: "c-uid"},
			Spec: clusterv1.ClusterSpec{
				Paused: ptr.To(true),
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "IBMPowerVSCluster",
					Name:     "pvs-cluster",
				},
			},
		}
		ownerMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-machine",
				Namespace: "default",
				UID:       "m-uid",
				Labels:    map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
			},
		}
		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Namespace:  "default",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				Labels:     map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       "owner-machine",
					UID:        "m-uid",
				}},
			},
			Spec: infrav1.IBMPowerVSMachineSpec{
				Workspace: infrav1.ResourceIdentifier{ID: "ws-id"},
				Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "img-id"}},
			},
		}
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(pvsMachine, ownerMachine, cluster).
			WithStatusSubresource(pvsMachine).
			Build()
		r := &IBMPowerVSMachineReconciler{Client: fakeClient, Recorder: record.NewFakeRecorder(4)}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("returns error when IBMPowerVSCluster not found", func(t *testing.T) {
		g := NewWithT(t)
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default", UID: "c-uid"},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav1.GroupVersion.Group,
					Kind:     "IBMPowerVSCluster",
					Name:     "pvs-cluster", // does not exist in the store
				},
			},
		}
		ownerMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-machine",
				Namespace: "default",
				UID:       "m-uid",
				Labels:    map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
			},
		}
		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Namespace:  "default",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
				Labels:     map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       "owner-machine",
					UID:        "m-uid",
				}},
			},
			Spec: infrav1.IBMPowerVSMachineSpec{
				Workspace: infrav1.ResourceIdentifier{ID: "ws-id"},
				Image:     infrav1.IBMPowerVSMachineImage{Type: infrav1.ImageSourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "img-id"}},
			},
			// Pre-set PausedCondition=False so EnsurePausedCondition does not requeue
			// before we reach the IBMPowerVSCluster fetch.
			Status: infrav1.IBMPowerVSMachineStatus{
				Conditions: []metav1.Condition{{
					Type:               clusterv1.PausedCondition,
					Status:             metav1.ConditionFalse,
					Reason:             "NotPaused",
					LastTransitionTime: metav1.Now(),
				}},
			},
		}
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(pvsMachine, ownerMachine, cluster).
			WithStatusSubresource(pvsMachine).
			Build()
		r := &IBMPowerVSMachineReconciler{Client: fakeClient, Recorder: record.NewFakeRecorder(4)}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-machine"}})
		g.Expect(err).To(HaveOccurred())
		// The aggregate error may include the patch defer failure too; check for the root cause
		g.Expect(err.Error()).To(ContainSubstring("IBMPowerVSCluster"))
	})
}

func TestIBMPowerVSMachineReconciler_reconcileDelete_dhcpCacheError(t *testing.T) {
	t.Run("logs but does not fail when DHCPIPCacheStore.Delete returns error", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockPVS := mock.NewMockPowerVS(mockCtrl)
		mockPVS.EXPECT().DeleteInstance(gomock.Any(), "pvs-id").Return(nil)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		reconciler := IBMPowerVSMachineReconciler{
			Client:   fakeClient,
			Recorder: record.NewFakeRecorder(2),
		}

		pvsMachine := &infrav1.IBMPowerVSMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-machine",
				Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
			},
			Status: infrav1.IBMPowerVSMachineStatus{InstanceID: "pvs-id"},
		}

		scope := &powervsscope.MachineScope{
			Client:            fakeClient,
			IBMPowerVSClient:  mockPVS,
			IBMPowerVSMachine: pvsMachine,
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{},
			// errCacheStore.Delete always returns an error, exercising the non-fatal log path
			DHCPIPCacheStore: errCacheStore{cache.NewTTLStore(powervs.CacheKeyFunc, powervs.CacheTTL)},
			Machine:          newMachine(),
			Recorder:         record.NewFakeRecorder(2),
		}

		_, err := reconciler.reconcileDelete(ctx, scope)
		// Must succeed despite cache error — it is explicitly non-fatal
		g.Expect(err).ToNot(HaveOccurred())
		// Finalizer should be removed on success
		g.Expect(scope.IBMPowerVSMachine.Finalizers).To(BeEmpty())
	})
}

func TestIBMPowerVSMachineReconciler_ibmPowerVSClusterToIBMPowerVSMachines_errorBranches(t *testing.T) {
	t.Run("returns empty when GetOwnerCluster returns non-NotFound error", func(t *testing.T) {
		g := NewWithT(t)
		// Inject a Get error for the Cluster lookup
		errClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, c client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					_ = c
					// Only error on Cluster lookups
					if _, ok := obj.(*clusterv1.Cluster); ok {
						return apierrors.NewInternalError(errors.New("cluster store unavailable"))
					}
					return nil
				},
			}).
			Build()

		pvsCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "test-cluster",
					UID:        "c-uid",
				}},
			},
		}
		reconciler := IBMPowerVSMachineReconciler{Client: errClient}
		result := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, pvsCluster)
		g.Expect(result).To(BeEmpty())
	})

	t.Run("returns nil when List Machines returns error", func(t *testing.T) {
		g := NewWithT(t)
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default", UID: "c-uid"},
		}
		pvsCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "test-cluster",
					UID:        "c-uid",
				}},
			},
		}
		// Inject a List error only for MachineList
		errClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(cluster, pvsCluster).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
					if _, ok := list.(*clusterv1.MachineList); ok {
						return apierrors.NewInternalError(errors.New("machines store unavailable"))
					}
					return nil
				},
			}).
			Build()

		reconciler := IBMPowerVSMachineReconciler{Client: errClient}
		result := reconciler.ibmPowerVSClusterToIBMPowerVSMachines(ctx, pvsCluster)
		g.Expect(result).To(BeNil())
	})
}

// reconcileTestEnvSetup creates the common set of Kubernetes objects needed
// for the "bottom half" of Reconcile (past EnsurePausedCondition) in testEnv.
// Returns the namespace, cluster, pvsCluster, ownerMachine, and pvsMachine.
// The caller is responsible for cleanup.
func reconcileTestEnvSetup(
	g *WithT,
	ns string,
	imageType infrav1.ImageSourceType,
) (
	cluster *clusterv1.Cluster,
	pvsCluster *infrav1.IBMPowerVSCluster,
	ownerMachine *clusterv1.Machine,
	pvsMachine *infrav1.IBMPowerVSMachine,
) {
	cluster = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: ns},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav1.GroupVersion.Group,
				Kind:     "IBMPowerVSCluster",
				Name:     "pvs-cluster",
			},
		},
	}
	pvsCluster = &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvs-cluster",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       "test-cluster",
				UID:        "c-uid",
			}},
		},
		Spec: infrav1.IBMPowerVSClusterSpec{
			Zone:     "us-south-1",
			Topology: infrav1.PowerVSVirtualIPTopology,
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "ws-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "net-id"},
			},
		},
	}
	ownerMachine = &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner-machine",
			Namespace: ns,
			UID:       "m-uid",
			Labels:    map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{DataSecretName: ptr.To("bootstrap-secret")},
		},
	}

	imageSpec := infrav1.IBMPowerVSMachineImage{Type: imageType}
	switch imageType {
	case infrav1.ImageSourceTypeReference:
		imageSpec.Reference = infrav1.ResourceIdentifier{ID: "img-id"}
	case infrav1.ImageSourceTypeImport:
		imageSpec.Import = infrav1.ImageReference{Name: "capi-image"}
	}

	pvsMachine = &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-pvm",
			Namespace:  ns,
			Labels:     map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
			Finalizers: []string{infrav1.IBMPowerVSMachineFinalizer},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Machine",
				Name:       "owner-machine",
				UID:        "m-uid",
			}},
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			MemoryGiB:  8,
			Processors: intstr.FromString("0.5"),
			Image:      imageSpec,
			Network:    infrav1.ResourceIdentifier{ID: "net-id"},
			Workspace:  infrav1.ResourceIdentifier{ID: "ws-id"},
		},
	}

	g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
	g.Expect(testEnv.Create(ctx, pvsCluster)).To(Succeed())
	g.Expect(testEnv.Create(ctx, ownerMachine)).To(Succeed())
	g.Expect(testEnv.Create(ctx, pvsMachine)).To(Succeed())

	// Wait for objects to be visible
	g.Eventually(func() bool {
		got := &infrav1.IBMPowerVSMachine{}
		return testEnv.Get(ctx, client.ObjectKey{Namespace: ns, Name: "test-pvm"}, got) == nil
	}, 10*time.Second).Should(BeTrue())

	return cluster, pvsCluster, ownerMachine, pvsMachine
}

// activeWorkspace returns a ResourceController stub that serves a valid active workspace.
func activeWorkspace() *resourcecontrollerv2.ResourceInstance {
	return &resourcecontrollerv2.ResourceInstance{
		GUID:     ptr.To("ws-guid"),
		RegionID: ptr.To("us-south"),
		State:    ptr.To("active"),
	}
}

// primeReconcile performs one reconcile with stubClientBuilder to drain the
// EnsurePausedCondition requeue that happens on the very first reconcile of
// any newly created IBMPowerVSMachine. Subsequent reconciles then proceed
// past that check straight to NewMachineScope.
func primeReconcile(g *WithT, ns, name string) {
	r := &IBMPowerVSMachineReconciler{
		Client:        testEnv.Client,
		Recorder:      record.NewFakeRecorder(4),
		ClientBuilder: stubClientBuilder{},
	}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: ns, Name: name}}
	// First call: EnsurePausedCondition sets PausedCondition and requeues.
	// We ignore the result — the only goal is to persist the condition.
	_, _ = r.Reconcile(ctx, req)
	// Wait until the PausedCondition is visible in the API server.
	g.Eventually(func() bool {
		got := &infrav1.IBMPowerVSMachine{}
		if err := testEnv.Get(ctx, req.NamespacedName, got); err != nil {
			return false
		}
		return conditions.Get(got, clusterv1.PausedCondition) != nil
	}, 10*time.Second).Should(BeTrue())
}

func TestIBMPowerVSMachineReconciler_Reconcile_withScope(t *testing.T) {
	// ── IBMPowerVSImage not found (lines 163-172) ─────────────────────────
	t.Run("returns nil when IBMPowerVSImage is not found", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster, pvsCluster, ownerMachine, pvsMachine := reconcileTestEnvSetup(
			g, ns.Name, infrav1.ImageSourceTypeImport,
		)
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, pvsMachine, ownerMachine, pvsCluster, cluster)).To(Succeed())
		}()
		// IBMPowerVSImage "capi-image" is deliberately NOT created → line 169-171

		// Prime: drain the first-reconcile EnsurePausedCondition requeue.
		primeReconcile(g, ns.Name, "test-pvm")

		r := &IBMPowerVSMachineReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(4),
			ClientBuilder: stubClientBuilder{},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: "test-pvm"},
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	// ── NewMachineScope error (lines 176-189) ─────────────────────────────
	t.Run("returns error when NewMachineScope fails", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster, pvsCluster, ownerMachine, pvsMachine := reconcileTestEnvSetup(
			g, ns.Name, infrav1.ImageSourceTypeReference,
		)
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, pvsMachine, ownerMachine, pvsCluster, cluster)).To(Succeed())
		}()

		// Prime: drain the first-reconcile EnsurePausedCondition requeue.
		primeReconcile(g, ns.Name, "test-pvm")

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		// Return an inactive workspace → resolveWorkspace fails → NewMachineScope errors
		inactiveState := "inactive"
		rcMock.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(&resourcecontrollerv2.ResourceInstance{
				GUID:  ptr.To("ws-guid"),
				State: &inactiveState,
			}, nil)

		r := &IBMPowerVSMachineReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(4),
			ClientBuilder: mockClientBuilder{rc: rcMock},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: "test-pvm"},
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to create IBMPowerVS machine scope"))
	})

	// ── reconcileDelete dispatch (lines 205-207) ──────────────────────────
	t.Run("dispatches to reconcileDelete when DeletionTimestamp is set", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster, pvsCluster, ownerMachine, pvsMachine := reconcileTestEnvSetup(
			g, ns.Name, infrav1.ImageSourceTypeReference,
		)
		defer func() {
			// machine may already be gone by the time cleanup runs
			_ = testEnv.Cleanup(ctx, pvsMachine, ownerMachine, pvsCluster, cluster)
		}()

		// Prime: drain the first-reconcile EnsurePausedCondition requeue.
		primeReconcile(g, ns.Name, "test-pvm")

		// Trigger deletion: object stays while finalizer is present;
		// DeletionTimestamp is set immediately.
		g.Expect(testEnv.Delete(ctx, pvsMachine)).To(Succeed())
		g.Eventually(func() bool {
			got := &infrav1.IBMPowerVSMachine{}
			_ = testEnv.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "test-pvm"}, got)
			return !got.DeletionTimestamp.IsZero()
		}, 10*time.Second).Should(BeTrue())

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(activeWorkspace(), nil)

		pvsMock := mock.NewMockPowerVS(mockCtrl)
		// InstanceID is empty so reconcileDelete returns early (no API call needed)

		r := &IBMPowerVSMachineReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(4),
			ClientBuilder: mockClientBuilder{rc: rcMock, pvs: pvsMock},
		}
		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: "test-pvm"},
		})
		g.Expect(err).ToNot(HaveOccurred())
	})

	// ── reconcileNormal dispatch (line 210) ───────────────────────────────
	t.Run("dispatches to reconcileNormal when machine is not being deleted", func(t *testing.T) {
		g := NewWithT(t)
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("ns-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())
		defer func() { g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed()) }()

		cluster, pvsCluster, ownerMachine, pvsMachine := reconcileTestEnvSetup(
			g, ns.Name, infrav1.ImageSourceTypeReference,
		)
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, pvsMachine, ownerMachine, pvsCluster, cluster)).To(Succeed())
		}()

		// Prime: drain the first-reconcile EnsurePausedCondition requeue.
		primeReconcile(g, ns.Name, "test-pvm")

		// Create bootstrap secret required by reconcileNormal → getRawBootstrapData
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-secret", Namespace: ns.Name},
			Data:       map[string][]byte{"value": []byte(`{"ignitionVersion":"3.4"}`)},
		}
		g.Expect(testEnv.Create(ctx, secret)).To(Succeed())
		defer func() { g.Expect(testEnv.Cleanup(ctx, secret)).To(Succeed()) }()

		rcMock := mockRC.NewMockResourceController(mockCtrl)
		rcMock.EXPECT().
			GetResourceInstanceByFilter(gomock.Any()).
			Return(activeWorkspace(), nil)

		pvsMock := mock.NewMockPowerVS(mockCtrl)
		// Cluster infra not ready → reconcileNormal returns early after first gate
		// (no ListInstances call needed)

		r := &IBMPowerVSMachineReconciler{
			Client:        testEnv.Client,
			Recorder:      record.NewFakeRecorder(4),
			ClientBuilder: mockClientBuilder{rc: rcMock, pvs: pvsMock},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: ns.Name, Name: "test-pvm"},
		})
		g.Expect(err).ToNot(HaveOccurred())
		// Cluster.Status.Initialization.InfrastructureProvisioned is nil →
		// reconcileNormal requeues
		g.Expect(result.RequeueAfter).ToNot(BeZero())
	})
}
