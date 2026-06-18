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

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence"
)

// ErrStateNotFound is returned when a host MAC has no waiting/manual state.
var ErrStateNotFound = sql.ErrNoRows

// PersistentStateStore adapts the persistence CQRS server-state interfaces to
// the polling-facing StateStore interface.
type PersistentStateStore struct {
	commands persistence.ServerStateCommands
	queries  persistence.ServerStateQueries
	now      func() time.Time
}

// NewPersistentStateStore returns a server-state store backed by persistence.
func NewPersistentStateStore(commands persistence.ServerStateCommands, queries persistence.ServerStateQueries) *PersistentStateStore {
	return &PersistentStateStore{
		commands: commands,
		queries:  queries,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// QueueServer records a host that is waiting for manual target selection.
func (s *PersistentStateStore) QueueServer(ctx context.Context, srv Server, allowedTargets []TargetOption) error {
	srv.AllowedTargets = copyTargetOptions(allowedTargets)
	return s.SaveState(ctx, &State{
		Server:     srv,
		Target:     InitTarget,
		Retry:      1,
		LastAccess: int(s.now().Unix()),
	})
}

// GetState loads and decodes one persisted host state by MAC.
func (s *PersistentStateStore) GetState(ctx context.Context, mac string) (*State, error) {
	record, err := s.queries.GetServerState(ctx, mac)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStateNotFound
		}
		return nil, err
	}
	return stateFromRecord(record)
}

// SaveState encodes and persists a complete host state snapshot.
func (s *PersistentStateStore) SaveState(ctx context.Context, state *State) error {
	record, err := recordFromState(state)
	if err != nil {
		return err
	}
	return s.commands.UpsertServerState(ctx, record)
}

// DeleteState removes one persisted host state by MAC.
func (s *PersistentStateStore) DeleteState(ctx context.Context, mac string) error {
	_, err := s.commands.DeleteServerState(ctx, mac)
	return err
}

// ListWaiting returns persisted hosts that still need manual target selection.
func (s *PersistentStateStore) ListWaiting(ctx context.Context) (Servers, error) {
	records, err := s.queries.ListServerStates(ctx)
	if err != nil {
		return nil, err
	}
	waiting := make(Servers, 0, len(records))
	for _, record := range records {
		state, err := stateFromRecord(record)
		if err != nil {
			return nil, err
		}
		if state.Target == InitTarget {
			waiting = append(waiting, state.Server)
		}
	}
	sortServers(waiting)
	return waiting, nil
}

// DeleteStatesBefore removes persisted host states older than cutoff.
func (s *PersistentStateStore) DeleteStatesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.commands.DeleteServerStatesBefore(ctx, cutoff)
}

func recordFromState(state *State) (persistence.ServerStateRecord, error) {
	if state == nil {
		return persistence.ServerStateRecord{}, errors.New("server state is nil")
	}
	paramsJSON, err := marshalStateJSON(state.Params, "{}")
	if err != nil {
		return persistence.ServerStateRecord{}, fmt.Errorf("marshal params: %w", err)
	}
	usersJSON, err := marshalStateJSON(state.Users, "{}")
	if err != nil {
		return persistence.ServerStateRecord{}, fmt.Errorf("marshal users: %w", err)
	}
	provisioningJSON, err := marshalStateJSON(state.Provisioning, "{}")
	if err != nil {
		return persistence.ServerStateRecord{}, fmt.Errorf("marshal provisioning: %w", err)
	}
	allowedTargetsJSON, err := marshalStateJSON(state.AllowedTargets, "[]")
	if err != nil {
		return persistence.ServerStateRecord{}, fmt.Errorf("marshal allowed targets: %w", err)
	}

	return persistence.ServerStateRecord{
		MAC:                state.Mac,
		IP:                 state.IP,
		Hostname:           state.Hostname,
		Target:             state.Target,
		Environment:        state.Environment,
		ParamsJSON:         paramsJSON,
		UsersJSON:          usersJSON,
		ProvisioningJSON:   provisioningJSON,
		AllowedTargetsJSON: allowedTargetsJSON,
		Retry:              int64(state.Retry),
		LastAccess:         time.Unix(int64(state.LastAccess), 0).UTC(),
	}, nil
}

func stateFromRecord(record persistence.ServerStateRecord) (*State, error) {
	state := &State{
		Server: Server{
			Mac:      record.MAC,
			IP:       record.IP,
			Hostname: record.Hostname,
		},
		Target:      record.Target,
		Environment: record.Environment,
		Retry:       int(record.Retry),
		LastAccess:  int(record.LastAccess.UTC().Unix()),
	}
	if err := unmarshalStateJSON(record.ParamsJSON, &state.Params); err != nil {
		return nil, fmt.Errorf("unmarshal params: %w", err)
	}
	if err := unmarshalStateJSON(record.UsersJSON, &state.Users); err != nil {
		return nil, fmt.Errorf("unmarshal users: %w", err)
	}
	if err := unmarshalStateJSON(record.ProvisioningJSON, &state.Provisioning); err != nil {
		return nil, fmt.Errorf("unmarshal provisioning: %w", err)
	}
	if err := unmarshalStateJSON(record.AllowedTargetsJSON, &state.AllowedTargets); err != nil {
		return nil, fmt.Errorf("unmarshal allowed targets: %w", err)
	}
	return state, nil
}

func marshalStateJSON(value any, empty string) ([]byte, error) {
	if value == nil {
		return []byte(empty), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func unmarshalStateJSON(encoded []byte, value any) error {
	if len(encoded) == 0 {
		return nil
	}
	return json.Unmarshal(encoded, value)
}

func copyState(state *State) *State {
	if state == nil {
		return nil
	}
	copied := *state
	copied.AllowedTargets = copyTargetOptions(state.AllowedTargets)
	copied.Params = copyParams(state.Params)
	copied.Users = copyUsers(state.Users)
	copied.Provisioning = state.Provisioning
	return &copied
}

func copyParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	copied := make(map[string]interface{}, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return copied
}

func copyUsers(users map[string]mappings.ResolvedUser) map[string]mappings.ResolvedUser {
	if users == nil {
		return nil
	}
	copied := make(map[string]mappings.ResolvedUser, len(users))
	for key, value := range users {
		value.SSHAuthorizedKeys = append([]string(nil), value.SSHAuthorizedKeys...)
		value.Groups = append([]string(nil), value.Groups...)
		copied[key] = value
	}
	return copied
}

func sortServers(servers Servers) {
	sort.Sort(servers)
}
