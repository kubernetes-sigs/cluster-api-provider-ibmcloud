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

	client "github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/power/models"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	pkgUtils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

type portCreateOptions struct {
	network     string
	ipAddress   string
	description string
}

// CreateCommand create powervs network port.
func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create PowerVS Port",
		Example: `
# Create PowerVS network port
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs port create --network <netword-id/network-name> --description <description> --service-instance-id <service-instance-id> --zone <zone>`,
	}

	var portCreateOption portCreateOptions
	cmd.Flags().StringVar(&portCreateOption.network, "network", "", "Network ID or Name on which port is to be created")
	cmd.Flags().StringVar(&portCreateOption.ipAddress, "ip-address", "", "IP Address to be assigned to the port")
	cmd.Flags().StringVar(&portCreateOption.description, "description", "", "Description of the port")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return createPort(cmd.Context(), portCreateOption)
	}
	options.AddCommonFlags(cmd)
	_ = cmd.MarkFlagRequired("network")
	return cmd
}

func createPort(ctx context.Context, portCreateOption portCreateOptions) error {
	logger := log.Log
	logger.Info("Creating Port ", "Network ID/Name", portCreateOption.network, "IP Address", portCreateOption.ipAddress, "Description", portCreateOption.description, "service-instance-id", options.GlobalOptions.ServiceInstanceID, "zone", options.GlobalOptions.PowerVSZone)
	accountID, err := pkgUtils.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}
	session, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	networkClient := client.NewIBMPINetworkClient(ctx, session, options.GlobalOptions.ServiceInstanceID)

	network, err := networkClient.Get(portCreateOption.network)
	if err != nil {
		return err
	}

	params := &models.NetworkPortCreate{
		IPAddress:   portCreateOption.ipAddress,
		Description: portCreateOption.description,
	}

	port, err := networkClient.CreatePort(*network.NetworkID, params)
	if err != nil {
		return fmt.Errorf("failed to create a port, err: %v", err)
	}
	logger.Info("Successfully created a port", "portID", *port.PortID)

	portInfo := PList{
		Items: []PSpec{},
	}

	portInfo.Items = append(portInfo.Items, PSpec{
		Description: utils.DereferencePointer(port.Description).(string),
		ExternalIP:  port.ExternalIP,
		IPAddress:   utils.DereferencePointer(port.IPAddress).(string),
		MacAddress:  utils.DereferencePointer(port.MacAddress).(string),
		PortID:      utils.DereferencePointer(port.PortID).(string),
		Status:      utils.DereferencePointer(port.Status).(string),
	})

	printerObj, err := printer.New(options.GlobalOptions.Output, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed creating output printer: %w", err)
	}

	if options.GlobalOptions.Output == printer.PrinterTypeTable {
		table := portInfo.ToTable()
		err = printerObj.Print(table)
	} else {
		err = printerObj.Print(portInfo)
	}
	return err
}
