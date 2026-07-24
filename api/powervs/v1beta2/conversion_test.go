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

package v1beta2

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/randfill"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"

	. "github.com/onsi/gomega"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for IBMPowerVSCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.IBMPowerVSCluster{},
		Spoke:       &IBMPowerVSCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{IBMPowerVSClusterFuzzFuncs},
	}))
	t.Run("for IBMPowerVSClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.IBMPowerVSClusterTemplate{},
		Spoke:       &IBMPowerVSClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{IBMPowerVSClusterTemplateFuzzFuncs},
	}))
	t.Run("for IBMPowerVSMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.IBMPowerVSMachine{},
		Spoke:       &IBMPowerVSMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{IBMPowerVSMachineFuzzFuncs},
	}))
	t.Run("for IBMPowerVSMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.IBMPowerVSMachineTemplate{},
		Spoke:       &IBMPowerVSMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{IBMPowerVSMachineTemplateFuzzFuncs},
	}))
	t.Run("for IBMPowerVSImage", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.IBMPowerVSImage{},
		Spoke:       &IBMPowerVSImage{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{IBMPowerVSImageFuzzFuncs},
	}))
}

func IBMPowerVSClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSClusterStatus,
		hubIBMPowerVSClusterSpec,
		spokeIBMPowerVSClusterStatus,
		spokeIBMPowerVSClusterSpec,
	}
}

func hubIBMPowerVSClusterStatus(in *infrav1.IBMPowerVSClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSClusterV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}

	in.Workspace.Name = ""
	in.Network.Name = ""
	in.ResourceGroup.Name = ""
	in.Network.DHCPServer.Name = ""

	if in.Initialization.Provisioned != nil && !*in.Initialization.Provisioned {
		in.Initialization.Provisioned = nil
	}

	if in.TransitGateway.Name != "" || in.TransitGateway.VPCConnection.Name != "" ||
		in.TransitGateway.VPCConnection.State != "" || in.TransitGateway.PowerVSConnection.Name != "" ||
		in.TransitGateway.PowerVSConnection.State != "" {
		in.TransitGateway.Name = ""
		in.TransitGateway.VPCConnection.Name = ""
		in.TransitGateway.VPCConnection.State = ""
		in.TransitGateway.PowerVSConnection.Name = ""
		in.TransitGateway.PowerVSConnection.State = ""
	}

	if len(in.VPCSubnets) == 0 {
		in.VPCSubnets = nil
	}
	if len(in.LoadBalancers) == 0 {
		in.LoadBalancers = nil
	}
	for i := range in.VPCSubnets {
		if in.VPCSubnets[i].ID == "" || in.VPCSubnets[i].Name == "" {
			in.VPCSubnets = nil
			break
		}
		if in.VPCSubnets[i].Zone != "" && in.VPCSubnets[i].ID != "" && in.VPCSubnets[i].Name != "" {
			in.VPCSubnets[i].Zone = ""
		}
	}
	for i := range in.LoadBalancers {
		if in.LoadBalancers[i].Name == "" {
			in.LoadBalancers = nil
			break
		}
	}
	in.VPC = infrav1.VPCStatus{}

	// COSInstance: only ID survives round-trip through v1beta2 (which uses *ResourceReference with only ID).
	// Name, BucketName, and BucketRegion are not stored in v1beta2 Status.COSInstance.
	in.COSInstance.Name = ""
	in.COSInstance.BucketName = ""
	in.COSInstance.BucketRegion = ""
	if in.COSInstance.ID == "" {
		in.COSInstance = infrav1.COSInstanceStatus{}
	}

	// VPCSecurityGroups: v1beta2 status is map[string]VPCSecurityGroupStatus keyed by Name.
	// When Name is empty the ID is used as the map key; on the return trip that becomes Name.
	// So if Name is empty but ID is set, set Name = ID to ensure round-trip equality.
	for i := range in.VPCSecurityGroups {
		sg := &in.VPCSecurityGroups[i]
		if sg.Name == "" && sg.ID != "" {
			sg.Name = sg.ID
		}
		// Normalize empty Rules entries that won't survive
		for j := len(sg.Rules) - 1; j >= 0; j-- {
			if sg.Rules[j].ID == "" {
				sg.Rules = append(sg.Rules[:j], sg.Rules[j+1:]...)
			}
		}
		if len(sg.Rules) == 0 {
			sg.Rules = nil
		}
	}
	// Drop entries missing both Name and ID (can't be keyed in v1beta2 map)
	filtered := in.VPCSecurityGroups[:0]
	for i := range in.VPCSecurityGroups {
		if in.VPCSecurityGroups[i].Name == "" && in.VPCSecurityGroups[i].ID == "" {
			continue
		}
		filtered = append(filtered, in.VPCSecurityGroups[i])
	}
	if len(filtered) == 0 {
		in.VPCSecurityGroups = nil
	} else {
		in.VPCSecurityGroups = filtered
	}
}

func hubIBMPowerVSClusterSpec(in *infrav1.IBMPowerVSClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	switch in.Topology {
	case infrav1.PowerVSVirtualIPTopology, infrav1.PowerVSLoadBalancerTopology:
	default:
		in.Topology = ""
	}

	switch in.Workspace.Type {
	case infrav1.SourceTypeReference:
		in.Workspace.Provision = infrav1.WorkspaceProvisionConfig{}
	case infrav1.SourceTypeProvision:
		in.Workspace.Reference = infrav1.ResourceIdentifier{}
	default:
		if in.Workspace.Reference.ID != "" || in.Workspace.Reference.Name != "" {
			in.Workspace.Type = infrav1.SourceTypeReference
			in.Workspace.Provision = infrav1.WorkspaceProvisionConfig{}
		} else {
			in.Workspace.Type = infrav1.SourceTypeProvision
			in.Workspace.Reference = infrav1.ResourceIdentifier{}
			in.Workspace.Provision = infrav1.WorkspaceProvisionConfig{}
		}
	}

	if in.Workspace.Type == infrav1.SourceTypeReference && in.Workspace.Reference.ID == "" && in.Workspace.Reference.Name == "" {
		in.Workspace.Reference.ID = "fuzzed-workspace-id"
	}

	switch in.ResourceGroup.Type {
	case infrav1.SourceTypeReference:
		if in.ResourceGroup.Reference.ID == "" && in.ResourceGroup.Reference.Name == "" {
			in.ResourceGroup.Reference.ID = "fuzzed-resource-group-id"
		}
	case "":
		in.ResourceGroup.Reference = infrav1.ResourceIdentifier{}
	default:
		in.ResourceGroup.Type = ""
		in.ResourceGroup.Reference = infrav1.ResourceIdentifier{}
	}

	switch in.Network.Type {
	case infrav1.SourceTypeReference:
		in.Network.Provision = infrav1.NetworkProvisionConfig{}
	case infrav1.SourceTypeProvision:
		in.Network.Reference = infrav1.ResourceIdentifier{}
	default:
		in.Network.Type = ""
		in.Network.Reference = infrav1.ResourceIdentifier{}
		in.Network.Provision = infrav1.NetworkProvisionConfig{}
	}

	in.Network.Provision.DHCPServer.Name = ""

	switch in.TransitGateway.Type {
	case infrav1.SourceTypeReference:
		in.TransitGateway.Provision = infrav1.TransitGatewayProvision{}
		if in.TransitGateway.Reference.ID == "" && in.TransitGateway.Reference.Name == "" {
			in.TransitGateway.Reference.ID = "fuzzed-tg-id"
		}
	case infrav1.SourceTypeProvision:
		in.TransitGateway.Reference = infrav1.ResourceIdentifier{}
		if in.TransitGateway.Provision.GlobalRouting != "" &&
			in.TransitGateway.Provision.GlobalRouting != infrav1.TransitGatewayRoutingGlobal &&
			in.TransitGateway.Provision.GlobalRouting != infrav1.TransitGatewayRoutingLocal {
			in.TransitGateway.Provision.GlobalRouting = ""
		}
	default:
		in.TransitGateway.Type = ""
		in.TransitGateway.Reference = infrav1.ResourceIdentifier{}
		in.TransitGateway.Provision = infrav1.TransitGatewayProvision{}
	}

	in.TransitGateway.VPCConnection = infrav1.TransitGatewayConnectionSource{}
	in.TransitGateway.PowerVSConnection = infrav1.TransitGatewayConnectionSource{}

	// COSInstance: v1beta2 has no Type/Reference concept — only Name/BucketName/BucketRegion (always Provision).
	// Restrict hub to SourceTypeProvision or empty so hub-spoke-hub round-trips faithfully.
	// SourceTypeReference cannot survive round-trip through v1beta2.
	switch in.COSInstance.Type {
	case infrav1.SourceTypeProvision:
		in.COSInstance.Reference = infrav1.ResourceIdentifier{}
	default:
		// Treat unknown/Reference/empty as Provision if there's bucket data
		in.COSInstance.Reference = infrav1.ResourceIdentifier{}
		if in.COSInstance.BucketName != "" || in.COSInstance.BucketRegion != "" || in.COSInstance.Provision.Name != "" {
			in.COSInstance.Type = infrav1.SourceTypeProvision
		} else {
			in.COSInstance = infrav1.COSInstanceSource{}
		}
	}

	// Ignition: only Version survives round-trip; constrain to valid v1beta2 enum values
	if in.Ignition.Version != "" {
		switch in.Ignition.Version {
		case "2.3", "2.4", "3.0", "3.1", "3.2", "3.3", "3.4":
		default:
			in.Ignition.Version = "3.4"
		}
	}

	switch in.VPC.Type {
	case infrav1.SourceTypeReference:
		in.VPC.Provision = infrav1.VPCProvision{}
		if in.VPC.Reference.ID == "" && in.VPC.Reference.Name == "" {
			in.VPC.Reference.ID = "fuzzed-vpc-id"
		}
	case infrav1.SourceTypeProvision:
		in.VPC.Reference = infrav1.ResourceIdentifier{}
	default:
		if in.VPC.Reference.ID != "" {
			in.VPC.Type = infrav1.SourceTypeReference
			in.VPC.Provision = infrav1.VPCProvision{}
		} else if in.VPC.Region != "" || in.VPC.Reference.Name != "" || in.VPC.Provision.Name != "" {
			in.VPC.Type = infrav1.SourceTypeProvision
			in.VPC.Reference = infrav1.ResourceIdentifier{}
			if in.VPC.Provision.Name == "" {
				in.VPC.Provision.Name = in.VPC.Reference.Name
			}
		} else {
			// No ID, Name, or Region — nothing survives to v1beta2 (VPC will be nil).
			// On the way back, a nil v1beta2 VPC always produces Type=Provision, so
			// the hub must also carry Type=Provision to survive the round-trip.
			in.VPC.Type = infrav1.SourceTypeProvision
			in.VPC.Reference = infrav1.ResourceIdentifier{}
			in.VPC.Provision = infrav1.VPCProvision{}
		}
	}

	if len(in.VPCSubnets) == 0 {
		in.VPCSubnets = nil
	} else {
		for i := range in.VPCSubnets {
			switch in.VPCSubnets[i].Type {
			case infrav1.SourceTypeReference:
				in.VPCSubnets[i].Provision = infrav1.VPCSubnetProvision{}
				if in.VPCSubnets[i].Reference.ID == "" && in.VPCSubnets[i].Reference.Name == "" {
					in.VPCSubnets[i].Reference.ID = "fuzzed-subnet-id"
				}
			case infrav1.SourceTypeProvision:
				in.VPCSubnets[i].Reference = infrav1.ResourceIdentifier{}
			default:
				if in.VPCSubnets[i].Reference.ID != "" {
					in.VPCSubnets[i].Type = infrav1.SourceTypeReference
					in.VPCSubnets[i].Provision = infrav1.VPCSubnetProvision{}
				} else {
					in.VPCSubnets[i].Type = infrav1.SourceTypeProvision
					if in.VPCSubnets[i].Provision.Name == "" {
						in.VPCSubnets[i].Provision.Name = in.VPCSubnets[i].Reference.Name
					}
					in.VPCSubnets[i].Reference = infrav1.ResourceIdentifier{}
				}
			}
		}
	}
	if len(in.VPCSecurityGroups) == 0 {
		in.VPCSecurityGroups = nil
	} else {
		for i := range in.VPCSecurityGroups {
			switch in.VPCSecurityGroups[i].Type {
			case infrav1.SourceTypeReference:
				in.VPCSecurityGroups[i].Provision = infrav1.VPCSecurityGroupProvision{}
				if in.VPCSecurityGroups[i].Reference.ID == "" && in.VPCSecurityGroups[i].Reference.Name == "" {
					in.VPCSecurityGroups[i].Reference.ID = "fuzzed-sg-id"
				}
			case infrav1.SourceTypeProvision:
				in.VPCSecurityGroups[i].Reference = infrav1.ResourceIdentifier{}
				if len(in.VPCSecurityGroups[i].Provision.Rules) == 0 {
					in.VPCSecurityGroups[i].Provision.Rules = nil
				}
				if len(in.VPCSecurityGroups[i].Provision.Tags) == 0 {
					in.VPCSecurityGroups[i].Provision.Tags = nil
				}
				for j := range in.VPCSecurityGroups[i].Provision.Rules {
					rule := &in.VPCSecurityGroups[i].Provision.Rules[j]
					if len(rule.Destination.Remotes) == 0 {
						rule.Destination.Remotes = nil
					}
					if len(rule.Source.Remotes) == 0 {
						rule.Source.Remotes = nil
					}
				}
			default:
				if in.VPCSecurityGroups[i].Reference.ID != "" {
					in.VPCSecurityGroups[i].Type = infrav1.SourceTypeReference
					in.VPCSecurityGroups[i].Provision = infrav1.VPCSecurityGroupProvision{}
				} else {
					in.VPCSecurityGroups[i].Type = infrav1.SourceTypeProvision
					in.VPCSecurityGroups[i].Reference = infrav1.ResourceIdentifier{}
					in.VPCSecurityGroups[i].Provision.Rules = nil
					in.VPCSecurityGroups[i].Provision.Tags = nil
					in.VPCSecurityGroups[i].Provision.Name = ""
				}
			}
		}
		// If all entries collapsed to empty Provision, nil out the slice
		allEmpty := true
		for i := range in.VPCSecurityGroups {
			if in.VPCSecurityGroups[i].Type != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			in.VPCSecurityGroups = nil
		}
	}

	if len(in.LoadBalancers) == 0 {
		in.LoadBalancers = nil
	} else {
		for i := range in.LoadBalancers {
			switch in.LoadBalancers[i].Type {
			case infrav1.SourceTypeReference:
				in.LoadBalancers[i].Provision = infrav1.LoadBalancerProvision{}
				if in.LoadBalancers[i].Reference.ID == "" && in.LoadBalancers[i].Reference.Name == "" {
					in.LoadBalancers[i].Reference.ID = "fuzzed-lb-id"
				}
			case infrav1.SourceTypeProvision:
				in.LoadBalancers[i].Reference = infrav1.ResourceIdentifier{}
				if in.LoadBalancers[i].Provision.Type != "" &&
					in.LoadBalancers[i].Provision.Type != infrav1.LoadBalancerTypePublic &&
					in.LoadBalancers[i].Provision.Type != infrav1.LoadBalancerTypePrivate {
					in.LoadBalancers[i].Provision.Type = infrav1.LoadBalancerTypePrivate
				}
				if len(in.LoadBalancers[i].Provision.AdditionalListeners) == 0 {
					in.LoadBalancers[i].Provision.AdditionalListeners = nil
				}
				if len(in.LoadBalancers[i].Provision.BackendPools) == 0 {
					in.LoadBalancers[i].Provision.BackendPools = nil
				}
				if len(in.LoadBalancers[i].Provision.SecurityGroups) == 0 {
					in.LoadBalancers[i].Provision.SecurityGroups = nil
				}
				if len(in.LoadBalancers[i].Provision.Subnets) == 0 {
					in.LoadBalancers[i].Provision.Subnets = nil
				}
				if in.LoadBalancers[i].Provision.Name == "" {
					in.LoadBalancers[i].Provision.Type = ""
				}
			default:
				if in.LoadBalancers[i].Reference.ID != "" {
					in.LoadBalancers[i].Type = infrav1.SourceTypeReference
					in.LoadBalancers[i].Provision = infrav1.LoadBalancerProvision{}
				} else {
					in.LoadBalancers[i].Type = infrav1.SourceTypeProvision
					if in.LoadBalancers[i].Provision.Name == "" {
						in.LoadBalancers[i].Provision.Name = in.LoadBalancers[i].Reference.Name
					}
					in.LoadBalancers[i].Reference = infrav1.ResourceIdentifier{}
					if in.LoadBalancers[i].Provision.Type != "" &&
						in.LoadBalancers[i].Provision.Type != infrav1.LoadBalancerTypePublic &&
						in.LoadBalancers[i].Provision.Type != infrav1.LoadBalancerTypePrivate {
						in.LoadBalancers[i].Provision.Type = infrav1.LoadBalancerTypePrivate
					}
					if len(in.LoadBalancers[i].Provision.AdditionalListeners) == 0 {
						in.LoadBalancers[i].Provision.AdditionalListeners = nil
					}
					if in.LoadBalancers[i].Provision.Name == "" {
						in.LoadBalancers[i].Type = ""
						in.LoadBalancers[i].Provision = infrav1.LoadBalancerProvision{}
					}
				}
			}
		}
	}
}

func spokeIBMPowerVSClusterStatus(in *IBMPowerVSClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}

	if in.ServiceInstance != nil && in.ServiceInstance.ID == nil {
		id := "fuzzed-id"
		in.ServiceInstance.ID = &id
	}

	if in.ServiceInstance != nil && in.ServiceInstance.ID != nil && *in.ServiceInstance.ID == "" {
		in.ServiceInstance = nil
	}

	if in.ServiceInstance != nil {
		in.ServiceInstance.ControllerCreated = nil
	}
	if in.ResourceGroup != nil {
		in.ResourceGroup.ControllerCreated = nil
	}
	if in.Network != nil {
		in.Network.ControllerCreated = nil
	}
	if in.DHCPServer != nil {
		in.DHCPServer.ControllerCreated = nil
	}
	if in.VPC != nil {
		in.VPC.ControllerCreated = nil
		if in.VPC.ID != nil && *in.VPC.ID == "" {
			in.VPC.ID = nil
		}
		if in.VPC.ID == nil {
			in.VPC = nil
		}
	}
	if in.COSInstance != nil {
		in.COSInstance.ControllerCreated = nil
		if in.COSInstance.ID == nil || *in.COSInstance.ID == "" {
			in.COSInstance = nil
		}
	}
	if in.ResourceGroup != nil && (in.ResourceGroup.ID == nil || *in.ResourceGroup.ID == "") {
		in.ResourceGroup = nil
	}

	if in.Network != nil && (in.Network.ID == nil || *in.Network.ID == "") {
		in.Network = nil
	}
	if in.DHCPServer != nil && (in.DHCPServer.ID == nil || *in.DHCPServer.ID == "") {
		in.DHCPServer = nil
	}

	if in.TransitGateway != nil {
		in.TransitGateway.ControllerCreated = nil

		if in.TransitGateway.VPCConnection != nil {
			in.TransitGateway.VPCConnection.ControllerCreated = nil
			if in.TransitGateway.VPCConnection.ID == nil || *in.TransitGateway.VPCConnection.ID == "" {
				in.TransitGateway.VPCConnection = nil
			}
		}

		if in.TransitGateway.PowerVSConnection != nil {
			in.TransitGateway.PowerVSConnection.ControllerCreated = nil
			if in.TransitGateway.PowerVSConnection.ID == nil || *in.TransitGateway.PowerVSConnection.ID == "" {
				in.TransitGateway.PowerVSConnection = nil
			}
		}

		if in.TransitGateway.ID != nil && *in.TransitGateway.ID == "" {
			in.TransitGateway.ID = nil
		}

		if (in.TransitGateway.ID == nil || *in.TransitGateway.ID == "") &&
			in.TransitGateway.VPCConnection == nil &&
			in.TransitGateway.PowerVSConnection == nil {
			in.TransitGateway = nil
		}
	}
	for name, subnet := range in.VPCSubnet {
		subnet.ControllerCreated = nil
		if subnet.ID != nil && *subnet.ID == "" {
			subnet.ID = nil
		}
		if subnet.ID == nil || name == "" {
			delete(in.VPCSubnet, name)
			continue
		}
		in.VPCSubnet[name] = subnet
	}
	if len(in.VPCSubnet) == 0 {
		in.VPCSubnet = nil
	}
	for name, lb := range in.LoadBalancers {
		lb.ControllerCreated = nil
		if lb.ID != nil && *lb.ID == "" {
			lb.ID = nil
		}
		if lb.Hostname != nil && *lb.Hostname == "" {
			lb.Hostname = nil
		}
		if name == "" {
			delete(in.LoadBalancers, name)
			continue
		}
		in.LoadBalancers[name] = lb
	}
	if len(in.LoadBalancers) == 0 {
		in.LoadBalancers = nil
	}

	for name, sg := range in.VPCSecurityGroups {
		sg.ControllerCreated = nil
		// Drop nil RuleID entries — they won't survive the round-trip
		filtered := sg.RuleIDs[:0]
		for _, rid := range sg.RuleIDs {
			if rid != nil && *rid != "" {
				filtered = append(filtered, rid)
			}
		}
		if len(filtered) == 0 {
			sg.RuleIDs = nil
		} else {
			sg.RuleIDs = filtered
		}
		// Empty ID means this entry can't survive (no key data)
		if sg.ID != nil && *sg.ID == "" {
			sg.ID = nil
		}
		if name == "" || (sg.ID == nil) {
			delete(in.VPCSecurityGroups, name)
			continue
		}
		in.VPCSecurityGroups[name] = sg
	}
	if len(in.VPCSecurityGroups) == 0 {
		in.VPCSecurityGroups = nil
	}
}

func spokeIBMPowerVSClusterSpec(in *IBMPowerVSClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.Zone != nil && *in.Zone == "" {
		in.Zone = nil
	}

	if in.ServiceInstance != nil {
		in.ServiceInstance.RegEx = nil // Tell fuzzer we intentionally drop RegEx in v1beta3
		// Empty string Name should be nil
		if in.ServiceInstance.Name != nil && *in.ServiceInstance.Name == "" {
			in.ServiceInstance.Name = nil
		}
		// Empty string ID should be nil
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID == "" {
			in.ServiceInstance.ID = nil
		}
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID != "" {
			in.ServiceInstanceID = *in.ServiceInstance.ID
		}
		// If ServiceInstance is empty, set to nil
		if in.ServiceInstance.ID == nil && in.ServiceInstance.Name == nil {
			in.ServiceInstance = nil
		}
	}
	// Ensure ServiceInstance is set when ServiceInstanceID is set
	if in.ServiceInstanceID != "" {
		if in.ServiceInstance == nil {
			in.ServiceInstance = &IBMPowerVSResourceReference{}
		}
		if in.ServiceInstance.ID == nil || *in.ServiceInstance.ID == "" {
			id := in.ServiceInstanceID
			in.ServiceInstance.ID = &id
		}
	} else {
		// If ServiceInstanceID is empty, ServiceInstance should be nil
		in.ServiceInstance = nil
	}

	if in.ResourceGroup != nil {
		in.ResourceGroup.RegEx = nil // Tell fuzzer we intentionally drop RegEx in v1beta3.
		if in.ResourceGroup.Name != nil && *in.ResourceGroup.Name == "" {
			in.ResourceGroup.Name = nil
		}
		if in.ResourceGroup.ID != nil && *in.ResourceGroup.ID == "" {
			in.ResourceGroup.ID = nil
		}
		if in.ResourceGroup.ID == nil && in.ResourceGroup.Name == nil {
			in.ResourceGroup = nil
		}
	}

	// Network.RegEx is not preserved in v1beta3, so clear it for round-trip
	in.Network.RegEx = nil

	// Empty string ID should be nil in Network
	if in.Network.ID != nil && *in.Network.ID == "" {
		in.Network.ID = nil
	}
	// Empty string Name should be nil in Network
	if in.Network.Name != nil && *in.Network.Name == "" {
		in.Network.Name = nil
	}

	// If Network has ID or Name (reference), DHCPServer should be nil
	if (in.Network.ID != nil && *in.Network.ID != "") || (in.Network.Name != nil && *in.Network.Name != "") {
		in.DHCPServer = nil
	}

	// DHCPServer.ID is not preserved in v1beta3 (it's a spec field in v1beta2 but not in v1beta3), so clear it
	if in.DHCPServer != nil {
		in.DHCPServer.ID = nil
		// Empty string fields should be nil
		if in.DHCPServer.Name != nil && *in.DHCPServer.Name == "" {
			in.DHCPServer.Name = nil
		}
		if in.DHCPServer.Cidr != nil && *in.DHCPServer.Cidr == "" {
			in.DHCPServer.Cidr = nil
		}
		if in.DHCPServer.DNSServer != nil && *in.DHCPServer.DNSServer == "" {
			in.DHCPServer.DNSServer = nil
		}
	}

	// Normalize only fields that are intentionally unsupported across versions.
	for i := range in.VPCSubnets {
		in.VPCSubnets[i].Ipv4CidrBlock = nil
		if in.VPCSubnets[i].ID != nil && *in.VPCSubnets[i].ID == "" {
			in.VPCSubnets[i].ID = nil
		}
		if in.VPCSubnets[i].Name != nil && *in.VPCSubnets[i].Name == "" {
			in.VPCSubnets[i].Name = nil
		}
		if in.VPCSubnets[i].Zone != nil && *in.VPCSubnets[i].Zone == "" {
			in.VPCSubnets[i].Zone = nil
		}
		if in.VPCSubnets[i].ID == nil && in.VPCSubnets[i].Name == nil {
			in.VPCSubnets[i].Zone = nil
		}
	}
	if len(in.VPCSubnets) == 0 {
		in.VPCSubnets = nil
	}

	if in.VPC != nil {
		if in.VPC.ID != nil && *in.VPC.ID == "" {
			in.VPC.ID = nil
		}
		if in.VPC.Name != nil && *in.VPC.Name == "" {
			in.VPC.Name = nil
		}
		if in.VPC.Region != nil && *in.VPC.Region == "" {
			in.VPC.Region = nil
		}
		if in.VPC.ID == nil && in.VPC.Name == nil && in.VPC.Region == nil {
			in.VPC = nil
		}
	}

	for i := range in.LoadBalancers {
		if in.LoadBalancers[i].ID != nil && *in.LoadBalancers[i].ID == "" {
			in.LoadBalancers[i].ID = nil
		}
		for j := range in.LoadBalancers[i].AdditionalListeners {
			if in.LoadBalancers[i].AdditionalListeners[j].DefaultPoolName != nil &&
				*in.LoadBalancers[i].AdditionalListeners[j].DefaultPoolName == "" {
				in.LoadBalancers[i].AdditionalListeners[j].DefaultPoolName = nil
			}
			if in.LoadBalancers[i].AdditionalListeners[j].Protocol != nil &&
				*in.LoadBalancers[i].AdditionalListeners[j].Protocol == "" {
				in.LoadBalancers[i].AdditionalListeners[j].Protocol = nil
			}
		}
		if in.LoadBalancers[i].Public != nil {
			in.LoadBalancers[i].ID = nil
		}
		if in.LoadBalancers[i].ID != nil {
			in.LoadBalancers[i].ID = nil
		}
		if in.LoadBalancers[i].Name == "" {
			in.LoadBalancers[i].ID = nil
			in.LoadBalancers[i].Public = nil
			in.LoadBalancers[i].SecurityGroups = nil
		}
		if len(in.LoadBalancers[i].AdditionalListeners) == 0 {
			in.LoadBalancers[i].AdditionalListeners = nil
		}
		if len(in.LoadBalancers[i].BackendPools) == 0 {
			in.LoadBalancers[i].BackendPools = nil
		}
		if len(in.LoadBalancers[i].SecurityGroups) == 0 {
			in.LoadBalancers[i].SecurityGroups = nil
		}
		if len(in.LoadBalancers[i].Subnets) == 0 {
			in.LoadBalancers[i].Subnets = nil
		}
		for j := range in.LoadBalancers[i].BackendPools {
			if in.LoadBalancers[i].BackendPools[j].Name != nil && *in.LoadBalancers[i].BackendPools[j].Name == "" {
				in.LoadBalancers[i].BackendPools[j].Name = nil
			}
			if in.LoadBalancers[i].BackendPools[j].HealthMonitor.URLPath != nil && *in.LoadBalancers[i].BackendPools[j].HealthMonitor.URLPath == "" {
				in.LoadBalancers[i].BackendPools[j].HealthMonitor.URLPath = nil
			}
		}
		for j := range in.LoadBalancers[i].SecurityGroups {
			if in.LoadBalancers[i].SecurityGroups[j].ID != nil && *in.LoadBalancers[i].SecurityGroups[j].ID == "" {
				in.LoadBalancers[i].SecurityGroups[j].ID = nil
			}
			if in.LoadBalancers[i].SecurityGroups[j].Name != nil && *in.LoadBalancers[i].SecurityGroups[j].Name == "" {
				in.LoadBalancers[i].SecurityGroups[j].Name = nil
			}
		}
		for j := range in.LoadBalancers[i].Subnets {
			if in.LoadBalancers[i].Subnets[j].ID != nil && *in.LoadBalancers[i].Subnets[j].ID == "" {
				in.LoadBalancers[i].Subnets[j].ID = nil
			}
			if in.LoadBalancers[i].Subnets[j].Name != nil && *in.LoadBalancers[i].Subnets[j].Name == "" {
				in.LoadBalancers[i].Subnets[j].Name = nil
			}
		}
	}
	if len(in.LoadBalancers) == 0 {
		in.LoadBalancers = nil
	}

	for i := range in.VPCSecurityGroups {
		sg := &in.VPCSecurityGroups[i]
		// Normalize pointer fields: empty string → nil
		if sg.ID != nil && *sg.ID == "" {
			sg.ID = nil
		}
		if sg.Name != nil && *sg.Name == "" {
			sg.Name = nil
		}
		// When an ID is set, the SG is treated as a Reference on conversion.
		// Rules and Tags are not persisted for Reference SGs.
		if sg.ID != nil && *sg.ID != "" {
			sg.Rules = nil
			sg.Tags = nil
			continue
		}
		// Empty Rules slice → nil
		if len(sg.Rules) == 0 {
			sg.Rules = nil
		} else {
			// Nil Rule pointers within slice won't survive.
			// v1beta3 only carries Destination for outbound rules and Source for inbound rules.
			// Rules with an invalid Direction will have both Destination and Source dropped on
			// round-trip, so we must also drop them here.
			filtered := sg.Rules[:0]
			for _, rule := range sg.Rules {
				if rule == nil {
					continue
				}
				// Only valid Direction values survive the round-trip.
				if rule.Direction != VPCSecurityGroupRuleDirectionInbound &&
					rule.Direction != VPCSecurityGroupRuleDirectionOutbound {
					continue
				}
				// Destination is only preserved for outbound; Source for inbound.
				if rule.Direction == VPCSecurityGroupRuleDirectionInbound {
					rule.Destination = nil
				}
				if rule.Direction == VPCSecurityGroupRuleDirectionOutbound {
					rule.Source = nil
				}
				// Normalize rule pointer fields
				if rule.SecurityGroupID != nil && *rule.SecurityGroupID == "" {
					rule.SecurityGroupID = nil
				}
				// Empty Destination/Source Remotes → nil
				if rule.Destination != nil && len(rule.Destination.Remotes) == 0 {
					rule.Destination.Remotes = nil
				}
				if rule.Source != nil && len(rule.Source.Remotes) == 0 {
					rule.Source.Remotes = nil
				}
				// Empty PortRange pointer fields
				if rule.Destination != nil && rule.Destination.PortRange != nil &&
					rule.Destination.PortRange.MaximumPort == 0 && rule.Destination.PortRange.MinimumPort == 0 {
					rule.Destination.PortRange = nil
				}
				if rule.Source != nil && rule.Source.PortRange != nil &&
					rule.Source.PortRange.MaximumPort == 0 && rule.Source.PortRange.MinimumPort == 0 {
					rule.Source.PortRange = nil
				}
				filtered = append(filtered, rule)
			}
			if len(filtered) == 0 {
				sg.Rules = nil
			} else {
				sg.Rules = filtered
			}
		}
		// Empty Tags → nil, nil tag pointer entries won't survive
		if len(sg.Tags) == 0 {
			sg.Tags = nil
		} else {
			filteredTags := sg.Tags[:0]
			for _, tag := range sg.Tags {
				if tag != nil && *tag != "" {
					filteredTags = append(filteredTags, tag)
				}
			}
			if len(filteredTags) == 0 {
				sg.Tags = nil
			} else {
				sg.Tags = filteredTags
			}
		}
	}
	// Drop SGs where neither ID nor Name is set
	filteredSGs := in.VPCSecurityGroups[:0]
	for i := range in.VPCSecurityGroups {
		if in.VPCSecurityGroups[i].ID == nil && in.VPCSecurityGroups[i].Name == nil {
			continue
		}
		filteredSGs = append(filteredSGs, in.VPCSecurityGroups[i])
	}
	if len(filteredSGs) == 0 {
		in.VPCSecurityGroups = nil
	} else {
		in.VPCSecurityGroups = filteredSGs
	}

	// TransitGateway: v1beta2 has simple structure (ID, Name, GlobalRouting)
	// v1beta3 has complex structure with Type, Reference, Provision, Connections
	// For round-trip, we need to normalize the v1beta2 structure
	if in.TransitGateway != nil {
		// Empty string ID should be nil
		if in.TransitGateway.ID != nil && *in.TransitGateway.ID == "" {
			in.TransitGateway.ID = nil
		}
		// Empty string Name should be nil
		if in.TransitGateway.Name != nil && *in.TransitGateway.Name == "" {
			in.TransitGateway.Name = nil
		}

		// IMPORTANT: v1beta2 Name field is ambiguous - it can be Reference.Name or Provision.Name
		// The conversion logic uses these rules:
		// 1. If ID is set -> Reference (Name is Reference.Name, GlobalRouting is lost)
		// 2. If GlobalRouting is set -> Provision (Name is Provision.Name)
		// 3. If only Name is set -> Reference (for backward compatibility, GlobalRouting is lost)
		//
		// To ensure round-trip works:
		// - When ID is set, clear GlobalRouting (it's Reference, GlobalRouting doesn't apply)
		// - When GlobalRouting is set, clear ID (it's Provision, ID doesn't apply)
		// - When only Name is set, clear GlobalRouting (it will be treated as Reference)

		hasID := in.TransitGateway.ID != nil && *in.TransitGateway.ID != ""
		hasName := in.TransitGateway.Name != nil && *in.TransitGateway.Name != ""

		if hasID {
			// ID is set -> Reference mode, clear GlobalRouting
			in.TransitGateway.GlobalRouting = nil
		} else if hasName {
			// Only Name is set -> will be treated as Reference, clear GlobalRouting
			in.TransitGateway.GlobalRouting = nil
		}
		// Note: if GlobalRouting is set, it's Provision mode and ID should already be nil

		// If all fields are nil/empty, set TransitGateway to nil
		if in.TransitGateway.ID == nil && in.TransitGateway.Name == nil && in.TransitGateway.GlobalRouting == nil {
			in.TransitGateway = nil
		}
	}

	// CosInstance: normalise for round-trip
	// v1beta3 COSInstanceSource.Reference is not representable in v1beta2 CosInstance (no ID/Name fields)
	// so Reference-type COS instances drop the Reference on round-trip through v1beta2.
	if in.CosInstance != nil {
		if in.CosInstance.Name == "" && in.CosInstance.BucketName == "" && in.CosInstance.BucketRegion == "" {
			in.CosInstance = nil
		}
	}

	// Ignition: empty Version should be nil
	if in.Ignition != nil && in.Ignition.Version == "" {
		in.Ignition = nil
	}
}

func IBMPowerVSClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSClusterTemplate,
		hubIBMPowerVSClusterTemplateResource,
		spokeIBMPowerVSClusterTemplateResource,
	}
}

func hubIBMPowerVSClusterTemplate(in *infrav1.IBMPowerVSClusterTemplate, c randfill.Continue) {
	c.FillNoCustom(in)
	hubIBMPowerVSClusterSpec(&in.Spec.Template.Spec, c)
	// Annotations will have conversion-data added during ConvertFrom, so we can't compare them.
	if len(in.Annotations) == 0 {
		in.Annotations = nil
	}
	if len(in.Spec.Template.ObjectMeta.Annotations) == 0 {
		in.Spec.Template.ObjectMeta.Annotations = nil
	}
}

func hubIBMPowerVSClusterTemplateResource(in *infrav1.IBMPowerVSClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	hubIBMPowerVSClusterSpec(&in.Spec, c)
}

func spokeIBMPowerVSClusterTemplateResource(in *IBMPowerVSClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	spokeIBMPowerVSClusterSpec(&in.Spec, c)
	if in.ObjectMeta.Annotations != nil && len(in.ObjectMeta.Annotations) == 0 {
		in.ObjectMeta.Annotations = nil
	}
	if len(in.ObjectMeta.Labels) == 0 {
		in.ObjectMeta.Labels = nil
	}
	if in.Spec.VPC != nil {
		if in.Spec.VPC.ID != nil && *in.Spec.VPC.ID == "" {
			in.Spec.VPC.ID = nil
		}
		if in.Spec.VPC.Name != nil && *in.Spec.VPC.Name == "" {
			in.Spec.VPC.Name = nil
		}
		if in.Spec.VPC.Region != nil && *in.Spec.VPC.Region == "" {
			in.Spec.VPC.Region = nil
		}
		if in.Spec.VPC.ID == nil && in.Spec.VPC.Name == nil && in.Spec.VPC.Region == nil {
			in.Spec.VPC = nil
		}
	}
}

func IBMPowerVSMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSMachineStatus,
		hubIBMPowerVSMachineSpec,
		spokeIBMPowerVSMachineSpec,
		spokeIBMPowerVSMachineStatus,
	}
}

func hubIBMPowerVSMachineStatus(in *infrav1.IBMPowerVSMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSMachineV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func hubIBMPowerVSMachineSpec(in *infrav1.IBMPowerVSMachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Constrain Image.Type to valid values and enforce xvalidation rules:
	// - Reference: must have Reference set, Import must be empty
	// - Import: must have Import set, Reference must be empty
	switch in.Image.Type {
	case infrav1.ImageSourceTypeReference:
		in.Image.Import = infrav1.ImageReference{}
		// Ensure Reference has at least one identifier
		if in.Image.Reference.ID == "" && in.Image.Reference.Name == "" {
			in.Image.Reference.ID = "fuzzed-image-id"
		}
	case infrav1.ImageSourceTypeImport:
		in.Image.Reference = infrav1.ResourceIdentifier{}
		// Ensure Import has a name
		if in.Image.Import.Name == "" {
			in.Image.Import.Name = "fuzzed-image-ref"
		}
	default:
		// Unknown type: pick Reference if there is reference data, Import if there is import data
		if in.Image.Import.Name != "" {
			in.Image.Type = infrav1.ImageSourceTypeImport
			in.Image.Reference = infrav1.ResourceIdentifier{}
		} else if in.Image.Reference.ID != "" || in.Image.Reference.Name != "" {
			in.Image.Type = infrav1.ImageSourceTypeReference
			in.Image.Import = infrav1.ImageReference{}
		} else {
			// Fall back to a minimal Reference image
			in.Image = infrav1.IBMPowerVSMachineImage{
				Type:      infrav1.ImageSourceTypeReference,
				Reference: infrav1.ResourceIdentifier{ID: "fuzzed-image-id"},
			}
		}
	}
}

func spokeIBMPowerVSMachineSpec(in *IBMPowerVSMachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.ProviderID != nil && *in.ProviderID == "" {
		in.ProviderID = nil
	}

	// Image and ImageRef are mutually exclusive; ImageRef takes priority (maps to Import).
	// RegEx on Image is dropped in v1beta3, so clear it.
	if in.ImageRef != nil {
		if in.ImageRef.Name == "" {
			in.ImageRef = nil
			// Fall through to handle in.Image below
		} else {
			// ImageRef set → Import path; Image must be nil
			in.Image = nil
		}
	}
	if in.Image != nil {
		in.Image.RegEx = nil // RegEx not preserved in v1beta3
		// Empty string ID should be nil
		if in.Image.ID != nil && *in.Image.ID == "" {
			in.Image.ID = nil
		}
		// Empty string Name should be nil
		if in.Image.Name != nil && *in.Image.Name == "" {
			in.Image.Name = nil
		}
		// If Image has no identifiers, set to nil
		if in.Image.ID == nil && in.Image.Name == nil {
			in.Image = nil
		}
	}

	if in.ServiceInstance != nil {
		in.ServiceInstance.RegEx = nil // Tell fuzzer we intentionally drop RegEx in v1beta3
		// Empty string Name should be nil
		if in.ServiceInstance.Name != nil && *in.ServiceInstance.Name == "" {
			in.ServiceInstance.Name = nil
		}
		// Empty string ID should be nil
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID == "" {
			in.ServiceInstance.ID = nil
		}
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID != "" {
			in.ServiceInstanceID = *in.ServiceInstance.ID
		}
		// If ServiceInstance is empty, set to nil
		if in.ServiceInstance.ID == nil && in.ServiceInstance.Name == nil {
			in.ServiceInstance = nil
		}
	}
	// Ensure ServiceInstance is set when ServiceInstanceID is set
	if in.ServiceInstanceID != "" {
		if in.ServiceInstance == nil {
			in.ServiceInstance = &IBMPowerVSResourceReference{}
		}
		if in.ServiceInstance.ID == nil || *in.ServiceInstance.ID == "" {
			id := in.ServiceInstanceID
			in.ServiceInstance.ID = &id
		}
	} else {
		// If ServiceInstanceID is empty, ServiceInstance should be nil
		in.ServiceInstance = nil
	}

	// Network.RegEx is not preserved in v1beta3, so clear it for round-trip
	in.Network.RegEx = nil

	// Empty string ID should be nil in Network
	if in.Network.ID != nil && *in.Network.ID == "" {
		in.Network.ID = nil
	}
	// Empty string Name should be nil in Network
	if in.Network.Name != nil && *in.Network.Name == "" {
		in.Network.Name = nil
	}
}

func spokeIBMPowerVSMachineStatus(in *IBMPowerVSMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Fault and FailureReason have no v1beta3 equivalent; they are not preserved across round-trips.
	in.Fault = ""
	in.FailureReason = nil
	in.FailureMessage = nil
	// Region/Zone: nil pointer converts to empty string in v1beta3, then back to &"".
	// Normalise nil to &"" to avoid false diff after round-trip, or clear both to nil.
	if in.Region != nil && *in.Region == "" {
		in.Region = nil
	}
	if in.Zone != nil && *in.Zone == "" {
		in.Zone = nil
	}
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSMachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func IBMPowerVSMachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSMachineTemplateResource,
		spokeIBMPowerVSMachineTemplateResource,
	}
}

func spokeIBMPowerVSMachineTemplateResource(in *IBMPowerVSMachineTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	spokeIBMPowerVSMachineSpec(&in.Spec, c)
}

func hubIBMPowerVSMachineTemplateResource(in *infrav1.IBMPowerVSMachineTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	in.ObjectMeta = clusterv1.ObjectMeta{}
	hubIBMPowerVSMachineSpec(&in.Spec, c)
}

func IBMPowerVSImageFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSImage,
		hubIBMPowerVSImageStatus,
		spokeIBMPowerVSImageStatus,
		spokeIBMPowerVSImageSpec,
	}
}

func hubIBMPowerVSImage(in *infrav1.IBMPowerVSImage, c randfill.Continue) {
	c.FillNoCustom(in)
	// Annotations will have conversion-data added during ConvertFrom, so we can't compare them.
	if len(in.Annotations) == 0 {
		in.Annotations = nil
	}
}

func hubIBMPowerVSImageStatus(in *infrav1.IBMPowerVSImageStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSImageV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeIBMPowerVSImageSpec(in *IBMPowerVSImageSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// v1beta3 Bucket/Object/Region are plain strings; an empty pointer cannot round-trip.
	if in.Bucket != nil && *in.Bucket == "" {
		in.Bucket = nil
	}
	if in.Object != nil && *in.Object == "" {
		in.Object = nil
	}
	if in.Region != nil && *in.Region == "" {
		in.Region = nil
	}

	if in.ServiceInstance != nil {
		in.ServiceInstance.RegEx = nil // Tell fuzzer we intentionally drop RegEx in v1beta3
		// Empty string Name should be nil
		if in.ServiceInstance.Name != nil && *in.ServiceInstance.Name == "" {
			in.ServiceInstance.Name = nil
		}
		// Empty string ID should be nil
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID == "" {
			in.ServiceInstance.ID = nil
		}
		if in.ServiceInstance.ID != nil && *in.ServiceInstance.ID != "" {
			in.ServiceInstanceID = *in.ServiceInstance.ID
		}
		// If ServiceInstance is empty, set to nil
		if in.ServiceInstance.ID == nil && in.ServiceInstance.Name == nil {
			in.ServiceInstance = nil
		}
	}
	// Ensure ServiceInstance is set when ServiceInstanceID is set
	if in.ServiceInstanceID != "" {
		if in.ServiceInstance == nil {
			in.ServiceInstance = &IBMPowerVSResourceReference{}
		}
		if in.ServiceInstance.ID == nil || *in.ServiceInstance.ID == "" {
			id := in.ServiceInstanceID
			in.ServiceInstance.ID = &id
		}
	} else {
		// If ServiceInstanceID is empty, ServiceInstance should be nil
		in.ServiceInstance = nil
	}
}

func spokeIBMPowerVSImageStatus(in *IBMPowerVSImageStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSImageV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}
