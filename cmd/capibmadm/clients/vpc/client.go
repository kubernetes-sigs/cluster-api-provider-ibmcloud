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

// Package vpc contains vpc client functions.
package vpc

import (
	"github.com/IBM/vpc-go-sdk/vpcv1"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
)

// NewV1Client creates new vpcv1 client.
// To-Do: Need to handle custom endpoint URL if user wants to use staging env.
func NewV1Client(region string) (*vpcv1.VpcV1, error) {
	svcEndpoint := "https://" + region + ".iaas.cloud.ibm.com/v1"

	return vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		ServiceName:   "vpcs",
		Authenticator: iam.GetIAMAuth(),
		URL:           svcEndpoint,
	})
}
