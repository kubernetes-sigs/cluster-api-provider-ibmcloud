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

package vpc

import (
	"testing"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
)

func Test_validateVolumes(t *testing.T) {
	tests := []struct {
		name      string
		spec      infrav1.IBMVPCMachineSpec
		wantError bool
	}{
		{
			name: "Nil bootvolume for Boot Volume",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: nil,
			},
			wantError: false,
		},
		{
			name: "valid sizeGiB for Boot Volume",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{SizeGiB: 20},
			},
			wantError: false,
		},
		{
			name: "Invalid sizeGiB for Boot Volume",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{SizeGiB: 1},
			},
			wantError: true,
		},
		{
			name: "Missing Iops for custom profile for Additional Volume",
			spec: infrav1.IBMVPCMachineSpec{
				AdditionalVolumes: []*infrav1.VPCVolume{{Profile: "custom", SizeGiB: 20}},
			},
			wantError: true,
		},
		{
			name: "Missing SizeGiB for custom profile for Additional Volume",
			spec: infrav1.IBMVPCMachineSpec{
				AdditionalVolumes: []*infrav1.VPCVolume{{Profile: "custom", Iops: 4000}},
			},
			wantError: true,
		},
		{
			name: "Valid iops and sizeGiB for custom profile",
			spec: infrav1.IBMVPCMachineSpec{
				AdditionalVolumes: []*infrav1.VPCVolume{{Profile: "custom", SizeGiB: 25, Iops: 4000}},
			},
			wantError: false,
		},
		{
			name: "Valid encryptionKeyCRN",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{SizeGiB: 20, EncryptionKeyCRN: "crn:v1:bluemix:public:kms:us-south:a/aa2432b1fa4d4ace891e9b80fc104e34:e4a29d1a-2ef0-42a6-8fd2-350deb1c647e:key:5437653b-c4b1-447f-9646-b2a2a4cd6179"},
			},
			wantError: false,
		},
		{
			name: "Invalid encryptionKeyCRN",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{EncryptionKeyCRN: "invalid-crn-format"},
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateVolumes(tt.spec); (err != nil) != tt.wantError {
				t.Errorf("validateBootVolume() = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
