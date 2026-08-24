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

package powervs

import (
	"context"
	"testing"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSClusterTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		newTemplate *infrav1.IBMPowerVSClusterTemplate
		oldTemplate *infrav1.IBMPowerVSClusterTemplate
		wantErr     bool
	}{
		{
			name: "IBMPowerVSClusterTemplate with immutable spec",
			newTemplate: &infrav1.IBMPowerVSClusterTemplate{
				Spec: infrav1.IBMPowerVSClusterTemplateSpec{
					Template: infrav1.IBMPowerVSClusterTemplateResource{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "test-instance1",
								},
							},
						},
					},
				},
			},
			oldTemplate: &infrav1.IBMPowerVSClusterTemplate{
				Spec: infrav1.IBMPowerVSClusterTemplateSpec{
					Template: infrav1.IBMPowerVSClusterTemplateResource{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "test-instance1",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: " IBMPowerVSClusterTemplate with mutable spec",
			newTemplate: &infrav1.IBMPowerVSClusterTemplate{
				Spec: infrav1.IBMPowerVSClusterTemplateSpec{
					Template: infrav1.IBMPowerVSClusterTemplateResource{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "test-instance1",
								},
							},
						},
					},
				},
			},
			oldTemplate: &infrav1.IBMPowerVSClusterTemplate{
				Spec: infrav1.IBMPowerVSClusterTemplateSpec{
					Template: infrav1.IBMPowerVSClusterTemplateResource{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "test-instance2",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			ibmPowerVSClusterTemplate := IBMPowerVSClusterTemplate{}
			_, err := ibmPowerVSClusterTemplate.ValidateUpdate(ctx, test.oldTemplate, test.newTemplate)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestIBMPowerVSClusterTemplate_Default(t *testing.T) {
	g := NewWithT(t)
	template := &infrav1.IBMPowerVSClusterTemplate{
		Spec: infrav1.IBMPowerVSClusterTemplateSpec{
			Template: infrav1.IBMPowerVSClusterTemplateResource{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "test-instance",
						},
					},
				},
			},
		},
	}
	// Default is a no-op; it should succeed and not modify the object.
	g.Expect((&IBMPowerVSClusterTemplate{}).Default(context.Background(), template)).To(Succeed())
	g.Expect(template.Spec.Template.Spec.Topology).To(Equal(infrav1.PowerVSVirtualIPTopology))
}

func TestIBMPowerVSClusterTemplate_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	template := &infrav1.IBMPowerVSClusterTemplate{
		Spec: infrav1.IBMPowerVSClusterTemplateSpec{
			Template: infrav1.IBMPowerVSClusterTemplateResource{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "test-instance",
						},
					},
				},
			},
		},
	}
	// ValidateCreate is a no-op; it should always succeed.
	w := IBMPowerVSClusterTemplate{}
	warnings, err := w.ValidateCreate(ctx, template)
	g.Expect(warnings).To(BeNil())
	g.Expect(err).NotTo(HaveOccurred())
}

func TestIBMPowerVSClusterTemplate_ValidateDelete(t *testing.T) {
	g := NewWithT(t)
	template := &infrav1.IBMPowerVSClusterTemplate{
		Spec: infrav1.IBMPowerVSClusterTemplateSpec{
			Template: infrav1.IBMPowerVSClusterTemplateResource{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "test-instance",
						},
					},
				},
			},
		},
	}
	// ValidateDelete is a no-op; it should always succeed.
	w := IBMPowerVSClusterTemplate{}
	warnings, err := w.ValidateDelete(ctx, template)
	g.Expect(warnings).To(BeNil())
	g.Expect(err).NotTo(HaveOccurred())
}
