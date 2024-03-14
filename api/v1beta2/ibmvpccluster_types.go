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
	// ClusterFinalizer allows DockerClusterReconciler to clean up resources associated with DockerCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "ibmvpccluster.infrastructure.cluster.x-k8s.io"
)

// IBMVPCClusterSpec defines the desired state of IBMVPCCluster.
type IBMVPCClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The IBM Cloud Region the cluster lives in.
	Region string `json:"region"`

	// The VPC resources should be created under the resource group.
	ResourceGroup string `json:"resourceGroup"`

	// The Name of VPC.
	VPC string `json:"vpc,omitempty"`

	// The Name of availability zone.
	Zone string `json:"zone,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint capiv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// ControlPlaneLoadBalancer is optional configuration for customizing control plane behavior.
	// +optional
	ControlPlaneLoadBalancer *VPCLoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`

	// cosInstance is the IBM COS instance to use for cluster resources.
	// +optional
	COSInstance *COSInstance `json:"cosInstance,omitempty"`

	// loadBalancers is a set of VPC Load Balancers definition to use for the cluster.
	// +optional
	LoadBalancers []*VPCLoadBalancerSpec `json:"loadbalancers,omitempty"`

	// networkSpec represents the VPC network to use for the cluster.
	// +optional
	NetworkSpec *VPCNetworkSpec `json:"networkSpec,omitempty"`
}

// VPCLoadBalancerSpec defines the desired state of an VPC load balancer.
type VPCLoadBalancerSpec struct {
	// Name sets the name of the VPC load balancer.
	// +kubebuilder:validation:MaxLength:=63
	// +optional
	Name string `json:"name,omitempty"`

	// id of the loadbalancer
	// +optional
	ID *string `json:"id,omitempty"`

	// public indicates that load balancer is public or private
	// +kubebuilder:default=true
	// +optional
	Public *bool `json:"public,omitempty"`

	// AdditionalListeners sets the additional listeners for the control plane load balancer.
	// +listType=map
	// +listMapKey=port
	// +optional
	AdditionalListeners []AdditionalListenerSpec `json:"additionalListeners,omitempty"`
}

// AdditionalListenerSpec defines the desired state of an
// additional listener on an VPC load balancer.
type AdditionalListenerSpec struct {
	// Port sets the port for the additional listener.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int64 `json:"port"`
}

// VPCLoadBalancerStatus defines the status VPC load balancer.
type VPCLoadBalancerStatus struct {
	// id of VPC load balancer.
	// +optional
	ID *string `json:"id,omitempty"`
	// State is the status of the load balancer.
	State VPCLoadBalancerState `json:"state,omitempty"`
	// hostname is the hostname of load balancer.
	// +optional
	Hostname *string `json:"hostname,omitempty"`
	// +kubebuilder:default=false
	// controllerCreated indicates whether the resource is created by the controller.
	ControllerCreated *bool `json:"controllerCreated,omitempty"`
}

// VPCNetworkSpec defines the desired state of the network resources for the cluster.
type VPCNetworkSpec struct {
	// computeSubnetsSpec is a set of Subnet's which define the Compute subnets.
	ComputeSubnetsSpec []Subnet `json:"computeSubnetsSpec,omitempty"`

	// controlPlaneSubnetsSpec is a set of Subnet's which define the Control Plane subnets.
	ControlPlaneSubnetsSpec []Subnet `json:"controlPlaneSubentsSpec,omitempty"`

	// resourceGroup is the name of the Resource Group containing all of the newtork resources.
	// This can be different than the Resource Group containing the remaining cluster resources.
	ResourceGroup *string `json:"resourceGroup,omitempty"`

	// securityGroups is a set of SecurityGroup's which define the VPC Security Groups that manage traffic within and out of the VPC.
	SecurityGroups []SecurityGroup `json:"securityGroups,omitempty"`

	// vpc defines the IBM Cloud VPC.
	VPC *VPCResource `json:"vpc,omitempty"`
}

// SecurityGroup dummy.
// TODO(cjschaef): Dummy SecurityGroup until it is defined in a common location.
type SecurityGroup struct {
	Name string `json:"name"`
}

// COSInstance dummy.
// TODO(cjschaef): Dummy COSInstance until it is defined in a common location.
type COSInstance struct {
	Name string `json:"name"`
}

// VPCResource dummy.
// TODO(cjschaef): Dummy VPCResource until it is defined in a common location.
type VPCResource struct {
	Name string `json:"name"`
}

// IBMVPCClusterStatus defines the observed state of IBMVPCCluster.
type IBMVPCClusterStatus struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions defines current service state of the load balancer.
	// +optional
	Conditions capiv1beta1.Conditions `json:"conditions,omitempty"`

	// ControlPlaneLoadBalancerState is the status of the load balancer.
	// dep: rely on NetworkStatus instead.
	// +optional
	ControlPlaneLoadBalancerState VPCLoadBalancerState `json:"controlPlaneLoadBalancerState,omitempty"`

	// COSInstance is the reference to the IBM Cloud COS Instance used for the cluster.
	COSInstance *ResourceReference `json:"cosInstance,omitempty"`

	// networkStatus is the status of the VPC network in its entirety resources.
	NetworkStatus *VPCNetworkStatus `json:"networkStatus,omitempty"`

	// ready is true when the provider resource is ready.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// resourceGroup is the reference to the IBM Cloud VPC resource group under which the resources will be created.
	ResourceGroup *ResourceReference `json:"resourceGroupID,omitempty"`

	// dep: rely on NetworkStatus instead.
	Subnet Subnet `json:"subnet,omitempty"`

	// dep: rely on NetworkStatus instead.
	VPC VPC `json:"vpc,omitempty"`

	// dep: rely on ControlPlaneEndpoint
	VPCEndpoint VPCEndpoint `json:"vpcEndpoint,omitempty"`
}

// VPCNetworkStatus provides details on the status of VPC network resources.
type VPCNetworkStatus struct {
	// computeSubnets references the VPC Subnets for the cluster's Data Plane.
	// +optional
	ComputeSubnets []*VPCResourceStatus `json:"computeSubnets,omitempty"`

	// controlPlaneSubnets references the VPC Subnets for the cluster's Control Plane.
	// +optional
	ControlPlaneSubnets []*VPCResourceStatus `json:"controlPlaneSubnets,omitempty"`

	// loadBalancers references the VPC Load Balancer's for the cluster.
	// +optional
	LoadBalancers []VPCLoadBalancerStatus `json:"loadBalancers,omitempty"`

	// publicGateways references the VPC Public Gateways for the cluster.
	// +optional
	PublicGateways []*VPCResourceStatus `json:"publicGateways,omitempty"`

	// securityGroups references the VPC Security Groups for the cluster.
	// +optional
	SecurityGroups []*VPCResourceStatus `json:"securityGroups,omitempty"`

	// vpc references the IBM Cloud VPC.
	// +optional
	VPC *VPCResourceStatus `json:"vpc,omitempty"`
}

// VPCResourceStatus identifies a resource by crn and type and whether it was created by the controller.
type VPCResourceStatus struct {
	// crn defines the IBM Cloud CRN of the resource.
	// +required
	CRN string `json:"crn"`
}

// VPC holds the VPC information.
type VPC struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ibmvpcclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this IBMVPCCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for IBM VPC instances"

// IBMVPCCluster is the Schema for the ibmvpcclusters API.
type IBMVPCCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IBMVPCClusterSpec   `json:"spec,omitempty"`
	Status IBMVPCClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IBMVPCClusterList contains a list of IBMVPCCluster.
type IBMVPCClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IBMVPCCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IBMVPCCluster{}, &IBMVPCClusterList{})
}

// GetConditions returns the observations of the operational state of the IBMVPCCluster resource.
func (r *IBMVPCCluster) GetConditions() capiv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the IBMVPCCluster to the predescribed clusterv1.Conditions.
func (r *IBMVPCCluster) SetConditions(conditions capiv1beta1.Conditions) {
	r.Status.Conditions = conditions
}
