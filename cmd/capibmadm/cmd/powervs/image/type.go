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

// ImgSpec defines a Image.
type ImgSpec struct {
	ImageID         string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	State           string          `json:"state"`
	StoragePool     string          `json:"storagePool"`
	StorageType     string          `json:"storageType"`
	CreationDate    strfmt.DateTime `json:"creationDate"`
	LastUpdateDate  strfmt.DateTime `json:"lastUpdateDate"`
	Architecture    string          `json:"architecture,omitempty"`
	ContainerFormat string          `json:"containerFormat,omitempty"`
	DiskFormat      string          `json:"diskFormat,omitempty"`
	Endianness      string          `json:"endianness,omitempty"`
	HypervisorType  string          `json:"hypervisorType,omitempty"`
	ImageType       string          `json:"imageType,omitempty"`
	OperatingSystem string          `json:"operatingSystem,omitempty"`
}

// ImgList defines a list of Images.
type ImgList struct {
	Items []ImgSpec `json:"items"`
}

// ToTable converts List to *metav1.Table.
func (imageList *ImgList) ToTable() *metav1.Table {
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
				Name: "STATE",
				Type: "string",
			},
			{
				Name: "DESCRIPTION",
				Type: "string",
			},
			{
				Name: "STORAGE POOL",
				Type: "string",
			},
			{
				Name: "STORAGE TYPE",
				Type: "string",
			},
			{
				Name: "CREATION DATE",
				Type: "string",
			},
			{
				Name: "LAST UPDATE DATE",
				Type: "string",
			},
			{
				Name: "ARCH",
				Type: "string",
			},
			{
				Name: "CONTAINER FORMAT",
				Type: "string",
			},
			{
				Name: "DISK FORMAT",
				Type: "string",
			},
			{
				Name: "ENDIANNESS",
				Type: "string",
			},
			{
				Name: "HYPERVISOR TYPE ",
				Type: "string",
			},
			{
				Name: "OS",
				Type: "string",
			},
			{
				Name: "IMAGE TYPE",
				Type: "string",
			},
		},
	}

	for _, image := range imageList.Items {
		row := metav1.TableRow{
			Cells: []interface{}{image.ImageID, image.Name, image.State, image.Description, image.StoragePool, image.StorageType, image.CreationDate, image.LastUpdateDate, image.Architecture, image.ContainerFormat, image.DiskFormat, image.Endianness, image.HypervisorType, image.OperatingSystem, image.ImageType},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
