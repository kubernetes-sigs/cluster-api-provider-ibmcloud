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

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSMachineTemplate_default(t *testing.T) {
	g := NewWithT(t)
	powervsMachineTemplate := &infrav1beta2.IBMPowerVSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-machine-template",
			Namespace: "default",
		},
		Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
			Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
				Spec: infrav1beta2.IBMPowerVSMachineSpec{
					MemoryGiB:  4,
					Processors: intstr.FromString("0.5"),
					Image: &infrav1beta2.IBMPowerVSResourceReference{
						ID: ptr.To("capi-image"),
					},
				},
			},
		},
	}
	g.Expect((&IBMPowerVSMachineTemplate{}).Default(context.Background(), powervsMachineTemplate)).ToNot(HaveOccurred())
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.SystemType).To(BeEquivalentTo("s922"))
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.ProcessorType).To(BeEquivalentTo(infrav1beta2.PowerVSProcessorTypeShared))
}

func TestIBMPowerVSMachineTemplate_create(t *testing.T) {
	tests := []struct {
		name                   string
		powervsMachineTemplate *infrav1beta2.IBMPowerVSMachineTemplate
		wantErr                bool
	}{
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - incorrect spec values",
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "a890",
							ProcessorType:     "unknown",
							Network: infrav1beta2.IBMPowerVSResourceReference{
								ID:   ptr.To("capi-net-id"),
								Name: ptr.To("capi-net"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - no Image or Imagref in Spec",
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - both Image and Imagref specified in Spec",
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image:    &infrav1beta2.IBMPowerVSResourceReference{},
							ImageRef: &corev1.LocalObjectReference{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - Both ID and Name specified for Image",
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID:   ptr.To("capi-image-id"),
								Name: ptr.To("capi-image"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to validate IBMPowerVSMachineTemplate - invalid memory and processor values",
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-image"),
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
			powervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
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
		oldPowervsMachineTemplate *infrav1beta2.IBMPowerVSMachineTemplate
		newPowervsMachineTemplate *infrav1beta2.IBMPowerVSMachineTemplate
		wantErr                   bool
	}{
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid ProcessorType",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "e980",
							ProcessorType:     "invalid",
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid Network",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
								ID:   ptr.To("capi-net-ID"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid Image",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID:   ptr.To("capi-image-id"),
								Name: ptr.To("capi-image"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid memory",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         int32(-8),
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid processors",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("two"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should successfully update IBMPowerVSMachineTemplate",
			oldPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         4,
							Processors:        intstr.FromString("0.25"),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
							},
						},
					},
				},
			},
			newPowervsMachineTemplate: &infrav1beta2.IBMPowerVSMachineTemplate{
				Spec: infrav1beta2.IBMPowerVSMachineTemplateSpec{
					Template: infrav1beta2.IBMPowerVSMachineTemplateResource{
						Spec: infrav1beta2.IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SystemType:        "s922",
							ProcessorType:     infrav1beta2.PowerVSProcessorTypeShared,
							MemoryGiB:         8,
							Processors:        intstr.FromInt(2),
							Network: infrav1beta2.IBMPowerVSResourceReference{
								Name: ptr.To("capi-net"),
							},
							Image: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("capi-image-id"),
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
