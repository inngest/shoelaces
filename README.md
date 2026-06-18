# Shoelaces at Inngest

This repository is Inngest's maintained fork of [ThousandEyes Shoelaces](https://github.com/thousandeyes/shoelaces).
Shoelaces serves iPXE boot scripts, cloud-init configuration, installer configs, and static provisioning assets for bare-metal provisioning.

## Build And Test

Use the same Go toolchain selected by `go.mod`.
CI uses `actions/setup-go` with `go-version-file: go.mod`.

```bash
go test ./...
go test -race ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o shoelaces ./cmd/shoelaces
./shoelaces --version
```

The `--version` flag prints the release version, commit, build date, and builder metadata embedded by GoReleaser.
The `make unit` target runs formatted Go unit tests.
The `make test` target runs `make unit` and then the historical Python integration test at `test/integ-test/integ_test.py`; that integration path may require local Python dependencies that are not installed by the Go toolchain.

CI currently runs:

- `go test -race ./...`
- Linux amd64 binary build from `./cmd/shoelaces`
- `golangci-lint`
- a GoReleaser snapshot dry run

## Run Locally

```bash
go build -o shoelaces ./cmd/shoelaces
./shoelaces --config configs/shoelaces.toml
```

The default web UI is available at [localhost:8081](http://localhost:8081).

## Documentation

- [Contributing](CONTRIBUTING.md): local development commands, testing expectations, and commit title conventions.
- [Configuration](docs/configuration.md): CLI flags, config files, environment variables, and embedded provisioning defaults.
- [Network Boot Flow](docs/network-boot.md): PXE to iPXE chainloading, TFTP/DHCP requirements, and boot UI behavior.
- [Mappings And Environments](docs/mappings-and-environments.md): target selection, mapping precedence, template params, and environment overrides.
- [Provisioning Defaults And Overrides](docs/provisioning-overrides.md): disk overlay model, full-template overrides, structured provisioning fields, and installer snippets.
- [Manual Page Source](docs/shoelaces.8.scd): command reference source.

Example configuration and data files live in [configs/](configs/).
