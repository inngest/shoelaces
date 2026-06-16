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

func newTestResolver(t *testing.T) *Resolver {
	t.Helper()

	resolver, err := NewResolver(&Mappings{
		Defaults: DefaultsMap{
			Params: map[string]any{"install_username": "infra"},
		},
		Targets: map[string]Target{
			"debian12": {
				Script: "debian.ipxe",
				Label:  "Debian 12",
				Params: map[string]any{"release": "bookworm"},
			},
			"debian13": {
				Script:      "debian.ipxe",
				Label:       "Debian 13",
				Environment: "testing",
				Params:      map[string]any{"release": "trixie"},
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
