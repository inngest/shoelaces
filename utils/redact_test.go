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
		{key: "provisioning", sensitive: true},
		{key: "installer_config_query", sensitive: true},
		{key: "installer_config_query_suffix", sensitive: true},
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
	params := map[string]any{
		"hostname":              "iad-1",
		"root_password_crypted": "hash",
		"bootstrap_token":       "token-value",
		"users": map[string]any{
			"root": map[string]any{"passwordCrypted": "root-hash"},
		},
	}

	redacted := RedactParams(params)

	assert.Equal(t, "iad-1", redacted["hostname"])
	assert.Equal(t, redactedValue, redacted["root_password_crypted"])
	assert.Equal(t, redactedValue, redacted["bootstrap_token"])
	assert.Equal(t, redactedValue, redacted["users"])
	redacted["hostname"] = "changed"
	assert.Equal(t, "iad-1", params["hostname"])
	assert.Equal(t, "hash", params["root_password_crypted"])
}

func TestRedactForLogPreservesStructuredContextAndMasksSecrets(t *testing.T) {
	payload := map[string]any{
		"hostname": "iad-1",
		"users": map[string]any{
			"root": map[string]any{
				"locked":          false,
				"passwordCrypted": "root-hash",
			},
			"infra": map[string]any{
				"sshAuthorizedKeys": []any{"ssh-ed25519 public"},
				"bootstrapToken":    "token-value",
			},
		},
		"provisioning": structuredRedactionFixture{
			Repos: repoRedactionFixture{
				Release: "trixie",
				Token:   "repo-token",
			},
		},
	}

	redacted := RedactForLog(payload).(map[string]any)

	assert.Equal(t, "iad-1", redacted["hostname"])
	users := redacted["users"].(map[string]any)
	root := users["root"].(map[string]any)
	assert.Equal(t, false, root["locked"])
	assert.Equal(t, redactedValue, root["passwordCrypted"])
	infra := users["infra"].(map[string]any)
	assert.Equal(t, redactedValue, infra["sshAuthorizedKeys"])
	assert.Equal(t, redactedValue, infra["bootstrapToken"])
	provisioning := redacted["provisioning"].(map[string]any)
	repos := provisioning["repos"].(map[string]any)
	assert.Equal(t, "trixie", repos["release"])
	assert.Equal(t, redactedValue, repos["token"])
}

func TestRedactJSONForLogMasksSecrets(t *testing.T) {
	redacted := RedactJSONForLog([]byte(`{
		"release": "trixie",
		"users": {
			"root": {
				"passwordCrypted": "hash"
			}
		}
	}`)).(map[string]any)

	assert.Equal(t, "trixie", redacted["release"])
	users := redacted["users"].(map[string]any)
	root := users["root"].(map[string]any)
	assert.Equal(t, redactedValue, root["passwordCrypted"])
	assert.Equal(t, redactedValue, RedactJSONForLog([]byte(`{`)))
}

type structuredRedactionFixture struct {
	Repos repoRedactionFixture `json:"repos"`
}

type repoRedactionFixture struct {
	Release string `json:"release"`
	Token   string `json:"token"`
}
