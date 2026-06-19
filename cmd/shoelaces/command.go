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
	"os"

	cli "github.com/urfave/cli/v3"
)

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
	return &cli.Command{
		Name:        "shoelaces",
		Usage:       "automated server bootstrapping",
		UsageText:   "shoelaces [options...] <command>",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Value:   configPath,
				Usage:   "Path to a config file",
				Sources: cli.EnvVars("CONFIG"),
			},
			&cli.BoolFlag{
				Name:  "version",
				Usage: "Print version information and exit",
				Local: true,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Bool("version") {
				_, err := fmt.Fprint(cmd.Writer, versionString())
				return err
			}
			return cli.ShowRootCommandHelp(cmd)
		},
		Commands: []*cli.Command{
			runCommand(configValues, run),
			eventsCommand(configValues),
			serversCommand(configValues),
			bootSessionsCommand(configValues),
		},
	}
}
