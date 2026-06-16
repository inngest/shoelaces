// Copyright 2018 ThousandEyes Inc.
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
	"sort"
	"testing"

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
