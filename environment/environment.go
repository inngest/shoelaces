// Copyright 2018 ThousandEyes Inc.
// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package environment

import (
	"fmt"
	"html/template"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/polling"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/templates"
)

// Environment struct holds the shoelaces instance global data.
type Environment struct {
	HostnameMaps []mappings.HostnameMap
	NetworkMaps  []mappings.NetworkMap
	// MappingResolver holds the new target resolver for the mappings.yaml
	// schema used by polling and manual boot paths.
	MappingResolver *mappings.Resolver
	ServerStates    *server.States
	EventLog        *event.Log
	Polling         *polling.Service
	ParamsBlacklist []string
	Templates       *templates.ShoelacesTemplates // Dynamic slc templates
	StaticTemplates *template.Template            // Static Templates
	Environments    []string                      // Valid config environments
	Logger          log.Logger

	BindAddr          string
	BaseURL           string
	DataDir           string
	UIDir             string
	UIOverrideDirSet  bool
	EnvDir            string
	TFTP              *TFTPConfig
	TemplateExtension string
	MappingsFile      string
	Debug             bool
	LogLevel          string
	LogHandler        string
}

// New returns an initialized environment structure
func New(options Options) *Environment {
	env := defaultEnvironment()
	env.applyOptions(options)
	logOptions := []log.Option{log.WithLevelString(env.LogLevel), log.WithHandlerString(env.LogHandler)}
	if env.Debug {
		logOptions = append(logOptions, log.WithLevel(log.LevelDebug))
	}
	env.Logger = log.MakeLogger(os.Stdout, logOptions...)
	env.Templates = templates.New(env.Logger)

	if env.TFTP != nil && env.TFTP.Root == "./tftp" && env.DataDir != "" {
		env.TFTP.Root = env.DataDir + "/tftp"
	}

	if env.BaseURL == "" {
		env.BaseURL = env.BindAddr
	}

	env.Environments = env.initEnvOverrides()

	env.EventLog = &event.Log{}

	env.Logger.Info("Override found", "component", "environment", "environment", env.Environments)

	mappingsPath := path.Join(env.DataDir, env.MappingsFile)
	if err := env.initMappings(mappingsPath); err != nil {
		panic(err)
	}

	env.initStaticTemplates()
	env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
	env.Polling = polling.NewService(env.Logger, env.ServerStates, env.MappingResolver, env.EventLog, env.Templates, env.BaseURL)
	server.StartStateCleaner(env.Logger, env.ServerStates)

	return env
}

func defaultEnvironment() *Environment {
	env := &Environment{}
	env.NetworkMaps = make([]mappings.NetworkMap, 0)
	env.HostnameMaps = make([]mappings.HostnameMap, 0)
	env.ServerStates = &server.States{Servers: make(map[string]*server.State)}
	env.EventLog = &event.Log{}
	env.ParamsBlacklist = []string{"baseURL"}
	env.Environments = make([]string, 0)
	env.Logger = log.MakeLogger(os.Stdout)
	env.Templates = templates.New(env.Logger)
	env.Polling = polling.NewService(env.Logger, env.ServerStates, env.MappingResolver, env.EventLog, env.Templates, env.BaseURL)

	return env
}

func (env *Environment) initStaticTemplates() {
	if env.UsesUIOverride() {
		env.initStaticTemplatesFromDisk()
		return
	}

	staticTemplates := []string{
		"header.html",
		"index.html",
		"events.html",
		"mappings.html",
		"footer.html",
	}

	env.StaticTemplates = template.Must(template.ParseFS(shoelaces.TemplateFS(), staticTemplates...))
}

// UsesUIOverride reports whether UI templates/assets should be loaded from disk.
func (env *Environment) UsesUIOverride() bool {
	return env.UIOverrideDirSet && env.UIDir != ""
}

func (env *Environment) initStaticTemplatesFromDisk() {
	staticTemplates := []string{
		filepath.Join(env.UIDir, "templates/html/header.html"),
		filepath.Join(env.UIDir, "templates/html/index.html"),
		filepath.Join(env.UIDir, "templates/html/events.html"),
		filepath.Join(env.UIDir, "templates/html/mappings.html"),
		filepath.Join(env.UIDir, "templates/html/footer.html"),
	}

	env.StaticTemplates = template.Must(template.ParseFiles(staticTemplates...))
}

func (env *Environment) initEnvOverrides() []string {
	var environments = make([]string, 0)
	envPath := filepath.Join(env.DataDir, env.EnvDir)
	files, err := os.ReadDir(envPath)
	if err == nil {
		for _, f := range files {
			if f.IsDir() {
				environments = append(environments, f.Name())
			}
		}
	}
	return environments
}

func (env *Environment) initMappings(mappingsPath string) error {
	configMappings, err := mappings.ParseMappings(env.Logger, mappingsPath)
	if err != nil {
		return err
	}
	env.MappingResolver, err = mappings.NewResolver(configMappings)
	if err != nil {
		return err
	}
	env.MappingResolver.WithLogger(env.Logger)

	for _, configNetMap := range configMappings.NetworkMaps {
		_, ipnet, err := net.ParseCIDR(configNetMap.Network)
		if err != nil {
			return err
		}

		script, err := initScriptForTarget(configMappings, configNetMap.DefaultTarget, configNetMap.Params)
		if err != nil {
			return err
		}
		if script == nil {
			continue
		}
		netMap := mappings.NetworkMap{Network: ipnet, Script: script}
		env.NetworkMaps = append(env.NetworkMaps, netMap)
	}

	for _, configHostMap := range configMappings.HostnameMaps {
		regex, err := regexp.Compile(configHostMap.Hostname)
		if err != nil {
			return err
		}

		script, err := initScriptForTarget(configMappings, configHostMap.DefaultTarget, configHostMap.Params)
		if err != nil {
			return err
		}
		if script == nil {
			continue
		}
		hostMap := mappings.HostnameMap{Hostname: regex, Script: script}
		env.HostnameMaps = append(env.HostnameMaps, hostMap)
	}

	return nil
}

// initScriptForTarget adapts default targets for legacy UI mapping display.
// Runtime boot selection uses MappingResolver instead of these Script objects.
func initScriptForTarget(configMappings *mappings.Mappings, targetName string, mappingParams map[string]any) (*mappings.Script, error) {
	if targetName == "" {
		return nil, nil
	}

	target, ok := configMappings.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("default target %q not found", targetName)
	}

	mappingScript := &mappings.Script{
		Name:        target.Script,
		Environment: target.Environment,
		Params:      make(map[string]interface{}),
	}
	for key, value := range configMappings.Defaults.Params {
		mappingScript.Params[key] = fmt.Sprint(value)
	}
	for key, value := range target.Params {
		mappingScript.Params[key] = fmt.Sprint(value)
	}
	for key, value := range mappingParams {
		mappingScript.Params[key] = fmt.Sprint(value)
	}

	return mappingScript, nil
}
