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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	v "github.com/IBM-Cloud/power-go-client/clients/instance"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
)

// ListCommand function to create PowerVS network.
func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PowerVS network",
		Example: `
# List PowerVS networks
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network list --service-instance-id <service-instance-id> --zone <zone>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := listNetwork(cmd.Context()); err != nil {
				return err
			}
			return nil
		},
	}

	options.AddCommonFlags(cmd)
	return cmd
}

func listNetwork(ctx context.Context) error {
	log := logf.Log
	log.Info("Listing PowerVS networks", "service-instance-id", options.GlobalOptions.ServiceInstanceID, "zone", options.GlobalOptions.PowerVSZone)

	accountID, err := utils.GetAccountID(ctx, iam.GetIAMAuth())
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	c := v.NewIBMPINetworkClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)
	nets, err := c.GetAll()
	if err != nil {
		return err
	}

	if len(nets.Networks) == 0 {
		fmt.Println("No Networks found")
		return nil
	}

	listByVersion := IList{
		Items: []NetSpec{},
	}

	for _, network := range nets.Networks {
		listByVersion.Items = append(listByVersion.Items, NetSpec{
			NetworkID:   *network.NetworkID,
			Name:        *network.Name,
			Type:        *network.Type,
			VlanID:      *network.VlanID,
			Jumbo:       *network.Jumbo,
			DhcpManaged: network.DhcpManaged,
		})
	}

	pr, err := printer.New(options.GlobalOptions.Output, os.Stdout)
	if err != nil {
		return err
	}

	if options.GlobalOptions.Output == printer.PrinterTypeJSON {
		err = pr.Print(listByVersion)
	} else {
		table := listByVersion.ToTable()
		err = pr.Print(table)
	}

	return err
}
