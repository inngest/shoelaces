package environment

import "github.com/inngest/shoelaces/persistence"

// Options contains startup configuration for a Shoelaces environment.
type Options struct {
	// BindAddr is the HTTP listen address for the Shoelaces web server.
	BindAddr string
	// BaseURL is used when rendered templates need to refer back to Shoelaces.
	BaseURL string
	// DataDir is the root for mappings, boot templates, static config assets, and environment overrides.
	DataDir string
	// UIDir optionally overrides the embedded web UI templates and static frontend assets.
	UIDir string
	// UIOverrideDirSet tracks whether UIDir came from CLI, environment, or config.
	UIOverrideDirSet bool
	// EnvDir is the directory under DataDir that contains environment-specific overrides.
	EnvDir string
	// TemplateExtension identifies files that should be parsed as dynamic Shoelaces templates.
	TemplateExtension string
	// MappingsFile is resolved relative to DataDir and maps hosts or networks to boot scripts.
	MappingsFile string
	// Debug enables debug-level logging.
	Debug bool
	// LogLevel controls the minimum log level when Debug is false.
	LogLevel string
	// LogHandler controls log output format: dev, text, or json.
	LogHandler string
	// TFTP controls the optional embedded TFTP server. A nil value means use defaults.
	TFTP *TFTPConfig
	// Persistence controls runtime data storage for events, host state, and references.
	Persistence persistence.Config
}

// DefaultOptions returns the runtime defaults for options not explicitly set.
func DefaultOptions() Options {
	tftp := DefaultTFTPConfig()
	return Options{
		BindAddr:          ":8081",
		EnvDir:            "env_overrides",
		TemplateExtension: ".slc",
		MappingsFile:      "mappings.yaml",
		TFTP:              &tftp,
		Persistence:       persistence.DefaultConfig(),
	}
}

func (env *Environment) applyOptions(options Options) {
	defaults := DefaultOptions()

	if options.BindAddr == "" {
		options.BindAddr = defaults.BindAddr
	}
	if options.EnvDir == "" {
		options.EnvDir = defaults.EnvDir
	}
	if options.TemplateExtension == "" {
		options.TemplateExtension = defaults.TemplateExtension
	}
	if options.MappingsFile == "" {
		options.MappingsFile = defaults.MappingsFile
	}
	if options.TFTP == nil {
		options.TFTP = defaults.TFTP
	}
	options.Persistence = persistence.ApplyDefaults(options.Persistence)

	env.BindAddr = options.BindAddr
	env.BaseURL = options.BaseURL
	env.DataDir = options.DataDir
	env.UIDir = options.UIDir
	env.UIOverrideDirSet = options.UIOverrideDirSet
	env.EnvDir = options.EnvDir
	env.TemplateExtension = options.TemplateExtension
	env.MappingsFile = options.MappingsFile
	env.Debug = options.Debug
	env.LogLevel = options.LogLevel
	env.LogHandler = options.LogHandler
	env.TFTP = options.TFTP
	env.PersistenceConfig = options.Persistence
}
