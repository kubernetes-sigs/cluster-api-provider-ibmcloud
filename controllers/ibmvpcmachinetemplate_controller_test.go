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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"

	. "github.com/onsi/gomega"
)

func TestIBMVPCMachineTemplateReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name               string
		expectError        bool
		VPCMachineTemplate *infrav1beta2.IBMVPCMachineTemplate
		expectedCapacity   corev1.ResourceList
	}{
		{
			name:        "Should Reconcile successfully if no IBMVPCMachineTemplate found",
			expectError: false,
		},
		{
			name:               "Should Reconcile with valid profile value",
			VPCMachineTemplate: stubVPCMachineTemplate("bx2-2x8"),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8G"),
			},
			expectError: false,
		},
		{
			name:               "Should Reconcile with high memory profile value",
			VPCMachineTemplate: stubVPCMachineTemplate("vx2d-8x112"),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("112G"),
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMVPCMachineTemplateReconciler{
				Client: testEnv.Client,
			}
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
					g.Eventually(func() bool {
						machineTemplate := &infrav1beta2.IBMVPCMachineTemplate{}
						key := client.ObjectKey{
							Name:      tc.VPCMachineTemplate.Name,
							Namespace: ns.Name,
						}
						err = testEnv.Get(ctx, key, machineTemplate)
						g.Expect(err).To(BeNil())
						return reflect.DeepEqual(machineTemplate.Status.Capacity, tc.expectedCapacity)
					}, 10*time.Second).Should(Equal(true))
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

func TestGetIBMVPCMachineCapacity(t *testing.T) {
	testCases := []struct {
		name               string
		VPCMachineTemplate infrav1beta2.IBMVPCMachineTemplate
		expectedCapacity   corev1.ResourceList
		expectErr          bool
	}{
		{
			name:               "with instance storage profile ",
			VPCMachineTemplate: *stubVPCMachineTemplate("bx2d-128x512"),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("128"),
				corev1.ResourceMemory: resource.MustParse("512G"),
			},
		},
		{
			name:               "with compute profile",
			VPCMachineTemplate: *stubVPCMachineTemplate("cx2d-16x32"),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("32G"),
			},
		},
		{
			name:               "with GPU profile",
			VPCMachineTemplate: *stubVPCMachineTemplate("gx2-32x256x2v100"),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("32"),
				corev1.ResourceMemory: resource.MustParse("256G"),
			},
		},
		{
			name:               "with invalid profile",
			VPCMachineTemplate: *stubVPCMachineTemplate("gx2-"),
			expectErr:          true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)
			capacity, err := getIBMVPCMachineCapacity(tc.VPCMachineTemplate)
			if tc.expectErr {
				if err == nil {
					t.Fatal("getIBMPowerVSMachineCapacity expected to return an error")
				}
			} else {
				if err != nil {
					t.Fatalf("getIBMVPCMachineCapacity is not expected to return an error, error: %v", err)
				}
				g.Expect(capacity).To(Equal(tc.expectedCapacity))
			}
		})
	}
}

func stubVPCMachineTemplate(profile string) *infrav1beta2.IBMVPCMachineTemplate {
	return &infrav1beta2.IBMVPCMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vpc-test-1",
		},
		Spec: infrav1beta2.IBMVPCMachineTemplateSpec{
			Template: infrav1beta2.IBMVPCMachineTemplateResource{
				Spec: infrav1beta2.IBMVPCMachineSpec{
					Image: &infrav1beta2.IBMVPCResourceReference{
						ID: pointer.String("capi-image"),
					},
					Profile: profile,
				},
			},
		},
	}
}
