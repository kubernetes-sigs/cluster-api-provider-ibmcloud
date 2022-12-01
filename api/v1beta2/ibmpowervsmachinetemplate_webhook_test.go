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
					Memory:     "4",
					Processors: "0.5",
					Image: &IBMPowerVSResourceReference{
						ID: pointer.String("capi-image"),
					},
				},
			},
		},
	}
	t.Run("Defaults for IBMPowerVSMachineTemplate", defaulting.DefaultValidateTest(powervsMachineTemplate))
	powervsMachineTemplate.Default()
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.SysType).To(BeEquivalentTo("s922"))
	g.Expect(powervsMachineTemplate.Spec.Template.Spec.ProcType).To(BeEquivalentTo("shared"))
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
							SysType:           "a890",
							ProcType:          "unknown",
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
							SysType:           "s922",
							ProcType:          "shared",
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
							SysType:           "s922",
							ProcType:          "shared",
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
							SysType:           "s922",
							ProcType:          "shared",
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
							SysType:           "s922",
							ProcType:          "shared",
							Network: IBMPowerVSResourceReference{
								Name: pointer.String("capi-net"),
							},
							Image: &IBMPowerVSResourceReference{
								Name: pointer.String("capi-image"),
							},
							Processors: "two",
							Memory:     "four",
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
							SysType:           "s922",
							ProcType:          "shared",
							Network: IBMPowerVSResourceReference{
								Name: pointer.String("capi-net"),
							},
							Image: &IBMPowerVSResourceReference{
								ID: pointer.String("capi-image-id"),
							},
							Processors: "0.25",
							Memory:     "4",
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
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid SysType",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
						Spec: IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "w112",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
			name: "Should fail to update IBMPowerVSMachineTemplate with invalid ProcType",
			oldPowervsMachineTemplate: &IBMPowerVSMachineTemplate{
				Spec: IBMPowerVSMachineTemplateSpec{
					Template: IBMPowerVSMachineTemplateResource{
						Spec: IBMPowerVSMachineSpec{
							ServiceInstanceID: "capi-si-id",
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "e980",
							ProcType:          "invalid",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "eight",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "two",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "4",
							Processors:        "0.25",
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
							SysType:           "s922",
							ProcType:          "shared",
							Memory:            "8",
							Processors:        "2",
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
