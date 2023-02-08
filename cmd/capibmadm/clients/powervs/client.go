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

// Package powervs contains powervs client functions.
package powervs

import (
	"github.com/IBM-Cloud/power-go-client/ibmpisession"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
)

// NewPISession creates new powervs client.
// To-Do: Need to handle custom endpoint URL if user wants to use staging env.
func NewPISession(accountID string, zone string, debug bool) (*ibmpisession.IBMPISession, error) {
	return ibmpisession.NewIBMPISession(&ibmpisession.IBMPIOptions{
		Authenticator: iam.GetIAMAuth(),
		Debug:         debug,
		UserAccount:   accountID,
		Zone:          zone})
}
