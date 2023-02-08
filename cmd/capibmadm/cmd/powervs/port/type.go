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

// Package port contains the commands to operate on PowerVS Network Port resources.
package port

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PSpec defines a port.
type PSpec struct {
	PortID      string `json:"id"`
	Description string `json:"description"`
	ExternalIP  string `json:"externalIP,omitempty"`
	IPAddress   string `json:"ipAddress"`
	MacAddress  string `json:"macAddress"`
	Status      string `json:"status"`
}

// PList defines a list of Ports.
type PList struct {
	Items []PSpec `json:"items"`
}

// ToTable converts List to *metav1.Table.
func (portList *PList) ToTable() *metav1.Table {
	table := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "DESCRIPTION",
				Type: "string",
			},
			{
				Name: "EXTERNAL IP",
				Type: "string",
			},
			{
				Name: "IP ADDRESS",
				Type: "string",
			},
			{
				Name: "MAC ADDRESS",
				Type: "string",
			},
			{
				Name: "PORT ID",
				Type: "string",
			},
			{
				Name: "STATUS",
				Type: "string",
			},
		},
	}

	for _, port := range portList.Items {
		row := metav1.TableRow{
			Cells: []interface{}{port.Description, port.ExternalIP, port.IPAddress, port.MacAddress, port.PortID, port.Status},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
