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

package mappings

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/inngest/shoelaces/log"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Mappings contains the runtime provisioning target policy loaded from
// mappings.yaml.
type Mappings struct {
	// Defaults contains params that are merged into every selected target.
	Defaults DefaultsMap `koanf:"defaults"`
	// Targets contains all named boot targets that mappings may reference.
	Targets map[string]Target `koanf:"targets"`
	// NetworkMaps selects targets by matching the polling host IP against CIDRs.
	NetworkMaps []NetworkMapConfig `koanf:"networkMaps"`
	// HostnameMaps selects targets by matching the resolved/request hostname.
	HostnameMaps []HostnameMapConfig `koanf:"hostnameMaps"`
	// MacMaps selects targets by exact MAC address.
	MacMaps []MacMapConfig `koanf:"macMaps"`
	// IPMaps selects targets by exact IP address.
	IPMaps []IPMapConfig `koanf:"ipMaps"`
}

// DefaultsMap holds global parameter defaults for boot target rendering.
type DefaultsMap struct {
	// Params is the global parameter map merged before target and mapping params.
	Params map[string]any `koanf:"params"`
	// Users contains keyed account defaults merged before target and mapping users.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains global locale defaults.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains global clock/time defaults.
	Time TimeConfig `koanf:"time"`
	// Network contains global network defaults.
	Network NetworkConfig `koanf:"network"`
	// Packages contains global package defaults.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains global storage defaults.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains global boot defaults.
	Boot BootConfig `koanf:"boot"`
	// Repos contains global repository defaults.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains global installer defaults.
	Installer InstallerConfig `koanf:"installer"`
}

// Target describes a named boot target.
type Target struct {
	// Script is the dynamic iPXE template name to render for this target.
	Script string `koanf:"script"`
	// Label is optional display text for UI/manual target selection.
	Label string `koanf:"label"`
	// Environment selects a Shoelaces env override when rendering templates.
	Environment string `koanf:"environment"`
	// Params is the target-specific parameter map merged after defaults.
	Params map[string]any `koanf:"params"`
	// Users contains target-specific account settings merged after defaults.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains target locale settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains target clock/time settings.
	Time TimeConfig `koanf:"time"`
	// Network contains target network settings.
	Network NetworkConfig `koanf:"network"`
	// Packages contains target package settings.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains target storage settings.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains target boot settings.
	Boot BootConfig `koanf:"boot"`
	// Repos contains target repository settings.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains target installer settings.
	Installer InstallerConfig `koanf:"installer"`
}

// NetworkMapConfig maps a CIDR network to one or more targets.
type NetworkMapConfig struct {
	// Network is the IPv4 or IPv6 CIDR matched against the polling host IP.
	Network string `koanf:"network"`
	// DefaultTarget is the target selected automatically for matching hosts.
	// If empty, matching hosts will be queued for manual target selection once
	// Phase 2 runtime integration is complete.
	DefaultTarget string `koanf:"defaultTarget"`
	// Targets lists the named targets allowed for hosts matching this network.
	Targets []string `koanf:"targets"`
	// Params is the mapping-specific parameter map merged after target params.
	Params map[string]any `koanf:"params"`
	// Users contains mapping-specific account settings merged after target users.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains mapping-specific locale settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains mapping-specific clock/time settings.
	Time TimeConfig `koanf:"time"`
	// NetworkSettings contains mapping-specific network settings. It is named
	// differently from Network to avoid colliding with the CIDR selector field.
	NetworkSettings NetworkConfig `koanf:"networkConfig"`
	// Packages contains mapping-specific package settings.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains mapping-specific storage settings.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains mapping-specific boot settings.
	Boot BootConfig `koanf:"boot"`
	// Repos contains mapping-specific repository settings.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains mapping-specific installer settings.
	Installer InstallerConfig `koanf:"installer"`
}

// HostnameMapConfig maps a hostname regular expression to one or more targets.
type HostnameMapConfig struct {
	// Hostname is a Go regular expression matched against the host name.
	Hostname string `koanf:"hostname"`
	// DefaultTarget is the target selected automatically for matching hosts.
	DefaultTarget string `koanf:"defaultTarget"`
	// Targets lists the named targets allowed for hosts matching this hostname.
	Targets []string `koanf:"targets"`
	// Params is the mapping-specific parameter map merged after target params.
	Params map[string]any `koanf:"params"`
	// Users contains mapping-specific account settings merged after target users.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains mapping-specific locale settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains mapping-specific clock/time settings.
	Time TimeConfig `koanf:"time"`
	// Network contains mapping-specific network settings.
	Network NetworkConfig `koanf:"network"`
	// Packages contains mapping-specific package settings.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains mapping-specific storage settings.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains mapping-specific boot settings.
	Boot BootConfig `koanf:"boot"`
	// Repos contains mapping-specific repository settings.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains mapping-specific installer settings.
	Installer InstallerConfig `koanf:"installer"`
}

// MacMapConfig maps an exact MAC address to one or more targets.
type MacMapConfig struct {
	// Mac is the exact hardware address matched against the polling host MAC.
	Mac string `koanf:"mac"`
	// DefaultTarget is the target selected automatically for matching hosts.
	DefaultTarget string `koanf:"defaultTarget"`
	// Targets lists the named targets allowed for hosts matching this MAC.
	Targets []string `koanf:"targets"`
	// Params is the mapping-specific parameter map merged after target params.
	Params map[string]any `koanf:"params"`
	// Users contains mapping-specific account settings merged after target users.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains mapping-specific locale settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains mapping-specific clock/time settings.
	Time TimeConfig `koanf:"time"`
	// Network contains mapping-specific network settings.
	Network NetworkConfig `koanf:"network"`
	// Packages contains mapping-specific package settings.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains mapping-specific storage settings.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains mapping-specific boot settings.
	Boot BootConfig `koanf:"boot"`
	// Repos contains mapping-specific repository settings.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains mapping-specific installer settings.
	Installer InstallerConfig `koanf:"installer"`
}

// IPMapConfig maps an exact IP address to one or more targets.
type IPMapConfig struct {
	// IP is the exact IPv4 or IPv6 address matched against the polling host IP.
	IP string `koanf:"ip"`
	// DefaultTarget is the target selected automatically for matching hosts.
	DefaultTarget string `koanf:"defaultTarget"`
	// Targets lists the named targets allowed for hosts matching this IP.
	Targets []string `koanf:"targets"`
	// Params is the mapping-specific parameter map merged after target params.
	Params map[string]any `koanf:"params"`
	// Users contains mapping-specific account settings merged after target users.
	Users map[string]UserConfig `koanf:"users"`
	// Locale contains mapping-specific locale settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time contains mapping-specific clock/time settings.
	Time TimeConfig `koanf:"time"`
	// Network contains mapping-specific network settings.
	Network NetworkConfig `koanf:"network"`
	// Packages contains mapping-specific package settings.
	Packages PackagesConfig `koanf:"packages"`
	// Storage contains mapping-specific storage settings.
	Storage StorageConfig `koanf:"storage"`
	// Boot contains mapping-specific boot settings.
	Boot BootConfig `koanf:"boot"`
	// Repos contains mapping-specific repository settings.
	Repos ReposConfig `koanf:"repos"`
	// Installer contains mapping-specific installer settings.
	Installer InstallerConfig `koanf:"installer"`
}

var knownTopLevelMappingKeys = map[string]struct{}{
	"defaults":     {},
	"targets":      {},
	"networkMaps":  {},
	"hostnameMaps": {},
	"macMaps":      {},
	"ipMaps":       {},
}

// ParseMappings parses the mappings YAML file into the new mappings schema.
func ParseMappings(logger log.Logger, mappingsFile string) (*Mappings, error) {
	logger.Info("component", "config", "msg", "Reading mappings", "source", mappingsFile)

	k := koanf.New(".")
	if err := k.Load(file.Provider(mappingsFile), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("read mappings %q: %w", mappingsFile, err)
	}
	if err := validateTopLevelKeys(k.Keys()); err != nil {
		return nil, err
	}

	mappings := defaultMappings()
	if err := k.Unmarshal("", mappings); err != nil {
		return nil, fmt.Errorf("decode mappings %q: %w", mappingsFile, err)
	}
	if err := validateMappings(mappings); err != nil {
		return nil, err
	}

	return mappings, nil
}

func defaultMappings() *Mappings {
	return &Mappings{
		Targets:      make(map[string]Target),
		NetworkMaps:  make([]NetworkMapConfig, 0),
		HostnameMaps: make([]HostnameMapConfig, 0),
		MacMaps:      make([]MacMapConfig, 0),
		IPMaps:       make([]IPMapConfig, 0),
	}
}

func validateTopLevelKeys(keys []string) error {
	for _, key := range keys {
		topLevel, _, _ := strings.Cut(key, ".")
		if _, ok := knownTopLevelMappingKeys[topLevel]; !ok {
			return fmt.Errorf("unknown top-level mappings key %q", topLevel)
		}
	}
	return nil
}

func validateMappings(mappings *Mappings) error {
	if err := validateProvisioningConfig("defaults", mappings.Defaults.provisioningConfig()); err != nil {
		return err
	}
	for name, target := range mappings.Targets {
		if name == "" {
			return fmt.Errorf("target name must not be empty")
		}
		if target.Script == "" {
			return fmt.Errorf("target %q script must not be empty", name)
		}
		if err := validateProvisioningConfig(fmt.Sprintf("targets[%q]", name), target.provisioningConfig()); err != nil {
			return err
		}
	}

	for i, m := range mappings.NetworkMaps {
		if _, _, err := net.ParseCIDR(m.Network); err != nil {
			return fmt.Errorf("networkMaps[%d] network %q is invalid: %w", i, m.Network, err)
		}
		if err := validateTargetPolicy("networkMaps", i, m.Targets, m.DefaultTarget, mappings.Targets); err != nil {
			return err
		}
		if err := validateProvisioningConfig(fmt.Sprintf("networkMaps[%d]", i), m.provisioningConfig()); err != nil {
			return err
		}
	}

	for i, m := range mappings.HostnameMaps {
		if _, err := regexp.Compile(m.Hostname); err != nil {
			return fmt.Errorf("hostnameMaps[%d] hostname %q is invalid: %w", i, m.Hostname, err)
		}
		if err := validateTargetPolicy("hostnameMaps", i, m.Targets, m.DefaultTarget, mappings.Targets); err != nil {
			return err
		}
		if err := validateProvisioningConfig(fmt.Sprintf("hostnameMaps[%d]", i), m.provisioningConfig()); err != nil {
			return err
		}
	}

	for i, m := range mappings.MacMaps {
		if _, err := net.ParseMAC(m.Mac); err != nil {
			return fmt.Errorf("macMaps[%d] mac %q is invalid: %w", i, m.Mac, err)
		}
		if err := validateTargetPolicy("macMaps", i, m.Targets, m.DefaultTarget, mappings.Targets); err != nil {
			return err
		}
		if err := validateProvisioningConfig(fmt.Sprintf("macMaps[%d]", i), m.provisioningConfig()); err != nil {
			return err
		}
	}

	for i, m := range mappings.IPMaps {
		if ip := net.ParseIP(m.IP); ip == nil {
			return fmt.Errorf("ipMaps[%d] ip %q is invalid", i, m.IP)
		}
		if err := validateTargetPolicy("ipMaps", i, m.Targets, m.DefaultTarget, mappings.Targets); err != nil {
			return err
		}
		if err := validateProvisioningConfig(fmt.Sprintf("ipMaps[%d]", i), m.provisioningConfig()); err != nil {
			return err
		}
	}

	return nil
}

func validateProvisioningConfig(path string, config ProvisioningConfig) error {
	if err := validateOneOf(path+".network.bootproto", config.Network.Bootproto, "", "dhcp", "static"); err != nil {
		return err
	}
	if err := validateStringList(path+".network.nameservers", config.Network.Nameservers); err != nil {
		return err
	}
	if err := validateStringList(path+".packages.install", config.Packages.Install); err != nil {
		return err
	}
	if err := validateStringList(path+".packages.groups", config.Packages.Groups); err != nil {
		return err
	}
	if err := validateOneOf(path+".storage.mode", config.Storage.Mode, "", "plain", "lvm", "raid", "manual"); err != nil {
		return err
	}
	if err := validateFilesystems(path+".storage.filesystems", config.Storage.Filesystems); err != nil {
		return err
	}
	if err := validateOneOf(path+".boot.firmware", config.Boot.Firmware, "", "uefi", "bios"); err != nil {
		return err
	}
	if err := validateOneOf(path+".boot.netboot.method", config.Boot.Netboot.Method, "", "ipxe"); err != nil {
		return err
	}
	if err := validateStringList(path+".boot.netboot.kernelArgs", config.Boot.Netboot.KernelArgs); err != nil {
		return err
	}
	if err := validateOneOf(path+".boot.installed.bootloader", config.Boot.Installed.Bootloader, "", "grub"); err != nil {
		return err
	}
	if config.Boot.Installed.TimeoutSeconds != nil && *config.Boot.Installed.TimeoutSeconds < 0 {
		return fmt.Errorf("%s.boot.installed.timeoutSeconds must be greater than or equal to 0", path)
	}
	if err := validateStringList(path+".boot.installed.kernelArgs", config.Boot.Installed.KernelArgs); err != nil {
		return err
	}
	if config.Repos.OSMirror != "" {
		parsed, err := url.Parse(config.Repos.OSMirror)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("%s.repos.osMirror must be an http or https URL", path)
		}
	}
	return nil
}

func validateFilesystems(path string, filesystems map[string]FilesystemConfig) error {
	for name, filesystem := range filesystems {
		if name == "" {
			return fmt.Errorf("%s contains an empty filesystem name", path)
		}
		if boolValue(filesystem.Absent) {
			continue
		}
		if filesystem.SizeMiB != nil && *filesystem.SizeMiB < 0 {
			return fmt.Errorf("%s[%q].sizeMiB must be greater than or equal to 0", path, name)
		}
	}
	return nil
}

func validateStringList(path string, values []string) error {
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] must not be empty", path, i)
		}
	}
	return nil
}

func validateOneOf(path string, value string, allowed ...string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("%s has unsupported value %q", path, value)
}

func validateTargetPolicy(section string, index int, targetNames []string, defaultTarget string, targets map[string]Target) error {
	if len(targetNames) == 0 {
		return fmt.Errorf("%s[%d] targets must not be empty", section, index)
	}
	for _, targetName := range targetNames {
		if _, ok := targets[targetName]; !ok {
			return fmt.Errorf("%s[%d] references unknown target %q", section, index, targetName)
		}
	}
	if defaultTarget == "" {
		return nil
	}
	if _, ok := targets[defaultTarget]; !ok {
		return fmt.Errorf("%s[%d] defaultTarget references unknown target %q", section, index, defaultTarget)
	}
	if !stringInSlice(defaultTarget, targetNames) {
		return fmt.Errorf("%s[%d] defaultTarget %q must be included in targets", section, index, defaultTarget)
	}
	return nil
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
