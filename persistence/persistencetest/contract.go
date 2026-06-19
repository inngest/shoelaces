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

package persistencetest

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/inngest/shoelaces/persistence"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StoreUnderTest is the behavior every persistence backend must provide.
type StoreUnderTest interface {
	persistence.Store
}

// StoreFactory creates a fresh persistence store for one contract test.
type StoreFactory func(t *testing.T) StoreUnderTest

// RunStoreContract verifies the backend-neutral CQRS persistence behavior.
func RunStoreContract(t *testing.T, factory StoreFactory) {
	t.Helper()

	t.Run("events", func(t *testing.T) {
		store := factory(t)
		t.Cleanup(func() { require.NoError(t, store.Close()) })

		ctx := context.Background()
		now := time.Unix(1700000000, 0).UTC()

		firstID, err := store.AppendEvent(ctx, persistence.EventRecord{
			Type:       1,
			OccurredAt: now.Add(-time.Hour),
			MAC:        "00:11:22:33:44:55",
			IP:         "192.0.2.10",
			Hostname:   "old-host",
			Message:    "old event",
			ParamsJSON: []byte(`{"token":"<redacted>"}`),
		})
		require.NoError(t, err)
		assert.False(t, firstID.IsZero())

		secondID, err := store.AppendEvent(ctx, persistence.EventRecord{
			Type:       2,
			OccurredAt: now,
			MAC:        "00:11:22:33:44:66",
			IP:         "192.0.2.11",
			Hostname:   "new-host",
			BootType:   "Subnet Match",
			Script:     "debian.ipxe",
			Message:    "new event",
			ParamsJSON: []byte(`{"role":"db"}`),
		})
		require.NoError(t, err)
		assert.False(t, secondID.IsZero())
		assert.NotEqual(t, firstID, secondID)

		events, err := store.ListEvents(ctx)
		require.NoError(t, err)
		require.Len(t, events, 2)
		assert.Equal(t, firstID, events[0].ID)
		assert.Equal(t, secondID, events[1].ID)
		assert.Equal(t, "old-host", events[0].Hostname)
		assert.Equal(t, []byte(`{"role":"db"}`), events[1].ParamsJSON)

		got, err := store.GetEvent(ctx, secondID)
		require.NoError(t, err)
		assert.Equal(t, secondID, got.ID)
		assert.Equal(t, "new-host", got.Hostname)
		assert.Equal(t, []byte(`{"role":"db"}`), got.ParamsJSON)

		_, err = store.GetEvent(ctx, ulid.Make())
		assert.ErrorIs(t, err, sql.ErrNoRows)

		deleted, err := store.DeleteEventsBefore(ctx, now.Add(-time.Minute))
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		events, err = store.ListEvents(ctx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "new-host", events[0].Hostname)
	})

	t.Run("server states", func(t *testing.T) {
		store := factory(t)
		t.Cleanup(func() { require.NoError(t, store.Close()) })

		ctx := context.Background()
		now := time.Unix(1700000000, 0).UTC()

		state := persistence.ServerStateRecord{
			MAC:                "00:11:22:33:44:55",
			IP:                 "192.0.2.10",
			Hostname:           "waiting-host",
			Target:             "NOTARGET",
			ParamsJSON:         []byte(`{"baseURL":"shoelaces"}`),
			UsersJSON:          []byte(`{"root":{"system":true}}`),
			ProvisioningJSON:   []byte(`{"storage":{"disk":"/dev/sda"}}`),
			AllowedTargetsJSON: []byte(`[{"name":"debian12"}]`),
			Retry:              1,
			LastAccess:         now.Add(-time.Hour),
		}
		require.NoError(t, store.UpsertServerState(ctx, state))

		state.Target = "debian.ipxe"
		state.Environment = "production"
		state.Retry = 2
		state.LastAccess = now
		require.NoError(t, store.UpsertServerState(ctx, state))

		got, err := store.GetServerState(ctx, state.MAC)
		require.NoError(t, err)
		assert.Equal(t, "debian.ipxe", got.Target)
		assert.Equal(t, "production", got.Environment)
		assert.Equal(t, int64(2), got.Retry)
		assert.Equal(t, []byte(`[{"name":"debian12"}]`), got.AllowedTargetsJSON)

		states, err := store.ListServerStates(ctx)
		require.NoError(t, err)
		require.Len(t, states, 1)

		deleted, err := store.DeleteServerStatesBefore(ctx, now.Add(-time.Minute))
		require.NoError(t, err)
		assert.Equal(t, int64(0), deleted)

		deleted, err = store.DeleteServerState(ctx, state.MAC)
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		_, err = store.GetServerState(ctx, state.MAC)
		assert.True(t, errors.Is(err, sql.ErrNoRows))
	})

	t.Run("boot sessions", func(t *testing.T) {
		store := factory(t)
		t.Cleanup(func() { require.NoError(t, store.Close()) })

		ctx := context.Background()
		now := time.Unix(1700000000, 0).UTC()
		session := persistence.BootSessionRecord{
			Ref:              "ref-123",
			MAC:              "00:11:22:33:44:55",
			IP:               "192.0.2.10",
			Hostname:         "install-host",
			Target:           "debian.ipxe",
			Environment:      "production",
			ParamsJSON:       []byte(`{"release":"trixie"}`),
			UsersJSON:        []byte(`{"infra":{"primary":true}}`),
			ProvisioningJSON: []byte(`{"repos":{"release":"trixie"}}`),
			CreatedAt:        now,
			ExpiresAt:        now.Add(time.Hour),
		}
		require.NoError(t, store.CreateBootSession(ctx, session))

		got, err := store.GetBootSession(ctx, "ref-123", now)
		require.NoError(t, err)
		assert.Equal(t, "install-host", got.Hostname)
		assert.Equal(t, []byte(`{"release":"trixie"}`), got.ParamsJSON)

		_, err = store.GetBootSession(ctx, "ref-123", now.Add(2*time.Hour))
		assert.True(t, errors.Is(err, sql.ErrNoRows))

		deleted, err := store.DeleteBootSessionsBefore(ctx, now.Add(2*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)
	})
}
