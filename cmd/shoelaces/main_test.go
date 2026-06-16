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
			return "env.conf", true
		}
		return "", false
	}

	assert.Equal(t, "flag.conf", configPathFromArgs([]string{"shoelaces", "-config", "flag.conf"}, lookupEnv))
	assert.Equal(t, "flag.conf", configPathFromArgs([]string{"shoelaces", "--config=flag.conf"}, lookupEnv))
	assert.Equal(t, "env.conf", configPathFromArgs([]string{"shoelaces"}, lookupEnv))
	assert.Equal(t, "env.conf", configPathFromArgs([]string{"shoelaces", "--", "--config=ignored.conf"}, lookupEnv))
}

func TestReadConfigSupportsFlatAndLegacyTFTPSections(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "shoelaces.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(`
bind-addr=localhost:8081
data-dir configs/data-dir/
debug

[tftp]
enabled = true
address = ":69"
root = "/var/lib/shoelaces/tftp"
readonly = true
timeout_seconds = 5
`), 0o644))

	values, err := readConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "localhost:8081", values["bind-addr"])
	assert.Equal(t, "configs/data-dir/", values["data-dir"])
	assert.Equal(t, "true", values["debug"])
	assert.Equal(t, "true", values["tftp-enabled"])
	assert.Equal(t, ":69", values["tftp-addr"])
	assert.Equal(t, "/var/lib/shoelaces/tftp", values["tftp-root"])
	assert.Equal(t, "true", values["tftp-readonly"])
	assert.Equal(t, "5s", values["tftp-timeout"])
}

func TestCommandAppliesCLIEnvConfigPrecedence(t *testing.T) {
	t.Setenv("BIND_ADDR", "env:8081")
	t.Setenv("TFTP_ADDR", ":2069")

	configValues := map[any]any{
		"bind-addr":    "config:8081",
		"data-dir":     "../../configs/data-dir",
		"static-dir":   "../../web",
		"tftp-enabled": "true",
		"tftp-addr":    ":1069",
		"tftp-timeout": "7s",
	}

	var got *environment.Environment
	cmd := command("test.conf", configValues, func(env *environment.Environment) error {
		got = env
		return nil
	})

	err := cmd.Run(context.Background(), []string{"shoelaces", "--bind-addr", "cli:8081"})
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "cli:8081", got.BindAddr)
	assert.Equal(t, "cli:8081", got.BaseURL)
	require.NotNil(t, got.TFTP)
	assert.True(t, got.TFTP.Enabled)
	assert.Equal(t, ":2069", got.TFTP.Addr)
	assert.Equal(t, 7*time.Second, got.TFTP.Timeout)
}

func TestCommandVersionDoesNotRequireDataDir(t *testing.T) {
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for -version")
		return nil
	})
	cmd.Writer = io.Discard

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "--version"}))
}
