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

package v1beta2

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/defaulting"
)

func TestIBMPowerVSMachine_default(t *testing.T) {
	g := NewWithT(t)
	powervsMachine := &IBMPowerVSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-machine",
			Namespace: "default",
		},
		Spec: IBMPowerVSMachineSpec{
			MemoryGiB:  4,
			Processors: intstr.FromString("0.5"),
			Image: &IBMPowerVSResourceReference{
				ID: pointer.String("capi-image"),
			},
		},
	}
	t.Run("Defaults for IBMPowerVSMachine", defaulting.DefaultValidateTest(powervsMachine))
	powervsMachine.Default()
	g.Expect(powervsMachine.Spec.SystemType).To(BeEquivalentTo("s922"))
	g.Expect(powervsMachine.Spec.ProcessorType).To(BeEquivalentTo(PowerVSProcessorTypeShared))
}

func TestIBMPowerVSMachine_create(t *testing.T) {
	tests := []struct {
		name           string
		powervsMachine *IBMPowerVSMachine
		wantErr        bool
	}{
		{
			name: "Should fail to validate IBMPowerVSMachine - incorrect spec values",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "a890",
					ProcessorType:     "unknown",
					Network: IBMPowerVSResourceReference{
						ID:   pointer.String("capi-net-id"),
						Name: pointer.String("capi-net"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - no Image or Imagref in Spec",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - both Image and Imagref specified in Spec",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image:    &IBMPowerVSResourceReference{},
					ImageRef: &corev1.LocalObjectReference{},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - Both Id and Name specified in Spec",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID:   pointer.String("capi-image-id"),
						Name: pointer.String("capi-image"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachine - invalid memory and processor values",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						Name: pointer.String("capi-image"),
					},
					Processors: intstr.FromString("two"),
					MemoryGiB:  int32(-4),
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully validate IBMPowerVSMachine - valid spec",
			powervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
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
			machine := tc.powervsMachine.DeepCopy()
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
		oldPowervsMachine *IBMPowerVSMachine
		newPowervsMachine *IBMPowerVSMachine
		wantErr           bool
	}{
		{
			name: "Should fail to update IBMPowerVSMachine with invalid ProcessorType",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "e980",
					ProcessorType:     "invalid",
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid Network",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
						ID:   pointer.String("capi-net-ID"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid Image",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID:   pointer.String("capi-image-id"),
						Name: pointer.String("capi-image"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid memory",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         int32(-8),
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachine with invalid processors",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("two"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully update IBMPowerVSMachine",
			oldPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         4,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
					},
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image-id"),
					},
				},
			},
			newPowervsMachine: &IBMPowerVSMachine{
				Spec: IBMPowerVSMachineSpec{
					ServiceInstanceID: "capi-si-id",
					SystemType:        "s922",
					ProcessorType:     PowerVSProcessorTypeShared,
					MemoryGiB:         8,
					Processors:        intstr.FromString("0.25"),
					Network: IBMPowerVSResourceReference{
						Name: pointer.String("capi-net"),
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
			machine := tc.oldPowervsMachine.DeepCopy()
			machine.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-machine-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, machine); err != nil {
				t.Errorf("failed to create machine: %v", err)
			}
			machine.Spec = tc.newPowervsMachine.Spec
			if err := testEnv.Update(ctx, machine); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
