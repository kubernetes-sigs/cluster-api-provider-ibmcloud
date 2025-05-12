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
	"strconv"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

func defaultIBMPowerVSMachineSpec(spec *infrav1beta2.IBMPowerVSMachineSpec) {
	if spec.MemoryGiB == 0 {
		spec.MemoryGiB = 2
	}
	if spec.Processors.StrVal == "" && spec.Processors.IntVal == 0 {
		spec.Processors = intstr.FromString("0.25")
	}
	if spec.SystemType == "" {
		spec.SystemType = "s922"
	}
	if spec.ProcessorType == "" {
		spec.ProcessorType = infrav1beta2.PowerVSProcessorTypeShared
	}
}

func validateIBMPowerVSResourceReference(res infrav1beta2.IBMPowerVSResourceReference, resType string) (bool, *field.Error) {
	if res.ID != nil && res.Name != nil {
		return false, field.Invalid(field.NewPath("spec", resType), res, "Only one of "+resType+" - ID or Name may be specified")
	}
	return true, nil
}

func validateIBMPowerVSNetworkReference(res infrav1beta2.IBMPowerVSResourceReference) (bool, *field.Error) {
	if (res.ID != nil && res.Name != nil) || (res.ID != nil && res.RegEx != nil) || (res.Name != nil && res.RegEx != nil) {
		return false, field.Invalid(field.NewPath("spec", "Network"), res, "Only one of Network - ID, Name or RegEx can be specified")
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

func defaultIBMVPCMachineSpec(spec *infrav1beta2.IBMVPCMachineSpec) {
	if spec.Profile == "" {
		spec.Profile = "bx2-2x8"
	}
}

func validateBootVolume(spec infrav1beta2.IBMVPCMachineSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.BootVolume == nil {
		return allErrs
	}

	if spec.BootVolume.SizeGiB < 10 || spec.BootVolume.SizeGiB > 250 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.bootVolume.sizeGiB"), spec, "valid Boot VPCVolume size is 10 - 250 GB"))
	}

	if spec.BootVolume.Iops != 0 && spec.BootVolume.Profile != "custom" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.bootVolume.iops"), spec, "iops applicable only to volumes using a profile of type `custom`"))
	}

	//TODO: Add validation for the spec.BootVolume.EncryptionKeyCRN to ensure its in proper IBM Cloud CRN format

	return allErrs
}
