// Copyright 2018 ThousandEyes Inc.
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
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thousandeyes/shoelaces/internal/event"
	"github.com/thousandeyes/shoelaces/internal/log"
	"github.com/thousandeyes/shoelaces/internal/mappings"
	"github.com/thousandeyes/shoelaces/internal/server"
	"github.com/thousandeyes/shoelaces/internal/templates"
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
		hostnameMaps []mappings.HostnameMap
		networkMaps  []mappings.NetworkMap
		wantBootType string
		wantHostname string
		wantRendered string
		wantParams   map[string]interface{}
	}{
		{
			name: "hostname match",
			srv:  server.New("06:66:de:ad:be:ef", "192.0.2.10", "matched-host"),
			hostnameMaps: []mappings.HostnameMap{{
				Hostname: regexp.MustCompile(`^matched-host$`),
				Script: &mappings.Script{
					Name:   "test.ipxe",
					Params: map[string]interface{}{"role": "web"},
				},
			}},
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
			networkMaps: []mappings.NetworkMap{{
				Network: mustCIDR(t, "192.0.2.0/24"),
				Script: &mappings.Script{
					Name: "test.ipxe",
					Params: map[string]interface{}{
						"hostnamePrefix": "rack-",
						"role":           "db",
					},
				},
			}},
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
				tt.hostnameMaps,
				tt.networkMaps,
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

func TestUpdateTargetStoresManualSelection(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	events := &event.Log{}
	templateRenderer := newTestTemplates(t)
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "manual-host")
	states.AddServer(srv)
	params := map[string]interface{}{"role": "manual"}

	inputErr, err := UpdateTarget(
		log.MakeLogger(testLogWriter{}),
		states,
		templateRenderer,
		events,
		"127.0.0.1:8081",
		srv,
		"test.ipxe",
		"",
		params,
	)

	require.NoError(t, err)
	assert.False(t, inputErr)
	require.NotNil(t, states.Servers[srv.Mac])
	assert.Equal(t, "test.ipxe", states.Servers[srv.Mac].Target)
	assert.Equal(t, "06-66-de-ad-be-ef", states.Servers[srv.Mac].Params["hostname"])
	assert.Equal(t, "127.0.0.1:8081", states.Servers[srv.Mac].Params["baseURL"])
	require.Len(t, events.Events[srv.Mac], 1)
	assert.Equal(t, event.UserSelection, events.Events[srv.Mac][0].Type)
	assert.Equal(t, "test.ipxe", events.Events[srv.Mac][0].Script)
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

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()

	_, network, err := net.ParseCIDR(cidr)
	require.NoError(t, err)
	return network
}
