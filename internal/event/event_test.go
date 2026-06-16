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
	"github.com/thousandeyes/shoelaces/internal/server"
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
