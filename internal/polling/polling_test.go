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

package polling

import (
	"strings"
	"testing"

	"github.com/thousandeyes/shoelaces/internal/event"
	"github.com/thousandeyes/shoelaces/internal/log"
	"github.com/thousandeyes/shoelaces/internal/server"
	"github.com/thousandeyes/shoelaces/internal/templates"
)

func TestGenStartScriptUsesBaseURL(t *testing.T) {
	script := GenStartScript(log.MakeLogger(testLogWriter{}), "127.0.0.1:8081")

	if !strings.Contains(script, "http://127.0.0.1:8081/poll/1/${netX/mac:hexhyp}") {
		t.Fatalf("start script does not chain to the configured base URL:\n%s", script)
	}
	if !strings.HasPrefix(script, "#!ipxe\n") {
		t.Fatalf("start script should be an iPXE script, got:\n%s", script)
	}
}

func TestPollUnknownServerRetriesThenTimesOut(t *testing.T) {
	states := &server.States{Servers: make(map[string]*server.State)}
	events := &event.Log{}
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "")

	script, err := Poll(
		log.MakeLogger(testLogWriter{}),
		states,
		nil,
		nil,
		events,
		templates.New(),
		"127.0.0.1:8081",
		srv,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "chain -ar http://127.0.0.1:8081/poll/1/06-66-de-ad-be-ef") {
		t.Fatalf("first poll should return retry script, got:\n%s", script)
	}
	if states.Servers[srv.Mac] == nil {
		t.Fatal("unknown server was not added to retry state")
	}
	if got := len(events.Events[srv.Mac]); got != 1 {
		t.Fatalf("expected one poll event, got %d", got)
	}

	for i := 0; i <= maxRetry; i++ {
		script, err = Poll(
			log.MakeLogger(testLogWriter{}),
			states,
			nil,
			nil,
			events,
			templates.New(),
			"127.0.0.1:8081",
			srv,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	if script != timeoutScript {
		t.Fatalf("expected timeout script after max retries, got:\n%s", script)
	}
	if states.Servers[srv.Mac] != nil {
		t.Fatal("timed-out server should be removed from retry state")
	}
}

func TestListServersReturnsOnlyWaitingServersSortedByMAC(t *testing.T) {
	states := &server.States{Servers: map[string]*server.State{
		"ff:ff:ff:ff:ff:ff": {
			Server: server.New("ff:ff:ff:ff:ff:ff", "192.0.2.3", "last"),
			Target: server.InitTarget,
		},
		"00:00:00:00:00:01": {
			Server: server.New("00:00:00:00:00:01", "192.0.2.1", "first"),
			Target: server.InitTarget,
		},
		"00:00:00:00:00:02": {
			Server: server.New("00:00:00:00:00:02", "192.0.2.2", "booting"),
			Target: "debian.ipxe",
		},
	}}

	servers := ListServers(states)
	if len(servers) != 2 {
		t.Fatalf("expected 2 waiting servers, got %d", len(servers))
	}
	if servers[0].Mac != "00:00:00:00:00:01" || servers[1].Mac != "ff:ff:ff:ff:ff:ff" {
		t.Fatalf("servers were not filtered and sorted by MAC: %#v", servers)
	}
}

type testLogWriter struct{}

func (testLogWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
