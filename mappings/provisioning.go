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

// ProvisioningConfig contains structured installer policy merged from
// defaults, the selected target, and the matched mapping rule.
type ProvisioningConfig struct {
	// Locale configures language and keyboard settings.
	Locale LocaleConfig `koanf:"locale"`
	// Time configures timezone, UTC clock, and NTP behavior.
	Time TimeConfig `koanf:"time"`
	// Network configures installer/runtime network defaults.
	Network NetworkConfig `koanf:"network"`
	// Packages configures package and package-group installation policy.
	Packages PackagesConfig `koanf:"packages"`
	// Storage configures disk layout policy.
	Storage StorageConfig `koanf:"storage"`
	// Boot separates netboot behavior from installed-system bootloader policy.
	Boot BootConfig `koanf:"boot"`
	// Repos configures OS mirror/release and repository components.
	Repos ReposConfig `koanf:"repos"`
	// Installer configures the installer config template and native extra snippet.
	Installer InstallerConfig `koanf:"installer"`
}

// LocaleConfig configures installer locale values.
type LocaleConfig struct {
	// Language is the locale string, for example en_US.UTF-8.
	Language string `koanf:"language"`
	// Keyboard is the installer keyboard layout, for example us.
	Keyboard string `koanf:"keyboard"`
}

// TimeConfig configures clock and timezone behavior.
type TimeConfig struct {
	// Timezone is the tz database timezone name, for example UTC.
	Timezone string `koanf:"timezone"`
	// UTC controls whether the hardware clock is treated as UTC.
	UTC *bool `koanf:"utc"`
	// NTP controls whether the installed system should use network time.
	NTP *bool `koanf:"ntp"`
}

// NetworkConfig configures common network settings.
type NetworkConfig struct {
	// Hostname overrides the generated hostname for installer config rendering.
	Hostname string `koanf:"hostname"`
	// Bootproto is the installer network mode, currently dhcp or static.
	Bootproto string `koanf:"bootproto"`
	// Nameservers contains DNS resolver addresses.
	Nameservers []string `koanf:"nameservers"`
}

// PackagesConfig configures package installation policy.
type PackagesConfig struct {
	// Install contains package names to install.
	Install []string `koanf:"install"`
	// Groups contains installer package group names.
	Groups []string `koanf:"groups"`
	// UpdatePolicy names an OS-specific update policy.
	UpdatePolicy string `koanf:"updatePolicy"`
}

// StorageConfig configures installer disk layout.
type StorageConfig struct {
	// Disk is the primary install disk path.
	Disk string `koanf:"disk"`
	// Wipe controls whether the installer should clear existing storage.
	Wipe *bool `koanf:"wipe"`
	// Mode selects the storage recipe family.
	Mode string `koanf:"mode"`
	// VolumeGroup is the LVM volume group name when Mode is lvm.
	VolumeGroup string `koanf:"volumeGroup"`
	// Filesystems contains named filesystem definitions.
	Filesystems map[string]FilesystemConfig `koanf:"filesystems"`
}

// FilesystemConfig describes one named filesystem entry.
type FilesystemConfig struct {
	// Absent suppresses an inherited filesystem entry.
	Absent *bool `koanf:"absent"`
	// Mountpoint is the filesystem mount path.
	Mountpoint string `koanf:"mountpoint"`
	// FSType is the filesystem type, for example ext4 or swap.
	FSType string `koanf:"fstype"`
	// Size is a symbolic size such as grow.
	Size string `koanf:"size"`
	// SizeMiB is the fixed size in MiB.
	SizeMiB *int `koanf:"sizeMiB"`
}

// BootConfig separates network boot and installed-system boot settings.
type BootConfig struct {
	// Firmware is the target firmware mode, such as uefi or bios.
	Firmware string `koanf:"firmware"`
	// Netboot configures the Shoelaces-served network boot path.
	Netboot NetbootConfig `koanf:"netboot"`
	// Installed configures the installed system bootloader.
	Installed InstalledBootConfig `koanf:"installed"`
}

// NetbootConfig configures the network boot path.
type NetbootConfig struct {
	// Method is the network boot method, currently ipxe.
	Method string `koanf:"method"`
	// KernelArgs contains kernel arguments used during network boot.
	KernelArgs []string `koanf:"kernelArgs"`
}

// InstalledBootConfig configures installed-system boot behavior.
type InstalledBootConfig struct {
	// Bootloader is the installed bootloader, currently grub.
	Bootloader string `koanf:"bootloader"`
	// TimeoutSeconds is the bootloader menu timeout.
	TimeoutSeconds *int `koanf:"timeoutSeconds"`
	// KernelArgs contains installed-system kernel arguments.
	KernelArgs []string `koanf:"kernelArgs"`
}

// ReposConfig configures OS repository settings.
type ReposConfig struct {
	// OSMirror is the base OS package mirror URL.
	OSMirror string `koanf:"osMirror"`
	// Release is the OS release/codename.
	Release string `koanf:"release"`
	// Firmware enables firmware repository support where the OS supports it.
	Firmware *bool `koanf:"firmware"`
	// Contrib enables contrib repository components where available.
	Contrib *bool `koanf:"contrib"`
	// NonFree enables non-free repository components where available.
	NonFree *bool `koanf:"nonFree"`
}

// InstallerConfig configures installer config rendering.
type InstallerConfig struct {
	// ConfigTemplate selects the installer config template to serve.
	ConfigTemplate string `koanf:"configTemplate"`
	// ConfigParams contains low-level query/config params for that template.
	ConfigParams map[string]any `koanf:"configParams"`
	// ExtraTemplate selects a native installer snippet template escape hatch.
	ExtraTemplate string `koanf:"extraTemplate"`
}

func (defaults DefaultsMap) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    defaults.Locale,
		Time:      defaults.Time,
		Network:   defaults.Network,
		Packages:  defaults.Packages,
		Storage:   defaults.Storage,
		Boot:      defaults.Boot,
		Repos:     defaults.Repos,
		Installer: defaults.Installer,
	}
}

func (target Target) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    target.Locale,
		Time:      target.Time,
		Network:   target.Network,
		Packages:  target.Packages,
		Storage:   target.Storage,
		Boot:      target.Boot,
		Repos:     target.Repos,
		Installer: target.Installer,
	}
}

func (mapping NetworkMapConfig) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    mapping.Locale,
		Time:      mapping.Time,
		Network:   mapping.NetworkSettings,
		Packages:  mapping.Packages,
		Storage:   mapping.Storage,
		Boot:      mapping.Boot,
		Repos:     mapping.Repos,
		Installer: mapping.Installer,
	}
}

func (mapping HostnameMapConfig) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    mapping.Locale,
		Time:      mapping.Time,
		Network:   mapping.Network,
		Packages:  mapping.Packages,
		Storage:   mapping.Storage,
		Boot:      mapping.Boot,
		Repos:     mapping.Repos,
		Installer: mapping.Installer,
	}
}

func (mapping MacMapConfig) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    mapping.Locale,
		Time:      mapping.Time,
		Network:   mapping.Network,
		Packages:  mapping.Packages,
		Storage:   mapping.Storage,
		Boot:      mapping.Boot,
		Repos:     mapping.Repos,
		Installer: mapping.Installer,
	}
}

func (mapping IPMapConfig) provisioningConfig() ProvisioningConfig {
	return ProvisioningConfig{
		Locale:    mapping.Locale,
		Time:      mapping.Time,
		Network:   mapping.Network,
		Packages:  mapping.Packages,
		Storage:   mapping.Storage,
		Boot:      mapping.Boot,
		Repos:     mapping.Repos,
		Installer: mapping.Installer,
	}
}

func mergeProvisioningConfig(base ProvisioningConfig, override ProvisioningConfig) ProvisioningConfig {
	base.Locale = mergeLocaleConfig(base.Locale, override.Locale)
	base.Time = mergeTimeConfig(base.Time, override.Time)
	base.Network = mergeNetworkConfig(base.Network, override.Network)
	base.Packages = mergePackagesConfig(base.Packages, override.Packages)
	base.Storage = mergeStorageConfig(base.Storage, override.Storage)
	base.Boot = mergeBootConfig(base.Boot, override.Boot)
	base.Repos = mergeReposConfig(base.Repos, override.Repos)
	base.Installer = mergeInstallerConfig(base.Installer, override.Installer)
	return base
}

func mergeLocaleConfig(base LocaleConfig, override LocaleConfig) LocaleConfig {
	if override.Language != "" {
		base.Language = override.Language
	}
	if override.Keyboard != "" {
		base.Keyboard = override.Keyboard
	}
	return base
}

func mergeTimeConfig(base TimeConfig, override TimeConfig) TimeConfig {
	if override.Timezone != "" {
		base.Timezone = override.Timezone
	}
	if override.UTC != nil {
		base.UTC = copyBoolPtr(override.UTC)
	}
	if override.NTP != nil {
		base.NTP = copyBoolPtr(override.NTP)
	}
	return base
}

func mergeNetworkConfig(base NetworkConfig, override NetworkConfig) NetworkConfig {
	if override.Hostname != "" {
		base.Hostname = override.Hostname
	}
	if override.Bootproto != "" {
		base.Bootproto = override.Bootproto
	}
	if override.Nameservers != nil {
		base.Nameservers = append([]string(nil), override.Nameservers...)
	}
	return base
}

func mergePackagesConfig(base PackagesConfig, override PackagesConfig) PackagesConfig {
	if override.Install != nil {
		base.Install = append([]string(nil), override.Install...)
	}
	if override.Groups != nil {
		base.Groups = append([]string(nil), override.Groups...)
	}
	if override.UpdatePolicy != "" {
		base.UpdatePolicy = override.UpdatePolicy
	}
	return base
}

func mergeStorageConfig(base StorageConfig, override StorageConfig) StorageConfig {
	if override.Disk != "" {
		base.Disk = override.Disk
	}
	if override.Wipe != nil {
		base.Wipe = copyBoolPtr(override.Wipe)
	}
	if override.Mode != "" {
		base.Mode = override.Mode
	}
	if override.VolumeGroup != "" {
		base.VolumeGroup = override.VolumeGroup
	}
	if override.Filesystems != nil {
		base.Filesystems = mergeFilesystemConfigMap(base.Filesystems, override.Filesystems)
	}
	return base
}

func mergeFilesystemConfigMap(base map[string]FilesystemConfig, override map[string]FilesystemConfig) map[string]FilesystemConfig {
	merged := copyFilesystemConfigMap(base)
	if merged == nil {
		merged = make(map[string]FilesystemConfig, len(override))
	}
	for name, filesystem := range override {
		if boolValue(filesystem.Absent) {
			delete(merged, name)
			continue
		}
		merged[name] = mergeFilesystemConfig(merged[name], filesystem)
	}
	return merged
}

func mergeFilesystemConfig(base FilesystemConfig, override FilesystemConfig) FilesystemConfig {
	if override.Absent != nil {
		base.Absent = copyBoolPtr(override.Absent)
	}
	if override.Mountpoint != "" {
		base.Mountpoint = override.Mountpoint
	}
	if override.FSType != "" {
		base.FSType = override.FSType
	}
	if override.Size != "" {
		base.Size = override.Size
	}
	if override.SizeMiB != nil {
		copied := *override.SizeMiB
		base.SizeMiB = &copied
	}
	return base
}

func mergeBootConfig(base BootConfig, override BootConfig) BootConfig {
	if override.Firmware != "" {
		base.Firmware = override.Firmware
	}
	base.Netboot = mergeNetbootConfig(base.Netboot, override.Netboot)
	base.Installed = mergeInstalledBootConfig(base.Installed, override.Installed)
	return base
}

func mergeNetbootConfig(base NetbootConfig, override NetbootConfig) NetbootConfig {
	if override.Method != "" {
		base.Method = override.Method
	}
	if override.KernelArgs != nil {
		base.KernelArgs = append([]string(nil), override.KernelArgs...)
	}
	return base
}

func mergeInstalledBootConfig(base InstalledBootConfig, override InstalledBootConfig) InstalledBootConfig {
	if override.Bootloader != "" {
		base.Bootloader = override.Bootloader
	}
	if override.TimeoutSeconds != nil {
		copied := *override.TimeoutSeconds
		base.TimeoutSeconds = &copied
	}
	if override.KernelArgs != nil {
		base.KernelArgs = append([]string(nil), override.KernelArgs...)
	}
	return base
}

func mergeReposConfig(base ReposConfig, override ReposConfig) ReposConfig {
	if override.OSMirror != "" {
		base.OSMirror = override.OSMirror
	}
	if override.Release != "" {
		base.Release = override.Release
	}
	if override.Firmware != nil {
		base.Firmware = copyBoolPtr(override.Firmware)
	}
	if override.Contrib != nil {
		base.Contrib = copyBoolPtr(override.Contrib)
	}
	if override.NonFree != nil {
		base.NonFree = copyBoolPtr(override.NonFree)
	}
	return base
}

func mergeInstallerConfig(base InstallerConfig, override InstallerConfig) InstallerConfig {
	if override.ConfigTemplate != "" {
		base.ConfigTemplate = override.ConfigTemplate
	}
	if override.ConfigParams != nil {
		base.ConfigParams = copyParamMap(base.ConfigParams)
		if base.ConfigParams == nil {
			base.ConfigParams = make(map[string]any, len(override.ConfigParams))
		}
		mergeParamMap(base.ConfigParams, override.ConfigParams)
	}
	if override.ExtraTemplate != "" {
		base.ExtraTemplate = override.ExtraTemplate
	}
	return base
}

func copyProvisioningConfig(config ProvisioningConfig) ProvisioningConfig {
	config.Time.UTC = copyBoolPtr(config.Time.UTC)
	config.Time.NTP = copyBoolPtr(config.Time.NTP)
	config.Network.Nameservers = append([]string(nil), config.Network.Nameservers...)
	config.Packages.Install = append([]string(nil), config.Packages.Install...)
	config.Packages.Groups = append([]string(nil), config.Packages.Groups...)
	config.Storage.Wipe = copyBoolPtr(config.Storage.Wipe)
	config.Storage.Filesystems = copyFilesystemConfigMap(config.Storage.Filesystems)
	config.Boot.Netboot.KernelArgs = append([]string(nil), config.Boot.Netboot.KernelArgs...)
	if config.Boot.Installed.TimeoutSeconds != nil {
		copied := *config.Boot.Installed.TimeoutSeconds
		config.Boot.Installed.TimeoutSeconds = &copied
	}
	config.Boot.Installed.KernelArgs = append([]string(nil), config.Boot.Installed.KernelArgs...)
	config.Repos.Firmware = copyBoolPtr(config.Repos.Firmware)
	config.Repos.Contrib = copyBoolPtr(config.Repos.Contrib)
	config.Repos.NonFree = copyBoolPtr(config.Repos.NonFree)
	config.Installer.ConfigParams = copyParamMap(config.Installer.ConfigParams)
	return config
}

func copyFilesystemConfigMap(filesystems map[string]FilesystemConfig) map[string]FilesystemConfig {
	if filesystems == nil {
		return nil
	}
	copied := make(map[string]FilesystemConfig, len(filesystems))
	for name, filesystem := range filesystems {
		copied[name] = copyFilesystemConfig(filesystem)
	}
	return copied
}

func copyFilesystemConfig(filesystem FilesystemConfig) FilesystemConfig {
	filesystem.Absent = copyBoolPtr(filesystem.Absent)
	if filesystem.SizeMiB != nil {
		copied := *filesystem.SizeMiB
		filesystem.SizeMiB = &copied
	}
	return filesystem
}
