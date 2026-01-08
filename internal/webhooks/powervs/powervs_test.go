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

	"k8s.io/apimachinery/pkg/util/intstr"
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
