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

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

func TestValidateIBMPowerVSProcessorValues(t *testing.T) {
	tests := []struct {
		name       string
		procType   infrav1.PowerVSProcessorType
		processors intstr.IntOrString
		wantErr    bool
	}{
		// Shared / Capped valid cases
		{
			name:       "Shared: 0.25 is the minimum and valid",
			procType:   infrav1.PowerVSProcessorTypeShared,
			processors: intstr.FromString("0.25"),
			wantErr:    false,
		},
		{
			name:       "Shared: 0.5 is valid",
			procType:   infrav1.PowerVSProcessorTypeShared,
			processors: intstr.FromString("0.5"),
			wantErr:    false,
		},
		{
			name:       "Shared: integer 1 is valid",
			procType:   infrav1.PowerVSProcessorTypeShared,
			processors: intstr.FromInt32(1),
			wantErr:    false,
		},
		{
			name:       "Capped: 0.25 is valid",
			procType:   infrav1.PowerVSProcessorTypeCapped,
			processors: intstr.FromString("0.25"),
			wantErr:    false,
		},
		// Shared / Capped invalid cases
		{
			name:       "Shared: 0.2 is below the 0.25 minimum",
			procType:   infrav1.PowerVSProcessorTypeShared,
			processors: intstr.FromString("0.2"),
			wantErr:    true,
		},
		{
			name:       "Shared: non-numeric string is invalid",
			procType:   infrav1.PowerVSProcessorTypeShared,
			processors: intstr.FromString("abc"),
			wantErr:    true,
		},
		// Dedicated valid cases
		{
			name:       "Dedicated: 1 is the minimum and valid",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromInt32(1),
			wantErr:    false,
		},
		{
			name:       "Dedicated: 4 is valid",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromInt32(4),
			wantErr:    false,
		},
		{
			name:       "Dedicated: whole number as string is valid",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromString("2"),
			wantErr:    false,
		},
		// Dedicated invalid cases
		{
			name:       "Dedicated: 0.25 is below the minimum of 1",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromString("0.25"),
			wantErr:    true,
		},
		{
			name:       "Dedicated: 1.5 is fractional and not allowed",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromString("1.5"),
			wantErr:    true,
		},
		{
			name:       "Dedicated: non-numeric string is invalid",
			procType:   infrav1.PowerVSProcessorTypeDedicated,
			processors: intstr.FromString("abc"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIBMPowerVSProcessorValues(tt.procType, tt.processors)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIBMPowerVSProcessorValues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
