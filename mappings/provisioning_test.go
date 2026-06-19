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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamsWithProvisioningProjectsStructuredValuesBeforeDefaults(t *testing.T) {
	timeout := 9
	utc := false
	params := ParamsWithProvisioning(map[string]interface{}{
		"storage_disk": "/dev/explicit",
	}, nil, ProvisioningConfig{
		Locale: LocaleConfig{
			Language: "en_GB.UTF-8",
			Keyboard: "gb",
		},
		Time: TimeConfig{
			Timezone: "Europe/London",
			UTC:      &utc,
		},
		Storage: StorageConfig{
			Disk:        "/dev/structured",
			VolumeGroup: "vgstructured",
		},
		Boot: BootConfig{
			Netboot: NetbootConfig{
				KernelArgs: []string{"console=ttyS1"},
			},
			Installed: InstalledBootConfig{
				TimeoutSeconds: &timeout,
				KernelArgs:     []string{"panic=30"},
			},
		},
		Repos: ReposConfig{
			OSMirror: "https://mirror.example/os",
			Release:  "trixie",
		},
		Installer: InstallerConfig{
			ConfigTemplate: "preseed/custom",
			ConfigParams: map[string]any{
				"encrypt_home": true,
				"token":        "abc",
			},
		},
	})

	assert.Equal(t, "/dev/explicit", params["storage_disk"])
	assert.Equal(t, "/dev/explicit", params["storage_wipe_disks"])
	assert.Equal(t, "regular", params["storage_mode"])
	assert.Equal(t, "vgstructured", params["vg_name"])
	assert.Equal(t, "en_GB.UTF-8", params["locale_language"])
	assert.Equal(t, "gb", params["locale_keyboard"])
	assert.Equal(t, "Europe/London", params["time_timezone"])
	assert.Equal(t, "false", params["time_utc"])
	assert.Equal(t, "", params["kickstart_utc_flag"])
	assert.Equal(t, "console=ttyS1", params["linuxargs"])
	assert.Equal(t, "panic=30", params["boot_installed_args"])
	assert.Equal(t, "9", params["boot_timeout_seconds"])
	assert.Equal(t, "https://mirror.example/os", params["repo_debian_mirror"])
	assert.Equal(t, "trixie", params["release"])
	assert.Equal(t, "preseed/custom", params["debian_installer_config_template"])
	assert.Equal(t, true, params["encrypt_home"])
	assert.Equal(t, "abc", params["token"])
	assert.Equal(t, "", params["boot_ref_query"])
	assert.Equal(t, "", params["boot_ref_query_suffix"])
	assert.Equal(t, "", params["boot_ref_query_question"])
	assert.Equal(t, "auto", params["iface"])
	assert.Equal(t, "", params["installerExtra"])
}

func TestParamsWithProvisioningProjectsStructuredWipeDiskPatterns(t *testing.T) {
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Disk:             "/dev/nvme0n1",
			WipeDiskPatterns: []string{"/dev/nvme*n*", "/dev/sd*"},
		},
	})

	assert.Equal(t, "/dev/nvme0n1", params["storage_disk"])
	assert.Equal(t, "/dev/nvme*n* /dev/sd*", params["storage_wipe_disks"])
}

func TestParamsWithProvisioningPreservesExplicitWipeDiskParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		config InstallerConfig
	}{
		{
			name: "explicit render params",
			params: map[string]interface{}{
				"storage_wipe_disks": "/dev/explicit*",
			},
		},
		{
			name: "installer config params",
			config: InstallerConfig{
				ConfigParams: map[string]any{
					"storage_wipe_disks": "/dev/config*",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ParamsWithProvisioning(tt.params, nil, ProvisioningConfig{
				Storage: StorageConfig{
					Disk:             "/dev/nvme0n1",
					WipeDiskPatterns: []string{"/dev/nvme*n*"},
				},
				Installer: tt.config,
			})

			if tt.params != nil {
				assert.Equal(t, "/dev/explicit*", params["storage_wipe_disks"])
			} else {
				assert.Equal(t, "/dev/config*", params["storage_wipe_disks"])
			}
		})
	}
}

func TestParamsWithProvisioningKeepsDefaultWipeDiskFallback(t *testing.T) {
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{})

	assert.Equal(t, "/dev/nvme0n1", params["storage_disk"])
	assert.Equal(t, "/dev/nvme0n1 /dev/nvme1n1", params["storage_wipe_disks"])
	assert.Equal(t, "regular", params["storage_mode"])
}

func TestParamsWithProvisioningProjectsStructuredStorageMode(t *testing.T) {
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Mode: "lvm",
		},
	})

	assert.Equal(t, "lvm", params["storage_mode"])
}

func TestParamsWithProvisioningProjectsStructuredStorageEncryption(t *testing.T) {
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Encryption: StorageEncryptionConfig{
				Enabled:    boolPtr(true),
				Passphrase: "luks-passphrase",
				Cipher:     "xchacha12,aes-adiantum-plain64",
				KeySize:    intPtr(256),
				Hash:       "sha256",
			},
		},
	})

	assert.Equal(t, "true", params["storage_encryption_enabled"])
	assert.Equal(t, "luks-passphrase", params["storage_encryption_passphrase"])
	assert.Equal(t, "xchacha12,aes-adiantum-plain64", params["storage_encryption_cipher"])
	assert.Equal(t, "256", params["storage_encryption_key_size"])
	assert.Equal(t, "sha256", params["storage_encryption_hash"])
}

func TestParamsWithProvisioningProjectsStructuredRAID(t *testing.T) {
	bootDegraded := true
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Mode: "raid",
			RAID: RAIDConfig{
				Level: 1,
				Devices: []string{
					"/dev/disk/by-id/nvme-os-a",
					"/dev/disk/by-id/nvme-os-b",
				},
				BootDegraded: &bootDegraded,
			},
		},
	})

	assert.Equal(t, "raid", params["storage_mode"])
	assert.Equal(t, "1", params["storage_raid_level"])
	assert.Equal(t, "/dev/disk/by-id/nvme-os-a /dev/disk/by-id/nvme-os-b", params["storage_raid_devices"])
	assert.Equal(t, "/dev/disk/by-id/nvme-os-a /dev/disk/by-id/nvme-os-b", params["storage_wipe_disks"])
	assert.Equal(t, "/dev/disk/by-id/nvme-os-a", params["storage_raid_device_0"])
	assert.Equal(t, "/dev/disk/by-id/nvme-os-b", params["storage_raid_device_1"])
	assert.Equal(t, "true", params["storage_raid_boot_degraded"])
}

func TestParamsWithProvisioningProjectsStructuredRAIDFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Mode: "raid",
			Filesystems: map[string]FilesystemConfig{
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

	assert.Equal(t, "mdadm-udeb partman-md partman-xfs", params["debian_raid_partman_modules"])
	assert.Equal(t, "768", params["debian_raid_esp_min_size_mib"])
	assert.Equal(t, "768", params["debian_raid_esp_priority"])
	assert.Equal(t, "768", params["debian_raid_esp_max_size_mib"])
	assert.Equal(t, "/efi", params["debian_raid_esp_mountpoint"])
	assert.Equal(t, "2048", params["debian_raid_boot_min_size_mib"])
	assert.Equal(t, "2048", params["debian_raid_boot_priority"])
	assert.Equal(t, "2048", params["debian_raid_boot_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_raid_boot_fstype"])
	assert.Equal(t, "4096", params["debian_raid_swap_min_size_mib"])
	assert.Equal(t, "4096", params["debian_raid_swap_priority"])
	assert.Equal(t, "4096", params["debian_raid_swap_max_size_mib"])
	assert.Equal(t, "65536", params["debian_raid_root_min_size_mib"])
	assert.Equal(t, "100000000", params["debian_raid_root_priority"])
	assert.Equal(t, "-1", params["debian_raid_root_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_raid_root_fstype"])
}

func TestCopyProvisioningConfigCopiesRAID(t *testing.T) {
	bootDegraded := true
	original := ProvisioningConfig{
		Storage: StorageConfig{
			RAID: RAIDConfig{
				Level:        1,
				Devices:      []string{"/dev/nvme0n1", "/dev/nvme1n1"},
				BootDegraded: &bootDegraded,
			},
		},
	}

	copied := copyProvisioningConfig(original)
	copied.Storage.RAID.Devices[0] = "/dev/mutated"
	*copied.Storage.RAID.BootDegraded = false

	assert.Equal(t, []string{"/dev/nvme0n1", "/dev/nvme1n1"}, original.Storage.RAID.Devices)
	require.NotNil(t, original.Storage.RAID.BootDegraded)
	assert.True(t, *original.Storage.RAID.BootDegraded)
}

func TestParamsWithProvisioningProjectsStructuredRegularFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Filesystems: map[string]FilesystemConfig{
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

	assert.Equal(t, "partman-xfs", params["debian_regular_partman_modules"])
	assert.Equal(t, "768", params["debian_regular_esp_min_size_mib"])
	assert.Equal(t, "768", params["debian_regular_esp_priority"])
	assert.Equal(t, "768", params["debian_regular_esp_max_size_mib"])
	assert.Equal(t, "/efi", params["debian_regular_esp_mountpoint"])
	assert.Equal(t, "2048", params["debian_regular_boot_min_size_mib"])
	assert.Equal(t, "2048", params["debian_regular_boot_priority"])
	assert.Equal(t, "2048", params["debian_regular_boot_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_regular_boot_fstype"])
	assert.Equal(t, "4096", params["debian_regular_swap_min_size_mib"])
	assert.Equal(t, "4096", params["debian_regular_swap_priority"])
	assert.Equal(t, "4096", params["debian_regular_swap_max_size_mib"])
	assert.Equal(t, "65536", params["debian_regular_root_min_size_mib"])
	assert.Equal(t, "100000000", params["debian_regular_root_priority"])
	assert.Equal(t, "-1", params["debian_regular_root_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_regular_root_fstype"])
}

func TestParamsWithProvisioningInfersMixedRegularPartmanModules(t *testing.T) {
	rootMinSize := 65536
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Filesystems: map[string]FilesystemConfig{
				"root": {
					FSType:  "xfs",
					Size:    "grow",
					SizeMiB: &rootMinSize,
				},
			},
		},
	})

	assert.Equal(t, "partman-ext4 partman-xfs", params["debian_regular_partman_modules"])
	assert.Equal(t, "ext4", params["debian_regular_boot_fstype"])
	assert.Equal(t, "xfs", params["debian_regular_root_fstype"])
}

func TestParamsWithProvisioningProjectsStructuredLVMFilesystems(t *testing.T) {
	espSize := 768
	bootSize := 2048
	swapSize := 4096
	rootMinSize := 65536
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Filesystems: map[string]FilesystemConfig{
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

	assert.Equal(t, "lvm2-udeb partman-lvm partman-xfs", params["debian_lvm_partman_modules"])
	assert.Equal(t, "768", params["debian_lvm_esp_min_size_mib"])
	assert.Equal(t, "768", params["debian_lvm_esp_priority"])
	assert.Equal(t, "768", params["debian_lvm_esp_max_size_mib"])
	assert.Equal(t, "/efi", params["debian_lvm_esp_mountpoint"])
	assert.Equal(t, "2048", params["debian_lvm_boot_min_size_mib"])
	assert.Equal(t, "2048", params["debian_lvm_boot_priority"])
	assert.Equal(t, "2048", params["debian_lvm_boot_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_lvm_boot_fstype"])
	assert.Equal(t, "4096", params["debian_lvm_swap_min_size_mib"])
	assert.Equal(t, "4096", params["debian_lvm_swap_priority"])
	assert.Equal(t, "4096", params["debian_lvm_swap_max_size_mib"])
	assert.Equal(t, "65536", params["debian_lvm_root_min_size_mib"])
	assert.Equal(t, "100000000", params["debian_lvm_root_priority"])
	assert.Equal(t, "-1", params["debian_lvm_root_max_size_mib"])
	assert.Equal(t, "xfs", params["debian_lvm_root_fstype"])
}

func TestParamsWithProvisioningInfersLVMPartmanModulesFromConfigParamFilesystems(t *testing.T) {
	params := ParamsWithProvisioning(nil, nil, ProvisioningConfig{
		Storage: StorageConfig{
			Mode: "lvm",
		},
		Installer: InstallerConfig{
			ConfigParams: map[string]any{
				"debian_lvm_root_fstype": "xfs",
			},
		},
	})

	assert.Equal(t, "lvm", params["storage_mode"])
	assert.Equal(t, "xfs", params["debian_lvm_root_fstype"])
	assert.Equal(t, "lvm2-udeb partman-lvm partman-ext4 partman-xfs", params["debian_lvm_partman_modules"])
}
