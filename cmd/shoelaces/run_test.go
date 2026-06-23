package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inngest/shoelaces/environment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandValuePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		configValue string
		envValue    string
		cliValue    string
		expected    string
		expectedURL string
	}{
		{
			name:        "default used when no source is set",
			expected:    ":8081",
			expectedURL: "localhost:8081",
		},
		{
			name:        "config overrides default",
			configValue: "config:8081",
			expected:    "config:8081",
			expectedURL: "config:8081",
		},
		{
			name:        "env overrides config",
			configValue: "config:8081",
			envValue:    "env:8081",
			expected:    "env:8081",
			expectedURL: "env:8081",
		},
		{
			name:        "cli overrides env",
			configValue: "config:8081",
			envValue:    "env:8081",
			cliValue:    "cli:8081",
			expected:    "cli:8081",
			expectedURL: "cli:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("BIND_ADDR", tt.envValue)
			}

			configValues := map[any]any{
				"data-dir":            "../../configs/data-dir",
				"persistence-backend": "memory",
			}
			if tt.configValue != "" {
				configValues["bind-addr"] = tt.configValue
			}

			var got *environment.Environment
			cmd := command("test.toml", configValues, func(env *environment.Environment) error {
				got = env
				return nil
			})

			args := []string{"shoelaces", "run"}
			if tt.cliValue != "" {
				args = append(args, "--bind-addr", tt.cliValue)
			}

			require.NoError(t, cmd.Run(context.Background(), args))
			require.NotNil(t, got)
			assert.Equal(t, tt.expected, got.BindAddr)
			assert.Equal(t, tt.expectedURL, got.BaseURL)
		})
	}
}

func TestCommandAppliesPrecedenceToTFTPConfig(t *testing.T) {
	t.Setenv("TFTP_ADDR", ":2069")

	configValues := map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "memory",
		"tftp-enabled":        true,
		"tftp-addr":           ":1069",
		"tftp-timeout":        "7s",
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "run"}))
	require.NotNil(t, got)

	require.NotNil(t, got.TFTP)
	assert.True(t, got.TFTP.Enabled)
	assert.Equal(t, ":2069", got.TFTP.Addr)
	assert.Equal(t, 7*time.Second, got.TFTP.Timeout)
}

func TestCommandAppliesPrecedenceToLoggingConfig(t *testing.T) {
	t.Setenv("LOG_LEVEL", "error")

	configValues := map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "memory",
		"log-level":           "warn",
		"log-handler":         "text",
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "run", "--log-handler", "json"}))
	require.NotNil(t, got)

	assert.Equal(t, "error", got.LogLevel)
	assert.Equal(t, "json", got.LogHandler)
}

func TestCommandAppliesPrecedenceToPersistenceConfig(t *testing.T) {
	t.Setenv("PERSISTENCE_RETENTION_EVENTS", "48h")
	t.Setenv("PERSISTENCE_RETENTION_EVENTS_SWEEP_INTERVAL", "12h")

	configValues := map[any]any{
		"data-dir":                                           "../../configs/data-dir",
		"persistence-backend":                                "memory",
		"persistence-path":                                   "runtime/config.db",
		"persistence-retention-events":                       "24h",
		"persistence-retention-events-sweep-interval":        "6h",
		"persistence-retention-boot-sessions":                "2h",
		"persistence-retention-boot-sessions-sweep-interval": "30m",
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{
		"shoelaces",
		"run",
		"--persistence-path", "runtime/cli.db",
		"--persistence-retention-boot-sessions", "6h",
		"--persistence-retention-boot-sessions-sweep-interval", "15m",
	}))
	require.NotNil(t, got)

	assert.Equal(t, "memory", got.PersistenceConfig.Backend)
	assert.Equal(t, "runtime/cli.db", got.PersistenceConfig.Path)
	assert.Equal(t, 48*time.Hour, got.PersistenceConfig.Retention.Events)
	assert.Equal(t, 12*time.Hour, got.PersistenceConfig.Retention.EventsSweepInterval)
	assert.Equal(t, 6*time.Hour, got.PersistenceConfig.Retention.BootSessions)
	assert.Equal(t, 15*time.Minute, got.PersistenceConfig.Retention.BootSessionsSweepInterval)
}

func TestCommandRejectsInvalidPersistenceBackend(t *testing.T) {
	configValues := map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "postgres",
	}

	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for invalid persistence backend")
		return nil
	})

	err := cmd.Run(context.Background(), []string{"shoelaces", "run"})
	assert.ErrorContains(t, err, `unsupported persistence backend "postgres"`)
}

func TestCommandDebugIsCLIOnly(t *testing.T) {
	t.Setenv("DEBUG", "true")

	configValues := map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "memory",
	}

	var envOnly *environment.Environment
	envCmd := command("test.toml", configValues, func(env *environment.Environment) error {
		envOnly = env
		return nil
	})

	require.NoError(t, envCmd.Run(context.Background(), []string{"shoelaces", "run"}))
	require.NotNil(t, envOnly)
	assert.False(t, envOnly.Debug)

	var cliDebug *environment.Environment
	cliCmd := command("test.toml", configValues, func(env *environment.Environment) error {
		cliDebug = env
		return nil
	})

	require.NoError(t, cliCmd.Run(context.Background(), []string{"shoelaces", "run", "--debug"}))
	require.NotNil(t, cliDebug)
	assert.True(t, cliDebug.Debug)
}

func TestCommandUIDirPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		configDir bool
		envDir    bool
		cliDir    bool
	}{
		{
			name: "default uses embedded UI",
		},
		{
			name:      "config overrides embedded UI",
			configDir: true,
		},
		{
			name:      "env overrides config",
			configDir: true,
			envDir:    true,
		},
		{
			name:      "cli overrides env",
			configDir: true,
			envDir:    true,
			cliDir:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expected string
			configValue := ""
			envValue := ""
			cliValue := ""
			if tt.configDir {
				configValue = writeCommandTestUIDir(t)
				expected = configValue
			}
			if tt.envDir {
				envValue = writeCommandTestUIDir(t)
				t.Setenv("UI_DIR", envValue)
				expected = envValue
			}
			if tt.cliDir {
				cliValue = writeCommandTestUIDir(t)
				expected = cliValue
			}

			configValues := map[any]any{
				"data-dir":            "../../configs/data-dir",
				"persistence-backend": "memory",
			}
			if configValue != "" {
				configValues["ui-dir"] = configValue
			}

			var got *environment.Environment
			cmd := command("test.toml", configValues, func(env *environment.Environment) error {
				got = env
				return nil
			})

			args := []string{"shoelaces", "run"}
			if cliValue != "" {
				args = append(args, "--ui-dir", cliValue)
			}

			require.NoError(t, cmd.Run(context.Background(), args))
			require.NotNil(t, got)
			assert.Equal(t, expected, got.UIDir)
			assert.Equal(t, expected != "", got.UIOverrideDirSet)
		})
	}
}

func TestCommandStaticDirCompatibilityAlias(t *testing.T) {
	uiDir := writeCommandTestUIDir(t)
	configValues := map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "memory",
		"static-dir":          uiDir,
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "run"}))
	require.NotNil(t, got)
	assert.Equal(t, uiDir, got.UIDir)
	assert.True(t, got.UIOverrideDirSet)
}

func writeCommandTestUIDir(t *testing.T) string {
	t.Helper()

	uiDir := t.TempDir()
	templatesDir := filepath.Join(uiDir, "templates/html")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	templates := map[string]string{
		"header.html":   `{{ define "header" }}header{{ end }}`,
		"index.html":    `{{ define "index" }}index{{ end }}`,
		"events.html":   `{{ define "events" }}events{{ end }}`,
		"mappings.html": `{{ define "mappings" }}mappings{{ end }}`,
		"footer.html":   `{{ define "footer" }}footer{{ end }}`,
	}
	for name, content := range templates {
		require.NoError(t, os.WriteFile(filepath.Join(templatesDir, name), []byte(content), 0o644))
	}

	return uiDir
}
