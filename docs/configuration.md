# Configuration

Shoelaces can be configured with CLI flags, environment variables, or a config file.
Command-line flags have the highest precedence, then environment variables, then config file values, then defaults.
Environment variable names are uppercase flag names with hyphens converted to underscores, such as `DATA_DIR`, `BIND_ADDR`, and `TFTP_ENABLED`.
`--debug` is CLI-only; use `log.level: debug` in config files.

Configuration files can be TOML, YAML, or JSON.
The parser is selected from the config file extension: `.toml`, `.yaml`, `.yml`, or `.json`.
Config files use nested keys for hyphenated CLI flags. For example, `network.bindAddr` maps to `--bind-addr`, `network.baseURL` maps to `--base-url`, `data.dir` maps to `--data-dir`, `log.level` maps to `--log-level`, and `tftp.enabled` maps to `--tftp-enabled`.
If neither `--config` nor `CONFIG` is provided, Shoelaces uses the first
existing default config file from `/etc/shoelaces/shoelaces.yaml`,
`/etc/shoelaces/shoelaces.json`, and `/etc/shoelaces/shoelaces.toml`.

## Commands

`shoelaces` with no command prints help and exits successfully. Start the
server explicitly with `shoelaces run`.

Runtime inspection commands read the configured SQLite persistence store
directly, so they can be run on the Shoelaces host even when the HTTP server is
stopped:

```sh
shoelaces events list
shoelaces events get 01J...
shoelaces servers list --waiting
shoelaces servers get 00:11:22:33:44:55
shoelaces boot-sessions get 01J...
```

Inspection commands support `--output table` and `--output json`; table output
is the default. Inspection requires `persistence.backend = "sqlite"` because a
separate CLI process cannot inspect another process' in-memory state.

## Flags

`--config` and `--version` are root options. Server startup options belong to
the `run` command, for example `shoelaces --config /srv/shoelaces.toml run
--data-dir /var/lib/shoelaces` when using a nonstandard config path.
Config-file and environment-variable forms are shared by `run` and the runtime
inspection commands where applicable.

| CLI flag                                | Config key                           | Environment                           | Default                | Notes                                                                                                       |
|-----------------------------------------|--------------------------------------|---------------------------------------|------------------------|-------------------------------------------------------------------------------------------------------------|
| `--config`                              | N/A                                  | `CONFIG`                             | First existing `/etc/shoelaces/shoelaces.yaml`, `.json`, then `.toml` | Path to a TOML, YAML, or JSON config file. |
| `--bind-addr`                           | `network.bindAddr`                   | `BIND_ADDR`                           | `:8081`                | HTTP listen address.                                                                                        |
| `--base-url`                            | `network.baseURL`                    | `BASE_URL`                            | `network.bindAddr`     | Used when rendered templates need to refer back to Shoelaces. Wildcard listen addresses advertise `localhost:<port>` by default. |
| `--data-dir`                            | `data.dir`                           | `DATA_DIR`                            | Required               | Root directory for mappings, disk template overrides, provisioning static files, and environment overrides. |
| `--ui-dir`                              | `ui.dir`                             | `UI_DIR`                              | Embedded UI            | Optional custom web UI directory containing templates and static frontend assets.                           |
| `--static-dir`                          | `static.dir`                         | `STATIC_DIR`                          | N/A                    | Compatibility alias for `ui.dir`; it does not control provisioning static files.                            |
| `--env-dir`                             | `env.dir`                            | `ENV_DIR`                             | `env_overrides`        | Directory under `data.dir` for environment-specific overrides.                                              |
| `--mappings-file`                       | `mappings.file`                      | `MAPPINGS_FILE`                       | `mappings.yaml`        | Mapping file path, resolved relative to `data.dir`.                                                         |
| `--template-extension`                  | `template.extension`                 | `TEMPLATE_EXTENSION`                  | `.slc`                 | File extension parsed as a Shoelaces template.                                                              |
| `--debug`                               | N/A                                  | N/A                                   | `false`                | CLI-only shortcut for debug logging. In config files, use `log.level = "debug"`.                            |
| `--log-level`                           | `log.level`                          | `LOG_LEVEL`                           | `info`                 | Minimum log level: `debug`, `info`, `warn`, or `error`.                                                     |
| `--log-handler`                         | `log.handler`                        | `LOG_HANDLER`                         | `dev`                  | Log output format: `dev`, `text`, or `json`.                                                                |
| `--tftp-enabled`                        | `tftp.enabled`                       | `TFTP_ENABLED`                        | `false`                | Start the embedded TFTP server.                                                                             |
| `--tftp-addr`                           | `tftp.address`                       | `TFTP_ADDR`                           | `:69`                  | Embedded TFTP UDP listen address.                                                                           |
| `--tftp-root`                           | `tftp.root`                          | `TFTP_ROOT`                           | `data.dir/tftp`        | Directory served over TFTP when `data.dir` is set and the root is not explicitly changed.                   |
| `--tftp-readonly`                       | `tftp.readonly`                      | `TFTP_READONLY`                       | `true`                 | Reject TFTP upload attempts.                                                                                |
| `--tftp-timeout`                        | `tftp.timeout`                       | `TFTP_TIMEOUT`                        | `5s`                   | Per-request embedded TFTP timeout.                                                                          |
| `--persistence-backend`                 | `persistence.backend`                | `PERSISTENCE_BACKEND`                 | `sqlite`               | Runtime persistence backend: `sqlite` or `memory`.                                                          |
| `--persistence-path`                    | `persistence.path`                   | `PERSISTENCE_PATH`                    | `runtime/shoelaces.db` | SQLite database path, relative to `data.dir` unless absolute.                                               |
| `--persistence-retention-events`        | `persistence.retention.events`       | `PERSISTENCE_RETENTION_EVENTS`        | `720h`                 | Retention window for persisted event history.                                                               |
| `--persistence-retention-events-sweep-interval` | `persistence.retention.eventsSweepInterval` | `PERSISTENCE_RETENTION_EVENTS_SWEEP_INTERVAL` | `24h` | How often persisted event retention cleanup runs while Shoelaces is running.                                |
| `--persistence-retention-boot-sessions` | `persistence.retention.bootSessions` | `PERSISTENCE_RETENTION_BOOT_SESSIONS` | `24h`                  | Retention window for persisted boot/config references.                                                      |
| `--persistence-retention-boot-sessions-sweep-interval` | `persistence.retention.bootSessionsSweepInterval` | `PERSISTENCE_RETENTION_BOOT_SESSIONS_SWEEP_INTERVAL` | `1h` | How often expired boot/config reference cleanup runs while Shoelaces is running.                            |

## Data Directory

Base templates live under `data-dir`.
Environment overrides live under `env_overrides/{environment}/` and preserve the base directory structure.
Typical contents include:

- `ipxe/`: iPXE boot scripts.
- `cloud-config/`: cloud-init configurations.
- `preseed/`: Debian/Ubuntu preseed files.
- `kickstart/`: CentOS/RHEL kickstart files.
- `static/`: static files served without templating.
- `mappings.yaml`: IP, MAC, hostname, and network mapping configuration.

Shoelaces embeds generic provisioning templates by default.
Disk templates in `data-dir` override embedded defaults.
Provisioning static files are served from `data-dir/static` first, then embedded generic defaults, at `/configs/static/*`.

Generic provisioning defaults are embedded in the binary, but `mappings.yaml` is still external site policy and must be provided through `data-dir`.
See [Provisioning Defaults And Overrides](provisioning-overrides.md) for the disk overlay model, full-template overrides, structured provisioning fields, and `installer.extraTemplate` snippets for native installer behavior such as Debian `late_command`, kickstart `%post`, or cloud-init `runcmd`.
See [Runtime Persistence](persistence.md) for database location, retention,
backup/recovery expectations, CQRS boundaries, and the sqlc/Goose workflow.
See [Runtime References](runtime-references.md) for the persisted boot refs,
event IDs, and redacted lookup APIs backed by the persistence store.

## Example

```toml
[network]
bindAddr = ":8081"
baseURL = "localhost:8081"

[data]
dir = "configs/data-dir/"

[template]
extension = ".slc"

[mappings]
file = "mappings.yaml"

[log]
level = "debug"
handler = "dev"

[persistence]
backend = "sqlite"
path = "runtime/shoelaces.db"

[persistence.retention]
events = "720h"
eventsSweepInterval = "24h"
bootSessions = "24h"
bootSessionsSweepInterval = "1h"

[tftp]
enabled = true
address = ":69"
root = "/var/lib/shoelaces/tftp"
readonly = true
timeout = "5s"
```

Example files are available for [TOML](../configs/shoelaces.toml), [YAML](../configs/shoelaces.yaml), and [JSON](../configs/shoelaces.json).
