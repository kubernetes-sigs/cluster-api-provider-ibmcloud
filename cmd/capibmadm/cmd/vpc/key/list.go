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
	"os"

	"github.com/go-openapi/strfmt"
	"github.com/spf13/cobra"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	pagingUtil "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

// ListCommand vpc key list command.
func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List VPC key",
		Example: `
# List key in VPC
export IBMCLOUD_API_KEY=<api-key>
capibmadm vpc key list --region <region> --resource-group-name <resource-group-name>`,
	}

	options.AddCommonFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return listKeys(cmd.Context())
	}

	return cmd
}

func listKeys(ctx context.Context) error {
	v1, err := vpc.NewV1Client(options.GlobalOptions.VPCRegion)
	if err != nil {
		return err
	}

	var keyNesList []*vpcv1.KeyCollection
	f := func(start string) (bool, string, error) {
		var listKeyOpt vpcv1.ListKeysOptions

		if start != "" {
			listKeyOpt.Start = &start
		}

		keyL, _, err := v1.ListKeysWithContext(ctx, &listKeyOpt)
		if err != nil {
			return false, "", err
		}
		keyNesList = append(keyNesList, keyL)

		if keyL.Next != nil && *keyL.Next.Href != "" {
			return false, *keyL.Next.Href, nil
		}

		return true, "", nil
	}

	if err = pagingUtil.PagingHelper(f); err != nil {
		return err
	}

	return display(keyNesList)
}

func display(keyNesList []*vpcv1.KeyCollection) error {
	var keyListToDisplay List
	for _, keyL := range keyNesList {
		for _, key := range keyL.Keys {
			keyToAppend := Key{
				CreatedAt:   utils.DereferencePointer(key.CreatedAt).(strfmt.DateTime),
				ID:          utils.DereferencePointer(key.ID).(string),
				Name:        utils.DereferencePointer(key.Name).(string),
				Type:        utils.DereferencePointer(key.Type).(string),
				Length:      utils.DereferencePointer(key.Length).(int64),
				FingerPrint: utils.DereferencePointer(key.Fingerprint).(string),
			}

			if key.ResourceGroup != nil {
				keyToAppend.ResourceGroup = utils.DereferencePointer(key.ResourceGroup.Name).(string)
			}

			keyListToDisplay = append(keyListToDisplay, keyToAppend)
		}
	}

	printkeys, err := printer.New(options.GlobalOptions.Output, os.Stdout)

	if err != nil {
		return err
	}

	switch options.GlobalOptions.Output {
	case printer.PrinterTypeJSON:
		err = printkeys.Print(keyListToDisplay)
	default:
		table := keyListToDisplay.ToTable()
		err = printkeys.Print(table)
	}

	return err
}
