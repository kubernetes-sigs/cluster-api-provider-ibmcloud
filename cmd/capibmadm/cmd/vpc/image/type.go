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
	"github.com/go-openapi/strfmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Image vpc image info.
type Image struct {
	CatalogOffering        bool            `json:"catalogOffering"`
	CreatedAt              strfmt.DateTime `json:"created_at"`
	Encryption             string          `json:"encryption"`
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	OperatingSystemName    string          `json:"operatingSystemName"`
	OperatingSystemVersion string          `json:"operatingSystemVersion"`
	Arch                   string          `json:"arch"`
	FileSize               int64           `json:"fileSizeInGB"`
	SourceVolumeName       string          `json:"sourceVolumeName"`
	ResourceGroupName      string          `json:"resourceGroupName"`
	Status                 string          `json:"status"`
	Visibility             string          `json:"visibility"`
}

// List is list of Image.
type List []Image

// ToTable converts List to *metav1.Table.
func (imageList *List) ToTable() *metav1.Table {
	table := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "ID",
				Type: "string",
			},
			{
				Name: "NAME",
				Type: "string",
			},
			{
				Name: "STATUS",
				Type: "string",
			},
			{
				Name: "CREATED AT",
				Type: "string",
			},
			{
				Name: "OS NAME",
				Type: "string",
			},
			{
				Name: "OS VERSION",
				Type: "string",
			},
			{
				Name: "ARCH",
				Type: "string",
			},
			{
				Name: "FILE SIZE(GB)",
				Type: "integer",
			},
			{
				Name: "SOURCE VOLUME",
				Type: "string",
			},
			{
				Name: "VISIBILITY",
				Type: "string",
			},
			{
				Name: "ENCRYPTION",
				Type: "string",
			},
			{
				Name: "RESOURCE GROUP",
				Type: "string",
			},
			{
				Name: "CATALOG OFFERING",
				Type: "boolean",
			},
		},
	}

	for _, image := range *imageList {
		row := metav1.TableRow{
			Cells: []interface{}{image.ID, image.Name, image.Status, image.CreatedAt, image.OperatingSystemName, image.OperatingSystemVersion, image.Arch, image.FileSize, image.SourceVolumeName, image.Visibility, image.Encryption, image.ResourceGroupName, image.CatalogOffering},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
