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

package polling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenStartScriptUsesBaseURL(t *testing.T) {
	script := GenStartScript(log.MakeLogger(testLogWriter{}), "127.0.0.1:8081")

	assert.Contains(t, script, "http://127.0.0.1:8081/poll/1/${netX/mac:hexhyp}")
	assert.True(t, strings.HasPrefix(script, "#!ipxe\n"), "start script should be an iPXE script, got:\n%s", script)
}

func TestPollUnknownServerRetriesThenTimesOut(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	events := &event.Log{}
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "")

	script, err := Poll(
		log.MakeLogger(testLogWriter{}),
		states,
		nil,
		events,
		templates.New(),
		"127.0.0.1:8081",
		srv,
	)
	require.NoError(t, err)
	assert.Contains(t, script, "chain -ar http://127.0.0.1:8081/poll/1/06-66-de-ad-be-ef")
	require.NotNil(t, states.Servers[srv.Mac])
	assert.Len(t, events.Events[srv.Mac], 1)

	for i := 0; i <= maxRetry; i++ {
		script, err = Poll(
			log.MakeLogger(testLogWriter{}),
			states,
			nil,
			events,
			templates.New(),
			"127.0.0.1:8081",
			srv,
		)
		require.NoError(t, err)
	}

	assert.Equal(t, timeoutScript, script)
	assert.Nil(t, states.Servers[srv.Mac])
}

func TestPollBootsAutomaticMatches(t *testing.T) {
	tests := []struct {
		name         string
		srv          server.Server
		resolver     *mappings.Resolver
		wantBootType string
		wantHostname string
		wantRendered string
		wantParams   map[string]interface{}
	}{
		{
			name: "hostname match keeps resolved hostname",
			srv:  server.New("06:66:de:ad:be:ef", "192.0.2.10", "matched-host"),
			resolver: mustResolver(t, &mappings.Mappings{
				Targets: map[string]mappings.Target{
					"web": {
						Script: "test.ipxe",
						Params: map[string]interface{}{
							"hostname": "target-host",
							"role":     "web",
						},
					},
				},
				HostnameMaps: []mappings.HostnameMapConfig{{
					Hostname:      `^matched-host$`,
					DefaultTarget: "web",
					Targets:       []string{"web"},
				}},
			}),
			wantBootType: event.PtrMatchBoot,
			wantHostname: "matched-host",
			wantRendered: "boot matched-host",
			wantParams: map[string]interface{}{
				"baseURL":  "127.0.0.1:8081",
				"hostname": "matched-host",
				"role":     "web",
			},
		},
		{
			name: "network match with hostname prefix",
			srv:  server.New("06:66:de:ad:be:ef", "192.0.2.10", ""),
			resolver: mustResolver(t, &mappings.Mappings{
				Targets: map[string]mappings.Target{
					"db": {
						Script: "test.ipxe",
						Params: map[string]interface{}{
							"hostnamePrefix": "rack-",
							"role":           "db",
						},
					},
				},
				NetworkMaps: []mappings.NetworkMapConfig{{
					Network:       "192.0.2.0/24",
					DefaultTarget: "db",
					Targets:       []string{"db"},
				}},
			}),
			wantBootType: event.SubnetMatchBoot,
			wantHostname: "rack-06-66-de-ad-be-ef",
			wantRendered: "boot rack-06-66-de-ad-be-ef",
			wantParams: map[string]interface{}{
				"baseURL":        "127.0.0.1:8081",
				"hostname":       "rack-06-66-de-ad-be-ef",
				"hostnamePrefix": "rack-",
				"role":           "db",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := &event.Log{}
			rendered, err := Poll(
				log.MakeLogger(testLogWriter{}),
				&server.States{Servers: make(map[string]*server.State)},
				tt.resolver,
				events,
				newTestTemplates(t),
				"127.0.0.1:8081",
				tt.srv,
			)

			require.NoError(t, err)
			assert.Contains(t, rendered, tt.wantRendered)
			assert.Contains(t, rendered, "base 127.0.0.1:8081")
			require.Len(t, events.Events[tt.srv.Mac], 1)
			got := events.Events[tt.srv.Mac][0]
			assert.Equal(t, event.HostBoot, got.Type)
			assert.Equal(t, tt.wantBootType, got.BootType)
			assert.Equal(t, "test.ipxe", got.Script)
			assert.Equal(t, tt.wantHostname, got.Server.Hostname)
			assert.Equal(t, tt.wantParams, got.Params)
		})
	}
}

func TestPollBootsMappingResolvedEmbeddedTemplate(t *testing.T) {
	events := &event.Log{}
	resolver := mustResolver(t, &mappings.Mappings{
		Targets: map[string]mappings.Target{
			"debian12": {
				Script: "debian.ipxe",
				Params: map[string]interface{}{
					"encrypt_home": false,
					"release":      "bookworm",
				},
			},
		},
		NetworkMaps: []mappings.NetworkMapConfig{{
			Network:       "192.0.2.0/24",
			DefaultTarget: "debian12",
			Targets:       []string{"debian12"},
		}},
	})
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "")

	rendered, err := Poll(
		log.MakeLogger(testLogWriter{}),
		&server.States{Servers: make(map[string]*server.State)},
		resolver,
		events,
		newEmbeddedProvisioningTemplates(t),
		"127.0.0.1:8081",
		srv,
	)

	require.NoError(t, err)
	assert.Contains(t, rendered, "Debian bookworm netboot")
	assert.Contains(t, rendered, "hostname=06-66-de-ad-be-ef")
	assert.Contains(t, rendered, "preseed/url=http://127.0.0.1:8081/configs/preseed/debian?encrypt_home=false")
	require.Len(t, events.Events[srv.Mac], 1)
	assert.Equal(t, event.HostBoot, events.Events[srv.Mac][0].Type)
	assert.Equal(t, "debian.ipxe", events.Events[srv.Mac][0].Script)
}

func TestUpdateTargetStoresManualSelection(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	events := &event.Log{}
	templateRenderer := newTestTemplates(t)
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "manual-host")
	states.AddServer(srv)
	resolver := mustResolver(t, &mappings.Mappings{
		Targets: map[string]mappings.Target{
			"manual": {
				Script: "test.ipxe",
				Params: map[string]interface{}{"role": "target"},
			},
		},
	})
	params := map[string]interface{}{"role": "manual"}

	inputErr, err := UpdateTarget(
		log.MakeLogger(testLogWriter{}),
		states,
		resolver,
		templateRenderer,
		events,
		"127.0.0.1:8081",
		srv,
		"manual",
		"",
		params,
	)

	require.NoError(t, err)
	assert.False(t, inputErr)
	require.NotNil(t, states.Servers[srv.Mac])
	assert.Equal(t, "test.ipxe", states.Servers[srv.Mac].Target)
	assert.Equal(t, "06-66-de-ad-be-ef", states.Servers[srv.Mac].Params["hostname"])
	assert.Equal(t, "127.0.0.1:8081", states.Servers[srv.Mac].Params["baseURL"])
	assert.Equal(t, "manual", states.Servers[srv.Mac].Params["role"])
	require.Len(t, events.Events[srv.Mac], 1)
	assert.Equal(t, event.UserSelection, events.Events[srv.Mac][0].Type)
	assert.Equal(t, "test.ipxe", events.Events[srv.Mac][0].Script)
}

func TestPollQueuesRestrictedManualTargets(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	resolver := mustResolver(t, &mappings.Mappings{
		Targets: map[string]mappings.Target{
			"debian12": {Script: "test.ipxe", Label: "Debian 12"},
			"debian13": {Script: "test.ipxe", Label: "Debian 13"},
			"ubuntu24": {Script: "test.ipxe", Label: "Ubuntu 24.04"},
		},
		NetworkMaps: []mappings.NetworkMapConfig{{
			Network: "192.0.2.0/24",
			Targets: []string{
				"debian12",
				"debian13",
			},
		}},
	})
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "")

	script, err := Poll(
		log.MakeLogger(testLogWriter{}),
		states,
		resolver,
		&event.Log{},
		newTestTemplates(t),
		"127.0.0.1:8081",
		srv,
	)

	require.NoError(t, err)
	assert.Contains(t, script, "/poll/1/06-66-de-ad-be-ef")
	require.NotNil(t, states.Servers[srv.Mac])
	assert.Equal(t, []string{"debian12", "debian13"}, targetOptionNames(states.Servers[srv.Mac].AllowedTargets))

	waiting := ListServers(states)
	require.Len(t, waiting, 1)
	assert.Equal(t, []string{"debian12", "debian13"}, targetOptionNames(waiting[0].AllowedTargets))

	inputErr, err := UpdateTarget(
		log.MakeLogger(testLogWriter{}),
		states,
		resolver,
		newTestTemplates(t),
		&event.Log{},
		"127.0.0.1:8081",
		srv,
		"ubuntu24",
		"",
		map[string]interface{}{},
	)
	assert.True(t, inputErr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestPollQueuesUnrestrictedManualTargets(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	resolver := mustResolver(t, &mappings.Mappings{
		Targets: map[string]mappings.Target{
			"ubuntu24": {Script: "test.ipxe"},
			"debian12": {Script: "test.ipxe"},
		},
	})
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "")

	_, err := Poll(
		log.MakeLogger(testLogWriter{}),
		states,
		resolver,
		&event.Log{},
		newTestTemplates(t),
		"127.0.0.1:8081",
		srv,
	)

	require.NoError(t, err)
	require.NotNil(t, states.Servers[srv.Mac])
	assert.Equal(t, []string{"debian12", "ubuntu24"}, targetOptionNames(states.Servers[srv.Mac].AllowedTargets))
}

func TestListServersReturnsOnlyWaitingServersSortedByMAC(t *testing.T) {
	states := &server.States{Servers: map[string]*server.State{
		"ff:ff:ff:ff:ff:ff": {
			Server: server.New("ff:ff:ff:ff:ff:ff", "192.0.2.3", "last"),
			Target: server.InitTarget,
		},
		"00:00:00:00:00:01": {
			Server: server.New("00:00:00:00:00:01", "192.0.2.1", "first"),
			Target: server.InitTarget,
		},
		"00:00:00:00:00:02": {
			Server: server.New("00:00:00:00:00:02", "192.0.2.2", "booting"),
			Target: "debian.ipxe",
		},
	}}

	servers := ListServers(states)
	require.Len(t, servers, 2)
	assert.Equal(t, "00:00:00:00:00:01", servers[0].Mac)
	assert.Equal(t, "ff:ff:ff:ff:ff:ff", servers[1].Mac)
}

type testLogWriter struct{}

func (testLogWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func newTestTemplates(t *testing.T) *templates.ShoelacesTemplates {
	t.Helper()

	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dataDir, "test.ipxe.slc"),
		[]byte(`{{define "test.ipxe"}}#!ipxe
boot {{.hostname}}
base {{.baseURL}}
role {{.role}}
{{end}}
`),
		0o644,
	))

	templateRenderer := templates.New()
	templateRenderer.ParseTemplates(log.MakeLogger(testLogWriter{}), dataDir, "env_overrides", nil, ".slc")
	return templateRenderer
}

func newEmbeddedProvisioningTemplates(t *testing.T) *templates.ShoelacesTemplates {
	t.Helper()

	templateRenderer := templates.New()
	templateRenderer.ParseTemplates(log.MakeLogger(testLogWriter{}), t.TempDir(), "env_overrides", nil, ".slc")
	return templateRenderer
}

func mustResolver(t *testing.T, mappingsConfig *mappings.Mappings) *mappings.Resolver {
	t.Helper()

	resolver, err := mappings.NewResolver(mappingsConfig)
	require.NoError(t, err)
	return resolver
}

func targetOptionNames(options []server.TargetOption) []string {
	names := make([]string, 0, len(options))
	for _, option := range options {
		names = append(names, option.Name)
	}
	return names
}
