package environment

// Options contains startup configuration for a Shoelaces environment.
type Options struct {
	BindAddr          string
	BaseURL           string
	DataDir           string
	StaticDir         string
	EnvDir            string
	TemplateExtension string
	MappingsFile      string
	Debug             bool
	TFTP              *TFTPConfig
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
