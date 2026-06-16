package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thousandeyes/shoelaces/internal/environment"
)

func TestConfigPathFromArgs(t *testing.T) {
	lookupEnv := func(key string) (string, bool) {
		if key == "CONFIG" {
			return "env.toml", true
		}
		return "", false
	}

	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "-config", "flag.toml"}, lookupEnv))
	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "--config=flag.toml"}, lookupEnv))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces"}, lookupEnv))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces", "--", "--config=ignored.toml"}, lookupEnv))
}

func TestCommandValuePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		configValue string
		envValue    string
		cliValue    string
		expected    string
	}{
		{
			name:     "default used when no source is set",
			expected: "localhost:8081",
		},
		{
			name:        "config overrides default",
			configValue: "config:8081",
			expected:    "config:8081",
		},
		{
			name:        "env overrides config",
			configValue: "config:8081",
			envValue:    "env:8081",
			expected:    "env:8081",
		},
		{
			name:        "cli overrides env",
			configValue: "config:8081",
			envValue:    "env:8081",
			cliValue:    "cli:8081",
			expected:    "cli:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("BIND_ADDR", tt.envValue)
			}

			configValues := map[any]any{
				"data-dir": "../../configs/data-dir",
			}
			if tt.configValue != "" {
				configValues["bind-addr"] = tt.configValue
			}

			var got *environment.Environment
			cmd := command("test.toml", configValues, func(env *environment.Environment) error {
				got = env
				return nil
			})

			args := []string{"shoelaces"}
			if tt.cliValue != "" {
				args = append(args, "--bind-addr", tt.cliValue)
			}

			require.NoError(t, cmd.Run(context.Background(), args))
			require.NotNil(t, got)
			assert.Equal(t, tt.expected, got.BindAddr)
		})
	}
}

func TestCommandAppliesPrecedenceToTFTPConfig(t *testing.T) {
	t.Setenv("TFTP_ADDR", ":2069")

	configValues := map[any]any{
		"data-dir":     "../../configs/data-dir",
		"tftp-enabled": true,
		"tftp-addr":    ":1069",
		"tftp-timeout": "7s",
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces"}))
	require.NotNil(t, got)

	require.NotNil(t, got.TFTP)
	assert.True(t, got.TFTP.Enabled)
	assert.Equal(t, ":2069", got.TFTP.Addr)
	assert.Equal(t, 7*time.Second, got.TFTP.Timeout)
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
				"data-dir": "../../configs/data-dir",
			}
			if configValue != "" {
				configValues["ui-dir"] = configValue
			}

			var got *environment.Environment
			cmd := command("test.toml", configValues, func(env *environment.Environment) error {
				got = env
				return nil
			})

			args := []string{"shoelaces"}
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
		"data-dir":   "../../configs/data-dir",
		"static-dir": uiDir,
	}

	var got *environment.Environment
	cmd := command("test.toml", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces"}))
	require.NotNil(t, got)
	assert.Equal(t, uiDir, got.UIDir)
	assert.True(t, got.UIOverrideDirSet)
}

func TestCommandVersionDoesNotRequireDataDir(t *testing.T) {
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for -version")
		return nil
	})
	cmd.Writer = io.Discard

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "--version"}))
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
