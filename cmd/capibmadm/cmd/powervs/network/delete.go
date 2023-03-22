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

package network

import (
	"context"

	"github.com/spf13/cobra"

	v "github.com/IBM-Cloud/power-go-client/clients/instance"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
)

// DeleteCommand function to delete network.
func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete PowerVS network",
		Example: `
# Delete PowerVS network
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network delete --network <network-name/network-id> --service-instance-id <service-instance-id> --zone <zone>`,
	}

	var networkID string
	cmd.Flags().StringVar(&networkID, "network", "", "Network ID or Name")
	_ = cmd.MarkFlagRequired("network")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return deleteNetwork(cmd.Context(), networkID)
	}
	return cmd
}

func deleteNetwork(ctx context.Context, networkID string) error {
	log := logf.Log
	log.Info("Deleting PowerVS network", "service-instance-id", options.GlobalOptions.ServiceInstanceID, "zone", options.GlobalOptions.PowerVSZone)

	accountID, err := utils.GetAccountID(ctx)
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	networkClient := v.NewIBMPINetworkClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)

	if err = networkClient.Delete(networkID); err != nil {
		return err
	}

	log.Info("Successfully deleted a network", "network", networkID)
	return nil
}
