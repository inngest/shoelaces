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

package persistence

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// EventCommands owns write-side event mutations.
type EventCommands interface {
	// AppendEvent persists an already-redacted event and returns its durable ID.
	AppendEvent(ctx context.Context, event EventRecord) (ulid.ULID, error)
	// DeleteEventsBefore removes events older than the cutoff.
	DeleteEventsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// EventQueries owns read-side event lookups.
type EventQueries interface {
	// ListEvents returns persisted events ordered by occurrence time.
	ListEvents(ctx context.Context) ([]EventRecord, error)
}

// ServerStateCommands owns write-side waiting/manual boot state mutations.
type ServerStateCommands interface {
	// UpsertServerState creates or replaces the state for a host MAC.
	UpsertServerState(ctx context.Context, state ServerStateRecord) error
	// DeleteServerState removes state for one host MAC.
	DeleteServerState(ctx context.Context, mac string) (int64, error)
	// DeleteServerStatesBefore removes inactive host states older than the cutoff.
	DeleteServerStatesBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// ServerStateQueries owns read-side waiting/manual boot state lookups.
type ServerStateQueries interface {
	// GetServerState returns the state for a host MAC.
	GetServerState(ctx context.Context, mac string) (ServerStateRecord, error)
	// ListServerStates returns all persisted host states ordered by MAC.
	ListServerStates(ctx context.Context) ([]ServerStateRecord, error)
}

// BootSessionCommands owns write-side boot/config reference mutations.
type BootSessionCommands interface {
	// CreateBootSession stores a resolved boot/config reference.
	CreateBootSession(ctx context.Context, session BootSessionRecord) error
	// DeleteBootSessionsBefore removes sessions that expired before the cutoff.
	DeleteBootSessionsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// BootSessionQueries owns read-side boot/config reference lookups.
type BootSessionQueries interface {
	// GetBootSession returns an unexpired boot/config reference.
	GetBootSession(ctx context.Context, ref string, now time.Time) (BootSessionRecord, error)
}

// Commands groups all write-side persistence interfaces.
type Commands interface {
	EventCommands
	ServerStateCommands
	BootSessionCommands
}

// Queries groups all read-side persistence interfaces.
type Queries interface {
	EventQueries
	ServerStateQueries
	BootSessionQueries
}

// Store is the full CQRS persistence boundary exposed by a backend.
type Store interface {
	Commands
	Queries
	Close() error
}
