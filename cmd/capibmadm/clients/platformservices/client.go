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

// Package platformservices contains client functions for platform services.
package platformservices

import (
	"github.com/IBM/platform-services-go-sdk/iamidentityv1"
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
)

// NewResourceManagerV2Client creates new resource manager client.
func NewResourceManagerV2Client() (*resourcemanagerv2.ResourceManagerV2, error) {
	return resourcemanagerv2.NewResourceManagerV2(&resourcemanagerv2.ResourceManagerV2Options{
		Authenticator: iam.GetIAMAuth(),
		URL:           resourcemanagerv2.DefaultServiceURL,
	})
}

// NewIAMIdentityClient creates iam identity client.
func NewIAMIdentityClient() (*iamidentityv1.IamIdentityV1, error) {
	return iamidentityv1.NewIamIdentityV1(&iamidentityv1.IamIdentityV1Options{
		Authenticator: iam.GetIAMAuth(),
		URL:           iamidentityv1.DefaultServiceURL,
	})
}
