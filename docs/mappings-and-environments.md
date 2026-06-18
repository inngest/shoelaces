# Mappings And Environments

Shoelaces resolves each booting host to a named boot target from `mappings.yaml`.
A target points at an iPXE template, an optional environment override, a UI label, and target-specific template parameters.

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
    linuxargs: ""

targets:
  debian12:
    script: debian.ipxe
    label: Debian 12 Bookworm
    params:
      release: bookworm

  debian13:
    script: debian.ipxe
    label: Debian 13 Trixie
    params:
      release: trixie

networkMaps:
  - network: 104.225.9.96/27
    defaultTarget: debian12
    targets:
      - debian12
      - debian13
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

## Parameters

Parameter merge order is:

1. Global `defaults.params`.
2. Selected target `params`.
3. Matched mapping `params`.
4. Manual/request params.
5. Generated values such as `hostname` and `baseURL`.

Raw scalar values can be placed directly in params.
Sensitive values can also come from the Shoelaces process environment using an explicit reference:

```yaml
params:
  root_password_crypted:
    env: SHOELACES_ROOT_PASSWORD_CRYPTED
```

This uses the environment of the running Shoelaces process, so systemd service environment variables work without a separate env file.
Missing referenced environment variables fail the boot render clearly.

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

Shoelaces is mostly stateless.
It sets a different `baseURL` for environment requests:

- Default requests use `http://$shoelaces_host:$port`.
- Environment requests use `http://$shoelaces_host:$port/env/$environment_name/`.

A host cannot boot into a non-default environment unless that environment contains a main iPXE script.
The `/ipxemenu` route only presents default and non-default iPXE entry points; templates included later in the boot process cannot be selected directly as environment overrides.
