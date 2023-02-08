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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetSpec defines a Network.
type NetSpec struct {
	NetworkID   string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	VlanID      float64 `json:"vlanID"`
	Jumbo       bool    `json:"jumbo"`
	DhcpManaged bool    `json:"dhcpManaged"`
}

// IList defines a list of Networks.
type IList struct {
	Items []NetSpec `json:"items"`
}

// ToTable converts List to *metav1.Table.
func (netList *IList) ToTable() *metav1.Table {
	table := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "NETWORK ID",
				Type: "string",
			},
			{
				Name: "Name",
				Type: "string",
			},
			{
				Name: "Type",
				Type: "string",
			},
			{
				Name: "VLAN ID",
				Type: "string",
			},
			{
				Name: "Jumbo",
				Type: "bool",
			},
			{
				Name: "DHCP Managed",
				Type: "bool",
			},
		},
	}

	for _, network := range netList.Items {
		row := metav1.TableRow{
			Cells: []interface{}{network.NetworkID, network.Name, network.Type, network.VlanID, network.Jumbo, network.DhcpManaged},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
