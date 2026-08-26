/*
Copyright 2024 The Kubernetes Authors.

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

package vpc

import (
	"testing"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"

	. "github.com/onsi/gomega"
)

func TestValidateIBMVPCClusterControlPlane(t *testing.T) {
	tests := []struct {
		name    string
		cluster *infrav1.IBMVPCCluster
		wantErr bool
	}{
		{
			name: "valid with legacy ControlPlaneLoadBalancer",
			cluster: &infrav1.IBMVPCCluster{
				Spec: infrav1.IBMVPCClusterSpec{
					ControlPlaneLoadBalancer: &infrav1.VPCLoadBalancerSpec{
						Name: "test-lb",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with ControlPlaneEndpoint",
			cluster: &infrav1.IBMVPCCluster{
				Spec: infrav1.IBMVPCClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "10.0.0.1",
						Port: 6443,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with network.loadBalancers (NLB path)",
			cluster: &infrav1.IBMVPCCluster{
				Spec: infrav1.IBMVPCClusterSpec{
					Network: &infrav1.VPCNetworkSpec{
						LoadBalancers: []infrav1.VPCLoadBalancerSpec{
							{Name: "test-nlb"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid with no endpoint or load balancer",
			cluster: &infrav1.IBMVPCCluster{
				Spec: infrav1.IBMVPCClusterSpec{
					Region:        "us-south",
					ResourceGroup: "rg",
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateIBMVPCClusterControlPlane(tc.cluster)
			if tc.wantErr {
				g.Expect(err).NotTo(BeNil())
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}
