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

	"github.com/spf13/cobra"

	"github.com/IBM-Cloud/power-go-client/clients/instance"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/accounts"
)

// DeleteSSHKeyCommand - child command of 'key' to delete an SSH key.
func DeleteSSHKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an SSH key in the PowerVS environment.",
		Example: `
# Delete an SSH key.
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs key delete --name <key-name> --service-instance-id <service-instance-id> --zone <zone>`,
	}

	var keyName string
	cmd.Flags().StringVar(&keyName, "name", "", "The name of the SSH key.")
	_ = cmd.MarkFlagRequired("name")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return deleteSSHKey(cmd.Context(), keyName)
	}
	return cmd
}

func deleteSSHKey(ctx context.Context, keyName string) error {
	logger := log.Log
	logger.Info("Deleting SSH key...")

	accountID, err := accounts.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	keyClient := instance.NewIBMPIKeyClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)
	if err = keyClient.Delete(keyName); err != nil {
		return err
	}
	logger.Info("Successfully deleted the SSH key.", "name", keyName)
	return nil
}
