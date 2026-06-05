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

	// Workspace.Name is not preserved in v1beta2 (only ID is), so clear it for round-trip
	in.Workspace.Name = ""
}

func hubIBMPowerVSClusterSpec(in *infrav1.IBMPowerVSClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Enforce safe Enum values for Topology so round-trips match
	if in.Topology != infrav1.PowerVSLoadBalancerTopology {
		in.Topology = infrav1.PowerVSVirtualIPTopology
	}

	// Enforce the SourceType union constraints for v1beta3 Workspace so round-trip tests pass
	switch in.Workspace.Type {
	case infrav1.SourceTypeReference:
		in.Workspace.Provision = infrav1.WorkspaceProvisionConfig{}
	case infrav1.SourceTypeProvision:
		in.Workspace.Reference = infrav1.ResourceIdentifier{}
	default:
		// If Type is not set or invalid, default to Provision with empty config
		in.Workspace.Type = infrav1.SourceTypeProvision
		in.Workspace.Reference = infrav1.ResourceIdentifier{}
		in.Workspace.Provision = infrav1.WorkspaceProvisionConfig{}
	}

	// Ensure Workspace.Reference has at least ID or Name when Type is Reference
	if in.Workspace.Type == infrav1.SourceTypeReference {
		if in.Workspace.Reference.ID == "" && in.Workspace.Reference.Name == "" {
			in.Workspace.Reference.ID = "fuzzed-workspace-id"
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

	// Enforce ID generation so the Workspace value-type mapping survives the round-trip
	if in.ServiceInstance != nil && in.ServiceInstance.ID == nil {
		id := "fuzzed-id"
		in.ServiceInstance.ID = &id
	}

	// ServiceInstance with empty ID should be nil
	if in.ServiceInstance != nil && in.ServiceInstance.ID != nil && *in.ServiceInstance.ID == "" {
		in.ServiceInstance = nil
	}

	// ControllerCreated is not preserved in v1beta3, so set to nil for round-trip
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
	}
	if in.COSInstance != nil {
		in.COSInstance.ControllerCreated = nil
	}
}

func spokeIBMPowerVSClusterSpec(in *IBMPowerVSClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

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

func IBMPowerVSClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSClusterTemplate,
		hubIBMPowerVSClusterTemplateResource,
		spokeIBMPowerVSClusterTemplateResource,
	}
}

func hubIBMPowerVSClusterTemplate(in *infrav1.IBMPowerVSClusterTemplate, c randfill.Continue) {
	c.FillNoCustom(in)
	// Annotations will have conversion-data added during ConvertFrom, so we can't compare them
	// The test framework will handle this via MarshalData
}

func hubIBMPowerVSClusterTemplateResource(in *infrav1.IBMPowerVSClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	hubIBMPowerVSClusterSpec(&in.Spec, c)
}

func spokeIBMPowerVSClusterTemplateResource(in *IBMPowerVSClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)
	spokeIBMPowerVSClusterSpec(&in.Spec, c)
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
}

func spokeIBMPowerVSMachineSpec(in *IBMPowerVSMachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.ProviderID != nil && *in.ProviderID == "" {
		in.ProviderID = nil
	}
	if in.ImageRef != nil && in.ImageRef.Name == "" {
		in.ImageRef = nil
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

func spokeIBMPowerVSMachineStatus(in *IBMPowerVSMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
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
}

func IBMPowerVSImageFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSImageStatus,
		spokeIBMPowerVSImageStatus,
		spokeIBMPowerVSImageSpec,
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
