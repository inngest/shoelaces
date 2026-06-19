package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigPathFromArgs(t *testing.T) {
	lookupEnv := func(key string) (string, bool) {
		if key == "CONFIG" {
			return "env.toml", true
		}
		return "", false
	}
	lookupDefault := func() string {
		return "default.yaml"
	}

	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "-config", "flag.toml"}, lookupEnv, lookupDefault))
	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "--config=flag.toml"}, lookupEnv, lookupDefault))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces"}, lookupEnv, lookupDefault))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces", "--", "--config=ignored.toml"}, lookupEnv, lookupDefault))
	assert.Equal(t, "default.yaml", configPathFromArgs([]string{"shoelaces"}, func(string) (string, bool) {
		return "", false
	}, lookupDefault))
	assert.Equal(t, "default.yaml", configPathFromArgs([]string{"shoelaces"}, func(string) (string, bool) {
		return "", true
	}, lookupDefault))
}

func TestDiscoverDefaultConfigPath(t *testing.T) {
	paths := []string{
		"/etc/shoelaces/shoelaces.yaml",
		"/etc/shoelaces/shoelaces.json",
		"/etc/shoelaces/shoelaces.toml",
	}
	existing := map[string]bool{
		"/etc/shoelaces/shoelaces.json": true,
		"/etc/shoelaces/shoelaces.toml": true,
	}

	path := discoverDefaultConfigPath(paths, func(path string) bool {
		return existing[path]
	})

	assert.Equal(t, "/etc/shoelaces/shoelaces.json", path)
}

func TestDiscoverDefaultConfigPathReturnsEmptyWhenNoDefaultsExist(t *testing.T) {
	path := discoverDefaultConfigPath(defaultConfigPaths, func(string) bool {
		return false
	})

	assert.Empty(t, path)
}
