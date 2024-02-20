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

// Package vpc contains the commands to operate on vpc resources.
package vpc

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/vpc/image"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/vpc/key"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// Commands initialises and returns VPC command.
func Commands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vpc",
		Short: "Commands for operations on VPC resources",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			apiKey := os.Getenv(options.IBMCloudAPIKeyEnvName)
			if apiKey == "" {
				return fmt.Errorf("ibmcloud api key is not provided, set %s environmental variable", options.IBMCloudAPIKeyEnvName)
			}
			options.GlobalOptions.IBMCloudAPIKey = apiKey
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&options.GlobalOptions.VPCRegion, "region", options.GlobalOptions.VPCRegion, "IBM cloud vpc region. (Required)")
	cmd.PersistentFlags().StringVar(&options.GlobalOptions.ResourceGroupName, "resource-group-name", options.GlobalOptions.ResourceGroupName, "IBM cloud resource group name")

	_ = cmd.MarkPersistentFlagRequired("region")

	cmd.AddCommand(key.Commands())
	cmd.AddCommand(image.Commands())

	return cmd
}
