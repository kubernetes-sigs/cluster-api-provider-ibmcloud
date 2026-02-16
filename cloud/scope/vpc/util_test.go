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

package vpc

import (
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
)

func TestNormalizedVPCSecurityGroupRulePrototype(t *testing.T) {
	t.Run("When prototype is nil", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(normalizedVPCSecurityGroupRulePrototype(nil)).To(BeNil())
	})
	t.Run("When protocol is not deprecated, the prototype is returned unchanged", func(t *testing.T) {
		g := NewWithT(t)
		prototype := &infrav1.VPCSecurityGroupRulePrototype{
			Protocol: infrav1.VPCSecurityGroupRuleProtocolTCP,
		}
		g.Expect(normalizedVPCSecurityGroupRulePrototype(prototype)).To(BeIdenticalTo(prototype))
	})
	t.Run("When protocol is deprecated 'all', a normalized copy is returned and the input is not mutated", func(t *testing.T) {
		g := NewWithT(t)
		prototype := &infrav1.VPCSecurityGroupRulePrototype{
			Protocol: infrav1.VPCSecurityGroupRuleProtocolAll,
			Remotes: []infrav1.VPCSecurityGroupRuleRemote{
				{RemoteType: infrav1.VPCSecurityGroupRuleRemoteTypeAny},
			},
		}
		normalized := normalizedVPCSecurityGroupRulePrototype(prototype)
		g.Expect(normalized).ToNot(BeIdenticalTo(prototype))
		g.Expect(normalized.Protocol).To(Equal(infrav1.VPCSecurityGroupRuleProtocolIcmpTCPUDP))
		g.Expect(normalized.Remotes).To(Equal(prototype.Remotes))
		g.Expect(prototype.Protocol).To(Equal(infrav1.VPCSecurityGroupRuleProtocolAll))
	})
}
