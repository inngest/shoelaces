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

package shoelaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePersistenceDocsReferenceExistingFiles(t *testing.T) {
	for _, path := range []string{
		"docs/persistence.md",
		"persistence/interfaces.go",
		"persistence/sqlite/migrations/001_initial.sql",
		"persistence/sqlite/query/events.sql",
		"persistence/sqlite/db/events.sql.go",
		"dev/shoelaces.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			info, err := os.Stat(filepath.Clean(path))
			if err != nil {
				t.Fatalf("expected documented file %s to exist: %v", path, err)
			}
			if info.IsDir() {
				t.Fatalf("expected documented path %s to be a file", path)
			}
		})
	}
}

func TestDevelopmentProfileUsesDocumentedSQLitePersistence(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("dev", "shoelaces.yaml"))
	if err != nil {
		t.Fatalf("read dev profile: %v", err)
	}
	config := string(content)
	for _, expected := range []string{
		"persistence:",
		"backend: sqlite",
		"path: runtime/shoelaces.db",
		"eventsSweepInterval: 24h",
		"bootSessionsSweepInterval: 1h",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("dev profile missing %q\n%s", expected, config)
		}
	}
}
