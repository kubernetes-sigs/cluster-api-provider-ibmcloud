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

// Package key contains the commands to operate on PowerVS ssh-key related resources.
package key

import (
	"github.com/spf13/cobra"
)

// Commands - A collection of supported commands for SSH key management in the PowerVS environment.
func Commands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Parent command for operations related to keys in the PowerVS environment.",
	}
	cmd.AddCommand(CreateSSHKeyCommand())
	cmd.AddCommand(DeleteSSHKeyCommand())
	cmd.AddCommand(ListSSHKeyCommand())
	return cmd
}
