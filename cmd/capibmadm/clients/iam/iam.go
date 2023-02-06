/*
Copyright 2023 The Kubernetes Authors.

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

// Package iam contains, client to create iam authenticator.
package iam

import (
	"github.com/IBM/go-sdk-core/v5/core"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// GetIAMAuth creates core Authenticator from API key provided.
func GetIAMAuth() *core.IamAuthenticator {
	return &core.IamAuthenticator{
		ApiKey: options.GlobalOptions.IBMCloudAPIKey,
	}
}
