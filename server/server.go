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
	"sync"
	"time"

	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
)

const (
	// InitTarget is an initial dummy target assigned to the servers
	InitTarget = "NOTARGET"

	// StateExpireAfter is how long a host can wait between polling requests
	// before Shoelaces drops its manual-selection state.
	StateExpireAfter = 3 * time.Minute

	stateCleanerComponent = "state_cleaner"
)

// TargetOption is a boot target that can be selected for a waiting server.
type TargetOption struct {
	// Name is the target key from mappings.yaml and the value posted by the UI.
	Name string
	// Script is the iPXE template name rendered when this target is selected.
	Script string
	// Label is optional display text for operators.
	Label string
	// Environment selects an optional template override environment.
	Environment string
}

// Server holds data that uniquely identifies a server.
type Server struct {
	// Mac is the server MAC address in colon-separated form.
	Mac string
	// IP is the IP address observed by Shoelaces during polling.
	IP string
	// Hostname is the resolved or request-provided host name.
	Hostname string
	// AllowedTargets is the resolver-approved manual target list for this host.
	AllowedTargets []TargetOption `json:",omitempty"`
}

// Servers is an array of Server
type Servers []Server

// Len implementation for the sort Interface
func (s Servers) Len() int {
	return len(s)
}

// Swap implementation for the sort interface
func (s Servers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less implementation for the Sort interface
func (s Servers) Less(i, j int) bool {
	return s[i].Mac < s[j].Mac
}

// State holds information regarding a host that is attempting to boot.
type State struct {
	Server
	Target       string
	Environment  string
	Params       map[string]interface{}
	Users        map[string]mappings.ResolvedUser
	Provisioning mappings.ProvisioningConfig
	Retry        int
	LastAccess   int
}

// States holds a map between MAC addresses and
// States. It provides a mutex for thread-safety.
type States struct {
	sync.RWMutex
	Servers map[string]*State
}

// StateStore is the boot-state boundary used by polling and the UI. It lets
// Shoelaces use either the legacy in-memory state map or a persistent backend
// without coupling polling code to SQL or persistence row types.
type StateStore interface {
	// QueueServer records a host that is waiting for manual target selection.
	QueueServer(ctx context.Context, server Server, allowedTargets []TargetOption) error
	// GetState returns the current waiting/manual state for one host MAC.
	GetState(ctx context.Context, mac string) (*State, error)
	// SaveState persists a complete state snapshot for one host MAC.
	SaveState(ctx context.Context, state *State) error
	// DeleteState removes the state for one host MAC.
	DeleteState(ctx context.Context, mac string) error
	// ListWaiting returns hosts whose target has not been selected yet.
	ListWaiting(ctx context.Context) (Servers, error)
	// DeleteStatesBefore removes states with LastAccess older than cutoff.
	DeleteStatesBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// New returns a Server with is values initialized
func New(mac string, ip string, hostname string) Server {
	return Server{
		Mac:      mac,
		IP:       ip,
		Hostname: hostname,
	}
}

// AddServer adds a server to the States struct.
func (m *States) AddServer(server Server) {
	m.AddServerWithTargets(server, nil)
}

// AddServerWithTargets adds a waiting server with resolver-approved choices.
func (m *States) AddServerWithTargets(server Server, allowedTargets []TargetOption) {
	server.AllowedTargets = copyTargetOptions(allowedTargets)
	m.Servers[server.Mac] = &State{
		Server:     server,
		Target:     InitTarget,
		Retry:      1,
		LastAccess: int(time.Now().UTC().Unix()),
	}
}

// QueueServer records a waiting host in the legacy in-memory state map.
func (m *States) QueueServer(_ context.Context, server Server, allowedTargets []TargetOption) error {
	m.Lock()
	defer m.Unlock()

	m.AddServerWithTargets(server, allowedTargets)
	return nil
}

// GetState returns one host state from the legacy in-memory state map.
func (m *States) GetState(_ context.Context, mac string) (*State, error) {
	m.RLock()
	defer m.RUnlock()

	state := m.Servers[mac]
	if state == nil {
		return nil, ErrStateNotFound
	}
	return copyState(state), nil
}

// SaveState replaces one host state in the legacy in-memory state map.
func (m *States) SaveState(_ context.Context, state *State) error {
	m.Lock()
	defer m.Unlock()

	m.Servers[state.Mac] = copyState(state)
	return nil
}

// DeleteState removes one host state from the legacy in-memory state map.
func (m *States) DeleteState(_ context.Context, mac string) error {
	m.Lock()
	defer m.Unlock()

	m.DeleteServer(mac)
	return nil
}

// ListWaiting returns waiting hosts from the legacy in-memory state map.
func (m *States) ListWaiting(_ context.Context) (Servers, error) {
	ret := make([]Server, 0)

	m.RLock()
	defer m.RUnlock()
	for _, state := range m.Servers {
		if state.Target == InitTarget {
			ret = append(ret, state.Server)
		}
	}
	sortServers(ret)
	return ret, nil
}

// DeleteStatesBefore removes inactive host states from the legacy in-memory map.
func (m *States) DeleteStatesBefore(_ context.Context, cutoff time.Time) (int64, error) {
	m.Lock()
	defer m.Unlock()

	expireBefore := int(cutoff.UTC().Unix())
	var removed int64
	for mac, state := range m.Servers {
		if state.LastAccess <= expireBefore {
			delete(m.Servers, mac)
			removed++
		}
	}
	return removed, nil
}

func copyTargetOptions(options []TargetOption) []TargetOption {
	if options == nil {
		return nil
	}
	copied := make([]TargetOption, len(options))
	copy(copied, options)
	return copied
}

// DeleteServer deletes a server from the States struct
func (m *States) DeleteServer(mac string) {
	delete(m.Servers, mac)
}

// StartStateCleaner spawns a goroutine that cleans MAC addresses that
// have been inactive in Shoelaces for more than 3 minutes.
func StartStateCleaner(logger log.Logger, serverStates *States) {
	StartStateStoreCleaner(logger, serverStates)
}

// StartStateStoreCleaner spawns a goroutine that cleans MAC addresses that
// have been inactive in Shoelaces for more than StateExpireAfter.
func StartStateStoreCleaner(logger log.Logger, stateStore StateStore) {
	const cleanInterval = time.Minute
	if logger == nil {
		logger = log.MakeLogger(io.Discard)
	}
	logger = logger.With("component", stateCleanerComponent)

	logger.Debug("Starting server state cleaner", "expire_after", StateExpireAfter, "interval", cleanInterval)
	go func() {
		for {
			time.Sleep(cleanInterval)
			if _, err := CleanExpiredStates(logger, stateStore, time.Now().UTC().Add(-StateExpireAfter)); err != nil {
				logger.Error("Failed to clean expired server states", "err", err)
			}
		}
	}()
}

func cleanExpiredStates(logger log.Logger, serverStates *States, expireBefore int) int {
	removed, err := CleanExpiredStates(logger, serverStates, time.Unix(int64(expireBefore), 0).UTC())
	if err != nil {
		logger.Error("Failed to clean expired server states", "err", err)
	}
	return int(removed)
}

// CleanExpiredStates removes stale states from any server state store.
func CleanExpiredStates(logger log.Logger, stateStore StateStore, cutoff time.Time) (int64, error) {
	states, err := stateStore.ListWaiting(context.Background())
	if err != nil {
		return 0, err
	}

	logger.Debug("Sweeping server states", "before", cutoff, "states", len(states))
	removed, err := stateStore.DeleteStatesBefore(context.Background(), cutoff)
	if err != nil {
		return 0, err
	}
	logger.Debug("Completed server state sweep", "removed", removed)
	return removed, nil
}
