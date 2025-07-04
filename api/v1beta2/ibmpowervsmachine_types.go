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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PowerVSProcessorType enum attribute to identify the PowerVS instance processor type.
type PowerVSProcessorType string

const (
	// IBMPowerVSMachineFinalizer allows IBMPowerVSMachineReconciler to clean up resources associated with IBMPowerVSMachine before
	// removing it from the apiserver.
	IBMPowerVSMachineFinalizer = "ibmpowervsmachine.infrastructure.cluster.x-k8s.io"
	// PowerVSProcessorTypeDedicated enum property to identify a Dedicated Power VS processor type.
	PowerVSProcessorTypeDedicated PowerVSProcessorType = "Dedicated"
	// PowerVSProcessorTypeShared enum property to identify a Shared Power VS processor type.
	PowerVSProcessorTypeShared PowerVSProcessorType = "Shared"
	// PowerVSProcessorTypeCapped enum property to identify a Capped Power VS processor type.
	PowerVSProcessorTypeCapped PowerVSProcessorType = "Capped"
	// DefaultIgnitionVersion represents default Ignition version generated for machine userdata.
	DefaultIgnitionVersion = "2.3"
)

// IBMPowerVSMachineSpec defines the desired state of IBMPowerVSMachine.
type IBMPowerVSMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ServiceInstanceID is the id of the power cloud instance where the vsi instance will get deployed.
	// Deprecated: use ServiceInstance instead
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

	// SSHKey is the name of the SSH key pair provided to the vsi for authenticating users.
	SSHKey string `json:"sshKey,omitempty"`

	// Image the reference to the image which is used to create the instance.
	// supported image identifier in IBMPowerVSResourceReference are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// +optional
	Image *IBMPowerVSResourceReference `json:"image,omitempty"`

	// ImageRef is an optional reference to a provider-specific resource that holds
	// the details for provisioning the Image for a Cluster.
	// +optional
	ImageRef *corev1.LocalObjectReference `json:"imageRef,omitempty"`

	// systemType is the System type used to host the instance.
	// systemType determines the number of cores and memory that is available.
	// Few of the supported SystemTypes are s922,e980,s1022,e1050,e1080.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The current default is s922 which is generally available.
	// + This is not an enum because we expect other values to be added later which should be supported implicitly.
	// +kubebuilder:validation:Enum:="s922";"e980";"s1022";"e1050";"e1080";""
	// +optional
	SystemType string `json:"systemType,omitempty"`

	// processorType is the VM instance processor type.
	// It must be set to one of the following values: Dedicated, Capped or Shared.
	// Dedicated: resources are allocated for a specific client, The hypervisor makes a 1:1 binding of a partition’s processor to a physical processor core.
	// Shared: Shared among other clients.
	// Capped: Shared, but resources do not expand beyond those that are requested, the amount of CPU time is Capped to the value specified for the entitlement.
	// if the processorType is selected as Dedicated, then processors value cannot be fractional.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The current default is Shared.
	// +kubebuilder:validation:Enum:="Dedicated";"Shared";"Capped";""
	// +optional
	ProcessorType PowerVSProcessorType `json:"processorType,omitempty"`

	// processors is the number of virtual processors in a virtual machine.
	// when the processorType is selected as Dedicated the processors value cannot be fractional.
	// maximum value for the Processors depends on the selected SystemType,
	// and minimum value for Processors depends on the selected ProcessorType, which can be found here: https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-pricing-virtual-server-on-cloud.
	// when ProcessorType is set as Shared or Capped, The minimum processors is 0.25.
	// when ProcessorType is set as Dedicated, The minimum processors is 1.
	// When omitted, this means that the user has no opinion and the platform is left to choose a
	// reasonable default, which is subject to change over time. The default is set based on the selected ProcessorType.
	// when ProcessorType selected as Dedicated, the default is set to 1.
	// when ProcessorType selected as Shared or Capped, the default is set to 0.25.
	// +optional
	Processors intstr.IntOrString `json:"processors,omitempty"`

	// memoryGiB is the size of a virtual machine's memory, in GiB.
	// maximum value for the MemoryGiB depends on the selected SystemType, which can be found here: https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-pricing-virtual-server-on-cloud
	// The minimum memory is 2 GiB.
	// When omitted, this means the user has no opinion and the platform is left to choose a reasonable
	// default, which is subject to change over time. The current default is 2.
	// +optional
	MemoryGiB int32 `json:"memoryGiB,omitempty"`

	// Network is the reference to the Network to use for this instance.
	// supported network identifier in IBMPowerVSResourceReference are Name, ID and RegEx and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	Network IBMPowerVSResourceReference `json:"network"`

	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
}

// IBMPowerVSResourceReference is a reference to a specific PowerVS resource by ID, Name or RegEx
// Only one of ID, Name or RegEx may be specified. Specifying more than one will result in
// a validation error.
type IBMPowerVSResourceReference struct {
	// ID of resource
	// +kubebuilder:validation:MinLength=1
	// +optional
	ID *string `json:"id,omitempty"`

	// Name of resource
	// +kubebuilder:validation:MinLength=1
	// +optional
	Name *string `json:"name,omitempty"`

	// Regular expression to match resource,
	// In case of multiple resources matches the provided regular expression the first matched resource will be selected
	// +kubebuilder:validation:MinLength=1
	// +optional
	RegEx *string `json:"regex,omitempty"`
}

// IBMPowerVSMachineStatus defines the observed state of IBMPowerVSMachine.
type IBMPowerVSMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	InstanceID string `json:"instanceID,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the vsi associated addresses.
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// Health is the health of the vsi.
	// +optional
	Health string `json:"health,omitempty"`

	// InstanceState is the status of the vsi.
	// +optional
	InstanceState PowerVSInstanceState `json:"instanceState,omitempty"`

	// Fault will report if any fault messages for the vsi.
	// +optional
	Fault string `json:"fault,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	//
	// Deprecated: This field is deprecated and is going to be removed in the next apiVersion. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	//
	// Deprecated: This field is deprecated and is going to be removed in the next apiVersion. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the IBMPowerVSMachine.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// Region specifies the Power VS Service instance region.
	Region *string `json:"region,omitempty"`

	// Zone specifies the Power VS Service instance zone.
	Zone *string `json:"zone,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in IBMPowerVSMachine's status with the V1Beta2 version.
	// +optional
	V1Beta2 *IBMPowerVSMachineV1Beta2Status `json:"v1beta2,omitempty"`
}

// IBMPowerVSMachineV1Beta2Status groups all the fields that will be added or modified in IBMPowerVSMachineStatus with the V1Beta2 version.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type IBMPowerVSMachineV1Beta2Status struct {
	// conditions represents the observations of a IBMPowerVSMachine's current state.
	// Known condition types are Ready, InstanceReady and Paused.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this IBMPowerVSMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",priority=1,JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object to which this IBMPowerVSMachine belongs"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of IBMPowerVSMachine"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for IBM PowerVS instances"
// +kubebuilder:printcolumn:name="Internal-IP",type="string",priority=1,JSONPath=".status.addresses[?(@.type==\"InternalIP\")].address",description="Instance Internal Addresses"
// +kubebuilder:printcolumn:name="External-IP",type="string",priority=1,JSONPath=".status.addresses[?(@.type==\"ExternalIP\")].address",description="Instance External Addresses"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.instanceState",description="PowerVS instance state"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.health",description="PowerVS instance health"

// IBMPowerVSMachine is the Schema for the ibmpowervsmachines API.
type IBMPowerVSMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMPowerVSMachineSpec   `json:"spec,omitempty"`
	Status IBMPowerVSMachineStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the IBMPowerVSMachine resource.
func (r *IBMPowerVSMachine) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the IBMPowerVSMachine to the predescribed clusterv1beta1.Conditions.
func (r *IBMPowerVSMachine) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for this object.
func (r *IBMPowerVSMachine) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets conditions for an API object.
func (r *IBMPowerVSMachine) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &IBMPowerVSMachineV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

//+kubebuilder:object:root=true

// IBMPowerVSMachineList contains a list of IBMPowerVSMachine.
type IBMPowerVSMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMPowerVSMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &IBMPowerVSMachine{}, &IBMPowerVSMachineList{})
}
