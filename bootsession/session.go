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

package bootsession

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/server"
	"github.com/oklog/ulid/v2"
)

const (
	// QueryParam is the URL parameter used by /configs/* to resolve boot data.
	QueryParam = "ref"

	// TemplateParam is exposed to boot templates when they need the raw ref.
	TemplateParam = "boot_ref"
)

// Store owns boot/config references at the application boundary. It keeps
// polling and handlers on persistence interfaces instead of backend row types.
type Store struct {
	commands  persistence.BootSessionCommands
	queries   persistence.BootSessionQueries
	retention time.Duration
	now       func() time.Time
	newRef    func() string
}

// Snapshot is resolved boot data stored under an opaque reference.
type Snapshot struct {
	// Ref is the opaque URL-safe token carried by boot/config URLs.
	Ref string
	// Server is the host identity observed when the boot script was rendered.
	Server server.Server
	// Target is the resolved target name or script associated with the boot.
	Target string
	// Environment is the selected Shoelaces template override environment.
	Environment string
	// Params contains resolved template parameters.
	Params map[string]any
	// Users contains resolved structured account data.
	Users map[string]mappings.ResolvedUser
	// Provisioning contains resolved structured installer data.
	Provisioning mappings.ProvisioningConfig
	// CreatedAt records when the reference was created.
	CreatedAt time.Time
	// ExpiresAt records when the reference should stop resolving.
	ExpiresAt time.Time
}

// NewStore returns a boot-session store backed by persistence.
func NewStore(commands persistence.BootSessionCommands, queries persistence.BootSessionQueries, retention time.Duration) *Store {
	return &Store{
		commands:  commands,
		queries:   queries,
		retention: retention,
		now:       func() time.Time { return time.Now().UTC() },
		newRef:    func() string { return ulid.Make().String() },
	}
}

// Create stores a resolved boot snapshot and returns its opaque reference.
func (s *Store) Create(ctx context.Context, snapshot Snapshot) (string, error) {
	now := s.now()
	if snapshot.Ref == "" {
		snapshot.Ref = s.newRef()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = now
	}
	if snapshot.ExpiresAt.IsZero() {
		snapshot.ExpiresAt = snapshot.CreatedAt.Add(s.retention)
	}

	paramsJSON, err := marshalJSON(snapshot.Params, "{}")
	if err != nil {
		return "", fmt.Errorf("marshal boot session params: %w", err)
	}
	usersJSON, err := marshalJSON(snapshot.Users, "{}")
	if err != nil {
		return "", fmt.Errorf("marshal boot session users: %w", err)
	}
	provisioningJSON, err := marshalJSON(snapshot.Provisioning, "{}")
	if err != nil {
		return "", fmt.Errorf("marshal boot session provisioning: %w", err)
	}

	if err := s.commands.CreateBootSession(ctx, persistence.BootSessionRecord{
		Ref:              snapshot.Ref,
		MAC:              snapshot.Server.Mac,
		IP:               snapshot.Server.IP,
		Hostname:         snapshot.Server.Hostname,
		Target:           snapshot.Target,
		Environment:      snapshot.Environment,
		ParamsJSON:       paramsJSON,
		UsersJSON:        usersJSON,
		ProvisioningJSON: provisioningJSON,
		CreatedAt:        snapshot.CreatedAt,
		ExpiresAt:        snapshot.ExpiresAt,
	}); err != nil {
		return "", err
	}
	return snapshot.Ref, nil
}

// Get resolves an unexpired boot snapshot by reference.
func (s *Store) Get(ctx context.Context, ref string) (Snapshot, error) {
	record, err := s.queries.GetBootSession(ctx, ref, s.now())
	if err != nil {
		return Snapshot{}, err
	}
	return snapshotFromRecord(record)
}

// DeleteExpired removes references that expired before cutoff.
func (s *Store) DeleteExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.commands.DeleteBootSessionsBefore(ctx, cutoff)
}

// ApplyReferenceParams sets the template params that make installer URLs carry
// a boot-session ref instead of serialized provisioning data.
func ApplyReferenceParams(params map[string]any, ref string) {
	if params == nil || ref == "" {
		return
	}
	values := url.Values{}
	values.Set(QueryParam, ref)
	encoded := values.Encode()
	params[TemplateParam] = ref
	params["installer_config_query"] = encoded
	params["installer_config_query_suffix"] = "&" + encoded
	params["installer_config_query_question"] = "?" + encoded
}

func snapshotFromRecord(record persistence.BootSessionRecord) (Snapshot, error) {
	snapshot := Snapshot{
		Ref: record.Ref,
		Server: server.Server{
			Mac:      record.MAC,
			IP:       record.IP,
			Hostname: record.Hostname,
		},
		Target:      record.Target,
		Environment: record.Environment,
		CreatedAt:   record.CreatedAt,
		ExpiresAt:   record.ExpiresAt,
	}
	if err := unmarshalJSON(record.ParamsJSON, &snapshot.Params); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal boot session params: %w", err)
	}
	if err := unmarshalJSON(record.UsersJSON, &snapshot.Users); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal boot session users: %w", err)
	}
	if err := unmarshalJSON(record.ProvisioningJSON, &snapshot.Provisioning); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal boot session provisioning: %w", err)
	}
	return snapshot, nil
}

func marshalJSON(value any, empty string) ([]byte, error) {
	if value == nil {
		return []byte(empty), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func unmarshalJSON(encoded []byte, value any) error {
	if len(encoded) == 0 {
		return nil
	}
	return json.Unmarshal(encoded, value)
}
