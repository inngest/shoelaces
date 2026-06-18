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
	"encoding/json"
	"reflect"
	"strings"
)

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
	if key == "users" || key == "provisioning" || strings.HasPrefix(key, "installer_config_query") {
		return true
	}
	normalized := normalizeParamKey(key)
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

// RedactParams returns a copy of params with sensitive values replaced. It is
// used for event/audit data, so top-level sensitive containers are intentionally
// collapsed instead of preserving their nested shape.
func RedactParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	redacted := make(map[string]any, len(params))
	for key, value := range params {
		if IsSensitiveParamKey(key) {
			redacted[key] = redactedValue
			continue
		}
		redacted[key] = RedactForLog(value)
	}
	return redacted
}

// RedactForLog returns a log-safe copy of value with sensitive fields replaced.
// Unlike RedactParams, it preserves structured containers such as users and
// provisioning so logs can include useful context without exposing secrets.
func RedactForLog(value any) any {
	return redactForLog("", value)
}

// RedactJSONForLog decodes a JSON blob and returns a redacted value for logs.
// Invalid JSON is not returned because raw blobs may contain secrets.
func RedactJSONForLog(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return redactedValue
	}
	return RedactForLog(decoded)
}

// RedactedMapToString formats params for logs after masking sensitive values.
func RedactedMapToString(params map[string]any) string {
	redacted, _ := RedactForLog(params).(map[string]any)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return redactedValue
	}
	return string(encoded)
}

func redactForLog(key string, value any) any {
	if shouldRedactLogValue(key, value) {
		return redactedValue
	}

	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			redacted[childKey] = redactForLog(childKey, childValue)
		}
		return redacted
	case map[string]string:
		redacted := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			redacted[childKey] = redactForLog(childKey, childValue)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, childValue := range typed {
			redacted[i] = redactForLog("", childValue)
		}
		return redacted
	case []map[string]any:
		redacted := make([]any, len(typed))
		for i, childValue := range typed {
			redacted[i] = redactForLog("", childValue)
		}
		return redacted
	default:
		if decoded, ok := decodeStructuredValue(value); ok {
			return redactForLog("", decoded)
		}
		return value
	}
}

func shouldRedactLogValue(key string, value any) bool {
	if key == "" {
		return false
	}
	if isStructuredContainer(value) && isStructuredContextKey(key) {
		return false
	}
	return IsSensitiveParamKey(key)
}

func isStructuredContainer(value any) bool {
	switch value.(type) {
	case map[string]any, map[string]string, []any, []map[string]any:
		return true
	default:
		return isJSONStructuredValue(value)
	}
}

func isStructuredContextKey(key string) bool {
	return key == "users" || key == "provisioning"
}

func decodeStructuredValue(value any) (any, bool) {
	if !isJSONStructuredValue(value) {
		return nil, false
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return redactedValue, true
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return redactedValue, true
	}
	return decoded, true
}

func isJSONStructuredValue(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return false
	}
	if reflected.Kind() == reflect.Slice && reflected.Type().Elem().Kind() == reflect.Uint8 {
		return false
	}
	switch reflected.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.Struct:
		return true
	default:
		return false
	}
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
