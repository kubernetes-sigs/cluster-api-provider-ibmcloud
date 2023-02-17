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

// Package port contains the commands to operate on PowerVS Port resources.
package port

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

// Commands function to add PowerVS port commands.
func Commands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port",
		Short: "Perform PowerVS port operations",
	}
	options.AddCommonFlags(cmd)

	cmd.AddCommand(DeleteCommand())
	cmd.AddCommand(ListCommand())
	cmd.AddCommand(CreateCommand())

	return cmd
}
