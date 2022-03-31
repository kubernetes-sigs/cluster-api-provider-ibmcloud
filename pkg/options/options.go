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

package options

// PowerVSProviderIDFormat is used to identify the Provider ID format for Machine.
var PowerVSProviderIDFormat string

// PowerVSProviderIDFormatV2 will set provider id to machine as ibmpowervs://<region>/<zone>/<service_instance_id>/<powervs_machine_id>
const PowerVSProviderIDFormatV2 = "v2"

// PowerVSProviderIDFormatV1 will set provider id to machine as ibmpowervs://<cluster_name>/<vm_hostname>
const PowerVSProviderIDFormatV1 = "v1"
