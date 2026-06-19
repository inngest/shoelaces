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

	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "-config", "flag.toml"}, lookupEnv))
	assert.Equal(t, "flag.toml", configPathFromArgs([]string{"shoelaces", "--config=flag.toml"}, lookupEnv))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces"}, lookupEnv))
	assert.Equal(t, "env.toml", configPathFromArgs([]string{"shoelaces", "--", "--config=ignored.toml"}, lookupEnv))
}
