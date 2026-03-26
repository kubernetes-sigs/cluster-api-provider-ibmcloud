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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ROKSControlPlaneSpec defines the desired state of ROKSControlPlane
type ROKSControlPlaneSpec struct {
	// Name is the name of the ROKS cluster
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// OpenshiftVersion is the OpenShift version to install
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+_openshift$`
	OpenshiftVersion string `json:"openshiftVersion"`

	// Location is the datacenter or zone where the cluster will be created
	// +kubebuilder:validation:Required
	Location string `json:"location"`

	// VPC configuration
	// +kubebuilder:validation:Required
	VPC VPCConfig `json:"vpc"`

	// Networking configuration
	Network NetworkConfig `json:"network,omitempty"`

	// Security and encryption
	Security SecurityConfig `json:"security,omitempty"`

	// OpenShift specific configuration
	Openshift OpenshiftConfig `json:"openshift"`

	// ResourceGroupID is the ID of the resource group
	// +kubebuilder:validation:Required
	ResourceGroupID string `json:"resourceGroupID"`

	// DefaultWorkerPool defines the default worker pool configuration
	// +kubebuilder:validation:Required
	DefaultWorkerPool DefaultWorkerPoolSpec `json:"defaultWorkerPool"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// VPCConfig contains VPC-specific configuration
type VPCConfig struct {
	// VPCID is the ID of the VPC
	// +kubebuilder:validation:Required
	VPCID string `json:"vpcID"`

	// SubnetID is the ID of the subnet
	// +kubebuilder:validation:Required
	SubnetID string `json:"subnetID"`
}

// NetworkConfig contains networking configuration
type NetworkConfig struct {
	// PodSubnet is the subnet for pods
	// +kubebuilder:default="172.30.0.0/16"
	PodSubnet string `json:"podSubnet,omitempty"`

	// ServiceSubnet is the subnet for services
	// +kubebuilder:default="172.21.0.0/16"
	ServiceSubnet string `json:"serviceSubnet,omitempty"`

	// PrivateServiceEndpoint enables private service endpoint
	// +kubebuilder:default=true
	PrivateServiceEndpoint bool `json:"privateServiceEndpoint,omitempty"`

	// PublicServiceEndpoint enables public service endpoint
	// +kubebuilder:default=true
	PublicServiceEndpoint bool `json:"publicServiceEndpoint,omitempty"`
}

// SecurityConfig contains security and encryption configuration
type SecurityConfig struct {
	// DiskEncryption enables disk encryption
	// +kubebuilder:default=true
	DiskEncryption bool `json:"diskEncryption,omitempty"`

	// KMS contains Key Management Service configuration
	KMS *KMSConfig `json:"kms,omitempty"`
}

// KMSConfig contains KMS encryption configuration
type KMSConfig struct {
	// InstanceID is the KMS instance ID
	// +kubebuilder:validation:Required
	InstanceID string `json:"instanceID"`

	// RootKeyID is the root key ID
	// +kubebuilder:validation:Required
	RootKeyID string `json:"rootKeyID"`
}

// OpenshiftConfig contains OpenShift-specific configuration
type OpenshiftConfig struct {
	// Entitlement specifies the OpenShift entitlement
	// +kubebuilder:validation:Enum=cloud_pak;ocp_entitled
	Entitlement string `json:"entitlement,omitempty"`

	// CosInstanceCRN is the CRN of the COS instance for image registry
	// +kubebuilder:validation:Required
	CosInstanceCRN string `json:"cosInstanceCRN"`
}

// DefaultWorkerPoolSpec defines the default worker pool
type DefaultWorkerPoolSpec struct {
	// Name is the name of the worker pool
	// +kubebuilder:default="default"
	Name string `json:"name,omitempty"`

	// Flavor is the machine type
	// +kubebuilder:validation:Required
	Flavor string `json:"flavor"`

	// WorkerCount is the number of workers
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	WorkerCount int `json:"workerCount,omitempty"`

	// Zones defines the zones for the worker pool
	// +kubebuilder:validation:MinItems=1
	Zones []WorkerPoolZoneSpec `json:"zones"`
}

// WorkerPoolZoneSpec defines a zone for a worker pool
type WorkerPoolZoneSpec struct {
	// ID is the zone ID
	// +kubebuilder:validation:Required
	ID string `json:"id"`

	// SubnetID is the subnet ID (for VPC clusters)
	SubnetID string `json:"subnetID,omitempty"`
}

// ROKSControlPlaneStatus defines the observed state of ROKSControlPlane.
type ROKSControlPlaneStatus struct {
	// Ready indicates the cluster is ready
	Ready bool `json:"ready"`

	// ClusterID is the IBM Cloud cluster ID
	ClusterID string `json:"clusterID,omitempty"`

	// State is the cluster state
	State string `json:"state,omitempty"`

	// MasterStatus is the master status
	MasterStatus string `json:"masterStatus,omitempty"`

	// MasterURL is the master API endpoint
	MasterURL string `json:"masterURL,omitempty"`

	// IngressHostname is the ingress hostname
	IngressHostname string `json:"ingressHostname,omitempty"`

	// IngressSecretName is the ingress secret name
	IngressSecretName string `json:"ingressSecretName,omitempty"`

	// WorkerCount is the current number of workers
	WorkerCount int `json:"workerCount,omitempty"`

	// Conditions defines current service state
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureReason indicates there is a fatal problem reconciling the state
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage indicates a fatal problem reconciling the state
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// V1beta2 groups all the fields that will be added or modified in ROKSControlPlane's status with the V1Beta2 version.
	// +optional
	V1Beta2 *ROKSControlPlaneV1Beta2Status `json:"v1beta2,omitempty"`
}

// ROKSControlPlaneV1Beta2Status groups all the fields that will be added or modified in ROKSControlPlaneStatus with the V1Beta2 version.
type ROKSControlPlaneV1Beta2Status struct {
	// Conditions represents the observations of a ROKSControlPlane's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=rokscontrolplanes,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.name",description="Cluster name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster ready status"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="Cluster state"
// +kubebuilder:printcolumn:name="ClusterID",type="string",JSONPath=".status.clusterID",description="IBM Cloud cluster ID"

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

// GetConditions returns the set of conditions for this object
func (c *ROKSControlPlane) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object
func (c *ROKSControlPlane) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for ROKSControlPlane object.
func (c *ROKSControlPlane) GetV1Beta2Conditions() []metav1.Condition {
	if c.Status.V1Beta2 == nil {
		return nil
	}
	return c.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets conditions for ROKSControlPlane object.
func (c *ROKSControlPlane) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if c.Status.V1Beta2 == nil {
		c.Status.V1Beta2 = &ROKSControlPlaneV1Beta2Status{}
	}
	c.Status.V1Beta2.Conditions = conditions
}
