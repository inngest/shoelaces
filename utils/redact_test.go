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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSensitiveParamKey(t *testing.T) {
	tests := []struct {
		key       string
		sensitive bool
	}{
		{key: "root_password_crypted", sensitive: true},
		{key: "luks-passphrase", sensitive: true},
		{key: "apiToken", sensitive: true},
		{key: "ssh_private_key", sensitive: true},
		{key: "authorized_keys", sensitive: true},
		{key: "install_secret", sensitive: true},
		{key: "credentials", sensitive: true},
		{key: "users", sensitive: true},
		{key: "hostname", sensitive: false},
		{key: "public_key_url", sensitive: false},
		{key: "release", sensitive: false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.sensitive, IsSensitiveParamKey(tt.key))
		})
	}
}

func TestRedactParamsCopiesAndMasksSensitiveValues(t *testing.T) {
	params := map[string]interface{}{
		"hostname":              "iad-1",
		"root_password_crypted": "hash",
		"bootstrap_token":       "token-value",
	}

	redacted := RedactParams(params)

	assert.Equal(t, "iad-1", redacted["hostname"])
	assert.Equal(t, redactedValue, redacted["root_password_crypted"])
	assert.Equal(t, redactedValue, redacted["bootstrap_token"])
	redacted["hostname"] = "changed"
	assert.Equal(t, "iad-1", params["hostname"])
	assert.Equal(t, "hash", params["root_password_crypted"])
}
