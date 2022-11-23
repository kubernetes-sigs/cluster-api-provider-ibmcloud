/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha3

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
)

func Convert_v1alpha3_APIEndpoint_To_v1beta1_APIEndpoint(in *capiv1alpha3.APIEndpoint, out *capiv1beta1.APIEndpoint, s apiconversion.Scope) error {
	return capiv1alpha3.Convert_v1alpha3_APIEndpoint_To_v1beta1_APIEndpoint(in, out, s)
}

func Convert_v1beta1_APIEndpoint_To_v1alpha3_APIEndpoint(in *capiv1beta1.APIEndpoint, out *capiv1alpha3.APIEndpoint, s apiconversion.Scope) error {
	return capiv1alpha3.Convert_v1beta1_APIEndpoint_To_v1alpha3_APIEndpoint(in, out, s)
}

func (src *IBMVPCCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCCluster)

	return Convert_v1alpha3_IBMVPCCluster_To_v1beta1_IBMVPCCluster(src, dst, nil)
}

func (dst *IBMVPCCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCCluster)

	return Convert_v1beta1_IBMVPCCluster_To_v1alpha3_IBMVPCCluster(src, dst, nil)
}

func (src *IBMVPCClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCClusterList)

	return Convert_v1alpha3_IBMVPCClusterList_To_v1beta1_IBMVPCClusterList(src, dst, nil)
}

func (dst *IBMVPCClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCClusterList)

	return Convert_v1beta1_IBMVPCClusterList_To_v1alpha3_IBMVPCClusterList(src, dst, nil)
}

func (src *IBMVPCMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCMachine)

	if err := Convert_v1alpha3_IBMVPCMachine_To_v1beta1_IBMVPCMachine(src, dst, nil); err != nil {
		return err
	}

	for _, sshKey := range src.Spec.SSHKeys {
		dst.Spec.SSHKeysRef = append(dst.Spec.SSHKeysRef, &infrav1beta1.IBMVPCResourceReference{
			ID: sshKey,
		})
	}

	dst.Spec.ImageRef = &infrav1beta1.IBMVPCResourceReference{
		ID: &src.Spec.Image,
	}

	return nil
}

func (dst *IBMVPCMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCMachine)

	if err := Convert_v1beta1_IBMVPCMachine_To_v1alpha3_IBMVPCMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	if src.Spec.SSHKeysRef != nil {
		for _, sshKey := range src.Spec.SSHKeysRef {
			if sshKey.ID != nil {
				// Only source keys with ID will be converted
				dst.Spec.SSHKeys = append(dst.Spec.SSHKeys, sshKey.ID)
			}
		}
	}

	if src.Spec.ImageRef != nil && src.Spec.ImageRef.ID != nil {
		// Only source image with ID will be converted
		dst.Spec.Image = *src.Spec.ImageRef.ID
	}

	return nil
}

func (src *IBMVPCMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCMachineList)

	return Convert_v1alpha3_IBMVPCMachineList_To_v1beta1_IBMVPCMachineList(src, dst, nil)
}

func (dst *IBMVPCMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCMachineList)

	return Convert_v1beta1_IBMVPCMachineList_To_v1alpha3_IBMVPCMachineList(src, dst, nil)
}

func (src *IBMVPCMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCMachineTemplate)

	if err := Convert_v1alpha3_IBMVPCMachineTemplate_To_v1beta1_IBMVPCMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	for _, sshKey := range src.Spec.Template.Spec.SSHKeys {
		dst.Spec.Template.Spec.SSHKeysRef = append(dst.Spec.Template.Spec.SSHKeysRef, &infrav1beta1.IBMVPCResourceReference{
			ID: sshKey,
		})
	}

	dst.Spec.Template.Spec.ImageRef = &infrav1beta1.IBMVPCResourceReference{
		ID: &src.Spec.Template.Spec.Image,
	}

	return nil
}

func (dst *IBMVPCMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCMachineTemplate)

	if err := Convert_v1beta1_IBMVPCMachineTemplate_To_v1alpha3_IBMVPCMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	if src.Spec.Template.Spec.SSHKeysRef != nil {
		for _, sshKey := range src.Spec.Template.Spec.SSHKeysRef {
			if sshKey.ID != nil {
				// Only source keys with ID will be converted
				dst.Spec.Template.Spec.SSHKeys = append(dst.Spec.Template.Spec.SSHKeys, sshKey.ID)
			}
		}
	}

	if src.Spec.Template.Spec.ImageRef != nil && src.Spec.Template.Spec.ImageRef.ID != nil {
		// Only source image with ID will be converted
		dst.Spec.Template.Spec.Image = *src.Spec.Template.Spec.ImageRef.ID
	}

	return nil
}

func (src *IBMVPCMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.IBMVPCMachineTemplateList)

	return Convert_v1alpha3_IBMVPCMachineTemplateList_To_v1beta1_IBMVPCMachineTemplateList(src, dst, nil)
}

func (dst *IBMVPCMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.IBMVPCMachineTemplateList)

	return Convert_v1beta1_IBMVPCMachineTemplateList_To_v1alpha3_IBMVPCMachineTemplateList(src, dst, nil)
}

// Convert_v1beta1_IBMVPCClusterSpec_To_v1alpha3_IBMVPCClusterSpec is an autogenerated conversion function.
// Requires manual conversion as ControlPlaneLoadBalancer does not exist in v1alpha4 version of IBMVPCClusterSpec.
func Convert_v1beta1_IBMVPCClusterSpec_To_v1alpha3_IBMVPCClusterSpec(in *infrav1beta1.IBMVPCClusterSpec, out *IBMVPCClusterSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_IBMVPCClusterSpec_To_v1alpha3_IBMVPCClusterSpec(in, out, s)
}

// Convert_v1beta1_IBMVPCClusterStatus_To_v1alpha3_IBMVPCClusterStatus is an autogenerated conversion function.
// Requires manual conversion as ControlPlaneLoadBalancerState and Conditions does not exist in v1alpha4 version of IBMVPCClusterStatus.
func Convert_v1beta1_IBMVPCClusterStatus_To_v1alpha3_IBMVPCClusterStatus(in *infrav1beta1.IBMVPCClusterStatus, out *IBMVPCClusterStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_IBMVPCClusterStatus_To_v1alpha3_IBMVPCClusterStatus(in, out, s)
}

// Convert_v1beta1_VPCEndpoint_To_v1alpha3_VPCEndpoint is an autogenerated conversion function.
// Requires manual conversion as LBID does not exist in v1alpha4 version of VPCEndpoint.
func Convert_v1beta1_VPCEndpoint_To_v1alpha3_VPCEndpoint(in *infrav1beta1.VPCEndpoint, out *VPCEndpoint, s apiconversion.Scope) error {
	return autoConvert_v1beta1_VPCEndpoint_To_v1alpha3_VPCEndpoint(in, out, s)
}

func Convert_v1beta1_IBMVPCMachineSpec_To_v1alpha3_IBMVPCMachineSpec(in *infrav1beta1.IBMVPCMachineSpec, out *IBMVPCMachineSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_IBMVPCMachineSpec_To_v1alpha3_IBMVPCMachineSpec(in, out, s)
}

func Convert_v1alpha3_IBMVPCMachineSpec_To_v1beta1_IBMVPCMachineSpec(in *IBMVPCMachineSpec, out *infrav1beta1.IBMVPCMachineSpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_IBMVPCMachineSpec_To_v1beta1_IBMVPCMachineSpec(in, out, s)
}
