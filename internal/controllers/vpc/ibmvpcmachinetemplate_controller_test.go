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

package vpc

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

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	. "github.com/onsi/gomega"
)

func TestIBMVPCMachineTemplateReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name               string
		expectError        bool
		VPCMachineTemplate *infrav1.IBMVPCMachineTemplate
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
					machineTemplate := &infrav1.IBMVPCMachineTemplate{}
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
			machineTemplate := &infrav1.IBMVPCMachineTemplate{}
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

	t.Run("with enum and range profile", func(tt *testing.T) {
		g := NewWithT(tt)
		profileDetails := vpcv1.InstanceProfile{
			Name: ptr.To("gx2-16x128"),
			VcpuCount: &vpcv1.InstanceProfileVcpuRange{
				Type:    ptr.To("range"),
				Default: ptr.To(int64(4)),
				Min:     ptr.To(int64(2)),
				Max:     ptr.To(int64(8)),
				Step:    ptr.To(int64(2)),
			},
			Memory: &vpcv1.InstanceProfileMemoryEnum{
				Type:    ptr.To("enum"),
				Default: ptr.To(int64(16)),
				Values:  []int64{16, 32},
			},
		}
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		vPCMachineTemplate := stubVPCMachineTemplate("bx2-4x16")

		expectedCapacity := corev1.ResourceList{
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
		g.Eventually(func() bool {
			machineTemplate := &infrav1.IBMVPCMachineTemplate{}
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

	t.Run("with profile advertising only a default value", func(tt *testing.T) {
		g := NewWithT(tt)
		// The SDK unmarshals "memory"/"vcpu_count" into the base model, where
		// "value" stays nil for enum/range/dependent shaped profiles.
		profileDetails := vpcv1.InstanceProfile{
			Name: ptr.To("mx2-8x64"),
			VcpuCount: &vpcv1.InstanceProfileVcpu{
				Type:    ptr.To("range"),
				Default: ptr.To(int64(8)),
			},
			Memory: &vpcv1.InstanceProfileMemory{
				Type:    ptr.To("enum"),
				Default: ptr.To(int64(64)),
			},
		}
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		vPCMachineTemplate := stubVPCMachineTemplate("bx2-4x16")

		expectedCapacity := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8"),
			corev1.ResourceMemory: resource.MustParse("64G"),
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
		g.Eventually(func() bool {
			machineTemplate := &infrav1.IBMVPCMachineTemplate{}
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

	t.Run("with dependent profile", func(tt *testing.T) {
		g := NewWithT(tt)
		// Dependent profiles carry no value or default, so no capacity is reported.
		profileDetails := vpcv1.InstanceProfile{
			Name: ptr.To("gx2-16x128x2"),
			VcpuCount: &vpcv1.InstanceProfileVcpuDependent{
				Type: ptr.To(vpcv1.InstanceProfileVcpuDependentTypeDependentConst),
			},
			Memory: &vpcv1.InstanceProfileMemoryDependent{
				Type: ptr.To(vpcv1.InstanceProfileMemoryDependentTypeDependentConst),
			},
		}
		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		vPCMachineTemplate := stubVPCMachineTemplate("gx2-16x128x2")

		// "capacity" is omitempty, so an empty ResourceList round-trips as nil.
		expectedCapacity := corev1.ResourceList(nil)
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
		g.Eventually(func() bool {
			machineTemplate := &infrav1.IBMVPCMachineTemplate{}
			key := client.ObjectKey{
				Name:      vPCMachineTemplate.Name,
				Namespace: ns.Name,
			}
			if err := testEnv.Get(ctx, key, machineTemplate); err != nil {
				return false
			}
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

func stubVPCMachineTemplate(profile string) infrav1.IBMVPCMachineTemplate {
	return infrav1.IBMVPCMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vpc-test-1",
		},
		Spec: infrav1.IBMVPCMachineTemplateSpec{
			Template: infrav1.IBMVPCMachineTemplateResource{
				Spec: infrav1.IBMVPCMachineSpec{
					Image: &infrav1.IBMVPCResourceReference{
						ID: ptr.To("capi-image"),
					},
					Profile: profile,
				},
			},
		},
	}
}

func TestProfileMemoryGiB(t *testing.T) {
	testCases := []struct {
		name   string
		memory vpcv1.InstanceProfileMemoryIntf
		want   int64
		wantOK bool
	}{
		{name: "nil interface", memory: nil},
		{name: "typed nil base model", memory: (*vpcv1.InstanceProfileMemory)(nil)},
		{name: "base model with value", memory: &vpcv1.InstanceProfileMemory{Value: ptr.To(int64(16))}, want: 16, wantOK: true},
		{name: "base model prefers value over default", memory: &vpcv1.InstanceProfileMemory{Value: ptr.To(int64(16)), Default: ptr.To(int64(32))}, want: 16, wantOK: true},
		{name: "base model with default only", memory: &vpcv1.InstanceProfileMemory{Default: ptr.To(int64(64))}, want: 64, wantOK: true},
		{name: "base model with neither", memory: &vpcv1.InstanceProfileMemory{Type: ptr.To("dependent")}},
		{name: "fixed", memory: &vpcv1.InstanceProfileMemoryFixed{Value: ptr.To(int64(8))}, want: 8, wantOK: true},
		{name: "typed nil fixed", memory: (*vpcv1.InstanceProfileMemoryFixed)(nil)},
		{name: "fixed without value", memory: &vpcv1.InstanceProfileMemoryFixed{}},
		{name: "enum", memory: &vpcv1.InstanceProfileMemoryEnum{Default: ptr.To(int64(16)), Values: []int64{16, 32}}, want: 16, wantOK: true},
		{name: "typed nil enum", memory: (*vpcv1.InstanceProfileMemoryEnum)(nil)},
		{name: "enum without default", memory: &vpcv1.InstanceProfileMemoryEnum{Values: []int64{16, 32}}},
		{name: "range", memory: &vpcv1.InstanceProfileMemoryRange{Default: ptr.To(int64(32)), Min: ptr.To(int64(16)), Max: ptr.To(int64(64))}, want: 32, wantOK: true},
		{name: "typed nil range", memory: (*vpcv1.InstanceProfileMemoryRange)(nil)},
		{name: "range without default", memory: &vpcv1.InstanceProfileMemoryRange{Min: ptr.To(int64(16))}},
		{name: "dependent", memory: &vpcv1.InstanceProfileMemoryDependent{Type: ptr.To(vpcv1.InstanceProfileMemoryDependentTypeDependentConst)}},
		{name: "typed nil dependent", memory: (*vpcv1.InstanceProfileMemoryDependent)(nil)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			got, ok := profileMemoryGiB(tc.memory)
			g.Expect(ok).To(Equal(tc.wantOK))
			g.Expect(got).To(Equal(tc.want))
		})
	}
}

func TestProfileVcpuCount(t *testing.T) {
	testCases := []struct {
		name      string
		vcpuCount vpcv1.InstanceProfileVcpuIntf
		want      int64
		wantOK    bool
	}{
		{name: "nil interface", vcpuCount: nil},
		{name: "typed nil base model", vcpuCount: (*vpcv1.InstanceProfileVcpu)(nil)},
		{name: "base model with value", vcpuCount: &vpcv1.InstanceProfileVcpu{Value: ptr.To(int64(4))}, want: 4, wantOK: true},
		{name: "base model prefers value over default", vcpuCount: &vpcv1.InstanceProfileVcpu{Value: ptr.To(int64(4)), Default: ptr.To(int64(8))}, want: 4, wantOK: true},
		{name: "base model with default only", vcpuCount: &vpcv1.InstanceProfileVcpu{Default: ptr.To(int64(8))}, want: 8, wantOK: true},
		{name: "base model with neither", vcpuCount: &vpcv1.InstanceProfileVcpu{Type: ptr.To("dependent")}},
		{name: "fixed", vcpuCount: &vpcv1.InstanceProfileVcpuFixed{Value: ptr.To(int64(2))}, want: 2, wantOK: true},
		{name: "typed nil fixed", vcpuCount: (*vpcv1.InstanceProfileVcpuFixed)(nil)},
		{name: "fixed without value", vcpuCount: &vpcv1.InstanceProfileVcpuFixed{}},
		{name: "enum", vcpuCount: &vpcv1.InstanceProfileVcpuEnum{Default: ptr.To(int64(4)), Values: []int64{4, 8}}, want: 4, wantOK: true},
		{name: "typed nil enum", vcpuCount: (*vpcv1.InstanceProfileVcpuEnum)(nil)},
		{name: "enum without default", vcpuCount: &vpcv1.InstanceProfileVcpuEnum{Values: []int64{4, 8}}},
		{name: "range", vcpuCount: &vpcv1.InstanceProfileVcpuRange{Default: ptr.To(int64(4)), Min: ptr.To(int64(2)), Max: ptr.To(int64(8))}, want: 4, wantOK: true},
		{name: "typed nil range", vcpuCount: (*vpcv1.InstanceProfileVcpuRange)(nil)},
		{name: "range without default", vcpuCount: &vpcv1.InstanceProfileVcpuRange{Min: ptr.To(int64(2))}},
		{name: "dependent", vcpuCount: &vpcv1.InstanceProfileVcpuDependent{Type: ptr.To(vpcv1.InstanceProfileVcpuDependentTypeDependentConst)}},
		{name: "typed nil dependent", vcpuCount: (*vpcv1.InstanceProfileVcpuDependent)(nil)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			got, ok := profileVcpuCount(tc.vcpuCount)
			g.Expect(ok).To(Equal(tc.wantOK))
			g.Expect(got).To(Equal(tc.want))
		})
	}
}
