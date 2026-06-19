// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	cli "github.com/urfave/cli/v3"
)

type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
)

type tableOutput struct {
	Headers []string
	Rows    [][]string
}

func inspectionOutputFlag() cli.Flag {
	return &cli.StringFlag{
		Name:      "output",
		Value:     string(outputFormatTable),
		Usage:     "Output format: table or json",
		Validator: validateOutputFormat,
	}
}

func outputFormatFromCommand(cmd *cli.Command) (outputFormat, error) {
	return parseOutputFormat(cmd.String("output"))
}

func validateOutputFormat(value string) error {
	_, err := parseOutputFormat(value)
	return err
}

func parseOutputFormat(value string) (outputFormat, error) {
	switch outputFormat(value) {
	case outputFormatTable:
		return outputFormatTable, nil
	case outputFormatJSON:
		return outputFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported output format %q; use table or json", value)
	}
}

func writeJSONOutput(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(value)
}

func writeTableOutput(w io.Writer, table tableOutput) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(table.Headers) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(table.Headers, "\t")); err != nil {
			return err
		}
	}
	for _, row := range table.Rows {
		if len(row) != len(table.Headers) {
			return fmt.Errorf("table row has %d columns, expected %d", len(row), len(table.Headers))
		}
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}
