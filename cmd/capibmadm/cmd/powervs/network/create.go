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
	"strings"

	"github.com/spf13/cobra"

	v "github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/power/models"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

type networkCreateOptions struct {
	name            string
	private         bool
	public          bool
	cidr            string
	dnsServers      []string
	gateway         string
	jumbo           bool
	ipAddressRanges []string
}

// CreateCommand function to create PowerVS network.
func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create PowerVS network",
		Example: `
# Create PowerVS network
export IBMCLOUD_API_KEY=<api-key>
Public network: capibmadm powervs network create --public --service-instance-id <service-instance-id> --zone <zone>
Private network: capibmadm powervs network create --private --cidr <cidr> --service-instance-id <service-instance-id> --zone <zone>
Private network with ip address ranges: capibmadm powervs network create --private --cidr <cidr> --ip-ranges <start-ip>-<end-ip>,<start-ip>-<end-ip> --service-instance-id <service-instance-id> --zone <zone>
`,
	}

	var netCreateOption networkCreateOptions
	cmd.Flags().StringVar(&netCreateOption.name, "name", "", "The name of the network")
	cmd.Flags().BoolVar(&netCreateOption.public, "public", true, "Public (pub-vlan) network type")
	cmd.Flags().BoolVar(&netCreateOption.private, "private", false, "Private (vlan) network type")
	cmd.Flags().StringVar(&netCreateOption.cidr, "cidr", "", "The network CIDR. Required for private network type")
	cmd.Flags().StringVar(&netCreateOption.name, "gateway", "", "The gateway ip address")
	cmd.Flags().StringSliceVar(&netCreateOption.dnsServers, "dns-servers", []string{"8.8.8.8", "9.9.9.9"}, "Comma separated list of DNS Servers to use")
	cmd.Flags().StringSliceVar(&netCreateOption.ipAddressRanges, "ip-ranges", []string{}, "Comma separated IP Address Ranges")
	cmd.Flags().BoolVar(&netCreateOption.jumbo, "jumbo", false, "Enable MTU Jumbo Network")

	// both cannot be provided, default is public
	cmd.MarkFlagsMutuallyExclusive("private", "public")
	// cidr is required for private vlan
	cmd.MarkFlagsRequiredTogether("private", "cidr")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return createNetwork(cmd.Context(), netCreateOption)
	}
	return cmd
}

func createNetwork(ctx context.Context, netCreateOption networkCreateOptions) error {
	log := logf.Log
	log.Info("Creating PowerVS network", "service-instance-id", options.GlobalOptions.ServiceInstanceID, "zone", options.GlobalOptions.PowerVSZone)

	accountID, err := utils.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}
	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	networkClient := v.NewIBMPINetworkClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)

	// default is public network
	ntype := "pub-vlan"
	if netCreateOption.private {
		ntype = "vlan"
	}

	body := &models.NetworkCreate{
		Name:       netCreateOption.name,
		Type:       &ntype,
		Cidr:       netCreateOption.cidr,
		DNSServers: netCreateOption.dnsServers,
		Gateway:    netCreateOption.gateway,
		Jumbo:      netCreateOption.jumbo,
	}

	var ipAddressRanges []*models.IPAddressRange
	for _, ipRange := range netCreateOption.ipAddressRanges {
		if ipRange != "" {
			ipAddresses := strings.Split(ipRange, "-")
			if len(ipAddresses) != 2 {
				return fmt.Errorf("failed to read ip range, provide a range of IP addresses \"startIP-endIP\"")
			}
			ipAddressRanges = append(ipAddressRanges, &models.IPAddressRange{
				StartingIPAddress: &ipAddresses[0],
				EndingIPAddress:   &ipAddresses[1],
			})
		}
	}
	body.IPAddressRanges = ipAddressRanges

	network, err := networkClient.Create(body)
	if err != nil {
		return fmt.Errorf("failed to create a network, err: %v", err)
	}
	log.Info("Successfully created a network", "networkID", *network.NetworkID)

	return nil
}
