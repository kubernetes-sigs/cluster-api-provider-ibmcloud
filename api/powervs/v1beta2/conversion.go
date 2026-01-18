/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta2

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

func (src *IBMPowerVSCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSCluster)

	return Convert_v1beta2_IBMPowerVSCluster_To_v1beta3_IBMPowerVSCluster(src, dst, nil)
}

func (dst *IBMPowerVSCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSCluster)

	return Convert_v1beta3_IBMPowerVSCluster_To_v1beta2_IBMPowerVSCluster(src, dst, nil)
}

func (src *IBMPowerVSClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSClusterTemplate)

	return Convert_v1beta2_IBMPowerVSClusterTemplate_To_v1beta3_IBMPowerVSClusterTemplate(src, dst, nil)
}

func (dst *IBMPowerVSClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSClusterTemplate)

	return Convert_v1beta3_IBMPowerVSClusterTemplate_To_v1beta2_IBMPowerVSClusterTemplate(src, dst, nil)
}

func (src *IBMPowerVSMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSMachine)

	return Convert_v1beta2_IBMPowerVSMachine_To_v1beta3_IBMPowerVSMachine(src, dst, nil)
}

func (dst *IBMPowerVSMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSMachine)

	return Convert_v1beta3_IBMPowerVSMachine_To_v1beta2_IBMPowerVSMachine(src, dst, nil)
}

func (src *IBMPowerVSMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSMachineTemplate)

	return Convert_v1beta2_IBMPowerVSMachineTemplate_To_v1beta3_IBMPowerVSMachineTemplate(src, dst, nil)
}

func (dst *IBMPowerVSMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSMachineTemplate)

	return Convert_v1beta3_IBMPowerVSMachineTemplate_To_v1beta2_IBMPowerVSMachineTemplate(src, dst, nil)
}

func (src *IBMPowerVSImage) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSImage)

	return Convert_v1beta2_IBMPowerVSImage_To_v1beta3_IBMPowerVSImage(src, dst, nil)
}

func (dst *IBMPowerVSImage) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSImage)

	return Convert_v1beta3_IBMPowerVSImage_To_v1beta2_IBMPowerVSImage(src, dst, nil)
}
