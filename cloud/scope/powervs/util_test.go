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

package powervs

import (
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

const (
	usEastRegion = "us-east"
)

func TestFetchBucketRegion(t *testing.T) {
	testRegion := region
	vpcRegion := usEastRegion

	testcases := []struct {
		name           string
		cos            infrav1.COSInstanceSource
		vpc            infrav1.VPCStatus
		expectedRegion string
	}{
		{
			name: "Returns bucket region from COS instance when set",
			cos: infrav1.COSInstanceSource{
				BucketRegion: testRegion,
			},
			vpc: infrav1.VPCStatus{
				Region: vpcRegion,
			},
			expectedRegion: testRegion,
		},
		{
			name: "Returns VPC region when COS bucket region is not set",
			cos:  infrav1.COSInstanceSource{},
			vpc: infrav1.VPCStatus{
				Region: vpcRegion,
			},
			expectedRegion: vpcRegion,
		},
		{
			name: "Returns VPC region when COS is empty",
			cos:  infrav1.COSInstanceSource{},
			vpc: infrav1.VPCStatus{
				Region: vpcRegion,
			},
			expectedRegion: vpcRegion,
		},
		{
			name:           "Returns empty string when both COS bucket region and VPC region are not set",
			cos:            infrav1.COSInstanceSource{},
			vpc:            infrav1.VPCStatus{},
			expectedRegion: "",
		},
		{
			name:           "Returns empty string when COS and VPC are empty",
			cos:            infrav1.COSInstanceSource{},
			vpc:            infrav1.VPCStatus{},
			expectedRegion: "",
		},
		{
			name: "Returns empty string when COS bucket region is empty and VPC is empty",
			cos: infrav1.COSInstanceSource{
				BucketRegion: "",
			},
			vpc:            infrav1.VPCStatus{},
			expectedRegion: "",
		},
		{
			name: "Prioritizes COS bucket region over VPC region",
			cos: infrav1.COSInstanceSource{
				BucketRegion: testRegion,
			},
			vpc: infrav1.VPCStatus{
				Region: vpcRegion,
			},
			expectedRegion: testRegion,
		},
	}

	for _, tc := range testcases {
		g := NewWithT(t)
		t.Run(tc.name, func(_ *testing.T) {
			region := fetchBucketRegion(tc.cos, tc.vpc)
			g.Expect(region).To(Equal(tc.expectedRegion))
		})
	}
}

func TestNormalizedVPCSecurityGroupRulePrototype(t *testing.T) {
	t.Run("When prototype is nil", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(normalizedVPCSecurityGroupRulePrototype(nil)).To(BeNil())
	})
	t.Run("When protocol is not deprecated, the prototype is returned unchanged", func(t *testing.T) {
		g := NewWithT(t)
		prototype := &infrav1.VPCSecurityGroupRulePrototype{
			Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
		}
		g.Expect(normalizedVPCSecurityGroupRulePrototype(prototype)).To(BeIdenticalTo(prototype))
	})
	t.Run("When protocol is deprecated 'all', a normalized copy is returned and the input is not mutated", func(t *testing.T) {
		g := NewWithT(t)
		prototype := &infrav1.VPCSecurityGroupRulePrototype{
			Protocol: infrav1.VPCSecurityGroupRuleProtocolAll,
			Remotes: []infrav1.VPCSecurityGroupRuleRemote{
				{RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny},
			},
		}
		normalized := normalizedVPCSecurityGroupRulePrototype(prototype)
		g.Expect(normalized).ToNot(BeIdenticalTo(prototype))
		g.Expect(normalized.Protocol).To(Equal(infrav1.VPCSecurityGroupRuleProtocolIcmpTCPUDP))
		g.Expect(normalized.Remotes).To(Equal(prototype.Remotes))
		g.Expect(prototype.Protocol).To(Equal(infrav1.VPCSecurityGroupRuleProtocolAll))
	})
}
