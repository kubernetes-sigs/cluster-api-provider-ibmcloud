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

package resourcecontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCOSResourcePlanID(t *testing.T) {
	testCases := []struct {
		name          string
		stagingEnvVal string
		expectedPlan  string
	}{
		{
			name:          "when IBMCLOUD_STAGING is not set",
			stagingEnvVal: "",
			expectedPlan:  CosResourcePlanID,
		},
		{
			name:          "when IBMCLOUD_STAGING is set",
			stagingEnvVal: "true",
			expectedPlan:  CosResourceLitePlanID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.stagingEnvVal != "" {
				t.Setenv("IBMCLOUD_STAGING", tc.stagingEnvVal)
			} else {
				t.Setenv("IBMCLOUD_STAGING", "")
			}
			planID := GetCOSResourcePlanID()
			assert.Equal(t, tc.expectedPlan, planID)
		})
	}
}
