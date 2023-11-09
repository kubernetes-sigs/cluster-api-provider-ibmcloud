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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// IBMPowerVSClusterFinalizer allows IBMPowerVSClusterReconciler to clean up resources associated with IBMPowerVSCluster before
	// removing it from the apiserver.
	IBMPowerVSClusterFinalizer = "ibmpowervscluster.infrastructure.cluster.x-k8s.io"
)

// IBMPowerVSClusterSpec defines the desired state of IBMPowerVSCluster.
type IBMPowerVSClusterSpec struct {
	// ServiceInstanceID is the id of the power cloud instance where the vsi instance will get deployed.
	// +kubebuilder:validation:MinLength=1
	// Deprecated: use ServiceInstance instead
	ServiceInstanceID string `json:"serviceInstanceID"`

	// Network is the reference to the Network to use for this cluster.
	// Whenf the field is omitted, A DHCP service will be created in the service instance and its private network will be used.
	Network IBMPowerVSResourceReference `json:"network"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint capiv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// serviceInstance is the reference to the Power VS service on which the server instance(VM) will be created.
	// Power VS service is a container for all Power VS instances at a specific geographic region.
	// serviceInstance can be created via IBM Cloud catalog or CLI.
	// supported serviceInstance identifier in PowerVSResource are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// More detail about Power VS service instance.
	// https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
	// when omitted system will dynamically create the service instance
	// +optional
	ServiceInstance IBMPowerVSResourceReference `json:"serviceInstance"`

	// zone is the name of Power VS zone where the cluster will be created
	Zone string `json:"zone"`

	// resourceGroup name under which the resources will be created.
	ResourceGroup string `json:"resourceGroup"`

	// vpc contains information about IBM Cloud VPC resources
	// +optional
	VPC IBMVPCResourceReference `json:"vpc,omitempty"`

	// transitGateway contains information about IBM Cloud TransitGateway.
	// +optional
	TransitGateway IBMTransitGatewayResource `json:"transitGateway,omitempty"`

	// controlPlaneLoadBalancer is optional configuration for customizing control plane behavior.
	// +optional
	ControlPlaneLoadBalancer *VPCLoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`
}

// IBMPowerVSClusterStatus defines the observed state of IBMPowerVSCluster.
type IBMPowerVSClusterStatus struct {
	// ready is true when the provider resource is ready.
	Ready bool `json:"ready"`

	// serviceInstanceID is the reference to the Power VS service on which the server instance(VM) will be created.
	ServiceInstanceID *string `json:"serviceInstanceID,omitempty"`

	// networkID is the reference to the Power VS network to use for this cluster.
	NetworkID *string `json:"networkID,omitempty"`

	// dhcpServerID is the reference to the Power VS DHCP server.
	DHCPServerID *string `json:"dhcpServerID,omitempty"`

	// vpcID is reference to IBM Cloud VPC resources.
	VPCID *string `json:"vpcID,omitempty"`

	// vpcSubnetID is reference to IBM Cloud VPC subnet.
	VPCSubnetID *string `json:"vpcSubnetID,omitempty"`

	// transitGatewayID is reference to IBM Cloud TransitGateway.
	TransitGatewayID *string `json:"transitGatewayID,omitempty"`

	// ControlPlaneLoadBalancer reference to IBM Cloud VPC Loadbalancer.
	ControlPlaneLoadBalancer *VPCLoadBalancerStatus `json:"controlPlaneLoadBalancer,omitempty"`

	// Conditions defines current service state of the IBMPowerVSCluster.
	Conditions capiv1beta1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this IBMPowerVSCluster belongs"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of IBMPowerVSCluster"
// +kubebuilder:printcolumn:name="PowerVS Cloud Instance ID",type="string",priority=1,JSONPath=".spec.serviceInstanceID"
// +kubebuilder:printcolumn:name="Endpoint",type="string",priority=1,JSONPath=".spec.controlPlaneEndpoint.host",description="Control Plane Endpoint"
// +kubebuilder:printcolumn:name="Port",type="string",priority=1,JSONPath=".spec.controlPlaneEndpoint.port",description="Control Plane Port"

// IBMPowerVSCluster is the Schema for the ibmpowervsclusters API.
type IBMPowerVSCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMPowerVSClusterSpec   `json:"spec,omitempty"`
	Status IBMPowerVSClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IBMPowerVSClusterList contains a list of IBMPowerVSCluster.
type IBMPowerVSClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMPowerVSCluster `json:"items"`
}

// IBMTransitGatewayResource holds the TransitGateway information.
type IBMTransitGatewayResource struct {
	Name *string `json:"name,omitempty"`
	ID   *string `json:"id,omitempty"`
}

func init() {
	SchemeBuilder.Register(&IBMPowerVSCluster{}, &IBMPowerVSClusterList{})
}
