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

package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

func (src *IBMVPCCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCCluster)

	return Convert_v1beta1_IBMVPCCluster_To_v1beta2_IBMVPCCluster(src, dst, nil)
}

func (dst *IBMVPCCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCCluster)

	return Convert_v1beta2_IBMVPCCluster_To_v1beta1_IBMVPCCluster(src, dst, nil)
}

func (src *IBMVPCClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCClusterList)

	return Convert_v1beta1_IBMVPCClusterList_To_v1beta2_IBMVPCClusterList(src, dst, nil)
}

func (dst *IBMVPCClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCClusterList)

	return Convert_v1beta2_IBMVPCClusterList_To_v1beta1_IBMVPCClusterList(src, dst, nil)
}

func (src *IBMVPCMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCMachine)

	return Convert_v1beta1_IBMVPCMachine_To_v1beta2_IBMVPCMachine(src, dst, nil)
}

func (dst *IBMVPCMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCMachine)

	return Convert_v1beta2_IBMVPCMachine_To_v1beta1_IBMVPCMachine(src, dst, nil)
}

func (src *IBMVPCMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCMachineList)

	return Convert_v1beta1_IBMVPCMachineList_To_v1beta2_IBMVPCMachineList(src, dst, nil)
}

func (dst *IBMVPCMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCMachineList)

	return Convert_v1beta2_IBMVPCMachineList_To_v1beta1_IBMVPCMachineList(src, dst, nil)
}

func (src *IBMVPCMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCMachineTemplate)

	return Convert_v1beta1_IBMVPCMachineTemplate_To_v1beta2_IBMVPCMachineTemplate(src, dst, nil)
}

func (dst *IBMVPCMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCMachineTemplate)

	return Convert_v1beta2_IBMVPCMachineTemplate_To_v1beta1_IBMVPCMachineTemplate(src, dst, nil)
}

func (src *IBMVPCMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.IBMVPCMachineTemplateList)

	return Convert_v1beta1_IBMVPCMachineTemplateList_To_v1beta2_IBMVPCMachineTemplateList(src, dst, nil)
}

func (dst *IBMVPCMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.IBMVPCMachineTemplateList)

	return Convert_v1beta2_IBMVPCMachineTemplateList_To_v1beta1_IBMVPCMachineTemplateList(src, dst, nil)
}
