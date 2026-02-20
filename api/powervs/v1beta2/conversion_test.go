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
	"sigs.k8s.io/randfill"
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"

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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
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
		spokeIBMPowerVSClusterStatus,
	}
}

func hubIBMPowerVSClusterStatus(in *infrav1.IBMPowerVSClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSClusterV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeIBMPowerVSClusterStatus(in *IBMPowerVSClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func IBMPowerVSMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSMachineStatus,
		spokeIBMPowerVSMachineStatus,
	}
}

func hubIBMPowerVSMachineStatus(in *infrav1.IBMPowerVSMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSMachineV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeIBMPowerVSMachineStatus(in *IBMPowerVSMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSMachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func IBMPowerVSImageFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubIBMPowerVSImageStatus,
		spokeIBMPowerVSImageStatus,
	}
}

func hubIBMPowerVSImageStatus(in *infrav1.IBMPowerVSImageStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta2 == nil || reflect.DeepEqual(in.Deprecated.V1Beta2, &infrav1.IBMPowerVSImageV1Beta2DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeIBMPowerVSImageStatus(in *IBMPowerVSImageStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &IBMPowerVSImageV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}
