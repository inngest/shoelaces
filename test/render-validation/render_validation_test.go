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

func TestRenderedDebianPreseedRejectsUnsupportedStorageMode(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "explicit query style param",
			params: paramsWith(defaultRenderParams, "storage_mode", "zfs"),
			want:   `preseed/debian storage_mode must be "regular" or "lvm", got "zfs"`,
		},
		{
			name: "structured provisioning",
			params: paramsWith(defaultRenderParams, "provisioning", mappings.ProvisioningConfig{
				Storage: mappings.StorageConfig{
					Mode: "zfs",
				},
			}),
			want: `preseed/debian storage_mode must be "regular" or "lvm", got "zfs"`,
		},
		{
			name:   "legacy plain mode",
			params: paramsWith(defaultRenderParams, "storage_mode", "plain"),
			want:   `preseed/debian storage_mode must be "regular" or "lvm", got "plain"`,
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

	renderer := templates.New(log.MakeLogger(io.Discard))
	renderer.ParseTemplates(t.TempDir(), "env_overrides", nil, ".slc")
	return renderer
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

func assertValidIPXEScript(t *testing.T, rendered string) []string {
	t.Helper()

	commands, err := validateIPXEScript(rendered)
	require.NoError(t, err)
	return commands
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
