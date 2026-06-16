package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type shoelacesConfigParser struct {
	// path is used only to preserve useful file/line context in parse errors.
	path string
}

// Unmarshal adapts the historical Shoelaces config format to Koanf's parser
// interface while preserving flat flag names as the final config keys.
func (p shoelacesConfigParser) Unmarshal(data []byte) (map[string]any, error) {
	values := map[string]any{}
	section := ""

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section != "tftp" {
				return nil, fmt.Errorf("%s:%d: unsupported config section %q", p.path, lineNumber, section)
			}
			continue
		}

		name, value := parseConfigLine(line)
		if section != "" {
			name = section + "." + name
		}
		name, value, err := normalizeConfigValue(name, value)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", p.path, lineNumber, err)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func (shoelacesConfigParser) Marshal(map[string]any) ([]byte, error) {
	return nil, errors.New("shoelaces config marshal is not supported")
}

func parseConfigLine(line string) (string, string) {
	if name, value, ok := strings.Cut(line, "="); ok {
		return strings.TrimSpace(name), trimConfigValue(value)
	}
	if index := strings.IndexAny(line, " \t"); index >= 0 {
		return strings.TrimSpace(line[:index]), trimConfigValue(line[index+1:])
	}
	return strings.TrimSpace(line), "true"
}

func trimConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			return value[1 : len(value)-1]
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func normalizeConfigValue(name, value string) (string, string, error) {
	switch name {
	case "bind-addr", "base-url", "data-dir", "static-dir", "env-dir", "template-extension", "mappings-file", "debug",
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
	case "tftp.timeout_seconds":
		// Preserve the checked-in legacy [tftp] config shape while feeding the
		// duration parser the unit-suffixed value it expects.
		return "tftp-timeout", value + "s", nil
	default:
		return "", "", fmt.Errorf("configuration variable provided but not defined: %s", name)
	}
}
