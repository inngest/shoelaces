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
	"context"
	"fmt"
	"html/template"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/bootsession"
	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/persistence/memory"
	persistencesqlite "github.com/inngest/shoelaces/persistence/sqlite"
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
	ServerStates    server.StateStore
	EventLog        *event.Log
	Polling         *polling.Service
	ParamsBlacklist []string
	Templates       *templates.ShoelacesTemplates // Dynamic slc templates
	StaticTemplates *template.Template            // Static Templates
	Environments    []string                      // Valid config environments
	Logger          log.Logger
	RuntimeStore    persistence.Store
	BootSessions    *bootsession.Store

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
	PersistenceConfig persistence.Config
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
	env.RuntimeStore = env.initPersistence()
	env.EventLog = event.NewLog(env.RuntimeStore, env.RuntimeStore)
	env.ServerStates = server.NewPersistentStateStore(env.RuntimeStore, env.RuntimeStore)
	env.BootSessions = bootsession.NewStore(env.RuntimeStore, env.RuntimeStore, env.PersistenceConfig.Retention.BootSessions)
	env.cleanupEventRetention()
	env.cleanupBootSessionRetention()

	env.logStartupConfig()
	env.Logger.Info("Discovered environment overrides", "component", "environment", "count", len(env.Environments), "environments", env.Environments)

	mappingsPath := path.Join(env.DataDir, env.MappingsFile)
	env.Logger.Info("Loading mappings", "component", "environment", "source", mappingsPath)
	if err := env.initMappings(mappingsPath); err != nil {
		panic(err)
	}

	env.initStaticTemplates()
	env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
	env.Polling = polling.NewService(env.Logger, env.ServerStates, env.MappingResolver, env.EventLog, env.Templates, env.BaseURL).WithBootSessions(env.BootSessions)
	server.StartStateStoreCleaner(env.Logger, env.ServerStates)
	env.startRetentionCleaners()

	return env
}

func (env *Environment) logStartupConfig() {
	env.Logger.Info("Initialized environment", "component", "environment", "bind_addr", env.BindAddr, "base_url", env.BaseURL, "data_dir", env.DataDir, "env_dir", env.EnvDir, "template_extension", env.TemplateExtension, "ui_source", env.uiSource(), "log_level", env.LogLevel, "log_handler", env.LogHandler, "persistence_backend", env.PersistenceConfig.Backend, "persistence_path", env.persistencePathForLog(), "events_retention", env.PersistenceConfig.Retention.Events, "events_sweep_interval", env.PersistenceConfig.Retention.EventsSweepInterval, "boot_sessions_retention", env.PersistenceConfig.Retention.BootSessions, "boot_sessions_sweep_interval", env.PersistenceConfig.Retention.BootSessionsSweepInterval)
	if env.TFTP == nil {
		env.Logger.Info("Configured TFTP", "component", "environment", "enabled", false)
		return
	}
	env.Logger.Info("Configured TFTP", "component", "environment", "enabled", env.TFTP.Enabled, "addr", env.TFTP.Addr, "root", env.TFTP.Root, "readonly", env.TFTP.Readonly, "timeout", env.TFTP.Timeout)
}

func (env *Environment) persistencePathForLog() string {
	if env.PersistenceConfig.Backend != persistence.BackendSQLite {
		return ""
	}
	return persistence.ResolvePath(env.DataDir, env.PersistenceConfig)
}

func (env *Environment) uiSource() string {
	if env.UsesUIOverride() {
		return env.UIDir
	}
	return "embedded"
}

func defaultEnvironment() *Environment {
	env := &Environment{}
	env.NetworkMaps = make([]mappings.NetworkMap, 0)
	env.HostnameMaps = make([]mappings.HostnameMap, 0)
	env.ParamsBlacklist = []string{"baseURL"}
	env.Environments = make([]string, 0)
	env.Logger = log.MakeLogger(os.Stdout)
	env.Templates = templates.New(env.Logger)
	env.PersistenceConfig = persistence.DefaultConfig()
	env.RuntimeStore = memory.New()
	env.EventLog = event.NewLog(env.RuntimeStore, env.RuntimeStore)
	env.ServerStates = server.NewPersistentStateStore(env.RuntimeStore, env.RuntimeStore)
	env.BootSessions = bootsession.NewStore(env.RuntimeStore, env.RuntimeStore, env.PersistenceConfig.Retention.BootSessions)
	env.Polling = polling.NewService(env.Logger, env.ServerStates, env.MappingResolver, env.EventLog, env.Templates, env.BaseURL).WithBootSessions(env.BootSessions)

	return env
}

func (env *Environment) initPersistence() persistence.Store {
	if err := persistence.Validate(env.PersistenceConfig); err != nil {
		panic(err)
	}

	switch env.PersistenceConfig.Backend {
	case persistence.BackendMemory:
		return memory.New()
	case persistence.BackendSQLite:
		store, err := persistencesqlite.Open(context.Background(), persistence.ResolvePath(env.DataDir, env.PersistenceConfig))
		if err != nil {
			panic(err)
		}
		return store
	default:
		panic(fmt.Errorf("unsupported persistence backend %q", env.PersistenceConfig.Backend))
	}
}

func (env *Environment) cleanupEventRetention() {
	retention := env.PersistenceConfig.Retention.Events
	if retention <= 0 || env.EventLog == nil {
		return
	}
	cutoff := time.Now().Add(-retention)
	deleted, err := env.EventLog.DeleteEventsBefore(context.Background(), cutoff)
	if err != nil {
		env.Logger.Error("Failed to clean up old events", "component", "environment", "err", err)
		return
	}
	if deleted > 0 {
		env.Logger.Info("Cleaned up old events", "component", "environment", "deleted", deleted, "retention", retention.String())
	}
}

func (env *Environment) cleanupBootSessionRetention() {
	retention := env.PersistenceConfig.Retention.BootSessions
	if retention <= 0 || env.BootSessions == nil {
		return
	}
	cutoff := time.Now()
	deleted, err := env.BootSessions.DeleteExpired(context.Background(), cutoff)
	if err != nil {
		env.Logger.Error("Failed to clean up old boot sessions", "component", "environment", "err", err)
		return
	}
	if deleted > 0 {
		env.Logger.Info("Cleaned up old boot sessions", "component", "environment", "deleted", deleted, "retention", retention.String())
	}
}

func (env *Environment) startRetentionCleaners() {
	retention := env.PersistenceConfig.Retention
	startRetentionCleaner(env.Logger, "events", retention.EventsSweepInterval, env.cleanupEventRetention)
	startRetentionCleaner(env.Logger, "boot_sessions", retention.BootSessionsSweepInterval, env.cleanupBootSessionRetention)
}

func startRetentionCleaner(logger log.Logger, name string, interval time.Duration, sweep func()) func() {
	if interval <= 0 || sweep == nil {
		return func() {}
	}
	if logger == nil {
		logger = log.MakeLogger(os.Stdout)
	}
	logger = logger.With("component", "retention", "records", name)
	logger.Debug("Starting retention cleaner", "interval", interval.String())

	stop := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sweep()
			case <-stop:
				return
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
		})
	}
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
		Params:      make(map[string]any),
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
