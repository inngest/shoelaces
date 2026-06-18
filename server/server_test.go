// Copyright 2018 ThousandEyes Inc.
// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	srv := New("06:66:de:ad:be:ef", "192.0.2.10", "test-host")

	assert.Equal(t, "06:66:de:ad:be:ef", srv.Mac)
	assert.Equal(t, "192.0.2.10", srv.IP)
	assert.Equal(t, "test-host", srv.Hostname)
}

func TestStatesAddAndDeleteServer(t *testing.T) {
	states := &States{Servers: make(map[string]*State)}
	srv := New("06:66:de:ad:be:ef", "192.0.2.10", "test-host")

	states.AddServer(srv)

	require.NotNil(t, states.Servers[srv.Mac])
	assert.Equal(t, srv, states.Servers[srv.Mac].Server)
	assert.Equal(t, InitTarget, states.Servers[srv.Mac].Target)
	assert.Equal(t, 1, states.Servers[srv.Mac].Retry)
	assert.NotZero(t, states.Servers[srv.Mac].LastAccess)

	states.DeleteServer(srv.Mac)

	assert.Nil(t, states.Servers[srv.Mac])
}

func TestServersSortByMAC(t *testing.T) {
	servers := Servers{
		New("ff:ff:ff:ff:ff:ff", "192.0.2.3", "last"),
		New("00:00:00:00:00:02", "192.0.2.2", "middle"),
		New("00:00:00:00:00:01", "192.0.2.1", "first"),
	}

	sort.Sort(servers)

	assert.Equal(t, "00:00:00:00:00:01", servers[0].Mac)
	assert.Equal(t, "00:00:00:00:00:02", servers[1].Mac)
	assert.Equal(t, "ff:ff:ff:ff:ff:ff", servers[2].Mac)
}

func TestCleanExpiredStatesRemovesInactiveServers(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	expireBefore := int(now.Add(-3 * time.Minute).Unix())
	states := &States{Servers: map[string]*State{
		"00:00:00:00:00:01": {
			Server:     New("00:00:00:00:00:01", "192.0.2.1", "expired"),
			Target:     InitTarget,
			Retry:      2,
			LastAccess: expireBefore - 1,
		},
		"00:00:00:00:00:02": {
			Server:     New("00:00:00:00:00:02", "192.0.2.2", "boundary"),
			Target:     InitTarget,
			Retry:      1,
			LastAccess: expireBefore,
		},
		"00:00:00:00:00:03": {
			Server:     New("00:00:00:00:00:03", "192.0.2.3", "active"),
			Target:     InitTarget,
			Retry:      1,
			LastAccess: expireBefore + 1,
		},
	}}

	removed := cleanExpiredStates(log.MakeLogger(io.Discard), states, expireBefore)

	assert.Equal(t, 2, removed)
	assert.Nil(t, states.Servers["00:00:00:00:00:01"])
	assert.Nil(t, states.Servers["00:00:00:00:00:02"])
	require.NotNil(t, states.Servers["00:00:00:00:00:03"])
	assert.Equal(t, "active", states.Servers["00:00:00:00:00:03"].Hostname)
}

func TestPersistentStateStorePersistsWaitingAndSelectedState(t *testing.T) {
	backend := memory.New()
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	store := NewPersistentStateStore(backend, backend)
	store.now = func() time.Time { return now }
	ctx := context.Background()

	waitingServer := New("00:00:00:00:00:02", "192.0.2.2", "waiting")
	require.NoError(t, store.QueueServer(ctx, waitingServer, []TargetOption{
		{Name: "debian12", Script: "debian.ipxe", Label: "Debian 12"},
	}))

	waiting, err := store.ListWaiting(ctx)
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, "00:00:00:00:00:02", waiting[0].Mac)
	assert.Equal(t, []TargetOption{{Name: "debian12", Script: "debian.ipxe", Label: "Debian 12"}}, waiting[0].AllowedTargets)

	state, err := store.GetState(ctx, waitingServer.Mac)
	require.NoError(t, err)
	assert.Equal(t, InitTarget, state.Target)
	assert.Equal(t, 1, state.Retry)
	assert.Equal(t, int(now.Unix()), state.LastAccess)

	state.Target = "debian.ipxe"
	state.Environment = "prod"
	state.Params = map[string]interface{}{"baseURL": "shoelaces.test", "release": "trixie"}
	state.Users = map[string]mappings.ResolvedUser{
		"infra": {Name: "infra", Primary: true, Groups: []string{"sudo"}},
	}
	state.Provisioning = mappings.ProvisioningConfig{
		Repos: mappings.ReposConfig{Release: "trixie"},
	}
	state.Retry = 3
	state.LastAccess = int(now.Add(time.Minute).Unix())
	require.NoError(t, store.SaveState(ctx, state))

	selected, err := store.GetState(ctx, waitingServer.Mac)
	require.NoError(t, err)
	assert.Equal(t, "debian.ipxe", selected.Target)
	assert.Equal(t, "prod", selected.Environment)
	assert.Equal(t, "trixie", selected.Params["release"])
	assert.Equal(t, "infra", selected.Users["infra"].Name)
	assert.Equal(t, []string{"sudo"}, selected.Users["infra"].Groups)
	assert.Equal(t, "trixie", selected.Provisioning.Repos.Release)
	assert.Equal(t, 3, selected.Retry)

	waiting, err = store.ListWaiting(ctx)
	require.NoError(t, err)
	assert.Empty(t, waiting)

	require.NoError(t, store.DeleteState(ctx, waitingServer.Mac))
	_, err = store.GetState(ctx, waitingServer.Mac)
	assert.ErrorIs(t, err, ErrStateNotFound)
}

func TestCleanExpiredStatesUsesPersistentStateStore(t *testing.T) {
	backend := memory.New()
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	store := NewPersistentStateStore(backend, backend)
	ctx := context.Background()

	require.NoError(t, store.SaveState(ctx, &State{
		Server:     New("00:00:00:00:00:01", "192.0.2.1", "expired"),
		Target:     InitTarget,
		Retry:      1,
		LastAccess: int(now.Add(-StateExpireAfter - time.Second).Unix()),
	}))
	require.NoError(t, store.SaveState(ctx, &State{
		Server:     New("00:00:00:00:00:02", "192.0.2.2", "active"),
		Target:     InitTarget,
		Retry:      1,
		LastAccess: int(now.Unix()),
	}))

	removed, err := CleanExpiredStates(log.MakeLogger(io.Discard), store, now.Add(-StateExpireAfter))
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)

	_, err = store.GetState(ctx, "00:00:00:00:00:01")
	assert.ErrorIs(t, err, ErrStateNotFound)
	active, err := store.GetState(ctx, "00:00:00:00:00:02")
	require.NoError(t, err)
	assert.Equal(t, "active", active.Hostname)
}
