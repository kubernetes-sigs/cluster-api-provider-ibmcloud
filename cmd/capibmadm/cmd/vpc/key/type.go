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

// Package key contains the commands to operate on vpc key resources.
package key

import (
	"github.com/go-openapi/strfmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Key vpc key info.
type Key struct {
	ID            string          `json:"id"`
	CreatedAt     strfmt.DateTime `json:"created_at"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	ResourceGroup string          `json:"resourceGroup"`
	FingerPrint   string          `json:"fingerPrint"`
	Length        int64           `json:"length"`
}

// List is list of Key.
type List []Key

// ToTable converts List to *metav1.Table.
func (keyList *List) ToTable() *metav1.Table {
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
				Name: "TYPE",
				Type: "string",
			},
			{
				Name: "CREATED AT",
				Type: "string",
			},
			{
				Name: "LENGTH",
				Type: "integer",
			},
			{
				Name: "FINGERPRINT",
				Type: "string",
			},
			{
				Name: "RESOURCE GROUP",
				Type: "string",
			},
		},
	}

	for _, key := range *keyList {
		row := metav1.TableRow{
			Cells: []interface{}{key.ID, key.Name, key.Type, key.CreatedAt, key.Length, key.FingerPrint, key.ResourceGroup},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
