package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// parserForConfig selects a Koanf parser from the config filename extension.
func parserForConfig(path string) (koanf.Parser, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		return toml.Parser(), nil
	case ".yaml", ".yml":
		return yaml.Parser(), nil
	case ".json":
		return json.Parser(), nil
	default:
		return nil, fmt.Errorf("unsupported config file extension %q; use .toml, .yaml, .yml, or .json", filepath.Ext(path))
	}
}

// configValuesFromKoanf flattens structured config into the flag names used as
// urfave/cli value-source keys.
func configValuesFromKoanf(k *koanf.Koanf) (map[any]any, error) {
	values := map[any]any{}
	for key, value := range k.All() {
		name, value, err := normalizeConfigValue(key, value)
		if err != nil {
			return nil, err
		}
		values[name] = value
	}
	return values, nil
}

func normalizeConfigValue(name string, value any) (string, any, error) {
	switch name {
	case "bind-addr", "base-url", "data-dir", "ui-dir", "static-dir", "env-dir", "template-extension", "mappings-file", "debug", "log-level", "log-handler",
		"tftp-enabled", "tftp-addr", "tftp-root", "tftp-readonly", "tftp-timeout":
		return name, value, nil
	case "tftp.enabled":
		return "tftp-enabled", value, nil
	case "tftp.address", "tftp.addr":
		return "tftp-addr", value, nil
	case "tftp.root":
		return "tftp-root", value, nil
	case "tftp.readonly":
		return "tftp-readonly", value, nil
	case "tftp.timeout":
		return "tftp-timeout", value, nil
	default:
		return "", nil, fmt.Errorf("configuration variable provided but not defined: %s", name)
	}
}
