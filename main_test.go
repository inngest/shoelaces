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

package main

import (
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	oldVersion, oldCommit, oldDate, oldBuiltBy := version, commit, date, builtBy
	t.Cleanup(func() {
		version, commit, date, builtBy = oldVersion, oldCommit, oldDate, oldBuiltBy
	})

	version = "v2026-06-11.01"
	commit = "abc123"
	date = "2026-06-11T00:00:00Z"
	builtBy = "goreleaser"

	got := versionString()
	for _, want := range []string{
		"shoelaces v2026-06-11.01",
		"commit: abc123",
		"date: 2026-06-11T00:00:00Z",
		"built by: goreleaser",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("version output missing %q:\n%s", want, got)
		}
	}
}
