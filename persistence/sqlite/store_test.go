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

package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/inngest/shoelaces/persistence/persistencetest"
	"github.com/inngest/shoelaces/persistence/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestStoreContract(t *testing.T) {
	persistencetest.RunStoreContract(t, func(t *testing.T) persistencetest.StoreUnderTest {
		t.Helper()
		store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "runtime", "shoelaces.db"))
		require.NoError(t, err)
		return store
	})
}

func TestOpenAppliesSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runtime", "shoelaces.db")
	store, err := sqlite.Open(context.Background(), dbPath)
	require.NoError(t, err)
	require.NoError(t, store.Close())

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	for _, table := range []string{"goose_db_version", "events", "server_states", "boot_sessions"} {
		t.Run(table, func(t *testing.T) {
			var name string
			err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
			require.NoError(t, err)
			assert.Equal(t, table, name)
		})
	}

	for _, index := range []string{"events_occurred_at_idx", "server_states_last_access_idx", "boot_sessions_expires_at_idx"} {
		t.Run(index, func(t *testing.T) {
			var name string
			err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&name)
			require.NoError(t, err)
			assert.Equal(t, index, name)
		})
	}

	var version int
	require.NoError(t, db.QueryRow(`SELECT version_id FROM goose_db_version WHERE version_id = 1 AND is_applied = 1`).Scan(&version))
	assert.Equal(t, 1, version)
}

func TestOpenReturnsDirectoryCreationError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "runtime")
	require.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0o644))

	_, err := sqlite.Open(context.Background(), filepath.Join(blocker, "shoelaces.db"))
	assert.ErrorContains(t, err, "create sqlite parent directory")
}
