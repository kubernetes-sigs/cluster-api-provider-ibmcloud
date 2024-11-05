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
	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	gtmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/globaltagging/mock"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMVPCMachineReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name         string
		vpcMachine   *infrav1beta2.IBMVPCMachine
		ownerMachine *capiv1beta1.Machine
		vpcCluster   *infrav1beta2.IBMVPCCluster
		ownerCluster *capiv1beta1.Cluster
		expectError  bool
	}{
		{
			name:        "Should Reconcile successfully if no IBMVPCMachine found",
			expectError: false,
		},
		{
			name: "Should Reconcile if Owner Reference is not set",
			vpcMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-1",
				},
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{},
				},
			},
			expectError: false,
		},
		{
			name: "Should fail Reconcile if no OwnerMachine found",
			vpcMachine: &infrav1beta2.IBMVPCMachine{
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
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{},
				},
			},
			expectError: true,
		},
		{
			name: "Should not Reconcile if machine does not contain cluster label",
			vpcMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-test-3", OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: capiv1beta1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "capi-test-machine",
							UID:        "1",
						},
					},
				},
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{},
				},
			},
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
			vpcMachine: &infrav1beta2.IBMVPCMachine{
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
				},
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{},
				},
			},
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
				Log:    klog.Background(),
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
					machine := &infrav1beta2.IBMVPCMachine{}
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
		mockvpc      *vpcmock.MockVpc
		mockCtrl     *gomock.Controller
		machineScope *scope.MachineScope
		reconciler   IBMVPCMachineReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = vpcmock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		machineScope = &scope.MachineScope{
			Logger: klog.Background(),
			IBMVPCMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-machine",
					Labels: map[string]string{
						capiv1beta1.MachineControlPlaneNameLabel: "capi-control-plane-machine",
					},
					Finalizers: []string{infrav1beta2.MachineFinalizer},
				},
			},
			Machine: &capiv1beta1.Machine{
				Spec: capiv1beta1.MachineSpec{
					ClusterName: "vpc-cluster",
				},
			},
			IBMVPCCluster: &infrav1beta2.IBMVPCCluster{},
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
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
		options := &vpcv1.ListInstancesOptions{}
		response := &core.DetailedResponse{}
		instancelist := &vpcv1.InstanceCollection{}
		t.Run("Should fail reconcile IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			machineScope.Machine.Spec.Bootstrap.DataSecretName = ptr.To("capi-machine")
			machineScope.IBMVPCCluster.Status.Subnet.ID = ptr.To("capi-subnet-id")
			mockvpc.EXPECT().ListInstances(options).Return(instancelist, response, errors.New("Failed to create or fetch instance"))
			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
	})
}

func TestIBMVPCMachineLBReconciler_reconcile(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *vpcmock.MockVpc, *gtmock.MockGlobalTagging, *scope.MachineScope, IBMVPCMachineReconciler) {
		t.Helper()
		mockvpc := vpcmock.NewMockVpc(gomock.NewController(t))
		mockgt := gtmock.NewMockGlobalTagging(gomock.NewController(t))
		reconciler := IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		machineScope := &scope.MachineScope{
			Logger: klog.Background(),
			IBMVPCMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-machine",
					Labels: map[string]string{
						capiv1beta1.MachineControlPlaneNameLabel: "capi-control-plane-machine",
					},
					Finalizers: []string{infrav1beta2.MachineFinalizer},
				},
			},
			Machine: &capiv1beta1.Machine{
				Spec: capiv1beta1.MachineSpec{
					ClusterName: "vpc-cluster",
					Bootstrap: capiv1beta1.Bootstrap{
						DataSecretName: ptr.To("capi-machine"),
					},
				},
			},
			IBMVPCCluster: &infrav1beta2.IBMVPCCluster{
				Spec: infrav1beta2.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1beta2.VPCLoadBalancerSpec{
						Name: "vpc-load-balancer",
					},
				},
				Status: infrav1beta2.IBMVPCClusterStatus{
					Subnet: infrav1beta2.Subnet{
						ID: ptr.To("capi-subnet-id"),
					},
					VPCEndpoint: infrav1beta2.VPCEndpoint{
						LBID: core.StringPtr("vpc-load-balancer-id"),
					},
				},
			},
			Cluster:             &capiv1beta1.Cluster{},
			IBMVPCClient:        mockvpc,
			GlobalTaggingClient: mockgt,
		}
		return gomock.NewController(t), mockvpc, mockgt, machineScope, reconciler
	}

	t.Run("Reconcile creating IBMVPCMachine associated with LoadBalancer", func(t *testing.T) {
		instancelist := &vpcv1.InstanceCollection{
			Instances: []vpcv1.Instance{
				{
					Name: ptr.To("capi-machine"),
					ID:   ptr.To("capi-machine-id"),
					CRN:  ptr.To("capi-machine-crn"),
					PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
						PrimaryIP: &vpcv1.ReservedIPReference{
							Address: ptr.To("192.129.11.50"),
						},
						ID: ptr.To("capi-net"),
					},
					Status: ptr.To(vpcv1.InstanceStatusRunningConst),
				},
			},
		}
		loadBalancer := &vpcv1.LoadBalancer{
			ID:                 core.StringPtr("vpc-load-balancer-id"),
			ProvisioningStatus: core.StringPtr("active"),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID: core.StringPtr("foo-pool-id"),
				},
			},
		}
		existingTag := &globaltaggingv1.Tag{
			Name: ptr.To("capi-cluster"),
		}

		t.Run("Invalid primary ip address", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, mockgt, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)

			customInstancelist := &vpcv1.InstanceCollection{
				Instances: []vpcv1.Instance{
					{
						Name: ptr.To("capi-machine"),
						ID:   ptr.To("capi-machine-id"),
						CRN:  ptr.To("capi-machine-crn"),
						PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
							PrimaryIP: &vpcv1.ReservedIPReference{
								Address: ptr.To("0.0.0.0"),
							},
							ID: ptr.To("capi-net"),
						},
						Status: ptr.To(vpcv1.InstanceStatusRunningConst),
					},
				},
			}
			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(customInstancelist, &core.DetailedResponse{}, nil)
			mockgt.EXPECT().GetTagByName(gomock.AssignableToTypeOf("capi-cluster")).Return(existingTag, nil)
			mockgt.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(nil, &core.DetailedResponse{}, nil)

			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To((Not(BeNil())))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
		t.Run("Should fail to bind loadBalancer IP to control plane", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, mockgt, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)

			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			mockgt.EXPECT().GetTagByName(gomock.AssignableToTypeOf("capi-cluster")).Return(existingTag, nil)
			mockgt.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(nil, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, errors.New("failed to list loadBalancerPoolMembers"))

			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
		t.Run("Should successfully reconcile IBMVPCMachine but its status should be set to Not Ready when the PoolMember is not yet in the active state requiring a requeue", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, mockgt, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			customloadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{
				ID:                 core.StringPtr("foo-member-id"),
				ProvisioningStatus: core.StringPtr("create_pending"),
			}

			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			mockgt.EXPECT().GetTagByName(gomock.AssignableToTypeOf("capi-cluster")).Return(existingTag, nil)
			mockgt.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(nil, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(customloadBalancerPoolMember, &core.DetailedResponse{}, nil)

			result, err := reconciler.reconcileNormal(machineScope)
			// Requeue should be set when the Pool Member is found, but not yet ready (active).
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
			// Machine Status should not be ready (running but LB Member Pools not active).
			g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(false))
		})
		t.Run("Should successfully reconcile IBMVPCMachine", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, mockgt, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			loadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{
				ID:                 core.StringPtr("foo-member-id"),
				ProvisioningStatus: core.StringPtr("active"),
			}

			mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(instancelist, &core.DetailedResponse{}, nil)
			mockgt.EXPECT().GetTagByName(gomock.AssignableToTypeOf("capi-cluster")).Return(existingTag, nil)
			mockgt.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(nil, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(loadBalancerPoolMember, &core.DetailedResponse{}, nil)

			_, err := reconciler.reconcileNormal(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
			g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(true))
		})

		t.Run("Should reconcile IBMVPCMachine instance creation in different states", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, mockgt, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)

			loadBalancerPoolMember := &vpcv1.LoadBalancerPoolMember{
				ID:                 core.StringPtr("foo-member-id"),
				ProvisioningStatus: core.StringPtr("active"),
			}

			// Mocks setup for each test (4) below.
			mockgt.EXPECT().GetTagByName(gomock.AssignableToTypeOf("capi-cluster")).Return(existingTag, nil).MaxTimes(4)
			mockgt.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(nil, &core.DetailedResponse{}, nil).MaxTimes(4)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil).MaxTimes(4)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil).MaxTimes(4)
			mockvpc.EXPECT().CreateLoadBalancerPoolMember(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerPoolMemberOptions{})).Return(loadBalancerPoolMember, &core.DetailedResponse{}, nil).MaxTimes(4)

			t.Run("When VPC instance is pending", func(_ *testing.T) {
				customInstancelist := &vpcv1.InstanceCollection{
					Instances: []vpcv1.Instance{
						{
							Name: ptr.To("capi-machine"),
							ID:   ptr.To("capi-machine-id"),
							CRN:  ptr.To("capi-machine-crn"),
							PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
								PrimaryIP: &vpcv1.ReservedIPReference{
									Address: ptr.To("10.0.0.0"),
								},
								ID: ptr.To("capi-net"),
							},
							Status: ptr.To(vpcv1.InstanceStatusPendingConst),
						},
					},
				}
				mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(customInstancelist, &core.DetailedResponse{}, nil)

				result, err := reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
				g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(false))
			})

			t.Run("When VPC instance is running", func(_ *testing.T) {
				customInstancelist := &vpcv1.InstanceCollection{
					Instances: []vpcv1.Instance{
						{
							Name: ptr.To("capi-machine"),
							ID:   ptr.To("capi-machine-id"),
							CRN:  ptr.To("capi-machine-crn"),
							PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
								PrimaryIP: &vpcv1.ReservedIPReference{
									Address: ptr.To("10.0.0.0"),
								},
								ID: ptr.To("capi-net"),
							},
							Status: ptr.To(vpcv1.InstanceStatusRunningConst),
						},
					},
				}
				mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(customInstancelist, &core.DetailedResponse{}, nil)

				_, err := reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(true))
			})

			t.Run("When VPC instance is stopped", func(_ *testing.T) {
				customInstancelist := &vpcv1.InstanceCollection{
					Instances: []vpcv1.Instance{
						{
							Name: ptr.To("capi-machine"),
							ID:   ptr.To("capi-machine-id"),
							CRN:  ptr.To("capi-machine-crn"),
							PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
								PrimaryIP: &vpcv1.ReservedIPReference{
									Address: ptr.To("10.0.0.0"),
								},
								ID: ptr.To("capi-net"),
							},
							Status: ptr.To(vpcv1.InstanceStatusStoppedConst),
						},
					},
				}
				mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(customInstancelist, &core.DetailedResponse{}, nil)

				result, err := reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(Not(BeZero()))
				g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(false))
			})

			t.Run("When VPC instance is failed", func(_ *testing.T) {
				customInstancelist := &vpcv1.InstanceCollection{
					Instances: []vpcv1.Instance{
						{
							Name: ptr.To("capi-machine"),
							ID:   ptr.To("capi-machine-id"),
							CRN:  ptr.To("capi-machine-crn"),
							PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
								PrimaryIP: &vpcv1.ReservedIPReference{
									Address: ptr.To("10.0.0.0"),
								},
								ID: ptr.To("capi-net"),
							},
							Status: ptr.To(vpcv1.InstanceStatusFailedConst),
						},
					},
				}
				mockvpc.EXPECT().ListInstances(gomock.AssignableToTypeOf(&vpcv1.ListInstancesOptions{})).Return(customInstancelist, &core.DetailedResponse{}, nil)

				result, err := reconciler.reconcileNormal(machineScope)
				g.Expect(err).To(BeNil())
				g.Expect(result.RequeueAfter).To(BeZero())
				g.Expect(machineScope.IBMVPCMachine.Status.Ready).To(Equal(false))
			})
		})
	})
}

func TestIBMVPCMachineReconciler_Delete(t *testing.T) {
	var (
		mockvpc      *vpcmock.MockVpc
		mockCtrl     *gomock.Controller
		machineScope *scope.MachineScope
		reconciler   IBMVPCMachineReconciler
	)

	setup := func(t *testing.T) {
		t.Helper()
		mockCtrl = gomock.NewController(t)
		mockvpc = vpcmock.NewMockVpc(mockCtrl)
		reconciler = IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		machineScope = &scope.MachineScope{
			Logger: klog.Background(),
			IBMVPCMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "capi-machine",
					Finalizers: []string{infrav1beta2.MachineFinalizer},
				},
				Status: infrav1beta2.IBMVPCMachineStatus{
					InstanceID: "capi-machine-id",
				},
			},
			IBMVPCClient: mockvpc,
		}
	}
	teardown := func() {
		mockCtrl.Finish()
	}

	options := &vpcv1.DeleteInstanceOptions{ID: ptr.To("capi-instance-id")}
	t.Run("Reconciling deleting IBMVPCMachine", func(t *testing.T) {
		t.Run("Should fail to delete VPC machine", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(options)).Return(nil, errors.New("Failed to delete the VPC instance"))
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(Not(BeNil()))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
		t.Run("Should successfully delete VPC machine and remove the finalizer", func(t *testing.T) {
			g := NewWithT(t)
			setup(t)
			t.Cleanup(teardown)
			response := &core.DetailedResponse{}
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(options)).Return(response, nil)
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(Not(ContainElement(infrav1beta2.MachineFinalizer)))
		})
	})
}

func TestIBMVPCMachineLBReconciler_Delete(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *vpcmock.MockVpc, *scope.MachineScope, IBMVPCMachineReconciler) {
		t.Helper()
		mockvpc := vpcmock.NewMockVpc(gomock.NewController(t))
		reconciler := IBMVPCMachineReconciler{
			Client: testEnv.Client,
			Log:    klog.Background(),
		}
		machineScope := &scope.MachineScope{
			Logger: klog.Background(),
			IBMVPCMachine: &infrav1beta2.IBMVPCMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "capi-machine",
					Finalizers: []string{infrav1beta2.MachineFinalizer},
					Labels: map[string]string{
						capiv1beta1.MachineControlPlaneNameLabel: "capi-control-plane-machine",
					},
				},
				Status: infrav1beta2.IBMVPCMachineStatus{
					InstanceID: "capi-machine-id",
				},
			},
			IBMVPCClient: mockvpc,
			IBMVPCCluster: &infrav1beta2.IBMVPCCluster{
				Spec: infrav1beta2.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1beta2.VPCLoadBalancerSpec{
						Name: "vpc-load-balancer",
					},
				},
				Status: infrav1beta2.IBMVPCClusterStatus{
					VPCEndpoint: infrav1beta2.VPCEndpoint{
						LBID: core.StringPtr("vpc-load-balancer-id"),
					},
				},
			},
		}
		return gomock.NewController(t), mockvpc, machineScope, reconciler
	}

	t.Run("Reconciling deleting IBMVPCMachine associated with LoadBalancer", func(t *testing.T) {
		loadBalancer := &vpcv1.LoadBalancer{
			ID:                 core.StringPtr("vpc-load-balancer-id"),
			ProvisioningStatus: core.StringPtr("active"),
			Pools: []vpcv1.LoadBalancerPoolReference{
				{
					ID: core.StringPtr("foo-pool-id"),
				},
			},
		}

		t.Run("Should fail to delete VPC LoadBalancerPoolMember", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(&vpcv1.Instance{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, errors.New("failed to list LoadBalancerPoolMembers"))
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To((Not(BeNil())))
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(ContainElement(infrav1beta2.MachineFinalizer))
		})
		t.Run("Should successfully delete VPC machine and remove the finalizer", func(t *testing.T) {
			g := NewWithT(t)
			mockController, mockvpc, machineScope, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			mockvpc.EXPECT().GetLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.GetLoadBalancerOptions{})).Return(loadBalancer, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().GetInstance(gomock.AssignableToTypeOf(&vpcv1.GetInstanceOptions{})).Return(&vpcv1.Instance{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().ListLoadBalancerPoolMembers(gomock.AssignableToTypeOf(&vpcv1.ListLoadBalancerPoolMembersOptions{})).Return(&vpcv1.LoadBalancerPoolMemberCollection{}, &core.DetailedResponse{}, nil)
			mockvpc.EXPECT().DeleteInstance(gomock.AssignableToTypeOf(&vpcv1.DeleteInstanceOptions{})).Return(&core.DetailedResponse{}, nil)
			_, err := reconciler.reconcileDelete(machineScope)
			g.Expect(err).To(BeNil())
			g.Expect(machineScope.IBMVPCMachine.Finalizers).To(Not(ContainElement(infrav1beta2.MachineFinalizer)))
		})
	})
}
