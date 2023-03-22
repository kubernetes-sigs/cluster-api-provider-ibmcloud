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
	"fmt"
	"os"

	"github.com/go-openapi/strfmt"
	"github.com/spf13/cobra"

	powerClient "github.com/IBM-Cloud/power-go-client/clients/instance"

	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/clients/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/printer"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/capibmadm/utils"
)

// ListCommand powervs image list command.
func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PowerVS image",
		Example: `
# List PowerVS images
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs image list --service-instance-id <service-instance-id> --zone <zone>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listimage(cmd.Context())
		},
	}
	options.AddCommonFlags(cmd)
	return cmd
}

func listimage(ctx context.Context) error {
	log := logf.Log
	log.Info("Listing PowerVS images", "service-instance-id", options.GlobalOptions.ServiceInstanceID)

	accountID, err := utils.GetAccountID(ctx)
	if err != nil {
		return err
	}

	sess, err := powervs.NewPISession(accountID, options.GlobalOptions.PowerVSZone, options.GlobalOptions.Debug)
	if err != nil {
		return err
	}

	imageClient := powerClient.NewIBMPIImageClient(ctx, sess, options.GlobalOptions.ServiceInstanceID)
	images, err := imageClient.GetAll()
	if err != nil {
		return err
	}
	if len(images.Images) == 0 {
		fmt.Println("No images found")
		return nil
	}

	imageList := ImgList{
		Items: []ImgSpec{},
	}

	for _, image := range images.Images {
		imageToAppend := ImgSpec{
			ImageID:        utils.DereferencePointer(image.ImageID).(string),
			Name:           utils.DereferencePointer(image.Name).(string),
			Description:    utils.DereferencePointer(image.Description).(string),
			State:          utils.DereferencePointer(image.State).(string),
			StoragePool:    utils.DereferencePointer(image.StoragePool).(string),
			StorageType:    utils.DereferencePointer(image.StorageType).(string),
			CreationDate:   utils.DereferencePointer(image.CreationDate).(strfmt.DateTime),
			LastUpdateDate: utils.DereferencePointer(image.LastUpdateDate).(strfmt.DateTime),
		}
		if image.Specifications != nil {
			imageToAppend.Architecture = image.Specifications.Architecture
			imageToAppend.ContainerFormat = image.Specifications.ContainerFormat
			imageToAppend.DiskFormat = image.Specifications.DiskFormat
			imageToAppend.Endianness = image.Specifications.Endianness
			imageToAppend.HypervisorType = image.Specifications.HypervisorType
			imageToAppend.ImageType = image.Specifications.ImageType
			imageToAppend.OperatingSystem = image.Specifications.OperatingSystem
		}

		imageList.Items = append(imageList.Items, imageToAppend)
	}

	printerObj, err := printer.New(options.GlobalOptions.Output, os.Stdout)

	if err != nil {
		return err
	}

	switch options.GlobalOptions.Output {
	case printer.PrinterTypeJSON:
		err = printerObj.Print(imageList)
	default:
		table := imageList.ToTable()
		err = printerObj.Print(table)
	}

	return err
}
