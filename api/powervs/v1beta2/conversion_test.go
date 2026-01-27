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
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"

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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
}
