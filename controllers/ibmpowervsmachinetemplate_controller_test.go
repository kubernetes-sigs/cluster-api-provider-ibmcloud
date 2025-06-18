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
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSMachineTemplateReconciler_Reconcile(t *testing.T) {
	testCases := []struct {
		name                   string
		expectError            bool
		powerVSMachineTemplate *infrav1.IBMPowerVSMachineTemplate
		expectedCapacity       corev1.ResourceList
	}{
		{
			name:        "Should Reconcile successfully if no IBMPowerVSMachineTemplate found",
			expectError: false,
		},
		{
			name:                   "Should Reconcile with memory and fractional processor values",
			powerVSMachineTemplate: stubPowerVSMachineTemplate(intstr.FromString("0.5"), 4),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("4G"),
			},
			expectError: false,
		},
		{
			name:                   "Should Reconcile with valid memory and processor values",
			powerVSMachineTemplate: stubPowerVSMachineTemplate(intstr.FromInt(2), 32),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("32G"),
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSMachineTemplateReconciler{
				Client: testEnv.Client,
			}
			ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
			g.Expect(err).To(BeNil())
			defer func() {
				g.Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
			}()

			createObject(g, tc.powerVSMachineTemplate, ns.Name)
			defer cleanupObject(g, tc.powerVSMachineTemplate)

			if tc.powerVSMachineTemplate != nil {
				g.Eventually(func() bool {
					machineTemplate := &infrav1.IBMPowerVSMachineTemplate{}
					key := client.ObjectKey{
						Name:      tc.powerVSMachineTemplate.Name,
						Namespace: ns.Name,
					}
					err = testEnv.Get(ctx, key, machineTemplate)
					return err == nil
				}, 10*time.Second).Should(Equal(true))

				_, err := reconciler.Reconcile(ctx, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: ns.Name,
						Name:      tc.powerVSMachineTemplate.Name,
					},
				})
				if tc.expectError {
					g.Expect(err).ToNot(BeNil())
				} else {
					g.Expect(err).To(BeNil())
					g.Eventually(func() bool {
						machineTemplate := &infrav1.IBMPowerVSMachineTemplate{}
						key := client.ObjectKey{
							Name:      tc.powerVSMachineTemplate.Name,
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

func TestGetIBMPowerVSMachineCapacity(t *testing.T) {
	testCases := []struct {
		name                   string
		powerVSMachineTemplate infrav1.IBMPowerVSMachineTemplate
		expectedCapacity       corev1.ResourceList
		expectErr              bool
	}{
		{
			name:                   "with memory and cpu in fractional",
			powerVSMachineTemplate: *stubPowerVSMachineTemplate(intstr.FromString("0.5"), 4),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("4G"),
			},
		},
		{
			name:                   "with memory and cpu in fractional value greater than 1",
			powerVSMachineTemplate: *stubPowerVSMachineTemplate(intstr.FromString("1.5"), 8),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("8G"),
			},
		},
		{
			name:                   "with memory and cpu in whole number",
			powerVSMachineTemplate: *stubPowerVSMachineTemplate(intstr.FromInt(3), 8),
			expectedCapacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("24"),
				corev1.ResourceMemory: resource.MustParse("8G"),
			},
		},
		{
			name:                   "with invalid cpu",
			powerVSMachineTemplate: *stubPowerVSMachineTemplate(intstr.FromString("invalid_cpu"), 8),
			expectErr:              true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)
			capacity, err := getIBMPowerVSMachineCapacity(tc.powerVSMachineTemplate)
			if tc.expectErr {
				if err == nil {
					t.Fatal("getIBMPowerVSMachineCapacity expected to return an error")
				}
			} else {
				if err != nil {
					t.Fatalf("getIBMPowerVSMachineCapacity is not expected to return an error, error: %v", err)
				}
				g.Expect(capacity).To(Equal(tc.expectedCapacity))
			}
		})
	}
}

func stubPowerVSMachineTemplate(processor intstr.IntOrString, memory int32) *infrav1.IBMPowerVSMachineTemplate {
	return &infrav1.IBMPowerVSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "powervs-test-1",
		},
		Spec: infrav1.IBMPowerVSMachineTemplateSpec{
			Template: infrav1.IBMPowerVSMachineTemplateResource{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "test_service_instance_id_27",
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image"),
					},
					Processors: processor,
					MemoryGiB:  memory,
				},
			},
		},
	}
}
