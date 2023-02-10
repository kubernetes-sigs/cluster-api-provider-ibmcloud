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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
)

type keyDeleteOptions struct {
	name string
}

// DeleteCommand vpc key delete command.
func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete VPC key",
		Example: `
# Delete key in VPC
export IBMCLOUD_API_KEY=<api-key>
capibmadm vpc key delete --name <key-name> --region <region>`,
	}

	options.AddCommonFlags(cmd)
	var keyDeleteOption keyDeleteOptions
	cmd.Flags().StringVar(&keyDeleteOption.name, "name", keyDeleteOption.name, "Key Name")
	_ = cmd.MarkFlagRequired("name")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := deleteKey(keyDeleteOption); err != nil {
			return err
		}
		return nil
	}

	return cmd
}

func deleteKey(keyDeleteOption keyDeleteOptions) error {
	log := logf.Log
	vpcClient, err := vpc.NewV1Client(options.GlobalOptions.VPCRegion)
	if err != nil {
		return err
	}

	listKeysOptions := &vpcv1.ListKeysOptions{}
	pager, err := vpcClient.NewKeysPager(listKeysOptions)
	if err != nil {
		panic(err)
	}

	var allResults []vpcv1.Key
	for pager.HasNext() {
		nextPage, err := pager.GetNext()
		if err != nil {
			panic(err)
		}
		allResults = append(allResults, nextPage...)
	}

	var keyID string
	for _, key := range allResults {
		if *key.Name == keyDeleteOption.name {
			keyID = *key.ID
			break
		}
	}

	if keyID == "" {
		return fmt.Errorf("specified key %s could not be found", keyDeleteOption.name)
	}

	options := &vpcv1.DeleteKeyOptions{}
	options.SetID(keyID)

	_, err = vpcClient.DeleteKey(options)
	if err == nil {
		log.Info("VPC Key deleted succssfully,", "key-name", keyDeleteOption.name)
	}
	return err
}
