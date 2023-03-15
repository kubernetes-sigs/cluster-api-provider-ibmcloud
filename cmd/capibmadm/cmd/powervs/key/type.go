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
	"time"

	"github.com/go-openapi/strfmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SSHKeySpec defines an SSH Key.
type SSHKeySpec struct {
	Name         string          `json:"name"`
	Key          string          `json:"key"`
	CreationDate strfmt.DateTime `json:"creationDate"`
}

// IList defines a list of SSH Keys.
type IList struct {
	Items []SSHKeySpec `json:"items"`
}

// ToTable converts List to *metav1.Table.
func (keyList *IList) ToTable() *metav1.Table {
	table := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "Name",
				Type: "string",
			},
			{
				Name: "Creation Date",
				Type: "string",
			},
			{
				Name: "Key",
				Type: "string",
			},
		},
	}

	for _, key := range keyList.Items {
		row := metav1.TableRow{
			Cells: []interface{}{key.Name, time.Time(key.CreationDate).Format(time.RFC822), key.Key},
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}
