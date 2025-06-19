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

package webhooks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSMachine_default(t *testing.T) {
	g := NewWithT(t)
	powervsMachine := &infrav1.IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-machine",
			Namespace: "default",
		},
		Spec: infrav1.IBMPowerVSMachineSpec{
			MemoryGiB:  4,
			Processors: intstr.FromString("0.5"),
			Image: &infrav1.IBMPowerVSResourceReference{
				ID: ptr.To("capi-image"),
			},
		},
	}
	g.Expect((&IBMPowerVSMachine{}).Default(context.Background(), powervsMachine)).ToNot(HaveOccurred())
	g.Expect(powervsMachine.Spec.SystemType).To(BeEquivalentTo("s922"))
	g.Expect(powervsMachine.Spec.ProcessorType).To(BeEquivalentTo(infrav1.PowerVSProcessorTypeShared))
}

func TestIBMPowerVSMachine_create(t *testing.T) {
	tests := []struct {
		name           string
		powerVSMachine *infrav1.IBMPowerVSMachine
		wantErr        bool
	}{
		{
			name: "Should fail to validate IBMPowerVSMachine - incorrect spec values",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "a890",
					ProcessorType:     "unknown",
					Network: infrav1.IBMPowerVSResourceReference{
						ID:   ptr.To("capi-net-id"),
						Name: ptr.To("capi-net"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - no Image or Imagref in Spec",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - both Image and Imagref specified in Spec",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image:    &infrav1.IBMPowerVSResourceReference{},
					ImageRef: &corev1.LocalObjectReference{},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - Both Id and Name specified in Spec",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID:   ptr.To("capi-image-id"),
						Name: ptr.To("capi-image"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - invalid memory and processor values",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-image"),
					},
					Processors: intstr.FromString("two"),
					MemoryGiB:  int32(-4),
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully validate IBMPowerVSMachine - valid spec",
			powerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
					Processors: intstr.FromString("0.25"),
					MemoryGiB:  4,
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := tc.powerVSMachine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, machine); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIBMPowerVSMachine_update(t *testing.T) {
	tests := []struct {
		name              string
		oldPowerVSMachine *infrav1.IBMPowerVSMachine
		newPowerVSMachine *infrav1.IBMPowerVSMachine
		wantErr           bool
	}{
		{
			name: "Should fail to update IBMPowerVSMachine with invalid ProcessorType",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "e980",
					ProcessorType:     "invalid",
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid Network",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
						ID:   ptr.To("capi-net-ID"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid Image",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID:   ptr.To("capi-image-id"),
						Name: ptr.To("capi-image"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid memory",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         int32(-8),
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid processors",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("two"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully update IBMPowerVSMachine",
			oldPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					Image: &infrav1.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image-id"),
					},
				},
			},
			newPowerVSMachine: &infrav1.IBMPowerVSMachine{
				Spec: infrav1.IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     infrav1.PowerVSProcessorTypeShared,
					MemoryGiB:         8,
					Processors:        intstr.FromString("0.25"),
					Network: infrav1.IBMPowerVSResourceReference{
						Name: ptr.To("capi-net"),
					},
					ImageRef: &corev1.LocalObjectReference{
						Name: "capi-image",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := tc.oldPowerVSMachine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, machine); err != nil {
				t.Errorf("failed to create machine: %v", err)
			}
			machine.Spec = tc.newPowerVSMachine.Spec
			if err := testEnv.Update(ctx, machine); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
