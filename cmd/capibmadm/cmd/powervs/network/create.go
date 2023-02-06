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
	"fmt"

	"github.com/spf13/cobra"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

type networkCreateOptions struct {
	name       string
	dnsServers []string
}

// CreateCommand function to create PowerVS network.
func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create NETWORK_NAME",
		Short: "Create PowerVS network",
		Example: `
# Create PowerVS network with name capi-network
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network create capi-network --service-instance-id <service-instance-id>`,
	}

	var netCreateOption networkCreateOptions
	cmd.Flags().StringSliceVar(&netCreateOption.dnsServers, "dns-servers", []string{"8.8.8.8", "9.9.9.9"}, "Comma separated list of DNS Servers to use for this network")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("network name is not provided")
		}
		netCreateOption.name = args[0]
		if err := createNetwork(netCreateOption); err != nil {
			return err
		}
		return nil
	}
	return cmd
}

func createNetwork(netCreateOption networkCreateOptions) error {
	log := logf.Log
	log.Info("Creating Power VS network", "name", netCreateOption.name, "service-instance-id", options.GlobalOptions.ServiceInstanceID, "dns-servers", netCreateOption.dnsServers)
	//TODO: add network creation logic here
	return nil
}
