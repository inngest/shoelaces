// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/persistence"
	persistencesqlite "github.com/inngest/shoelaces/persistence/sqlite"
	"github.com/inngest/shoelaces/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v3"
)

func TestServersListCommandOutputsEmptyJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t)
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "list", "--output", "json"}))

	var states []serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &states))
	assert.Empty(t, states)
}

func TestServersListCommandOutputsWaitingAndSelectedJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t,
		waitingServerStateFixture(fixtureTime().Add(-time.Minute)),
		selectedServerStateFixture(fixtureTime()),
	)
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "list", "--output", "json"}))

	var states []serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &states))
	require.Len(t, states, 2)

	assert.Equal(t, "00:11:22:33:44:55", states[0].MAC)
	assert.True(t, states[0].Waiting)
	assert.Equal(t, server.InitTarget, states[0].Target)
	assert.Equal(t, int64(1), states[0].Retry)
	require.Len(t, states[0].AllowedTargets, 2)
	assert.Equal(t, "debian12", states[0].AllowedTargets[0].Name)
	assert.Equal(t, "shoelaces", states[0].Params["baseURL"])
	assert.Contains(t, output.String(), `"name":"debian12"`)
	assert.NotContains(t, output.String(), `"Name"`)

	assert.Equal(t, "00:11:22:33:44:66", states[1].MAC)
	assert.False(t, states[1].Waiting)
	assert.Equal(t, "ubuntu.ipxe", states[1].Target)
	assert.Equal(t, "production", states[1].Environment)
	assert.Equal(t, int64(3), states[1].Retry)
	assert.Equal(t, "db", states[1].Params["role"])
}

func TestServersListCommandRedactsSensitiveParamsInJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t, sensitiveServerStateFixture(fixtureTime()))
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "list", "--output", "json"}))

	var states []serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &states))
	require.Len(t, states, 1)
	assertRedactedServerStateParams(t, states[0].Params)
	assert.NotContains(t, output.String(), "$6$root")
	assert.NotContains(t, output.String(), "secret-token")
	assert.NotContains(t, output.String(), "private-key")
	assert.NotContains(t, output.String(), "nested-secret")
	assert.NotContains(t, output.String(), "boot-ref")
}

func TestServersListCommandOutputsTable(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t, waitingServerStateFixture(fixtureTime()))
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "list"}))

	assert.Contains(t, output.String(), "MAC")
	assert.Contains(t, output.String(), "HOSTNAME")
	assert.Contains(t, output.String(), "TARGET")
	assert.Contains(t, output.String(), "ALLOWED TARGETS")
	assert.Contains(t, output.String(), "PARAMS")
	assert.Contains(t, output.String(), "00:11:22:33:44:55")
	assert.Contains(t, output.String(), "waiting-host")
	assert.Contains(t, output.String(), server.InitTarget)
	assert.Contains(t, output.String(), "debian12,ubuntu24")
	assert.Contains(t, output.String(), "baseURL,root_password_crypted")
}

func TestServersListCommandFiltersWaitingAndMAC(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t,
		waitingServerStateFixture(fixtureTime().Add(-time.Minute)),
		selectedServerStateFixture(fixtureTime()),
	)
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{
		"shoelaces",
		"servers",
		"list",
		"--output",
		"json",
		"--waiting",
		"--mac",
		"00:11:22:33:44:55",
	}))

	var states []serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &states))
	require.Len(t, states, 1)
	assert.Equal(t, "00:11:22:33:44:55", states[0].MAC)
	assert.True(t, states[0].Waiting)
}

func TestServersGetCommandOutputsJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t, selectedServerStateFixture(fixtureTime()))
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "get", "00:11:22:33:44:66", "--output", "json"}))

	var state serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &state))
	assert.Equal(t, "00:11:22:33:44:66", state.MAC)
	assert.Equal(t, "selected-host", state.Hostname)
	assert.Equal(t, "ubuntu.ipxe", state.Target)
	assert.False(t, state.Waiting)
	assert.Equal(t, "db", state.Params["role"])
}

func TestServersGetCommandRedactsSensitiveParamsInJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t, sensitiveServerStateFixture(fixtureTime()))
	var output bytes.Buffer
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "servers", "get", "00:11:22:33:44:77", "--output", "json"}))

	var state serverStateOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &state))
	assertRedactedServerStateParams(t, state.Params)
	assert.NotContains(t, output.String(), "$6$root")
	assert.NotContains(t, output.String(), "secret-token")
	assert.NotContains(t, output.String(), "private-key")
	assert.NotContains(t, output.String(), "nested-secret")
	assert.NotContains(t, output.String(), "boot-ref")
}

func TestServersGetCommandReturnsMissingMACError(t *testing.T) {
	ctx := context.Background()
	fixture := writeServerCommandFixture(t)
	cmd := serverCommandFixtureCommand(fixture.configValues)
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	err := cmd.Run(ctx, []string{"shoelaces", "servers", "get", "00:11:22:33:44:99"})

	assert.ErrorContains(t, err, "server state not found: 00:11:22:33:44:99")
}

func TestServerStateOutputRecordFromRecordRedactsParams(t *testing.T) {
	tests := []struct {
		name       string
		paramsJSON []byte
		assertions func(t *testing.T, params map[string]any)
	}{
		{
			name:       "top level sensitive params",
			paramsJSON: []byte(`{"role":"db","root_password_crypted":"$6$root","bootstrap_token":"secret-token","ssh_private_key":"private-key"}`),
			assertions: func(t *testing.T, params map[string]any) {
				assert.Equal(t, "db", params["role"])
				assert.Equal(t, "[REDACTED]", params["root_password_crypted"])
				assert.Equal(t, "[REDACTED]", params["bootstrap_token"])
				assert.Equal(t, "[REDACTED]", params["ssh_private_key"])
			},
		},
		{
			name:       "boot reference params",
			paramsJSON: []byte(`{"boot_ref":"boot-ref","boot_ref_query":"ref=boot-ref","boot_ref_query_suffix":"&ref=boot-ref","boot_ref_query_question":"?ref=boot-ref"}`),
			assertions: func(t *testing.T, params map[string]any) {
				assert.Equal(t, "[REDACTED]", params["boot_ref"])
				assert.Equal(t, "[REDACTED]", params["boot_ref_query"])
				assert.Equal(t, "[REDACTED]", params["boot_ref_query_suffix"])
				assert.Equal(t, "[REDACTED]", params["boot_ref_query_question"])
			},
		},
		{
			name:       "nested sensitive fields",
			paramsJSON: []byte(`{"metadata":{"password":"nested-secret","region":"iad"},"installers":[{"Token":"nested-token","name":"primary"}]}`),
			assertions: func(t *testing.T, params map[string]any) {
				metadata := params["metadata"].(map[string]any)
				assert.Equal(t, "[REDACTED]", metadata["password"])
				assert.Equal(t, "iad", metadata["region"])

				installers := params["installers"].([]any)
				firstInstaller := installers[0].(map[string]any)
				assert.Equal(t, "[REDACTED]", firstInstaller["Token"])
				assert.Equal(t, "primary", firstInstaller["name"])
			},
		},
		{
			name:       "structured top level secret containers",
			paramsJSON: []byte(`{"users":[{"Name":"root","PasswordCrypted":"$6$root"}],"provisioning":{"Installers":[{"ConfigParams":{"bootstrap_token":"secret-token"}}]}}`),
			assertions: func(t *testing.T, params map[string]any) {
				assert.Equal(t, "[REDACTED]", params["users"])
				assert.Equal(t, "[REDACTED]", params["provisioning"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := sensitiveServerStateFixture(fixtureTime())
			record.ParamsJSON = test.paramsJSON

			output, err := serverStateOutputRecordFromRecord(record)

			require.NoError(t, err)
			test.assertions(t, output.Params)
			encoded, err := json.Marshal(output)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), "$6$root")
			assert.NotContains(t, string(encoded), "secret-token")
			assert.NotContains(t, string(encoded), "private-key")
			assert.NotContains(t, string(encoded), "nested-secret")
			assert.NotContains(t, string(encoded), "boot-ref")
		})
	}
}

type serverCommandFixture struct {
	configValues map[any]any
}

func writeServerCommandFixture(t *testing.T, states ...persistence.ServerStateRecord) serverCommandFixture {
	t.Helper()

	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "runtime", "shoelaces.db")
	store, err := persistencesqlite.Open(ctx, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	for _, state := range states {
		require.NoError(t, store.UpsertServerState(ctx, state))
	}

	return serverCommandFixture{
		configValues: map[any]any{
			"data-dir":            dataDir,
			"persistence-backend": persistence.BackendSQLite,
			"persistence-path":    filepath.Join("runtime", "shoelaces.db"),
		},
	}
}

func serverCommandFixtureCommand(configValues map[any]any) *cli.Command {
	return command("", configValues, func(env *environment.Environment) error {
		return nil
	})
}

func waitingServerStateFixture(lastAccess time.Time) persistence.ServerStateRecord {
	return persistence.ServerStateRecord{
		MAC:                "00:11:22:33:44:55",
		IP:                 "192.0.2.10",
		Hostname:           "waiting-host",
		Target:             server.InitTarget,
		ParamsJSON:         []byte(`{"baseURL":"shoelaces","root_password_crypted":"[REDACTED]"}`),
		UsersJSON:          []byte(`{}`),
		ProvisioningJSON:   []byte(`{}`),
		AllowedTargetsJSON: []byte(`[{"Name":"debian12","Script":"debian.ipxe","Label":"Debian 12"},{"Name":"ubuntu24","Script":"ubuntu.ipxe","Label":"Ubuntu 24"}]`),
		Retry:              1,
		LastAccess:         lastAccess,
	}
}

func selectedServerStateFixture(lastAccess time.Time) persistence.ServerStateRecord {
	return persistence.ServerStateRecord{
		MAC:                "00:11:22:33:44:66",
		IP:                 "192.0.2.11",
		Hostname:           "selected-host",
		Target:             "ubuntu.ipxe",
		Environment:        "production",
		ParamsJSON:         []byte(`{"role":"db"}`),
		UsersJSON:          []byte(`{}`),
		ProvisioningJSON:   []byte(`{}`),
		AllowedTargetsJSON: []byte(`[{"Name":"ubuntu24","Script":"ubuntu.ipxe","Environment":"production"}]`),
		Retry:              3,
		LastAccess:         lastAccess,
	}
}

func sensitiveServerStateFixture(lastAccess time.Time) persistence.ServerStateRecord {
	return persistence.ServerStateRecord{
		MAC:                "00:11:22:33:44:77",
		IP:                 "192.0.2.12",
		Hostname:           "sensitive-host",
		Target:             "debian.ipxe",
		Environment:        "production",
		ParamsJSON:         []byte(`{"role":"db","root_password_crypted":"$6$root","bootstrap_token":"secret-token","ssh_private_key":"private-key","metadata":{"password":"nested-secret","region":"iad"},"boot_ref":"boot-ref"}`),
		UsersJSON:          []byte(`{}`),
		ProvisioningJSON:   []byte(`{}`),
		AllowedTargetsJSON: []byte(`[]`),
		Retry:              2,
		LastAccess:         lastAccess,
	}
}

func assertRedactedServerStateParams(t *testing.T, params map[string]any) {
	t.Helper()

	assert.Equal(t, "db", params["role"])
	assert.Equal(t, "[REDACTED]", params["root_password_crypted"])
	assert.Equal(t, "[REDACTED]", params["bootstrap_token"])
	assert.Equal(t, "[REDACTED]", params["ssh_private_key"])
	assert.Equal(t, "[REDACTED]", params["boot_ref"])
	metadata := params["metadata"].(map[string]any)
	assert.Equal(t, "[REDACTED]", metadata["password"])
	assert.Equal(t, "iad", metadata["region"])
}

func fixtureTime() time.Time {
	return time.Unix(1700000000, 0).UTC()
}
