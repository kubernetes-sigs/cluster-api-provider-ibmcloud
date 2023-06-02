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

	"github.com/IBM/vpc-go-sdk/vpcv1"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	pkgUtils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

type keyCreateOptions struct {
	name              string
	publicKey         string
	resourceGroupName string
}

// CreateCommand vpc key create command.
func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create VPC key",
		Example: `
# Create key in VPC
export IBMCLOUD_API_KEY=<api-key>
capibmadm vpc key create --name <key-name> --region <region> --public-key "<public-key-string>"
Using file-path to SSH key : capibmadm vpc key create --name <key-name> --region <region> --key-path <path/to/ssh/key>
`,
	}

	options.AddCommonFlags(cmd)
	var keyCreateOption keyCreateOptions
	var filePath string
	cmd.Flags().StringVar(&keyCreateOption.name, "name", keyCreateOption.name, "Key Name")
	cmd.Flags().StringVar(&filePath, "key-path", "", "The absolute path to the SSH key file.")
	cmd.Flags().StringVar(&keyCreateOption.publicKey, "public-key", keyCreateOption.publicKey, "Public Key")
	_ = cmd.MarkFlagRequired("name")
	// TODO: Flag validation is handled in PreRunE until the support for MarkFlagsMutuallyExclusiveAndRequired is available.
	// Related issue: https://github.com/spf13/cobra/issues/1216
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if (keyCreateOption.publicKey == "") == (filePath == "") {
			return fmt.Errorf("the required flags either key-path of SSH key or the public-key within double quotation marks is not found")
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if filePath != "" {
			sshKey, err := os.ReadFile(filePath) // #nosec
			if err != nil {
				return fmt.Errorf("error while reading the SSH key from path. %w", err)
			}
			keyCreateOption.publicKey = string(sshKey)
		}

		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyCreateOption.publicKey)); err != nil {
			return fmt.Errorf("the provided SSH key is invalid. %w ", err)
		}
		return createKey(cmd.Context(), keyCreateOption)
	}
	return cmd
}

func createKey(ctx context.Context, keyCreateOption keyCreateOptions) error {
	log := logf.Log
	vpcClient, err := vpc.NewV1Client(options.GlobalOptions.VPCRegion)
	if err != nil {
		return err
	}

	accountID, err := pkgUtils.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}

	options := &vpcv1.CreateKeyOptions{}

	options.SetName(keyCreateOption.name)
	options.SetPublicKey(keyCreateOption.publicKey)

	if keyCreateOption.resourceGroupName != "" {
		resourceGroupID, err := utils.GetResourceGroupID(ctx, keyCreateOption.resourceGroupName, accountID)
		if err != nil {
			return err
		}
		resourceGroup := &vpcv1.ResourceGroupIdentity{
			ID: &resourceGroupID,
		}
		options.SetResourceGroup(resourceGroup)
	}

	key, _, err := vpcClient.CreateKey(options)
	if err != nil {
		return err
	}
	log.Info("SSH Key created successfully,", "key-name", *key.Name)
	return nil
}
