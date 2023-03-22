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

// Package utils contains utility and printer functions for cli.
package utils

import (
	"context"
	"fmt"

	"github.com/go-openapi/strfmt"

	"github.com/IBM/platform-services-go-sdk/iamidentityv1"
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/platformservices"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// GetAccountID returns IBM cloud account ID of API key used.
func GetAccountID(ctx context.Context) (string, error) {
	iamv1, err := platformservices.NewIAMIdentityClient()
	if err != nil {
		return "", err
	}

	apiKeyDetailsOpt := iamidentityv1.GetAPIKeysDetailsOptions{IamAPIKey: &options.GlobalOptions.IBMCloudAPIKey}
	apiKey, _, err := iamv1.GetAPIKeysDetailsWithContext(ctx, &apiKeyDetailsOpt)
	if err != nil {
		return "", err
	}
	if apiKey == nil {
		return "", fmt.Errorf("could not retrieve account id")
	}

	return *apiKey.AccountID, nil
}

// GetResourceGroupID returns ID of given resource group name.
func GetResourceGroupID(ctx context.Context, resourceGroup string, accountID string) (string, error) {
	rmv2, err := platformservices.NewResourceManagerV2Client()

	if err != nil {
		return "", err
	}

	if rmv2 == nil {
		return "", fmt.Errorf("unable to get resource controller")
	}

	rmv2ListResourceGroupOpt := resourcemanagerv2.ListResourceGroupsOptions{Name: &resourceGroup, AccountID: &accountID}
	resourceGroupListResult, _, err := rmv2.ListResourceGroupsWithContext(ctx, &rmv2ListResourceGroupOpt)
	if err != nil {
		return "", err
	}

	if resourceGroupListResult != nil && len(resourceGroupListResult.Resources) > 0 {
		rg := resourceGroupListResult.Resources[0]
		resourceGroupID := *rg.ID
		return resourceGroupID, nil
	}

	err = fmt.Errorf("could not retrieve resource group id for %s", resourceGroup)
	return "", err
}

// DereferencePointer dereferences pointer.
func DereferencePointer(value interface{}) interface{} {
	switch v := value.(type) {
	case *string:
		if v != nil {
			return *v
		}
		return ""
	case *int, *int8, *int16, *int32, *int64:
		i := value.(*int64)
		if i != nil {
			return *i
		}
		return 0
	case *strfmt.DateTime:
		if v != nil {
			return *v
		}
		return strfmt.DateTime{}
	case *bool:
		if v != nil {
			return *v
		}
		return false
	case *float32, *float64:
		f := value.(*float64)
		if f != nil {
			return *f
		}
		return 0
	}
	return nil
}
