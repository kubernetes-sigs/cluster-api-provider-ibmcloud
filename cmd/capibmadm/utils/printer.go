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

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
)

// PrinterType is a type declaration for a printer type.
type PrinterType string

var (
	// PrinterTypeTable is a table printer type.
	PrinterTypeTable = PrinterType("table")
	// PrinterTypeJSON is a json printer type.
	PrinterTypeJSON = PrinterType("json")
)

var (
	// ErrUnknowPrinterType is an error if a printer type isn't known.
	ErrUnknowPrinterType = errors.New("unknown printer type")
	// ErrTableRequired is an error if the object being printed
	// isn't a metav1.Table.
	ErrTableRequired = errors.New("metav1.Table is required")
)

// Printer is an interface for a printer.
type Printer interface {
	// Print is a method to print an object
	Print(in interface{}) error
}

// NewPrinter creates a new printer.
func NewPrinter(printerType string, writer io.Writer) (Printer, error) {
	switch printerType {
	case string(PrinterTypeTable):
		return &tablePrinter{writer: writer}, nil
	case string(PrinterTypeJSON):
		return &jsonPrinter{writer: writer}, nil
	default:
		return nil, ErrUnknowPrinterType
	}
}

type tablePrinter struct {
	writer io.Writer
}

func (p *tablePrinter) Print(in interface{}) error {
	table, ok := in.(*metav1.Table)
	if !ok {
		return ErrTableRequired
	}

	options := printers.PrintOptions{}
	tablePrinter := printers.NewTablePrinter(options)
	scheme := runtime.NewScheme()
	printer, err := printers.NewTypeSetter(scheme).WrapToPrinter(tablePrinter, nil)
	if err != nil {
		return err
	}

	return printer.PrintObj(table, p.writer)
}

type jsonPrinter struct {
	writer io.Writer
}

func (p *jsonPrinter) Print(in interface{}) error {
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling object as json: %w", err)
	}
	_, err = p.writer.Write(data)
	return err
}
