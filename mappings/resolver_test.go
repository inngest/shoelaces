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
