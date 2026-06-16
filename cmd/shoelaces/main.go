// Copyright 2018 ThousandEyes Inc.
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
	"net/http"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/thousandeyes/shoelaces/environment"
	"github.com/thousandeyes/shoelaces/handlers"
	"github.com/thousandeyes/shoelaces/router"
	"github.com/thousandeyes/shoelaces/tftpserver"
	cli "github.com/urfave/cli/v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cmd, err := newCommand(os.Args, runServer)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(env *environment.Environment) error {
	app := handlers.MiddlewareChain(env).Then(router.ShoelacesRouter(env))

	if env.TFTP != nil && env.TFTP.Enabled {
		tf := tftpserver.New(env.TFTP.Addr, env.TFTP.Root, env.TFTP.Readonly, env.TFTP.Timeout)
		go func() {
			env.Logger.Info(
				"component", "tftp",
				"transport", "udp",
				"addr", env.TFTP.Addr,
				"root", env.TFTP.Root,
				"readonly", env.TFTP.Readonly,
				"msg", "listening",
			)
			if err := tf.ListenAndServe(); err != nil {
				env.Logger.Error("component", "tftp", "err", err)
			}
		}()
	}

	env.Logger.Info("component", "main", "transport", "http", "addr", env.BindAddr, "msg", "listening")
	return http.ListenAndServe(env.BindAddr, app)
}

func versionString() string {
	return fmt.Sprintf("shoelaces %s\ncommit: %s\ndate: %s\nbuilt by: %s\n", version, commit, date, builtBy)
}

type serverRunner func(*environment.Environment) error

func newCommand(args []string, run serverRunner) (*cli.Command, error) {
	// The config file path must be known before the urfave command is built
	// because config values are wired in as flag value sources.
	configPath := configPathFromArgs(args, os.LookupEnv)
	configValues, err := readConfig(configPath)
	if err != nil {
		return nil, err
	}
	return command(configPath, configValues, run), nil
}

func command(configPath string, configValues map[any]any, run serverRunner) *cli.Command {
	defaults := environment.DefaultOptions()
	tftpDefaults := environment.DefaultTFTPConfig()
	configSource := cli.NewMapSource("config", configValues)

	flagSources := func(name, env string) cli.ValueSourceChain {
		// Keep the previous precedence: command-line flags are handled by
		// urfave/cli first, then env vars, then config values, then defaults.
		return cli.NewValueSourceChain(cli.EnvVar(env), cli.NewMapValueSource(name, configSource))
	}

	return &cli.Command{
		Name:        "shoelaces",
		Usage:       "automated server bootstrapping",
		UsageText:   "shoelaces [options...]",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Value:   configPath,
				Usage:   "Path to a config file",
				Sources: cli.EnvVars("CONFIG"),
			},
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
				Name:    "debug",
				Value:   defaults.Debug,
				Usage:   "Enable debug mode",
				Sources: flagSources("debug", "DEBUG"),
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
			&cli.BoolFlag{
				Name:  "version",
				Usage: "Print version information and exit",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Bool("version") {
				_, err := fmt.Fprint(cmd.Writer, versionString())
				return err
			}

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
		TFTP:              tftp,
	}
}

func validateOptions(options environment.Options) error {
	if options.DataDir == "" {
		return fmt.Errorf("you must specify the data-dir parameter")
	}
	return nil
}

func configPathFromArgs(args []string, lookupEnv func(string) (string, bool)) string {
	// Only discover -config/--config here. Full argument validation happens
	// later in urfave/cli after config values have been loaded.
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "-config" || arg == "--config" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if value, ok := strings.CutPrefix(arg, "-config="); ok {
			return value
		}
		if value, ok := strings.CutPrefix(arg, "--config="); ok {
			return value
		}
	}
	if value, ok := lookupEnv("CONFIG"); ok {
		return value
	}
	return ""
}

func readConfig(path string) (map[any]any, error) {
	values := map[any]any{}
	if path == "" {
		return values, nil
	}

	parser, err := parserForConfig(path)
	if err != nil {
		return nil, err
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(path), parser); err != nil {
		return nil, err
	}

	return configValuesFromKoanf(k)
}
