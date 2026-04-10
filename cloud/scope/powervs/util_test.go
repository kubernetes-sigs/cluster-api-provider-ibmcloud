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
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

const (
	usEastRegion = "us-east"
)

func TestFetchBucketRegion(t *testing.T) {
	testRegion := "us-south"
	vpcRegion := usEastRegion

	testcases := []struct {
		name           string
		cos            *infrav1.CosInstance
		vpc            *infrav1.VPCResourceReference
		expectedRegion string
	}{
		{
			name: "Returns bucket region from COS instance when set",
			cos: &infrav1.CosInstance{
				BucketRegion: testRegion,
			},
			vpc: &infrav1.VPCResourceReference{
				Region: ptr.To(vpcRegion),
			},
			expectedRegion: testRegion,
		},
		{
			name: "Returns VPC region when COS bucket region is not set",
			cos:  &infrav1.CosInstance{},
			vpc: &infrav1.VPCResourceReference{
				Region: ptr.To(vpcRegion),
			},
			expectedRegion: vpcRegion,
		},
		{
			name: "Returns VPC region when COS is nil",
			cos:  nil,
			vpc: &infrav1.VPCResourceReference{
				Region: ptr.To(vpcRegion),
			},
			expectedRegion: vpcRegion,
		},
		{
			name:           "Returns empty string when both COS bucket region and VPC region are not set",
			cos:            &infrav1.CosInstance{},
			vpc:            &infrav1.VPCResourceReference{},
			expectedRegion: "",
		},
		{
			name:           "Returns empty string when COS is nil and VPC region is not set",
			cos:            nil,
			vpc:            &infrav1.VPCResourceReference{},
			expectedRegion: "",
		},
		{
			name:           "Returns empty string when both COS and VPC are nil",
			cos:            nil,
			vpc:            nil,
			expectedRegion: "",
		},
		{
			name: "Returns empty string when COS bucket region is empty and VPC is nil",
			cos: &infrav1.CosInstance{
				BucketRegion: "",
			},
			vpc:            nil,
			expectedRegion: "",
		},
		{
			name: "Prioritizes COS bucket region over VPC region",
			cos: &infrav1.CosInstance{
				BucketRegion: testRegion,
			},
			vpc: &infrav1.VPCResourceReference{
				Region: ptr.To(vpcRegion),
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
