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

package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/inngest/shoelaces/persistence"
	sqlitedb "github.com/inngest/shoelaces/persistence/sqlite/db"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// store implements the persistence CQRS interfaces using SQLite.
type store struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Open creates the database parent directory, opens SQLite, and applies schema
// migrations before returning a store.
func Open(ctx context.Context, path string) (persistence.Store, error) {
	if err := persistence.EnsureParentDir(path); err != nil {
		return nil, fmt.Errorf("create sqlite parent directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	store := &store{
		db:      db,
		queries: sqlitedb.New(db),
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the underlying SQLite connection pool.
func (s *store) Close() error {
	return s.db.Close()
}

func (s *store) migrate(ctx context.Context) error {
	goose.SetBaseFS(migrationFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("configure goose sqlite dialect: %w", err)
	}
	if err := goose.UpContext(ctx, s.db, "migrations"); err != nil {
		return fmt.Errorf("apply sqlite migrations: %w", err)
	}
	return nil
}

// AppendEvent persists an event and returns its database ID.
func (s *store) AppendEvent(ctx context.Context, event persistence.EventRecord) (int64, error) {
	return s.queries.InsertEvent(ctx, sqlitedb.InsertEventParams{
		EventType:          int64(event.Type),
		OccurredAtUnixNano: unixNano(event.OccurredAt),
		Mac:                event.MAC,
		Ip:                 event.IP,
		Hostname:           event.Hostname,
		BootType:           event.BootType,
		Script:             event.Script,
		Message:            event.Message,
		ParamsJson:         cloneBytes(event.ParamsJSON),
	})
}

// DeleteEventsBefore removes events older than the cutoff.
func (s *store) DeleteEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.queries.DeleteEventsBefore(ctx, unixNano(cutoff))
}

// ListEvents returns persisted events ordered by occurrence time.
func (s *store) ListEvents(ctx context.Context) ([]persistence.EventRecord, error) {
	rows, err := s.queries.ListEvents(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]persistence.EventRecord, len(rows))
	for i, row := range rows {
		events[i] = persistence.EventRecord{
			ID:         row.ID,
			Type:       int(row.EventType),
			OccurredAt: fromUnixNano(row.OccurredAtUnixNano),
			MAC:        row.Mac,
			IP:         row.Ip,
			Hostname:   row.Hostname,
			BootType:   row.BootType,
			Script:     row.Script,
			Message:    row.Message,
			ParamsJSON: cloneBytes(row.ParamsJson),
		}
	}
	return events, nil
}

// UpsertServerState creates or replaces host state by MAC.
func (s *store) UpsertServerState(ctx context.Context, state persistence.ServerStateRecord) error {
	return s.queries.UpsertServerState(ctx, sqlitedb.UpsertServerStateParams{
		Mac:                state.MAC,
		Ip:                 state.IP,
		Hostname:           state.Hostname,
		Target:             state.Target,
		Environment:        state.Environment,
		ParamsJson:         cloneBytes(state.ParamsJSON),
		UsersJson:          cloneBytes(state.UsersJSON),
		ProvisioningJson:   cloneBytes(state.ProvisioningJSON),
		AllowedTargetsJson: cloneBytes(state.AllowedTargetsJSON),
		Retry:              state.Retry,
		LastAccessUnixNano: unixNano(state.LastAccess),
	})
}

// DeleteServerState removes host state by MAC.
func (s *store) DeleteServerState(ctx context.Context, mac string) (int64, error) {
	return s.queries.DeleteServerState(ctx, mac)
}

// DeleteServerStatesBefore removes host states older than the cutoff.
func (s *store) DeleteServerStatesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.queries.DeleteServerStatesBefore(ctx, unixNano(cutoff))
}

// GetServerState returns host state by MAC.
func (s *store) GetServerState(ctx context.Context, mac string) (persistence.ServerStateRecord, error) {
	row, err := s.queries.GetServerState(ctx, mac)
	if err != nil {
		return persistence.ServerStateRecord{}, err
	}
	return serverStateFromRow(row), nil
}

// ListServerStates returns all host states ordered by MAC.
func (s *store) ListServerStates(ctx context.Context) ([]persistence.ServerStateRecord, error) {
	rows, err := s.queries.ListServerStates(ctx)
	if err != nil {
		return nil, err
	}
	states := make([]persistence.ServerStateRecord, len(rows))
	for i, row := range rows {
		states[i] = serverStateFromRow(row)
	}
	return states, nil
}

// CreateBootSession stores a boot/config reference by opaque ref.
func (s *store) CreateBootSession(ctx context.Context, session persistence.BootSessionRecord) error {
	return s.queries.CreateBootSession(ctx, sqlitedb.CreateBootSessionParams{
		Ref:               session.Ref,
		Mac:               session.MAC,
		Ip:                session.IP,
		Hostname:          session.Hostname,
		Target:            session.Target,
		Environment:       session.Environment,
		ParamsJson:        cloneBytes(session.ParamsJSON),
		UsersJson:         cloneBytes(session.UsersJSON),
		ProvisioningJson:  cloneBytes(session.ProvisioningJSON),
		CreatedAtUnixNano: unixNano(session.CreatedAt),
		ExpiresAtUnixNano: unixNano(session.ExpiresAt),
	})
}

// DeleteBootSessionsBefore removes expired boot/config references.
func (s *store) DeleteBootSessionsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.queries.DeleteBootSessionsBefore(ctx, unixNano(cutoff))
}

// GetBootSession returns an unexpired boot/config reference by opaque ref.
func (s *store) GetBootSession(ctx context.Context, ref string, now time.Time) (persistence.BootSessionRecord, error) {
	row, err := s.queries.GetBootSession(ctx, sqlitedb.GetBootSessionParams{
		Ref:               ref,
		ExpiresAtUnixNano: unixNano(now),
	})
	if err != nil {
		return persistence.BootSessionRecord{}, err
	}
	return persistence.BootSessionRecord{
		Ref:              row.Ref,
		MAC:              row.Mac,
		IP:               row.Ip,
		Hostname:         row.Hostname,
		Target:           row.Target,
		Environment:      row.Environment,
		ParamsJSON:       cloneBytes(row.ParamsJson),
		UsersJSON:        cloneBytes(row.UsersJson),
		ProvisioningJSON: cloneBytes(row.ProvisioningJson),
		CreatedAt:        fromUnixNano(row.CreatedAtUnixNano),
		ExpiresAt:        fromUnixNano(row.ExpiresAtUnixNano),
	}, nil
}

func serverStateFromRow(row sqlitedb.ServerState) persistence.ServerStateRecord {
	return persistence.ServerStateRecord{
		MAC:                row.Mac,
		IP:                 row.Ip,
		Hostname:           row.Hostname,
		Target:             row.Target,
		Environment:        row.Environment,
		ParamsJSON:         cloneBytes(row.ParamsJson),
		UsersJSON:          cloneBytes(row.UsersJson),
		ProvisioningJSON:   cloneBytes(row.ProvisioningJson),
		AllowedTargetsJSON: cloneBytes(row.AllowedTargetsJson),
		Retry:              row.Retry,
		LastAccess:         fromUnixNano(row.LastAccessUnixNano),
	}
}

func unixNano(t time.Time) int64 {
	return t.UTC().UnixNano()
}

func fromUnixNano(n int64) time.Time {
	return time.Unix(0, n).UTC()
}

func cloneBytes(value []byte) []byte {
	return append([]byte(nil), value...)
}
