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

package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thousandeyes/shoelaces/server"
)

const expectedEvent = `{"eventType":0,"date":"1970-01-01T00:00:00Z","server":{"Mac":"","IP":"","Hostname":"test_host"},"bootType":"Manual","script":"freebsd.ipxe","message":"","params":{"baseURL":"localhost:8080","cloudconfig":"virtual","hostname":"","version":"12345"}}`

func TestNew(t *testing.T) {
	event := New(HostPoll, server.Server{Mac: "", IP: "", Hostname: "test_host"}, PtrMatchBoot, "msdos.ipxe", map[string]interface{}{"test": "testParam"})
	assert.Equal(t, HostPoll, event.Type)
	assert.Equal(t, "test_host", event.Server.Hostname)
	assert.Equal(t, PtrMatchBoot, event.BootType)
	assert.Equal(t, "msdos.ipxe", event.Script)
	require.Len(t, event.Params, 1)
	assert.Equal(t, "testParam", event.Params["test"])

	now := time.Now()
	assert.False(t, event.Date.After(now))
}

func TestNewSetsMessage(t *testing.T) {
	tests := []struct {
		name       string
		eventType  Type
		bootType   string
		script     string
		params     map[string]interface{}
		wantSubstr string
	}{
		{
			name:       "host poll",
			eventType:  HostPoll,
			wantSubstr: "Host test_host polled for a script.",
		},
		{
			name:       "user selection",
			eventType:  UserSelection,
			script:     "freebsd.ipxe",
			wantSubstr: "A user selected freebsd.ipxe for the host test_host.",
		},
		{
			name:       "host boot",
			eventType:  HostBoot,
			bootType:   ManualBoot,
			params:     map[string]interface{}{"hostname": "test_host"},
			wantSubstr: "Host test_host booted using Manual method",
		},
		{
			name:       "host timeout",
			eventType:  HostTimeout,
			wantSubstr: "Host test_host timed out.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := New(tt.eventType, server.Server{Hostname: "test_host"}, tt.bootType, tt.script, tt.params)

			assert.Contains(t, event.Message, tt.wantSubstr)
		})
	}
}

func TestLogAddEventInitializesAndAppendsEvents(t *testing.T) {
	log := &Log{}
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "test_host")

	log.AddEvent(HostPoll, srv, "", "", nil)
	log.AddEvent(UserSelection, srv, "", "freebsd.ipxe", nil)

	require.Len(t, log.Events[srv.Mac], 2)
	assert.Equal(t, HostPoll, log.Events[srv.Mac][0].Type)
	assert.Equal(t, UserSelection, log.Events[srv.Mac][1].Type)
	assert.Equal(t, "freebsd.ipxe", log.Events[srv.Mac][1].Script)
}

func TestEventMarshalJSON(t *testing.T) {
	event := Event{
		Type:     HostPoll,
		Date:     time.Unix(0, 0).UTC(),
		Server:   server.Server{Mac: "", IP: "", Hostname: "test_host"},
		BootType: ManualBoot,
		Script:   "freebsd.ipxe",
		Message:  "",
		Params: map[string]interface{}{
			"baseURL":     "localhost:8080",
			"cloudconfig": "virtual",
			"hostname":    "",
			"version":     "12345",
		},
	}
	marshaled, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Equal(t, expectedEvent, string(marshaled))
}
