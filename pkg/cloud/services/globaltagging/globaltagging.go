/*
Copyright 2024 The Kubernetes Authors.

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

package globaltagging

import (
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
)

// GlobalTagging interface defines a method that a IBMCLOUD service object should implement in order to
// use the manage tags with the Global Tagging APIs.
type GlobalTagging interface {
	CreateTag(*globaltaggingv1.CreateTagOptions) (*globaltaggingv1.CreateTagResults, *core.DetailedResponse, error)
	AttachTag(*globaltaggingv1.AttachTagOptions) (*globaltaggingv1.TagResults, *core.DetailedResponse, error)
	GetTagByName(string) (*globaltaggingv1.Tag, error)
}
