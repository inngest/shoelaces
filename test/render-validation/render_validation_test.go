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

const requireProvisioningValidatorsEnv = "SHOELACES_REQUIRE_PROVISIONING_VALIDATORS"

var defaultRenderParams = map[string]interface{}{
	"baseURL":      "shoelaces.example.test:8081",
	"encrypt_home": false,
	"hostname":     "render-validation-host",
	"linuxargs":    "console=ttyS0",
	"release":      "bookworm",
}

var kickstartRenderParams = paramsWith(defaultRenderParams, "release", "8")

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
		"preseed/storage",
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
			commands := assertValidIPXEScript(t, rendered)
			if templateName == "debian.ipxe" {
				assert.Contains(t, rendered, "preseed/url=http://shoelaces.example.test:8081/configs/preseed/debian?encrypt_home=false \\")
				assert.Contains(t, findIPXECommand(t, commands, "kernel"), "preseed/url=http://shoelaces.example.test:8081/configs/preseed/debian?encrypt_home=false")
			}
		})
	}
}

func TestRenderedPreseedsHaveRequiredShape(t *testing.T) {
	renderer := newRenderer(t)
	for _, templateName := range []string{
		"preseed/debian",
		"preseed/storage",
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

func TestRenderedPreseedsPassDebconfSetSelectionsWhenAvailable(t *testing.T) {
	debconfSetSelections := validatorPath(t, "debconf-set-selections")
	renderer := newRenderer(t)

	for _, templateName := range []string{
		"preseed/debian",
		"preseed/storage",
		"preseed/ubuntu-minimal",
	} {
		t.Run(templateName, func(t *testing.T) {
			preseedPath := writeRenderedFile(t, "preseed.cfg", renderTemplate(t, renderer, templateName, defaultRenderParams))

			output, err := exec.Command(debconfSetSelections, "--checkonly", preseedPath).CombinedOutput()
			require.NoError(t, err, string(output))
		})
	}
}

func TestRenderedPreseedsApplyInstallUserParams(t *testing.T) {
	renderer := newRenderer(t)
	params := paramsWith(defaultRenderParams, "install_username", "alice")
	params = paramsWith(params, "install_user_fullname", "Alice Example")
	params = paramsWith(params, "install_user_password_crypted", "$6$rounds=4096$testsalt$testhash")

	for _, templateName := range []string{
		"preseed/debian",
		"preseed/storage",
		"preseed/ubuntu-minimal",
	} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, params)

			assert.Contains(t, rendered, "d-i passwd/user-fullname string Alice Example")
			assert.Contains(t, rendered, "d-i passwd/username string alice")
			assert.Contains(t, rendered, "d-i passwd/user-password-crypted password $6$rounds=4096$testsalt$testhash")
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
	rendered := renderTemplate(t, newRenderer(t), "centos.ks", kickstartRenderParams)

	assert.Contains(t, rendered, "cmdline")
	assert.Contains(t, rendered, `url  --url="http://mirror.netcologne.de/centos/8/os/x86_64"`)
	assert.Contains(t, rendered, "network --bootproto dhcp --hostname render-validation-host")
	assert.Contains(t, rendered, "rootpw --lock")
	assert.Contains(t, rendered, "authselect --useshadow --passalgo=sha512 --enablefingerprint")
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
	assert.Equal(t, false, parsed["preserve_hostname"])
	assert.NotContains(t, parsed, "coreos")
}

func TestRenderedCloudConfigPassesCloudInitSchemaWhenAvailable(t *testing.T) {
	cloudInit := validatorPath(t, "cloud-init")
	configPath := writeRenderedFile(t, "cloud-config.yaml", renderTemplate(t, newRenderer(t), "cloudconfig-coreos", defaultRenderParams))

	output, err := exec.Command(cloudInit, "schema", "--config-file", configPath).CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestRenderedKickstartPassesKsvalidatorWhenAvailable(t *testing.T) {
	ksvalidator := validatorPath(t, "ksvalidator")
	kickstartPath := writeRenderedFile(t, "centos.ks", renderTemplate(t, newRenderer(t), "centos.ks", kickstartRenderParams))

	output, err := exec.Command(ksvalidator, "--version", "RHEL8", kickstartPath).CombinedOutput()
	require.NoError(t, err, string(output))
}

func validatorPath(t *testing.T, name string) string {
	t.Helper()

	if os.Getenv(requireProvisioningValidatorsEnv) != "1" {
		t.Skipf("%s validation requires %s=1", name, requireProvisioningValidatorsEnv)
	}
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}
	t.Fatalf("%s is required when %s=1", name, requireProvisioningValidatorsEnv)
	return ""
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

func assertValidIPXEScript(t *testing.T, rendered string) []string {
	t.Helper()

	allowedCommands := map[string]bool{
		"boot":    true,
		"chain":   true,
		"echo":    true,
		"imgfree": true,
		"initrd":  true,
		"kernel":  true,
		"set":     true,
	}

	var commands []string
	var continued string
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)
		if lineNumber == 1 {
			assert.Equal(t, "#!ipxe", trimmed)
			continue
		}
		if trimmed == "\\" {
			assert.Failf(t, "standalone iPXE continuation", "line %d contains only a continuation marker", lineNumber)
			continue
		}
		if continued != "" && trimmed == "" {
			assert.Failf(t, "empty iPXE continuation target", "line %d is empty after a continued command", lineNumber)
			continue
		}
		if continued == "" && (trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ":")) {
			continue
		}

		withoutTrailingSpace := strings.TrimRight(line, " \t")
		hasContinuation := strings.HasSuffix(withoutTrailingSpace, "\\")
		segment := strings.TrimSpace(strings.TrimSuffix(withoutTrailingSpace, "\\"))
		if segment == "" {
			assert.Failf(t, "empty iPXE command segment", "line %d has no command before a continuation marker", lineNumber)
			continue
		}
		if continued != "" {
			continued += " " + segment
		} else {
			continued = segment
		}
		if hasContinuation {
			continue
		}

		command := strings.Join(strings.Fields(continued), " ")
		fields := strings.Fields(command)
		require.NotEmpty(t, fields, "line %d produced an empty iPXE command", lineNumber)
		assert.True(t, allowedCommands[fields[0]], "line %d uses unknown iPXE command %q", lineNumber, fields[0])
		commands = append(commands, command)
		continued = ""
	}
	assert.Empty(t, continued, "iPXE script ended with an unterminated line continuation")
	return commands
}

func findIPXECommand(t *testing.T, commands []string, commandName string) string {
	t.Helper()
	prefix := commandName + " "
	for _, command := range commands {
		if strings.HasPrefix(command, prefix) {
			return command
		}
	}
	t.Fatalf("missing iPXE command %q in %#v", commandName, commands)
	return ""
}

func writeRenderedFile(t *testing.T, name string, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func paramsWith(params map[string]interface{}, key string, value interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(params)+1)
	for k, v := range params {
		copied[k] = v
	}
	copied[key] = value
	return copied
}
