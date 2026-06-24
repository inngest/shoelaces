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
  locale:
    language: en_US.UTF-8
    keyboard: us
  time:
    timezone: UTC
    utc: true
  network:
    bootproto: dhcp
    nameservers:
      - 1.1.1.1
  packages:
    install:
      - openssh-server
    groups:
      - core
  storage:
    mode: lvm
    volumeGroup: vg0
    wipeDiskPatterns:
      - /dev/nvme*n*
    encryption:
      enabled: true
      passphrase:
        env: SHOELACES_LUKS_PASSPHRASE
      cipher: aes-xts-plain64
      keySize: 512
      hash: sha512
    filesystems:
      root:
        mountpoint: /
        fstype: ext4
        size: grow
  boot:
    firmware: uefi
    netboot:
      method: ipxe
      kernelArgs:
        - console=ttyS0
    installed:
      bootloader: grub
      timeoutSeconds: 5
  repos:
    osMirror: https://deb.debian.org/debian
    release: bookworm
    firmware: true
  installer:
    configTemplate: preseed/debian
    extraTemplate: provisioning/extra
    configParams:
      encrypt_home: false
  users:
    root:
      locked: true
      passwordCrypted:
        env: SHOELACES_ROOT_PASSWORD_CRYPTED
    infra:
      primary: true
      fullName: Infrastructure User
      sshAuthorizedKeys:
        - "ssh-ed25519 AAAA example"
        - env: SHOELACES_INFRA_SSH_KEY
      groups:
        - sudo
      shell: /bin/bash
      sudo: "ALL=(ALL) NOPASSWD:ALL"
targets:
  debian12:
    script: debian.ipxe
    label: Debian 12 Bookworm
    environment: testing
    params:
      release: bookworm
    packages:
      install:
        - curl
    repos:
      release: bookworm
    users:
      infra:
        fullName: Debian Infrastructure User
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
    networkConfig:
      hostname: net-host
    storage:
      disk: /dev/nvme0n1
      wipeDiskPatterns:
        - /dev/disk/by-id/inngest-*
    users:
      breakglass:
        locked: true
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
	assert.Equal(t, "en_US.UTF-8", parsed.Defaults.Locale.Language)
	assert.Equal(t, "us", parsed.Defaults.Locale.Keyboard)
	assert.Equal(t, "UTC", parsed.Defaults.Time.Timezone)
	require.NotNil(t, parsed.Defaults.Time.UTC)
	assert.True(t, *parsed.Defaults.Time.UTC)
	assert.Equal(t, "dhcp", parsed.Defaults.Network.Bootproto)
	assert.Equal(t, []string{"1.1.1.1"}, parsed.Defaults.Network.Nameservers)
	assert.Equal(t, []string{"openssh-server"}, parsed.Defaults.Packages.Install)
	assert.Equal(t, []string{"core"}, parsed.Defaults.Packages.Groups)
	assert.Equal(t, "lvm", parsed.Defaults.Storage.Mode)
	assert.Equal(t, "vg0", parsed.Defaults.Storage.VolumeGroup)
	assert.Equal(t, []string{"/dev/nvme*n*"}, parsed.Defaults.Storage.WipeDiskPatterns)
	require.NotNil(t, parsed.Defaults.Storage.Encryption.Enabled)
	assert.True(t, *parsed.Defaults.Storage.Encryption.Enabled)
	assert.Equal(t, map[string]any{"env": "SHOELACES_LUKS_PASSPHRASE"}, parsed.Defaults.Storage.Encryption.Passphrase)
	assert.Equal(t, "aes-xts-plain64", parsed.Defaults.Storage.Encryption.Cipher)
	require.NotNil(t, parsed.Defaults.Storage.Encryption.KeySize)
	assert.Equal(t, 512, *parsed.Defaults.Storage.Encryption.KeySize)
	assert.Equal(t, "sha512", parsed.Defaults.Storage.Encryption.Hash)
	assert.Equal(t, "/", parsed.Defaults.Storage.Filesystems["root"].Mountpoint)
	assert.Equal(t, "uefi", parsed.Defaults.Boot.Firmware)
	assert.Equal(t, "ipxe", parsed.Defaults.Boot.Netboot.Method)
	assert.Equal(t, []string{"console=ttyS0"}, parsed.Defaults.Boot.Netboot.KernelArgs)
	assert.Equal(t, "grub", parsed.Defaults.Boot.Installed.Bootloader)
	require.NotNil(t, parsed.Defaults.Boot.Installed.TimeoutSeconds)
	assert.Equal(t, 5, *parsed.Defaults.Boot.Installed.TimeoutSeconds)
	assert.Equal(t, "https://deb.debian.org/debian", parsed.Defaults.Repos.OSMirror)
	assert.Equal(t, "bookworm", parsed.Defaults.Repos.Release)
	require.NotNil(t, parsed.Defaults.Repos.Firmware)
	assert.True(t, *parsed.Defaults.Repos.Firmware)
	assert.Equal(t, "preseed/debian", parsed.Defaults.Installer.ConfigTemplate)
	assert.Equal(t, "provisioning/extra", parsed.Defaults.Installer.ExtraTemplate)
	assert.Equal(t, false, parsed.Defaults.Installer.ConfigParams["encrypt_home"])
	require.NotNil(t, parsed.Defaults.Users["root"].Locked)
	assert.True(t, *parsed.Defaults.Users["root"].Locked)
	assert.Equal(t, map[string]any{"env": "SHOELACES_ROOT_PASSWORD_CRYPTED"}, parsed.Defaults.Users["root"].PasswordCrypted)
	require.NotNil(t, parsed.Defaults.Users["infra"].Primary)
	assert.True(t, *parsed.Defaults.Users["infra"].Primary)
	assert.Equal(t, "Infrastructure User", parsed.Defaults.Users["infra"].FullName)
	assert.Equal(t, []any{"ssh-ed25519 AAAA example", map[string]any{"env": "SHOELACES_INFRA_SSH_KEY"}}, parsed.Defaults.Users["infra"].SSHAuthorizedKeys)
	assert.Equal(t, []string{"sudo"}, parsed.Defaults.Users["infra"].Groups)
	assert.Equal(t, "/bin/bash", parsed.Defaults.Users["infra"].Shell)
	assert.Equal(t, "ALL=(ALL) NOPASSWD:ALL", parsed.Defaults.Users["infra"].Sudo)
	assert.Equal(t, "debian.ipxe", parsed.Targets["debian12"].Script)
	assert.Equal(t, "Debian 12 Bookworm", parsed.Targets["debian12"].Label)
	assert.Equal(t, "testing", parsed.Targets["debian12"].Environment)
	assert.Equal(t, "bookworm", parsed.Targets["debian12"].Params["release"])
	assert.Equal(t, []string{"curl"}, parsed.Targets["debian12"].Packages.Install)
	assert.Equal(t, "bookworm", parsed.Targets["debian12"].Repos.Release)
	assert.Equal(t, "Debian Infrastructure User", parsed.Targets["debian12"].Users["infra"].FullName)
	require.Len(t, parsed.NetworkMaps, 1)
	assert.Equal(t, "debian12", parsed.NetworkMaps[0].DefaultTarget)
	assert.Equal(t, []string{"debian12", "debian13"}, parsed.NetworkMaps[0].Targets)
	assert.Equal(t, "net-host", parsed.NetworkMaps[0].NetworkSettings.Hostname)
	assert.Equal(t, "/dev/nvme0n1", parsed.NetworkMaps[0].Storage.Disk)
	assert.Equal(t, []string{"/dev/disk/by-id/inngest-*"}, parsed.NetworkMaps[0].Storage.WipeDiskPatterns)
	require.NotNil(t, parsed.NetworkMaps[0].Users["breakglass"].Locked)
	assert.True(t, *parsed.NetworkMaps[0].Users["breakglass"].Locked)
	require.Len(t, parsed.HostnameMaps, 1)
	require.Len(t, parsed.MacMaps, 1)
	require.Len(t, parsed.IPMaps, 1)
}

func TestParseMappingsLoadsRepositoryExample(t *testing.T) {
	parsed, err := ParseMappings(log.MakeLogger(io.Discard), filepath.Join("..", "configs", "data-dir", "mappings.yaml"))

	require.NoError(t, err)
	assert.NotEmpty(t, parsed.Targets)
	assert.NotEmpty(t, parsed.NetworkMaps)
	assert.Equal(t, "trixie", parsed.Defaults.Repos.Release)
	assert.Equal(t, "bookworm", parsed.Targets["debian12"].Repos.Release)
	assert.Equal(t, "trixie", parsed.Targets["debian13"].Repos.Release)
	assert.Equal(t, "debian.ipxe", parsed.Targets["debian13-luks"].Script)
	assert.Equal(t, "regular", parsed.Targets["debian13-luks"].Storage.Mode)
	require.NotNil(t, parsed.Targets["debian13-luks"].Storage.Encryption.Enabled)
	assert.True(t, *parsed.Targets["debian13-luks"].Storage.Encryption.Enabled)
	assert.Equal(t, map[string]any{"env": "SHOELACES_LUKS_PASSPHRASE"}, parsed.Targets["debian13-luks"].Storage.Encryption.Passphrase)
	assert.Equal(t, "debian13", parsed.NetworkMaps[0].DefaultTarget)
	assert.Contains(t, parsed.NetworkMaps[0].Targets, "debian13-luks")
	assert.Equal(t, "debian13", parsed.IPMaps[0].DefaultTarget)
}

func TestParseMappingsLoadsDevelopmentExample(t *testing.T) {
	parsed, err := ParseMappings(log.MakeLogger(io.Discard), filepath.Join("..", "dev", "data-dir", "mappings.yaml"))

	require.NoError(t, err)
	assert.Equal(t, "trixie", parsed.Defaults.Repos.Release)
	assert.Equal(t, "bookworm", parsed.Targets["debian12"].Repos.Release)
	assert.Equal(t, "trixie", parsed.Targets["debian13"].Repos.Release)
	assert.Equal(t, "debian.ipxe", parsed.Targets["debian13-luks"].Script)
	require.NotNil(t, parsed.Targets["debian13-luks"].Storage.Encryption.Enabled)
	assert.True(t, *parsed.Targets["debian13-luks"].Storage.Encryption.Enabled)
	assert.Equal(t, map[string]any{"env": "SHOELACES_DEV_LUKS_PASSPHRASE"}, parsed.Targets["debian13-luks"].Storage.Encryption.Passphrase)
	assert.Equal(t, "debian13", parsed.NetworkMaps[0].DefaultTarget)
	assert.Contains(t, parsed.NetworkMaps[0].Targets, "debian13-luks")
}

func TestParseMappingsAcceptsRegularStorageMode(t *testing.T) {
	mappingsPath := writeMappingsFile(t, `
targets:
  debian12:
    script: debian.ipxe
    storage:
      mode: regular
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
`)

	parsed, err := ParseMappings(log.MakeLogger(io.Discard), mappingsPath)

	require.NoError(t, err)
	assert.Equal(t, "regular", parsed.Targets["debian12"].Storage.Mode)
}

func TestParseMappingsLoadsStructuredRAIDStorage(t *testing.T) {
	mappingsPath := writeMappingsFile(t, `
targets:
  debian12:
    script: debian.ipxe
    boot:
      firmware: uefi
    storage:
      mode: raid
      raid:
        level: 1
        devices:
          - /dev/nvme0n1
          - /dev/nvme1n1
        bootDegraded: true
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
`)

	parsed, err := ParseMappings(log.MakeLogger(io.Discard), mappingsPath)

	require.NoError(t, err)
	assert.Equal(t, "raid", parsed.Targets["debian12"].Storage.Mode)
	assert.Equal(t, 1, parsed.Targets["debian12"].Storage.RAID.Level)
	assert.Equal(t, []string{"/dev/nvme0n1", "/dev/nvme1n1"}, parsed.Targets["debian12"].Storage.RAID.Devices)
	require.NotNil(t, parsed.Targets["debian12"].Storage.RAID.BootDegraded)
	assert.True(t, *parsed.Targets["debian12"].Storage.RAID.BootDegraded)
}

func TestParseMappingsLoadsStructuredStorageEncryption(t *testing.T) {
	mappingsPath := writeMappingsFile(t, `
targets:
  debian13:
    script: debian.ipxe
    storage:
      mode: lvm
      encryption:
        enabled: true
        passphrase: lab-passphrase
        cipher: aes-xts-plain64
        keySize: 512
        hash: sha512
        tpm:
          enabled: true
          device: auto
          pcrs: "7"
          requireSha256Bank: true
          initramfs: dracut
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian13
    targets:
      - debian13
`)

	parsed, err := ParseMappings(log.MakeLogger(io.Discard), mappingsPath)

	require.NoError(t, err)
	encryption := parsed.Targets["debian13"].Storage.Encryption
	require.NotNil(t, encryption.Enabled)
	assert.True(t, *encryption.Enabled)
	assert.Equal(t, "lab-passphrase", encryption.Passphrase)
	assert.Equal(t, "aes-xts-plain64", encryption.Cipher)
	require.NotNil(t, encryption.KeySize)
	assert.Equal(t, 512, *encryption.KeySize)
	assert.Equal(t, "sha512", encryption.Hash)
	require.NotNil(t, encryption.TPM.Enabled)
	assert.True(t, *encryption.TPM.Enabled)
	assert.Equal(t, "auto", encryption.TPM.Device)
	assert.Equal(t, "7", encryption.TPM.PCRs)
	require.NotNil(t, encryption.TPM.RequireSHA256Bank)
	assert.True(t, *encryption.TPM.RequireSHA256Bank)
	assert.Equal(t, "dracut", encryption.TPM.Initramfs)
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
		{
			name: "invalid storage mode",
			content: `
targets:
  debian12:
    script: debian.ipxe
    storage:
      mode: zfs
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
`,
			want: `targets["debian12"].storage.mode has unsupported value "zfs"`,
		},
		{
			name: "invalid package list",
			content: `
targets:
  debian12:
    script: debian.ipxe
    packages:
      install:
        - ""
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
`,
			want: `targets["debian12"].packages.install[0] must not be empty`,
		},
		{
			name: "invalid wipe disk pattern outside dev",
			content: `
defaults:
  storage:
    wipeDiskPatterns:
      - /tmp/nvme*
targets:
  debian12:
    script: debian.ipxe
`,
			want: `defaults.storage.wipeDiskPatterns[0] must start with /dev/`,
		},
		{
			name: "invalid wipe disk pattern shell metacharacter",
			content: `
defaults:
  storage:
    wipeDiskPatterns:
      - /dev/nvme*;rm
targets:
  debian12:
    script: debian.ipxe
`,
			want: `defaults.storage.wipeDiskPatterns[0] contains unsupported shell metacharacters`,
		},
		{
			name: "invalid wipe disk pattern dev catch all",
			content: `
defaults:
  storage:
    wipeDiskPatterns:
      - /dev/*
targets:
  debian12:
    script: debian.ipxe
`,
			want: `defaults.storage.wipeDiskPatterns[0] must be narrower than /dev/*`,
		},
		{
			name: "invalid raid level",
			content: `
targets:
  debian12:
    script: debian.ipxe
    storage:
      mode: raid
      raid:
        level: 5
        devices:
          - /dev/nvme0n1
          - /dev/nvme1n1
`,
			want: `targets["debian12"].storage.raid.level must be 1 for Debian RAID mode`,
		},
		{
			name: "invalid raid device count",
			content: `
targets:
  debian12:
    script: debian.ipxe
    storage:
      mode: raid
      raid:
        level: 1
        devices:
          - /dev/nvme0n1
`,
			want: `targets["debian12"].storage.raid.devices must contain exactly 2 devices for Debian RAID mode`,
		},
		{
			name: "invalid raid device outside dev",
			content: `
targets:
  debian12:
    script: debian.ipxe
    storage:
      raid:
        devices:
          - nvme0n1
          - /dev/nvme1n1
`,
			want: `targets["debian12"].storage.raid.devices[0] must start with /dev/`,
		},
		{
			name: "invalid raid device glob",
			content: `
targets:
  debian12:
    script: debian.ipxe
    storage:
      raid:
        devices:
          - /dev/nvme*
          - /dev/nvme1n1
`,
			want: `targets["debian12"].storage.raid.devices[0] must be an explicit /dev path without glob patterns`,
		},
		{
			name: "raid rejects bios firmware",
			content: `
targets:
  debian12:
    script: debian.ipxe
    boot:
      firmware: bios
    storage:
      mode: raid
      raid:
        level: 1
        devices:
          - /dev/nvme0n1
          - /dev/nvme1n1
`,
			want: `targets["debian12"].boot.firmware must be "uefi" when storage.mode is raid`,
		},
		{
			name: "encryption passphrase must be non-empty",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase: ""
`,
			want: `targets["debian13"].storage.encryption.passphrase must not be empty`,
		},
		{
			name: "encryption passphrase env reference must be valid",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase:
          env: ""
`,
			want: `targets["debian13"].storage.encryption.passphrase: environment reference "env" must be a non-empty string`,
		},
		{
			name: "encryption passphrase rejects unsupported shape",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase:
          secretRef: luks
`,
			want: `targets["debian13"].storage.encryption.passphrase must be a string or { env: ENV_VAR } reference`,
		},
		{
			name: "encryption key size must be positive",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase: lab-passphrase
        keySize: 0
`,
			want: `targets["debian13"].storage.encryption.keySize must be greater than 0`,
		},
		{
			name: "encryption TPM initramfs must be dracut",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase: lab-passphrase
        tpm:
          enabled: true
          initramfs: initramfs-tools
`,
			want: `targets["debian13"].storage.encryption.tpm.initramfs has unsupported value "initramfs-tools"`,
		},
		{
			name: "encryption TPM pcrs must be safe",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase: lab-passphrase
        tpm:
          enabled: true
          pcrs: "7;reboot"
`,
			want: `targets["debian13"].storage.encryption.tpm.pcrs contains unsupported shell metacharacters`,
		},
		{
			name: "encryption TPM device must be safe",
			content: `
targets:
  debian13:
    script: debian.ipxe
    storage:
      encryption:
        enabled: true
        passphrase: lab-passphrase
        tpm:
          enabled: true
          device: "auto;reboot"
`,
			want: `targets["debian13"].storage.encryption.tpm.device contains unsupported shell metacharacters`,
		},
		{
			name: "invalid mirror url",
			content: `
targets:
  debian12:
    script: debian.ipxe
    repos:
      osMirror: ftp://example.test/debian
networkMaps:
  - network: 192.0.2.0/24
    defaultTarget: debian12
    targets:
      - debian12
`,
			want: `targets["debian12"].repos.osMirror must be an http or https URL`,
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
