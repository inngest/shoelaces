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
	"strings"
)

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
	// WipeDiskPatterns contains explicit /dev glob patterns for disks that the
	// installer may wipe before partitioning.
	WipeDiskPatterns []string `koanf:"wipeDiskPatterns"`
	// Mode selects the storage recipe family.
	Mode string `koanf:"mode"`
	// VolumeGroup is the LVM volume group name when Mode is lvm.
	VolumeGroup string `koanf:"volumeGroup"`
	// RAID contains mdraid settings when Mode is raid.
	RAID RAIDConfig `koanf:"raid"`
	// Filesystems contains named filesystem definitions.
	Filesystems map[string]FilesystemConfig `koanf:"filesystems"`

	// absentFilesystems tracks inherited entries suppressed during merging.
	absentFilesystems map[string]bool
}

// RAIDConfig describes the narrow mdraid shape supported by installers.
type RAIDConfig struct {
	// Level is the mdraid level. Debian support initially accepts only RAID1.
	Level int `koanf:"level"`
	// Devices contains explicit member disk paths.
	Devices []string `koanf:"devices"`
	// BootDegraded controls whether the installed system may boot degraded.
	BootDegraded *bool `koanf:"bootDegraded"`
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

// ParamsWithProvisioning returns template parameters with structured users and
// provisioning data attached. It also projects structured fields into the flat
// params consumed by the embedded installer templates.
func ParamsWithProvisioning(params map[string]interface{}, users map[string]ResolvedUser, provisioning ProvisioningConfig) map[string]interface{} {
	copied := ParamsWithUsers(params, users)
	copied["provisioning"] = copyProvisioningConfig(provisioning)

	projectProvisioningParams(copied, users, provisioning)
	return copied
}

func projectProvisioningParams(params map[string]interface{}, users map[string]ResolvedUser, provisioning ProvisioningConfig) {
	_, explicitDebianRegularPartmanModules := params["debian_regular_partman_modules"]
	_, explicitDebianLVMPartmanModules := params["debian_lvm_partman_modules"]
	_, explicitDebianRAIDPartmanModules := params["debian_raid_partman_modules"]
	if _, ok := provisioning.Installer.ConfigParams["debian_regular_partman_modules"]; ok {
		explicitDebianRegularPartmanModules = true
	}
	if _, ok := provisioning.Installer.ConfigParams["debian_lvm_partman_modules"]; ok {
		explicitDebianLVMPartmanModules = true
	}
	if _, ok := provisioning.Installer.ConfigParams["debian_raid_partman_modules"]; ok {
		explicitDebianRAIDPartmanModules = true
	}

	// Preserve explicit caller/query parameters first, project structured
	// provisioning fields second, then fill remaining keys with built-in
	// defaults. This lets /configs/* query params override mapping policy while
	// keeping templates free of repeated fallback conditionals.
	setDefaultParam(params, "release", provisioning.Repos.Release)
	setDefaultParam(params, "linuxargs", strings.Join(provisioning.Boot.Netboot.KernelArgs, " "))
	setDefaultParam(params, "vg_name", provisioning.Storage.VolumeGroup)
	setDefaultParam(params, "hostname", provisioning.Network.Hostname)
	setDefaultParam(params, "locale_language", provisioning.Locale.Language)
	setDefaultParam(params, "locale_keyboard", provisioning.Locale.Keyboard)
	setDefaultParam(params, "time_timezone", provisioning.Time.Timezone)
	setDefaultParam(params, "network_bootproto", provisioning.Network.Bootproto)
	setDefaultParam(params, "nameservers", strings.Join(provisioning.Network.Nameservers, " "))
	setDefaultParam(params, "packages_install", strings.Join(provisioning.Packages.Install, " "))
	setDefaultParam(params, "packages_groups", strings.Join(provisioning.Packages.Groups, " "))
	setDefaultParam(params, "packages_update_policy", provisioning.Packages.UpdatePolicy)
	setDefaultParam(params, "storage_disk", provisioning.Storage.Disk)
	setDefaultParam(params, "storage_mode", provisioning.Storage.Mode)
	projectRAIDParams(params, provisioning.Storage.RAID)
	projectDebianRegularFilesystemParams(params, provisioning.Storage)
	projectDebianLVMFilesystemParams(params, provisioning.Storage)
	projectDebianRAIDFilesystemParams(params, provisioning.Storage)
	setDefaultParam(params, "repo_debian_mirror", provisioning.Repos.OSMirror)
	setDefaultParam(params, "repo_ubuntu_mirror", provisioning.Repos.OSMirror)
	setDefaultParam(params, "repo_centos_mirror", provisioning.Repos.OSMirror)
	setDefaultParam(params, "repo_coreos_mirror", provisioning.Repos.OSMirror)
	setDefaultParam(params, "boot_installed_args", strings.Join(provisioning.Boot.Installed.KernelArgs, " "))
	setDefaultParam(params, "debian_installer_config_template", provisioning.Installer.ConfigTemplate)
	setDefaultParam(params, "storage_installer_config_template", provisioning.Installer.ConfigTemplate)
	setDefaultParam(params, "ubuntu_minimal_installer_config_template", provisioning.Installer.ConfigTemplate)
	setDefaultParam(params, "centos_installer_config_template", provisioning.Installer.ConfigTemplate)
	setDefaultParam(params, "coreos_installer_config_template", provisioning.Installer.ConfigTemplate)
	if provisioning.Time.UTC != nil {
		setDefaultParamValue(params, "time_utc", *provisioning.Time.UTC)
		if *provisioning.Time.UTC {
			setDefaultParamValue(params, "kickstart_utc_flag", "--utc")
		} else {
			setDefaultParamValue(params, "kickstart_utc_flag", "")
		}
	}
	if provisioning.Time.NTP != nil {
		setDefaultParamValue(params, "time_ntp", *provisioning.Time.NTP)
	}
	if provisioning.Storage.Wipe != nil {
		setDefaultParamValue(params, "storage_wipe", *provisioning.Storage.Wipe)
	}
	if provisioning.Boot.Installed.TimeoutSeconds != nil {
		setDefaultParamValue(params, "boot_timeout_seconds", *provisioning.Boot.Installed.TimeoutSeconds)
	}
	if provisioning.Repos.Firmware != nil {
		setDefaultParamValue(params, "repo_firmware", *provisioning.Repos.Firmware)
	}
	if provisioning.Repos.Contrib != nil {
		setDefaultParamValue(params, "repo_contrib", *provisioning.Repos.Contrib)
	}
	if provisioning.Repos.NonFree != nil {
		setDefaultParamValue(params, "repo_non_free", *provisioning.Repos.NonFree)
	}
	for key, value := range provisioning.Installer.ConfigParams {
		if _, ok := params[key]; !ok {
			params[key] = value
		}
	}
	if len(provisioning.Storage.WipeDiskPatterns) > 0 {
		setDefaultParamValue(params, "storage_wipe_disks", strings.Join(provisioning.Storage.WipeDiskPatterns, " "))
	}
	if release, ok := params["release"]; ok {
		setDefaultParamValue(params, "coreos_release", release)
	}
	if linuxArgs, ok := params["linuxargs"]; ok {
		setDefaultParamValue(params, "linux_cfg_args", linuxArgs)
	}
	if disk, ok := params["storage_disk"]; ok {
		setDefaultParamValue(params, "storage_wipe_disks", disk)
		setDefaultParamValue(params, "storage_template_disk", disk)
		setDefaultParamValue(params, "kickstart_storage_drive", kickstartDriveName(fmt.Sprint(disk)))
	}
	if storageMode, ok := params["storage_mode"]; ok {
		setDefaultParamValue(params, "ubuntu_minimal_storage_mode", storageMode)
	}
	if timezone, ok := params["time_timezone"]; ok {
		setDefaultParamValue(params, "ubuntu_minimal_timezone", timezone)
	}
	if timeUTC, ok := params["time_utc"]; ok {
		setDefaultParamValue(params, "kickstart_utc_flag", kickstartUTCFlag(fmt.Sprint(timeUTC)))
	}
	if groups, ok := params["packages_groups"]; ok {
		setDefaultParamValue(params, "kickstart_package_groups", kickstartGroupLines(fmt.Sprint(groups)))
	}
	if packages, ok := params["packages_install"]; ok {
		setDefaultParamValue(params, "kickstart_packages", packageLines(fmt.Sprint(packages)))
	}
	if bootArgs, ok := params["boot_installed_args"]; ok {
		setDefaultParamValue(params, "kickstart_boot_args", bootArgs)
	}
	if centosMirror, ok := params["repo_centos_mirror"]; ok {
		setDefaultParamValue(params, "kickstart_centos_mirror", centosMirror)
	}
	setProvisioningDefaults(params)
	deriveDebianPartmanModules(params, explicitDebianRegularPartmanModules, explicitDebianLVMPartmanModules, explicitDebianRAIDPartmanModules)
}

func setProvisioningDefaults(params map[string]interface{}) {
	setDefaultParamValue(params, "encrypt_home", "false")
	setDefaultParamValue(params, "iface", "auto")
	setDefaultParamValue(params, "linuxargs", "")
	setDefaultParamValue(params, "linux_cfg_args", "console=tty0 console=ttyS0,115200n8 console=ttyS1,115200n8 vga=normal biosdevname=0 nomodeset interface=auto libata.force=noncq consoleblank=0")
	setDefaultParamValue(params, "vg_name", "vg0")
	setDefaultParamValue(params, "locale_language", "en_US.UTF-8")
	setDefaultParamValue(params, "locale_keyboard", "us")
	setDefaultParamValue(params, "time_timezone", "UTC")
	setDefaultParamValue(params, "ubuntu_minimal_timezone", "America/Los_Angeles")
	setDefaultParamValue(params, "time_utc", "true")
	setDefaultParamValue(params, "kickstart_utc_flag", "--utc")
	setDefaultParamValue(params, "time_ntp", "true")
	setDefaultParamValue(params, "network_bootproto", "dhcp")
	setDefaultParamValue(params, "nameservers", "")
	setDefaultParamValue(params, "packages_install", "openssh-server curl ca-certificates git")
	setDefaultParamValue(params, "packages_groups", "")
	setDefaultParamValue(params, "kickstart_package_groups", "@core")
	setDefaultParamValue(params, "kickstart_packages", "")
	setDefaultParamValue(params, "packages_update_policy", "unattended-upgrades")
	setDefaultParamValue(params, "storage_disk", "/dev/nvme0n1")
	setDefaultParamValue(params, "storage_wipe_disks", "/dev/nvme0n1 /dev/nvme1n1")
	setDefaultParamValue(params, "storage_template_disk", "/dev/sda /dev/sdb")
	setDefaultParamValue(params, "kickstart_storage_drive", "sda")
	setDefaultParamValue(params, "storage_wipe", "true")
	setDefaultParamValue(params, "storage_mode", "regular")
	setDefaultParamValue(params, "ubuntu_minimal_storage_mode", "regular")
	setDefaultParamValue(params, "debian_regular_partman_modules", "partman-ext4")
	setDefaultParamValue(params, "debian_regular_esp_min_size_mib", "512")
	setDefaultParamValue(params, "debian_regular_esp_priority", "512")
	setDefaultParamValue(params, "debian_regular_esp_max_size_mib", "512")
	setDefaultParamValue(params, "debian_regular_esp_fstype", "fat32")
	setDefaultParamValue(params, "debian_regular_esp_mountpoint", "/boot/efi")
	setDefaultParamValue(params, "debian_regular_boot_min_size_mib", "1024")
	setDefaultParamValue(params, "debian_regular_boot_priority", "1024")
	setDefaultParamValue(params, "debian_regular_boot_max_size_mib", "1024")
	setDefaultParamValue(params, "debian_regular_boot_fstype", "ext4")
	setDefaultParamValue(params, "debian_regular_boot_mountpoint", "/boot")
	setDefaultParamValue(params, "debian_regular_swap_enabled", "true")
	setDefaultParamValue(params, "debian_regular_swap_min_size_mib", "8192")
	setDefaultParamValue(params, "debian_regular_swap_priority", "8192")
	setDefaultParamValue(params, "debian_regular_swap_max_size_mib", "8192")
	setDefaultParamValue(params, "debian_regular_root_min_size_mib", "20000")
	setDefaultParamValue(params, "debian_regular_root_priority", "100000000")
	setDefaultParamValue(params, "debian_regular_root_max_size_mib", "-1")
	setDefaultParamValue(params, "debian_regular_root_fstype", "ext4")
	setDefaultParamValue(params, "debian_regular_root_mountpoint", "/")
	setDefaultParamValue(params, "debian_lvm_partman_modules", "lvm2-udeb partman-lvm partman-ext4")
	setDefaultParamValue(params, "debian_lvm_esp_min_size_mib", "512")
	setDefaultParamValue(params, "debian_lvm_esp_priority", "512")
	setDefaultParamValue(params, "debian_lvm_esp_max_size_mib", "512")
	setDefaultParamValue(params, "debian_lvm_esp_fstype", "fat32")
	setDefaultParamValue(params, "debian_lvm_esp_mountpoint", "/boot/efi")
	setDefaultParamValue(params, "debian_lvm_boot_min_size_mib", "1024")
	setDefaultParamValue(params, "debian_lvm_boot_priority", "1024")
	setDefaultParamValue(params, "debian_lvm_boot_max_size_mib", "1024")
	setDefaultParamValue(params, "debian_lvm_boot_fstype", "ext4")
	setDefaultParamValue(params, "debian_lvm_boot_mountpoint", "/boot")
	setDefaultParamValue(params, "debian_lvm_pv_min_size_mib", "1000")
	setDefaultParamValue(params, "debian_lvm_pv_priority", "10000")
	setDefaultParamValue(params, "debian_lvm_pv_max_size_mib", "-1")
	setDefaultParamValue(params, "debian_lvm_swap_enabled", "true")
	setDefaultParamValue(params, "debian_lvm_swap_min_size_mib", "8192")
	setDefaultParamValue(params, "debian_lvm_swap_priority", "8192")
	setDefaultParamValue(params, "debian_lvm_swap_max_size_mib", "8192")
	setDefaultParamValue(params, "debian_lvm_root_min_size_mib", "20000")
	setDefaultParamValue(params, "debian_lvm_root_priority", "100000000")
	setDefaultParamValue(params, "debian_lvm_root_max_size_mib", "-1")
	setDefaultParamValue(params, "debian_lvm_root_fstype", "ext4")
	setDefaultParamValue(params, "debian_lvm_root_mountpoint", "/")
	setDefaultParamValue(params, "debian_raid_partman_modules", "mdadm-udeb partman-md partman-ext4")
	setDefaultParamValue(params, "debian_raid_esp_min_size_mib", "512")
	setDefaultParamValue(params, "debian_raid_esp_priority", "512")
	setDefaultParamValue(params, "debian_raid_esp_max_size_mib", "512")
	setDefaultParamValue(params, "debian_raid_esp_fstype", "fat32")
	setDefaultParamValue(params, "debian_raid_esp_mountpoint", "/boot/efi")
	setDefaultParamValue(params, "debian_raid_boot_min_size_mib", "1024")
	setDefaultParamValue(params, "debian_raid_boot_priority", "1024")
	setDefaultParamValue(params, "debian_raid_boot_max_size_mib", "1024")
	setDefaultParamValue(params, "debian_raid_boot_fstype", "ext4")
	setDefaultParamValue(params, "debian_raid_boot_mountpoint", "/boot")
	setDefaultParamValue(params, "debian_raid_swap_enabled", "true")
	setDefaultParamValue(params, "debian_raid_swap_min_size_mib", "8192")
	setDefaultParamValue(params, "debian_raid_swap_priority", "8192")
	setDefaultParamValue(params, "debian_raid_swap_max_size_mib", "8192")
	setDefaultParamValue(params, "debian_raid_root_min_size_mib", "20000")
	setDefaultParamValue(params, "debian_raid_root_priority", "100000000")
	setDefaultParamValue(params, "debian_raid_root_max_size_mib", "-1")
	setDefaultParamValue(params, "debian_raid_root_fstype", "ext4")
	setDefaultParamValue(params, "debian_raid_root_mountpoint", "/")
	setDefaultParamValue(params, "storage_raid_level", "1")
	setDefaultParamValue(params, "storage_raid_boot_degraded", "true")
	setDefaultParamValue(params, "repo_firmware", "true")
	setDefaultParamValue(params, "repo_contrib", "true")
	setDefaultParamValue(params, "repo_non_free", "true")
	setDefaultParamValue(params, "repo_debian_mirror", "http://ftp.debian.org/debian")
	setDefaultParamValue(params, "repo_ubuntu_mirror", "http://mirror.rackspace.com/ubuntu")
	setDefaultParamValue(params, "repo_centos_mirror", "http://mirror.centos.org/centos")
	setDefaultParamValue(params, "kickstart_centos_mirror", "http://mirror.netcologne.de/centos")
	setDefaultParamValue(params, "repo_coreos_mirror", "http://stable.release.core-os.net/amd64-usr")
	setDefaultParamValue(params, "coreos_release", "current")
	setDefaultParamValue(params, "boot_installed_args", "consoleblank=0 loglevel=6 sysrq_always_enabled clocksource=tsc tsc=reliable")
	setDefaultParamValue(params, "kickstart_boot_args", "crashkernel=auto panic=60")
	setDefaultParamValue(params, "boot_timeout_seconds", "5")
	setDefaultParamValue(params, "debian_installer_config_template", "preseed/debian")
	setDefaultParamValue(params, "storage_installer_config_template", "preseed/storage")
	setDefaultParamValue(params, "ubuntu_minimal_installer_config_template", "preseed/ubuntu-minimal")
	setDefaultParamValue(params, "centos_installer_config_template", "centos.ks")
	setDefaultParamValue(params, "coreos_installer_config_template", "cloudconfig-coreos")
	setDefaultParamValue(params, "boot_ref_query", "")
	setDefaultParamValue(params, "boot_ref_query_suffix", "")
	setDefaultParamValue(params, "boot_ref_query_question", "")
	setDefaultParamValue(params, "installerExtra", "")
}

func setDefaultParam(params map[string]interface{}, key string, value string) {
	if value == "" {
		return
	}
	if _, ok := params[key]; !ok {
		params[key] = value
	}
}

func setDefaultParamValue(params map[string]interface{}, key string, value any) {
	if _, ok := params[key]; !ok {
		params[key] = fmt.Sprint(value)
	}
}

func projectRAIDParams(params map[string]interface{}, raid RAIDConfig) {
	if raid.Level != 0 {
		setDefaultParamValue(params, "storage_raid_level", raid.Level)
	}
	if len(raid.Devices) > 0 {
		setDefaultParamValue(params, "storage_raid_devices", strings.Join(raid.Devices, " "))
		setDefaultParamValue(params, "storage_raid_device_0", raid.Devices[0])
		if len(raid.Devices) > 1 {
			setDefaultParamValue(params, "storage_raid_device_1", raid.Devices[1])
		}
	}
	if raid.BootDegraded != nil {
		setDefaultParamValue(params, "storage_raid_boot_degraded", *raid.BootDegraded)
	}
}

func projectDebianRegularFilesystemParams(params map[string]interface{}, storage StorageConfig) {
	filesystems := storage.Filesystems
	if len(filesystems) == 0 {
		if storage.absentFilesystems["swap"] {
			setDefaultParamValue(params, "debian_regular_swap_enabled", "false")
		}
		return
	}

	projectDebianRegularPartitionParams(params, "esp", "debian_regular_esp", filesystems["esp"])
	projectDebianRegularPartitionParams(params, "boot", "debian_regular_boot", filesystems["boot"])
	projectDebianRegularPartitionParams(params, "swap", "debian_regular_swap", filesystems["swap"])
	projectDebianRegularPartitionParams(params, "root", "debian_regular_root", filesystems["root"])

	if modules := debianRegularPartmanModules(filesystems); modules != "" {
		setDefaultParamValue(params, "debian_regular_partman_modules", modules)
	}
	if storage.absentFilesystems["swap"] {
		setDefaultParamValue(params, "debian_regular_swap_enabled", "false")
	}
}

func projectDebianLVMFilesystemParams(params map[string]interface{}, storage StorageConfig) {
	filesystems := storage.Filesystems
	if len(filesystems) == 0 {
		if storage.absentFilesystems["swap"] {
			setDefaultParamValue(params, "debian_lvm_swap_enabled", "false")
		}
		return
	}

	projectDebianRegularPartitionParams(params, "esp", "debian_lvm_esp", filesystems["esp"])
	projectDebianRegularPartitionParams(params, "boot", "debian_lvm_boot", filesystems["boot"])
	projectDebianRegularPartitionParams(params, "swap", "debian_lvm_swap", filesystems["swap"])
	projectDebianRegularPartitionParams(params, "root", "debian_lvm_root", filesystems["root"])

	if modules := debianLVMPartmanModules(filesystems); modules != "" {
		setDefaultParamValue(params, "debian_lvm_partman_modules", modules)
	}
	if storage.absentFilesystems["swap"] {
		setDefaultParamValue(params, "debian_lvm_swap_enabled", "false")
	}
}

func projectDebianRAIDFilesystemParams(params map[string]interface{}, storage StorageConfig) {
	filesystems := storage.Filesystems
	if len(filesystems) == 0 {
		if storage.absentFilesystems["swap"] {
			setDefaultParamValue(params, "debian_raid_swap_enabled", "false")
		}
		return
	}

	projectDebianRegularPartitionParams(params, "esp", "debian_raid_esp", filesystems["esp"])
	projectDebianRegularPartitionParams(params, "boot", "debian_raid_boot", filesystems["boot"])
	projectDebianRegularPartitionParams(params, "swap", "debian_raid_swap", filesystems["swap"])
	projectDebianRegularPartitionParams(params, "root", "debian_raid_root", filesystems["root"])

	if modules := debianRAIDPartmanModules(filesystems); modules != "" {
		setDefaultParamValue(params, "debian_raid_partman_modules", modules)
	}
	if storage.absentFilesystems["swap"] {
		setDefaultParamValue(params, "debian_raid_swap_enabled", "false")
	}
}

func projectDebianRegularPartitionParams(params map[string]interface{}, name, prefix string, filesystem FilesystemConfig) {
	if boolValue(filesystem.Absent) {
		if name == "swap" {
			setDefaultParamValue(params, prefix+"_enabled", "false")
		}
		return
	}
	if filesystem.Mountpoint != "" && name != "swap" {
		setDefaultParamValue(params, prefix+"_mountpoint", filesystem.Mountpoint)
	}
	if filesystem.FSType != "" && name != "swap" {
		setDefaultParamValue(params, prefix+"_fstype", filesystem.FSType)
	}
	if filesystem.SizeMiB != nil {
		setDefaultParamValue(params, prefix+"_min_size_mib", *filesystem.SizeMiB)
		if name == "root" && strings.EqualFold(filesystem.Size, "grow") {
			setDefaultParamValue(params, prefix+"_max_size_mib", "-1")
		} else {
			setDefaultParamValue(params, prefix+"_priority", *filesystem.SizeMiB)
			setDefaultParamValue(params, prefix+"_max_size_mib", *filesystem.SizeMiB)
		}
		return
	}
	if name == "root" && strings.EqualFold(filesystem.Size, "grow") {
		setDefaultParamValue(params, prefix+"_max_size_mib", "-1")
	}
}

func debianRegularPartmanModules(filesystems map[string]FilesystemConfig) string {
	modules := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, name := range []string{"boot", "root"} {
		filesystem := filesystems[name]
		if boolValue(filesystem.Absent) {
			continue
		}
		fstype := filesystem.FSType
		if fstype == "" {
			fstype = "ext4"
		}
		module := debianPartmanModule(fstype)
		if module == "" || seen[module] {
			continue
		}
		modules = append(modules, module)
		seen[module] = true
	}
	return strings.Join(modules, " ")
}

func debianLVMPartmanModules(filesystems map[string]FilesystemConfig) string {
	modules := []string{"lvm2-udeb", "partman-lvm"}
	seen := map[string]bool{
		"lvm2-udeb":   true,
		"partman-lvm": true,
	}
	for _, name := range []string{"boot", "root"} {
		filesystem := filesystems[name]
		if boolValue(filesystem.Absent) {
			continue
		}
		fstype := filesystem.FSType
		if fstype == "" {
			fstype = "ext4"
		}
		module := debianPartmanModule(fstype)
		if module == "" || seen[module] {
			continue
		}
		modules = append(modules, module)
		seen[module] = true
	}
	return strings.Join(modules, " ")
}

func debianRAIDPartmanModules(filesystems map[string]FilesystemConfig) string {
	modules := []string{"mdadm-udeb", "partman-md"}
	seen := map[string]bool{
		"mdadm-udeb": true,
		"partman-md": true,
	}
	for _, name := range []string{"boot", "root"} {
		filesystem := filesystems[name]
		if boolValue(filesystem.Absent) {
			continue
		}
		fstype := filesystem.FSType
		if fstype == "" {
			fstype = "ext4"
		}
		module := debianPartmanModule(fstype)
		if module == "" || seen[module] {
			continue
		}
		modules = append(modules, module)
		seen[module] = true
	}
	return strings.Join(modules, " ")
}

func deriveDebianPartmanModules(params map[string]interface{}, explicitRegularModules, explicitLVMModules, explicitRAIDModules bool) {
	if !explicitRegularModules {
		params["debian_regular_partman_modules"] = debianPartmanModulesForFinalParams(params, "debian_regular", false)
	}
	if !explicitLVMModules {
		params["debian_lvm_partman_modules"] = debianPartmanModulesForFinalParams(params, "debian_lvm", true)
	}
	if !explicitRAIDModules {
		params["debian_raid_partman_modules"] = debianRAIDPartmanModulesForFinalParams(params)
	}
}

func debianPartmanModulesForFinalParams(params map[string]interface{}, prefix string, lvm bool) string {
	modules := make([]string, 0, 4)
	seen := map[string]bool{}
	if lvm {
		modules = append(modules, "lvm2-udeb", "partman-lvm")
		seen["lvm2-udeb"] = true
		seen["partman-lvm"] = true
	}
	for _, name := range []string{"boot", "root"} {
		fstype := strings.TrimSpace(fmt.Sprint(params[prefix+"_"+name+"_fstype"]))
		module := debianPartmanModule(fstype)
		if module == "" || seen[module] {
			continue
		}
		modules = append(modules, module)
		seen[module] = true
	}
	return strings.Join(modules, " ")
}

func debianRAIDPartmanModulesForFinalParams(params map[string]interface{}) string {
	modules := []string{"mdadm-udeb", "partman-md"}
	seen := map[string]bool{
		"mdadm-udeb": true,
		"partman-md": true,
	}
	for _, name := range []string{"boot", "root"} {
		fstype := strings.TrimSpace(fmt.Sprint(params["debian_raid_"+name+"_fstype"]))
		module := debianPartmanModule(fstype)
		if module == "" || seen[module] {
			continue
		}
		modules = append(modules, module)
		seen[module] = true
	}
	return strings.Join(modules, " ")
}

func debianPartmanModule(fstype string) string {
	switch strings.ToLower(strings.TrimSpace(fstype)) {
	case "ext2", "ext3", "ext4":
		return "partman-ext4"
	case "xfs":
		return "partman-xfs"
	case "btrfs":
		return "partman-btrfs"
	default:
		return ""
	}
}

func kickstartGroupLines(raw string) string {
	groups := strings.Fields(raw)
	if len(groups) == 0 {
		return ""
	}
	for i, group := range groups {
		if !strings.HasPrefix(group, "@") {
			groups[i] = "@" + group
		}
	}
	return strings.Join(groups, "\n")
}

func packageLines(raw string) string {
	return strings.Join(strings.Fields(raw), "\n")
}

func kickstartDriveName(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimPrefix(fields[0], "/dev/")
}

func kickstartUTCFlag(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "false") {
		return ""
	}
	return "--utc"
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
	base.absentFilesystems = copyBoolMap(base.absentFilesystems)
	if override.Disk != "" {
		base.Disk = override.Disk
	}
	if override.Wipe != nil {
		base.Wipe = copyBoolPtr(override.Wipe)
	}
	if override.WipeDiskPatterns != nil {
		base.WipeDiskPatterns = append([]string(nil), override.WipeDiskPatterns...)
	}
	if override.Mode != "" {
		base.Mode = override.Mode
	}
	if override.VolumeGroup != "" {
		base.VolumeGroup = override.VolumeGroup
	}
	base.RAID = mergeRAIDConfig(base.RAID, override.RAID)
	if override.Filesystems != nil {
		base = mergeFilesystemConfigMapWithAbsent(base, override.Filesystems)
	}
	return base
}

func mergeRAIDConfig(base RAIDConfig, override RAIDConfig) RAIDConfig {
	if override.Level != 0 {
		base.Level = override.Level
	}
	if override.Devices != nil {
		base.Devices = append([]string(nil), override.Devices...)
	}
	if override.BootDegraded != nil {
		base.BootDegraded = copyBoolPtr(override.BootDegraded)
	}
	return base
}

func mergeFilesystemConfigMapWithAbsent(base StorageConfig, override map[string]FilesystemConfig) StorageConfig {
	merged := copyFilesystemConfigMap(base.Filesystems)
	if merged == nil {
		merged = make(map[string]FilesystemConfig, len(override))
	}
	absent := copyBoolMap(base.absentFilesystems)
	for name, filesystem := range override {
		if boolValue(filesystem.Absent) {
			delete(merged, name)
			if absent == nil {
				absent = make(map[string]bool)
			}
			absent[name] = true
			continue
		}
		if absent != nil {
			delete(absent, name)
		}
		merged[name] = mergeFilesystemConfig(merged[name], filesystem)
	}
	base.Filesystems = merged
	base.absentFilesystems = absent
	return base
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
	config.Storage.WipeDiskPatterns = append([]string(nil), config.Storage.WipeDiskPatterns...)
	config.Storage.RAID = copyRAIDConfig(config.Storage.RAID)
	config.Storage.Filesystems = copyFilesystemConfigMap(config.Storage.Filesystems)
	config.Storage.absentFilesystems = copyBoolMap(config.Storage.absentFilesystems)
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

func copyRAIDConfig(raid RAIDConfig) RAIDConfig {
	raid.Devices = append([]string(nil), raid.Devices...)
	raid.BootDegraded = copyBoolPtr(raid.BootDegraded)
	return raid
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

func copyBoolMap(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	copied := make(map[string]bool, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
