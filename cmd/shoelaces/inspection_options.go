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
	"fmt"
	"os"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/persistence"
)

func inspectionOptionsFromConfig(configValues map[any]any) (environment.Options, error) {
	return inspectionOptionsFromConfigWithEnv(configValues, os.LookupEnv)
}

func inspectionOptionsFromConfigWithEnv(configValues map[any]any, lookupEnv func(string) (string, bool)) (environment.Options, error) {
	options := environment.DefaultOptions()
	persistenceConfig := persistence.DefaultConfig()

	dataDir, err := stringInspectionConfigValue(configValues, "data-dir", "DATA_DIR", lookupEnv)
	if err != nil {
		return environment.Options{}, err
	}
	if dataDir != "" {
		options.DataDir = dataDir
	}

	backend, err := stringInspectionConfigValue(configValues, "persistence-backend", "PERSISTENCE_BACKEND", lookupEnv)
	if err != nil {
		return environment.Options{}, err
	}
	if backend != "" {
		persistenceConfig.Backend = backend
	}

	path, err := stringInspectionConfigValue(configValues, "persistence-path", "PERSISTENCE_PATH", lookupEnv)
	if err != nil {
		return environment.Options{}, err
	}
	if path != "" {
		persistenceConfig.Path = path
	}

	options.Persistence = persistenceConfig
	return options, nil
}

func stringInspectionConfigValue(configValues map[any]any, key, env string, lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv != nil {
		if value, ok := lookupEnv(env); ok {
			return value, nil
		}
	}
	return stringConfigValue(configValues, key)
}

func stringConfigValue(configValues map[any]any, key string) (string, error) {
	value, ok := configValues[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("configuration value %q must be a string", key)
	}
	return text, nil
}
