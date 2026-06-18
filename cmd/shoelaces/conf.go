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
	case "bind-addr", "base-url", "data-dir", "ui-dir", "static-dir", "env-dir", "template-extension", "mappings-file", "log-level", "log-handler",
		"tftp-enabled", "tftp-addr", "tftp-root", "tftp-readonly", "tftp-timeout",
		"persistence-backend", "persistence-path", "persistence-retention-events", "persistence-retention-boot-sessions":
		return name, value, nil
	case "network.bindAddr", "network.bindaddr", "network.bind-addr":
		return "bind-addr", value, nil
	case "network.baseURL", "network.baseurl", "network.base-url":
		return "base-url", value, nil
	case "data.dir":
		return "data-dir", value, nil
	case "ui.dir":
		return "ui-dir", value, nil
	case "static.dir":
		return "static-dir", value, nil
	case "env.dir":
		return "env-dir", value, nil
	case "template.extension":
		return "template-extension", value, nil
	case "mappings.file":
		return "mappings-file", value, nil
	case "log.level":
		return "log-level", value, nil
	case "log.handler":
		return "log-handler", value, nil
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
	case "persistence.backend":
		return "persistence-backend", value, nil
	case "persistence.path":
		return "persistence-path", value, nil
	case "persistence.retention.events":
		return "persistence-retention-events", value, nil
	case "persistence.retention.bootSessions", "persistence.retention.bootsessions", "persistence.retention.boot-sessions":
		return "persistence-retention-boot-sessions", value, nil
	default:
		return "", nil, fmt.Errorf("configuration variable provided but not defined: %s", name)
	}
}
