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

import "github.com/IBM/vpc-go-sdk/vpcv1"

// VPC Security Group related fields.

const (
	// VPCSecurityGroupRuleProtocolAllType is a string representation of the 'SecurityGroupRuleSecurityGroupRuleProtocolAll' type.
	VPCSecurityGroupRuleProtocolAllType = "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll"

	// VPCSecurityGroupRuleProtocolIcmpType is a string representation of the 'SecurityGroupRuleSecurityGroupRuleProtocolIcmp' type.
	VPCSecurityGroupRuleProtocolIcmpType = "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp"

	// VPCSecurityGroupRuleProtocolTcpudpType is a string representation of the 'SecurityGroupRuleSecurityGroupRuleProtocolTcpudp' type.
	VPCSecurityGroupRuleProtocolTcpudpType = "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp"
)

// VPCSecurityGroupRuleAction represents the actions for a Security Group Rule.
// +kubebuilder:validation:Enum=allow;deny
type VPCSecurityGroupRuleAction string

const (
	// VPCSecurityGroupRuleActionAllow defines that the Rule should allow traffic.
	VPCSecurityGroupRuleActionAllow VPCSecurityGroupRuleAction = vpcv1.NetworkACLRuleActionAllowConst
	// VPCSecurityGroupRuleActionDeny defines that the Rule should deny traffic.
	VPCSecurityGroupRuleActionDeny VPCSecurityGroupRuleAction = vpcv1.NetworkACLRuleActionDenyConst
)

// VPCSecurityGroupRuleDirection represents the directions for a Security Group Rule.
// +kubebuilder:validation:Enum=inbound;outbound
type VPCSecurityGroupRuleDirection string

const (
	// VPCSecurityGroupRuleDirectionInbound defines the Rule is for inbound traffic.
	VPCSecurityGroupRuleDirectionInbound VPCSecurityGroupRuleDirection = vpcv1.NetworkACLRuleDirectionInboundConst
	// VPCSecurityGroupRuleDirectionOutbound defines the Rule is for outbound traffic.
	VPCSecurityGroupRuleDirectionOutbound VPCSecurityGroupRuleDirection = vpcv1.NetworkACLRuleDirectionOutboundConst
)

// VPCSecurityGroupRuleProtocol represents the protocols for a Security Group Rule.
// +kubebuilder:validation:Enum=all;icmp;tcp;udp
type VPCSecurityGroupRuleProtocol string

const (
	// VPCSecurityGroupRuleProtocolAll defines the Rule is for all network protocols.
	VPCSecurityGroupRuleProtocolAll VPCSecurityGroupRuleProtocol = vpcv1.NetworkACLRuleProtocolAllConst
	// VPCSecurityGroupRuleProtocolIcmp defiens the Rule is for ICMP network protocol.
	VPCSecurityGroupRuleProtocolIcmp VPCSecurityGroupRuleProtocol = vpcv1.NetworkACLRuleProtocolIcmpConst
	// VPCSecurityGroupRuleProtocolTCP defines the Rule is for TCP network protocol.
	VPCSecurityGroupRuleProtocolTCP VPCSecurityGroupRuleProtocol = vpcv1.NetworkACLRuleProtocolTCPConst
	// VPCSecurityGroupRuleProtocolUDP defines the Rule is for UDP network protocol.
	VPCSecurityGroupRuleProtocolUDP VPCSecurityGroupRuleProtocol = vpcv1.NetworkACLRuleProtocolUDPConst
)

// VPCSecurityGroupRuleRemoteType represents the type of Security Group Rule's destination or source is
// intended. This is intended to define the VPCSecurityGroupRulePrototype subtype.
// For example:
// - any - Any source or destination (0.0.0.0/0)
// - cidr - A CIDR representing a set of IP's (10.0.0.0/28)
// - address - A specific address (192.168.0.1)
// - sg - A Security Group.
// +kubebuilder:validation:Enum=any;cidr;address;sg
type VPCSecurityGroupRuleRemoteType string

const (
	// VPCSecurityGroupRuleRemoteTypeAny defines the destination or source for the Rule is anything/anywhere.
	VPCSecurityGroupRuleRemoteTypeAny VPCSecurityGroupRuleRemoteType = VPCSecurityGroupRuleRemoteType("any")
	// VPCSecurityGroupRuleRemoteTypeCIDR defines the destination or source for the Rule is a CIDR block.
	VPCSecurityGroupRuleRemoteTypeCIDR VPCSecurityGroupRuleRemoteType = VPCSecurityGroupRuleRemoteType("cidr")
	// VPCSecurityGroupRuleRemoteTypeAddress defines the destination or source for the Rule is an address.
	VPCSecurityGroupRuleRemoteTypeAddress VPCSecurityGroupRuleRemoteType = VPCSecurityGroupRuleRemoteType("address")
	// VPCSecurityGroupRuleRemoteTypeSG defines the destination or source for the Rule is a VPC Security Group.
	VPCSecurityGroupRuleRemoteTypeSG VPCSecurityGroupRuleRemoteType = VPCSecurityGroupRuleRemoteType("sg")
)

// VPCSecurityGroupSource defines a VPC Security Group that should exist or be created.
// +kubebuilder:validation:XValidation:rule="self.type == 'Reference' ? has(self.reference) : !has(self.reference)",message="reference configuration is required when type is Reference, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="self.type == 'Provision' ? has(self.provision) : !has(self.provision)",message="provision configuration is required when type is Provision, and forbidden otherwise"
type VPCSecurityGroupSource struct {
	// Type defines whether to use an existing Security Group or provision a new one.
	// +required
	// +kubebuilder:validation:Enum=Reference;Provision
	Type SourceType `json:"type,omitempty"`

	// Reference contains the information to identify an existing Security Group.
	// CAPI will not manage rules for referenced Security Groups.
	// +optional
	Reference ResourceIdentifier `json:"reference,omitempty"`

	// Provision contains the configuration for provisioning a new Security Group.
	// +optional
	Provision VPCSecurityGroupProvision `json:"provision,omitempty"`
}

// VPCSecurityGroupProvision holds the configuration for creating a new Security Group.
type VPCSecurityGroupProvision struct {
	// Name of the Security Group.
	// +optional
	Name string `json:"name,omitempty"`

	// Rules are the Security Group Rules for the Security Group.
	// +optional
	Rules []VPCSecurityGroupRule `json:"rules,omitempty"`

	// Tags are tags to add to the Security Group.
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// VPCSecurityGroupRule defines a VPC Security Group Rule for a specified Security Group.
// +kubebuilder:validation:XValidation:rule="(has(self.destination) && !has(self.source)) || (!has(self.destination) && has(self.source))",message="both destination and source cannot be provided"
// +kubebuilder:validation:XValidation:rule="self.direction == 'inbound' ? has(self.source) : true",message="source must be set for VPCSecurityGroupRuleDirectionInbound direction"
// +kubebuilder:validation:XValidation:rule="self.direction == 'inbound' ? !has(self.destination) : true",message="destination is not valid for VPCSecurityGroupRuleDirectionInbound direction"
// +kubebuilder:validation:XValidation:rule="self.direction == 'outbound' ? has(self.destination) : true",message="destination must be set for VPCSecurityGroupRuleDirectionOutbound direction"
// +kubebuilder:validation:XValidation:rule="self.direction == 'outbound' ? !has(self.source) : true",message="source is not valid for VPCSecurityGroupRuleDirectionOutbound direction"
type VPCSecurityGroupRule struct {
	// Action defines whether to allow or deny traffic defined by the Security Group Rule.
	// +required
	Action VPCSecurityGroupRuleAction `json:"action,omitempty"`

	// Destination defines the destination of outbound traffic for the Security Group Rule.
	// Only used when direction is VPCSecurityGroupRuleDirectionOutbound.
	// +optional
	Destination VPCSecurityGroupRulePrototype `json:"destination,omitempty,omitzero"`

	// Direction defines whether the traffic is inbound or outbound for the Security Group Rule.
	// +required
	Direction VPCSecurityGroupRuleDirection `json:"direction,omitempty"`

	// SecurityGroupID is the ID of the Security Group for the Security Group Rule.
	// +optional
	SecurityGroupID string `json:"securityGroupID,omitempty"`

	// Source defines the source of inbound traffic for the Security Group Rule.
	// Only used when direction is VPCSecurityGroupRuleDirectionInbound.
	// +optional
	Source VPCSecurityGroupRulePrototype `json:"source,omitempty,omitzero"`
}

// VPCSecurityGroupRuleRemote defines a VPC Security Group Rule's remote details.
// +kubebuilder:validation:XValidation:rule="self.remoteType == 'any' ? (!has(self.cidrSubnetName) && !has(self.address) && !has(self.securityGroupName)) : true",message="cidrSubnetName, address, and securityGroupName are not valid for VPCSecurityGroupRuleRemoteTypeAny remoteType"
// +kubebuilder:validation:XValidation:rule="self.remoteType == 'cidr' ? (has(self.cidrSubnetName) && !has(self.address) && !has(self.securityGroupName)) : true",message="only cidrSubnetName is valid for VPCSecurityGroupRuleRemoteTypeCIDR remoteType"
// +kubebuilder:validation:XValidation:rule="self.remoteType == 'address' ? (has(self.address) && !has(self.cidrSubnetName) && !has(self.securityGroupName)) : true",message="only address is valid for VPCSecurityGroupRuleRemoteTypeAddress remoteType"
// +kubebuilder:validation:XValidation:rule="self.remoteType == 'sg' ? (has(self.securityGroupName) && !has(self.cidrSubnetName) && !has(self.address)) : true",message="only securityGroupName is valid for VPCSecurityGroupRuleRemoteTypeSG remoteType"
type VPCSecurityGroupRuleRemote struct {
	// CIDRSubnetName is the name of the VPC Subnet to retrieve the CIDR from.
	// +optional
	CIDRSubnetName string `json:"cidrSubnetName,omitempty"`

	// Address is the address to use for the remote's destination/source.
	// +optional
	Address string `json:"address,omitempty"`

	// RemoteType defines the type of filter to define for the remote's destination/source.
	// +required
	RemoteType VPCSecurityGroupRuleRemoteType `json:"remoteType,omitempty"`

	// SecurityGroupName is the name of the VPC Security Group to use for the remote.
	// +optional
	SecurityGroupName string `json:"securityGroupName,omitempty"`
}

// VPCSecurityGroupPortRange represents a range of ports, minimum to maximum.
// +kubebuilder:validation:XValidation:rule="self.maximumPort >= self.minimumPort",message="maximum port must be greater than or equal to minimum port"
type VPCSecurityGroupPortRange struct {
	// MaximumPort is the inclusive upper range of ports.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	MaximumPort int64 `json:"maximumPort,omitempty"`

	// MinimumPort is the inclusive lower range of ports.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	MinimumPort int64 `json:"minimumPort,omitempty"`
}

// VPCSecurityGroupRulePrototype defines a VPC Security Group Rule's traffic specifics.
// +kubebuilder:validation:XValidation:rule="self.protocol != 'icmp' ? (!has(self.icmpCode) && !has(self.icmpType)) : true",message="icmpCode and icmpType are only supported for VPCSecurityGroupRuleProtocolIcmp protocol"
// +kubebuilder:validation:XValidation:rule="self.protocol == 'all' ? !has(self.portRange) : true",message="portRange is not valid for VPCSecurityGroupRuleProtocolAll protocol"
// +kubebuilder:validation:XValidation:rule="self.protocol == 'icmp' ? !has(self.portRange) : true",message="portRange is not valid for VPCSecurityGroupRuleProtocolIcmp protocol"
type VPCSecurityGroupRulePrototype struct {
	// ICMPCode is the ICMP code for the Rule.
	// +optional
	ICMPCode *int64 `json:"icmpCode,omitempty"`

	// ICMPType is the ICMP type for the Rule.
	// +optional
	ICMPType *int64 `json:"icmpType,omitempty"`

	// PortRange is a range of ports allowed for the Rule's remote.
	// +optional
	PortRange VPCSecurityGroupPortRange `json:"portRange,omitempty,omitzero"`

	// Protocol defines the traffic protocol used for the Security Group Rule.
	// +required
	Protocol VPCSecurityGroupRuleProtocol `json:"protocol,omitempty"`

	// Remotes is a set of VPCSecurityGroupRuleRemote's that define the traffic allowed.
	// +optional
	Remotes []VPCSecurityGroupRuleRemote `json:"remotes,omitempty"`
}

// VPCSecurityGroupStatus tracks the observed state of an individual VPC security group.
type VPCSecurityGroupStatus struct {
	// ID is the unique cloud identifier (GUID) generated by IBM Cloud for this Security Group.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`

	// Name is the human-readable unique identifier assigned to the Security Group.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// Rules tracks the synchronized IDs of the rules belonging to this security group.
	// Tracking rule IDs ensures we can cleanly reconcile, update, or remove rules later.
	// +optional
	Rules []VPCSecurityGroupRuleStatus `json:"rules,omitempty"`
}

// VPCSecurityGroupRuleStatus tracks individual security group rule identifiers returned by the API.
type VPCSecurityGroupRuleStatus struct {
	// ID is the unique string identifier generated by IBM Cloud for this specific rule.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
}
