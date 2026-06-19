# Mappings And Environments

Shoelaces resolves each booting host to a named boot target from `mappings.yaml`.
A target points at an iPXE template, an optional environment override, a UI label, structured provisioning policy, and optional target-specific escape-hatch parameters.

## Target Selection

Mappings can select targets by MAC address, exact IP address, hostname regular expression, or CIDR network.
Match precedence is:

1. Manual selection.
2. MAC address.
3. IP address.
4. Hostname.
5. Network.
6. Unmatched/manual queue.

If a mapping has `defaultTarget`, Shoelaces boots it automatically.
If a mapping only has `targets`, the host is queued in the UI and operators can choose from that restricted target list.
Unmatched hosts can choose from all configured targets.

## Example Mapping

```yaml
defaults:
  params:
    encrypt_home: "false"
  boot:
    netboot:
      kernelArgs: []
  repos:
    osMirror: http://ftp.debian.org/debian
    release: trixie

targets:
  debian12:
    script: debian.ipxe
    label: Debian 12 Bookworm (oldstable)
    repos:
      release: bookworm

  debian13:
    script: debian.ipxe
    label: Debian 13 Trixie
    repos:
      release: trixie

  debian13-luks:
    script: debian.ipxe
    label: Debian 13 Trixie LUKS
    repos:
      release: trixie
    storage:
      mode: regular
      encryption:
        enabled: true
        passphrase:
          env: SHOELACES_LUKS_PASSPHRASE

networkMaps:
  - network: 104.225.9.96/27
    defaultTarget: debian13
    targets:
      - debian12
      - debian13
      - debian13-luks
    params:
      hostnamePrefix: iad-

hostnameMaps:
  - hostname: '^debian13-[0-9]+\.example\.com$'
    defaultTarget: debian13
    targets:
      - debian13
```

Shoelaces reads mappings from the YAML file configured by `mappings-file`, relative to `data-dir`.
See [configs/data-dir/mappings.yaml](../configs/data-dir/mappings.yaml) for a complete example.
The old `networkMaps[].script` and `hostnameMaps[].script` schema is no longer supported; define named `targets` and reference them from mapping rules instead.
See [Provisioning Defaults And Overrides](provisioning-overrides.md) for the
structured provisioning sections that targets and mapping rules can set.

## Parameters

`params` is an escape hatch for custom site templates and low-level values that
do not have a structured field yet. Prefer structured sections such as `repos`,
`storage`, `boot`, `packages`, `network`, `locale`, `time`, `users`, and
`installer` for built-in provisioning behavior.

Parameter merge order for escape-hatch params is:

1. Global `defaults.params`.
2. Selected target `params`.
3. Matched mapping `params`.
4. Manual/request params.
5. Generated values such as `hostname` and `baseURL`.

Raw scalar values can be placed directly in `params`.
Sensitive values can also come from the Shoelaces process environment using an explicit reference:

```yaml
params:
  root_password_crypted:
    env: SHOELACES_ROOT_PASSWORD_CRYPTED
```

This uses the environment of the running Shoelaces process, so systemd service environment variables work without a separate env file.
Missing referenced environment variables fail the boot render clearly.

When a built-in field exists both in structured config and legacy `params`,
structured mapping policy is treated as canonical for mapping-resolved boots.
Explicit manual/request params can still override for one-off renders.

## Storage Disk Selection

`storage.disk` is the primary install disk. Installer templates use it when
choosing the disk to partition for the operating system.

`storage.wipeDiskPatterns` is separate. It is an explicit list of `/dev` paths
or glob patterns that templates may clear before partitioning. Use it when a
host must remove stale partition tables, mdraid metadata, or LVM metadata from a
known disposable disk set.

NVMe-only hosts can use a narrow NVMe selector:

```yaml
storage:
  disk: /dev/nvme0n1
  wipe: true
  wipeDiskPatterns:
    - /dev/nvme*n*
```

SATA/SCSI-style hosts can use an `sd` selector:

```yaml
storage:
  disk: /dev/sda
  wipe: true
  wipeDiskPatterns:
    - /dev/sd*
```

Use broad patterns such as `/dev/sd*` only when every matching disk on the host
is known to be disposable. Prefer stable, narrower selectors such as
`/dev/disk/by-id/<fleet-prefix>*` when available.

## Debian Storage Modes

Structured `storage.mode` rendering currently applies to Debian preseed
targets. Kickstart, Ubuntu-specific installers, CoreOS/Ignition, and cloud-init
storage semantics should be handled as OS-specific work rather than inheriting
the Debian preseed layout.

Debian `storage.mode: regular` is the default. It renders a plain GPT/UEFI
layout on `storage.disk` with an EFI System Partition mounted at `/boot/efi`, a
separate ext4 `/boot`, optional swap, and root mounted at `/`. `/boot` remains
separate for consistency with LVM mode and to keep future encryption-sensitive
layouts straightforward.

Debian `storage.mode: lvm` preserves the explicit LVM layout. It renders the
same ESP and `/boot` partitions, then creates root and swap logical volumes in
`storage.volumeGroup`. LVM installs should opt in explicitly:

```yaml
storage:
  mode: lvm
  disk: /dev/nvme0n1
  volumeGroup: vg0
```

Set `storage.encryption.enabled: true` on Debian `regular`, `lvm`, or `raid`
targets to render LUKS preseeding from structured storage fields. The
passphrase is required when encryption is enabled. `cipher`, `keySize`, and
`hash` default to `aes-xts-plain64`, `512`, and `sha512` unless overridden:

```yaml
storage:
  mode: regular
  disk: /dev/nvme0n1
  encryption:
    enabled: true
    passphrase:
      env: SHOELACES_LUKS_PASSPHRASE
```

The encrypted Debian layouts keep boot-critical files outside LUKS. In
`regular` mode, the ESP and `/boot` are unencrypted partitions while swap and
root are separate LUKS volumes. In `lvm` mode, the ESP and `/boot` are
unencrypted partitions, and LUKS wraps the physical volume that backs
`storage.volumeGroup`; root and swap remain logical volumes inside that volume
group. In `raid` mode, each disk gets a normal duplicated ESP, `/boot` is
unencrypted RAID1, and LUKS is placed on the RAID1 devices used for swap and
root.

Preseeded encryption is unattended, so the installer config necessarily
contains the LUKS passphrase when rendered. Use environment-backed passphrases,
boot-session references, and trusted provisioning networks for encrypted
targets.

`storage.encryption` follows the normal structured provisioning merge order:
defaults, selected target, then the matched mapping rule. Prefer per-host
environment references on specific MAC, IP, or hostname mappings for production
passphrases. Raw passphrase values are accepted only for disposable lab
fixtures.

Structured `storage.encryption` is supported only by the embedded
`preseed/debian` installer template. Embedded Ubuntu minimal, CentOS kickstart,
CoreOS/cloud-init, and other non-Debian installer templates reject it instead
of rendering an unencrypted install by accident. For non-Debian encrypted
installs, use `installer.extraTemplate` or a full template override with native
installer syntax.

The legacy `plain` value remains parseable in mappings for compatibility with
older generic config, but Debian preseed rejects it clearly. Use `regular` for
new Debian plain-disk installs.

For Debian `storage.mode: raid`, keep member disks as layout data under
`storage.raid.devices`, not as wipe selectors. Initial RAID support is UEFI-only
RAID1 with exactly two member disks. It remains compatible with Shoelaces/iPXE
installer startup when the host network boots a UEFI iPXE binary; the UEFI-only
constraint applies to installed-system disk boot and ESP layout. Prefer stable
`/dev/disk/by-id/...` paths for production servers:

```yaml
boot:
  firmware: uefi
storage:
  mode: raid
  raid:
    level: 1
    devices:
      - /dev/disk/by-id/nvme-Samsung_SSD_990_PRO_os_a
      - /dev/disk/by-id/nvme-Samsung_SSD_990_PRO_os_b
    bootDegraded: true
```

Debian RAID mode creates a normal 512 MiB ESP on each disk, a 1 GiB RAID1
ext4 `/boot`, optional RAID1 swap, and a growable RAID1 ext4 root filesystem.
`storage.filesystems` can override the same named entries used by regular and
LVM modes (`esp`, `boot`, `swap`, and `root`), but the initial documented
defaults remain intentionally conservative. The ESPs are duplicated normal FAT
partitions rather than mdraid members because UEFI firmware does not assemble
Linux md arrays.

### RAID1 OS Disk Recovery

When replacing a failed Debian RAID1 OS disk:

1. Identify the failed disk and surviving disk with `lsblk`, `cat /proc/mdstat`,
   and `mdadm --detail /dev/md*`.
2. Replace the failed disk and confirm the new disk path is the expected
   `storage.raid.devices` member, preferably under `/dev/disk/by-id/...`.
3. Recreate the replacement disk partition table to match the surviving disk.
   For matching devices, `sgdisk --replicate` followed by `sgdisk --randomize-guids`
   is usually the least error-prone approach.
4. Format or restore the replacement ESP, copy the surviving ESP contents to
   it, and reinstall or refresh the EFI fallback bootloader on that ESP.
5. Add the replacement RAID member partitions back to the md arrays with
   `mdadm --add`.
6. Monitor resync with `cat /proc/mdstat` until all arrays are clean.
7. Refresh mdadm config and initramfs with `mdadm --detail --scan` and
   `update-initramfs -u`.
8. Verify firmware boot entries or fallback paths can boot from either disk
   before considering the repair complete.

## Environment Overrides

Environment overrides let a target serve selected templates from `env_overrides/{environment}/` while falling back to the base `data-dir` for everything else.

Example `data-dir` layout:

```txt
├── cloud-config
│   └── coreos-cloud-config.yaml.slc
├── env_overrides
│   └── testing
│       └── cloud-config
│           └── coreos-cloud-config.yaml.slc
├── ipxe
│   ├── coreos.ipxe.slc
│   └── ubuntu-minimal.ipxe.slc
├── mappings.yaml
├── preseed
│   └── common.preseed.slc
└── static
    ├── bootstrap.sh
    └── rc.local-bootstrap
```

A target with `environment: testing` uses templates from `env_overrides/testing` when present.
All other templates are served from the base directory.
Everything except `mappings.yaml` can be overridden by preserving its path under `env_overrides/{environment}/`.
For the full embedded, disk, and environment override precedence model, see
[Provisioning Defaults And Overrides](provisioning-overrides.md).

Shoelaces is mostly stateless.
It sets a different `baseURL` for environment requests:

- Default requests use `http://$shoelaces_host:$port`.
- Environment requests use `http://$shoelaces_host:$port/env/$environment_name/`.

A host cannot boot into a non-default environment unless that environment contains a main iPXE script.
The `/ipxemenu` route only presents default and non-default iPXE entry points; templates included later in the boot process cannot be selected directly as environment overrides.
