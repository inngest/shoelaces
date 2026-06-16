// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mappings

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMappingsLoadsNewSchema(t *testing.T) {
	mappingsPath := writeMappingsFile(t, `
defaults:
  params:
    install_username: infra
targets:
  debian12:
    script: debian.ipxe
    label: Debian 12 Bookworm
    environment: testing
    params:
      release: bookworm
  debian13:
    script: debian.ipxe
    params:
      release: trixie
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
      - debian13
    params:
      role: network
hostnameMaps:
  - hostname: '^host-\d+$'
    defaultTarget: debian12
    targets:
      - debian12
macMaps:
  - mac: "0c:42:a1:c3:52:96"
    defaultTarget: debian12
    targets:
      - debian12
ipMaps:
  - ip: 2001:db8::1
    defaultTarget: debian13
    targets:
      - debian13
`)

	parsed, err := ParseMappings(log.MakeLogger(io.Discard), mappingsPath)

	require.NoError(t, err)
	require.Len(t, parsed.Targets, 2)
	assert.Equal(t, "infra", parsed.Defaults.Params["install_username"])
	assert.Equal(t, "debian.ipxe", parsed.Targets["debian12"].Script)
	assert.Equal(t, "Debian 12 Bookworm", parsed.Targets["debian12"].Label)
	assert.Equal(t, "testing", parsed.Targets["debian12"].Environment)
	assert.Equal(t, "bookworm", parsed.Targets["debian12"].Params["release"])
	require.Len(t, parsed.NetworkMaps, 1)
	assert.Equal(t, "debian12", parsed.NetworkMaps[0].DefaultTarget)
	assert.Equal(t, []string{"debian12", "debian13"}, parsed.NetworkMaps[0].Targets)
	require.Len(t, parsed.HostnameMaps, 1)
	require.Len(t, parsed.MacMaps, 1)
	require.Len(t, parsed.IPMaps, 1)
}

func TestParseMappingsLoadsRepositoryExample(t *testing.T) {
	parsed, err := ParseMappings(log.MakeLogger(io.Discard), filepath.Join("..", "configs", "data-dir", "mappings.yaml"))

	require.NoError(t, err)
	assert.NotEmpty(t, parsed.Targets)
	assert.NotEmpty(t, parsed.NetworkMaps)
}

func TestParseMappingsReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "invalid yaml",
			content: "targets:\n  debian12: [",
			want:    "read mappings",
		},
		{
			name: "unknown top-level key",
			content: `
bogus:
  value: true
`,
			want: `unknown top-level mappings key "bogus"`,
		},
		{
			name: "unknown target reference",
			content: `
targets:
  debian12:
    script: debian.ipxe
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian13
    targets:
      - debian13
`,
			want: `networkMaps[0] references unknown target "debian13"`,
		},
		{
			name: "default target outside allowed targets",
			content: `
targets:
  debian12:
    script: debian.ipxe
  debian13:
    script: debian.ipxe
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian13
    targets:
      - debian12
`,
			want: `networkMaps[0] defaultTarget "debian13" must be included in targets`,
		},
		{
			name: "invalid cidr",
			content: `
targets:
  debian12:
    script: debian.ipxe
networkMaps:
  - network: invalid-cidr
    defaultTarget: debian12
    targets:
      - debian12
`,
			want: `networkMaps[0] network "invalid-cidr" is invalid`,
		},
		{
			name: "invalid hostname regex",
			content: `
targets:
  debian12:
    script: debian.ipxe
hostnameMaps:
  - hostname: '['
    defaultTarget: debian12
    targets:
      - debian12
`,
			want: `hostnameMaps[0] hostname "[" is invalid`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMappings(log.MakeLogger(io.Discard), writeMappingsFile(t, tt.content))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseMappingsReturnsMissingFileError(t *testing.T) {
	_, err := ParseMappings(log.MakeLogger(io.Discard), filepath.Join(t.TempDir(), "missing.yaml"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read mappings")
}

func writeMappingsFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mappings.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
