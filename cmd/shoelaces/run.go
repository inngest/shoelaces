// Copyright 2018 ThousandEyes Inc.
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

package main

import (
	"context"
	"fmt"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/persistence"
	cli "github.com/urfave/cli/v3"
)

type serverRunner func(*environment.Environment) error

func runCommand(configValues map[any]any, run serverRunner) *cli.Command {
	defaults := environment.DefaultOptions()
	tftpDefaults := environment.DefaultTFTPConfig()
	persistenceDefaults := persistence.DefaultConfig()
	configSource := cli.NewMapSource("config", configValues)

	flagSources := func(name, env string) cli.ValueSourceChain {
		// Keep the previous precedence: command-line flags are handled by
		// urfave/cli first, then env vars, then config values, then defaults.
		return cli.NewValueSourceChain(cli.EnvVar(env), cli.NewMapValueSource(name, configSource))
	}

	return &cli.Command{
		Name:      "run",
		Usage:     "Start the Shoelaces server",
		UsageText: "shoelaces [options...] run [run options...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "bind-addr",
				Value:   defaults.BindAddr,
				Usage:   "The address where Shoelaces will listen",
				Sources: flagSources("bind-addr", "BIND_ADDR"),
			},
			&cli.StringFlag{
				Name:    "base-url",
				Value:   defaults.BaseURL,
				Usage:   "The base Shoelaces URL. Defaults to bind-addr when omitted.",
				Sources: flagSources("base-url", "BASE_URL"),
			},
			&cli.StringFlag{
				Name:    "data-dir",
				Value:   defaults.DataDir,
				Usage:   "Directory with mappings, configs, templates, etc.",
				Sources: flagSources("data-dir", "DATA_DIR"),
			},
			&cli.StringFlag{
				Name:    "ui-dir",
				Value:   defaults.UIDir,
				Usage:   "Optional custom web UI directory with templates and static frontend assets",
				Sources: flagSources("ui-dir", "UI_DIR"),
			},
			&cli.StringFlag{
				Name:    "static-dir",
				Usage:   "Deprecated alias for --ui-dir",
				Sources: flagSources("static-dir", "STATIC_DIR"),
			},
			&cli.StringFlag{
				Name:    "env-dir",
				Value:   defaults.EnvDir,
				Usage:   "Directory with environment overrides",
				Sources: flagSources("env-dir", "ENV_DIR"),
			},
			&cli.StringFlag{
				Name:    "template-extension",
				Value:   defaults.TemplateExtension,
				Usage:   "Shoelaces template extension",
				Sources: flagSources("template-extension", "TEMPLATE_EXTENSION"),
			},
			&cli.StringFlag{
				Name:    "mappings-file",
				Value:   defaults.MappingsFile,
				Usage:   "Mappings YAML file, relative to data-dir",
				Sources: flagSources("mappings-file", "MAPPINGS_FILE"),
			},
			&cli.BoolFlag{
				Name:  "debug",
				Value: defaults.Debug,
				Usage: "Enable debug mode (CLI only)",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   defaults.LogLevel,
				Usage:   "Log level: debug, info, warn, or error",
				Sources: flagSources("log-level", "LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "log-handler",
				Value:   defaults.LogHandler,
				Usage:   "Log handler: dev, text, or json",
				Sources: flagSources("log-handler", "LOG_HANDLER"),
			},
			&cli.BoolFlag{
				Name:    "tftp-enabled",
				Value:   tftpDefaults.Enabled,
				Usage:   "Enable embedded TFTP server",
				Sources: flagSources("tftp-enabled", "TFTP_ENABLED"),
			},
			&cli.StringFlag{
				Name:    "tftp-addr",
				Value:   tftpDefaults.Addr,
				Usage:   "TFTP listen address (UDP), e.g. 0.0.0.0:69",
				Sources: flagSources("tftp-addr", "TFTP_ADDR"),
			},
			&cli.StringFlag{
				Name:    "tftp-root",
				Value:   tftpDefaults.Root,
				Usage:   "Directory to serve via TFTP",
				Sources: flagSources("tftp-root", "TFTP_ROOT"),
			},
			&cli.BoolFlag{
				Name:    "tftp-readonly",
				Value:   tftpDefaults.Readonly,
				Usage:   "Disable TFTP uploads",
				Sources: flagSources("tftp-readonly", "TFTP_READONLY"),
			},
			&cli.DurationFlag{
				Name:    "tftp-timeout",
				Value:   tftpDefaults.Timeout,
				Usage:   "Per-request TFTP timeout",
				Sources: flagSources("tftp-timeout", "TFTP_TIMEOUT"),
			},
			&cli.StringFlag{
				Name:    "persistence-backend",
				Value:   persistenceDefaults.Backend,
				Usage:   "Runtime persistence backend: sqlite or memory",
				Sources: flagSources("persistence-backend", "PERSISTENCE_BACKEND"),
			},
			&cli.StringFlag{
				Name:    "persistence-path",
				Value:   persistenceDefaults.Path,
				Usage:   "SQLite persistence database path, relative to data-dir unless absolute",
				Sources: flagSources("persistence-path", "PERSISTENCE_PATH"),
			},
			&cli.DurationFlag{
				Name:    "persistence-retention-events",
				Value:   persistenceDefaults.Retention.Events,
				Usage:   "Retention window for persisted event history",
				Sources: flagSources("persistence-retention-events", "PERSISTENCE_RETENTION_EVENTS"),
			},
			&cli.DurationFlag{
				Name:    "persistence-retention-events-sweep-interval",
				Value:   persistenceDefaults.Retention.EventsSweepInterval,
				Usage:   "How often persisted event retention cleanup runs while Shoelaces is running",
				Sources: flagSources("persistence-retention-events-sweep-interval", "PERSISTENCE_RETENTION_EVENTS_SWEEP_INTERVAL"),
			},
			&cli.DurationFlag{
				Name:    "persistence-retention-boot-sessions",
				Value:   persistenceDefaults.Retention.BootSessions,
				Usage:   "Retention window for persisted boot/config references",
				Sources: flagSources("persistence-retention-boot-sessions", "PERSISTENCE_RETENTION_BOOT_SESSIONS"),
			},
			&cli.DurationFlag{
				Name:    "persistence-retention-boot-sessions-sweep-interval",
				Value:   persistenceDefaults.Retention.BootSessionsSweepInterval,
				Usage:   "How often expired boot/config reference cleanup runs while Shoelaces is running",
				Sources: flagSources("persistence-retention-boot-sessions-sweep-interval", "PERSISTENCE_RETENTION_BOOT_SESSIONS_SWEEP_INTERVAL"),
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			options := optionsFromCommand(cmd)
			if err := validateOptions(options); err != nil {
				return err
			}
			return run(environment.New(options))
		},
	}
}

func optionsFromCommand(cmd *cli.Command) environment.Options {
	tftp := &environment.TFTPConfig{
		Enabled:  cmd.Bool("tftp-enabled"),
		Addr:     cmd.String("tftp-addr"),
		Root:     cmd.String("tftp-root"),
		Readonly: cmd.Bool("tftp-readonly"),
		Timeout:  cmd.Duration("tftp-timeout"),
	}
	persistenceConfig := persistence.Config{
		Backend: cmd.String("persistence-backend"),
		Path:    cmd.String("persistence-path"),
		Retention: persistence.RetentionConfig{
			Events:                    cmd.Duration("persistence-retention-events"),
			EventsSweepInterval:       cmd.Duration("persistence-retention-events-sweep-interval"),
			BootSessions:              cmd.Duration("persistence-retention-boot-sessions"),
			BootSessionsSweepInterval: cmd.Duration("persistence-retention-boot-sessions-sweep-interval"),
		},
	}
	uiDir := cmd.String("ui-dir")
	uiDirSet := cmd.IsSet("ui-dir")
	if !uiDirSet && cmd.IsSet("static-dir") {
		uiDir = cmd.String("static-dir")
		uiDirSet = true
	}

	return environment.Options{
		BindAddr:          cmd.String("bind-addr"),
		BaseURL:           cmd.String("base-url"),
		DataDir:           cmd.String("data-dir"),
		UIDir:             uiDir,
		UIOverrideDirSet:  uiDirSet,
		EnvDir:            cmd.String("env-dir"),
		TemplateExtension: cmd.String("template-extension"),
		MappingsFile:      cmd.String("mappings-file"),
		Debug:             cmd.Bool("debug"),
		LogLevel:          cmd.String("log-level"),
		LogHandler:        cmd.String("log-handler"),
		TFTP:              tftp,
		Persistence:       persistenceConfig,
	}
}

func validateOptions(options environment.Options) error {
	if options.DataDir == "" {
		return fmt.Errorf("you must specify the data-dir parameter")
	}
	if err := persistence.Validate(persistence.ApplyDefaults(options.Persistence)); err != nil {
		return err
	}
	return nil
}
