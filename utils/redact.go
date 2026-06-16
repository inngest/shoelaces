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

import "strings"

const redactedValue = "[REDACTED]"

var sensitiveKeyFragments = []string{
	"apikey",
	"authorizedkey",
	"credential",
	"passphrase",
	"password",
	"privatekey",
	"secret",
	"token",
}

// IsSensitiveParamKey reports whether a template parameter key should be
// hidden before it is written to logs, events, or UI-facing event JSON.
func IsSensitiveParamKey(key string) bool {
	normalized := normalizeParamKey(key)
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

// RedactParams returns a copy of params with sensitive values replaced.
func RedactParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}

	redacted := make(map[string]interface{}, len(params))
	for key, value := range params {
		if IsSensitiveParamKey(key) {
			redacted[key] = redactedValue
			continue
		}
		redacted[key] = value
	}
	return redacted
}

// RedactedMapToString formats params for logs after masking sensitive values.
func RedactedMapToString(params map[string]interface{}) string {
	return MapToString(RedactParams(params))
}

func normalizeParamKey(key string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(key) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}
