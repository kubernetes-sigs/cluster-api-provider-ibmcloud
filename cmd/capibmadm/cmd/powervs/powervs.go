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

package powervs

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/powervs/image"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/powervs/key"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/powervs/network"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/powervs/port"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// Commands initialises and returns powervs command.
func Commands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "powervs",
		Short: "Commands for operations on PowerVS resources",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			apiKey := os.Getenv(options.IBMCloudAPIKeyEnvName)
			if apiKey == "" {
				return fmt.Errorf("ibmcloud api key is not provided, set %s environmental variable", options.IBMCloudAPIKeyEnvName)
			}
			options.GlobalOptions.IBMCloudAPIKey = apiKey
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&options.GlobalOptions.ServiceInstanceID, "service-instance-id", "", "PowerVS service instance id (Required)")
	cmd.PersistentFlags().StringVar(&options.GlobalOptions.PowerVSZone, "zone", options.GlobalOptions.PowerVSZone, "PowerVS service instance location (Required)")
	cmd.PersistentFlags().BoolVar(&options.GlobalOptions.Debug, "debug", false, "Enable/Disable http transport debugging log")

	_ = cmd.MarkPersistentFlagRequired("service-instance-id")
	_ = cmd.MarkPersistentFlagRequired("zone")

	cmd.AddCommand(key.Commands())
	cmd.AddCommand(network.Commands())
	cmd.AddCommand(port.Commands())
	cmd.AddCommand(image.Commands())

	return cmd
}
