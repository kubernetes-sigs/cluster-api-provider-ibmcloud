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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIBMPowerVSCluster_create(t *testing.T) {
	tests := []struct {
		name           string
		powervsCluster *IBMPowerVSCluster
		wantErr        bool
	}{
		{
			name: "Should allow if either Network ID or name is set",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID: ptr.To("capi-net-id"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if both Network ID and name are set",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID:   ptr.To("capi-net-id"),
						Name: ptr.To("capi-net"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should error if all Network ID, name and regex are set",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID:    ptr.To("capi-net-id"),
						Name:  ptr.To("capi-net"),
						RegEx: ptr.To("^capi$"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should error network regex and dhcp server name is set but does not match dhcp server name",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("test"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if network regex, dhcp server name is set and matches dhcp server name",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if only network regex is set, dhcp server name is not set and does not match cluster name",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if network regex is set, dhcp server name is not set and matches cluster name",
			powervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi-cluster-.*"),
					},
				},
			},
			wantErr: false,
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
		oldPowervsCluster *IBMPowerVSCluster
		newPowervsCluster *IBMPowerVSCluster
		wantErr           bool
	}{
		{
			name: "Should allow if either Network ID or name is set",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID: ptr.To("capi-net-id"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID: ptr.To("capi-net-id"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if both Network ID and name are set",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID: ptr.To("capi-net-id"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID:   ptr.To("capi-net-id"),
						Name: ptr.To("capi-net-name"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if Network ID is set",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi-net-id$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi-net-id"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi-net-id$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi-net-id"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if all Network ID, name and regex are set",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID: ptr.To("capi-net-id"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						ID:    ptr.To("capi-net-id"),
						Name:  ptr.To("capi-net-name"),
						RegEx: ptr.To("^capi-net-id$"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should error if network regex and dhcp server name is set but network regex does not match dhcp server name",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("test"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if network regex, dhcp server name is set and network regex matches dhcp server name",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Should error if network regex is set, dhcp server name is not set and network regex does not match cluster name",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi",
				},
			},
			wantErr: true,
		},
		{
			name: "Should allow if network regex is set, dhcp server name is not set and network regex matches cluster name",
			oldPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi$"),
					},
					DHCPServer: &DHCPServer{
						Name: ptr.To("capi"),
					},
				},
			},
			newPowervsCluster: &IBMPowerVSCluster{
				Spec: IBMPowerVSClusterSpec{
					ServiceInstanceID: "capi-si-id",
					Network: IBMPowerVSResourceReference{
						RegEx: ptr.To("^capi-cluster-.*"),
					},
				},
			},
			wantErr: false,
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
