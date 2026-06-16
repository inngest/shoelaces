// Copyright 2018 ThousandEyes Inc.
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

package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplatesAndRenderTemplate(t *testing.T) {
	renderer := newTestRenderer(t)
	logger := log.MakeLogger(testLogWriter{})

	rendered, err := renderer.RenderTemplate(logger, "boot.ipxe", map[string]interface{}{
		"hostname": "default-host",
		"baseURL":  "127.0.0.1:8081",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "default default-host")
	assert.Contains(t, rendered, "127.0.0.1:8081")
	assert.ElementsMatch(t, []string{"hostname", "baseURL"}, renderer.ListVariables("boot.ipxe", defaultEnvironment))
}

func TestRenderTemplateUsesEnvironmentOverride(t *testing.T) {
	renderer := newTestRenderer(t)

	rendered, err := renderer.RenderTemplate(log.MakeLogger(testLogWriter{}), "boot.ipxe", map[string]interface{}{
		"hostname": "override-host",
		"baseURL":  "127.0.0.1:8081",
	}, "testing")

	require.NoError(t, err)
	assert.Contains(t, rendered, "override override-host")
	assert.NotContains(t, rendered, "default override-host")
	assert.ElementsMatch(t, []string{"hostname", "baseURL"}, renderer.ListVariables("boot.ipxe", "testing"))
}

func TestRenderTemplateFallsBackToDefaultEnvironment(t *testing.T) {
	renderer := newTestRenderer(t)

	rendered, err := renderer.RenderTemplate(log.MakeLogger(testLogWriter{}), "fallback.ipxe", map[string]interface{}{
		"hostname": "fallback-host",
	}, "testing")

	require.NoError(t, err)
	assert.Contains(t, rendered, "fallback fallback-host")
}

func TestRenderTemplateReturnsMissingVariableError(t *testing.T) {
	renderer := newTestRenderer(t)

	rendered, err := renderer.RenderTemplate(log.MakeLogger(testLogWriter{}), "boot.ipxe", map[string]interface{}{
		"hostname": "missing-base-url",
	}, "")

	assert.Empty(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing variables in request: baseURL")
}

func TestRenderTemplateRedactsSensitiveParamsInLogs(t *testing.T) {
	renderer := newTestRenderer(t)
	var logOutput bytes.Buffer

	rendered, err := renderer.RenderTemplate(log.MakeLogger(&logOutput), "boot.ipxe", map[string]interface{}{
		"hostname":              "secure-host",
		"baseURL":               "127.0.0.1:8081",
		"root_password_crypted": "hash",
		"bootstrap_token":       "token-value",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "secure-host")
	assert.NotContains(t, logOutput.String(), "hash")
	assert.NotContains(t, logOutput.String(), "token-value")
	assert.Contains(t, logOutput.String(), "root_password_crypted:[REDACTED]")
	assert.Contains(t, logOutput.String(), "bootstrap_token:[REDACTED]")
}

func TestListVariablesReturnsEmptyForUnknownTemplate(t *testing.T) {
	renderer := newTestRenderer(t)

	assert.Empty(t, renderer.ListVariables("missing.ipxe", defaultEnvironment))
	assert.Empty(t, renderer.ListVariables("boot.ipxe", "missing-env"))
}

func newTestRenderer(t *testing.T) *ShoelacesTemplates {
	t.Helper()

	dataDir := t.TempDir()
	envDir := filepath.Join(dataDir, "env_overrides", "testing")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	writeTemplate(t, filepath.Join(dataDir, "boot.ipxe.slc"), `{{define "boot.ipxe"}}#!ipxe
echo default {{.hostname}}
chain http://{{.baseURL}}/boot
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "fallback.ipxe.slc"), `{{define "fallback.ipxe"}}#!ipxe
echo fallback {{.hostname}}
{{end}}
`)
	writeTemplate(t, filepath.Join(envDir, "boot.ipxe.slc"), `{{define "boot.ipxe"}}#!ipxe
echo override {{.hostname}}
chain http://{{.baseURL}}/override
{{end}}
`)

	renderer := New()
	renderer.ParseTemplates(log.MakeLogger(testLogWriter{}), dataDir, "env_overrides", []string{"testing"}, ".slc")
	return renderer
}

func writeTemplate(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

type testLogWriter struct{}

func (testLogWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
