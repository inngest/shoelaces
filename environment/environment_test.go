// Copyright 2018 ThousandEyes Inc.
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

package environment

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEnvironment(t *testing.T) {
	env := defaultEnvironment()
	assert.Empty(t, env.BaseURL)
	assert.Empty(t, env.HostnameMaps)
	assert.Empty(t, env.NetworkMaps)
	assert.Equal(t, []string{"baseURL"}, env.ParamsBlacklist)
}

func TestInitScript(t *testing.T) {
	configMappings := &mappings.Mappings{
		Defaults: mappings.DefaultsMap{Params: map[string]any{"one": "default", "shared": "default"}},
		Targets: map[string]mappings.Target{
			"debian12": {
				Script:      "testscript",
				Environment: "testing",
				Params:      map[string]any{"one": "target", "two": "target"},
			},
		},
	}

	mappingScript, err := initScriptForTarget(configMappings, "debian12", map[string]any{"two": "mapping"})

	require.NoError(t, err)
	assert.Equal(t, "testscript", mappingScript.Name)
	assert.Equal(t, "testing", mappingScript.Environment)
	assert.Equal(t, "target", mappingScript.Params["one"])
	assert.Equal(t, "mapping", mappingScript.Params["two"])
	assert.Equal(t, "default", mappingScript.Params["shared"])
}

func TestInitEnvOverridesReturnsOnlyDirectories(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "env_overrides", "testing"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "env_overrides", "staging"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "env_overrides", "README"), []byte("ignored"), 0o644))
	env := defaultEnvironment()
	env.DataDir = dataDir
	env.EnvDir = "env_overrides"

	envs := env.initEnvOverrides()

	assert.ElementsMatch(t, []string{"testing", "staging"}, envs)
}

func TestInitEnvOverridesReturnsEmptyWhenDirectoryIsMissing(t *testing.T) {
	env := defaultEnvironment()
	env.DataDir = t.TempDir()
	env.EnvDir = "env_overrides"

	assert.Empty(t, env.initEnvOverrides())
}

func TestInitStaticTemplatesUsesEmbeddedTemplates(t *testing.T) {
	env := defaultEnvironment()
	env.UIDir = filepath.Join(t.TempDir(), "missing-web")

	env.initStaticTemplates()

	for _, name := range []string{"header", "index", "events", "mappings", "footer"} {
		assert.NotNil(t, env.StaticTemplates.Lookup(name), "template %q should be parsed", name)
	}
}

func TestInitStaticTemplatesUsesUIDirOverrideWhenSet(t *testing.T) {
	uiDir := writeTestUITemplates(t)
	env := defaultEnvironment()
	env.UIDir = uiDir
	env.UIOverrideDirSet = true

	env.initStaticTemplates()

	var rendered bytes.Buffer
	require.NoError(t, env.StaticTemplates.ExecuteTemplate(&rendered, "index", nil))
	assert.Equal(t, "disk index", rendered.String())
}

func TestInitMappingsLoadsNetworkAndHostnameMaps(t *testing.T) {
	env := defaultEnvironment()
	env.Logger = log.MakeLogger(io.Discard)
	mappingsPath := filepath.Join(t.TempDir(), "mappings.yaml")
	require.NoError(t, os.WriteFile(mappingsPath, []byte(`
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
    params:
      role: network
  - network: 2001:db8::/64
    defaultTarget: debian13
    targets:
      - debian13
    params:
      role: ipv6-network
hostnameMaps:
  - hostname: '^host-\d+$'
    defaultTarget: ubuntu2404
    targets:
      - ubuntu2404
    params:
      role: host
targets:
  debian12:
    script: network.ipxe
    environment: testing
    params:
      release: bookworm
  debian13:
    script: network.ipxe
    params:
      release: trixie
  ubuntu2404:
    script: host.ipxe
`), 0o644))

	require.NoError(t, env.initMappings(mappingsPath))

	require.NotNil(t, env.MappingResolver)
	require.Len(t, env.NetworkMaps, 2)
	assert.True(t, env.NetworkMaps[0].Network.Contains(mustParseIP(t, "192.0.2.10")))
	assert.Equal(t, "network.ipxe", env.NetworkMaps[0].Script.Name)
	assert.Equal(t, "testing", env.NetworkMaps[0].Script.Environment)
	assert.Equal(t, "network", env.NetworkMaps[0].Script.Params["role"])
	assert.Equal(t, "bookworm", env.NetworkMaps[0].Script.Params["release"])
	assert.True(t, env.NetworkMaps[1].Network.Contains(mustParseIP(t, "2001:db8::1")))
	assert.Equal(t, "trixie", env.NetworkMaps[1].Script.Params["release"])

	require.Len(t, env.HostnameMaps, 1)
	assert.True(t, env.HostnameMaps[0].Hostname.MatchString("host-123"))
	assert.Equal(t, "host.ipxe", env.HostnameMaps[0].Script.Name)
	assert.Equal(t, "host", env.HostnameMaps[0].Script.Params["role"])
}

func TestInitMappingsRendersMappingsTemplateWithNonStringDefaultTargetParams(t *testing.T) {
	env := defaultEnvironment()
	env.Logger = log.MakeLogger(io.Discard)
	env.initStaticTemplates()
	mappingsPath := filepath.Join(t.TempDir(), "mappings.yaml")
	require.NoError(t, os.WriteFile(mappingsPath, []byte(`
defaults:
  params:
    install_retries: 3
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
    params:
      install_token:
        env: INSTALL_TOKEN
hostnameMaps:
  - hostname: '^host-\d+$'
    defaultTarget: debian12
    targets:
      - debian12
targets:
  debian12:
    script: network.ipxe
    params:
      secure_boot: true
`), 0o644))

	require.NoError(t, env.initMappings(mappingsPath))

	tplVars := struct {
		HostnameMaps *[]mappings.HostnameMap
		NetworkMaps  *[]mappings.NetworkMap
	}{
		&env.HostnameMaps,
		&env.NetworkMaps,
	}
	var rendered bytes.Buffer
	require.NoError(t, env.StaticTemplates.ExecuteTemplate(&rendered, "mappings", tplVars))
	assert.Contains(t, rendered.String(), "network.ipxe")
	assert.Contains(t, rendered.String(), "install_retries: 3")
	assert.Contains(t, rendered.String(), "secure_boot: true")
	assert.Contains(t, rendered.String(), "install_token: map[env:INSTALL_TOKEN]")
}

func TestInitMappingsReturnsInvalidCIDRError(t *testing.T) {
	env := defaultEnvironment()
	env.Logger = log.MakeLogger(io.Discard)
	mappingsPath := filepath.Join(t.TempDir(), "mappings.yaml")
	require.NoError(t, os.WriteFile(mappingsPath, []byte(`
networkMaps:
  - network: invalid-cidr
    defaultTarget: debian12
    targets:
      - debian12
targets:
  debian12:
    script: network.ipxe
`), 0o644))

	assert.Error(t, env.initMappings(mappingsPath))
}

func TestInitMappingsReturnsInvalidHostnameRegexError(t *testing.T) {
	env := defaultEnvironment()
	env.Logger = log.MakeLogger(io.Discard)
	mappingsPath := filepath.Join(t.TempDir(), "mappings.yaml")
	require.NoError(t, os.WriteFile(mappingsPath, []byte(`
hostnameMaps:
  - hostname: '['
    defaultTarget: debian12
    targets:
      - debian12
targets:
  debian12:
    script: host.ipxe
`), 0o644))

	assert.Error(t, env.initMappings(mappingsPath))
}

func mustParseIP(t *testing.T, ip string) net.IP {
	t.Helper()

	parsed := net.ParseIP(ip)
	require.NotNil(t, parsed)
	return parsed
}

func writeTestUITemplates(t *testing.T) string {
	t.Helper()

	uiDir := t.TempDir()
	templatesDir := filepath.Join(uiDir, "templates/html")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	templates := map[string]string{
		"header.html":   `{{ define "header" }}disk header{{ end }}`,
		"index.html":    `{{ define "index" }}disk index{{ end }}`,
		"events.html":   `{{ define "events" }}disk events{{ end }}`,
		"mappings.html": `{{ define "mappings" }}disk mappings{{ end }}`,
		"footer.html":   `{{ define "footer" }}disk footer{{ end }}`,
	}
	for name, content := range templates {
		require.NoError(t, os.WriteFile(filepath.Join(templatesDir, name), []byte(content), 0o644))
	}

	return uiDir
}
