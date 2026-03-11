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

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ROKSControlPlaneSpec defines the desired state of ROKSControlPlane
type ROKSControlPlaneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +immutable
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="roksClusterName is immutable"
	// +kubebuilder:validation:MaxLength:=54
	// +kubebuilder:validation:Pattern:=`^[a-z]([-a-z0-9]*[a-z0-9])?$`
	RoksClusterName *string `json:"roksClusterName"`

	// OpenShift semantic version, for example "4.14.5".
	Version *string `json:"version"`

	// VPC provider type.
	Provider *string `json:"provider"`

	// Worker node flavour.
	Flavor *string `json:"flavor"`

	// No of worker nodes.
	WorkerCount *int64 `json:"workerCount"`

	// VPC ID.
	VpcID *string `json:"vpcID"`

	// Operating System for the worker node
	OperatingSystem *string `json:"operatingSystem"`

	// IBM AvailabilityZones of the worker nodes
	// should match the AvailabilityZones of the Subnets.
	AvailabilityZones []string `json:"availabilityZones"`

	// The Subnet IDs to use when installing the cluster.
	// SubnetIDs should come in pairs; two per availability zone, one private and one public.
	Subnets []string `json:"subnets"`

	// CosInstance for the ROKS Cluster
	CosInstanceCRN *string `json:"cosInstanceCRN"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// ROKSControlPlaneStatus defines the observed state of ROKSControlPlane.
type ROKSControlPlaneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ROKSControlPlane resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ExternalManagedControlPlane indicates to cluster-api that the control plane
	// is managed by an external service such as AKS, EKS, GKE, etc.
	// +kubebuilder:default=true
	ExternalManagedControlPlane *bool `json:"externalManagedControlPlane,omitempty"`

	// Initialized denotes whether or not the control plane has the
	// uploaded kubernetes config-map.
	// +optional
	Initialized bool `json:"initialized"`
	// Ready denotes that the ROKSControlPlane API Server is ready to receive requests.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the state and will be set to a descriptive error message.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the spec or the configuration of
	// the controller, and that manual intervention is required.
	//
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// ID is the cluster ID given by ROKS.
	ID string `json:"id,omitempty"`
	// ConsoleURL is the url for the openshift console.
	ConsoleURL string `json:"consoleURL,omitempty"`
	// OIDCEndpointURL is the endpoint url for the managed OIDC provider.
	OIDCEndpointURL string `json:"oidcEndpointURL,omitempty"`

	// OpenShift semantic version, for example "4.14.5".
	// +optional
	Version string `json:"version"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ROKSControlPlane is the Schema for the rokscontrolplanes API
type ROKSControlPlane struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ROKSControlPlane
	// +required
	Spec ROKSControlPlaneSpec `json:"spec"`

	// status defines the observed state of ROKSControlPlane
	// +optional
	Status ROKSControlPlaneStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ROKSControlPlaneList contains a list of ROKSControlPlane
type ROKSControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ROKSControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ROKSControlPlane{}, &ROKSControlPlaneList{})
}
