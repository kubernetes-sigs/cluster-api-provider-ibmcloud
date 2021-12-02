/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha4

import (
	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *IBMPowerVSCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSCluster)

	return Convert_v1alpha4_IBMPowerVSCluster_To_v1beta1_IBMPowerVSCluster(src, dst, nil)
}

func (dst *IBMPowerVSCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSCluster)

	return Convert_v1beta1_IBMPowerVSCluster_To_v1alpha4_IBMPowerVSCluster(src, dst, nil)
}

func (src *IBMPowerVSClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSClusterList)

	return Convert_v1alpha4_IBMPowerVSClusterList_To_v1beta1_IBMPowerVSClusterList(src, dst, nil)
}

func (dst *IBMPowerVSClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSClusterList)

	return Convert_v1beta1_IBMPowerVSClusterList_To_v1alpha4_IBMPowerVSClusterList(src, dst, nil)
}

func (src *IBMPowerVSMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSMachine)

	return Convert_v1alpha4_IBMPowerVSMachine_To_v1beta1_IBMPowerVSMachine(src, dst, nil)
}

func (dst *IBMPowerVSMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSMachine)

	return Convert_v1beta1_IBMPowerVSMachine_To_v1alpha4_IBMPowerVSMachine(src, dst, nil)
}

func (src *IBMPowerVSMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSMachineList)

	return Convert_v1alpha4_IBMPowerVSMachineList_To_v1beta1_IBMPowerVSMachineList(src, dst, nil)
}

func (dst *IBMPowerVSMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSMachineList)

	return Convert_v1beta1_IBMPowerVSMachineList_To_v1alpha4_IBMPowerVSMachineList(src, dst, nil)
}

func (src *IBMPowerVSMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSMachineTemplate)

	return Convert_v1alpha4_IBMPowerVSMachineTemplate_To_v1beta1_IBMPowerVSMachineTemplate(src, dst, nil)
}

func (dst *IBMPowerVSMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSMachineTemplate)

	return Convert_v1beta1_IBMPowerVSMachineTemplate_To_v1alpha4_IBMPowerVSMachineTemplate(src, dst, nil)
}

func (src *IBMPowerVSMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.IBMPowerVSMachineTemplateList)

	return Convert_v1alpha4_IBMPowerVSMachineTemplateList_To_v1beta1_IBMPowerVSMachineTemplateList(src, dst, nil)
}

func (dst *IBMPowerVSMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.IBMPowerVSMachineTemplateList)

	return Convert_v1beta1_IBMPowerVSMachineTemplateList_To_v1alpha4_IBMPowerVSMachineTemplateList(src, dst, nil)
}
