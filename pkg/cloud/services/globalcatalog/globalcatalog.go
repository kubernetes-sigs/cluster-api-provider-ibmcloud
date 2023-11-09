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
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globalcatalogv1"
)

// GlobalCatalog interface defines a method that a IBMCLOUD service object should implement in order to
// use the globalcatalogv1 package for listing resource instances.
type GlobalCatalog interface {
	SetServiceURL(url string) error
	GetServiceURL() string
	GetServiceInfo(string, string) (string, string, error)

	ListCatalogEntries(*globalcatalogv1.ListCatalogEntriesOptions) (*globalcatalogv1.EntrySearchResult, *core.DetailedResponse, error)
	GetChildObjects(*globalcatalogv1.GetChildObjectsOptions) (*globalcatalogv1.EntrySearchResult, *core.DetailedResponse, error)
}
