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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
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

var structuredRenderUsers = map[string]mappings.ResolvedUser{
	"infra": {
		Name:              "infra",
		Primary:           true,
		FullName:          "Infrastructure User",
		PasswordCrypted:   "$6$infra",
		SSHAuthorizedKeys: []string{"ssh-ed25519 AAAA infra"},
		Groups:            []string{"sudo", "adm"},
		Shell:             "/bin/bash",
		Sudo:              "ALL=(ALL) NOPASSWD:ALL",
	},
	"locked": {
		Name:   "locked",
		Locked: true,
	},
	"root": {
		Name:            "root",
		System:          true,
		PasswordCrypted: "$6$root",
	},
}

var siteOnlyMarkers = []string{
	"git@example.com:infra/provisioning.git",
	"/configs/static/firstboot",
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

func TestRenderedDebianIPXEAppliesStructuredProvisioning(t *testing.T) {
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "shoelaces.example.test:8081",
		"hostname": "structured-host",
	}, nil, mappings.ProvisioningConfig{
		Boot: mappings.BootConfig{
			Netboot: mappings.NetbootConfig{
				KernelArgs: []string{"console=ttyS1"},
			},
		},
		Repos: mappings.ReposConfig{
			OSMirror: "https://deb.example/debian",
			Release:  "trixie",
		},
		Installer: mappings.InstallerConfig{
			ConfigTemplate: "preseed/debian",
			ConfigParams: map[string]any{
				"site": "iad",
			},
		},
	})

	rendered := renderTemplate(t, newRenderer(t), "debian.ipxe", params)

	assert.Contains(t, rendered, "Debian trixie netboot")
	assert.Contains(t, rendered, "set mirror https://deb.example/debian/dists/trixie/")
	assert.Contains(t, rendered, "preseed/url=http://shoelaces.example.test:8081/configs/preseed/debian?encrypt_home=false")
	assert.NotContains(t, rendered, "&site=iad")
	assert.NotContains(t, rendered, "&provisioning=")
	assert.NotContains(t, rendered, "&users=")
	assert.Contains(t, rendered, "console=ttyS1")
}

func TestValidateIPXEScriptCases(t *testing.T) {
	for _, tt := range []struct {
		name        string
		rendered    string
		expected    []string
		expectedErr string
	}{
		{
			name: "single line commands",
			rendered: `#!ipxe
set hostname render-validation-host
kernel http://example.test/vmlinuz initrd=initrd.img
initrd http://example.test/initrd.img
boot
`,
			expected: []string{
				"set hostname render-validation-host",
				"kernel http://example.test/vmlinuz initrd=initrd.img",
				"initrd http://example.test/initrd.img",
				"boot",
			},
		},
		{
			name: "continued kernel command",
			rendered: `#!ipxe
kernel http://example.test/vmlinuz \
  initrd=initrd.img \
  console=ttyS0
initrd http://example.test/initrd.img
boot
`,
			expected: []string{
				"kernel http://example.test/vmlinuz initrd=initrd.img console=ttyS0",
				"initrd http://example.test/initrd.img",
				"boot",
			},
		},
		{
			name: "comments labels and blanks are ignored",
			rendered: `#!ipxe

# comment
:retry
echo hello
boot
`,
			expected: []string{"echo hello", "boot"},
		},
		{
			name: "missing shebang",
			rendered: `echo hello
boot
`,
			expectedErr: "missing #!ipxe shebang",
		},
		{
			name: "standalone continuation",
			rendered: `#!ipxe
kernel http://example.test/vmlinuz \
\
initrd http://example.test/initrd.img
`,
			expectedErr: "standalone continuation",
		},
		{
			name: "empty continuation target",
			rendered: `#!ipxe
kernel http://example.test/vmlinuz \

initrd http://example.test/initrd.img
`,
			expectedErr: "empty continuation target",
		},
		{
			name:        "unterminated continuation",
			rendered:    "#!ipxe\nkernel http://example.test/vmlinuz \\",
			expectedErr: "unterminated line continuation",
		},
		{
			name: "unknown command",
			rendered: `#!ipxe
bogus command
`,
			expectedErr: "unknown iPXE command",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			commands, err := validateIPXEScript(tt.rendered)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, commands)
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
	// debconf-set-selections is the closest lightweight syntax checker for
	// preseed files; CI requires it while local runs skip when unavailable.
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

func TestRenderedEncryptedDebianPreseedsPassDebconfSetSelectionsWhenAvailable(t *testing.T) {
	debconfSetSelections := validatorPath(t, "debconf-set-selections")
	renderer := newRenderer(t)
	for _, tt := range []struct {
		name   string
		params map[string]interface{}
	}{
		{name: "regular", params: encryptedDebianParams(mappings.StorageConfig{})},
		{name: "lvm", params: encryptedDebianParams(mappings.StorageConfig{Mode: "lvm", VolumeGroup: "vgluks"})},
		{name: "raid", params: encryptedDebianParams(mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level:   1,
				Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			},
		})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			preseedPath := writeRenderedFile(t, "preseed.cfg", renderTemplate(t, renderer, "preseed/debian", tt.params))

			output, err := exec.Command(debconfSetSelections, "--checkonly", preseedPath).CombinedOutput()
			require.NoError(t, err, string(output))
		})
	}
}

func TestRenderedDebianPreseedsDoNotContainEmptyContinuationTargets(t *testing.T) {
	renderer := newRenderer(t)
	for _, tt := range []struct {
		name   string
		params map[string]interface{}
	}{
		{name: "default", params: defaultRenderParams},
		{name: "regular encrypted", params: encryptedDebianParams(mappings.StorageConfig{})},
		{name: "regular with late commands", params: installerLateCommandsParams(mappings.StorageConfig{})},
		{name: "regular encrypted with late commands", params: installerLateCommandsParams(mappings.StorageConfig{
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    boolPtr(true),
				Passphrase: "luks-passphrase",
			},
		})},
		{name: "lvm encrypted", params: encryptedDebianParams(mappings.StorageConfig{Mode: "lvm", VolumeGroup: "vgluks"})},
		{name: "lvm encrypted with late commands", params: installerLateCommandsParams(mappings.StorageConfig{
			Mode:        "lvm",
			VolumeGroup: "vgluks",
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    boolPtr(true),
				Passphrase: "luks-passphrase",
			},
		})},
		{name: "raid encrypted", params: encryptedDebianParams(mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level:   1,
				Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			},
		})},
		{name: "raid with late commands", params: installerLateCommandsParams(mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level:   1,
				Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			},
		})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, "preseed/debian", tt.params)

			assertNoEmptyContinuationTargets(t, rendered)
		})
	}
}

func TestExampleMappingsRenderDebian13Targets(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		luksEnv    string
		passphrase string
	}{
		{
			name:       "repository",
			path:       filepath.Join("..", "..", "configs", "data-dir", "mappings.yaml"),
			luksEnv:    "SHOELACES_LUKS_PASSPHRASE",
			passphrase: "repo-luks-passphrase",
		},
		{
			name:       "development",
			path:       filepath.Join("..", "..", "dev", "data-dir", "mappings.yaml"),
			luksEnv:    "SHOELACES_DEV_LUKS_PASSPHRASE",
			passphrase: "dev-luks-passphrase",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := mappings.ParseMappings(log.MakeLogger(io.Discard), tt.path)
			require.NoError(t, err)
			resolver, err := mappings.NewResolver(parsed)
			require.NoError(t, err)
			renderer := newRenderer(t)

			plain := resolveExampleTarget(t, resolver, "debian13", tt.luksEnv, tt.passphrase)
			plainIPXE := renderTemplate(t, renderer, plain.Target.Script, paramsForResolvedTarget(plain))
			plainPreseed := renderTemplate(t, renderer, "preseed/debian", paramsForResolvedTarget(plain))
			assert.Contains(t, plainIPXE, "Debian trixie netboot")
			assert.Contains(t, plainPreseed, "d-i partman-auto/choose_recipe select uefi-regular")
			assert.NotContains(t, plainPreseed, "d-i partman-crypto/passphrase")

			luks := resolveExampleTarget(t, resolver, "debian13-luks", tt.luksEnv, tt.passphrase)
			luksIPXE := renderTemplate(t, renderer, luks.Target.Script, paramsForResolvedTarget(luks))
			luksPreseed := renderTemplate(t, renderer, "preseed/debian", paramsForResolvedTarget(luks))
			assert.Contains(t, luksIPXE, "Debian trixie netboot")
			assert.NotContains(t, luksIPXE, tt.passphrase)
			assert.Contains(t, luksPreseed, "d-i partman-auto/method string crypto")
			assert.Contains(t, luksPreseed, "d-i partman-auto/choose_recipe select uefi-regular-luks")
			assert.Contains(t, luksPreseed, "d-i partman-crypto/passphrase password "+tt.passphrase)
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

func TestRenderedPreseedsApplyStructuredUsers(t *testing.T) {
	renderer := newRenderer(t)
	params := paramsWithStructuredUsers(defaultRenderParams)

	for _, templateName := range []string{
		"preseed/debian",
		"preseed/storage",
		"preseed/ubuntu-minimal",
	} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, params)

			assert.Contains(t, rendered, "d-i passwd/root-login boolean true")
			assert.Contains(t, rendered, "d-i passwd/root-password-crypted password $6$root")
			assert.Contains(t, rendered, "d-i passwd/user-fullname string Infrastructure User")
			assert.Contains(t, rendered, "d-i passwd/username string infra")
			assert.Contains(t, rendered, "d-i passwd/user-password-crypted password $6$infra")
		})
	}
}

func TestRenderedDebianPreseedDoesNotIncludeSitePostInstallLogicByDefault(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", defaultRenderParams)

	assert.NotContains(t, rendered, "d-i preseed/late_command")
	assert.NotContains(t, rendered, "firstboot")
	assert.NotContains(t, rendered, "ansible")
}

func TestRenderedDebianPreseedStorageModeSelection(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "default regular",
			params: defaultRenderParams,
			want:   "d-i partman-auto/method string regular",
		},
		{
			name: "structured lvm",
			params: paramsWith(defaultRenderParams, "provisioning", mappings.ProvisioningConfig{
				Storage: mappings.StorageConfig{
					Mode: "lvm",
				},
			}),
			want: "d-i partman-auto/method string lvm",
		},
		{
			name:   "explicit lvm",
			params: paramsWith(defaultRenderParams, "storage_mode", "lvm"),
			want:   "d-i partman-auto/method string lvm",
		},
		{
			name: "structured raid",
			params: paramsWith(defaultRenderParams, "provisioning", mappings.ProvisioningConfig{
				Storage: mappings.StorageConfig{
					Mode: "raid",
					RAID: mappings.RAIDConfig{
						Level:   1,
						Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
					},
				},
				Boot: mappings.BootConfig{Firmware: "uefi"},
			}),
			want: "d-i partman-auto/method string raid",
		},
	}

	renderer := newRenderer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, "preseed/debian", tt.params)

			assert.Contains(t, rendered, tt.want)
		})
	}
}

func TestRenderedDebianPreseedPreservesStorageFlow(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", defaultRenderParams)

	assertInOrder(t, rendered,
		"d-i apt-setup/contrib boolean",
		"d-i partman-auto/disk string",
		"d-i partman/early_command string",
		"d-i partman-auto/method string",
		"d-i partman-partitioning/confirm_write_new_label boolean true",
		"d-i grub2/linux_cmdline string",
		"d-i user-setup/encrypt-home boolean",
		"d-i grub-installer/timeout string",
	)
}

func TestRenderedDebianPreseedWipeTrueIncludesDiskWipeEarlyCommand(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", paramsWith(defaultRenderParams, "storage_wipe", "true"))

	assert.Contains(t, rendered, "d-i partman/early_command string")
	assert.Contains(t, rendered, "for d in /dev/nvme0n1 /dev/nvme1n1; do")
	assert.Contains(t, rendered, `[ -b "$d" ] || continue`)
	assert.Contains(t, rendered, `d="$(readlink -f "$d" 2>/dev/null || echo "$d")"`)
	assert.Contains(t, rendered, `b="${d##*/}"`)
	assert.Contains(t, rendered, `[ ! -e "/sys/class/block/$b/partition" ] || continue`)
	assert.Contains(t, rendered, `sfdisk --delete "$d" 2>/dev/null || true`)
	assert.Contains(t, rendered, `dd if=/dev/zero of="$d" bs=1M count=10 conv=fsync || true`)
	assert.Contains(t, rendered, `sectors="$(blockdev --getsz "$d" 2>/dev/null || echo 0)"`)
	assert.Contains(t, rendered, `seek="$((sectors - 20480))"`)
	assert.Contains(t, rendered, `dd if=/dev/zero of="$d" bs=512 seek="$seek" count=20480 conv=fsync || true`)
	assert.Contains(t, rendered, `blockdev --rereadpt "$d" 2>/dev/null || partprobe "$d" 2>/dev/null || true`)
}

func TestRenderedDebianPreseedWipeFalseOmitsDestructiveWipeCommands(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", paramsWith(defaultRenderParams, "storage_wipe", "false"))

	assert.NotContains(t, rendered, "d-i partman/early_command string")
	assert.NotContains(t, rendered, "dd if=/dev/zero")
	assert.NotContains(t, rendered, "sfdisk --delete")
	assert.NotContains(t, rendered, "vgremove")
	assert.NotContains(t, rendered, "pvremove")
}

func TestRenderedDebianPreseedWipeUsesDiskSelectors(t *testing.T) {
	params := paramsWith(defaultRenderParams, "storage_wipe", "true")
	params = paramsWith(params, "storage_wipe_disks", "/dev/nvme*n* /dev/sd*")

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "for d in /dev/nvme*n* /dev/sd*; do")
	assert.Contains(t, rendered, `[ -b "$d" ] || continue`)
	assert.Contains(t, rendered, `[ ! -e "/sys/class/block/$b/partition" ] || continue`)
}

func TestRenderedDebianPreseedDefaultRegularRecipeIsNonLVM(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", defaultRenderParams)

	assert.Contains(t, rendered, "d-i partman-auto/method string regular")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-regular")
	assert.Contains(t, rendered, "512 512 512 fat32")
	assert.Contains(t, rendered, "method{ efi } format{ }")
	assert.Contains(t, rendered, "mountpoint{ /boot/efi }")
	assert.Contains(t, rendered, "1024 1024 1024 ext4")
	assert.Contains(t, rendered, "mountpoint{ /boot }")
	assert.Contains(t, rendered, "8192 8192 8192 linux-swap")
	assert.Contains(t, rendered, "method{ swap } format{ }")
	assert.Contains(t, rendered, "20000 100000000 -1 ext4")
	assert.Contains(t, rendered, "use_filesystem{ } filesystem{ ext4 }")
	assert.Contains(t, rendered, "mountpoint{ / }")
	assert.NotContains(t, rendered, "partman-lvm")
	assert.NotContains(t, rendered, "partman-auto-lvm")
	assert.NotContains(t, rendered, "method{ lvm }")
	assert.NotContains(t, rendered, "$lvmok{ }")
	assert.NotContains(t, rendered, "in_vg{")
	assert.NotContains(t, rendered, "vg_name{")
	assert.NotContains(t, rendered, "lv_name{")
}

func TestRenderedDebianPreseedRegularRecipeAppliesStructuredFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Filesystems: map[string]mappings.FilesystemConfig{
				"esp": {
					Mountpoint: "/efi",
					SizeMiB:    &espSize,
				},
				"boot": {
					Mountpoint: "/boot",
					FSType:     "xfs",
					SizeMiB:    &bootSize,
				},
				"swap": {
					SizeMiB: &swapSize,
				},
				"root": {
					Mountpoint: "/",
					FSType:     "xfs",
					Size:       "grow",
					SizeMiB:    &rootMinSize,
				},
			},
		},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i anna/choose_modules string partman-xfs")
	assert.Contains(t, rendered, "768 768 768 fat32")
	assert.Contains(t, rendered, "mountpoint{ /efi }")
	assert.Contains(t, rendered, "2048 2048 2048 xfs")
	assert.Contains(t, rendered, "use_filesystem{ } filesystem{ xfs }")
	assert.Contains(t, rendered, "4096 4096 4096 linux-swap")
	assert.Contains(t, rendered, "65536 100000000 -1 xfs")
	assert.NotContains(t, rendered, "1024 1024 1024 ext4")
	assert.NotContains(t, rendered, "8192 8192 8192 linux-swap")
	assert.NotContains(t, rendered, "20000 100000000 -1 ext4")
}

func TestRenderedDebianPreseedPreservesExplicitLVMRecipe(t *testing.T) {
	params := paramsWith(defaultRenderParams, "storage_mode", "lvm")
	params = paramsWith(params, "vg_name", "vgtest")

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i partman-auto/method string lvm")
	assert.Contains(t, rendered, "d-i anna/choose_modules string lvm2-udeb partman-lvm partman-ext4")
	assert.Contains(t, rendered, "d-i partman-auto-lvm/new_vg_name string vgtest")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-lvm")
	assert.Contains(t, rendered, "vg_name{ vgtest }")
	assert.Contains(t, rendered, "in_vg{ vgtest }")
	assert.Contains(t, rendered, "lv_name{ root }")
	assert.Contains(t, rendered, "lv_name{ swap }")
}

func TestRenderedDebianPreseedLVMRecipeAppliesStructuredFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Mode:        "lvm",
			VolumeGroup: "vgtest",
			Filesystems: map[string]mappings.FilesystemConfig{
				"esp": {
					Mountpoint: "/efi",
					SizeMiB:    &espSize,
				},
				"boot": {
					Mountpoint: "/boot",
					FSType:     "xfs",
					SizeMiB:    &bootSize,
				},
				"swap": {
					SizeMiB: &swapSize,
				},
				"root": {
					Mountpoint: "/",
					FSType:     "xfs",
					Size:       "grow",
					SizeMiB:    &rootMinSize,
				},
			},
		},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i anna/choose_modules string lvm2-udeb partman-lvm partman-xfs")
	assert.Contains(t, rendered, "d-i partman-auto-lvm/new_vg_name string vgtest")
	assert.Contains(t, rendered, "768 768 768 fat32")
	assert.Contains(t, rendered, "mountpoint{ /efi }")
	assert.Contains(t, rendered, "2048 2048 2048 xfs")
	assert.Contains(t, rendered, "use_filesystem{ } filesystem{ xfs }")
	assert.Contains(t, rendered, "1000 10000 -1 lvm")
	assert.Contains(t, rendered, "4096 4096 4096 linux-swap")
	assert.Contains(t, rendered, "65536 100000000 -1 xfs")
	assert.Contains(t, rendered, "in_vg{ vgtest }")
	assert.Contains(t, rendered, "lv_name{ root }")
	assert.Contains(t, rendered, "lv_name{ swap }")
	assert.NotContains(t, rendered, "1024 1024 1024 ext4")
	assert.NotContains(t, rendered, "8192 8192 8192 linux-swap")
	assert.NotContains(t, rendered, "20000 100000000 -1 ext4")
}

func TestRenderedDebianPreseedLVMModulesFollowFinalFilesystemOverrides(t *testing.T) {
	params := paramsWith(defaultRenderParams, "storage_mode", "lvm")
	params = paramsWith(params, "debian_lvm_root_fstype", "xfs")

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i anna/choose_modules string lvm2-udeb partman-lvm partman-ext4 partman-xfs")
	assert.Contains(t, rendered, "use_filesystem{ } filesystem{ xfs }")
}

func TestRenderedDebianPreseedRAIDRecipe(t *testing.T) {
	bootDegraded := false
	params := mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level:        1,
				Devices:      []string{"/dev/nvme0n1", "/dev/nvme1n1"},
				BootDegraded: &bootDegraded,
			},
		},
		Boot: mappings.BootConfig{Firmware: "uefi"},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i partman-auto/disk string /dev/nvme0n1 /dev/nvme1n1")
	assert.Contains(t, rendered, "d-i anna/choose_modules string mdadm-udeb partman-md partman-ext4")
	assert.Contains(t, rendered, "d-i partman-auto/method string raid")
	assert.Contains(t, rendered, "d-i partman-md/device_remove_md boolean true")
	assert.Contains(t, rendered, "d-i partman-md/confirm boolean true")
	assert.Contains(t, rendered, "d-i partman-md/confirm_nooverwrite boolean true")
	assert.Contains(t, rendered, "d-i mdadm/boot_degraded boolean false")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-raid1")
	assert.Contains(t, rendered, "d-i partman-auto-raid/recipe string")

	assertInOrder(t, rendered,
		"device{ /dev/nvme0n1 }",
		"mountpoint{ /boot/efi }",
		"device{ /dev/nvme1n1 }",
		"1024 1024 1024 raid",
		"method{ raid }",
		"raidid{ 1 }",
	)
	assert.Contains(t, rendered, "1 2 0 ext4 /boot")
	assert.Contains(t, rendered, "raidid=1")
	assert.Contains(t, rendered, "1 2 0 swap -")
	assert.Contains(t, rendered, "raidid=2")
	assert.Contains(t, rendered, "1 2 0 ext4 /")
	assert.Contains(t, rendered, "raidid=3")

	assert.NotContains(t, rendered, "partman-lvm")
	assert.NotContains(t, rendered, "partman-auto-lvm/new_vg_name")
	assert.NotContains(t, rendered, "method{ lvm }")
	assert.NotContains(t, rendered, "$lvmok{ }")
	assert.NotContains(t, rendered, "in_vg{")
	assert.NotContains(t, rendered, "lv_name{")
	assert.NotContains(t, rendered, "vg_name{")

	assert.Contains(t, rendered, "d-i preseed/late_command string")
	assert.Contains(t, rendered, "mdadm --detail --scan > /target/etc/mdadm/mdadm.conf")
	assert.Contains(t, rendered, "in-target update-initramfs -u")
	assert.Contains(t, rendered, "for d in /dev/nvme0n1 /dev/nvme1n1; do")
	assert.Contains(t, rendered, `case "$d" in /dev/disk/*) p="${d}-part1";; *[0-9]) p="${d}p1";; esac`)
	assert.Contains(t, rendered, "grub-install --target=x86_64-efi")
	assert.Contains(t, rendered, "EFI/BOOT/BOOTX64.EFI")
	assert.Contains(t, rendered, "/target/boot/efi-secondary")
}

func TestRenderedDebianPreseedRAIDLateCommandHandlesStableDiskSymlinks(t *testing.T) {
	params := mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level: 1,
				Devices: []string{
					"/dev/disk/by-path/pci-0000:00:17.0-ata-1",
					"/dev/disk/by-path/pci-0000:00:17.0-ata-2",
				},
			},
		},
		Boot: mappings.BootConfig{Firmware: "uefi"},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "for d in /dev/disk/by-path/pci-0000:00:17.0-ata-1 /dev/disk/by-path/pci-0000:00:17.0-ata-2; do")
	assert.Contains(t, rendered, `case "$d" in /dev/disk/*) p="${d}-part1";; *[0-9]) p="${d}p1";; esac`)
}

func TestRenderedDebianPreseedRAIDRecipeAppliesStructuredFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Mode: "raid",
			RAID: mappings.RAIDConfig{
				Level:   1,
				Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
			},
			Filesystems: map[string]mappings.FilesystemConfig{
				"esp": {
					Mountpoint: "/efi",
					SizeMiB:    &espSize,
				},
				"boot": {
					Mountpoint: "/boot",
					FSType:     "xfs",
					SizeMiB:    &bootSize,
				},
				"swap": {
					SizeMiB: &swapSize,
				},
				"root": {
					Mountpoint: "/",
					FSType:     "xfs",
					Size:       "grow",
					SizeMiB:    &rootMinSize,
				},
			},
		},
		Boot: mappings.BootConfig{Firmware: "uefi"},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i anna/choose_modules string mdadm-udeb partman-md partman-xfs")
	assert.Contains(t, rendered, "768 768 768 fat32")
	assert.Contains(t, rendered, "mountpoint{ /efi }")
	assert.Contains(t, rendered, "2048 2048 2048 raid")
	assert.Contains(t, rendered, "4096 4096 4096 raid")
	assert.Contains(t, rendered, "65536 100000000 -1 raid")
	assert.Contains(t, rendered, "1 2 0 xfs /boot")
	assert.Contains(t, rendered, "1 2 0 xfs /")
	assert.NotContains(t, rendered, "1024 1024 1024 raid")
	assert.NotContains(t, rendered, "8192 8192 8192 raid")
	assert.NotContains(t, rendered, "20000 100000000 -1 raid")
}

func TestRenderedDebianPreseedRAIDRecipeRejectsMissingDevices(t *testing.T) {
	err := renderTemplateError(t, newRenderer(t), "preseed/debian", paramsWith(defaultRenderParams, "storage_mode", "raid"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `preseed/debian storage_raid_devices must contain exactly 2 devices for storage_mode "raid", got 0`)
}

func TestRenderedDebianPreseedRAIDRecipeAcceptsExplicitDeviceList(t *testing.T) {
	params := paramsWith(defaultRenderParams, "storage_mode", "raid")
	params = paramsWith(params, "storage_raid_devices", "/dev/vda /dev/vdb")

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i partman-auto/disk string /dev/vda /dev/vdb")
	assert.Contains(t, rendered, "device{ /dev/vda }")
	assert.Contains(t, rendered, "device{ /dev/vdb }")
	assert.Contains(t, rendered, "d-i mdadm/boot_degraded boolean true")
}

func TestRenderedDebianPreseedEncryptedRegularRecipe(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", encryptedDebianParams(mappings.StorageConfig{}))

	assert.Contains(t, rendered, "d-i partman-crypto/passphrase password luks-passphrase")
	assert.Contains(t, rendered, "d-i anna/choose_modules string crypto-dm-modules partman-auto-crypto partman-crypto partman-ext4")
	assert.Contains(t, rendered, "d-i partman/early_command string")
	assert.Equal(t, 1, strings.Count(rendered, "d-i partman/early_command string"))
	assertInOrder(t, rendered,
		"d-i partman/early_command string",
		"mdadm --stop --scan || true",
		"wget -O /bin/autopartition-crypto http://shoelaces.example.test:8081/configs/static/plain-luks-autopartition-crypto.sh",
		"d-i partman-crypto/passphrase password luks-passphrase",
	)
	assert.Contains(t, rendered, "wget -O /bin/autopartition-crypto http://shoelaces.example.test:8081/configs/static/plain-luks-autopartition-crypto.sh")
	assert.Contains(t, rendered, "chmod 0755 /bin/autopartition-crypto")
	assert.Contains(t, rendered, "d-i partman-auto/method string crypto")
	assert.Contains(t, rendered, "d-i partman-basicfilesystems/no_swap boolean false")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-regular-luks")
	assert.Contains(t, rendered, "method{ efi } format{ }")
	assert.Contains(t, rendered, "mountpoint{ /boot/efi }")
	assert.Contains(t, rendered, "mountpoint{ /boot }")
	assert.NotContains(t, rendered, "8192 8192 8192 linux-swap")
	assert.Contains(t, rendered, "20000 100000000 -1 crypto")
	assert.Contains(t, rendered, "method{ crypto }")
	assert.NotContains(t, rendered, "method{ swap } format{ }")
	assert.Contains(t, rendered, "options/crypto_type{ luks }")
	assert.Contains(t, rendered, "options/cipher{ aes-xts-plain64 }")
	assert.Contains(t, rendered, "options/keysize{ 512 }")
	assert.Contains(t, rendered, "options/hash{ sha512 }")
	rootStanza := regularEncryptedRootStanza(t, rendered)
	assert.Contains(t, rootStanza, "method{ crypto }")
	assert.Contains(t, rootStanza, "options/crypto_type{ luks }")
	assert.NotContains(t, rootStanza, "method{ format }")
	assert.NotContains(t, rootStanza, "format{ }")
	assert.NotContains(t, rootStanza, "use_filesystem{ }")
	assert.NotContains(t, rootStanza, "filesystem{ ext4 }")
	assert.NotContains(t, rootStanza, "mountpoint{ / }")
	assert.Contains(t, rendered, "root_source=\"$(awk '$2 == \"/target\" { print $1; exit }' /proc/mounts)\"")
	assert.Contains(t, rendered, "cryptsetup status \"$crypt_name\"")
	assert.Contains(t, rendered, "printf '%s UUID=%s none luks,discard,x-initrd.attach\\n' \"$crypt_name\" \"$backing_uuid\" > /target/etc/crypttab")
	assert.Contains(t, rendered, "in-target apt-get -y install cryptsetup-initramfs || true")
	assert.Contains(t, rendered, "in-target update-initramfs -u -k all")
	assert.NotContains(t, rendered, "systemd-cryptenroll")
	assert.NotContains(t, rendered, "tpm2-device=%s")
	assert.Contains(t, rendered, "chroot /target /usr/bin/fallocate -l 8192M /swapfile")
	assert.Contains(t, rendered, "dd if=/dev/zero of=/target/swapfile bs=1M count=8192")
	assert.Contains(t, rendered, "chroot /target /sbin/mkswap /swapfile")
	assert.Contains(t, rendered, "printf '/swapfile none swap sw 0 0\\n' >> /target/etc/fstab")
	assert.NotContains(t, rendered, "partman-auto-lvm/new_vg_name")
	assert.NotContains(t, rendered, "$lvmok{ }")
	assert.NotContains(t, rendered, "partman-auto-raid/recipe")
}

func TestRenderedDebianPreseedEncryptedRegularTPMUnlock(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", paramsWith(
		encryptedDebianTPMParams(),
		"boot_ref_query_question",
		"?ref=01JTPMREFTESTREFTEST0000",
	))

	assert.Contains(t, rendered, "d-i preseed/late_command string \\\n  set -eu;")
	assert.Contains(t, rendered, `wget -O /tmp/shoelaces-luks-tpm-setup.sh "http://shoelaces.example.test:8081/configs/generated/debian/luks-tpm-setup.sh?ref=01JTPMREFTESTREFTEST0000"`)
	assert.Contains(t, rendered, "chmod 0755 /tmp/shoelaces-luks-tpm-setup.sh")
	assert.Contains(t, rendered, "/tmp/shoelaces-luks-tpm-setup.sh")
	assert.Contains(t, rendered, "rm -f /tmp/shoelaces-luks-tpm-setup.sh")
	assert.NotContains(t, rendered, "systemd-cryptenroll")
	assert.NotContains(t, rendered, "cryptsetup luksDump")
}

func TestRenderedDebianLUKSTPMHelperUsesTwoPhaseEnrollment(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "generated/debian/luks-tpm-setup.sh", paramsWith(
		encryptedDebianTPMParams(),
		"boot_ref_query_question",
		"?ref=01JTPMREFTESTREFTEST0000",
	))

	assert.Contains(t, rendered, "base_url='http://shoelaces.example.test:8081'")
	assert.Contains(t, rendered, "boot_ref_query='?ref=01JTPMREFTESTREFTEST0000'")
	assert.Contains(t, rendered, "installer_tpm_pcrs=\"\"")
	assert.Contains(t, rendered, `wget -O "$tpm_passphrase_file" "$base_url/configs/generated/plain/luks-tpm.passphrase$boot_ref_query"`)
	assert.Contains(t, rendered, "in-target apt-get -y install cryptsetup-initramfs systemd-cryptsetup dracut-core dracut-config-generic tpm2-tools util-linux")
	assert.Contains(t, rendered, "mount --bind /sys /target/sys")
	assert.Contains(t, rendered, "chroot /target systemd-cryptenroll --tpm2-device=list")
	assert.Contains(t, rendered, "[ ! -e /sys/class/tpm/tpm0/pcr-sha256 ]")
	assert.Contains(t, rendered, `chroot /target systemd-cryptenroll "$backing_device" \`)
	assert.Contains(t, rendered, `--unlock-key-file="${tpm_passphrase_file#/target}" \`)
	assert.Contains(t, rendered, `--tpm2-device="$tpm_device" \`)
	assert.Contains(t, rendered, `--tpm2-pcrs="$installer_tpm_pcrs"`)
	assert.NotContains(t, rendered, `--tpm2-pcrs='7'`)
	assert.NotContains(t, rendered, "luks-passphrase")
	assert.Contains(t, rendered, "reenroll_tpm_passphrase_file=/target/var/lib/shoelaces/luks-tpm.passphrase")
	assert.Contains(t, rendered, "cp \"$tpm_passphrase_file\" \"$reenroll_tpm_passphrase_file\"")
	assert.Contains(t, rendered, "chmod 0600 \"$reenroll_tpm_passphrase_file\"")
	assert.Contains(t, rendered, "printf '%s UUID=%s none luks,discard,x-initrd.attach,tpm2-device=%s\\n'")
	assert.Contains(t, rendered, "add_dracutmodules+=\" systemd systemd-cryptsetup crypt tpm2-tss \"")
	assert.Contains(t, rendered, "add_drivers+=\" tpm tpm_tis tpm_tis_core tpm_crb \"")
	assert.Contains(t, rendered, "install_items+=\" /etc/crypttab \"")
	assert.Contains(t, rendered, `chroot /target dracut --force --hostonly --kver "$kernel_version" \`)
	assert.Contains(t, rendered, "menuentry '$grub_entry_name'")
	assert.Contains(t, rendered, `GRUB_DEFAULT=\"`)
	assert.Contains(t, rendered, "in-target update-grub")
	assert.Contains(t, rendered, "grep -F \"root=UUID=$root_uuid\" /target/boot/grub/grub.cfg")
	assert.Contains(t, rendered, "wget -O /target/usr/local/lib/shoelaces/luks-tpm-reenroll.sh \"$base_url/configs/generated/static/luks-tpm-reenroll.sh$boot_ref_query\"")
	assert.Contains(t, rendered, "wget -O /target/etc/default/shoelaces-luks-tpm-reenroll \"$base_url/configs/generated/plain/luks-tpm-reenroll.env$boot_ref_query\"")
	assert.Contains(t, rendered, "wget -O /target/etc/systemd/system/shoelaces-luks-tpm-reenroll.service \"$base_url/configs/generated/static/luks-tpm-reenroll.service$boot_ref_query\"")
	assert.Contains(t, rendered, "chroot /target /bin/systemctl enable shoelaces-luks-tpm-reenroll.service")
	assert.Contains(t, rendered, "chroot /target /sbin/mkswap /swapfile")
}

func TestRenderedGeneratedLUKSTPMReenrollScriptReenrollsLUKSTPM(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "generated/static/luks-tpm-reenroll.sh", defaultRenderParams)

	assert.Contains(t, rendered, "REENROLL_DONE=/var/lib/shoelaces/luks-tpm-reenroll.done")
	assert.Contains(t, rendered, "reenroll_phase reenroll-luks-tpm")
	assert.Contains(t, rendered, ": \"${SHOELACES_LUKS_TPM_PASSPHRASE_FILE:=/var/lib/shoelaces/luks-tpm.passphrase}\"")
	assert.Contains(t, rendered, ": \"${SHOELACES_LUKS_TPM_PCRS:=7}\"")
	assert.Contains(t, rendered, "root_source=\"$(findmnt -no SOURCE /)\"")
	assert.Contains(t, rendered, "systemd-cryptenroll --tpm2-device=list")
	assert.Contains(t, rendered, "systemd-cryptenroll \"$backing_device\" --wipe-slot=tpm2")
	assert.Contains(t, rendered, "--unlock-key-file=\"$SHOELACES_LUKS_TPM_PASSPHRASE_FILE\"")
	assert.Contains(t, rendered, "--tpm2-pcrs=\"$SHOELACES_LUKS_TPM_PCRS\"")
	assert.Contains(t, rendered, "grep -E \"tpm2-hash-pcrs:[[:space:]]*$SHOELACES_LUKS_TPM_PCRS\"")
	assert.Contains(t, rendered, "grep -E 'tpm2-pcr-bank:[[:space:]]*sha256'")
	assert.Contains(t, rendered, "rm -f \"$SHOELACES_LUKS_TPM_PASSPHRASE_FILE\"")
}

func TestRenderedGeneratedLUKSTPMPassphraseUsesResolvedSecret(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "generated/plain/luks-tpm.passphrase", encryptedDebianTPMParams())

	assert.Equal(t, "luks-passphrase", rendered)
}

func TestRenderedDebianPreseedEncryptedRegularTPMUnlockHonorsDisabledSHA256BankRequirement(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "generated/debian/luks-tpm-setup.sh", encryptedDebianParams(mappings.StorageConfig{
		Encryption: mappings.StorageEncryptionConfig{
			TPM: mappings.StorageEncryptionTPMConfig{
				Enabled:           boolPtr(true),
				RequireSHA256Bank: boolPtr(false),
			},
		},
	}))

	assert.Contains(t, rendered, "systemd-cryptenroll")
	assert.Contains(t, rendered, "require_sha256_bank='false'")
}

func TestRenderedDebianPreseedAppendsInstallerLateCommandsToRegularLUKS(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", installerLateCommandsParams(mappings.StorageConfig{
		Encryption: mappings.StorageEncryptionConfig{
			Enabled:    boolPtr(true),
			Passphrase: "luks-passphrase",
		},
	}))

	assert.Equal(t, 1, strings.Count(rendered, "d-i preseed/late_command string"))
	assertInOrder(t, rendered,
		"printf '%s UUID=%s none luks,discard,x-initrd.attach\\n' \"$crypt_name\" \"$backing_uuid\" > /target/etc/crypttab",
		"chroot /target /sbin/mkswap /swapfile",
		"    in-target systemctl enable ssh; \\",
		"    in-target touch /root/from-late-command; \\",
		"    true; \\\n  fi",
	)
	assert.NotContains(t, rendered, "\nd-i preseed/late_command string \\\n    in-target systemctl enable ssh")
}

func TestRenderedDebianPreseedRendersInstallerLateCommandsWithoutStorageLateCommand(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", installerLateCommandsParams(mappings.StorageConfig{}))

	assert.Equal(t, 1, strings.Count(rendered, "d-i preseed/late_command string"))
	assertInOrder(t, rendered,
		"# Site late commands. Storage modes with Shoelaces-managed late_command blocks",
		"d-i preseed/late_command string \\",
		"    in-target systemctl enable ssh; \\",
		"    in-target touch /root/from-late-command; \\\n  true",
	)
	assert.NotContains(t, rendered, "cryptsetup status \"$crypt_name\"")
	assert.NotContains(t, rendered, "mdadm --detail --scan > /target/etc/mdadm/mdadm.conf")
}

func TestRenderedDebianPreseedEncryptedRegularInstallsHelperWhenWipeDisabled(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", paramsWith(
		encryptedDebianParams(mappings.StorageConfig{}),
		"storage_wipe",
		"false",
	))

	assert.Equal(t, 1, strings.Count(rendered, "d-i partman/early_command string"))
	assert.Contains(t, rendered, "wget -O /bin/autopartition-crypto http://shoelaces.example.test:8081/configs/static/plain-luks-autopartition-crypto.sh")
	assert.Contains(t, rendered, "chmod 0755 /bin/autopartition-crypto")
	assert.NotContains(t, rendered, "dd if=/dev/zero of=\"$d\"")
	assert.NotContains(t, rendered, "sfdisk --delete")
	assert.NotContains(t, rendered, "vgremove")
	assert.NotContains(t, rendered, "pvremove")
}

func TestRenderedDebianPreseedEncryptedRegularRecipeHonorsDisabledSwap(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", paramsWith(
		encryptedDebianParams(mappings.StorageConfig{}),
		"debian_regular_swap_enabled",
		"false",
	))

	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-regular-luks")
	assert.NotContains(t, rendered, "d-i partman-basicfilesystems/no_swap boolean false")
	assert.NotContains(t, rendered, "8192 8192 8192 linux-swap")
	assert.NotContains(t, rendered, "/swapfile")
	assert.NotContains(t, rendered, "mkswap")
	assert.NotContains(t, rendered, "method{ swap }")
}

func TestPlainLUKSAutopartitionHelperIsEmbedded(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "configs", "data-dir", "static", "plain-luks-autopartition-crypto.sh"))
	require.NoError(t, err)
	script := string(content)

	assert.Contains(t, script, "crypto_check_setup || exit 1")
	assert.Contains(t, script, "crypto_setup no || exit 1")
	assert.Contains(t, script, "rm -f \"$id/format\" \"$id/use_filesystem\" \"$id/filesystem\" \"$id/mountpoint\"")
	assert.Contains(t, script, "touch \"$id/skip_erase\"")
	assert.Contains(t, script, `keysize="$((keysize / 2))"`)
	assertInOrder(t, script,
		"crypto_setup no || exit 1",
		"echo format > \"$id/method\"",
		"echo ext4 > \"$id/filesystem\"",
		"echo / > \"$id/mountpoint\"",
		"update_all",
	)
}

func TestRenderedDebianPreseedEncryptedLVMRecipe(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", encryptedDebianParams(mappings.StorageConfig{
		Mode:        "lvm",
		VolumeGroup: "vgluks",
	}))

	assert.Contains(t, rendered, "d-i partman-crypto/passphrase password luks-passphrase")
	assert.Contains(t, rendered, "d-i anna/choose_modules string crypto-dm-modules partman-crypto lvm2-udeb partman-lvm partman-ext4")
	assert.Contains(t, rendered, "d-i partman-auto/method string crypto")
	assert.Contains(t, rendered, "d-i partman-auto-lvm/new_vg_name string vgluks")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-lvm-luks")
	assert.Contains(t, rendered, "1000 10000 -1 crypto")
	assert.Contains(t, rendered, "method{ crypto }")
	assert.Contains(t, rendered, "method{ lvm }")
	assert.Contains(t, rendered, "vg_name{ vgluks }")
	assert.Contains(t, rendered, "in_vg{ vgluks }")
	assert.Contains(t, rendered, "lv_name{ root }")
	assert.Contains(t, rendered, "lv_name{ swap }")
	assert.Contains(t, rendered, "options/cipher{ aes-xts-plain64 }")
	assert.NotContains(t, rendered, "partman-auto/choose_recipe select uefi-lvm\n")
}

func TestRenderedDebianPreseedEncryptedRAIDRecipe(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", encryptedDebianParams(mappings.StorageConfig{
		Mode: "raid",
		RAID: mappings.RAIDConfig{
			Level:   1,
			Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		},
	}))

	assert.Contains(t, rendered, "d-i partman-crypto/passphrase password luks-passphrase")
	assert.Contains(t, rendered, "d-i anna/choose_modules string crypto-dm-modules partman-crypto mdadm-udeb partman-md partman-ext4")
	assert.Contains(t, rendered, "d-i partman-auto/disk string /dev/nvme0n1 /dev/nvme1n1")
	assert.Contains(t, rendered, "d-i partman-auto/method string raid")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-raid1-luks")
	assert.Contains(t, rendered, "device{ /dev/nvme0n1 }")
	assert.Contains(t, rendered, "device{ /dev/nvme1n1 }")
	assert.Contains(t, rendered, "1 2 0 ext4 /boot")
	assert.Contains(t, rendered, "1 2 0 crypto -")
	assert.Contains(t, rendered, "method=crypto")
	assert.Contains(t, rendered, "options/cipher=aes-xts-plain64")
	assert.Contains(t, rendered, "options/keysize=512")
	assert.Contains(t, rendered, "options/hash=sha512")
	assert.Contains(t, rendered, "raidid=2")
	assert.Contains(t, rendered, "raidid=3")
	assert.Contains(t, rendered, "mdadm --detail --scan > /target/etc/mdadm/mdadm.conf")
	assert.Contains(t, rendered, "grub-install --target=x86_64-efi")
	assert.NotContains(t, rendered, "partman-auto-lvm/new_vg_name")
	assert.NotContains(t, rendered, "1 2 0 ext4 / \\\n        raidid=3")
}

func TestRenderedDebianPreseedRejectsInvalidEncryptionParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "missing passphrase",
			params: paramsWith(defaultRenderParams, "storage_encryption_enabled", "true"),
			want:   "preseed/debian storage_encryption_passphrase is required when storage_encryption_enabled is true",
		},
		{
			name:   "invalid enabled value",
			params: paramsWith(defaultRenderParams, "storage_encryption_enabled", "yes"),
			want:   `preseed/debian storage_encryption_enabled must be "true" or "false", got "yes"`,
		},
		{
			name: "empty cipher",
			params: paramsWith(
				paramsWith(
					paramsWith(defaultRenderParams, "storage_encryption_enabled", "true"),
					"storage_encryption_passphrase", "luks-passphrase",
				),
				"storage_encryption_cipher", "",
			),
			want: "preseed/debian storage_encryption_cipher must not be empty when storage_encryption_enabled is true",
		},
		{
			name: "invalid key size",
			params: paramsWith(
				paramsWith(
					paramsWith(defaultRenderParams, "storage_encryption_enabled", "true"),
					"storage_encryption_passphrase", "luks-passphrase",
				),
				"storage_encryption_key_size", "0",
			),
			want: "preseed/debian storage_encryption_key_size must be a positive integer when storage_encryption_enabled is true",
		},
		{
			name: "tpm without encryption",
			params: paramsWith(
				defaultRenderParams,
				"storage_encryption_tpm_enabled",
				"true",
			),
			want: "preseed/debian storage_encryption_tpm_enabled requires storage_encryption_enabled to be true",
		},
		{
			name: "tpm requires regular mode",
			params: paramsWith(
				paramsWith(encryptedDebianTPMParams(), "storage_mode", "lvm"),
				"vg_name",
				"vgluks",
			),
			want: "preseed/debian storage_encryption_tpm_enabled is supported only when storage_mode is regular",
		},
		{
			name: "tpm requires dracut",
			params: paramsWith(
				encryptedDebianTPMParams(),
				"storage_encryption_tpm_initramfs",
				"initramfs-tools",
			),
			want: `preseed/debian storage_encryption_tpm_initramfs must be "dracut" when TPM unlock is enabled`,
		},
	}

	renderer := newRenderer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := renderTemplateError(t, renderer, "preseed/debian", tt.params)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRenderedDebianPreseedRejectsUnsupportedStorageMode(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "explicit query style param",
			params: paramsWith(defaultRenderParams, "storage_mode", "zfs"),
			want:   `preseed/debian storage_mode must be "regular", "lvm", or "raid", got "zfs"`,
		},
		{
			name: "structured provisioning",
			params: paramsWith(defaultRenderParams, "provisioning", mappings.ProvisioningConfig{
				Storage: mappings.StorageConfig{
					Mode: "zfs",
				},
			}),
			want: `preseed/debian storage_mode must be "regular", "lvm", or "raid", got "zfs"`,
		},
		{
			name:   "legacy plain mode",
			params: paramsWith(defaultRenderParams, "storage_mode", "plain"),
			want:   `preseed/debian storage_mode must be "regular", "lvm", or "raid", got "plain"`,
		},
	}

	renderer := newRenderer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := renderTemplateError(t, renderer, "preseed/debian", tt.params)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRenderedTemplatesRejectUnsupportedStructuredEncryption(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   map[string]interface{}
	}{
		{name: "ubuntu preseed", template: "preseed/ubuntu-minimal", params: encryptedUnsupportedParams(defaultRenderParams)},
		{name: "legacy storage preseed", template: "preseed/storage", params: encryptedUnsupportedParams(defaultRenderParams)},
		{name: "centos kickstart", template: "centos.ks", params: encryptedUnsupportedParams(kickstartRenderParams)},
		{name: "coreos cloud config", template: "cloudconfig-coreos", params: encryptedUnsupportedParams(defaultRenderParams)},
		{name: "ubuntu ipxe", template: "ubuntu-minimal.ipxe", params: encryptedUnsupportedParams(defaultRenderParams)},
		{name: "legacy storage ipxe", template: "storage.ipxe", params: encryptedUnsupportedParams(defaultRenderParams)},
		{name: "centos ipxe", template: "centos.ipxe", params: encryptedUnsupportedParams(kickstartRenderParams)},
		{name: "coreos ipxe", template: "coreos.ipxe", params: encryptedUnsupportedParams(defaultRenderParams)},
	}

	renderer := newRenderer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := renderTemplateError(t, renderer, tt.template, tt.params)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.template+" does not support structured storage encryption")
			assert.Contains(t, err.Error(), "storage.encryption is supported only by preseed/debian")
		})
	}
}

func TestDiskBackedNonDebianInstallerTemplatesAllowStructuredEncryption(t *testing.T) {
	dataDir := t.TempDir()
	writeRenderTemplate(t, dataDir, "preseed/custom.slc", `{{define "preseed/custom" -}}custom preseed{{end}}`)
	writeRenderTemplate(t, dataDir, "kickstart/custom.ks.slc", `{{define "kickstart/custom.ks" -}}custom kickstart{{end}}`)
	writeRenderTemplate(t, dataDir, "cloud-config/custom.slc", `{{define "cloud-config/custom" -}}#cloud-config{{end}}`)
	renderer := newRendererWithDataDir(t, dataDir)

	for _, templateName := range []string{"preseed/custom", "kickstart/custom.ks", "cloud-config/custom"} {
		t.Run(templateName, func(t *testing.T) {
			rendered := renderTemplate(t, renderer, templateName, encryptedUnsupportedParams(defaultRenderParams))

			assert.NotEmpty(t, rendered)
		})
	}
}

func TestDiskOverridesAllowStructuredEncryptionForUnsupportedEmbeddedTemplates(t *testing.T) {
	dataDir := t.TempDir()
	writeRenderTemplate(t, dataDir, "preseed/ubuntu-minimal.preseed.slc", `{{define "preseed/ubuntu-minimal" -}}
native encrypted ubuntu for {{.hostname}} {{.storage_encryption_enabled}}
{{end}}
`)
	renderer := newRendererWithDataDir(t, dataDir)

	rendered := renderTemplate(t, renderer, "preseed/ubuntu-minimal", encryptedUnsupportedParams(defaultRenderParams))

	assert.Contains(t, rendered, "native encrypted ubuntu for render-validation-host true")
}

func TestInstallerExtraAllowsStructuredEncryptionForUnsupportedEmbeddedTemplates(t *testing.T) {
	dataDir := t.TempDir()
	writeRenderTemplate(t, dataDir, "provisioning/extra.slc", `{{define "provisioning/extra" -}}
d-i preseed/late_command string echo native encryption {{.storage_encryption_enabled}}
{{end}}
`)
	renderer := newRendererWithDataDir(t, dataDir)
	enabled := true
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "shoelaces.example.test:8081",
		"hostname": "extra-encrypted-host",
	}, nil, mappings.ProvisioningConfig{
		Installer: mappings.InstallerConfig{
			ExtraTemplate: "provisioning/extra",
		},
		Storage: mappings.StorageConfig{
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    &enabled,
				Passphrase: "luks-passphrase",
			},
		},
	})

	rendered := renderTemplate(t, renderer, "preseed/ubuntu-minimal", params)

	assert.Contains(t, rendered, "d-i preseed/late_command string echo native encryption true")
}

func TestRenderedDebianPreseedAppliesStructuredProvisioning(t *testing.T) {
	utc := false
	ntp := false
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "shoelaces.example.test:8081",
		"hostname": "structured-preseed",
	}, nil, mappings.ProvisioningConfig{
		Locale: mappings.LocaleConfig{
			Language: "en_GB.UTF-8",
			Keyboard: "gb",
		},
		Time: mappings.TimeConfig{
			Timezone: "Europe/London",
			UTC:      &utc,
			NTP:      &ntp,
		},
		Packages: mappings.PackagesConfig{
			Install:      []string{"curl", "vim"},
			UpdatePolicy: "none",
		},
		Storage: mappings.StorageConfig{
			Disk:             "/dev/vda",
			Mode:             "regular",
			VolumeGroup:      "vgtest",
			WipeDiskPatterns: []string{"/dev/nvme*n*", "/dev/sd*"},
		},
		Boot: mappings.BootConfig{
			Installed: mappings.InstalledBootConfig{
				KernelArgs: []string{"panic=30"},
			},
		},
		Repos: mappings.ReposConfig{
			Release: "trixie",
		},
	})

	rendered := renderTemplate(t, newRenderer(t), "preseed/debian", params)

	assert.Contains(t, rendered, "d-i debian-installer/locale string en_GB.UTF-8")
	assert.Contains(t, rendered, "d-i keyboard-configuration/xkb-keymap select gb")
	assert.Contains(t, rendered, "d-i time/zone string Europe/London")
	assert.Contains(t, rendered, "d-i clock-setup/utc boolean false")
	assert.Contains(t, rendered, "d-i partman-auto/disk string /dev/vda")
	assert.Contains(t, rendered, "for d in /dev/nvme*n* /dev/sd*; do \\")
	assert.Contains(t, rendered, "d-i partman-auto/method string regular")
	assert.Contains(t, rendered, "d-i partman-auto/choose_recipe select uefi-regular")
	assert.NotContains(t, rendered, "vg_name{ vgtest }")
	assert.Contains(t, rendered, "d-i grub2/linux_cmdline string panic=30")
	assert.Contains(t, rendered, "d-i clock-setup/ntp boolean false")
	assert.Contains(t, rendered, "d-i pkgsel/update-policy select none")
	assert.Contains(t, rendered, "d-i pkgsel/include string curl vim")
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
	assert.Contains(t, rendered, "reboot")
}

func TestRenderedKickstartAppliesStructuredProvisioning(t *testing.T) {
	params := mappings.ParamsWithProvisioning(map[string]interface{}{
		"baseURL":  "shoelaces.example.test:8081",
		"hostname": "structured-centos",
	}, nil, mappings.ProvisioningConfig{
		Locale: mappings.LocaleConfig{
			Language: "en_GB.UTF-8",
			Keyboard: "gb",
		},
		Network: mappings.NetworkConfig{
			Bootproto: "dhcp",
		},
		Packages: mappings.PackagesConfig{
			Groups:  []string{"core", "minimal-environment"},
			Install: []string{"curl", "vim"},
		},
		Storage: mappings.StorageConfig{
			Disk:        "/dev/vda",
			VolumeGroup: "vgks",
		},
		Boot: mappings.BootConfig{
			Installed: mappings.InstalledBootConfig{
				KernelArgs: []string{"panic=30"},
			},
		},
		Repos: mappings.ReposConfig{
			OSMirror: "https://centos.example/centos",
			Release:  "9-stream",
		},
	})

	rendered := renderTemplate(t, newRenderer(t), "centos.ks", params)

	assert.Contains(t, rendered, `url  --url="https://centos.example/centos/9-stream/os/x86_64"`)
	assert.Contains(t, rendered, "network --bootproto dhcp --hostname structured-centos")
	assert.Contains(t, rendered, "keyboard --vckeymap=gb --xlayouts='gb'")
	assert.Contains(t, rendered, "lang      en_GB.UTF-8")
	assert.Contains(t, rendered, "clearpart --drives=vda --all --disklabel=gpt")
	assert.Contains(t, rendered, `bootloader --append="panic=30" --location=mbr`)
	assert.Contains(t, rendered, "volgroup vgks pv.01")
	assert.Contains(t, rendered, "@core\n@minimal-environment\ncurl\nvim")
}

func TestRenderedKickstartAppliesStructuredUsers(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "centos.ks", paramsWithStructuredUsers(kickstartRenderParams))

	assert.Contains(t, rendered, "rootpw --iscrypted $6$root")
	assert.Contains(t, rendered, `user --name=infra --gecos="Infrastructure User" --groups=sudo,adm --shell=/bin/bash --iscrypted --password=$6$infra`)
	assert.Contains(t, rendered, "user --name=locked --lock")
	assert.Contains(t, rendered, "%post --log=/root/ks-infra-ssh-keys.log")
	assert.Contains(t, rendered, "ssh-ed25519 AAAA infra")
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

func TestRenderedCloudConfigAppliesStructuredUsers(t *testing.T) {
	rendered := renderTemplate(t, newRenderer(t), "cloudconfig-coreos", paramsWithStructuredUsers(defaultRenderParams))
	withoutHeader := strings.TrimPrefix(rendered, "#cloud-config\n")
	var parsed map[string]interface{}

	require.NoError(t, yaml.Unmarshal([]byte(withoutHeader), &parsed))
	assert.Contains(t, parsed, "users")
	assert.Contains(t, rendered, "  - name: infra")
	assert.Contains(t, rendered, "    lock_passwd: false")
	assert.Contains(t, rendered, `    gecos: "Infrastructure User"`)
	assert.Contains(t, rendered, "    shell: /bin/bash")
	assert.Contains(t, rendered, `    passwd: "$6$infra"`)
	assert.Contains(t, rendered, "    groups:")
	assert.Contains(t, rendered, "      - sudo")
	assert.Contains(t, rendered, `    sudo: "ALL=(ALL) NOPASSWD:ALL"`)
	assert.Contains(t, rendered, "    ssh_authorized_keys:")
	assert.Contains(t, rendered, `      - "ssh-ed25519 AAAA infra"`)
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

	return newRendererWithDataDir(t, t.TempDir())
}

func newRendererWithDataDir(t *testing.T, dataDir string) *templates.ShoelacesTemplates {
	t.Helper()

	renderer := templates.New(log.MakeLogger(io.Discard))
	renderer.ParseTemplates(dataDir, "env_overrides", nil, ".slc")
	return renderer
}

func writeRenderTemplate(t *testing.T, dataDir, relativePath, content string) {
	t.Helper()

	path := filepath.Join(dataDir, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func renderTemplate(t *testing.T, renderer *templates.ShoelacesTemplates, name string, params map[string]interface{}) string {
	t.Helper()

	rendered, err := renderer.RenderTemplate(name, params, "")
	require.NoError(t, err)
	return rendered
}

func renderTemplateError(t *testing.T, renderer *templates.ShoelacesTemplates, name string, params map[string]interface{}) error {
	t.Helper()

	_, err := renderer.RenderTemplate(name, params, "")
	return err
}

func assertInOrder(t *testing.T, text string, needles ...string) {
	t.Helper()

	previous := -1
	for _, needle := range needles {
		current := strings.Index(text, needle)
		require.NotEqualf(t, -1, current, "missing %q", needle)
		require.Greaterf(t, current, previous, "%q was not after previous marker", needle)
		previous = current
	}
}

func regularEncryptedRootStanza(t *testing.T, rendered string) string {
	t.Helper()

	start := strings.Index(rendered, "20000 100000000 -1 crypto \\")
	require.NotEqual(t, -1, start, "missing regular encrypted root partition stanza")
	remaining := rendered[start:]
	end := strings.Index(remaining, "\n    .")
	require.NotEqual(t, -1, end, "missing regular encrypted root stanza terminator")
	return remaining[:end]
}

func assertValidIPXEScript(t *testing.T, rendered string) []string {
	t.Helper()

	commands, err := validateIPXEScript(rendered)
	require.NoError(t, err)
	return commands
}

func assertNoEmptyContinuationTargets(t *testing.T, rendered string) {
	t.Helper()

	lines := strings.Split(rendered, "\n")
	for i := 0; i < len(lines)-1; i++ {
		if strings.HasSuffix(strings.TrimRight(lines[i], " \t"), "\\") && strings.TrimSpace(lines[i+1]) == "" {
			t.Fatalf("line %d ends with a continuation marker but line %d is empty", i+1, i+2)
		}
	}
}

func validateIPXEScript(rendered string) ([]string, error) {
	// This is a rendered-script lint, not a full iPXE emulator. It catches the
	// line-continuation and command-shape failures that break before booting.
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
			if trimmed != "#!ipxe" {
				return nil, fmt.Errorf("line %d missing #!ipxe shebang", lineNumber)
			}
			continue
		}
		if trimmed == "\\" {
			return nil, fmt.Errorf("line %d contains a standalone continuation marker", lineNumber)
		}
		if continued != "" && trimmed == "" {
			return nil, fmt.Errorf("line %d is an empty continuation target", lineNumber)
		}
		if continued == "" && (trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ":")) {
			continue
		}

		withoutTrailingSpace := strings.TrimRight(line, " \t")
		hasContinuation := strings.HasSuffix(withoutTrailingSpace, "\\")
		segment := strings.TrimSpace(strings.TrimSuffix(withoutTrailingSpace, "\\"))
		if segment == "" {
			return nil, fmt.Errorf("line %d has no command before a continuation marker", lineNumber)
		}
		if continued != "" {
			continued += " " + segment
		} else {
			continued = segment
		}
		// Keep collecting physical lines until the iPXE continuation ends, then
		// validate the reconstructed logical command.
		if hasContinuation {
			continue
		}

		command := strings.Join(strings.Fields(continued), " ")
		fields := strings.Fields(command)
		if len(fields) == 0 {
			return nil, fmt.Errorf("line %d produced an empty iPXE command", lineNumber)
		}
		if !allowedCommands[fields[0]] {
			return nil, fmt.Errorf("line %d uses unknown iPXE command %q", lineNumber, fields[0])
		}
		commands = append(commands, command)
		continued = ""
	}
	if continued != "" {
		return nil, fmt.Errorf("iPXE script ended with an unterminated line continuation")
	}
	return commands, nil
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

func paramsWithStructuredUsers(params map[string]interface{}) map[string]interface{} {
	return mappings.ParamsWithUsers(params, structuredRenderUsers)
}

func boolPtr(value bool) *bool {
	return &value
}

func encryptedDebianParams(storage mappings.StorageConfig) map[string]interface{} {
	enabled := true
	if storage.Encryption.Enabled == nil {
		storage.Encryption.Enabled = &enabled
	}
	if storage.Encryption.Passphrase == nil {
		storage.Encryption.Passphrase = "luks-passphrase"
	}
	return mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Storage: storage,
		Boot:    mappings.BootConfig{Firmware: "uefi"},
	})
}

func encryptedDebianTPMParams() map[string]interface{} {
	return encryptedDebianParams(mappings.StorageConfig{
		Encryption: mappings.StorageEncryptionConfig{
			TPM: mappings.StorageEncryptionTPMConfig{
				Enabled:           boolPtr(true),
				Device:            "auto",
				PCRs:              "7",
				RequireSHA256Bank: boolPtr(true),
				Initramfs:         "dracut",
			},
		},
	})
}

func installerLateCommandsParams(storage mappings.StorageConfig) map[string]interface{} {
	return mappings.ParamsWithProvisioning(defaultRenderParams, nil, mappings.ProvisioningConfig{
		Installer: mappings.InstallerConfig{
			LateCommands: []string{
				"in-target systemctl enable ssh",
				"in-target touch /root/from-late-command",
			},
		},
		Storage: storage,
		Boot:    mappings.BootConfig{Firmware: "uefi"},
	})
}

func encryptedUnsupportedParams(base map[string]interface{}) map[string]interface{} {
	enabled := true
	return mappings.ParamsWithProvisioning(base, nil, mappings.ProvisioningConfig{
		Storage: mappings.StorageConfig{
			Encryption: mappings.StorageEncryptionConfig{
				Enabled:    &enabled,
				Passphrase: "luks-passphrase",
			},
		},
	})
}

func resolveExampleTarget(t *testing.T, resolver *mappings.Resolver, targetName, envName, envValue string) mappings.ResolveResult {
	t.Helper()

	result, err := resolver.Resolve(mappings.ResolveRequest{
		ManualTarget: targetName,
		GeneratedParams: map[string]any{
			"baseURL":  "shoelaces.example.test:8081",
			"hostname": targetName + "-example",
		},
		EnvLookup: func(name string) (string, bool) {
			if name == envName {
				return envValue, true
			}
			return "", false
		},
	})
	require.NoError(t, err)
	return result
}

func paramsForResolvedTarget(result mappings.ResolveResult) map[string]interface{} {
	return mappings.ParamsWithProvisioning(result.Params, result.Users, result.Provisioning)
}
