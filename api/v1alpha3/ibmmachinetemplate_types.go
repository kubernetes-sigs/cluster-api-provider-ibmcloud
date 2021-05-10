package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IBMMachineTemplateSpec defines the desired state of IBMMachineTemplate
type IBMMachineTemplateSpec struct {
	Template IBMMachineTemplateResource `json:"template"`
}

// IBMMachineTemplateResource describes the data needed to create am IBMMachine from a template
type IBMMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec IBMMachineSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ibmmachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// IBMMachineTemplate is the Schema for the IBMMachinetemplates API
type IBMMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IBMMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// IBMMachineTemplateList contains a list of IBMMachineTemplate
type IBMMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IBMMachineTemplate{}, &IBMMachineTemplateList{})
}
