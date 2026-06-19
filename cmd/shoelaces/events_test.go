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
	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/persistence"
	persistencesqlite "github.com/inngest/shoelaces/persistence/sqlite"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v3"
)

func TestEventsListCommandOutputsJSONInChronologicalOrder(t *testing.T) {
	ctx := context.Background()
	fixture := writeEventCommandFixture(t)
	var output bytes.Buffer
	cmd := eventCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "events", "list", "--output", "json"}))

	var events []eventOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &events))
	require.Len(t, events, 3)
	assert.Equal(t, []string{
		fixture.oldID.String(),
		fixture.selectionID.String(),
		fixture.bootID.String(),
	}, []string{events[0].ID, events[1].ID, events[2].ID})
	assert.Equal(t, "host-poll", events[0].Type)
	assert.Equal(t, "user-selection", events[1].Type)
	assert.Equal(t, "host-boot", events[2].Type)
	assert.NotContains(t, output.String(), "secret-token")
	assert.NotContains(t, output.String(), "params")
}

func TestEventsListCommandUsesEnvironmentPersistenceConfig(t *testing.T) {
	ctx := context.Background()
	fixture := writeEventCommandFixture(t)
	dataDir := fixture.configValues["data-dir"].(string)
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PERSISTENCE_BACKEND", persistence.BackendSQLite)
	t.Setenv("PERSISTENCE_PATH", filepath.Join("runtime", "shoelaces.db"))
	var output bytes.Buffer
	cmd := eventCommandFixtureCommand(nil)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "events", "list", "--output", "json"}))

	var events []eventOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &events))
	require.Len(t, events, 3)
	assert.Equal(t, fixture.oldID.String(), events[0].ID)
}

func TestEventsListCommandOutputsTable(t *testing.T) {
	ctx := context.Background()
	fixture := writeEventCommandFixture(t)
	var output bytes.Buffer
	cmd := eventCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "events", "list"}))

	assert.Contains(t, output.String(), "TIME")
	assert.Contains(t, output.String(), "EVENT ID")
	assert.Contains(t, output.String(), "MAC")
	assert.Contains(t, output.String(), fixture.oldID.String())
	assert.Contains(t, output.String(), "00:11:22:33:44:55")
	assert.Contains(t, output.String(), "host-poll")
	assert.Contains(t, output.String(), "old host polled")
}

func TestEventsListCommandFiltersAndLimits(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	records := []persistence.EventRecord{
		eventRecordFixture(t, "01K7XJ7CD80000000000000000", event.HostPoll, now.Add(-2*time.Hour), "00:11:22:33:44:55"),
		eventRecordFixture(t, "01K7XJ7CD90000000000000000", event.UserSelection, now.Add(-time.Hour), "00:11:22:33:44:66"),
		eventRecordFixture(t, "01K7XJ7CDA0000000000000000", event.UserSelection, now, "00:11:22:33:44:66"),
	}
	since := now.Add(-90 * time.Minute)
	eventType := int(event.UserSelection)

	filtered := filterEventRecords(records, eventListOptions{
		MAC:   "00:11:22:33:44:66",
		Type:  &eventType,
		Since: &since,
		Limit: 1,
	})

	require.Len(t, filtered, 1)
	assert.Equal(t, "01K7XJ7CD90000000000000000", filtered[0].ID.String())
}

func TestEventsGetCommandOutputsJSON(t *testing.T) {
	ctx := context.Background()
	fixture := writeEventCommandFixture(t)
	var output bytes.Buffer
	cmd := eventCommandFixtureCommand(fixture.configValues)
	cmd.Writer = &output

	require.NoError(t, cmd.Run(ctx, []string{"shoelaces", "events", "get", fixture.selectionID.String(), "--output", "json"}))

	var event eventOutputRecord
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, fixture.selectionID.String(), event.ID)
	assert.Equal(t, "user-selection", event.Type)
	assert.Equal(t, "new-host", event.Hostname)
	assert.Equal(t, "debian.ipxe", event.Script)
	assert.NotContains(t, output.String(), "role")
}

func TestEventsGetCommandRejectsInvalidULID(t *testing.T) {
	cmd := eventCommandFixtureCommand(nil)
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	err := cmd.Run(context.Background(), []string{"shoelaces", "events", "get", "not-a-ulid"})

	assert.ErrorContains(t, err, `invalid event id "not-a-ulid"`)
}

func TestEventsGetCommandReturnsMissingEventError(t *testing.T) {
	ctx := context.Background()
	fixture := writeEventCommandFixture(t)
	cmd := eventCommandFixtureCommand(fixture.configValues)
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard
	missingID := "01K7XJ7CDZ0000000000000000"

	err := cmd.Run(ctx, []string{"shoelaces", "events", "get", missingID})

	assert.ErrorContains(t, err, "event not found: "+missingID)
}

func TestParseEventType(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr string
	}{
		{name: "name", raw: "host-poll", want: int(event.HostPoll)},
		{name: "alias", raw: "selection", want: int(event.UserSelection)},
		{name: "number", raw: "2", want: int(event.HostBoot)},
		{name: "invalid", raw: "other", wantErr: `unsupported event type "other"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseEventType(test.raw)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

type eventCommandFixture struct {
	configValues map[any]any
	oldID        ulid.ULID
	selectionID  ulid.ULID
	bootID       ulid.ULID
}

func writeEventCommandFixture(t *testing.T) eventCommandFixture {
	t.Helper()

	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "runtime", "shoelaces.db")
	store, err := persistencesqlite.Open(ctx, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	now := time.Unix(1700000000, 0).UTC()
	oldID := appendEventFixture(t, store, persistence.EventRecord{
		ID:         mustParseULID(t, "01K7XJ7CD80000000000000000"),
		Type:       int(event.HostPoll),
		OccurredAt: now.Add(-2 * time.Hour),
		MAC:        "00:11:22:33:44:55",
		IP:         "192.0.2.10",
		Hostname:   "old-host",
		Message:    "old host polled",
		ParamsJSON: []byte(`{"token":"secret-token"}`),
	})
	selectionID := appendEventFixture(t, store, persistence.EventRecord{
		ID:         mustParseULID(t, "01K7XJ7CD90000000000000000"),
		Type:       int(event.UserSelection),
		OccurredAt: now.Add(-time.Hour),
		MAC:        "00:11:22:33:44:66",
		IP:         "192.0.2.11",
		Hostname:   "new-host",
		BootType:   event.ManualBoot,
		Script:     "debian.ipxe",
		Message:    "selected debian.ipxe",
		ParamsJSON: []byte(`{"role":"db"}`),
	})
	bootID := appendEventFixture(t, store, persistence.EventRecord{
		ID:         mustParseULID(t, "01K7XJ7CDA0000000000000000"),
		Type:       int(event.HostBoot),
		OccurredAt: now,
		MAC:        "00:11:22:33:44:66",
		IP:         "192.0.2.11",
		Hostname:   "new-host",
		BootType:   event.SubnetMatchBoot,
		Script:     "debian.ipxe",
		Message:    "new host booted",
		ParamsJSON: []byte(`{"role":"web"}`),
	})

	return eventCommandFixture{
		configValues: map[any]any{
			"data-dir":                     dataDir,
			"persistence-backend":          persistence.BackendSQLite,
			"persistence-path":             filepath.Join("runtime", "shoelaces.db"),
			"persistence-retention-events": "720h",
		},
		oldID:       oldID,
		selectionID: selectionID,
		bootID:      bootID,
	}
}

func appendEventFixture(t *testing.T, store persistence.Store, record persistence.EventRecord) ulid.ULID {
	t.Helper()

	id, err := store.AppendEvent(context.Background(), record)
	require.NoError(t, err)
	return id
}

func eventCommandFixtureCommand(configValues map[any]any) *cli.Command {
	return command("", configValues, func(env *environment.Environment) error {
		return nil
	})
}

func eventRecordFixture(t *testing.T, id string, eventType event.Type, occurredAt time.Time, mac string) persistence.EventRecord {
	t.Helper()

	return persistence.EventRecord{
		ID:         mustParseULID(t, id),
		Type:       int(eventType),
		OccurredAt: occurredAt,
		MAC:        mac,
	}
}

func mustParseULID(t *testing.T, id string) ulid.ULID {
	t.Helper()

	parsed, err := ulid.Parse(id)
	require.NoError(t, err)
	return parsed
}
