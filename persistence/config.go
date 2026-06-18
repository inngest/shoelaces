// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// BackendSQLite stores runtime data in a local SQLite database.
	BackendSQLite = "sqlite"
	// BackendMemory stores runtime data in process memory only.
	BackendMemory = "memory"

	defaultSQLitePath = "runtime/shoelaces.db"
)

// RetentionConfig controls how long runtime records remain queryable.
type RetentionConfig struct {
	// Events is the retention window for operator-facing event history.
	Events time.Duration
	// BootSessions is the retention window for boot/config references.
	BootSessions time.Duration
}

// Config controls the runtime persistence backend.
type Config struct {
	// Backend selects the persistence implementation.
	Backend string
	// Path is the SQLite database path. Relative paths are resolved under data-dir.
	Path string
	// Retention controls cleanup windows for persisted runtime records.
	Retention RetentionConfig
}

// DefaultConfig returns the default persistence settings for normal operation.
func DefaultConfig() Config {
	return Config{
		Backend: BackendSQLite,
		Path:    defaultSQLitePath,
		Retention: RetentionConfig{
			Events:       720 * time.Hour,
			BootSessions: 24 * time.Hour,
		},
	}
}

// ApplyDefaults fills unset persistence fields without overriding explicit
// operator configuration.
func ApplyDefaults(config Config) Config {
	defaults := DefaultConfig()
	if config.Backend == "" {
		config.Backend = defaults.Backend
	}
	if config.Path == "" {
		config.Path = defaults.Path
	}
	if config.Retention.Events == 0 {
		config.Retention.Events = defaults.Retention.Events
	}
	if config.Retention.BootSessions == 0 {
		config.Retention.BootSessions = defaults.Retention.BootSessions
	}
	return config
}

// Validate rejects unsupported persistence settings before backend startup.
func Validate(config Config) error {
	switch config.Backend {
	case BackendSQLite, BackendMemory:
		return nil
	default:
		return fmt.Errorf("unsupported persistence backend %q", config.Backend)
	}
}

// ResolvePath resolves a configured database path relative to data-dir.
func ResolvePath(dataDir string, config Config) string {
	config = ApplyDefaults(config)
	if filepath.IsAbs(config.Path) {
		return config.Path
	}
	return filepath.Join(dataDir, config.Path)
}

// EnsureParentDir creates the database parent directory using restrictive
// permissions. SQLite creates the database file itself when it opens the path.
func EnsureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o700)
}
