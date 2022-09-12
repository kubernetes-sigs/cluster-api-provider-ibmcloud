/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"

	capiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Convert_v1alpha4_APIEndpoint_To_v1beta1_APIEndpoint(in *capiv1alpha4.APIEndpoint, out *capiv1beta1.APIEndpoint, s apiconversion.Scope) error {
	return capiv1alpha4.Convert_v1alpha4_APIEndpoint_To_v1beta1_APIEndpoint(in, out, s)
}

func Convert_v1beta1_APIEndpoint_To_v1alpha4_APIEndpoint(in *capiv1beta1.APIEndpoint, out *capiv1alpha4.APIEndpoint, s apiconversion.Scope) error {
	return capiv1alpha4.Convert_v1beta1_APIEndpoint_To_v1alpha4_APIEndpoint(in, out, s)
}
