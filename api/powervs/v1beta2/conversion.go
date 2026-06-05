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

	// Manual conversion for Status: ServiceInstance -> Workspace
	// v1beta2 ResourceReference only has an ID, no Name.
	if in.ServiceInstance != nil && in.ServiceInstance.ID != nil {
		out.Workspace.ID = *in.ServiceInstance.ID
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

	// Manual conversion for Status: Workspace -> ServiceInstance
	if in.Workspace.ID != "" {
		out.ServiceInstance = &ResourceReference{ // v1beta2 type
			ID: ptr.To(in.Workspace.ID),
		}
	} else {
		out.ServiceInstance = nil
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
	return autoConvert_v1beta3_IBMPowerVSMachineTemplateResource_To_v1beta2_IBMPowerVSMachineTemplateResource(in, out, s)
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

	// Cluster uses WorkspaceSource (Type: Reference | Provision)
	if in.ServiceInstance != nil || in.ServiceInstanceID != "" {
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
	} else {
		out.Workspace.Type = infrav1.SourceTypeProvision
	}

	return nil
}

func Convert_v1beta3_IBMPowerVSClusterSpec_To_v1beta2_IBMPowerVSClusterSpec(in *infrav1.IBMPowerVSClusterSpec, out *IBMPowerVSClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta3_IBMPowerVSClusterSpec_To_v1beta2_IBMPowerVSClusterSpec(in, out, s); err != nil {
		return err
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
