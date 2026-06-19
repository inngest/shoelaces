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
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/persistence"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenInspectionStoreResolvesRelativeSQLitePath(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	var openedPath string

	store, err := openInspectionStoreUsing(context.Background(), environment.Options{
		DataDir: dataDir,
		Persistence: persistence.Config{
			Backend: persistence.BackendSQLite,
			Path:    "runtime/inspect.db",
		},
	}, func(_ context.Context, path string) (inspectionQueries, error) {
		openedPath = path
		return &fakeInspectionQueries{}, nil
	})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(dataDir, "runtime", "inspect.db"), openedPath)
	assert.Equal(t, openedPath, store.Path)
}

func TestOpenInspectionStoreRejectsMemoryBackend(t *testing.T) {
	_, err := openInspectionStoreUsing(context.Background(), environment.Options{
		DataDir: t.TempDir(),
		Persistence: persistence.Config{
			Backend: persistence.BackendMemory,
		},
	}, func(_ context.Context, path string) (inspectionQueries, error) {
		t.Fatal("opener should not run for memory backend")
		return nil, nil
	})

	assert.ErrorContains(t, err, "runtime inspection requires sqlite persistence backend")
}

func TestOpenInspectionStoreRejectsInvalidBackend(t *testing.T) {
	_, err := openInspectionStoreUsing(context.Background(), environment.Options{
		DataDir: t.TempDir(),
		Persistence: persistence.Config{
			Backend: "postgres",
		},
	}, func(_ context.Context, path string) (inspectionQueries, error) {
		t.Fatal("opener should not run for invalid backend")
		return nil, nil
	})

	assert.ErrorContains(t, err, `unsupported persistence backend "postgres"`)
}

func TestWithInspectionStoreClosesOnSuccess(t *testing.T) {
	queries := &fakeInspectionQueries{}

	err := withInspectionStoreUsing(context.Background(), environment.Options{}, func(context.Context, environment.Options) (*inspectionStore, error) {
		return &inspectionStore{Queries: queries, Path: "inspect.db"}, nil
	}, func(store *inspectionStore) error {
		assert.Equal(t, "inspect.db", store.Path)
		return nil
	})
	require.NoError(t, err)
	assert.True(t, queries.closed)
}

func TestWithInspectionStoreClosesOnCallbackError(t *testing.T) {
	queries := &fakeInspectionQueries{}

	err := withInspectionStoreUsing(context.Background(), environment.Options{}, func(context.Context, environment.Options) (*inspectionStore, error) {
		return &inspectionStore{Queries: queries}, nil
	}, func(store *inspectionStore) error {
		return errors.New("callback failed")
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "callback failed")
	assert.True(t, queries.closed)
}

func TestWithInspectionStoreReturnsCloseError(t *testing.T) {
	queries := &fakeInspectionQueries{closeErr: errors.New("close failed")}

	err := withInspectionStoreUsing(context.Background(), environment.Options{}, func(context.Context, environment.Options) (*inspectionStore, error) {
		return &inspectionStore{Queries: queries}, nil
	}, func(store *inspectionStore) error {
		return errors.New("callback failed")
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "callback failed")
	assert.ErrorContains(t, err, "close failed")
	assert.True(t, queries.closed)
}

type fakeInspectionQueries struct {
	closed   bool
	closeErr error
}

func (f *fakeInspectionQueries) ListEvents(context.Context) ([]persistence.EventRecord, error) {
	return nil, nil
}

func (f *fakeInspectionQueries) GetEvent(context.Context, ulid.ULID) (persistence.EventRecord, error) {
	return persistence.EventRecord{}, sql.ErrNoRows
}

func (f *fakeInspectionQueries) GetServerState(context.Context, string) (persistence.ServerStateRecord, error) {
	return persistence.ServerStateRecord{}, sql.ErrNoRows
}

func (f *fakeInspectionQueries) ListServerStates(context.Context) ([]persistence.ServerStateRecord, error) {
	return nil, nil
}

func (f *fakeInspectionQueries) GetBootSession(context.Context, string, time.Time) (persistence.BootSessionRecord, error) {
	return persistence.BootSessionRecord{}, sql.ErrNoRows
}

func (f *fakeInspectionQueries) Close() error {
	f.closed = true
	return f.closeErr
}
