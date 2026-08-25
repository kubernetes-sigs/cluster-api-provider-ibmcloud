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

	. "github.com/onsi/gomega"
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
			// Two load balancers that share port 23 — the immutability check must
			// be scoped per load balancer name, not just per port. Changing lb-2's
			// selector on port 23 must still be rejected even though lb-1's port-23
			// selector is unchanged.
			name: "Should error on selector change for lb-2 port 23 even when lb-1 port 23 selector is unchanged",
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

func TestIBMPowerVSCluster_ValidateDelete(t *testing.T) {
	g := NewWithT(t)
	// Create the cluster through testEnv so the full webhook admission path is
	// exercised on delete, consistent with the other create/update tests.
	cluster := &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "capi-cluster-del-",
			Namespace:    "default",
		},
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
	}
	g.Expect(testEnv.Create(ctx, cluster)).To(Succeed())
	// ValidateDelete is a no-op; deletion must always succeed.
	g.Expect(testEnv.Delete(ctx, cluster)).To(Succeed())
}

func TestIBMPowerVSCluster_Default(t *testing.T) {
	g := NewWithT(t)
	cluster := &infrav1.IBMPowerVSCluster{
		Spec: infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSVirtualIPTopology,
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
			},
		},
	}
	// Default is a no-op for IBMPowerVSCluster; verify it does not mutate the
	// object (Topology remains as set) and returns no error.
	g.Expect((&IBMPowerVSCluster{}).Default(ctx, cluster)).To(Succeed())
	g.Expect(cluster.Spec.Topology).To(Equal(infrav1.PowerVSVirtualIPTopology))
}

// TestIBMPowerVSClusterLoadBalancers exercises the webhook-layer load balancer
// validation paths that are not reachable through the testEnv create/update tests.
func TestIBMPowerVSClusterLoadBalancers(t *testing.T) {
	// baseSpec returns a minimal LoadBalancer-topology cluster spec. Each test
	// case sets only the LoadBalancers field on top of this base.
	baseSpec := func() infrav1.IBMPowerVSClusterSpec {
		return infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Zone:     "dal10",
			ResourceGroup: infrav1.ResourceGroupSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
			},
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-net-id"},
			},
			VPC: infrav1.VPCSource{
				Type:   infrav1.SourceTypeProvision,
				Region: "us-south",
				Provision: infrav1.VPCProvision{
					Name: "capi-vpc",
				},
			},
		}
	}

	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErr        bool
	}{
		{
			// A provision-type load balancer with no name set — the name-deduplication
			// code skips it (continue branch in validateIBMPowerVSClusterLoadBalancerNames).
			name: "Should allow provision load balancer with empty name (skips duplicate check)",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				// Public LB with no name → the name deduplication skips it (continue branch).
				spec.LoadBalancers = []infrav1.LoadBalancerSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							// Name intentionally left empty — exercises the `name == ""` continue branch.
							Type: infrav1.LoadBalancerTypePublic,
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			// Only a private load balancer → should fail: no public LB.
			name: "Should error when only a private load balancer is configured",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.LoadBalancers = []infrav1.LoadBalancerSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "private-lb",
							Type: infrav1.LoadBalancerTypePrivate,
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
		{
			// At least one public load balancer → should succeed.
			name: "Should allow when at least one public load balancer is configured",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.LoadBalancers = []infrav1.LoadBalancerSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "public-lb",
							Type: infrav1.LoadBalancerTypePublic,
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			// Duplicate load balancer names (Provision type) → should fail.
			name: "Should error when duplicate provision load balancer names exist",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.LoadBalancers = []infrav1.LoadBalancerSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "duplicate-lb",
							Type: infrav1.LoadBalancerTypePublic,
						},
					},
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "duplicate-lb",
							Type: infrav1.LoadBalancerTypePrivate,
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
		{
			// Duplicate load balancer names using Reference type → should fail.
			name: "Should error when duplicate reference load balancer names exist",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.LoadBalancers = []infrav1.LoadBalancerSource{
					{
						// First LB is a Provision public LB to satisfy the "at least one public" check.
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.LoadBalancerProvision{
							Name: "public-lb",
							Type: infrav1.LoadBalancerTypePublic,
						},
					},
					{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "ref-lb"},
					},
					{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{Name: "ref-lb"},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.powervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-lb-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestIBMPowerVSClusterVPCSubnetNames exercises the VPC subnet name validation
// paths, including duplicate names for both Provision and Reference subnet types.
func TestIBMPowerVSClusterVPCSubnetNames(t *testing.T) {
	// baseSpec returns a base LoadBalancer-topology cluster spec, ready for subnet customisation.
	baseSpec := func() infrav1.IBMPowerVSClusterSpec {
		return infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Zone:     "dal10",
			ResourceGroup: infrav1.ResourceGroupSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
			},
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-net-id"},
			},
			VPC: infrav1.VPCSource{
				Type:   infrav1.SourceTypeProvision,
				Region: "us-south",
				Provision: infrav1.VPCProvision{
					Name: "capi-vpc",
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{
				{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.LoadBalancerProvision{
						Name: "public-lb",
						Type: infrav1.LoadBalancerTypePublic,
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErr        bool
	}{
		{
			name: "Should allow unique Provision subnet names",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.VPCSubnets = []infrav1.VPCSubnetSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{
							Name: "subnet-1",
						},
					},
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{
							Name: "subnet-2",
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			name: "Should error on duplicate Provision subnet names",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.VPCSubnets = []infrav1.VPCSubnetSource{
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{
							Name: "subnet-dup",
						},
					},
					{
						Type: infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{
							Name: "subnet-dup",
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
		{
			name: "Should error on duplicate Reference subnet names",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.VPCSubnets = []infrav1.VPCSubnetSource{
					{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "subnet-ref",
						},
					},
					{
						Type: infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{
							Name: "subnet-ref",
						},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
		{
			name: "Should allow subnets where the name is empty (no duplicate check)",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec()
				spec.VPCSubnets = []infrav1.VPCSubnetSource{
					{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{},
					},
					{
						Type:      infrav1.SourceTypeProvision,
						Provision: infrav1.VPCSubnetProvision{},
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.powervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-subnet-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestIBMPowerVSClusterTransitGateway exercises the webhook-layer transit gateway
// validation paths that are not covered by the existing test cases.
func TestIBMPowerVSClusterTransitGateway(t *testing.T) {
	// baseSpec returns a LoadBalancer-topology spec suitable for transit gateway testing.
	baseSpec := func(zone, vpcRegion string) infrav1.IBMPowerVSClusterSpec {
		return infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Zone:     zone,
			ResourceGroup: infrav1.ResourceGroupSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
			},
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-net-id"},
			},
			VPC: infrav1.VPCSource{
				Type:   infrav1.SourceTypeProvision,
				Region: vpcRegion,
				Provision: infrav1.VPCProvision{
					Name: "capi-vpc",
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{
				{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.LoadBalancerProvision{
						Name: "public-lb",
						Type: infrav1.LoadBalancerTypePublic,
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErr        bool
	}{
		{
			// No TransitGateway type set → skip check entirely.
			name: "Should allow when transitGateway type is not set",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec("dal10", "us-south")
				// TransitGateway is zero-value → Type == ""
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			// TransitGateway type is Reference → no routing check performed.
			name: "Should allow when transitGateway type is Reference",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec("dal10", "us-south")
				spec.TransitGateway = infrav1.TransitGatewaySource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "tg-id",
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			// TransitGateway type is Provision, same region (dal10 → us-south, VPC us-south) →
			// global routing not required, so Local is fine.
			name: "Should allow Provision TransitGateway with Local routing in same region",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec("dal10", "us-south")
				spec.TransitGateway = infrav1.TransitGatewaySource{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.TransitGatewayProvision{
						GlobalRouting: infrav1.TransitGatewayRoutingLocal,
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
		{
			// TransitGateway type is Provision, different regions (dal10 → us-south, VPC eu-de) →
			// global routing IS required; using Local routing must fail.
			name: "Should error when Provision TransitGateway uses Local routing across regions",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec("dal10", "eu-de")
				spec.TransitGateway = infrav1.TransitGatewaySource{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.TransitGatewayProvision{
						GlobalRouting: infrav1.TransitGatewayRoutingLocal,
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: true,
		},
		{
			// TransitGateway type is Provision, different regions, but using Global routing → allowed.
			name: "Should allow Provision TransitGateway with Global routing across regions",
			powervsCluster: func() *infrav1.IBMPowerVSCluster {
				spec := baseSpec("dal10", "eu-de")
				spec.TransitGateway = infrav1.TransitGatewaySource{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.TransitGatewayProvision{
						GlobalRouting: infrav1.TransitGatewayRoutingGlobal,
					},
				}
				return &infrav1.IBMPowerVSCluster{Spec: spec}
			}(),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.powervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-tg-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestIBMPowerVSClusterInfraPrereq exercises the zone and VPC-region
// validation paths inside validateIBMPowerVSClusterCreateInfraPrereq.
func TestIBMPowerVSClusterInfraPrereq(t *testing.T) {
	minimalLBSpec := func(zone, vpcRegion string) infrav1.IBMPowerVSClusterSpec {
		return infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Zone:     zone,
			ResourceGroup: infrav1.ResourceGroupSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "rg-id"},
			},
			Workspace: infrav1.WorkspaceSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
			},
			Network: infrav1.NetworkSource{
				Type:      infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "capi-net-id"},
			},
			VPC: infrav1.VPCSource{
				Type:   infrav1.SourceTypeProvision,
				Region: vpcRegion,
				Provision: infrav1.VPCProvision{
					Name: "capi-vpc",
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{
				{
					Type: infrav1.SourceTypeProvision,
					Provision: infrav1.LoadBalancerProvision{
						Name: "public-lb",
						Type: infrav1.LoadBalancerTypePublic,
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErr        bool
	}{
		{
			name: "Should allow valid zone and VPC region",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: minimalLBSpec("dal10", "us-south"),
			},
			wantErr: false,
		},
		{
			name: "Should error when zone is invalid",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: minimalLBSpec("invalid-zone", "us-south"),
			},
			wantErr: true,
		},
		{
			name: "Should error when VPC region is invalid",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: minimalLBSpec("dal10", "invalid-region"),
			},
			wantErr: true,
		},
		{
			// VirtualIP topology skips prereq validation entirely.
			name: "Should skip prereq validation for VirtualIP topology",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
					Workspace: infrav1.WorkspaceSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "capi-si-id"},
					},
					Network: infrav1.NetworkSource{
						Type:      infrav1.SourceTypeReference,
						Reference: infrav1.ResourceIdentifier{ID: "capi-net-id"},
					},
					Zone: "invalid-zone",
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := tc.powervsCluster.DeepCopy()
			cluster.ObjectMeta = metav1.ObjectMeta{
				GenerateName: "capi-cluster-prereq-",
				Namespace:    "default",
			}
			if err := testEnv.Create(ctx, cluster); (err != nil) != tc.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestValidateIBMPowerVSClusterLoadBalancerNames directly tests the load balancer
// name deduplication helper, including the `continue` branch when name is empty.
func TestValidateIBMPowerVSClusterLoadBalancerNames(t *testing.T) {
	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErrs       int
	}{
		{
			name: "No LBs — returns empty error list",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{},
			},
			wantErrs: 0,
		},
		{
			name: "LBs with empty names — continue branch; no duplicate error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							// Provision.Name is "" → name stays "" → continue
							Provision: infrav1.LoadBalancerProvision{
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
					},
				},
			},
			wantErrs: 0,
		},
		{
			name: "LBs with unique Provision names — no duplicate error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "lb-a",
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "lb-b",
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
					},
				},
			},
			wantErrs: 0,
		},
		{
			name: "LBs with duplicate Provision names — duplicate error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "same-lb",
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "same-lb",
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "LBs with duplicate Reference names — duplicate error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "ref-lb"},
						},
						{
							Type:      infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{Name: "ref-lb"},
						},
					},
				},
			},
			wantErrs: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			errs := validateIBMPowerVSClusterLoadBalancerNames(tc.powervsCluster)
			g.Expect(errs).To(HaveLen(tc.wantErrs))
		})
	}
}

// TestValidateIBMPowerVSClusterLoadBalancers directly tests the load balancer
// validation helper, including the early-return path when no LBs are configured.
func TestValidateIBMPowerVSClusterLoadBalancers(t *testing.T) {
	tests := []struct {
		name           string
		powervsCluster *infrav1.IBMPowerVSCluster
		wantErrs       int
	}{
		{
			// len(LoadBalancers) == 0 → early return, no errors.
			name: "No LBs configured — early return with no error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{},
				},
			},
			wantErrs: 0,
		},
		{
			// At least one Provision Public LB → allowed, early return.
			name: "One public Provision LB — allowed",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "public-lb",
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
					},
				},
			},
			wantErrs: 0,
		},
		{
			// All LBs are private → must error.
			name: "Only private Provision LBs — error: no public LB",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "private-lb",
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
					},
				},
			},
			wantErrs: 1,
		},
		{
			// Duplicate provision names → duplicate error.
			name: "Duplicate provision LB names — duplicate error",
			powervsCluster: &infrav1.IBMPowerVSCluster{
				Spec: infrav1.IBMPowerVSClusterSpec{
					LoadBalancers: []infrav1.LoadBalancerSource{
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "dup-lb",
								Type: infrav1.LoadBalancerTypePublic,
							},
						},
						{
							Type: infrav1.SourceTypeProvision,
							Provision: infrav1.LoadBalancerProvision{
								Name: "dup-lb",
								Type: infrav1.LoadBalancerTypePrivate,
							},
						},
					},
				},
			},
			// The first entry is Public so there is no "no public LB" error, but
			// the duplicate name still produces exactly 1 error.
			wantErrs: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			errs := validateIBMPowerVSClusterLoadBalancers(tc.powervsCluster)
			g.Expect(errs).To(HaveLen(tc.wantErrs))
		})
	}
}
