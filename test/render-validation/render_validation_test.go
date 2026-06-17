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

package rendervalidation

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var defaultRenderParams = map[string]interface{}{
	"baseURL":      "shoelaces.example.test:8081",
	"encrypt_home": false,
	"hostname":     "render-validation-host",
	"linuxargs":    "console=ttyS0",
	"release":      "bookworm",
}

var siteOnlyMarkers = []string{
	"git@github.com:inngest/ansible.git",
	"/static/firstboot",
	"/static/authorized_keys",
	"/static/id_ansible",
	"ANSIBLE_REPO_URL",
	"ANSIBLE_PLAYBOOK",
	"id_ansible",
	"ssh-rsa fake-key",
	"$6$6C2rNs/iVblJ.PbR$",
}

func TestEmbeddedProvisioningTemplatesRenderWithoutMissingValues(t *testing.T) {
	renderer := newRenderer(t)
	for _, templateName := range []string{
		"cloudconfig-coreos",
		"centos.ipxe",
		"centos.ks",
		"coreos.ipxe",
		"debian.ipxe",
		"preseed/debian",
		"preseed/ubuntu-minimal",
		"ubuntu-minimal.ipxe",
	} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, defaultRenderParams)

			assert.NotContains(t, rendered, "<no value>")
			assert.NotContains(t, rendered, "{{")
			assert.NotContains(t, rendered, "}}")
			for _, marker := range siteOnlyMarkers {
				assert.NotContains(t, rendered, marker)
			}
		})
	}
}

func TestRenderedIPXEScriptsHaveRequiredShape(t *testing.T) {
	renderer := newRenderer(t)
	for _, templateName := range []string{
		"centos.ipxe",
		"coreos.ipxe",
		"debian.ipxe",
		"ubuntu-minimal.ipxe",
	} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, defaultRenderParams)

			assert.True(t, strings.HasPrefix(rendered, "#!ipxe\n"))
			assert.Contains(t, rendered, "\nkernel ")
			assert.Contains(t, rendered, "\ninitrd ")
			assert.Contains(t, rendered, "\nboot")
			assertNoDanglingLineContinuations(t, rendered)
		})
	}
}

func TestRenderedPreseedsHaveRequiredShape(t *testing.T) {
	renderer := newRenderer(t)
	for _, templateName := range []string{
		"preseed/debian",
		"preseed/ubuntu-minimal",
	} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, defaultRenderParams)

			assert.Contains(t, rendered, "d-i auto-install/enable boolean true")
			assert.Contains(t, rendered, "d-i debian-installer/locale string en_US.UTF-8")
			assert.Contains(t, rendered, "d-i passwd/user-password-crypted password !")
			assert.Contains(t, rendered, "d-i finish-install/reboot_in_progress note")
		})
	}
}

func TestRenderedDebianPreseedKeepsGenericNoOpLateCommand(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", defaultRenderParams)

	assert.Contains(t, rendered, "d-i preseed/late_command string true")
	assert.NotContains(t, rendered, "firstboot")
	assert.NotContains(t, rendered, "ansible")
}

func TestRenderedKickstartHasRequiredShape(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "centos.ks", defaultRenderParams)

	assert.Contains(t, rendered, "cmdline")
	assert.Contains(t, rendered, `url  --url="http://mirror.netcologne.de/centos/bookworm/os/x86_64"`)
	assert.Contains(t, rendered, "network --bootproto dhcp --hostname render-validation-host")
	assert.Contains(t, rendered, "rootpw --lock")
	assert.Contains(t, rendered, "%packages")
	assert.Contains(t, rendered, "@core")
	assert.Contains(t, rendered, "%post")
	assert.Contains(t, rendered, "true")
	assert.Contains(t, rendered, "reboot")
}

func TestRenderedCloudConfigParsesAsYAML(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "cloudconfig-coreos", defaultRenderParams)
	withoutHeader := strings.TrimPrefix(rendered, "#cloud-config\n")
	var parsed map[string]interface{}

	require.NoError(t, yaml.Unmarshal([]byte(withoutHeader), &parsed))
	assert.Equal(t, "render-validation-host", parsed["hostname"])
	assert.Contains(t, parsed, "users")
	assert.Contains(t, parsed, "ssh_authorized_keys")
	assert.Contains(t, parsed, "coreos")
}

func TestRenderedCloudConfigPassesCloudInitSchemaWhenAvailable(t *testing.T) {
	cloudInit, err := exec.LookPath("cloud-init")
	if err != nil {
		t.Skip("cloud-init is not installed")
	}
	configPath := writeRenderedFile(t, "cloud-config.yaml", renderTemplate(t, newRenderer(t), "cloudconfig-coreos", defaultRenderParams))

	output, err := exec.Command(cloudInit, "schema", "--config-file", configPath).CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestRenderedKickstartPassesKsvalidatorWhenAvailable(t *testing.T) {
	ksvalidator, err := exec.LookPath("ksvalidator")
	if err != nil {
		t.Skip("ksvalidator is not installed")
	}
	kickstartPath := writeRenderedFile(t, "centos.ks", renderTemplate(t, newRenderer(t), "centos.ks", defaultRenderParams))

	output, err := exec.Command(ksvalidator, kickstartPath).CombinedOutput()
	require.NoError(t, err, string(output))
}

func newRenderer(t *testing.T) *templates.ShoelacesTemplates {
	t.Helper()

	renderer := templates.New()
	renderer.ParseTemplates(log.MakeLogger(io.Discard), t.TempDir(), "env_overrides", nil, ".slc")
	return renderer
}

func renderTemplate(t *testing.T, renderer *templates.ShoelacesTemplates, name string, params map[string]interface{}) string {
	t.Helper()

	rendered, err := renderer.RenderTemplate(log.MakeLogger(io.Discard), name, params, "")
	require.NoError(t, err)
	return rendered
}

func assertNoDanglingLineContinuations(t *testing.T, rendered string) {
	t.Helper()

	lines := strings.Split(rendered, "\n")
	for i, line := range lines[:len(lines)-1] {
		if !strings.HasSuffix(strings.TrimRight(line, " \t"), "\\") {
			continue
		}
		next := strings.TrimSpace(lines[i+1])
		assert.NotEmpty(t, next, "line %d has a dangling continuation", i+1)
	}
}

func writeRenderedFile(t *testing.T, name string, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
