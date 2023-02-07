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

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/cmd/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

func init() {
	verbosity := flag.CommandLine.Int("v", 0, "Set the log level verbosity.")
	logf.SetLogger(logf.NewLogger(logf.WithThreshold(verbosity)))
}

func rootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capibmadm",
		Short: "Kubernetes Cluster API Provider IBM Cloud Management Utility",
		Long:  `capibmadm provides helpers for completing the prerequisite operations for creating IBM Cloud Power VS or VPC clusters.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			apiKey := os.Getenv(options.IBMCloudAPIKeyEnvName)
			if apiKey == "" {
				return fmt.Errorf("ibmcloud api key is not provided, set %s environmental variable", options.IBMCloudAPIKeyEnvName)
			}
			options.GlobalOptions.IBMCloudAPIKey = apiKey
			return nil
		},
	}

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	cmd.AddCommand(powervs.Commands())
	cmd.AddCommand(vpc.Commands())

	return cmd
}

// Execute executes the root command.
func Execute() {
	cmd := rootCommand()

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		fmt.Fprintln(os.Stderr, "\nAborted...")
		cancel()
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
