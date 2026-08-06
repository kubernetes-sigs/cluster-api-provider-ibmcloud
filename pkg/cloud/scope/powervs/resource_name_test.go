/*
Copyright 2025 The Kubernetes Authors.

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
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestResourceName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		rt          ResourceType
		qualifier   string
		want        string
	}{
		{
			name:        "workspace without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeWorkspace,
			want:        "my-cluster-workspace",
		},
		{
			name:        "transit gateway without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeTransitGateway,
			want:        "my-cluster-transitgateway",
		},
		{
			name:        "VPC without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeVPC,
			want:        "my-cluster-vpc",
		},
		{
			name:        "subnet with zone qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeSubnet,
			qualifier:   "us-south-1",
			want:        "my-cluster-subnet-us-south-1",
		},
		{
			name:        "subnet with index qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeSubnet,
			qualifier:   "2",
			want:        "my-cluster-subnet-2",
		},
		{
			name:        "public LB without qualifier (default, index 0)",
			clusterName: "my-cluster",
			rt:          ResourceTypeLBPublic,
			want:        "my-cluster-loadbalancer-public",
		},
		{
			name:        "public LB with index qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeLBPublic,
			qualifier:   "1",
			want:        "my-cluster-loadbalancer-public-1",
		},
		{
			name:        "private LB without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeLBPrivate,
			want:        "my-cluster-loadbalancer-private",
		},
		{
			name:        "security group without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeSG,
			want:        "my-cluster-sg",
		},
		{
			name:        "DHCP server without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeDHCP,
			want:        "my-cluster-dhcp",
		},
		{
			name:        "COS instance without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeCOS,
			want:        "my-cluster-cos",
		},
		{
			name:        "COS bucket without qualifier",
			clusterName: "my-cluster",
			rt:          ResourceTypeCOSBucket,
			want:        "my-cluster-cos-bucket",
		},
		{
			// suffix "workspace" = 9 chars; separator = 1; maxBase = 63-9-1 = 53
			// cluster name of 60 chars must be truncated to 53
			name:        "long cluster name is truncated, suffix preserved",
			clusterName: strings.Repeat("a", 60),
			rt:          ResourceTypeWorkspace,
			want:        strings.Repeat("a", 53) + "-workspace",
		},
		{
			// suffix "transitgateway" = 14; separator = 1; maxBase = 48
			name:        "long cluster name with long suffix preserved",
			clusterName: strings.Repeat("b", 60),
			rt:          ResourceTypeTransitGateway,
			want:        strings.Repeat("b", 48) + "-transitgateway",
		},
		{
			// result must never exceed 63 chars
			name:        "result length never exceeds 63",
			clusterName: strings.Repeat("c", 100),
			rt:          ResourceTypeVPC,
			want:        strings.Repeat("c", 59) + "-vpc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			got := ResourceName(tc.clusterName, tc.rt, tc.qualifier)
			g.Expect(got).To(Equal(tc.want))
			g.Expect(len(got)).To(BeNumerically("<=", resourceNameMaxLen),
				"generated name %q exceeds %d character limit", got, resourceNameMaxLen)
		})
	}
}
