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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    outputFormat
		wantErr string
	}{
		{
			name:  "table",
			value: "table",
			want:  outputFormatTable,
		},
		{
			name:  "json",
			value: "json",
			want:  outputFormatJSON,
		},
		{
			name:    "invalid",
			value:   "yaml",
			wantErr: `unsupported output format "yaml"; use table or json`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseOutputFormat(test.value)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestWriteTableOutput(t *testing.T) {
	var output bytes.Buffer

	err := writeTableOutput(&output, tableOutput{
		Headers: []string{"MAC", "STATE"},
		Rows: [][]string{
			{"00:11:22:33:44:55", "waiting"},
			{"66:77:88:99:aa:bb", "selected"},
		},
	})
	require.NoError(t, err)

	assert.Contains(t, output.String(), "MAC")
	assert.Contains(t, output.String(), "STATE")
	assert.Contains(t, output.String(), "00:11:22:33:44:55")
	assert.Contains(t, output.String(), "waiting")
	assert.Contains(t, output.String(), "66:77:88:99:aa:bb")
	assert.Contains(t, output.String(), "selected")
}

func TestWriteTableOutputRejectsMismatchedRows(t *testing.T) {
	var output bytes.Buffer

	err := writeTableOutput(&output, tableOutput{
		Headers: []string{"MAC", "STATE"},
		Rows: [][]string{
			{"00:11:22:33:44:55"},
		},
	})

	assert.ErrorContains(t, err, "table row has 1 columns, expected 2")
}

func TestWriteJSONOutput(t *testing.T) {
	var output bytes.Buffer

	err := writeJSONOutput(&output, struct {
		MAC   string `json:"mac"`
		State string `json:"state"`
	}{
		MAC:   "00:11:22:33:44:55",
		State: "waiting",
	})
	require.NoError(t, err)

	var got map[string]string
	require.NoError(t, json.Unmarshal(output.Bytes(), &got))
	assert.Equal(t, map[string]string{
		"mac":   "00:11:22:33:44:55",
		"state": "waiting",
	}, got)
	assert.Contains(t, output.String(), "\n")
}
