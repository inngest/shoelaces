# Provisioning Defaults Audit

This audit classifies the current `configs/data-dir` files before Plan 003
embeds any provisioning defaults into the Shoelaces binary.

Plan 003 should embed only generic, non-secret defaults. Site policy,
credentials, firstboot orchestration, and operator-specific bootstrap scripts
must remain disk-backed examples or external automation.

## Generic Embeddable Candidates

These files are candidates for embedded runtime defaults after the overlay
implementation exists:

| Path                                    | Notes                                                            |
|-----------------------------------------|------------------------------------------------------------------|
| `cloud-config/cloud-config-release.slc` | Generic CoreOS cloud-config template.                            |
| `cloud-config/users.slc`                | Generic empty users/key partial. Sites should override it.       |
| `ipxe/centos.ipxe.slc`                  | Generic CentOS iPXE entrypoint.                                  |
| `ipxe/coreos.ipxe.slc`                  | Generic CoreOS iPXE entrypoint.                                  |
| `ipxe/debian.ipxe.slc`                  | Generic Debian iPXE entrypoint.                                  |
| `ipxe/linux.cfg.slc`                    | Generic Linux config template.                                   |
| `ipxe/storage.ipxe.slc`                 | Generic storage install iPXE entrypoint.                         |
| `ipxe/ubuntu-minimal.ipxe.slc`          | Generic Ubuntu minimal iPXE entrypoint.                          |
| `kickstart/centos.ks.slc`               | Generic CentOS kickstart with locked root password.              |
| `preseed/common.preseed.slc`            | Generic Debian common preseed with locked default user password. |
| `preseed/debian.preseed.slc`            | Generic Debian preseed with no-op late command.                  |
| `preseed/storage.preseed.slc`           | Generic storage/RAID preseed example.                            |
| `preseed/ubuntu-minimal.preseed.slc`    | Generic Ubuntu minimal preseed.                                  |

## External Runtime Policy

These files should stay outside embedded production defaults:

| Path            | Reason                                               |
|-----------------|------------------------------------------------------|
| `mappings.yaml` | Runtime/site boot policy. Operators must provide it. |

## Site-Only Examples

These files are examples of how an operator can wire post-install behavior from
disk. They should not become embedded generic defaults:

| Path                           | Reason                                           |
|--------------------------------|--------------------------------------------------|
| `plain/firstboot.defaults.slc` | Example firstboot environment file template.     |
| `static/firstboot.sh`          | Example firstboot hook placeholder.              |
| `static/firstboot.service`     | Example service unit for a firstboot hook.       |
| `static/test-script`           | Static-file serving example fixture.             |

Operators that need firstboot behavior should provide their own disk-backed
late-command partial and scripts under `data-dir`, usually fetched through
`/configs/static/*` for raw files or `/configs/<template>` for templated scripts.
