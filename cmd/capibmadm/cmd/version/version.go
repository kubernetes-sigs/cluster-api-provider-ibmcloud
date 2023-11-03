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

package version

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/version"
)

// Version provides the version information of capibmadm.
type Version struct {
	ClientVersion *version.Info `json:"ibmcloudProviderVersion"`
}

// Commands provides the version information capibmadm.
func Commands(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version of capibmadm",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(out, cmd)
		},
	}

	cmd.Flags().StringP("output", "o", "", "The output format of the result. Supported printer types: short, yaml, json")

	return cmd
}

func runVersion(out io.Writer, cmd *cobra.Command) error {
	clientVersion := version.Get()
	v := Version{
		ClientVersion: &clientVersion,
	}

	const flag = "output"
	of, err := cmd.Flags().GetString(flag)
	if err != nil {
		return fmt.Errorf("error accessing flag %s for command %s: %w", flag, cmd.Name(), err)
	}

	switch of {
	case "":
		fmt.Fprintf(out, "capibmadm version: %#v\n", *v.ClientVersion)
	case "short":
		fmt.Fprintf(out, "%s\n", v.ClientVersion.GitVersion)
	case "yaml":
		y, err := yaml.Marshal(&v)
		if err != nil {
			return err
		}
		fmt.Fprint(out, string(y))
	case "json":
		y, err := json.MarshalIndent(&v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(y))
	default:
		return fmt.Errorf("invalid output format: %s", of)
	}

	return nil
}
