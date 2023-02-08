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

// Package options contains the reusable and global variables.
package options

import (
	"github.com/spf13/cobra"
)

// IBMCloudAPIKeyEnvName holds the environmental variable name to set PowerVS service instance ID.
const IBMCloudAPIKeyEnvName = "IBMCLOUD_API_KEY" //nolint:gosec

// GlobalOptions holds the global variable struct.
var GlobalOptions = &options{}

type options struct {
	IBMCloudAPIKey    string
	ServiceInstanceID string
	PowerVSZone       string
	VPCRegion         string
	ResourceGroupName string
	Debug             bool
	Output            string
}

// AddCommonFlags will add common flags to the cli.
func AddCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&GlobalOptions.Output, "output", "o", "table", "The output format of the results. Possible values: table,json,yaml")
}

// AddPowerVSCommonFlags will add a common Power VS flag to the cli.
func AddPowerVSCommonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&GlobalOptions.ServiceInstanceID, "service-instance-id", "", "PowerVS service instance id")
	cmd.PersistentFlags().StringVar(&GlobalOptions.PowerVSZone, "zone", GlobalOptions.PowerVSZone, "IBM cloud PowerVS location. (Required)")
	cmd.PersistentFlags().BoolVar(&GlobalOptions.Debug, "debug", false, "To display the debugging output")

	_ = cmd.MarkPersistentFlagRequired("service-instance-id")
	_ = cmd.MarkPersistentFlagRequired("zone")
}
