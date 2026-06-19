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

func TestResolverMatchPrecedence(t *testing.T) {
	resolver := newTestResolver(t)

	tests := []struct {
		name       string
		request    ResolveRequest
		wantType   MatchType
		wantTarget string
	}{
		{
			name: "manual target wins",
			request: ResolveRequest{
				Mac:          "0c:42:a1:c3:52:96",
				IP:           "192.0.2.10",
				Hostname:     "host-1",
				ManualTarget: "debian13",
			},
			wantType:   MatchManual,
			wantTarget: "debian13",
		},
		{
			name: "mac wins over ip hostname and network",
			request: ResolveRequest{
				Mac:      "0c:42:a1:c3:52:96",
				IP:       "192.0.2.10",
				Hostname: "host-1",
			},
			wantType:   MatchMAC,
			wantTarget: "debian13",
		},
		{
			name: "ip wins over hostname and network",
			request: ResolveRequest{
				Mac:      "0c:42:a1:c3:52:97",
				IP:       "192.0.2.10",
				Hostname: "host-1",
			},
			wantType:   MatchIP,
			wantTarget: "ubuntu2404",
		},
		{
			name: "hostname wins over network",
			request: ResolveRequest{
				Mac:      "0c:42:a1:c3:52:97",
				IP:       "198.51.100.10",
				Hostname: "host-1",
			},
			wantType:   MatchHostname,
			wantTarget: "debian13",
		},
		{
			name: "network wins over unmatched",
			request: ResolveRequest{
				Mac:      "0c:42:a1:c3:52:97",
				IP:       "198.51.100.10",
				Hostname: "other",
			},
			wantType:   MatchNetwork,
			wantTarget: "debian12",
		},
		{
			name: "unmatched queues for manual selection",
			request: ResolveRequest{
				Mac:      "0c:42:a1:c3:52:97",
				IP:       "203.0.113.10",
				Hostname: "other",
			},
			wantType: MatchUnmatched,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(tt.request)

			require.NoError(t, err)
			assert.Equal(t, tt.wantType, result.MatchType)
			assert.Equal(t, tt.wantTarget, result.TargetName)
			assert.Equal(t, tt.wantTarget != "", result.HasTarget())
			if tt.wantTarget == "" {
				assert.True(t, result.RequiresManualSelection)
			}
		})
	}
}

func TestResolverExactIPMapsMatchIPv4AndIPv6(t *testing.T) {
	resolver := newTestResolver(t)

	tests := []struct {
		name       string
		ip         string
		wantTarget string
	}{
		{name: "ipv4 exact match", ip: "192.0.2.10", wantTarget: "ubuntu2404"},
		{name: "ipv6 exact match", ip: "2001:db8::99", wantTarget: "debian13"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:97", IP: tt.ip})

			require.NoError(t, err)
			assert.Equal(t, MatchIP, result.MatchType)
			assert.Equal(t, tt.wantTarget, result.TargetName)
		})
	}
}

func TestResolverMultipleTargetsAndNoDefaultManualSelection(t *testing.T) {
	resolver := newTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{
		Mac:      "0c:42:a1:c3:52:98",
		IP:       "2001:db8:1::10",
		Hostname: "other",
	})

	require.NoError(t, err)
	assert.Equal(t, MatchNetwork, result.MatchType)
	assert.Empty(t, result.TargetName)
	assert.True(t, result.RequiresManualSelection)
	assert.Equal(t, []string{"debian12", "debian13"}, result.AllowedTargetNames())
}

func TestResolverManualSelectionUsesAllowedTargets(t *testing.T) {
	resolver := newTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{
		Mac:          "0c:42:a1:c3:52:98",
		IP:           "2001:db8:1::10",
		ManualTarget: "debian13",
	})

	require.NoError(t, err)
	assert.Equal(t, MatchManual, result.MatchType)
	assert.Equal(t, "debian13", result.TargetName)
	assert.Equal(t, "debian.ipxe", result.Target.Script)
}

func TestResolverManualSelectionRejectsDisallowedTarget(t *testing.T) {
	resolver := newTestResolver(t)

	_, err := resolver.Resolve(ResolveRequest{
		Mac:          "0c:42:a1:c3:52:98",
		IP:           "2001:db8:1::10",
		ManualTarget: "ubuntu2404",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `manual target "ubuntu2404" is not allowed`)
}

func TestResolverManualSelectionAllowsAnyKnownTargetWhenUnmatched(t *testing.T) {
	resolver := newTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{
		Mac:          "0c:42:a1:c3:52:99",
		IP:           "203.0.113.10",
		ManualTarget: "ubuntu2404",
	})

	require.NoError(t, err)
	assert.Equal(t, MatchManual, result.MatchType)
	assert.Equal(t, "ubuntu2404", result.TargetName)
}

func TestResolverReturnsImmutableTargetSnapshots(t *testing.T) {
	resolver := newTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})
	require.NoError(t, err)
	result.Target.Params["release"] = "mutated"

	result, err = resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})
	require.NoError(t, err)
	assert.Equal(t, "trixie", result.Target.Params["release"])
}

func TestResolverResolvesParamsInMergeOrder(t *testing.T) {
	resolver := newTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		Params: map[string]any{
			"shared":          "request",
			"request_only":    "request",
			"install_disk":    "/dev/nvme0n1",
			"linuxargs":       "console=ttyS0",
			"install_retries": 3,
			"enable_ssh":      true,
		},
		GeneratedParams: map[string]any{
			"shared":   "generated",
			"hostname": "iad-1",
			"baseURL":  "http://shoelaces.example.com",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"install_username":       "infra",
		"default_only_parameter": "default",
		"release":                "trixie",
		"role":                   "mac",
		"shared":                 "generated",
		"request_only":           "request",
		"install_disk":           "/dev/nvme0n1",
		"linuxargs":              "console=ttyS0",
		"install_retries":        3,
		"enable_ssh":             true,
		"hostname":               "iad-1",
		"baseURL":                "http://shoelaces.example.com",
	}, result.Params)
}

func TestResolverStructuredRepoReleaseOverridesLegacyMappingParams(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Params: map[string]any{
				"release": "bookworm",
			},
			Repos: ReposConfig{
				Release: "bookworm",
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Repos: ReposConfig{
					Release: "trixie",
				},
			},
		},
		NetworkMaps: []NetworkMapConfig{{
			Network:       "192.0.2.0/24",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		IP: "192.0.2.10",
	})

	require.NoError(t, err)
	assert.Equal(t, "trixie", result.Params["release"])
	assert.Equal(t, "trixie", result.Provisioning.Repos.Release)
}

func TestResolverRequestParamsOverrideStructuredRepoRelease(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Repos: ReposConfig{
				Release: "bookworm",
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Repos: ReposConfig{
					Release: "trixie",
				},
			},
		},
		NetworkMaps: []NetworkMapConfig{{
			Network:       "192.0.2.0/24",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		IP: "192.0.2.10",
		Params: map[string]any{
			"release": "operator-release",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "operator-release", result.Params["release"])
	assert.Equal(t, "trixie", result.Provisioning.Repos.Release)
}

func TestResolverResolvesExplicitEnvironmentBackedParams(t *testing.T) {
	resolver := newEnvTestResolver(t)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(key string) (string, bool) {
			return map[string]string{
				"SHOELACES_ROOT_PASSWORD_CRYPTED": "$6$root",
			}[key], key == "SHOELACES_ROOT_PASSWORD_CRYPTED"
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "$6$root", result.Params["root_password_crypted"])
}

func TestResolverReturnsMissingEnvironmentVariableError(t *testing.T) {
	resolver := newEnvTestResolver(t)

	_, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "", false
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `parameter "root_password_crypted" references missing environment variable "SHOELACES_ROOT_PASSWORD_CRYPTED"`)
}

func TestResolverDoesNotLeakResolvedParamsBetweenBoots(t *testing.T) {
	resolver := newEnvTestResolver(t)

	first, err := resolver.Resolve(ResolveRequest{
		Mac:             "0c:42:a1:c3:52:96",
		GeneratedParams: map[string]any{"hostname": "first-host"},
		EnvLookup: func(string) (string, bool) {
			return "$6$first", true
		},
	})
	require.NoError(t, err)
	first.Params["hostname"] = "mutated"
	first.Params["root_password_crypted"] = "mutated"

	second, err := resolver.Resolve(ResolveRequest{
		Mac:             "0c:42:a1:c3:52:96",
		GeneratedParams: map[string]any{"hostname": "second-host"},
		EnvLookup: func(string) (string, bool) {
			return "$6$second", true
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "second-host", second.Params["hostname"])
	assert.Equal(t, "$6$second", second.Params["root_password_crypted"])
}

func TestResolverResolvesStructuredUsersInMergeOrder(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Users: map[string]UserConfig{
				"root": {
					Locked:            boolPtr(true),
					PasswordCrypted:   map[string]any{"env": "ROOT_PASSWORD_CRYPTED"},
					SSHAuthorizedKeys: []any{"ssh-ed25519 root-default"},
				},
				"infra": {
					Primary:           boolPtr(true),
					FullName:          "Default Infrastructure User",
					Locked:            boolPtr(true),
					PasswordCrypted:   "default-hash",
					SSHAuthorizedKeys: []any{"ssh-ed25519 default"},
					Groups:            []string{"sudo"},
				},
				"breakglass": {
					Locked: boolPtr(true),
				},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Users: map[string]UserConfig{
					"infra": {
						FullName:        "Target Infrastructure User",
						Locked:          boolPtr(false),
						PasswordCrypted: map[string]any{"env": "INFRA_PASSWORD_CRYPTED"},
						Groups:          []string{"sudo", "adm"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
				Users: map[string]UserConfig{
					"infra": {
						SSHAuthorizedKeys: []any{map[string]any{"env": "INFRA_SSH_KEY"}},
						Shell:             "/bin/bash",
					},
					"breakglass": {
						Absent: boolPtr(true),
					},
					"siteadmin": {
						FullName: "Site Admin",
						Locked:   boolPtr(true),
					},
				},
			},
		},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(key string) (string, bool) {
			values := map[string]string{
				"ROOT_PASSWORD_CRYPTED":  "$6$root",
				"INFRA_PASSWORD_CRYPTED": "$6$infra",
				"INFRA_SSH_KEY":          "ssh-ed25519 host",
			}
			value, ok := values[key]
			return value, ok
		},
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]ResolvedUser{
		"root": {
			Name:              "root",
			System:            true,
			Locked:            true,
			PasswordCrypted:   "$6$root",
			SSHAuthorizedKeys: []string{"ssh-ed25519 root-default"},
		},
		"infra": {
			Name:              "infra",
			Primary:           true,
			FullName:          "Target Infrastructure User",
			PasswordCrypted:   "$6$infra",
			SSHAuthorizedKeys: []string{"ssh-ed25519 host"},
			Groups:            []string{"sudo", "adm"},
			Shell:             "/bin/bash",
		},
		"siteadmin": {
			Name:     "siteadmin",
			FullName: "Site Admin",
			Locked:   true,
		},
	}, result.Users)
	assert.NotContains(t, result.Users, "breakglass")
}

func TestResolverReturnsMissingEnvironmentVariableErrorForStructuredUsers(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Users: map[string]UserConfig{
					"infra": {
						PasswordCrypted: map[string]any{"env": "INFRA_PASSWORD_CRYPTED"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
	})
	require.NoError(t, err)

	_, err = resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "", false
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `parameter "user \"infra\" passwordCrypted" references missing environment variable "INFRA_PASSWORD_CRYPTED"`)
}

func TestResolverRejectsMultiplePrimaryNonRootUsers(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Users: map[string]UserConfig{
				"infra": {Primary: boolPtr(true)},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Users: map[string]UserConfig{
					"ops": {Primary: boolPtr(true)},
				},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
	})
	require.NoError(t, err)

	_, err = resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `multiple primary non-root users configured: [infra ops]`)
}

func TestResolverDoesNotLeakResolvedUsersBetweenBoots(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Users: map[string]UserConfig{
					"infra": {
						FullName:        "Infrastructure User",
						PasswordCrypted: map[string]any{"env": "INFRA_PASSWORD_CRYPTED"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
	})
	require.NoError(t, err)

	first, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "$6$first", true
		},
	})
	require.NoError(t, err)
	mutated := first.Users["infra"]
	mutated.FullName = "mutated"
	mutated.PasswordCrypted = "mutated"
	first.Users["infra"] = mutated

	second, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "$6$second", true
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "Infrastructure User", second.Users["infra"].FullName)
	assert.Equal(t, "$6$second", second.Users["infra"].PasswordCrypted)
}

func TestResolverMergesStructuredProvisioningConfig(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Locale: LocaleConfig{Language: "en_US.UTF-8", Keyboard: "us"},
			Time:   TimeConfig{Timezone: "UTC", UTC: boolPtr(true), NTP: boolPtr(true)},
			Network: NetworkConfig{
				Bootproto:   "dhcp",
				Nameservers: []string{"1.1.1.1", "8.8.8.8"},
			},
			Packages: PackagesConfig{
				Install:      []string{"openssh-server", "curl"},
				Groups:       []string{"core"},
				UpdatePolicy: "none",
			},
			Storage: StorageConfig{
				Mode:             "lvm",
				VolumeGroup:      "vg0",
				WipeDiskPatterns: []string{"/dev/sd*"},
				Filesystems: map[string]FilesystemConfig{
					"root": {Mountpoint: "/", FSType: "ext4", Size: "grow"},
					"swap": {FSType: "swap", SizeMiB: intPtr(8192)},
				},
			},
			Boot: BootConfig{
				Firmware:  "uefi",
				Netboot:   NetbootConfig{Method: "ipxe", KernelArgs: []string{"console=ttyS0", "loglevel=6"}},
				Installed: InstalledBootConfig{Bootloader: "grub", TimeoutSeconds: intPtr(5), KernelArgs: []string{"consoleblank=0"}},
			},
			Repos: ReposConfig{
				OSMirror: "https://deb.debian.org/debian",
				Release:  "bookworm",
				Firmware: boolPtr(true),
				Contrib:  boolPtr(true),
				NonFree:  boolPtr(false),
			},
			Installer: InstallerConfig{
				ConfigTemplate: "preseed/debian",
				ConfigParams: map[string]any{
					"encrypt_home": false,
				},
				ExtraTemplate: "provisioning/extra",
			},
			Params: map[string]any{"release": "legacy-param"},
		},
		Targets: map[string]Target{
			"debian13": {
				Script:   "debian.ipxe",
				Network:  NetworkConfig{Nameservers: []string{"9.9.9.9"}},
				Packages: PackagesConfig{Install: []string{"qemu-guest-agent"}},
				Storage: StorageConfig{
					Disk:             "/dev/nvme0n1",
					WipeDiskPatterns: []string{"/dev/nvme*n*"},
					Filesystems: map[string]FilesystemConfig{
						"root": {FSType: "xfs"},
						"home": {Mountpoint: "/home", FSType: "ext4", Size: "grow"},
					},
				},
				Repos: ReposConfig{Release: "trixie", NonFree: boolPtr(true)},
				Installer: InstallerConfig{
					ConfigParams: map[string]any{
						"locale": "en_US",
					},
				},
				Params: map[string]any{"release": "target-param"},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
			Locale:        LocaleConfig{Keyboard: "de"},
			Storage: StorageConfig{
				WipeDiskPatterns: []string{"/dev/disk/by-id/inngest-*"},
				Filesystems: map[string]FilesystemConfig{
					"swap": {Absent: boolPtr(true)},
				},
			},
			Boot: BootConfig{
				Netboot: NetbootConfig{KernelArgs: []string{"console=ttyS1"}},
			},
			Installer: InstallerConfig{
				ConfigParams: map[string]any{
					"secret": map[string]any{"env": "INSTALL_SECRET"},
				},
			},
			Params: map[string]any{"role": "database"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(key string) (string, bool) {
			if key == "INSTALL_SECRET" {
				return "resolved-secret", true
			}
			return "", false
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "trixie", result.Params["release"])
	assert.Equal(t, "database", result.Params["role"])
	assert.Equal(t, "en_US.UTF-8", result.Provisioning.Locale.Language)
	assert.Equal(t, "de", result.Provisioning.Locale.Keyboard)
	assert.Equal(t, "UTC", result.Provisioning.Time.Timezone)
	require.NotNil(t, result.Provisioning.Time.UTC)
	assert.True(t, *result.Provisioning.Time.UTC)
	assert.Equal(t, "dhcp", result.Provisioning.Network.Bootproto)
	assert.Equal(t, []string{"9.9.9.9"}, result.Provisioning.Network.Nameservers)
	assert.Equal(t, []string{"qemu-guest-agent"}, result.Provisioning.Packages.Install)
	assert.Equal(t, []string{"core"}, result.Provisioning.Packages.Groups)
	assert.Equal(t, "/dev/nvme0n1", result.Provisioning.Storage.Disk)
	assert.Equal(t, "lvm", result.Provisioning.Storage.Mode)
	assert.Equal(t, "vg0", result.Provisioning.Storage.VolumeGroup)
	assert.Equal(t, []string{"/dev/disk/by-id/inngest-*"}, result.Provisioning.Storage.WipeDiskPatterns)
	assert.Equal(t, FilesystemConfig{Mountpoint: "/", FSType: "xfs", Size: "grow"}, result.Provisioning.Storage.Filesystems["root"])
	assert.Equal(t, FilesystemConfig{Mountpoint: "/home", FSType: "ext4", Size: "grow"}, result.Provisioning.Storage.Filesystems["home"])
	assert.NotContains(t, result.Provisioning.Storage.Filesystems, "swap")
	assert.Equal(t, "uefi", result.Provisioning.Boot.Firmware)
	assert.Equal(t, "ipxe", result.Provisioning.Boot.Netboot.Method)
	assert.Equal(t, []string{"console=ttyS1"}, result.Provisioning.Boot.Netboot.KernelArgs)
	assert.Equal(t, "grub", result.Provisioning.Boot.Installed.Bootloader)
	require.NotNil(t, result.Provisioning.Boot.Installed.TimeoutSeconds)
	assert.Equal(t, 5, *result.Provisioning.Boot.Installed.TimeoutSeconds)
	assert.Equal(t, "https://deb.debian.org/debian", result.Provisioning.Repos.OSMirror)
	assert.Equal(t, "trixie", result.Provisioning.Repos.Release)
	require.NotNil(t, result.Provisioning.Repos.NonFree)
	assert.True(t, *result.Provisioning.Repos.NonFree)
	assert.Equal(t, "preseed/debian", result.Provisioning.Installer.ConfigTemplate)
	assert.Equal(t, "provisioning/extra", result.Provisioning.Installer.ExtraTemplate)
	assert.Equal(t, map[string]any{
		"encrypt_home": false,
		"locale":       "en_US",
		"secret":       "resolved-secret",
	}, result.Provisioning.Installer.ConfigParams)
}

func TestResolverProjectsInheritedSwapAbsentToRegularParams(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Storage: StorageConfig{
				Filesystems: map[string]FilesystemConfig{
					"root": {Mountpoint: "/", FSType: "ext4", Size: "grow"},
					"swap": {FSType: "swap", SizeMiB: intPtr(8192)},
				},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
			Storage: StorageConfig{
				Filesystems: map[string]FilesystemConfig{
					"swap": {Absent: boolPtr(true)},
				},
			},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
	})
	require.NoError(t, err)

	params := ParamsWithProvisioning(result.Params, result.Users, result.Provisioning)

	assert.Equal(t, "false", params["debian_regular_swap_enabled"])
}

func TestResolverMergesStructuredRAIDStorage(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Storage: StorageConfig{
				Mode: "raid",
				RAID: RAIDConfig{
					Level:        1,
					Devices:      []string{"/dev/disk/by-id/default-a", "/dev/disk/by-id/default-b"},
					BootDegraded: boolPtr(false),
				},
			},
			Boot: BootConfig{Firmware: "uefi"},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Storage: StorageConfig{
					RAID: RAIDConfig{
						Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
			Storage: StorageConfig{
				RAID: RAIDConfig{
					BootDegraded: boolPtr(true),
				},
			},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})

	require.NoError(t, err)
	assert.Equal(t, "raid", result.Provisioning.Storage.Mode)
	assert.Equal(t, 1, result.Provisioning.Storage.RAID.Level)
	assert.Equal(t, []string{"/dev/nvme0n1", "/dev/nvme1n1"}, result.Provisioning.Storage.RAID.Devices)
	require.NotNil(t, result.Provisioning.Storage.RAID.BootDegraded)
	assert.True(t, *result.Provisioning.Storage.RAID.BootDegraded)
}

func TestResolverResolvesRawStorageEncryptionPassphrase(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Storage: StorageConfig{
					Encryption: StorageEncryptionConfig{
						Enabled:    boolPtr(true),
						Passphrase: "lab-passphrase",
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})

	require.NoError(t, err)
	assert.Equal(t, "lab-passphrase", result.Provisioning.Storage.Encryption.Passphrase)
}

func TestResolverResolvesEnvironmentBackedStorageEncryptionPassphrase(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Storage: StorageConfig{
					Encryption: StorageEncryptionConfig{
						Enabled:    boolPtr(true),
						Passphrase: map[string]any{"env": "SHOELACES_LUKS_PASSPHRASE"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(key string) (string, bool) {
			if key == "SHOELACES_LUKS_PASSPHRASE" {
				return "env-passphrase", true
			}
			return "", false
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "env-passphrase", result.Provisioning.Storage.Encryption.Passphrase)
}

func TestResolverMergesStructuredStorageEncryption(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Storage: StorageConfig{
				Encryption: StorageEncryptionConfig{
					Enabled:    boolPtr(true),
					Passphrase: map[string]any{"env": "DEFAULT_LUKS_PASSPHRASE"},
					Cipher:     "aes-xts-plain64",
					KeySize:    intPtr(256),
					Hash:       "sha256",
				},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Storage: StorageConfig{
					Encryption: StorageEncryptionConfig{
						KeySize: intPtr(512),
						Hash:    "sha512",
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
			Storage: StorageConfig{
				Encryption: StorageEncryptionConfig{
					Passphrase: map[string]any{"env": "HOST_LUKS_PASSPHRASE"},
				},
			},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(key string) (string, bool) {
			if key == "HOST_LUKS_PASSPHRASE" {
				return "host-passphrase", true
			}
			return "", false
		},
	})

	require.NoError(t, err)
	encryption := result.Provisioning.Storage.Encryption
	require.NotNil(t, encryption.Enabled)
	assert.True(t, *encryption.Enabled)
	assert.Equal(t, "host-passphrase", encryption.Passphrase)
	assert.Equal(t, "aes-xts-plain64", encryption.Cipher)
	require.NotNil(t, encryption.KeySize)
	assert.Equal(t, 512, *encryption.KeySize)
	assert.Equal(t, "sha512", encryption.Hash)
}

func TestResolverDoesNotResolveDisabledStorageEncryptionPassphrase(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Storage: StorageConfig{
				Encryption: StorageEncryptionConfig{
					Enabled:    boolPtr(true),
					Passphrase: map[string]any{"env": "DEFAULT_LUKS_PASSPHRASE"},
				},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Storage: StorageConfig{
					Encryption: StorageEncryptionConfig{
						Enabled: boolPtr(false),
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	result, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "", false
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Provisioning.Storage.Encryption.Enabled)
	assert.False(t, *result.Provisioning.Storage.Encryption.Enabled)
	assert.Equal(t, map[string]any{"env": "DEFAULT_LUKS_PASSPHRASE"}, result.Provisioning.Storage.Encryption.Passphrase)
}

func TestResolverRejectsIncompleteRAIDStorage(t *testing.T) {
	tests := []struct {
		name      string
		storage   StorageConfig
		boot      BootConfig
		wantError string
	}{
		{
			name: "missing level",
			storage: StorageConfig{
				Mode: "raid",
				RAID: RAIDConfig{Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"}},
			},
			boot:      BootConfig{Firmware: "uefi"},
			wantError: "resolved provisioning.storage.raid.level must be 1 for Debian RAID mode",
		},
		{
			name: "missing devices",
			storage: StorageConfig{
				Mode: "raid",
				RAID: RAIDConfig{Level: 1},
			},
			boot:      BootConfig{Firmware: "uefi"},
			wantError: "resolved provisioning.storage.raid.devices must contain exactly 2 devices for Debian RAID mode",
		},
		{
			name: "missing firmware",
			storage: StorageConfig{
				Mode: "raid",
				RAID: RAIDConfig{
					Level:   1,
					Devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
				},
			},
			wantError: `resolved provisioning.boot.firmware must be "uefi" when storage.mode is raid`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewResolver(&Mappings{
				Targets: map[string]Target{
					"debian13": {
						Script:  "debian.ipxe",
						Storage: tt.storage,
						Boot:    tt.boot,
					},
				},
				MacMaps: []MacMapConfig{{
					Mac:           "0c:42:a1:c3:52:96",
					DefaultTarget: "debian13",
					Targets:       []string{"debian13"},
				}},
			})
			require.NoError(t, err)

			_, err = resolver.Resolve(ResolveRequest{Mac: "0c:42:a1:c3:52:96"})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestResolverDoesNotLeakResolvedProvisioningBetweenBoots(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Installer: InstallerConfig{
					ConfigTemplate: "preseed/debian",
					ConfigParams: map[string]any{
						"token": map[string]any{"env": "INSTALL_TOKEN"},
					},
				},
				Packages: PackagesConfig{Install: []string{"curl"}},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	first, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "first", true
		},
	})
	require.NoError(t, err)
	first.Provisioning.Installer.ConfigParams["token"] = "mutated"
	first.Provisioning.Packages.Install[0] = "mutated"

	second, err := resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "second", true
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "second", second.Provisioning.Installer.ConfigParams["token"])
	assert.Equal(t, []string{"curl"}, second.Provisioning.Packages.Install)
}

func TestResolverReturnsMissingEnvironmentVariableErrorForStructuredInstallerParams(t *testing.T) {
	resolver, err := NewResolver(&Mappings{
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Installer: InstallerConfig{
					ConfigTemplate: "preseed/debian",
					ConfigParams: map[string]any{
						"token": map[string]any{"env": "INSTALL_TOKEN"},
					},
				},
			},
		},
		MacMaps: []MacMapConfig{{
			Mac:           "0c:42:a1:c3:52:96",
			DefaultTarget: "debian13",
			Targets:       []string{"debian13"},
		}},
	})
	require.NoError(t, err)

	_, err = resolver.Resolve(ResolveRequest{
		Mac: "0c:42:a1:c3:52:96",
		EnvLookup: func(string) (string, bool) {
			return "", false
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `parameter "installer.configParams.token" references missing environment variable "INSTALL_TOKEN"`)
}

func newTestResolver(t *testing.T) *Resolver {
	t.Helper()

	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Params: map[string]any{
				"install_username":       "infra",
				"shared":                 "defaults",
				"install_retries":        1,
				"enable_ssh":             false,
				"default_only_parameter": "default",
			},
		},
		Targets: map[string]Target{
			"debian12": {
				Script: "debian.ipxe",
				Label:  "Debian 12",
				Params: map[string]any{"release": "bookworm", "shared": "target"},
			},
			"debian13": {
				Script:      "debian.ipxe",
				Label:       "Debian 13",
				Environment: "testing",
				Params:      map[string]any{"release": "trixie", "shared": "target"},
			},
			"ubuntu2404": {
				Script: "ubuntu.ipxe",
				Label:  "Ubuntu 24.04",
				Params: map[string]any{"release": "noble"},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
				Params:        map[string]any{"role": "mac", "shared": "mapping"},
			},
		},
		IPMaps: []IPMapConfig{
			{
				IP:            "192.0.2.10",
				DefaultTarget: "ubuntu2404",
				Targets:       []string{"ubuntu2404"},
			},
			{
				IP:            "2001:db8::99",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
		HostnameMaps: []HostnameMapConfig{
			{
				Hostname:      "^host-\\d+$",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
		NetworkMaps: []NetworkMapConfig{
			{
				Network:       "198.51.100.0/24",
				DefaultTarget: "debian12",
				Targets:       []string{"debian12"},
			},
			{
				Network: "2001:db8:1::/64",
				Targets: []string{"debian12", "debian13"},
			},
		},
	})
	require.NoError(t, err)
	return resolver
}

func newEnvTestResolver(t *testing.T) *Resolver {
	t.Helper()

	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Params: map[string]any{
				"root_password_crypted": map[string]any{"env": "SHOELACES_ROOT_PASSWORD_CRYPTED"},
			},
		},
		Targets: map[string]Target{
			"debian13": {
				Script: "debian.ipxe",
				Params: map[string]any{"release": "trixie"},
			},
		},
		MacMaps: []MacMapConfig{
			{
				Mac:           "0c:42:a1:c3:52:96",
				DefaultTarget: "debian13",
				Targets:       []string{"debian13"},
			},
		},
	})
	require.NoError(t, err)
	return resolver
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}
