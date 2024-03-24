/*
Copyright 2023 The Kubernetes Authors.

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
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	. "github.com/onsi/gomega"
)

func TestIBMVPCMachineTemplateReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name               string
		expectError        bool
		VPCMachineTemplate *infrav1beta2.IBMVPCMachineTemplate
	}{
		{
			name:        "Should Reconcile successfully if no IBMVPCMachineTemplate found",
			expectError: false,
		},
	}
	for _, tc := range testCases {
		setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc, *IBMVPCMachineTemplateReconciler) {
			t.Helper()
			mockvpc := mock.NewMockVpc(gomock.NewController(t))
			reconciler := &IBMVPCMachineTemplateReconciler{
				Client: testEnv.Client,
			}
			return gomock.NewController(t), mockvpc, reconciler
		}
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockController, _, reconciler := setup(t)
			t.Cleanup(mockController.Finish)
			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())
			defer func() {
				g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
			}()

			createObject(g, tc.VPCMachineTemplate, ns.Name)
			defer cleanupObject(g, tc.VPCMachineTemplate)

			if tc.VPCMachineTemplate != nil {
				g.Eventually(func() bool {
					machineTemplate := &infrav1beta2.IBMVPCMachineTemplate{}
					key := client.ObjectKey{
						Name:      tc.VPCMachineTemplate.Name,
						Namespace: ns.Name,
					}
					err = testEnv.Get(ctx, key, machineTemplate)
					return err == nil
				}, 10*time.Second).Should(Equal(true))
				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: ns.Name,
						Name:      tc.VPCMachineTemplate.Name,
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

func TestIBMVPCMachineTemplateReconciler_reconcileNormal(t *testing.T) {
	setup := func(t *testing.T) (*gomock.Controller, *mock.MockVpc, *IBMVPCMachineTemplateReconciler) {
		t.Helper()
		mockvpc := mock.NewMockVpc(gomock.NewController(t))
		reconciler := &IBMVPCMachineTemplateReconciler{
			Client: testEnv.Client,
		}
		return gomock.NewController(t), mockvpc, reconciler
	}

	t.Run("with valid profile ", func(tt *testing.T) {
		g := NewWithT(tt)
		var expectedCapacity corev1.ResourceList
		profileDetails := vpcv1.InstanceProfile{
			Name: ptr.To("bx2-4x16"),
			VcpuCount: &vpcv1.InstanceProfileVcpu{
				Type:  ptr.To("fixed"),
				Value: ptr.To(int64(4)),
			},
			Memory: &vpcv1.InstanceProfileMemory{
				Type:  ptr.To("fixed"),
				Value: ptr.To(int64(16)),
			},
		}
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		vPCMachineTemplate := stubVPCMachineTemplate("bx2-4x16")

		expectedCapacity = map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("16G"),
		}
		createObject(g, &vPCMachineTemplate, ns.Name)
		defer cleanupObject(g, &vPCMachineTemplate)

		mockController, mockvpc, reconciler := setup(t)
		t.Cleanup(mockController.Finish)
		g.Expect(err).To(BeNil())
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
		}()
		mockvpc.EXPECT().GetInstanceProfile(gomock.AssignableToTypeOf(&vpcv1.GetInstanceProfileOptions{})).Return(&profileDetails, &core.DetailedResponse{}, nil)
		_, err = reconciler.reconcileNormal(ctx, mockvpc, vPCMachineTemplate)
		if err != nil {
			t.Fatalf("ReconcileNormal is not expected to return an error, error: %v", err)
		}
		g.Expect(err).To(BeNil())
		g.Eventually(func() bool {
			machineTemplate := &infrav1beta2.IBMVPCMachineTemplate{}
			key := client.ObjectKey{
				Name:      vPCMachineTemplate.Name,
				Namespace: ns.Name,
			}
			err = testEnv.Get(ctx, key, machineTemplate)
			g.Expect(err).To(BeNil())
			return reflect.DeepEqual(machineTemplate.Status.Capacity, expectedCapacity)
		}, 10*time.Second).Should(Equal(true))
	},
	)

	t.Run("with invalid profile ", func(tt *testing.T) {
		g := NewWithT(tt)
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))

		vPCMachineTemplate := stubVPCMachineTemplate("")
		createObject(g, &vPCMachineTemplate, ns.Name)
		defer cleanupObject(g, &vPCMachineTemplate)

		mockController, mockvpc, reconciler := setup(t)
		t.Cleanup(mockController.Finish)
		g.Expect(err).To(BeNil())
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
		}()
		mockvpc.EXPECT().GetInstanceProfile(gomock.AssignableToTypeOf(&vpcv1.GetInstanceProfileOptions{})).Return(nil, &core.DetailedResponse{}, nil)
		_, err = reconciler.reconcileNormal(ctx, mockvpc, vPCMachineTemplate)
		if err == nil {
			t.Fatalf("ReconcileNormal is  expected to return an error")
		} else {
			g.Expect(err).NotTo(BeNil())
		}
	},
	)

	t.Run("Error while fetching profile details ", func(tt *testing.T) {
		g := NewWithT(tt)
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))

		vPCMachineTemplate := stubVPCMachineTemplate("")
		createObject(g, &vPCMachineTemplate, ns.Name)
		defer cleanupObject(g, &vPCMachineTemplate)

		mockController, mockvpc, reconciler := setup(t)
		t.Cleanup(mockController.Finish)
		g.Expect(err).To(BeNil())
		defer func() {
			g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
		}()
		mockvpc.EXPECT().GetInstanceProfile(gomock.AssignableToTypeOf(&vpcv1.GetInstanceProfileOptions{})).Return(nil, nil, fmt.Errorf("intentional error"))
		_, err = reconciler.reconcileNormal(ctx, mockvpc, vPCMachineTemplate)
		if err == nil {
			t.Fatalf("ReconcileNormal is  expected to return an error")
		} else {
			g.Expect(err).NotTo(BeNil())
		}
	},
	)
}

func stubVPCMachineTemplate(profile string) infrav1beta2.IBMVPCMachineTemplate {
	return infrav1beta2.IBMVPCMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vpc-test-1",
		},
		Spec: infrav1beta2.IBMVPCMachineTemplateSpec{
			Template: infrav1beta2.IBMVPCMachineTemplateResource{
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{
						ID: ptr.To("capi-image"),
					},
					Profile: profile,
				},
			},
		},
	}
}
