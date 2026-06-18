# Configuration

Shoelaces can be configured with CLI flags, environment variables, or a config file.
Command-line flags have the highest precedence, then environment variables, then config file values, then defaults.
Environment variable names are uppercase flag names with hyphens converted to underscores, such as `DATA_DIR`, `BIND_ADDR`, and `TFTP_ENABLED`.

Configuration files can be TOML, YAML, or JSON.
The parser is selected from the config file extension: `.toml`, `.yaml`, `.yml`, or `.json`.
Nested TFTP settings use a `tftp` object/table and map to the `tftp-*` CLI flags, such as `tftp.enabled`, `tftp.address`, and `tftp.timeout`.

## Flags

- `config`: path to a configuration file.
- `bind-addr`: address Shoelaces listens on. Defaults to `localhost:8081`.
- `base-url`: base URL used when generating URLs. Defaults to `bind-addr`.
- `data-dir`: root directory for templates, mappings, and static provisioning files.
- `ui-dir`: optional path to a custom UI directory containing web templates and frontend assets. By default, Shoelaces uses UI assets embedded in the binary.
- `static-dir`: deprecated alias for `ui-dir`, retained for compatibility. Do not use it for provisioning static files.
- `env-dir`: environment overrides directory inside `data-dir`. Defaults to `env_overrides`.
- `mappings-file`: YAML mappings file path relative to `data-dir`. Defaults to `mappings.yaml`.
- `template-extension`: template filename extension. Defaults to `.slc`.
- `debug`: enable debug messages.
- `tftp-enabled`: enable the embedded TFTP server.
- `tftp-addr`: embedded TFTP listen address. Defaults to `:69`.
- `tftp-root`: directory served over TFTP. Defaults to `data-dir/tftp` when not explicitly set.
- `tftp-readonly`: disable TFTP uploads. Defaults to `true`.
- `tftp-timeout`: embedded TFTP per-request timeout. Defaults to `5s`.

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

## Example

```toml
bind-addr = "localhost:8081"
data-dir = "configs/data-dir/"
template-extension = ".slc"
mappings-file = "mappings.yaml"
debug = true

[tftp]
enabled = true
address = ":69"
root = "/var/lib/shoelaces/tftp"
readonly = true
timeout = "5s"
```

Example files are available for [TOML](../configs/shoelaces.toml), [YAML](../configs/shoelaces.yaml), and [JSON](../configs/shoelaces.json).
