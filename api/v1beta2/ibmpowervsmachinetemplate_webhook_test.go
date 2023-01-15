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

func TestIBMPowerVSMachineTemplate_default(t *testing.T) {
	g := NewWithT(t)
	powervsMachineTemplate := &IBMPowerVSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-machine-template",
			Namespace: "default",
		},
		Spec: IBMPowerVSMachineTemplateSpec{
			Template: IBMPowerVSMachineTemplateResource{
				Spec: IBMPowerVSMachineSpec{
					MemoryGiB:  4,
					Processors: intstr.FromString("0.5"),
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image"),
					},
				},
			},
		},
	}
	t.Run("Defaults for IBMPowerVSMachineTemplate", defaulting.DefaultValidateTest(powervsMachineTemplate))
	powervsMachineTemplate.Default()
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.SystemType).To(BeEquivalentTo("s922"))
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.ProcessorType).To(BeEquivalentTo(PowerVSProcessorTypeShared))
}

func TestIBMPowerVSMachineTemplate_create(t *testing.T) {
	tests := []struct {
		name                   string
		powervsMachineTemplate *IBMPowerVSMachineTemplate
		wantErr                bool
	}{
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - incorrect spec values",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - no Image or Imagref in Spec",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
						Spec: IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     PowerVSProcessorTypeShared,
							Network: IBMPowerVSResourceReference{
								Name: pointer.String("capi-net"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - both Image and Imagref specified in Spec",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - Both ID and Name specified for Image",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - invalid memory and processor values",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully validate IBMPowerVSMachineTemplate - valid spec",
			powervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := tc.powervsMachineTemplate.DeepCopy()
			template.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-template-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, template); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIBMPowerVSMachineTemplate_update(t *testing.T) {
	tests := []struct {
		name                      string
		oldPowervsMachineTemplate *IBMPowerVSMachineTemplate
		newPowervsMachineTemplate *IBMPowerVSMachineTemplate
		wantErr                   bool
	}{
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid ProcessorType",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid Network",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid Image",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid memory",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid processors",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully update IBMPowerVSMachineTemplate",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
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
				},
			},
			newPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
						Spec: IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     PowerVSProcessorTypeShared,
							MemoryGiB:         8,
							Processors:        intstr.FromInt(2),
							Network: IBMPowerVSResourceReference{
								Name: pointer.String("capi-net"),
							},
							Image: &IBMPowerVSResourceReference{
								ID: pointer.String("capi-image-id"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := tc.oldPowervsMachineTemplate.DeepCopy()
			template.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-template-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, template); err != nil {
				t.Errorf("failed to create template: %v", err)
			}
			template.Spec = tc.newPowervsMachineTemplate.Spec
			if err := testEnv.Update(ctx, template); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
