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
	"errors"
	"fmt"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/persistence"
	persistencesqlite "github.com/inngest/shoelaces/persistence/sqlite"
)

type inspectionQueries interface {
	persistence.Queries
	Close() error
}

type inspectionStore struct {
	Queries inspectionQueries
	Path    string
}

type inspectionStoreOpener func(context.Context, string) (inspectionQueries, error)

func openInspectionStore(ctx context.Context, options environment.Options) (*inspectionStore, error) {
	return openInspectionStoreUsing(ctx, options, func(ctx context.Context, path string) (inspectionQueries, error) {
		return persistencesqlite.OpenReadOnly(ctx, path)
	})
}

func openInspectionStoreUsing(ctx context.Context, options environment.Options, open inspectionStoreOpener) (*inspectionStore, error) {
	config := persistence.ApplyDefaults(options.Persistence)
	if err := persistence.Validate(config); err != nil {
		return nil, err
	}
	if config.Backend != persistence.BackendSQLite {
		return nil, fmt.Errorf("runtime inspection requires sqlite persistence backend; configured backend is %q", config.Backend)
	}
	if options.DataDir == "" {
		return nil, fmt.Errorf("you must specify the data-dir parameter")
	}

	path := persistence.ResolvePath(options.DataDir, config)
	store, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	return &inspectionStore{
		Queries: store,
		Path:    path,
	}, nil
}

func withInspectionStore(ctx context.Context, options environment.Options, fn func(*inspectionStore) error) error {
	return withInspectionStoreUsing(ctx, options, openInspectionStore, fn)
}

func withInspectionStoreUsing(ctx context.Context, options environment.Options, open func(context.Context, environment.Options) (*inspectionStore, error), fn func(*inspectionStore) error) error {
	store, err := open(ctx, options)
	if err != nil {
		return err
	}

	fnErr := fn(store)
	closeErr := store.Queries.Close()
	if fnErr != nil || closeErr != nil {
		return errors.Join(fnErr, closeErr)
	}
	return nil
}
