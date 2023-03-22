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

	v "github.com/IBM-Cloud/power-go-client/clients/instance"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
)

// ListSSHKeyCommand function to list PowerVS SSH Keys.
func ListSSHKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SSH Keys",
		Example: `
# List PowerVS SSH Keys
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs key list --service-instance-id <service-instance-id> --zone <zone>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSSHKeys(cmd.Context())
		},
	}

	options.AddCommonFlags(cmd)
	return cmd
}

func listSSHKeys(ctx context.Context) error {
	log := logf.Log
	log.Info("Listing PowerVS SSH Keys", "service-instance-id", options.GlobalOptions.ServiceInstanceID, "zone", options.GlobalOptions.PowerVSZone)

	accountID, err := utils.GetAccountID(ctx)
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	c := v.NewIBMPIKeyClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)
	keys, err := c.GetAll()
	if err != nil {
		return err
	}

	if len(keys.SSHKeys) == 0 {
		fmt.Println("No SSH Key found")
		return nil
	}

	listByVersion := IList{
		Items: []SSHKeySpec{},
	}

	for _, key := range keys.SSHKeys {
		listByVersion.Items = append(listByVersion.Items, SSHKeySpec{
			Name:         *key.Name,
			Key:          *key.SSHKey,
			CreationDate: *key.CreationDate,
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
