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

package powervs

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

func TestIBMPowerVSCluster_create(t *testing.T) {
	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErr        bool
	}{
		{
			// Network reference with exactly one identifier is valid.
			name: "Should allow if either Network ID or name is set",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// CRD CEL rule on ResourceIdentifier rejects both ID and Name being set.
			name: "Should error if both Network ID and name are set",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID:   "capi-net-id",
							Name: "capi-net",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			// CRD CEL rule on NetworkSource rejects Provision type for VirtualIP topology.
			name: "Should error if Network with Provision type when topology is VirtualIP",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{
								Name: "capi-dhcp",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			// CRD CEL rule on NetworkSource rejects reference being set when type is Provision.
			name: "Should error if Reference is set when Type is Provision",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeProvision,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{
								Name: "capi-dhcp",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			// CRD CEL rule on NetworkSource rejects provision being set when type is Reference.
			name: "Should error if Provision is set when Type is Reference",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
						Provision: infrav1.NetworkProvisionConfig{
							DHCPServer: infrav1.DHCPServer{
								Name: "capi-dhcp",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			// CRD CEL rule on ResourceIdentifier rejects a reference with no ID or Name.
			name: "Should error if neither Reference ID nor Name is set for Reference type",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.powervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIBMPowerVSCluster_update(t *testing.T) {
	tests := []struct {
		name              string
		oldPowervsCluster *infrav1.IBMPowerVSCluster
		newPowervsCluster *infrav1.IBMPowerVSCluster
		wantErr           bool
	}{
		{
			name: "Should allow if either Network ID or name is set",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// CRD CEL rule on ResourceIdentifier rejects both ID and Name being set.
			name: "Should error if both Network ID and name are set",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID:   "capi-net-id",
							Name: "capi-net-name",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if Network name is set",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "capi-net-name",
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "capi-net-name",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if the additionalListener selector is changed for same port and protocol",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23-changed",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if there is an additional listener added",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
									{
										Port:     25,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-25",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should allow if the additionalListener selector is updated with a new port and protocol",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     25,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-25",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// The old protocol-less key ("%d-<default>") is replaced by the typed listenerKey struct,
			// so an empty protocol is simply the zero value of LoadBalancerListenerProtocol — correct.
			name: "Should not panic with empty protocol in additionalListener",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: "",
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: "",
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"listener-selector": "port-23",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// Two load balancers that share port 23 must not have their selector
			// immutability check confused with each other.
			name: "Should allow selector change on lb-2 port 23 when lb-1 port 23 selector is unchanged",
			oldPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{"lb": "lb1-port23"},
										},
									},
								},
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-2",
								Type: infrav1.LoadBalancerTypePrivate,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{"lb": "lb2-port23-old"},
										},
									},
								},
							},
						},
					},
				},
			},
			newPowervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSLoadBalancerTopology,
					Zone:     "dal10",
					ResourceGroup: infrav1.ResourceGroupSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "rg-id",
						},
					},
					Workspace: infrav1.WorkspaceSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-si-id",
						},
					},
					Network: infrav1.NetworkSource{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							ID: "capi-net-id",
						},
					},
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-1",
								Type: infrav1.LoadBalancerTypePublic,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{"lb": "lb1-port23"},
										},
									},
								},
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "load-balancer-2",
								Type: infrav1.LoadBalancerTypePrivate,
								AdditionalListeners: []infrav1.AdditionalListener{
									{
										Port:     23,
										Protocol: infrav1.LoadBalancerListenerProtocolTCP,
										Selector: metav1.LabelSelector{
											MatchLabels: map[string]string{"lb": "lb2-port23-new"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.oldPowervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-",
				Namespace:    "default",
			}

			if err := testEnv.Create(ctx, cluster); err != nil {
				t.Errorf("failed to create cluster: %v", err)
			}

			cluster.Spec = tc.newPowervsCluster.Spec
			if err := testEnv.Update(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
