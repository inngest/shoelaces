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
	"os"
	"strings"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var defaultConfigPaths = []string{
	"/etc/shoelaces/shoelaces.yaml",
	"/etc/shoelaces/shoelaces.json",
	"/etc/shoelaces/shoelaces.toml",
}

func configPathFromArgs(args []string, lookupEnv func(string) (string, bool), lookupDefault func() string) string {
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
	if value, ok := lookupEnv("CONFIG"); ok && value != "" {
		return value
	}
	return lookupDefault()
}

func defaultConfigPath() string {
	return discoverDefaultConfigPath(defaultConfigPaths, fileExists)
}

func discoverDefaultConfigPath(paths []string, exists func(string) bool) string {
	for _, path := range paths {
		if exists(path) {
			return path
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
