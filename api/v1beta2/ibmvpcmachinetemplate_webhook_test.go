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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/defaulting"
)

func TestVPCMachineTemplate_default(t *testing.T) {
	g := NewWithT(t)
	vpcMachineTemplate := &IBMVPCMachineTemplate{ObjectMeta: metav1.ObjectMeta{Name: "capi-machine-template", Namespace: "default"}}
	t.Run("Defaults for IBMVPCMachineTemplate", defaulting.DefaultValidateTest(vpcMachineTemplate))
	vpcMachineTemplate.Default()
	g.Expect(vpcMachineTemplate.Spec.Template.Spec.Profile).To(BeEquivalentTo("bx2-2x8"))
}
