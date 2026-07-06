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

// VPCSecurityGroupStatus defines a vpc security group resource status with its id and respective rule's ids.
type VPCSecurityGroupStatus struct {
	// id represents the id of the resource.
	ID *string `json:"id,omitempty"`
	// ruleIDs contains the id of rules created under the security group
	RuleIDs []*string `json:"ruleIDs,omitempty"`
	// controllerCreated indicates whether the resource is created by the controller.
	// +kubebuilder:default=false
	ControllerCreated *bool `json:"controllerCreated,omitempty"`
}
