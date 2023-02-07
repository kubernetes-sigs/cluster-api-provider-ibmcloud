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

// Package printer implements printing functionality for cli.
package printer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
)

// PType is a type declaration for a printer type.
type PType string

// String type casts to string.
func (p *PType) String() string {
	return string(*p)
}

// Set sets value for var.
func (p *PType) Set(s string) error {
	switch s {
	case string(PrinterTypeTable), string(PrinterTypeJSON):
		*p = PType(s)
		return nil
	default:
		return ErrUnknowPrinterType
	}
}

// Type returns type in string format.
func (p *PType) Type() string {
	return "PType"
}

const (
	// PrinterTypeTable is a table printer PType.
	PrinterTypeTable = PType("table")
	// PrinterTypeJSON is a json printer PType.
	PrinterTypeJSON = PType("json")
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

// New creates a new printer.
func New(printerType PType, writer io.Writer) (Printer, error) {
	switch printerType {
	case PrinterTypeTable:
		return &tablePrinter{writer: writer}, nil
	case PrinterTypeJSON:
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
