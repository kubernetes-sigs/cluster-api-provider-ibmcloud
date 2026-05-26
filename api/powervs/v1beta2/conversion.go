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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
)

func Convert_v1beta2_IBMPowerVSClusterStatus_To_v1beta3_IBMPowerVSClusterStatus(in *IBMPowerVSClusterStatus, out *infrav1.IBMPowerVSClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSClusterStatus_To_v1beta3_IBMPowerVSClusterStatus(in, out, s); err != nil {
		return err
	}

	// Manual conversion for Status: ServiceInstance -> Workspace.
	// v1beta2 ResourceReference only has an ID, no Name.
	if in.ServiceInstance != nil && in.ServiceInstance.ID != nil {
		out.Workspace.ID = *in.ServiceInstance.ID
	}

	// Manual conversion for Status: ResourceGroup.
	// v1beta2 ResourceReference only has an ID, no Name.
	if in.ResourceGroup != nil && in.ResourceGroup.ID != nil {
		out.ResourceGroup.ID = *in.ResourceGroup.ID
	}

	// Convert Network status
	if in.Network != nil && in.Network.ID != nil {
		out.Network.ID = *in.Network.ID
	}

	// Convert DHCPServer status
	if in.DHCPServer != nil && in.DHCPServer.ID != nil {
		out.Network.DHCPServer.ID = *in.DHCPServer.ID
	}

	out.Conditions = nil
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}
	if in.Conditions == nil {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &infrav1.IBMPowerVSClusterDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta2 == nil {
		out.Deprecated.V1Beta2 = &infrav1.IBMPowerVSClusterV1Beta2DeprecatedStatus{}
	}
	clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta2.Conditions)
	return nil
}

func Convert_v1beta3_IBMPowerVSClusterStatus_To_v1beta2_IBMPowerVSClusterStatus(in *infrav1.IBMPowerVSClusterStatus, out *IBMPowerVSClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSClusterStatus_To_v1beta2_IBMPowerVSClusterStatus(in, out, s); err != nil {
		return err
	}

	// Manual conversion for Status: Workspace -> ServiceInstance.
	if in.Workspace.ID != "" {
		out.ServiceInstance = &ResourceReference{
			ID: ptr.To(in.Workspace.ID),
		}
	} else {
		out.ServiceInstance = nil
	}

	// Manual conversion for Status: ResourceGroup.
	if in.ResourceGroup.ID != "" {
		out.ResourceGroup = &ResourceReference{
			ID: ptr.To(in.ResourceGroup.ID),
		}
	} else {
		out.ResourceGroup = nil
	}

	// Convert Network status
	if in.Network.ID != "" || in.Network.Name != "" {
		out.Network = &ResourceReference{}
		if in.Network.ID != "" {
			out.Network.ID = ptr.To(in.Network.ID)
		}
	}

	// Convert DHCPServer status
	if in.Network.DHCPServer.ID != "" || in.Network.DHCPServer.Name != "" {
		out.DHCPServer = &ResourceReference{}
		if in.Network.DHCPServer.ID != "" {
			out.DHCPServer.ID = ptr.To(in.Network.DHCPServer.ID)
		}
	}

	out.Conditions = nil
	if in.Deprecated != nil && in.Deprecated.V1Beta2 != nil && in.Deprecated.V1Beta2.Conditions != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta2.Conditions, &out.Conditions)
	}

	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &IBMPowerVSClusterV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta2_IBMPowerVSMachineStatus_To_v1beta3_IBMPowerVSMachineStatus(in *IBMPowerVSMachineStatus, out *infrav1.IBMPowerVSMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSMachineStatus_To_v1beta3_IBMPowerVSMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = nil
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}
	if in.Conditions == nil {
		return nil
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1.IBMPowerVSMachineDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta2 == nil {
		out.Deprecated.V1Beta2 = &infrav1.IBMPowerVSMachineV1Beta2DeprecatedStatus{}
	}
	clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta2.Conditions)
	return nil
}

func Convert_v1beta3_IBMPowerVSMachineStatus_To_v1beta2_IBMPowerVSMachineStatus(in *infrav1.IBMPowerVSMachineStatus, out *IBMPowerVSMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSMachineStatus_To_v1beta2_IBMPowerVSMachineStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = nil
	if in.Deprecated != nil && in.Deprecated.V1Beta2 != nil && in.Deprecated.V1Beta2.Conditions != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta2.Conditions, &out.Conditions)
	}
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &IBMPowerVSMachineV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta2_IBMPowerVSImageStatus_To_v1beta3_IBMPowerVSImageStatus(in *IBMPowerVSImageStatus, out *infrav1.IBMPowerVSImageStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSImageStatus_To_v1beta3_IBMPowerVSImageStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = nil
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}
	if in.Conditions == nil {
		return nil
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1.IBMPowerVSImageDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta2 == nil {
		out.Deprecated.V1Beta2 = &infrav1.IBMPowerVSImageV1Beta2DeprecatedStatus{}
	}
	clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta2.Conditions)
	return nil
}

func Convert_v1beta3_IBMPowerVSImageStatus_To_v1beta2_IBMPowerVSImageStatus(in *infrav1.IBMPowerVSImageStatus, out *IBMPowerVSImageStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSImageStatus_To_v1beta2_IBMPowerVSImageStatus(in, out, s); err != nil {
		return err
	}
	out.Conditions = nil
	if in.Deprecated != nil && in.Deprecated.V1Beta2 != nil && in.Deprecated.V1Beta2.Conditions != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta2.Conditions, &out.Conditions)
	}
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &IBMPowerVSImageV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1_Condition_To_v1beta1_Condition(in *metav1.Condition, out *clusterv1beta1.Condition, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1_Condition_To_v1beta1_Condition(in, out, s)
}

func Convert_v1beta1_Condition_To_v1_Condition(in *clusterv1beta1.Condition, out *metav1.Condition, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_Condition_To_v1_Condition(in, out, s)
}

func Convert_v1beta3_IBMPowerVSMachineTemplateResource_To_v1beta2_IBMPowerVSMachineTemplateResource(in *infrav1.IBMPowerVSMachineTemplateResource, out *IBMPowerVSMachineTemplateResource, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSMachineTemplateResource_To_v1beta2_IBMPowerVSMachineTemplateResource(in, out, s); err != nil {
		return err
	}
	// Network.RegEx is not preserved during round-trip conversion
	// because v1beta3 ResourceIdentifier doesn't support RegEx.
	out.Spec.Network.RegEx = nil

	// Ensure empty string pointers are converted to nil
	if out.Spec.Network.ID != nil && *out.Spec.Network.ID == "" {
		out.Spec.Network.ID = nil
	}
	if out.Spec.Network.Name != nil && *out.Spec.Network.Name == "" {
		out.Spec.Network.Name = nil
	}
	return nil
}

func Convert_v1beta1_APIEndpoint_To_v1beta3_APIEndpoint(in *clusterv1beta1.APIEndpoint, out *infrav1.APIEndpoint, _ apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = in.Port
	return nil
}

func Convert_v1beta3_APIEndpoint_To_v1beta1_APIEndpoint(in *infrav1.APIEndpoint, out *clusterv1beta1.APIEndpoint, _ apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = in.Port
	return nil
}

func Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in *clusterv1beta1.ObjectMeta, out *clusterv1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in, out, s)
}

func Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1.ObjectMeta, out *clusterv1beta1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in, out, s)
}

func (src *IBMPowerVSCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSCluster)
	if err := Convert_v1beta2_IBMPowerVSCluster_To_v1beta3_IBMPowerVSCluster(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.IBMPowerVSCluster{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}
	initialization := infrav1.IBMPowerVSClusterInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.IBMPowerVSClusterInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}

	// If the old v1beta2 annotation is true, map it to LoadBalancer. Otherwise, VirtualIP.
	if val, exists := src.Annotations["powervs.cluster.x-k8s.io/create-infra"]; exists && val == "true" {
		dst.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
	} else {
		dst.Spec.Topology = infrav1.PowerVSVirtualIPTopology
	}

	// Clean up the annotation in v1beta3 so we don't have duplicated sources of truth
	if dst.Annotations != nil {
		delete(dst.Annotations, "powervs.cluster.x-k8s.io/create-infra")
		if len(dst.Annotations) == 0 {
			dst.Annotations = nil
		}
	}

	return nil
}

func (dst *IBMPowerVSCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSCluster)
	if err := Convert_v1beta3_IBMPowerVSCluster_To_v1beta2_IBMPowerVSCluster(src, dst, nil); err != nil {
		return err
	}

	// Map the v1beta3 Topology explicit enum back to the v1beta2 annotation
	if src.Spec.Topology == infrav1.PowerVSLoadBalancerTopology {
		if dst.Annotations == nil {
			dst.Annotations = make(map[string]string)
		}
		dst.Annotations["powervs.cluster.x-k8s.io/create-infra"] = "true"
	} else if dst.Annotations != nil {
		// For VirtualIP, we ensure the annotation is removed
		delete(dst.Annotations, "powervs.cluster.x-k8s.io/create-infra")
	}

	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	// Fix annotation discrepancy during round-trip
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	return nil
}

func (src *IBMPowerVSClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSClusterTemplate)
	if err := Convert_v1beta2_IBMPowerVSClusterTemplate_To_v1beta3_IBMPowerVSClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.IBMPowerVSClusterTemplate{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}
	if ok {
		// Restore any fields that were lost in conversion
		dst.Spec = restored.Spec
	}
	return nil
}

func (dst *IBMPowerVSClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSClusterTemplate)
	if err := Convert_v1beta3_IBMPowerVSClusterTemplate_To_v1beta2_IBMPowerVSClusterTemplate(src, dst, nil); err != nil {
		return err
	}
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	if dst.Spec.Template.Spec.ResourceGroup != nil &&
		dst.Spec.Template.Spec.ResourceGroup.ID == nil &&
		dst.Spec.Template.Spec.ResourceGroup.Name == nil &&
		dst.Spec.Template.Spec.ResourceGroup.RegEx == nil {
		dst.Spec.Template.Spec.ResourceGroup = nil
	}
	return nil
}

func (src *IBMPowerVSMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSMachine)
	if err := Convert_v1beta2_IBMPowerVSMachine_To_v1beta3_IBMPowerVSMachine(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.IBMPowerVSMachine{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}
	initialization := infrav1.IBMPowerVSMachineInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.IBMPowerVSMachineInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

func (dst *IBMPowerVSMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSMachine)
	if err := Convert_v1beta3_IBMPowerVSMachine_To_v1beta2_IBMPowerVSMachine(src, dst, nil); err != nil {
		return err
	}
	if dst.Spec.ProviderID != nil && *dst.Spec.ProviderID == "" {
		dst.Spec.ProviderID = nil
	}
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	return nil
}

func (src *IBMPowerVSMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSMachineTemplate)
	if err := Convert_v1beta2_IBMPowerVSMachineTemplate_To_v1beta3_IBMPowerVSMachineTemplate(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.IBMPowerVSMachineTemplate{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}
	if ok {
		dst.Status = restored.Status
	}
	return nil
}

func (dst *IBMPowerVSMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSMachineTemplate)
	if err := Convert_v1beta3_IBMPowerVSMachineTemplate_To_v1beta2_IBMPowerVSMachineTemplate(src, dst, nil); err != nil {
		return err
	}
	if dst.Spec.Template.Spec.ProviderID != nil && *dst.Spec.Template.Spec.ProviderID == "" {
		dst.Spec.Template.Spec.ProviderID = nil
	}
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	// Network.RegEx is not preserved during round-trip conversion
	// because v1beta3 ResourceIdentifier doesn't support RegEx.
	// Clear it after MarshalData to ensure it's not restored.
	dst.Spec.Template.Spec.Network.RegEx = nil
	return nil
}

func (src *IBMPowerVSImage) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.IBMPowerVSImage)
	if err := Convert_v1beta2_IBMPowerVSImage_To_v1beta3_IBMPowerVSImage(src, dst, nil); err != nil {
		return err
	}
	restored := &infrav1.IBMPowerVSImage{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}
	if ok {
		// Restore any fields that were lost in conversion
		dst.Spec = restored.Spec
	}
	return nil
}

func (dst *IBMPowerVSImage) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.IBMPowerVSImage)
	if err := Convert_v1beta3_IBMPowerVSImage_To_v1beta2_IBMPowerVSImage(src, dst, nil); err != nil {
		return err
	}
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	return nil
}

func Convert_v1beta2_IBMPowerVSClusterSpec_To_v1beta3_IBMPowerVSClusterSpec(in *IBMPowerVSClusterSpec, out *infrav1.IBMPowerVSClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSClusterSpec_To_v1beta3_IBMPowerVSClusterSpec(in, out, s); err != nil {
		return err
	}

	convertV1beta2WorkspaceToV1beta3(in, out)
	convertV1beta2ResourceGroupToV1beta3(in, out)

	if err := convertV1beta2NetworkToV1beta3(in, out); err != nil {
		return err
	}

	return nil
}

func convertV1beta2WorkspaceToV1beta3(in *IBMPowerVSClusterSpec, out *infrav1.IBMPowerVSClusterSpec) {
	if in.ServiceInstance == nil && in.ServiceInstanceID == "" {
		out.Workspace.Type = infrav1.SourceTypeProvision
		return
	}

	out.Workspace.Type = infrav1.SourceTypeReference
	if in.ServiceInstance != nil {
		if in.ServiceInstance.ID != nil {
			out.Workspace.Reference.ID = *in.ServiceInstance.ID
		}
		if in.ServiceInstance.Name != nil {
			out.Workspace.Reference.Name = *in.ServiceInstance.Name
		}
	}
	if in.ServiceInstanceID != "" {
		out.Workspace.Reference.ID = in.ServiceInstanceID
	}
}

func convertV1beta2ResourceGroupToV1beta3(in *IBMPowerVSClusterSpec, out *infrav1.IBMPowerVSClusterSpec) {
	if in.ResourceGroup == nil {
		return
	}

	out.ResourceGroup.Type = infrav1.SourceTypeReference
	if in.ResourceGroup.ID != nil {
		out.ResourceGroup.Reference.ID = *in.ResourceGroup.ID
	}
	if in.ResourceGroup.Name != nil {
		out.ResourceGroup.Reference.Name = *in.ResourceGroup.Name
	}
}

func convertV1beta2NetworkToV1beta3(in *IBMPowerVSClusterSpec, out *infrav1.IBMPowerVSClusterSpec) error {
	if (in.Network.ID != nil && *in.Network.ID != "") || (in.Network.Name != nil && *in.Network.Name != "") {
		out.Network.Type = infrav1.SourceTypeReference
		if in.Network.ID != nil && *in.Network.ID != "" {
			out.Network.Reference.ID = *in.Network.ID
		}
		if in.Network.Name != nil && *in.Network.Name != "" {
			out.Network.Reference.Name = *in.Network.Name
		}
		return nil
	}

	if in.DHCPServer == nil {
		return nil
	}

	out.Network.Type = infrav1.SourceTypeProvision
	return Convert_v1beta2_DHCPServer_To_v1beta3_DHCPServer(in.DHCPServer, &out.Network.Provision.DHCPServer, nil)
}

func Convert_v1beta3_IBMPowerVSClusterSpec_To_v1beta2_IBMPowerVSClusterSpec(in *infrav1.IBMPowerVSClusterSpec, out *IBMPowerVSClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSClusterSpec_To_v1beta2_IBMPowerVSClusterSpec(in, out, s); err != nil {
		return err
	}

	if in.Zone == "" {
		out.Zone = nil
	}

	if in.Workspace.Type == infrav1.SourceTypeReference || in.Workspace.Reference.ID != "" || in.Workspace.Reference.Name != "" {
		out.ServiceInstance = &IBMPowerVSResourceReference{}
		if in.Workspace.Reference.ID != "" {
			out.ServiceInstance.ID = ptr.To(in.Workspace.Reference.ID)
			out.ServiceInstanceID = in.Workspace.Reference.ID
		}
		if in.Workspace.Reference.Name != "" {
			out.ServiceInstance.Name = ptr.To(in.Workspace.Reference.Name)
		}
	} else {
		out.ServiceInstance = nil
		out.ServiceInstanceID = ""
	}

	if in.ResourceGroup.Type == infrav1.SourceTypeReference || in.ResourceGroup.Reference.ID != "" || in.ResourceGroup.Reference.Name != "" {
		out.ResourceGroup = &IBMPowerVSResourceReference{}
		if in.ResourceGroup.Reference.ID != "" {
			out.ResourceGroup.ID = ptr.To(in.ResourceGroup.Reference.ID)
		}
		if in.ResourceGroup.Reference.Name != "" {
			out.ResourceGroup.Name = ptr.To(in.ResourceGroup.Reference.Name)
		}
	} else {
		out.ResourceGroup = nil
	}

	// Convert Network field
	switch in.Network.Type {
	case infrav1.SourceTypeReference:
		// Convert reference
		if in.Network.Reference.ID != "" {
			out.Network.ID = ptr.To(in.Network.Reference.ID)
		}
		if in.Network.Reference.Name != "" {
			out.Network.Name = ptr.To(in.Network.Reference.Name)
		}
		out.DHCPServer = nil
	case infrav1.SourceTypeProvision:
		// Convert provision to DHCPServer
		dhcp := &DHCPServer{}
		if err := Convert_v1beta3_DHCPServer_To_v1beta2_DHCPServer(&in.Network.Provision.DHCPServer, dhcp, nil); err != nil {
			return err
		}
		out.DHCPServer = dhcp
		// Clear network reference when provisioning
		out.Network.ID = nil
		out.Network.Name = nil
	default:
		// Empty NetworkSource - clear both fields
		out.Network.ID = nil
		out.Network.Name = nil
		out.DHCPServer = nil
	}
	// Note: v1beta2 Network.RegEx field is dropped in v1beta3, cannot be restored
	out.Network.RegEx = nil

	return nil
}

func Convert_v1beta2_IBMPowerVSMachineSpec_To_v1beta3_IBMPowerVSMachineSpec(in *IBMPowerVSMachineSpec, out *infrav1.IBMPowerVSMachineSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSMachineSpec_To_v1beta3_IBMPowerVSMachineSpec(in, out, s); err != nil {
		return err
	}

	if in.ImageRef != nil && in.ImageRef.Name != "" {
		out.ImageRef = infrav1.ImageReference{Name: in.ImageRef.Name}
	}

	// Machine uses ResourceIdentifier, NO Type/Provision fields!
	if in.ServiceInstance != nil {
		if in.ServiceInstance.ID != nil {
			out.Workspace.ID = *in.ServiceInstance.ID
		}
		if in.ServiceInstance.Name != nil {
			out.Workspace.Name = *in.ServiceInstance.Name
		}
	} else if in.ServiceInstanceID != "" {
		out.Workspace.ID = in.ServiceInstanceID
	}

	return nil
}

func Convert_v1beta3_IBMPowerVSMachineSpec_To_v1beta2_IBMPowerVSMachineSpec(in *infrav1.IBMPowerVSMachineSpec, out *IBMPowerVSMachineSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSMachineSpec_To_v1beta2_IBMPowerVSMachineSpec(in, out, s); err != nil {
		return err
	}

	if in.ImageRef.Name != "" {
		out.ImageRef = &corev1.LocalObjectReference{Name: in.ImageRef.Name}
	} else {
		out.ImageRef = nil
	}

	if in.Workspace.ID != "" || in.Workspace.Name != "" {
		out.ServiceInstance = &IBMPowerVSResourceReference{}
		if in.Workspace.ID != "" {
			out.ServiceInstance.ID = ptr.To(in.Workspace.ID)
			out.ServiceInstanceID = in.Workspace.ID
		}
		if in.Workspace.Name != "" {
			out.ServiceInstance.Name = ptr.To(in.Workspace.Name)
		}
	} else {
		out.ServiceInstance = nil
		out.ServiceInstanceID = ""
	}

	// Network.RegEx is not preserved during round-trip conversion
	// because v1beta3 ResourceIdentifier doesn't support RegEx.
	// Set RegEx to nil to ensure clean conversion.
	out.Network.RegEx = nil

	return nil
}

func Convert_v1beta2_IBMPowerVSImageSpec_To_v1beta3_IBMPowerVSImageSpec(in *IBMPowerVSImageSpec, out *infrav1.IBMPowerVSImageSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_IBMPowerVSImageSpec_To_v1beta3_IBMPowerVSImageSpec(in, out, s); err != nil {
		return err
	}

	// Image uses ResourceIdentifier, NO Type/Provision fields!
	if in.ServiceInstance != nil {
		if in.ServiceInstance.ID != nil {
			out.Workspace.ID = *in.ServiceInstance.ID
		}
		if in.ServiceInstance.Name != nil {
			out.Workspace.Name = *in.ServiceInstance.Name
		}
	} else if in.ServiceInstanceID != "" {
		out.Workspace.ID = in.ServiceInstanceID
	}

	return nil
}

func Convert_v1beta3_IBMPowerVSImageSpec_To_v1beta2_IBMPowerVSImageSpec(in *infrav1.IBMPowerVSImageSpec, out *IBMPowerVSImageSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSImageSpec_To_v1beta2_IBMPowerVSImageSpec(in, out, s); err != nil {
		return err
	}

	if in.Workspace.ID != "" || in.Workspace.Name != "" {
		out.ServiceInstance = &IBMPowerVSResourceReference{}
		if in.Workspace.ID != "" {
			out.ServiceInstance.ID = ptr.To(in.Workspace.ID)
			out.ServiceInstanceID = in.Workspace.ID
		}
		if in.Workspace.Name != "" {
			out.ServiceInstance.Name = ptr.To(in.Workspace.Name)
		}
	} else {
		out.ServiceInstance = nil
		out.ServiceInstanceID = ""
	}

	return nil
}

// Convert_v1beta2_DHCPServer_To_v1beta3_DHCPServer handles the conversion from v1beta2 to v1beta3 DHCPServer.
func Convert_v1beta2_DHCPServer_To_v1beta3_DHCPServer(in *DHCPServer, out *infrav1.DHCPServer, _ apimachineryconversion.Scope) error {
	// Convert Cidr (pointer) to CIDR (string)
	if in.Cidr != nil {
		out.CIDR = *in.Cidr
	}

	// Convert DNSServer (pointer) to DNSServer (string)
	if in.DNSServer != nil {
		out.DNSServer = *in.DNSServer
	}

	// Convert Name (pointer) to Name (string)
	if in.Name != nil {
		out.Name = *in.Name
	}

	// Convert Snat (bool pointer) to Snat (DHCPSnatPolicy enum)
	if in.Snat != nil {
		if *in.Snat {
			out.Snat = infrav1.DHCPSnatPolicyEnabled
		} else {
			out.Snat = infrav1.DHCPSnatPolicyDisabled
		}
	}

	// Note: v1beta2.ID field is dropped in v1beta3 as it's not part of the spec
	return nil
}

// Convert_v1beta3_DHCPServer_To_v1beta2_DHCPServer handles the conversion from v1beta3 to v1beta2 DHCPServer.
func Convert_v1beta3_DHCPServer_To_v1beta2_DHCPServer(in *infrav1.DHCPServer, out *DHCPServer, _ apimachineryconversion.Scope) error {
	// Convert CIDR (string) to Cidr (pointer)
	if in.CIDR != "" {
		out.Cidr = ptr.To(in.CIDR)
	}

	// Convert DNSServer (string) to DNSServer (pointer)
	if in.DNSServer != "" {
		out.DNSServer = ptr.To(in.DNSServer)
	}

	// Convert Name (string) to Name (pointer)
	if in.Name != "" {
		out.Name = ptr.To(in.Name)
	}

	// Convert Snat (DHCPSnatPolicy enum) to Snat (bool pointer)
	if in.Snat != "" {
		out.Snat = ptr.To(in.Snat == infrav1.DHCPSnatPolicyEnabled)
	}

	// Note: v1beta2.ID field cannot be populated from v1beta3 as it doesn't exist there
	return nil
}

// Convert_v1beta2_IBMPowerVSResourceReference_To_v1beta3_NetworkSource is a stub conversion function.
// The actual conversion is handled in Convert_v1beta2_IBMPowerVSClusterSpec_To_v1beta3_IBMPowerVSClusterSpec
// because Network and DHCPServer fields need to be converted together.
func Convert_v1beta2_IBMPowerVSResourceReference_To_v1beta3_NetworkSource(_ *IBMPowerVSResourceReference, _ *infrav1.NetworkSource, _ apimachineryconversion.Scope) error {
	// Conversion handled in Spec conversion
	return nil
}

// Convert_v1beta3_NetworkSource_To_v1beta2_IBMPowerVSResourceReference is a stub conversion function.
// The actual conversion is handled in Convert_v1beta3_IBMPowerVSClusterSpec_To_v1beta2_IBMPowerVSClusterSpec
// because Network and DHCPServer fields need to be converted together.
func Convert_v1beta3_NetworkSource_To_v1beta2_IBMPowerVSResourceReference(_ *infrav1.NetworkSource, _ *IBMPowerVSResourceReference, _ apimachineryconversion.Scope) error {
	// Conversion handled in Spec conversion
	return nil
}

// Convert_v1beta2_IBMPowerVSResourceReference_To_v1beta3_ResourceIdentifier converts v1beta2 IBMPowerVSResourceReference to v1beta3 ResourceIdentifier.
func Convert_v1beta2_IBMPowerVSResourceReference_To_v1beta3_ResourceIdentifier(in *IBMPowerVSResourceReference, out *infrav1.ResourceIdentifier, _ apimachineryconversion.Scope) error {
	if in.ID != nil && *in.ID != "" {
		out.ID = *in.ID
	}
	if in.Name != nil && *in.Name != "" {
		out.Name = *in.Name
	}
	// Note: RegEx field is dropped in v1beta3
	return nil
}

// Convert_v1beta3_ResourceIdentifier_To_v1beta2_IBMPowerVSResourceReference converts v1beta3 ResourceIdentifier to v1beta2 IBMPowerVSResourceReference.
func Convert_v1beta3_ResourceIdentifier_To_v1beta2_IBMPowerVSResourceReference(in *infrav1.ResourceIdentifier, out *IBMPowerVSResourceReference, _ apimachineryconversion.Scope) error {
	if in.ID != "" {
		out.ID = ptr.To(in.ID)
	} else {
		out.ID = nil
	}
	if in.Name != "" {
		out.Name = ptr.To(in.Name)
	} else {
		out.Name = nil
	}
	// RegEx field cannot be restored from v1beta3
	out.RegEx = nil
	return nil
}
