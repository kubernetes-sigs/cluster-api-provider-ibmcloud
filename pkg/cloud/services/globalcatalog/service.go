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

package globalcatalog

import (
	"fmt"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globalcatalogv1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
)

// Service holds the IBM Cloud Resource Controller Service specific information.
type Service struct {
	client *globalcatalogv1.GlobalCatalogV1
}

// ServiceOptions holds the IBM Cloud Resource Controller Service Options specific information.
type ServiceOptions struct {
	*globalcatalogv1.GlobalCatalogV1Options
}

// SetServiceURL sets the service URL.
func (s *Service) SetServiceURL(url string) error {
	return s.client.SetServiceURL(url)
}

// GetServiceURL will get the service URL.
func (s *Service) GetServiceURL() string {
	return s.client.GetServiceURL()
}

// ListCatalogEntries lists the catalog entries.
func (s *Service) ListCatalogEntries(options *globalcatalogv1.ListCatalogEntriesOptions) (*globalcatalogv1.EntrySearchResult, *core.DetailedResponse, error) {
	return s.client.ListCatalogEntries(options)
}

// GetChildObjects get the child object.
func (s *Service) GetChildObjects(options *globalcatalogv1.GetChildObjectsOptions) (*globalcatalogv1.EntrySearchResult, *core.DetailedResponse, error) {
	return s.client.GetChildObjects(options)
}

// GetServiceInfo returns the give service's service id and plan id.
func (s *Service) GetServiceInfo(service string, servicePlan string) (string, string, error) {
	var serviceID, servicePlanID string
	include := "*"
	listCatalogEntriesOpt := globalcatalogv1.ListCatalogEntriesOptions{Include: &include, Q: &service}
	catalogEntriesList, _, err := s.client.ListCatalogEntries(&listCatalogEntriesOpt)
	if err != nil {
		return "", "", err
	}
	if catalogEntriesList != nil {
		for _, catalog := range catalogEntriesList.Resources {
			if *catalog.Name == service {
				serviceID = *catalog.ID
			}
		}
	}

	if serviceID == "" {
		return "", "", fmt.Errorf("could not retrieve service id for service %s", service)
	} else if servicePlan == "" {
		return serviceID, "", nil
	} else {
		kind := "plan"
		getChildOpt := globalcatalogv1.GetChildObjectsOptions{ID: &serviceID, Kind: &kind}
		var childObjResult *globalcatalogv1.EntrySearchResult
		childObjResult, _, err = s.client.GetChildObjects(&getChildOpt)
		if err != nil {
			return "", "", err
		}
		for _, plan := range childObjResult.Resources {
			if *plan.Name == servicePlan {
				servicePlanID = *plan.ID
				return serviceID, servicePlanID, nil
			}
		}
	}
	err = fmt.Errorf("could not retrieve plan id for service name: %s & service plan name: %s", service, servicePlan)
	return "", "", err
}

// NewService returns a new service for the IBM Cloud Global catalog api client.
func NewService(options ServiceOptions) (*Service, error) {
	if options.GlobalCatalogV1Options == nil {
		options.GlobalCatalogV1Options = &globalcatalogv1.GlobalCatalogV1Options{}
	}
	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, err
	}
	options.Authenticator = auth
	gcv1, err := globalcatalogv1.NewGlobalCatalogV1(options.GlobalCatalogV1Options)
	if err != nil {
		return nil, err
	}
	return &Service{
		client: gcv1,
	}, nil
}
