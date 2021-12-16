//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSCluster) DeepCopyInto(out *IBMPowerVSCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSCluster.
func (in *IBMPowerVSCluster) DeepCopy() *IBMPowerVSCluster {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSClusterList) DeepCopyInto(out *IBMPowerVSClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMPowerVSCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSClusterList.
func (in *IBMPowerVSClusterList) DeepCopy() *IBMPowerVSClusterList {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSClusterSpec) DeepCopyInto(out *IBMPowerVSClusterSpec) {
	*out = *in
	in.Network.DeepCopyInto(&out.Network)
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSClusterSpec.
func (in *IBMPowerVSClusterSpec) DeepCopy() *IBMPowerVSClusterSpec {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSClusterStatus) DeepCopyInto(out *IBMPowerVSClusterStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSClusterStatus.
func (in *IBMPowerVSClusterStatus) DeepCopy() *IBMPowerVSClusterStatus {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachine) DeepCopyInto(out *IBMPowerVSMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachine.
func (in *IBMPowerVSMachine) DeepCopy() *IBMPowerVSMachine {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineList) DeepCopyInto(out *IBMPowerVSMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMPowerVSMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineList.
func (in *IBMPowerVSMachineList) DeepCopy() *IBMPowerVSMachineList {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineSpec) DeepCopyInto(out *IBMPowerVSMachineSpec) {
	*out = *in
	in.Image.DeepCopyInto(&out.Image)
	in.Network.DeepCopyInto(&out.Network)
	if in.ProviderID != nil {
		in, out := &in.ProviderID, &out.ProviderID
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineSpec.
func (in *IBMPowerVSMachineSpec) DeepCopy() *IBMPowerVSMachineSpec {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineStatus) DeepCopyInto(out *IBMPowerVSMachineStatus) {
	*out = *in
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]v1.NodeAddress, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineStatus.
func (in *IBMPowerVSMachineStatus) DeepCopy() *IBMPowerVSMachineStatus {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineTemplate) DeepCopyInto(out *IBMPowerVSMachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineTemplate.
func (in *IBMPowerVSMachineTemplate) DeepCopy() *IBMPowerVSMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSMachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineTemplateList) DeepCopyInto(out *IBMPowerVSMachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMPowerVSMachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineTemplateList.
func (in *IBMPowerVSMachineTemplateList) DeepCopy() *IBMPowerVSMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMPowerVSMachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineTemplateResource) DeepCopyInto(out *IBMPowerVSMachineTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineTemplateResource.
func (in *IBMPowerVSMachineTemplateResource) DeepCopy() *IBMPowerVSMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineTemplateSpec) DeepCopyInto(out *IBMPowerVSMachineTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineTemplateSpec.
func (in *IBMPowerVSMachineTemplateSpec) DeepCopy() *IBMPowerVSMachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSMachineTemplateStatus) DeepCopyInto(out *IBMPowerVSMachineTemplateStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSMachineTemplateStatus.
func (in *IBMPowerVSMachineTemplateStatus) DeepCopy() *IBMPowerVSMachineTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSMachineTemplateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMPowerVSResourceReference) DeepCopyInto(out *IBMPowerVSResourceReference) {
	*out = *in
	if in.ID != nil {
		in, out := &in.ID, &out.ID
		*out = new(string)
		**out = **in
	}
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMPowerVSResourceReference.
func (in *IBMPowerVSResourceReference) DeepCopy() *IBMPowerVSResourceReference {
	if in == nil {
		return nil
	}
	out := new(IBMPowerVSResourceReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCCluster) DeepCopyInto(out *IBMVPCCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCCluster.
func (in *IBMVPCCluster) DeepCopy() *IBMVPCCluster {
	if in == nil {
		return nil
	}
	out := new(IBMVPCCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCClusterList) DeepCopyInto(out *IBMVPCClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMVPCCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCClusterList.
func (in *IBMVPCClusterList) DeepCopy() *IBMVPCClusterList {
	if in == nil {
		return nil
	}
	out := new(IBMVPCClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCClusterSpec) DeepCopyInto(out *IBMVPCClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCClusterSpec.
func (in *IBMVPCClusterSpec) DeepCopy() *IBMVPCClusterSpec {
	if in == nil {
		return nil
	}
	out := new(IBMVPCClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCClusterStatus) DeepCopyInto(out *IBMVPCClusterStatus) {
	*out = *in
	out.VPC = in.VPC
	in.Subnet.DeepCopyInto(&out.Subnet)
	in.VPCEndpoint.DeepCopyInto(&out.VPCEndpoint)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCClusterStatus.
func (in *IBMVPCClusterStatus) DeepCopy() *IBMVPCClusterStatus {
	if in == nil {
		return nil
	}
	out := new(IBMVPCClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachine) DeepCopyInto(out *IBMVPCMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachine.
func (in *IBMVPCMachine) DeepCopy() *IBMVPCMachine {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineList) DeepCopyInto(out *IBMVPCMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMVPCMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineList.
func (in *IBMVPCMachineList) DeepCopy() *IBMVPCMachineList {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineSpec) DeepCopyInto(out *IBMVPCMachineSpec) {
	*out = *in
	if in.ProviderID != nil {
		in, out := &in.ProviderID, &out.ProviderID
		*out = new(string)
		**out = **in
	}
	out.PrimaryNetworkInterface = in.PrimaryNetworkInterface
	if in.SSHKeys != nil {
		in, out := &in.SSHKeys, &out.SSHKeys
		*out = make([]*string, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(string)
				**out = **in
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineSpec.
func (in *IBMVPCMachineSpec) DeepCopy() *IBMVPCMachineSpec {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineStatus) DeepCopyInto(out *IBMVPCMachineStatus) {
	*out = *in
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]v1.NodeAddress, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineStatus.
func (in *IBMVPCMachineStatus) DeepCopy() *IBMVPCMachineStatus {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineTemplate) DeepCopyInto(out *IBMVPCMachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineTemplate.
func (in *IBMVPCMachineTemplate) DeepCopy() *IBMVPCMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCMachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineTemplateList) DeepCopyInto(out *IBMVPCMachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IBMVPCMachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineTemplateList.
func (in *IBMVPCMachineTemplateList) DeepCopy() *IBMVPCMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IBMVPCMachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineTemplateResource) DeepCopyInto(out *IBMVPCMachineTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineTemplateResource.
func (in *IBMVPCMachineTemplateResource) DeepCopy() *IBMVPCMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IBMVPCMachineTemplateSpec) DeepCopyInto(out *IBMVPCMachineTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IBMVPCMachineTemplateSpec.
func (in *IBMVPCMachineTemplateSpec) DeepCopy() *IBMVPCMachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(IBMVPCMachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkInterface) DeepCopyInto(out *NetworkInterface) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkInterface.
func (in *NetworkInterface) DeepCopy() *NetworkInterface {
	if in == nil {
		return nil
	}
	out := new(NetworkInterface)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Subnet) DeepCopyInto(out *Subnet) {
	*out = *in
	if in.Ipv4CidrBlock != nil {
		in, out := &in.Ipv4CidrBlock, &out.Ipv4CidrBlock
		*out = new(string)
		**out = **in
	}
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	if in.ID != nil {
		in, out := &in.ID, &out.ID
		*out = new(string)
		**out = **in
	}
	if in.Zone != nil {
		in, out := &in.Zone, &out.Zone
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Subnet.
func (in *Subnet) DeepCopy() *Subnet {
	if in == nil {
		return nil
	}
	out := new(Subnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VPC) DeepCopyInto(out *VPC) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VPC.
func (in *VPC) DeepCopy() *VPC {
	if in == nil {
		return nil
	}
	out := new(VPC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VPCEndpoint) DeepCopyInto(out *VPCEndpoint) {
	*out = *in
	if in.Address != nil {
		in, out := &in.Address, &out.Address
		*out = new(string)
		**out = **in
	}
	if in.FIPID != nil {
		in, out := &in.FIPID, &out.FIPID
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VPCEndpoint.
func (in *VPCEndpoint) DeepCopy() *VPCEndpoint {
	if in == nil {
		return nil
	}
	out := new(VPCEndpoint)
	in.DeepCopyInto(out)
	return out
}
