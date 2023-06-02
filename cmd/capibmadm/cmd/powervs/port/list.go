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

package port

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	powerClient "github.com/IBM-Cloud/power-go-client/clients/instance"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	pkgUtils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

// ListCommand powervs port list command.
func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PowerVS ports",
		Example: `
 # List ports in PowerVS Network
 export IBMCLOUD_API_KEY=<api-key>
 capibmadm powervs port list --service-instance-id <service-instance-id> --zone <zone> --network <network-name/network-id>`,
	}

	var network string

	cmd.Flags().StringVar(&network, "network", "", "Network ID or Name")
	_ = cmd.MarkFlagRequired("network")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return listPorts(cmd.Context(), network)
	}

	options.AddCommonFlags(cmd)
	return cmd
}

func listPorts(ctx context.Context, network string) error {
	log := logf.Log
	log.Info("Listing PowerVS ports", "service-instance-id", options.GlobalOptions.ServiceInstanceID, "network", network)

	accountID, err := pkgUtils.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	networkClient := powerClient.NewIBMPINetworkClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)

	// validating if network exists before fetching all ports.
	_, err = networkClient.Get(network)
	if err != nil {
		return err
	}

	portListResp, err := networkClient.GetAllPorts(network)
	if err != nil {
		return err
	}

	portList := PList{
		Items: []PSpec{},
	}

	for _, port := range portListResp.Ports {
		portList.Items = append(portList.Items, PSpec{
			Description: utils.DereferencePointer(port.Description).(string),
			ExternalIP:  port.ExternalIP,
			IPAddress:   utils.DereferencePointer(port.IPAddress).(string),
			MacAddress:  utils.DereferencePointer(port.MacAddress).(string),
			PortID:      utils.DereferencePointer(port.PortID).(string),
			Status:      utils.DereferencePointer(port.Status).(string),
		})
	}

	printerObj, err := printer.New(options.GlobalOptions.Output, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed creating output printer: %w", err)
	}

	if options.GlobalOptions.Output == printer.PrinterTypeTable {
		table := portList.ToTable()
		err = printerObj.Print(table)
	} else {
		err = printerObj.Print(portList)
	}

	return err
}
