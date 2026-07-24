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
	"strconv"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

const (
	defaultSystemType = "s922"
)

func defaultIBMPowerVSMachineSpec(spec *infrav1.IBMPowerVSMachineSpec) {
	if spec.MemoryGiB == 0 {
		spec.MemoryGiB = 2
	}
	if spec.Processors.StrVal == "" && spec.Processors.IntVal == 0 {
		spec.Processors = intstr.FromString("0.25")
	}
	if spec.SystemType == "" {
		spec.SystemType = defaultSystemType
	}
	if spec.ProcessorType == "" {
		spec.ProcessorType = infrav1.PowerVSProcessorTypeShared
	}
}

func validateIBMPowerVSNetworkReference(res infrav1.ResourceIdentifier) (bool, *field.Error) {
	count := 0
	if res.ID != "" {
		count++
	}
	if res.Name != "" {
		count++
	}
	if count > 1 {
		return false, field.Invalid(field.NewPath("spec", "Network"), res, "Only one of Network - ID or Name can be specified")
	}
	return true, nil
}

func validateIBMPowerVSMemoryValues(resValue int32) bool {
	if val := float64(resValue); val < 2 {
		return false
	}
	return true
}

func validateIBMPowerVSProcessorValues(resValue intstr.IntOrString) bool {
	switch resValue.Type {
	case intstr.Int:
		if val := float64(resValue.IntVal); val < 0.25 {
			return false
		}
	case intstr.String:
		if val, err := strconv.ParseFloat(resValue.StrVal, 64); err != nil || val < 0.25 {
			return false
		}
	}

	return true
}
