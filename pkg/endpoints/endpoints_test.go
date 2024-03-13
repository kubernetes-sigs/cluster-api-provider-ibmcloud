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

package endpoints

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	testCases := []struct {
		name           string
		flagToParse    string
		expectedOutput []ServiceEndpoint
		expectedError  error
	}{
		{
			name:           "no configuration",
			flagToParse:    "",
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			name:           "none configuration",
			flagToParse:    "none",
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			name:        "single region, single vpc service",
			flagToParse: "eu-gb:vpc=https://vpchost:8080",
			expectedOutput: []ServiceEndpoint{
				{
					ID:     "vpc",
					URL:    "https://vpchost:8080",
					Region: "eu-gb",
				},
			},
			expectedError: nil,
		},
		{
			name:        "single region, single powervs service",
			flagToParse: "lon:powervs=https://pvshost:8080",
			expectedOutput: []ServiceEndpoint{
				{
					ID:     "powervs",
					URL:    "https://pvshost:8080",
					Region: "lon",
				},
			},
			expectedError: nil,
		},
		{
			name:        "single region, multiple services",
			flagToParse: "lon:powervs=https://pvshost:8080,rc=https://rchost:8080",
			expectedOutput: []ServiceEndpoint{
				{
					ID:     "powervs",
					URL:    "https://pvshost:8080",
					Region: "lon",
				},
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "lon",
				},
			},
			expectedError: nil,
		},
		{
			name:           "single region, duplicate service",
			flagToParse:    "eu-gb:vpc=https://localhost:8080,vpc=https://vpchost:8080",
			expectedOutput: nil,
			expectedError:  errServiceEndpointDuplicateID,
		},
		{
			name:           "single region, non-valid URI",
			flagToParse:    "eu-gb:vpc=fdsfs",
			expectedOutput: nil,
			expectedError:  errServiceEndpointURL,
		},
		{
			name:        "multiples regions",
			flagToParse: "eu-gb:vpc=https://vpchost:8080;lon:powervs=https://pvshost:8080,rc=https://rchost:8080",
			expectedOutput: []ServiceEndpoint{
				{
					ID:     "vpc",
					URL:    "https://vpchost:8080",
					Region: "eu-gb",
				},
				{
					ID:     "powervs",
					URL:    "https://pvshost:8080",
					Region: "lon",
				},
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "lon",
				},
			},
			expectedError: nil,
		},
		{
			name:        "multiples regions, multiple services",
			flagToParse: "eu-gb:vpc=https://vpchost:8080;lon:powervs=https://pvshost:8080,rc=https://rchost:8080;us-south:powervs=https://pvshost-us:8080",
			expectedOutput: []ServiceEndpoint{
				{
					ID:     "vpc",
					URL:    "https://vpchost:8080",
					Region: "eu-gb",
				},
				{
					ID:     "powervs",
					URL:    "https://pvshost:8080",
					Region: "lon",
				},
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "lon",
				},
				{
					ID:     "powervs",
					URL:    "https://pvshost-us:8080",
					Region: "us-south",
				},
			},
			expectedError: nil,
		},
		{
			name:           "invalid config",
			flagToParse:    "eu-gb=localhost",
			expectedOutput: nil,
			expectedError:  errServiceEndpointRegion,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseServiceEndpointFlag(tc.flagToParse)
			require.ErrorIs(t, err, tc.expectedError)
			require.ElementsMatch(t, out, tc.expectedOutput)
		})
	}
}

func TestFetchVPCEndpoint(t *testing.T) {
	testCases := []struct {
		name            string
		region          string
		serviceEndpoint []ServiceEndpoint
		expectedOutput  string
	}{
		{
			name:            "Return constructed endpoint",
			region:          "us-south",
			serviceEndpoint: []ServiceEndpoint{},
			expectedOutput:  "https://us-south.iaas.cloud.ibm.com/v1",
		},
		{
			name:   "Return fetched endpoint",
			region: "us-south",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "vpc",
					URL:    "https://vpchost:8080",
					Region: "us-south",
				},
			},
			expectedOutput: "https://vpchost:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := FetchVPCEndpoint(tc.region, tc.serviceEndpoint)
			require.Equal(t, tc.expectedOutput, out)
		})
	}
}

func TestFetchPVSEndpoint(t *testing.T) {
	testCases := []struct {
		name            string
		region          string
		serviceEndpoint []ServiceEndpoint
		expectedOutput  string
	}{
		{
			name:            "Return empty endpoint",
			region:          "us-south",
			serviceEndpoint: []ServiceEndpoint{},
			expectedOutput:  "",
		},
		{
			name:   "Return fetched endpoint",
			region: "us-south",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "powervs",
					URL:    "https://powervshost:8080",
					Region: "us-south",
				},
			},
			expectedOutput: "https://powervshost:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := FetchPVSEndpoint(tc.region, tc.serviceEndpoint)
			require.Equal(t, tc.expectedOutput, out)
		})
	}
}

func TestFetchRCEndpoint(t *testing.T) {
	testCases := []struct {
		name            string
		serviceEndpoint []ServiceEndpoint
		expectedOutput  string
	}{
		{
			name:            "Return empty endpoint",
			serviceEndpoint: []ServiceEndpoint{},
			expectedOutput:  "",
		},
		{
			name: "Return fetched endpoint",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "us-south",
				},
			},
			expectedOutput: "https://rchost:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := FetchRCEndpoint(tc.serviceEndpoint)
			require.Equal(t, tc.expectedOutput, out)
		})
	}
}

func TestFetchEndpoints(t *testing.T) {
	testCases := []struct {
		name            string
		serviceEndpoint []ServiceEndpoint
		serviceID       string
		expectedOutput  string
	}{
		{
			name:            "With empty service endpoints",
			serviceEndpoint: []ServiceEndpoint{},
			expectedOutput:  "",
		},
		{
			name:      "With invalid service id",
			serviceID: "abc",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "us-south",
				},
			},
			expectedOutput: "",
		},
		{
			name:      "With service id not preset in service endpoints",
			serviceID: "powervs",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "us-south",
				},
			},
			expectedOutput: "",
		},
		{
			name:      "With valid service id",
			serviceID: "rc",
			serviceEndpoint: []ServiceEndpoint{
				{
					ID:     "rc",
					URL:    "https://rchost:8080",
					Region: "us-south",
				},
				{
					ID:  "powervs",
					URL: "https://powervs:8081",
				},
			},
			expectedOutput: "https://rchost:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := FetchEndpoints(tc.serviceID, tc.serviceEndpoint)
			require.Equal(t, tc.expectedOutput, out)
		})
	}
}

func TestCostructRegionFromZone(t *testing.T) {
	testCases := []struct {
		name           string
		zone           string
		expectedRegion string
	}{
		{
			name:           "Return region dal",
			zone:           "dal12",
			expectedRegion: "dal",
		},
		{
			name:           "Return region eu-de",
			zone:           "eu-de-1",
			expectedRegion: "eu-de",
		},
		{
			name:           "Return region lon04",
			zone:           "lon04",
			expectedRegion: "lon",
		},
		{
			name:           "Return region mon01",
			zone:           "mon01",
			expectedRegion: "mon",
		},
		{
			name:           "Return region osa21",
			zone:           "osa21",
			expectedRegion: "osa",
		},
		{
			name:           "Return region sao01",
			zone:           "sao01",
			expectedRegion: "sao",
		},
		{
			name:           "Return region syd04",
			zone:           "syd04",
			expectedRegion: "syd",
		},
		{
			name:           "Return region tok04",
			zone:           "tok04",
			expectedRegion: "tok",
		},
		{
			name:           "Return region tor01",
			zone:           "tor01",
			expectedRegion: "tor",
		},
		{
			name:           "Return region wdc06",
			zone:           "wdc06",
			expectedRegion: "wdc",
		},
		{
			name:           "Return region us-east",
			zone:           "us-east",
			expectedRegion: "us-east",
		},
		{
			name:           "Return region us-south",
			zone:           "us-south",
			expectedRegion: "us-south",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := ConstructRegionFromZone(tc.zone)
			require.Equal(t, tc.expectedRegion, out)
		})
	}
}
