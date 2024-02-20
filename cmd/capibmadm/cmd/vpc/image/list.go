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

package image

import (
	"context"
	"os"

	"github.com/go-openapi/strfmt"
	"github.com/spf13/cobra"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/iam"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
	pkgUtils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

// ListCommand vpc image list command.
func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List VPC images",
		Example: `
 # List images in VPC
 export IBMCLOUD_API_KEY=<api-key>
 capibmadm vpc image list --region <region> --resource-group-name <resource-group-name>`,
	}

	options.AddCommonFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return listImages(cmd.Context(), options.GlobalOptions.ResourceGroupName)
	}

	return cmd
}

func listImages(ctx context.Context, resourceGroupName string) error {
	v1, err := vpc.NewV1Client(options.GlobalOptions.VPCRegion)
	if err != nil {
		return err
	}

	accountID, err := pkgUtils.GetAccount(iam.GetIAMAuth())
	if err != nil {
		return err
	}

	var resourceGroupID string
	if resourceGroupName != "" {
		resourceGroupID, err = utils.GetResourceGroupID(ctx, resourceGroupName, accountID)
		if err != nil {
			return err
		}
	}

	var imageNesList []*vpcv1.ImageCollection
	f := func(start string) (bool, string, error) {
		var listImageOpt vpcv1.ListImagesOptions

		if resourceGroupID != "" {
			listImageOpt.ResourceGroupID = &resourceGroupID
		}
		if start != "" {
			listImageOpt.Start = &start
		}

		imageL, _, err := v1.ListImagesWithContext(ctx, &listImageOpt)
		if err != nil {
			return false, "", err
		}
		imageNesList = append(imageNesList, imageL)

		if imageL.Next != nil && *imageL.Next.Href != "" {
			return false, *imageL.Next.Href, nil
		}

		return true, "", nil
	}

	if err = pkgUtils.PagingHelper(f); err != nil {
		return err
	}

	return display(imageNesList)
}

func display(imageNesList []*vpcv1.ImageCollection) error {
	var imageListToDisplay List
	for _, imageL := range imageNesList {
		for _, image := range imageL.Images {
			imageToAppend := Image{
				ID:         utils.DereferencePointer(image.ID).(string),
				Name:       utils.DereferencePointer(image.Name).(string),
				Status:     utils.DereferencePointer(image.Status).(string),
				CreatedAt:  utils.DereferencePointer(image.CreatedAt).(strfmt.DateTime),
				Visibility: utils.DereferencePointer(image.Visibility).(string),
				Encryption: utils.DereferencePointer(image.Encryption).(string),
			}

			if image.File != nil {
				imageToAppend.FileSize = utils.DereferencePointer(image.File.Size).(int64)
			}

			if image.ResourceGroup != nil {
				imageToAppend.ResourceGroupName = utils.DereferencePointer(image.ResourceGroup.Name).(string)
			}

			if image.OperatingSystem != nil {
				imageToAppend.OperatingSystemName = utils.DereferencePointer(image.OperatingSystem.DisplayName).(string)
				imageToAppend.OperatingSystemVersion = utils.DereferencePointer(image.OperatingSystem.Version).(string)
				imageToAppend.Arch = utils.DereferencePointer(image.OperatingSystem.Architecture).(string)
			}

			if image.SourceVolume != nil {
				imageToAppend.SourceVolumeName = utils.DereferencePointer(image.SourceVolume.Name).(string)
			}

			if image.CatalogOffering != nil {
				imageToAppend.CatalogOffering = utils.DereferencePointer(image.CatalogOffering.Managed).(bool)
			}

			imageListToDisplay = append(imageListToDisplay, imageToAppend)
		}
	}

	p, err := printer.New(options.GlobalOptions.Output, os.Stdout)

	if err != nil {
		return err
	}

	switch options.GlobalOptions.Output {
	case printer.PrinterTypeJSON:
		err = p.Print(imageListToDisplay)
	default:
		table := imageListToDisplay.ToTable()
		err = p.Print(table)
	}

	return err
}
