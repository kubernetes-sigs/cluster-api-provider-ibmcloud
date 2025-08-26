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

package key

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/power/models"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/accounts"
)

type keyCreateOptions struct {
	key     string
	keyName string
}

// CreateSSHKeyCommand - child command of 'key' to create an SSH key.
func CreateSSHKeyCommand() *cobra.Command {
	var keyCreateOption keyCreateOptions
	var filePath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an SSH key in the PowerVS environment.",
		Example: `
# Create an SSH key.
export IBMCLOUD_API_KEY=<api-key>
Using SSH key : capibmadm powervs key create --name <key-name> --key "<ssh-key>" --service-instance-id <service-instance-id> --zone <zone>
Using file-path to SSH key : capibmadm powervs key create --name <key-name> --key-path <path/to/ssh/key> --service-instance-id <service-instance-id> --zone <zone>`,
	}
	cmd.Flags().StringVar(&keyCreateOption.key, "key", "", "SSH RSA key string within a double quotation marks. For example, \"ssh-rsa AAA... \".")
	cmd.Flags().StringVar(&keyCreateOption.keyName, "name", "", "The name of the SSH key.")
	cmd.Flags().StringVar(&filePath, "key-path", "", "The absolute path to the SSH key file.")
	_ = cmd.MarkFlagRequired("name")

	// TODO: Flag validation is handled in PreRunE until the support for MarkFlagsMutuallyExclusiveAndRequired is available.
	// Related issue: https://github.com/spf13/cobra/issues/1216
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if (keyCreateOption.key == "") == (filePath == "") {
			return fmt.Errorf("the required flags either file-path of SSH key or the SSH key within double quotation marks")
		}
		return nil
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if filePath != "" {
			sshKey, err := os.ReadFile(filePath) // #nosec
			if err != nil {
				return fmt.Errorf("error while reading the SSH key from path: %w", err)
			}
			keyCreateOption.key = string(sshKey)
		}

		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyCreateOption.key)); err != nil {
			return fmt.Errorf("the provided SSH key is invalid: %w", err)
		}
		return createSSHKey(cmd.Context(), keyCreateOption)
	}
	return cmd
}

func createSSHKey(ctx context.Context, keyCreateOption keyCreateOptions) error {
	logger := log.Log
	logger.Info("Creating SSH key...")

	accountID, err := accounts.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}
	keyClient := instance.NewIBMPIKeyClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)

	sshBody := models.SSHKey{Name: &keyCreateOption.keyName, SSHKey: &keyCreateOption.key}
	if _, err = keyClient.Create(&sshBody); err != nil {
		return err
	}
	logger.Info("Successfully created the SSH key.", "name", &keyCreateOption.keyName)
	return nil
}
