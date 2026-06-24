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

package validation

import "strings"

// ContainsShellUnsafeValue reports whether a TPM-related value contains
// characters that should not be interpolated into installer shell commands.
func ContainsShellUnsafeValue(value string) bool {
	return strings.ContainsAny(value, " \t\n\r'\"\\;$&|<>`")
}

// IsTPMPCRSelection accepts PCR numbers separated by "+" or ",". It rejects
// empty selections, leading/trailing separators, and repeated separators.
func IsTPMPCRSelection(value string) bool {
	hasToken := false
	for _, r := range value {
		if r >= '0' && r <= '9' {
			hasToken = true
			continue
		}
		if r == '+' || r == ',' {
			if !hasToken {
				return false
			}
			hasToken = false
			continue
		}
		return false
	}
	return hasToken
}
