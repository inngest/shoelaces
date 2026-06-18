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

package memory

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/inngest/shoelaces/persistence"
)

// store is an in-process implementation of the persistence CQRS interfaces.
type store struct {
	mu           sync.RWMutex
	nextEventID  int64
	events       []persistence.EventRecord
	serverStates map[string]persistence.ServerStateRecord
	bootSessions map[string]persistence.BootSessionRecord
}

// New returns an empty memory-backed persistence store.
func New() persistence.Store {
	return &store{
		serverStates: make(map[string]persistence.ServerStateRecord),
		bootSessions: make(map[string]persistence.BootSessionRecord),
	}
}

// AppendEvent persists an event in memory.
func (s *store) AppendEvent(_ context.Context, event persistence.EventRecord) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextEventID++
	event.ID = s.nextEventID
	s.events = append(s.events, copyEvent(event))
	return event.ID, nil
}

// DeleteEventsBefore removes events older than the cutoff.
func (s *store) DeleteEventsBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := s.events[:0]
	var deleted int64
	for _, event := range s.events {
		if event.OccurredAt.Before(cutoff) {
			deleted++
			continue
		}
		kept = append(kept, event)
	}
	s.events = kept
	return deleted, nil
}

// ListEvents returns events in insertion order, matching occurrence order in tests.
func (s *store) ListEvents(_ context.Context) ([]persistence.EventRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]persistence.EventRecord, len(s.events))
	for i, event := range s.events {
		events[i] = copyEvent(event)
	}
	return events, nil
}

// UpsertServerState creates or replaces host state by MAC.
func (s *store) UpsertServerState(_ context.Context, state persistence.ServerStateRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.serverStates[state.MAC] = copyServerState(state)
	return nil
}

// DeleteServerState removes host state by MAC.
func (s *store) DeleteServerState(_ context.Context, mac string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.serverStates[mac]; !ok {
		return 0, nil
	}
	delete(s.serverStates, mac)
	return 1, nil
}

// DeleteServerStatesBefore removes host states older than the cutoff.
func (s *store) DeleteServerStatesBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64
	for mac, state := range s.serverStates {
		if state.LastAccess.Before(cutoff) {
			delete(s.serverStates, mac)
			deleted++
		}
	}
	return deleted, nil
}

// GetServerState returns host state by MAC.
func (s *store) GetServerState(_ context.Context, mac string) (persistence.ServerStateRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.serverStates[mac]
	if !ok {
		return persistence.ServerStateRecord{}, sql.ErrNoRows
	}
	return copyServerState(state), nil
}

// ListServerStates returns all host states.
func (s *store) ListServerStates(_ context.Context) ([]persistence.ServerStateRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]persistence.ServerStateRecord, 0, len(s.serverStates))
	for _, state := range s.serverStates {
		states = append(states, copyServerState(state))
	}
	return states, nil
}

// CreateBootSession stores a boot/config reference by opaque ref.
func (s *store) CreateBootSession(_ context.Context, session persistence.BootSessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.bootSessions[session.Ref] = copyBootSession(session)
	return nil
}

// DeleteBootSessionsBefore removes expired boot/config references.
func (s *store) DeleteBootSessionsBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64
	for ref, session := range s.bootSessions {
		if session.ExpiresAt.Before(cutoff) {
			delete(s.bootSessions, ref)
			deleted++
		}
	}
	return deleted, nil
}

// GetBootSession returns an unexpired boot/config reference by opaque ref.
func (s *store) GetBootSession(_ context.Context, ref string, now time.Time) (persistence.BootSessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.bootSessions[ref]
	if !ok || !session.ExpiresAt.After(now) {
		return persistence.BootSessionRecord{}, sql.ErrNoRows
	}
	return copyBootSession(session), nil
}

// Close releases memory store resources.
func (s *store) Close() error {
	return nil
}

func copyEvent(event persistence.EventRecord) persistence.EventRecord {
	event.ParamsJSON = append([]byte(nil), event.ParamsJSON...)
	return event
}

func copyServerState(state persistence.ServerStateRecord) persistence.ServerStateRecord {
	state.ParamsJSON = append([]byte(nil), state.ParamsJSON...)
	state.UsersJSON = append([]byte(nil), state.UsersJSON...)
	state.ProvisioningJSON = append([]byte(nil), state.ProvisioningJSON...)
	state.AllowedTargetsJSON = append([]byte(nil), state.AllowedTargetsJSON...)
	return state
}

func copyBootSession(session persistence.BootSessionRecord) persistence.BootSessionRecord {
	session.ParamsJSON = append([]byte(nil), session.ParamsJSON...)
	session.UsersJSON = append([]byte(nil), session.UsersJSON...)
	session.ProvisioningJSON = append([]byte(nil), session.ProvisioningJSON...)
	return session
}
