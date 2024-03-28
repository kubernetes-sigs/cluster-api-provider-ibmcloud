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

// DefaultAPIServerPort is defuault API server port number.
const DefaultAPIServerPort int32 = 6443

// PowerVSInstanceState describes the state of an IBM Power VS instance.
type PowerVSInstanceState string

var (
	// PowerVSInstanceStateACTIVE is the string representing an instance in a ACTIVE state.
	PowerVSInstanceStateACTIVE = PowerVSInstanceState("ACTIVE")

	// PowerVSInstanceStateBUILD is the string representing an instance in a BUILD state.
	PowerVSInstanceStateBUILD = PowerVSInstanceState("BUILD")

	// PowerVSInstanceStateSHUTOFF is the string representing an instance in a SHUTOFF state.
	PowerVSInstanceStateSHUTOFF = PowerVSInstanceState("SHUTOFF")

	// PowerVSInstanceStateREBOOT is the string representing an instance in a REBOOT state.
	PowerVSInstanceStateREBOOT = PowerVSInstanceState("REBOOT")

	// PowerVSInstanceStateERROR is the string representing an instance in a ERROR state.
	PowerVSInstanceStateERROR = PowerVSInstanceState("ERROR")
)

// PowerVSImageState describes the state of an IBM Power VS image.
type PowerVSImageState string

var (
	// PowerVSImageStateACTIVE is the string representing an image in a active state.
	PowerVSImageStateACTIVE = PowerVSImageState("active")

	// PowerVSImageStateQue is the string representing an image in a queued state.
	PowerVSImageStateQue = PowerVSImageState("queued")

	// PowerVSImageStateFailed is the string representing an image in a failed state.
	PowerVSImageStateFailed = PowerVSImageState("failed")

	// PowerVSImageStateImporting is the string representing an image in a failed state.
	PowerVSImageStateImporting = PowerVSImageState("importing")
)

// ServiceInstanceState describes the state of a service instance.
type ServiceInstanceState string

var (
	// ServiceInstanceStateActive is the string representing a service instance in an active state.
	ServiceInstanceStateActive = ServiceInstanceState("active")

	// ServiceInstanceStateProvisioning is the string representing a service instance in a provisioning state.
	ServiceInstanceStateProvisioning = ServiceInstanceState("provisioning")

	// ServiceInstanceStateFailed is the string representing a service instance in a failed state.
	ServiceInstanceStateFailed = ServiceInstanceState("failed")

	// ServiceInstanceStateRemoved is the string representing a service instance in a removed state.
	ServiceInstanceStateRemoved = ServiceInstanceState("removed")
)

// TransitGatewayState describes the state of an IBM Transit Gateway.
type TransitGatewayState string

var (
	// TransitGatewayStateAvailable is the string representing a transit gateway in available state.
	TransitGatewayStateAvailable = TransitGatewayState("available")

	// TransitGatewayStatePending is the string representing a transit gateway in pending state.
	TransitGatewayStatePending = TransitGatewayState("pending")

	// TransitGatewayStateFailed is the string representing a transit gateway in failed state.
	TransitGatewayStateFailed = TransitGatewayState("failed")

	// TransitGatewayStateDeletePending is the string representing a transit gateway in deleting state.
	TransitGatewayStateDeletePending = TransitGatewayState("deleting")
)

// TransitGatewayConnectionState describes the state of an IBM Transit Gateway connection.
type TransitGatewayConnectionState string

var (
	// TransitGatewayConnectionStateAttached is the string representing a transit gateway connection in attached state.
	TransitGatewayConnectionStateAttached = TransitGatewayConnectionState("attached")

	// TransitGatewayConnectionStateFailed is the string representing a transit gateway connection in failed state.
	TransitGatewayConnectionStateFailed = TransitGatewayConnectionState("failed")

	// TransitGatewayConnectionStatePending is the string representing a transit gateway connection in pending state.
	TransitGatewayConnectionStatePending = TransitGatewayConnectionState("pending")

	// TransitGatewayConnectionStateDeleting is the string representing a transit gateway connection in deleting state.
	TransitGatewayConnectionStateDeleting = TransitGatewayConnectionState("deleting")
)

// VPCLoadBalancerState describes the state of the load balancer.
type VPCLoadBalancerState string

var (
	// VPCLoadBalancerStateActive is the string representing the load balancer in a active state.
	VPCLoadBalancerStateActive = VPCLoadBalancerState("active")

	// VPCLoadBalancerStateCreatePending is the string representing the load balancer in a queued state.
	VPCLoadBalancerStateCreatePending = VPCLoadBalancerState("create_pending")

	// VPCLoadBalancerStateDeletePending is the string representing the load balancer in deleting state.
	VPCLoadBalancerStateDeletePending = VPCLoadBalancerState("delete_pending")
)

// VPCSubnetState describes the state of a VPC Subnet.
type VPCSubnetState string

var (
	// VPCSubnetStateDeleting is the string representing a VPC subnet in deleting state.
	VPCSubnetStateDeleting = VPCSubnetState("deleting")
)

// VPCState describes the state of a VPC.
type VPCState string

var (
	// VPCStatePending is the string representing a VPC in pending state.
	VPCStatePending = VPCState("pending")

	// VPCStateDeleting is the string representing a VPC in deleting state.
	VPCStateDeleting = VPCState("deleting")
)

// DHCPServerState describes the state of the DHCP Server.
type DHCPServerState string

var (
	// DHCPServerStateActive indicates the active state of DHCP server.
	DHCPServerStateActive = DHCPServerState("ACTIVE")

	// DHCPServerStateBuild indicates the build state of DHCP server.
	DHCPServerStateBuild = DHCPServerState("BUILD")

	// DHCPServerStateError indicates the error state of DHCP server.
	DHCPServerStateError = DHCPServerState("ERROR")
)

// DeletePolicy defines the policy used to identify images to be preserved.
type DeletePolicy string

var (
	// DeletePolicyRetain is the string representing an image to be retained.
	DeletePolicyRetain = DeletePolicy("retain")
)

// ResourceType describes IBM Cloud resource name.
type ResourceType string

var (
	// ResourceTypeServiceInstance is Power VS service instance resource.
	ResourceTypeServiceInstance = ResourceType("serviceInstance")
	// ResourceTypeNetwork is Power VS network resource.
	ResourceTypeNetwork = ResourceType("network")
	// ResourceTypeDHCPServer is Power VS DHCP server.
	ResourceTypeDHCPServer = ResourceType("dhcpServer")
	// ResourceTypeLoadBalancer VPC loadBalancer resource.
	ResourceTypeLoadBalancer = ResourceType("loadBalancer")
	// ResourceTypeTransitGateway is transit gateway resource.
	ResourceTypeTransitGateway = ResourceType("transitGateway")
	// ResourceTypeVPC is Power VS network resource.
	ResourceTypeVPC = ResourceType("vpc")
	// ResourceTypeSubnet is VPC subnet resource.
	ResourceTypeSubnet = ResourceType("subnet")
	// ResourceTypeCOSInstance is IBM COS instance resource.
	ResourceTypeCOSInstance = ResourceType("cosInstance")
	// ResourceTypeCOSBucket is IBM COS bucket resource.
	ResourceTypeCOSBucket = ResourceType("cosBucket")
	// ResourceTypeResourceGroup is IBM Resource Group.
	ResourceTypeResourceGroup = ResourceType("resourceGroup")
)

// NetworkInterface holds the network interface information like subnet id.
type NetworkInterface struct {
	// Subnet ID of the network interface.
	Subnet string `json:"subnet,omitempty"`
}

// Subnet describes a subnet.
type Subnet struct {
	Ipv4CidrBlock *string `json:"cidr,omitempty"`
	Name          *string `json:"name,omitempty"`
	ID            *string `json:"id,omitempty"`
	Zone          *string `json:"zone,omitempty"`
}

// VPCEndpoint describes a VPCEndpoint.
type VPCEndpoint struct {
	Address *string `json:"address"`
	// +optional
	// Deprecated: This field has no function and is going to be removed in the next release.
	FIPID *string `json:"floatingIPID,omitempty"`
	// +optional
	LBID *string `json:"loadBalancerIPID,omitempty"`
}
