package environment

// Options contains startup configuration for a Shoelaces environment.
type Options struct {
	// BindAddr is the HTTP listen address for the Shoelaces web server.
	BindAddr string
	// BaseURL is used when rendered templates need to refer back to Shoelaces.
	BaseURL string
	// DataDir is the root for mappings, boot templates, static config assets, and environment overrides.
	DataDir string
	// StaticDir contains the web UI templates and static frontend assets.
	StaticDir string
	// EnvDir is the directory under DataDir that contains environment-specific overrides.
	EnvDir string
	// TemplateExtension identifies files that should be parsed as dynamic Shoelaces templates.
	TemplateExtension string
	// MappingsFile is resolved relative to DataDir and maps hosts or networks to boot scripts.
	MappingsFile string
	// Debug enables debug-level logging.
	Debug bool
	// TFTP controls the optional embedded TFTP server. A nil value means use defaults.
	TFTP *TFTPConfig
}

// DefaultOptions returns the runtime defaults for options not explicitly set.
func DefaultOptions() Options {
	tftp := DefaultTFTPConfig()
	return Options{
		BindAddr:          "localhost:8081",
		StaticDir:         "web",
		EnvDir:            "env_overrides",
		TemplateExtension: ".slc",
		MappingsFile:      "mappings.yaml",
		TFTP:              &tftp,
	}
}

func (env *Environment) applyOptions(options Options) {
	defaults := DefaultOptions()

	if options.BindAddr == "" {
		options.BindAddr = defaults.BindAddr
	}
	if options.StaticDir == "" {
		options.StaticDir = defaults.StaticDir
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

	env.BindAddr = options.BindAddr
	env.BaseURL = options.BaseURL
	env.DataDir = options.DataDir
	env.StaticDir = options.StaticDir
	env.EnvDir = options.EnvDir
	env.TemplateExtension = options.TemplateExtension
	env.MappingsFile = options.MappingsFile
	env.Debug = options.Debug
	env.TFTP = options.TFTP
}
