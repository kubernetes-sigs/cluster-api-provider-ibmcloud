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

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	// IBMPowerVSImageFinalizer allows IBMPowerVSImageReconciler to clean up resources associated with IBMPowerVSImage before
	// removing it from the apiserver.
	IBMPowerVSImageFinalizer = "ibmpowervsimage.infrastructure.cluster.x-k8s.io"
)

func init() {
	objectTypes = append(objectTypes, &IBMPowerVSImage{}, &IBMPowerVSImageList{})
}

// IBMPowerVSImageSpec defines the desired state of IBMPowerVSImage.
type IBMPowerVSImageSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Deprecated: use ServiceInstance instead
	//
	// ServiceInstanceID is the id of the power cloud instance where the image will get imported.
	ServiceInstanceID string `json:"serviceInstanceID"`

	// serviceInstance is the reference to the Power VS workspace on which the server instance(VM) will be created.
	// Power VS workspace is a container for all Power VS instances at a specific geographic region.
	// serviceInstance can be created via IBM Cloud catalog or CLI.
	// supported serviceInstance identifier in PowerVSResource are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// More detail about Power VS service instance.
	// https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
	// when omitted system will dynamically create the service instance
	// +optional
	ServiceInstance *IBMPowerVSResourceReference `json:"serviceInstance,omitempty"`

	// Cloud Object Storage bucket name; bucket-name[/optional/folder]
	Bucket *string `json:"bucket"`

	// Cloud Object Storage image filename.
	Object *string `json:"object"`

	// Cloud Object Storage region.
	Region *string `json:"region"`

	// Type of storage, storage pool with the most available space will be selected.
	// +kubebuilder:default=tier1
	// +kubebuilder:validation:Enum=tier0;tier1;tier3
	// +optional
	StorageType string `json:"storageType,omitempty"`

	// DeletePolicy defines the policy used to identify images to be preserved beyond the lifecycle of associated cluster.
	// +kubebuilder:default=delete
	// +kubebuilder:validation:Enum=delete;retain
	// +optional
	DeletePolicy string `json:"deletePolicy,omitempty"`
}

// IBMPowerVSImageStatus defines the observed state of IBMPowerVSImage.
type IBMPowerVSImageStatus struct {
	// conditions represents the observations of a IBMPowerVSImage's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// ImageID is the id of the imported image.
	ImageID string `json:"imageID,omitempty"`

	// ImageState is the status of the imported image.
	// +optional
	ImageState PowerVSImageState `json:"imageState,omitempty"`

	// JobID is the job ID of an import operation.
	// +optional
	JobID string `json:"jobID,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.
	// +optional
	Deprecated *IBMPowerVSImageDeprecatedStatus `json:"deprecated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=ibmpowervsimages,scope=Namespaced,categories=cluster-api
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.imageState",description="PowerVS image state"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Image is ready for IBM PowerVS instances"

// IBMPowerVSImage is the Schema for the ibmpowervsimages API.
type IBMPowerVSImage struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of IBMPowerVSImage
	// +required
	Spec IBMPowerVSImageSpec `json:"spec"`

	// status defines the observed state of IBMPowerVSImage
	// +optional
	Status IBMPowerVSImageStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// IBMPowerVSImageList contains a list of IBMPowerVSImage.
type IBMPowerVSImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IBMPowerVSImage `json:"items"`
}

// IBMPowerVSImageDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type IBMPowerVSImageDeprecatedStatus struct {
	// v1beta2 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	V1Beta2 *IBMPowerVSImageV1Beta2DeprecatedStatus `json:"v1beta2,omitempty"`
}

// IBMPowerVSImageV1Beta2DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type IBMPowerVSImageV1Beta2DeprecatedStatus struct {
	// conditions defines current service state of the VSphereMachine.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the observations of the operational state of the IBMPowerVSImage resource.
func (r *IBMPowerVSImage) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (r *IBMPowerVSImage) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the set of conditions for this object.
func (r *IBMPowerVSImage) GetV1Beta1Conditions() clusterv1.Conditions {
	if r.Status.Deprecated == nil || r.Status.Deprecated.V1Beta2 == nil {
		return nil
	}
	return r.Status.Deprecated.V1Beta2.Conditions
}

// SetV1Beta1Conditions sets conditions for an API object.
func (r *IBMPowerVSImage) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if r.Status.Deprecated == nil {
		r.Status.Deprecated = &IBMPowerVSImageDeprecatedStatus{}
	}
	if r.Status.Deprecated.V1Beta2 == nil {
		r.Status.Deprecated.V1Beta2 = &IBMPowerVSImageV1Beta2DeprecatedStatus{}
	}
	r.Status.Deprecated.V1Beta2.Conditions = conditions
}
