/*
Copyright 2026 The Kubernetes Authors.

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

package vpc

import (
	"fmt"
	"strings"
)

// CRN is a local duplicate of IBM Cloud CRN for parsing and references.
type CRN struct {
	Scheme          string
	Version         string
	CName           string
	CType           string
	ServiceName     string
	Region          string
	ScopeType       string
	Scope           string
	ServiceInstance string
	ResourceType    string
	Resource        string
}

// parseCRN is a local duplicate of IBM Cloud CRN Parse functionality, to convert a string into a CRN, if it is in the correct format.
func parseCRN(s string) (*CRN, error) {
	if s == "" {
		return nil, nil
	}

	segments := strings.Split(s, ":")
	if len(segments) != 10 || segments[0] != "crn" {
		return nil, fmt.Errorf("malformed CRN")
	}

	crn := &CRN{
		Scheme:          segments[0],
		Version:         segments[1],
		CName:           segments[2],
		CType:           segments[3],
		ServiceName:     segments[4],
		Region:          segments[5],
		ServiceInstance: segments[7],
		ResourceType:    segments[8],
		Resource:        segments[9],
	}

	// Scope portions require additional parsing.
	scopeSegments := segments[6]
	if scopeSegments != "" {
		if scopeSegments == "global" {
			crn.Scope = scopeSegments
		} else {
			scopeParts := strings.Split(scopeSegments, "/")
			if len(scopeParts) != 2 {
				return nil, fmt.Errorf("malformed scope in CRN")
			}
			crn.ScopeType, crn.Scope = scopeParts[0], scopeParts[1]
		}
	}

	return crn, nil
}
