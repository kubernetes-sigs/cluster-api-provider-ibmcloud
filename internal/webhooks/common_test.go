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

package webhooks

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

func TestValidateIBMPowerVSMemoryValues(t *testing.T) {
	type args struct {
		n int32
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "N is 4",
			args: args{n: 4},
			want: true,
		},
		{
			name: "N is 10",
			args: args{n: 10},
			want: true,
		},
		{
			name: "N is 1",
			args: args{n: 1},
			want: false,
		},
		{
			name: "N is -2",
			args: args{n: -2},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateIBMPowerVSMemoryValues(tt.args.n); got != tt.want {
				t.Errorf("validateIBMPowerVSMemoryValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateIBMPowerVSProcessorValues(t *testing.T) {
	type args struct {
		n intstr.IntOrString
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "N is 0.25",
			args: args{n: intstr.FromString("0.25")},
			want: true,
		},
		{
			name: "N is 0.5",
			args: args{n: intstr.FromString("0.5")},
			want: true,
		},
		{
			name: "N is 1",
			args: args{n: intstr.FromInt(1)},
			want: true,
		},
		{
			name: "N is 10",
			args: args{n: intstr.FromInt(10)},
			want: true,
		},
		{
			name: "N is 0.2",
			args: args{n: intstr.FromString("0.2")},
			want: false,
		},
		{
			name: "N is abc",
			args: args{n: intstr.FromString("abc")},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateIBMPowerVSProcessorValues(tt.args.n); got != tt.want {
				t.Errorf("validateIBMPowerVSProcessorValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateBootVolume(t *testing.T) {
	tests := []struct {
		name      string
		spec      infrav1.IBMVPCMachineSpec
		wantError bool
	}{
		{
			name: "Nil bootvolume",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: nil,
			},
			wantError: false,
		},
		{
			name: "valid sizeGiB",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{SizeGiB: 20},
			},
			wantError: false,
		},
		{
			name: "Invalid sizeGiB",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{SizeGiB: 1},
			},
			wantError: true,
		},
		{
			name: "Valid Iops",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{Iops: 1000, Profile: "custom"},
			},
			wantError: true,
		},
		{
			name: "Invalid Iops",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{Iops: 1234, Profile: "general-purpose"},
			},
			wantError: true,
		},
		{
			name: "Missing Iops for custom profile",
			spec: infrav1.IBMVPCMachineSpec{
				BootVolume: &infrav1.VPCVolume{Profile: "general-purpose"},
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBootVolume(tt.spec); (err != nil) != tt.wantError {
				t.Errorf("validateBootVolume() = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
