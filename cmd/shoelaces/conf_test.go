package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfigSupportsStructuredFormats(t *testing.T) {
	tests := map[string]string{
		"shoelaces.toml": `
bind-addr = "localhost:8081"
data-dir = "configs/data-dir/"
ui-dir = "/srv/shoelaces/ui"
log-level = "warn"
log-handler = "json"

[persistence]
backend = "sqlite"
path = "runtime/test.db"

[persistence.retention]
events = "24h"
bootSessions = "2h"

[tftp]
enabled = true
address = ":69"
root = "/var/lib/shoelaces/tftp"
readonly = true
timeout = "5s"
`,
		"shoelaces.yaml": `
bind-addr: localhost:8081
data-dir: configs/data-dir/
ui-dir: /srv/shoelaces/ui
log-level: warn
log-handler: json
persistence:
  backend: sqlite
  path: runtime/test.db
  retention:
    events: 24h
    bootSessions: 2h
tftp:
  enabled: true
  address: ":69"
  root: /var/lib/shoelaces/tftp
  readonly: true
  timeout: 5s
`,
		"shoelaces.json": `{
  "bind-addr": "localhost:8081",
  "data-dir": "configs/data-dir/",
  "ui-dir": "/srv/shoelaces/ui",
  "log-level": "warn",
  "log-handler": "json",
  "persistence": {
    "backend": "sqlite",
    "path": "runtime/test.db",
    "retention": {
      "events": "24h",
      "bootSessions": "2h"
    }
  },
  "tftp": {
    "enabled": true,
    "address": ":69",
    "root": "/var/lib/shoelaces/tftp",
    "readonly": true,
    "timeout": "5s"
  }
}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), name)
			require.NoError(t, os.WriteFile(configPath, []byte(body), 0o644))

			values, err := readConfig(configPath)
			require.NoError(t, err)

			assert.Equal(t, "localhost:8081", values["bind-addr"])
			assert.Equal(t, "configs/data-dir/", values["data-dir"])
			assert.Equal(t, "/srv/shoelaces/ui", values["ui-dir"])
			assert.Equal(t, "warn", values["log-level"])
			assert.Equal(t, "json", values["log-handler"])
			assert.Equal(t, "sqlite", values["persistence-backend"])
			assert.Equal(t, "runtime/test.db", values["persistence-path"])
			assert.Equal(t, "24h", values["persistence-retention-events"])
			assert.Equal(t, "2h", values["persistence-retention-boot-sessions"])
			assert.Equal(t, true, values["tftp-enabled"])
			assert.Equal(t, ":69", values["tftp-addr"])
			assert.Equal(t, "/var/lib/shoelaces/tftp", values["tftp-root"])
			assert.Equal(t, true, values["tftp-readonly"])
			assert.Equal(t, "5s", values["tftp-timeout"])
		})
	}
}

func TestReadConfigSupportsFlatPersistenceKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shoelaces.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
data-dir = "configs/data-dir/"
persistence-backend = "memory"
persistence-path = "/tmp/ignored.db"
persistence-retention-events = "1h"
persistence-retention-boot-sessions = "30m"
`), 0o644))

	values, err := readConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "memory", values["persistence-backend"])
	assert.Equal(t, "/tmp/ignored.db", values["persistence-path"])
	assert.Equal(t, "1h", values["persistence-retention-events"])
	assert.Equal(t, "30m", values["persistence-retention-boot-sessions"])
	_, err = time.ParseDuration(values["persistence-retention-events"].(string))
	assert.NoError(t, err)
}

func TestReadConfigSamplesIncludePersistence(t *testing.T) {
	for _, configPath := range []string{
		"../../configs/shoelaces.toml",
		"../../configs/shoelaces.yaml",
		"../../configs/shoelaces.json",
		"../../dev/shoelaces.yaml",
	} {
		t.Run(configPath, func(t *testing.T) {
			values, err := readConfig(configPath)
			require.NoError(t, err)

			assert.Equal(t, "sqlite", values["persistence-backend"])
			assert.Equal(t, "runtime/shoelaces.db", values["persistence-path"])
			assert.Equal(t, "720h", values["persistence-retention-events"])
			assert.Equal(t, "24h", values["persistence-retention-boot-sessions"])
		})
	}
}

func TestReadConfigRejectsDebugConfigOption(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shoelaces.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("debug: true\n"), 0o644))

	_, err := readConfig(configPath)
	assert.ErrorContains(t, err, "configuration variable provided but not defined: debug")
}

func TestReadConfigRejectsUnsupportedFormat(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shoelaces.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("data-dir=configs/data-dir/"), 0o644))

	_, err := readConfig(configPath)
	assert.ErrorContains(t, err, "unsupported config file extension")
}
