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
	"database/sql"
	"testing"
	"time"

	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence/memory"
	"github.com/inngest/shoelaces/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreCreatesAndResolvesBootSession(t *testing.T) {
	backend := memory.New()
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	store := NewStore(backend, backend, 2*time.Hour)
	store.now = func() time.Time { return now }
	store.newRef = func() string { return "01JREFTESTREFTESTREFTEST00" }

	ref, err := store.Create(context.Background(), Snapshot{
		Server: server.New("06:66:de:ad:be:ef", "192.0.2.10", "install-host"),
		Target: "debian.ipxe",
		Params: map[string]any{
			"hostname": "install-host",
			"release":  "trixie",
		},
		Users: map[string]mappings.ResolvedUser{
			"infra": {Name: "infra", Primary: true, Groups: []string{"sudo"}},
		},
		Provisioning: mappings.ProvisioningConfig{
			Repos: mappings.ReposConfig{Release: "trixie"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "01JREFTESTREFTESTREFTEST00", ref)

	got, err := store.Get(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, ref, got.Ref)
	assert.Equal(t, "install-host", got.Server.Hostname)
	assert.Equal(t, "debian.ipxe", got.Target)
	assert.Equal(t, "trixie", got.Params["release"])
	assert.Equal(t, "infra", got.Users["infra"].Name)
	assert.Equal(t, []string{"sudo"}, got.Users["infra"].Groups)
	assert.Equal(t, "trixie", got.Provisioning.Repos.Release)
	assert.Equal(t, now, got.CreatedAt)
	assert.Equal(t, now.Add(2*time.Hour), got.ExpiresAt)
}

func TestStoreHonorsBootSessionExpiry(t *testing.T) {
	backend := memory.New()
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	store := NewStore(backend, backend, time.Hour)
	store.now = func() time.Time { return now }
	store.newRef = func() string { return "expired-ref" }

	ref, err := store.Create(context.Background(), Snapshot{
		Server: server.New("06:66:de:ad:be:ef", "192.0.2.10", "install-host"),
	})
	require.NoError(t, err)

	store.now = func() time.Time { return now.Add(2 * time.Hour) }
	_, err = store.Get(context.Background(), ref)
	assert.ErrorIs(t, err, sql.ErrNoRows)

	deleted, err := store.DeleteExpired(context.Background(), now.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
}

func TestApplyReferenceParamsSetsBootReferenceQueryParams(t *testing.T) {
	params := map[string]any{
		"boot_ref_query":          "",
		"boot_ref_query_suffix":   "",
		"boot_ref_query_question": "",
	}

	ApplyReferenceParams(params, "01JREFTESTREFTESTREFTEST00")

	assert.Equal(t, "01JREFTESTREFTESTREFTEST00", params[TemplateParam])
	assert.Equal(t, "ref=01JREFTESTREFTESTREFTEST00", params["boot_ref_query"])
	assert.Equal(t, "&ref=01JREFTESTREFTESTREFTEST00", params["boot_ref_query_suffix"])
	assert.Equal(t, "?ref=01JREFTESTREFTESTREFTEST00", params["boot_ref_query_question"])
}
