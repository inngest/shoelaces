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

package templates

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplatesAndRenderTemplate(t *testing.T) {
	renderer := newTestRenderer(t)

	rendered, err := renderer.RenderTemplate("boot.ipxe", map[string]any{
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

	rendered, err := renderer.RenderTemplate("boot.ipxe", map[string]interface{}{
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

	rendered, err := renderer.RenderTemplate("fallback.ipxe", map[string]interface{}{
		"hostname": "fallback-host",
	}, "testing")

	require.NoError(t, err)
	assert.Contains(t, rendered, "fallback fallback-host")
}

func TestRenderTemplateReturnsMissingVariableError(t *testing.T) {
	renderer := newTestRenderer(t)

	rendered, err := renderer.RenderTemplate("boot.ipxe", map[string]interface{}{
		"hostname": "missing-base-url",
	}, "")

	assert.Empty(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing variables in request: baseURL")
}

func TestListVariablesIncludesPartialDependencies(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "boot.ipxe.slc"), `{{define "boot.ipxe"}}#!ipxe
echo {{.hostname}}
{{template "boot/args" .}}
{{end}}
{{define "boot/args"}}args {{.partial_required}}{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)

	assert.ElementsMatch(t, []string{"hostname", "partial_required"}, renderer.ListVariables("boot.ipxe", ""))
}

func TestRenderTemplateReturnsPartialMissingVariableError(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "boot.ipxe.slc"), `{{define "boot.ipxe"}}#!ipxe
echo {{.hostname}}
{{template "boot/args" .}}
{{end}}
{{define "boot/args"}}args {{.partial_required}}{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)

	rendered, err := renderer.RenderTemplate("boot.ipxe", map[string]interface{}{
		"hostname": "partial-host",
	}, "")

	assert.Empty(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing variables in request: partial_required")
}

func TestListVariablesPartialDependencyCases(t *testing.T) {
	for _, tt := range []struct {
		name      string
		files     map[string]string
		template  string
		env       string
		expected  []string
		notExpect []string
	}{
		{
			name: "nested partial dependency",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{.hostname}}{{template "boot/args" .}}{{end}}
{{define "boot/args"}}{{template "boot/deep" .}}{{end}}
{{define "boot/deep"}}{{.deep_required}}{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"hostname", "deep_required"},
		},
		{
			name: "shared partial dependencies are deduped",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{template "boot/first" .}}{{template "boot/second" .}}{{end}}
{{define "boot/first"}}{{.hostname}}{{.shared_required}}{{end}}
{{define "boot/second"}}{{.hostname}}{{.shared_required}}{{.second_required}}{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"hostname", "shared_required", "second_required"},
		},
		{
			name: "cyclic partial dependencies terminate",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{.hostname}}{{template "boot/a" .}}{{end}}
{{define "boot/a"}}{{.from_a}}{{template "boot/b" .}}{{end}}
{{define "boot/b"}}{{.from_b}}{{template "boot/a" .}}{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"hostname", "from_a", "from_b"},
		},
		{
			name: "template call argument variables are collected",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{template "boot/arg" .hostname}}{{template "boot/chain" .network.interface}}{{end}}
{{define "boot/arg"}}literal{{end}}
{{define "boot/chain"}}literal{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"hostname", "network.interface"},
		},
		{
			name: "conditional branch variables are collected",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{if .install_enabled}}{{.enabled_value}}{{else}}{{.disabled_value}}{{end}}{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"install_enabled", "enabled_value", "disabled_value"},
		},
		{
			name: "range and with variables are collected",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{range .items}}{{.name}}{{else}}{{.empty_items}}{{end}}{{with .metadata}}{{.owner}}{{else}}{{.missing_metadata}}{{end}}{{end}}
`,
			},
			template: "boot.ipxe",
			expected: []string{"items", "name", "empty_items", "metadata", "owner", "missing_metadata"},
		},
		{
			name: "environment override inherits default partial",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{template "boot/args" .}}{{end}}
{{define "boot/args"}}{{.default_partial_required}}{{end}}
`,
				filepath.Join("env_overrides", "testing", "boot.ipxe.slc"): `{{define "boot.ipxe"}}{{.override_required}}{{template "boot/args" .}}{{end}}
`,
			},
			template: "boot.ipxe",
			env:      "testing",
			expected: []string{"override_required", "default_partial_required"},
		},
		{
			name: "environment partial override replaces default partial vars",
			files: map[string]string{
				"boot.ipxe.slc": `{{define "boot.ipxe"}}{{.hostname}}{{template "boot/args" .}}{{end}}
{{define "boot/args"}}{{.default_partial_required}}{{end}}
`,
				filepath.Join("env_overrides", "testing", "boot_args.slc"): `{{define "boot/args"}}{{.override_partial_required}}{{end}}
`,
			},
			template:  "boot.ipxe",
			env:       "testing",
			expected:  []string{"hostname", "override_partial_required"},
			notExpect: []string{"default_partial_required"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			for path, content := range tt.files {
				writeTemplate(t, filepath.Join(dataDir, path), content)
			}

			renderer := New(log.MakeLogger(testLogWriter{}))
			renderer.ParseTemplates(dataDir, "env_overrides", []string{"testing"}, ".slc")
			variables := renderer.ListVariables(tt.template, tt.env)

			assert.ElementsMatch(t, tt.expected, variables)
			for _, unexpected := range tt.notExpect {
				assert.NotContains(t, variables, unexpected)
			}
		})
	}
}

func TestRenderTemplateReturnsNestedPartialMissingVariableErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		template    string
		params      map[string]interface{}
		expectedErr string
	}{
		{
			name: "deep partial variable",
			template: `{{define "boot.ipxe"}}{{.hostname}}{{template "boot/args" .}}{{end}}
{{define "boot/args"}}{{template "boot/deep" .}}{{end}}
{{define "boot/deep"}}{{.deep_required}}{{end}}
`,
			params:      map[string]interface{}{"hostname": "missing-deep"},
			expectedErr: "deep_required",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			writeTemplate(t, filepath.Join(dataDir, "boot.ipxe.slc"), tt.template)
			renderer := newEmbeddedFallbackRenderer(t, dataDir)

			rendered, err := renderer.RenderTemplate("boot.ipxe", tt.params, "")

			assert.Empty(t, rendered)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Missing variables in request: "+tt.expectedErr)
		})
	}
}

func TestRenderTemplateRedactsSensitiveParamsInLogs(t *testing.T) {
	var logOutput bytes.Buffer
	renderer := newTestRendererWithLogger(t, log.MakeLogger(&logOutput))

	rendered, err := renderer.RenderTemplate("boot.ipxe", map[string]interface{}{
		"hostname":              "secure-host",
		"baseURL":               "127.0.0.1:8081",
		"root_password_crypted": "root-password-value",
		"bootstrap_token":       "token-value",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "secure-host")
	assert.NotContains(t, logOutput.String(), "root-password-value")
	assert.NotContains(t, logOutput.String(), "token-value")
	logParams := loggedTemplateParams(t, logOutput.String())
	assert.Equal(t, "[REDACTED]", logParams["root_password_crypted"])
	assert.Equal(t, "[REDACTED]", logParams["bootstrap_token"])
}

func TestRenderTemplateRedactsStructuredUsersInLogs(t *testing.T) {
	var logOutput bytes.Buffer
	renderer := newTestRendererWithLogger(t, log.MakeLogger(&logOutput))
	params := mappings.ParamsWithUsers(map[string]any{
		"hostname": "secure-host",
		"baseURL":  "127.0.0.1:8081",
	}, map[string]mappings.ResolvedUser{
		"infra": {
			Name:              "infra",
			PasswordCrypted:   "secret-hash",
			SSHAuthorizedKeys: []string{"ssh-ed25519 AAAA secret-key"},
		},
	})

	rendered, err := renderer.RenderTemplate("boot.ipxe", params, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "secure-host")
	assert.NotContains(t, logOutput.String(), "secret-hash")
	assert.NotContains(t, logOutput.String(), "ssh-ed25519 AAAA secret-key")
	logParams := loggedTemplateParams(t, logOutput.String())
	users := logParams["users"].(map[string]any)
	byName := users["ByName"].(map[string]any)
	infra := byName["infra"].(map[string]any)
	assert.Equal(t, "[REDACTED]", infra["PasswordCrypted"])
	assert.Equal(t, "[REDACTED]", infra["SSHAuthorizedKeys"])
}

func TestRenderTemplateRedactsProvisioningInLogs(t *testing.T) {
	var logOutput bytes.Buffer
	renderer := newTestRendererWithLogger(t, log.MakeLogger(&logOutput))
	enabled := true
	params := mappings.ParamsWithProvisioning(map[string]any{
		"hostname": "secure-host",
		"baseURL":  "127.0.0.1:8081",
	}, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    &enabled,
				Passphrase: "raw-luks-passphrase",
			},
		},
		Installer: mappings.InstallerConfig{
			ConfigParams: map[string]any{
				"bootstrap_token": "secret-token",
			},
		},
	})

	rendered, err := renderer.RenderTemplate("boot.ipxe", params, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "secure-host")
	assert.NotContains(t, logOutput.String(), "secret-token")
	assert.NotContains(t, logOutput.String(), "raw-luks-passphrase")
	logParams := loggedTemplateParams(t, logOutput.String())
	assert.Equal(t, "[REDACTED]", logParams["bootstrap_token"])
	provisioning := logParams["provisioning"].(map[string]any)
	storage := provisioning["Storage"].(map[string]any)
	encryption := storage["Encryption"].(map[string]any)
	assert.Equal(t, "[REDACTED]", encryption["Passphrase"])
	installer := provisioning["Installer"].(map[string]any)
	configParams := installer["ConfigParams"].(map[string]any)
	assert.Equal(t, "[REDACTED]", configParams["bootstrap_token"])
}

func TestRenderTemplateRedactsEnvironmentResolvedLUKSPassphraseInLogs(t *testing.T) {
	var logOutput bytes.Buffer
	renderer := newTestRendererWithLogger(t, log.MakeLogger(&logOutput))
	enabled := true
	params := mappings.ParamsWithProvisioning(map[string]any{
		"hostname": "secure-host",
		"baseURL":  "127.0.0.1:8081",
	}, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    &enabled,
				Passphrase: "env-resolved-luks-passphrase",
			},
		},
	})

	rendered, err := renderer.RenderTemplate("boot.ipxe", params, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "secure-host")
	assert.NotContains(t, logOutput.String(), "env-resolved-luks-passphrase")
	logParams := loggedTemplateParams(t, logOutput.String())
	provisioning := logParams["provisioning"].(map[string]any)
	storage := provisioning["Storage"].(map[string]any)
	encryption := storage["Encryption"].(map[string]any)
	assert.Equal(t, "[REDACTED]", encryption["Passphrase"])
}

func TestListVariablesReturnsEmptyForUnknownTemplate(t *testing.T) {
	renderer := newTestRenderer(t)

	assert.Empty(t, renderer.ListVariables("missing.ipxe", defaultEnvironment))
	assert.Empty(t, renderer.ListVariables("boot.ipxe", "missing-env"))
}

func TestRenderTemplateUsesEmbeddedProvisioningFallback(t *testing.T) {
	renderer := newEmbeddedFallbackRenderer(t, t.TempDir())

	rendered, err := renderer.RenderTemplate("debian.ipxe", map[string]interface{}{
		"baseURL":      "127.0.0.1:8081",
		"encrypt_home": false,
		"hostname":     "embedded-host",
		"release":      "bookworm",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "Debian bookworm netboot")
	assert.Contains(t, rendered, "hostname=embedded-host")
	assert.Contains(t, rendered, "preseed/url=http://127.0.0.1:8081/configs/preseed/debian?encrypt_home=false")
}

func TestListVariablesIncludesEmbeddedProvisioningPartialDependencies(t *testing.T) {
	renderer := newEmbeddedFallbackRenderer(t, t.TempDir())

	assert.Contains(t, renderer.ListVariables("debian.ipxe", ""), "encrypt_home")
	assert.Contains(t, renderer.ListVariables("preseed/debian", ""), "encrypt_home")
}

func TestDiskTemplateOverridesEmbeddedProvisioningTemplate(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "ipxe", "debian.ipxe.slc"), `{{define "debian.ipxe"}}#!ipxe
echo disk override {{.hostname}}
{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)

	rendered, err := renderer.RenderTemplate("debian.ipxe", map[string]interface{}{
		"hostname": "disk-host",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "disk override disk-host")
	assert.NotContains(t, rendered, "Debian bookworm netboot")
}

func TestRemovedDeclarativeHookPartialsDoNotAffectEmbeddedProvisioningTemplates(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "preseed", "debian", "late_command.slc"), `{{define "preseed/debian/late_command" -}}
d-i preseed/late_command string echo removed preseed hook for {{.hostname}}
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "ipxe", "linux_args.slc"), `{{define "ipxe/linux_args" -}}
removed-linux-args
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "ipxe", "debian", "preseed_url.slc"), `{{define "ipxe/debian/preseed_url" -}}
http://removed.example/preseed
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "kickstart", "centos", "post.slc"), `{{define "kickstart/centos/post" -}}
%post
echo removed kickstart hook
%end
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "cloud-config", "units.slc"), `{{define "cloudconfig/coreos/units" -}}
removed-cloudconfig-units: true
{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)

	params := map[string]interface{}{
		"baseURL":      "127.0.0.1:8081",
		"encrypt_home": false,
		"hostname":     "partial-host",
		"release":      "bookworm",
	}

	preseed, err := renderer.RenderTemplate("preseed/debian", params, "")
	require.NoError(t, err)
	assert.NotContains(t, preseed, "removed preseed hook")
	assert.NotContains(t, preseed, "d-i preseed/late_command string true")

	ipxe, err := renderer.RenderTemplate("debian.ipxe", params, "")
	require.NoError(t, err)
	assert.NotContains(t, ipxe, "removed-linux-args")
	assert.NotContains(t, ipxe, "removed.example")
	assert.Contains(t, ipxe, "preseed/url=http://127.0.0.1:8081/configs/preseed/debian?encrypt_home=false")

	kickstart, err := renderer.RenderTemplate("centos.ks", params, "")
	require.NoError(t, err)
	assert.NotContains(t, kickstart, "removed kickstart hook")

	cloudConfig, err := renderer.RenderTemplate("cloudconfig-coreos", params, "")
	require.NoError(t, err)
	assert.NotContains(t, cloudConfig, "removed-cloudconfig-units")
}

func TestRenderTemplateAppliesInstallerExtraWithDefaultedParams(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "provisioning", "extra.slc"), `{{define "provisioning/extra" -}}
d-i preseed/late_command string echo extra {{.hostname}} {{.storage_disk}}
{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "127.0.0.1:8081",
		"hostname": "extra-host",
	}, nil, mappings.ProvisioningConfig{
		Installer: mappings.InstallerConfig{
			ExtraTemplate: "provisioning/extra",
		},
	})

	rendered, err := renderer.RenderTemplate("preseed/debian", params, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "d-i preseed/late_command string echo extra extra-host /dev/nvme0n1")
	assert.NotContains(t, rendered, "<no value>")
}

func TestRenderTemplateAppliesBooleanInstallerEncryptionParam(t *testing.T) {
	renderer := newEmbeddedFallbackRenderer(t, t.TempDir())
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "127.0.0.1:8081",
		"hostname": "encrypted-host",
	}, nil, mappings.ProvisioningConfig{
		Installer: mappings.InstallerConfig{
			ConfigParams: map[string]any{
				"storage_encryption_enabled":    true,
				"storage_encryption_passphrase": "luks-passphrase",
			},
		},
	})

	rendered, err := renderer.RenderTemplate("preseed/debian", params, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "d-i partman-auto/method string crypto")
	assert.Contains(t, rendered, "d-i partman-crypto/passphrase password luks-passphrase")
	assert.NotContains(t, rendered, "<no value>")
}

func TestEmbeddedUserRenderingDoesNotRequireDiskUserPartials(t *testing.T) {
	renderer := newEmbeddedFallbackRenderer(t, t.TempDir())
	params := mappings.ParamsWithUsers(map[string]interface{}{
		"baseURL":      "127.0.0.1:8081",
		"encrypt_home": false,
		"hostname":     "structured-host",
	}, map[string]mappings.ResolvedUser{
		"infra": {
			Name:            "infra",
			Primary:         true,
			FullName:        "Infrastructure User",
			PasswordCrypted: "$6$infra",
		},
	})

	preseed, err := renderer.RenderTemplate("preseed/debian", params, "")
	require.NoError(t, err)
	assert.Contains(t, preseed, "d-i passwd/user-fullname string Infrastructure User")
	assert.Contains(t, preseed, "d-i passwd/username string infra")

	cloudConfig, err := renderer.RenderTemplate("cloudconfig-coreos", params, "")
	require.NoError(t, err)
	assert.Contains(t, cloudConfig, "  - name: infra")
	assert.Contains(t, cloudConfig, `    passwd: "$6$infra"`)
}

func TestDiskUserPartialsDoNotOverrideEmbeddedUserRendering(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "preseed", "common", "users.slc"), `{{define "preseed/common/users" -}}
d-i passwd/username string disk-partial-user
{{end}}
`)
	writeTemplate(t, filepath.Join(dataDir, "cloud-config", "users.slc"), `{{define "cloudconfig/coreos/users" -}}
users:
  - name: disk-partial-user
{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)
	params := mappings.ParamsWithUsers(map[string]interface{}{
		"baseURL":      "127.0.0.1:8081",
		"encrypt_home": false,
		"hostname":     "structured-host",
	}, map[string]mappings.ResolvedUser{
		"infra": {
			Name:    "infra",
			Primary: true,
		},
	})

	preseed, err := renderer.RenderTemplate("preseed/debian", params, "")
	require.NoError(t, err)
	assert.Contains(t, preseed, "d-i passwd/username string infra")
	assert.NotContains(t, preseed, "disk-partial-user")

	cloudConfig, err := renderer.RenderTemplate("cloudconfig-coreos", params, "")
	require.NoError(t, err)
	assert.Contains(t, cloudConfig, "  - name: infra")
	assert.NotContains(t, cloudConfig, "disk-partial-user")
}

func TestDiskFullTemplateOverrideCanReplaceEmbeddedUserRendering(t *testing.T) {
	dataDir := t.TempDir()
	writeTemplate(t, filepath.Join(dataDir, "preseed", "debian.preseed.slc"), `{{define "preseed/debian" -}}
d-i passwd/username string full-template-user
d-i netcfg/get_hostname string {{ .hostname }}
{{end}}
`)
	renderer := newEmbeddedFallbackRenderer(t, dataDir)

	rendered, err := renderer.RenderTemplate("preseed/debian", map[string]interface{}{
		"hostname": "override-host",
	}, "")

	require.NoError(t, err)
	assert.Contains(t, rendered, "d-i passwd/username string full-template-user")
	assert.Contains(t, rendered, "d-i netcfg/get_hostname string override-host")
	assert.NotContains(t, rendered, "d-i passwd/username string debian")
}

func newTestRenderer(t *testing.T) *ShoelacesTemplates {
	t.Helper()
	return newTestRendererWithLogger(t, log.MakeLogger(testLogWriter{}))
}

func newTestRendererWithLogger(t *testing.T, logger log.Logger) *ShoelacesTemplates {
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

	renderer := New(logger)
	renderer.ParseTemplates(dataDir, "env_overrides", []string{"testing"}, ".slc")
	return renderer
}

func writeTemplate(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func newEmbeddedFallbackRenderer(t *testing.T, dataDir string) *ShoelacesTemplates {
	t.Helper()

	renderer := New(log.MakeLogger(testLogWriter{}))
	renderer.ParseTemplates(dataDir, "env_overrides", nil, ".slc")
	return renderer
}

func loggedTemplateParams(t *testing.T, logs string) map[string]any {
	t.Helper()

	const key = "parameters="
	start := strings.Index(logs, key)
	require.NotEqual(t, -1, start, "template log should include parameters attribute:\n%s", logs)
	raw := logs[start+len(key):]
	if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
		raw = raw[:newline]
	}

	var jsonText string
	if strings.HasPrefix(raw, `"`) {
		var err error
		jsonText, err = strconv.Unquote(raw)
		require.NoError(t, err)
	} else {
		jsonText = raw
	}

	var params map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonText), &params))
	return params
}

type testLogWriter struct{}

func (testLogWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
