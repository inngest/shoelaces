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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults(t *testing.T) {
	config := ApplyDefaults(Config{})

	assert.Equal(t, BackendSQLite, config.Backend)
	assert.Equal(t, "runtime/shoelaces.db", config.Path)
	assert.Equal(t, 720*time.Hour, config.Retention.Events)
	assert.Equal(t, 24*time.Hour, config.Retention.BootSessions)
}

func TestValidateRejectsUnsupportedBackend(t *testing.T) {
	assert.NoError(t, Validate(Config{Backend: BackendSQLite}))
	assert.NoError(t, Validate(Config{Backend: BackendMemory}))

	err := Validate(Config{Backend: "postgres"})
	assert.ErrorContains(t, err, `unsupported persistence backend "postgres"`)
}

func TestResolvePath(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")

	assert.Equal(t, filepath.Join(dataDir, "runtime", "shoelaces.db"), ResolvePath(dataDir, Config{}))

	absolute := filepath.Join(t.TempDir(), "shoelaces.db")
	assert.Equal(t, absolute, ResolvePath(dataDir, Config{Path: absolute}))
}

func TestEnsureParentDirCreatesRestrictiveDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runtime", "nested", "shoelaces.db")

	require.NoError(t, EnsureParentDir(dbPath))

	info, err := os.Stat(filepath.Dir(dbPath))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}
