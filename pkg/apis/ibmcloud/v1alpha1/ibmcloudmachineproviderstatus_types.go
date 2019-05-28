/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IBMCloudMachineProviderStatusSpec defines the desired state of IBMCloudMachineProviderStatus
type IBMCloudMachineProviderStatusSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// IBMCloudMachineProviderStatusStatus defines the observed state of IBMCloudMachineProviderStatus
type IBMCloudMachineProviderStatusStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IBMCloudMachineProviderStatus is the Schema for the ibmcloudmachineproviderstatuses API
// +k8s:openapi-gen=true
type IBMCloudMachineProviderStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMCloudMachineProviderStatusSpec   `json:"spec,omitempty"`
	Status IBMCloudMachineProviderStatusStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IBMCloudMachineProviderStatusList contains a list of IBMCloudMachineProviderStatus
type IBMCloudMachineProviderStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMCloudMachineProviderStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IBMCloudMachineProviderStatus{}, &IBMCloudMachineProviderStatusList{})
}
