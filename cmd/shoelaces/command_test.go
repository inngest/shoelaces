package main

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/inngest/shoelaces/environment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandWithoutSubcommandPrintsHelp(t *testing.T) {
	var output bytes.Buffer
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute without a subcommand")
		return nil
	})
	cmd.Writer = &output

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces"}))
	assert.Contains(t, output.String(), "USAGE")
	assert.Contains(t, output.String(), "COMMANDS")
	assert.Contains(t, output.String(), "run")
	assert.Contains(t, output.String(), "events")
	assert.Contains(t, output.String(), "servers")
	assert.Contains(t, output.String(), "boot-sessions")
	assert.Contains(t, output.String(), "--config")
	assert.Contains(t, output.String(), "--version")
	assert.NotContains(t, output.String(), "--bind-addr")
	assert.NotContains(t, output.String(), "--tftp-enabled")
	assert.NotContains(t, output.String(), "--persistence-backend")
}

func TestRunCommandHelpShowsServerOptions(t *testing.T) {
	var output bytes.Buffer
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for run help")
		return nil
	})
	cmd.Writer = &output

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "run", "--help"}))
	assert.Contains(t, output.String(), "--config")
	assert.Contains(t, output.String(), "--bind-addr")
	assert.Contains(t, output.String(), "--tftp-enabled")
	assert.Contains(t, output.String(), "--persistence-backend")
	assert.NotContains(t, output.String(), "--version")
}

func TestServerOptionsAreRunLocal(t *testing.T) {
	cmd := command("test.toml", map[any]any{
		"data-dir":            "../../configs/data-dir",
		"persistence-backend": "memory",
	}, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for invalid root options")
		return nil
	})
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	err := cmd.Run(context.Background(), []string{"shoelaces", "--bind-addr", "localhost:9999", "run"})
	assert.ErrorContains(t, err, "flag provided but not defined")
}

func TestCommandDoesNotRegisterStartAlias(t *testing.T) {
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for start")
		return nil
	})
	assert.Nil(t, cmd.Command("start"))
}

func TestCommandVersionDoesNotRequireDataDir(t *testing.T) {
	cmd := command("", nil, func(env *environment.Environment) error {
		t.Fatal("server runner should not execute for -version")
		return nil
	})
	cmd.Writer = io.Discard

	require.NoError(t, cmd.Run(context.Background(), []string{"shoelaces", "--version"}))
}
