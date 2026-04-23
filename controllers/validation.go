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

package powervs

import (
	"context"
	"fmt"
	"sort"

	"github.com/IBM-Cloud/power-go-client/clients/instance"

	powervsscope "sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope/powervs"
)

// validateSystemType validates that the specified systemType is supported by PowerVS.
// This performs dynamic validation against the PowerVS API to check if the systemType is currently supported in any datacenter.
//
// Returns:
//   - bool: true if valid (or empty, since systemType is optional)
//   - []string: list of supported system types (for error messages)
//   - error: any error encountered during validation
func validateSystemType(ctx context.Context, machineScope *powervsscope.MachineScope) (bool, []string, error) {
	systemType := machineScope.IBMPowerVSMachine.Spec.SystemType

	// SystemType is optional - empty string is valid
	if systemType == "" {
		return true, nil, nil
	}

	// Get the PISession from the machine scope
	piSession := machineScope.IBMPowerVSClient.GetPISession()
	if piSession == nil {
		return false, nil, fmt.Errorf("PISession is not available")
	}

	// Dynamically get all available datacenters from PowerVS API
	datacenterClient := instance.NewIBMPIDatacenterClient(ctx, piSession, "")
	datacenters, err := datacenterClient.GetAll()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get all datacenters: %w", err)
	}

	if datacenters == nil || len(datacenters.Datacenters) == 0 {
		return false, nil, fmt.Errorf("no datacenters found")
	}

	// Use a map to collect unique system types across all datacenters
	systemTypesMap := make(map[string]bool)

	for _, dc := range datacenters.Datacenters {
		// Skip datacenters without system type information
		if dc.CapabilitiesDetails == nil || dc.CapabilitiesDetails.SupportedSystems == nil {
			continue
		}

		// Add all system types from this datacenter (map automatically deduplicates)
		for _, sysType := range dc.CapabilitiesDetails.SupportedSystems.General {
			systemTypesMap[sysType] = true
		}
	}

	if len(systemTypesMap) == 0 {
		return false, nil, fmt.Errorf("no system types found across any PowerVS datacenters")
	}

	// Check if the specified systemType is supported
	if systemTypesMap[systemType] {
		return true, nil, nil
	}

	// Convert map to sorted slice for error message
	supportedTypes := make([]string, 0, len(systemTypesMap))
	for sysType := range systemTypesMap {
		supportedTypes = append(supportedTypes, sysType)
	}
	sort.Strings(supportedTypes)

	return false, supportedTypes, nil
}
