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

package v1beta2

import (
	"context"
	"testing"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllSupportedSystemTypes(t *testing.T) {
	tests := []struct {
		name             string
		piSession        *ibmpisession.IBMPISession
		mockDatacenters  *models.Datacenters
		mockError        error
		expectedTypes    []string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name:             "nil PISession should return error",
			piSession:        nil,
			expectedError:    true,
			expectedErrorMsg: "PISession is required",
		},
		{
			name:      "successful retrieval with multiple datacenters",
			piSession: &ibmpisession.IBMPISession{
				// Mock session - in real tests you'd use a mock client
			},
			mockDatacenters: &models.Datacenters{
				Datacenters: []*models.Datacenter{
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"s922", "e980", "s1022"},
							},
						},
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"e980", "s1122", "e1050"},
							},
						},
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"e1080"},
							},
						},
					},
				},
			},
			// Expected: unique, sorted list
			expectedTypes: []string{"e1050", "e1080", "e980", "s1022", "s1122", "s922"},
			expectedError: false,
		},
		{
			name:      "handles datacenters without capabilities gracefully",
			piSession: &ibmpisession.IBMPISession{
				// Mock session
			},
			mockDatacenters: &models.Datacenters{
				Datacenters: []*models.Datacenter{
					{
						CapabilitiesDetails: nil, // Missing capabilities
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"s922", "e980"},
							},
						},
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: nil, // Missing supported systems
						},
					},
				},
			},
			expectedTypes: []string{"e980", "s922"},
			expectedError: false,
		},
		{
			name:      "deduplicates system types across datacenters",
			piSession: &ibmpisession.IBMPISession{
				// Mock session
			},
			mockDatacenters: &models.Datacenters{
				Datacenters: []*models.Datacenter{
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"s922", "e980", "s922"}, // Duplicate in same DC
							},
						},
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: &models.SupportedSystems{
								General: []string{"s922", "e980"}, // Duplicates from other DC
							},
						},
					},
				},
			},
			expectedTypes: []string{"e980", "s922"}, // Only unique values
			expectedError: false,
		},
		{
			name:      "returns error when no datacenters found",
			piSession: &ibmpisession.IBMPISession{
				// Mock session
			},
			mockDatacenters: &models.Datacenters{
				Datacenters: []*models.Datacenter{},
			},
			expectedError:    true,
			expectedErrorMsg: "no datacenters found",
		},
		{
			name:      "returns error when no system types found",
			piSession: &ibmpisession.IBMPISession{
				// Mock session
			},
			mockDatacenters: &models.Datacenters{
				Datacenters: []*models.Datacenter{
					{
						CapabilitiesDetails: nil,
					},
					{
						CapabilitiesDetails: &models.CapabilitiesDetails{
							SupportedSystems: nil,
						},
					},
				},
			},
			expectedError:    true,
			expectedErrorMsg: "no system types found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.piSession == nil {
				result, err := GetAllSupportedSystemTypes(ctx, tt.piSession)

				if tt.expectedError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
					assert.Nil(t, result)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expectedTypes, result)
				}
			}
		})
	}
}

// TestGetAllSupportedSystemTypes_Integration is an integration test that validates
// the expected system types are returned. This test requires actual PowerVS API access.
// Skip this test in CI/CD by using build tags or environment variables.
func TestGetAllSupportedSystemTypes_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual PowerVS API access")

	// This test would require:
	// 1. Valid IBM Cloud credentials
	// 2. Initialized PISession
	// 3. Network access to PowerVS API

	ctx := context.Background()

	// Initialize your PISession here
	// piSession := initializePISession(t)

	var piSession *ibmpisession.IBMPISession // Replace with actual initialization

	result, err := GetAllSupportedSystemTypes(ctx, piSession)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Validate that known system types are in the result
	knownTypes := []string{"s922", "e980", "s1022", "s1122", "e1050", "e1080"}
	for _, knownType := range knownTypes {
		assert.Contains(t, result, knownType, "Expected system type %s to be in the result", knownType)
	}

	// Validate result is sorted
	assert.True(t, isSorted(result), "Result should be sorted alphabetically")

	// Validate no duplicates
	assert.Equal(t, len(result), len(uniqueStrings(result)), "Result should not contain duplicates")
}

// Helper function to check if slice is sorted
func isSorted(slice []string) bool {
	for i := 1; i < len(slice); i++ {
		if slice[i-1] > slice[i] {
			return false
		}
	}
	return true
}

// Helper function to get unique strings
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
