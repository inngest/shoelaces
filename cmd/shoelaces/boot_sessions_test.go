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

	"github.com/inngest/shoelaces/bootsession"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence"
	persistencesqlite "github.com/inngest/shoelaces/persistence/sqlite"
	"github.com/inngest/shoelaces/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v3"
)

func TestBootSessionsGetCommandOutputsJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeBootSessionCommandFixture(t, bootSessionSnapshotFixture("ref-valid", time.Now().UTC().Add(time.Hour)))
	var output bytes.Buffer
	cmd := bootSessionCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "boot-sessions", "get", "ref-valid", "--output", "json"}))

	var reference bootsession.Reference
	require.NoError(t, json.Unmarshal(output.Bytes(), &reference))
	assert.Equal(t, "ref-valid", reference.Ref)
	assert.Equal(t, "06:66:de:ad:be:ef", reference.Server.Mac)
	assert.Equal(t, "install-host", reference.Server.Hostname)
	assert.Equal(t, "debian.ipxe", reference.Target)
	assert.Equal(t, "production", reference.Environment)
	assert.Equal(t, "[REDACTED]", reference.Params["bootstrap_token"])
	assert.NotContains(t, output.String(), "secret-token")
	assert.NotContains(t, output.String(), "$6$secret")
	assert.NotContains(t, output.String(), "ssh-ed25519 secret")
}

func TestBootSessionsGetCommandOutputsTable(t *testing.T) {
	ctx := context.Background()
	fixture := writeBootSessionCommandFixture(t, bootSessionSnapshotFixture("ref-valid", time.Now().UTC().Add(time.Hour)))
	var output bytes.Buffer
	cmd := bootSessionCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "boot-sessions", "get", "ref-valid"}))

	assert.Contains(t, output.String(), "REF")
	assert.Contains(t, output.String(), "MAC")
	assert.Contains(t, output.String(), "HOSTNAME")
	assert.Contains(t, output.String(), "TARGET")
	assert.Contains(t, output.String(), "EXPIRES")
	assert.Contains(t, output.String(), "ref-valid")
	assert.Contains(t, output.String(), "06:66:de:ad:be:ef")
	assert.Contains(t, output.String(), "install-host")
	assert.Contains(t, output.String(), "debian.ipxe")
	assert.NotContains(t, output.String(), "secret-token")
}

func TestBootSessionsGetCommandReturnsExpiredRefError(t *testing.T) {
	ctx := context.Background()
	fixture := writeBootSessionCommandFixture(t, bootSessionSnapshotFixture("ref-expired", time.Now().UTC().Add(-time.Hour)))
	cmd := bootSessionCommandFixtureCommand(fixture.configValues)
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	err := cmd.Run(ctx, []string{"shoelaces", "boot-sessions", "get", "ref-expired"})

	assert.ErrorContains(t, err, "boot session not found: ref-expired")
}

func TestBootSessionsGetCommandReturnsMissingRefError(t *testing.T) {
	ctx := context.Background()
	fixture := writeBootSessionCommandFixture(t)
	cmd := bootSessionCommandFixtureCommand(fixture.configValues)
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	err := cmd.Run(ctx, []string{"shoelaces", "boot-sessions", "get", "missing-ref"})

	assert.ErrorContains(t, err, "boot session not found: missing-ref")
}

type bootSessionCommandFixture struct {
	configValues map[any]any
}

func writeBootSessionCommandFixture(t *testing.T, snapshots ...bootsession.Snapshot) bootSessionCommandFixture {
	t.Helper()

	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "runtime", "shoelaces.db")
	store, err := persistencesqlite.Open(ctx, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	bootSessions := bootsession.NewStore(store, store, time.Hour)
	for _, snapshot := range snapshots {
		ref, err := bootSessions.Create(ctx, snapshot)
		require.NoError(t, err)
		assert.Equal(t, snapshot.Ref, ref)
	}

	return bootSessionCommandFixture{
		configValues: map[any]any{
			"data-dir":            dataDir,
			"persistence-backend": persistence.BackendSQLite,
			"persistence-path":    filepath.Join("runtime", "shoelaces.db"),
		},
	}
}

func bootSessionCommandFixtureCommand(configValues map[any]any) *cli.Command {
	return command("", configValues, func(env *environment.Environment) error {
		return nil
	})
}

func bootSessionSnapshotFixture(ref string, expiresAt time.Time) bootsession.Snapshot {
	createdAt := expiresAt.Add(-time.Hour)
	return bootsession.Snapshot{
		Ref:         ref,
		Server:      server.New("06:66:de:ad:be:ef", "192.0.2.10", "install-host"),
		Target:      "debian.ipxe",
		Environment: "production",
		Params: map[string]any{
			"hostname":        "install-host",
			"bootstrap_token": "secret-token",
		},
		Users: map[string]mappings.ResolvedUser{
			"infra": {
				Name:              "infra",
				Primary:           true,
				PasswordCrypted:   "$6$secret",
				SSHAuthorizedKeys: []string{"ssh-ed25519 secret"},
			},
		},
		Provisioning: mappings.ProvisioningConfig{
			Installer: mappings.InstallerConfig{
				ConfigParams: map[string]any{
					"bootstrap_token": "secret-token",
				},
			},
		},
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}
}
