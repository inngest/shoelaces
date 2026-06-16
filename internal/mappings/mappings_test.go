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

package mappings

import (
	"net"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mockScriptParams1 = map[string]interface{}{
		"param11": "param1_value1",
		"param21": "param2_value1",
	}
	mockScriptParams2 = map[string]interface{}{
		"param12": "param1_value2",
		"param22": "param2_value2",
	}
	mockScript1 = Script{Name: "mock_script1", Params: mockScriptParams1}
	mockScript2 = Script{Name: "mock_script2", Params: mockScriptParams2}

	mockRegex1, _ = regexp.Compile("mock_host1")
	mockRegex2, _ = regexp.Compile("mock_host2")

	mockHostNameMap1 = HostnameMap{
		Hostname: mockRegex1,
		Script:   &mockScript1,
	}

	mockHostNameMap2 = HostnameMap{
		Hostname: mockRegex2,
		Script:   &mockScript2,
	}

	_, mockNetwork1, _ = net.ParseCIDR("10.0.0.0/8")
	_, mockNetwork2, _ = net.ParseCIDR("192.168.0.0/16")

	mockNetworkMap1 = NetworkMap{
		Network: mockNetwork1,
		Script:  &mockScript1,
	}
	mockNetworkMap2 = NetworkMap{
		Network: mockNetwork2,
		Script:  &mockScript2,
	}
)

func TestScript(t *testing.T) {
	expected1 := "mock_script1 : { param11: param1_value1, param21: param2_value1 }"
	expected2 := "mock_script1 : { param21: param2_value1, param11: param1_value1 }"
	mockScriptString := mockScript1.String()
	assert.Contains(t, []string{expected1, expected2}, mockScriptString)
}

func TestFindScriptForHostname(t *testing.T) {
	maps := []HostnameMap{mockHostNameMap1, mockHostNameMap2}
	tests := []struct {
		name       string
		maps       []HostnameMap
		hostname   string
		wantName   string
		wantFound  bool
		wantScript bool
	}{
		{
			name:       "first hostname map matches",
			maps:       maps,
			hostname:   "mock_host1",
			wantName:   "mock_script1",
			wantFound:  true,
			wantScript: true,
		},
		{
			name:       "second hostname map matches",
			maps:       maps,
			hostname:   "mock_host2",
			wantName:   "mock_script2",
			wantFound:  true,
			wantScript: true,
		},
		{
			name:      "hostname does not match",
			maps:      maps,
			hostname:  "mock_host_bad",
			wantFound: false,
		},
		{
			name:      "empty hostname maps do not match",
			hostname:  "mock_host1",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, success := FindScriptForHostname(tt.maps, tt.hostname)

			assert.Equal(t, tt.wantFound, success)
			if !tt.wantScript {
				assert.Nil(t, script)
				return
			}
			require.NotNil(t, script)
			assert.Equal(t, tt.wantName, script.Name)
		})
	}
}

func TestScriptForNetwork(t *testing.T) {
	maps := []NetworkMap{mockNetworkMap1, mockNetworkMap2}
	tests := []struct {
		name       string
		maps       []NetworkMap
		ip         string
		wantName   string
		wantFound  bool
		wantScript bool
	}{
		{
			name:       "first network map matches",
			maps:       maps,
			ip:         "10.0.0.1",
			wantName:   "mock_script1",
			wantFound:  true,
			wantScript: true,
		},
		{
			name:       "second network map matches",
			maps:       maps,
			ip:         "192.168.0.1",
			wantName:   "mock_script2",
			wantFound:  true,
			wantScript: true,
		},
		{
			name:      "ip does not match",
			maps:      maps,
			ip:        "8.8.8.8",
			wantFound: false,
		},
		{
			name:      "invalid ip does not match",
			maps:      maps,
			ip:        "not-an-ip",
			wantFound: false,
		},
		{
			name:      "empty network maps do not match",
			ip:        "10.0.0.1",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, success := FindScriptForNetwork(tt.maps, tt.ip)

			assert.Equal(t, tt.wantFound, success)
			if !tt.wantScript {
				assert.Nil(t, script)
				return
			}
			require.NotNil(t, script)
			assert.Equal(t, tt.wantName, script.Name)
		})
	}
}
