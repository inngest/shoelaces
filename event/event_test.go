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

package event

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/persistence/memory"
	"github.com/inngest/shoelaces/server"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expectedEvent = `{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","eventType":0,"occurred_at":"1970-01-01T00:00:00Z","server":{"Mac":"","IP":"","Hostname":"test_host"},"bootType":"Manual","script":"freebsd.ipxe","message":"","params":{"baseURL":"localhost:8080","cloudconfig":"virtual","hostname":"","version":"12345"}}`

func TestNew(t *testing.T) {
	event := New(HostPoll, server.Server{Mac: "", IP: "", Hostname: "test_host"}, PtrMatchBoot, "msdos.ipxe", map[string]any{"test": "testParam"})
	assert.Equal(t, HostPoll, event.Type)
	assert.Equal(t, "test_host", event.Server.Hostname)
	assert.Equal(t, PtrMatchBoot, event.BootType)
	assert.Equal(t, "msdos.ipxe", event.Script)
	assert.False(t, event.ID.IsZero())
	require.Len(t, event.Params, 1)
	assert.Equal(t, "testParam", event.Params["test"])

	now := time.Now()
	assert.False(t, event.OccurredAt.After(now))
}

func TestNewSetsMessage(t *testing.T) {
	tests := []struct {
		name       string
		eventType  Type
		bootType   string
		script     string
		params     map[string]any
		wantSubstr string
	}{
		{
			name:       "host poll",
			eventType:  HostPoll,
			wantSubstr: "Host test_host polled for a script.",
		},
		{
			name:       "user selection",
			eventType:  UserSelection,
			script:     "freebsd.ipxe",
			wantSubstr: "A user selected freebsd.ipxe for the host test_host.",
		},
		{
			name:       "host boot",
			eventType:  HostBoot,
			bootType:   ManualBoot,
			params:     map[string]any{"hostname": "test_host"},
			wantSubstr: "Host test_host booted using Manual method",
		},
		{
			name:       "host timeout",
			eventType:  HostTimeout,
			wantSubstr: "Host test_host timed out.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := New(tt.eventType, server.Server{Hostname: "test_host"}, tt.bootType, tt.script, tt.params)

			assert.Contains(t, event.Message, tt.wantSubstr)
		})
	}
}

func TestLogAddEventInitializesAndAppendsEvents(t *testing.T) {
	log := &Log{}
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "test_host")

	log.AddEvent(HostPoll, srv, "", "", nil)
	log.AddEvent(UserSelection, srv, "", "freebsd.ipxe", nil)

	events := eventsForMAC(t, log, srv.Mac)
	require.Len(t, events, 2)
	assert.Equal(t, HostPoll, events[0].Type)
	assert.Equal(t, UserSelection, events[1].Type)
	assert.Equal(t, "freebsd.ipxe", events[1].Script)
}

func TestLogPersistsAndGroupsEventsByMAC(t *testing.T) {
	store := memory.New()
	log := NewLog(store, store)
	firstMAC := "06:66:de:ad:be:ef"
	secondMAC := "06:66:de:ad:be:f0"

	log.now = fixedClock(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 2, 0, 0, time.UTC),
	)
	log.newID = fixedIDs(
		ulid.MustParse("01K7XJ7CD80000000000000000"),
		ulid.MustParse("01K7XJ7CD80000000000000001"),
		ulid.MustParse("01K7XJ7CD80000000000000002"),
	)
	require.NoError(t, log.AppendEvent(context.Background(), HostPoll, server.New(firstMAC, "192.0.2.10", "first"), "", "", nil))
	require.NoError(t, log.AppendEvent(context.Background(), HostBoot, server.New(secondMAC, "192.0.2.11", "second"), SubnetMatchBoot, "debian.ipxe", map[string]any{"role": "db"}))
	require.NoError(t, log.AppendEvent(context.Background(), UserSelection, server.New(firstMAC, "192.0.2.10", "first"), "", "ubuntu.ipxe", nil))

	grouped, err := log.ListEvents(context.Background())
	require.NoError(t, err)
	require.Len(t, grouped[firstMAC], 2)
	require.Len(t, grouped[secondMAC], 1)
	assert.Equal(t, HostPoll, grouped[firstMAC][0].Type)
	assert.Equal(t, UserSelection, grouped[firstMAC][1].Type)
	assert.Equal(t, HostBoot, grouped[secondMAC][0].Type)
	assert.Equal(t, ulid.MustParse("01K7XJ7CD80000000000000000"), grouped[firstMAC][0].ID)
	assert.Equal(t, "debian.ipxe", grouped[secondMAC][0].Script)
	assert.Equal(t, map[string]any{"role": "db"}, grouped[secondMAC][0].Params)
}

func TestLogPersistsRedactedParams(t *testing.T) {
	store := memory.New()
	log := NewLog(store, store)
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "test_host")

	require.NoError(t, log.AppendEvent(context.Background(), HostBoot, srv, ManualBoot, "debian.ipxe", map[string]any{
		"hostname":              "test_host",
		"root_password_crypted": "hash",
		"bootstrap_token":       "token-value",
	}))

	events := eventsForMAC(t, log, srv.Mac)
	require.Len(t, events, 1)
	assert.Equal(t, "test_host", events[0].Params["hostname"])
	assert.Equal(t, "[REDACTED]", events[0].Params["root_password_crypted"])
	assert.Equal(t, "[REDACTED]", events[0].Params["bootstrap_token"])
	assert.NotContains(t, events[0].Message, "hash")
	assert.NotContains(t, events[0].Message, "token-value")
}

func TestLogDeleteEventsBefore(t *testing.T) {
	store := memory.New()
	log := NewLog(store, store)
	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	_, err := store.AppendEvent(context.Background(), persistence.EventRecord{
		Type:       int(HostPoll),
		OccurredAt: oldTime,
		MAC:        "06:66:de:ad:be:ef",
		Message:    "old",
	})
	require.NoError(t, err)
	_, err = store.AppendEvent(context.Background(), persistence.EventRecord{
		Type:       int(HostBoot),
		OccurredAt: newTime,
		MAC:        "06:66:de:ad:be:f0",
		Message:    "new",
	})
	require.NoError(t, err)

	deleted, err := log.DeleteEventsBefore(context.Background(), cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	grouped, err := log.ListEvents(context.Background())
	require.NoError(t, err)
	assert.Empty(t, grouped["06:66:de:ad:be:ef"])
	require.Len(t, grouped["06:66:de:ad:be:f0"], 1)
	assert.Equal(t, "new", grouped["06:66:de:ad:be:f0"][0].Message)
}

func TestHostBootEventRedactsSensitiveParams(t *testing.T) {
	event := New(HostBoot, server.Server{Hostname: "test_host"}, ManualBoot, "debian.ipxe", map[string]any{
		"hostname":              "test_host",
		"root_password_crypted": "hash",
		"bootstrap_token":       "token-value",
		"ssh_private_key":       "private-key",
	})

	assert.Equal(t, "test_host", event.Params["hostname"])
	assert.Equal(t, "[REDACTED]", event.Params["root_password_crypted"])
	assert.Equal(t, "[REDACTED]", event.Params["bootstrap_token"])
	assert.Equal(t, "[REDACTED]", event.Params["ssh_private_key"])
	assert.NotContains(t, event.Message, "hash")
	assert.NotContains(t, event.Message, "token-value")
	assert.NotContains(t, event.Message, "private-key")
	assert.Contains(t, event.Message, "[REDACTED]")
}

func TestEventMarshalJSON(t *testing.T) {
	event := Event{
		ID:         ulid.MustParse("01ARZ3NDEKTSV4RRFFQ69G5FAV"),
		Type:       HostPoll,
		OccurredAt: time.Unix(0, 0).UTC(),
		Server:     server.Server{Mac: "", IP: "", Hostname: "test_host"},
		BootType:   ManualBoot,
		Script:     "freebsd.ipxe",
		Message:    "",
		Params: map[string]any{
			"baseURL":     "localhost:8080",
			"cloudconfig": "virtual",
			"hostname":    "",
			"version":     "12345",
		},
	}
	marshaled, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Equal(t, expectedEvent, string(marshaled))
}

func eventsForMAC(t *testing.T, log *Log, mac string) []Event {
	t.Helper()

	grouped, err := log.ListEvents(context.Background())
	require.NoError(t, err)
	return grouped[mac]
}

func fixedClock(times ...time.Time) func() time.Time {
	var index int
	return func() time.Time {
		if index >= len(times) {
			return times[len(times)-1]
		}
		now := times[index]
		index++
		return now
	}
}

func fixedIDs(ids ...ulid.ULID) func() ulid.ULID {
	var index int
	return func() ulid.ULID {
		if index >= len(ids) {
			return ids[len(ids)-1]
		}
		id := ids[index]
		index++
		return id
	}
}
