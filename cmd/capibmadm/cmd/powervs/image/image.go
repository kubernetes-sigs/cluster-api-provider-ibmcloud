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

// Package image contains the commands to operate on PowerVS image resources.
package image

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// Commands function to add PowerVS image commands.
func Commands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Perform PowerVS image operations",
	}
	options.AddCommonFlags(cmd)

	cmd.AddCommand(ListCommand())
	cmd.AddCommand(ImportCommand())

	return cmd
}
