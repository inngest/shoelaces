# Agent Guide

This file provides guidance to AI coding agents working in this repository.

## Commit Titles

Use conventional commit titles for any commit you create. This repository's
changelog configuration in `cliff.toml` enables `conventional_commits = true`
and `filter_unconventional = true`, and `.github/workflows/commits.yml` checks
PR commit titles.

Prefer these commit types because they are grouped or validated in the repo
today:

- `feat`
- `fix`
- `doc`
- `perf`
- `refactor`
- `style`
- `test`
- `chore`
- `ci`
- `revert`
- `security`

Use standard conventional commit formatting such as
`fix(cli): preserve config precedence` or `doc(agent): add repository guide`.

## Development Commands

### Building and Testing

- `go build` builds the Shoelaces binary.
- `make` or `make all` builds the binary.
- `go test ./...` runs the Go unit test suite.
- `make test` runs unit tests and the historical Python integration tests.
- `go fmt` or `make fmt` formats Go code.
- `make clean` removes build artifacts.

### Integration Testing

- `./test/integ-test/integ_test.py -vv` runs integration tests with verbose output.
- Integration tests require the binary to be built first with `go build`.
- Integration tests run on port `18888` and use fixtures in
  `test/integ-test/expected-results/`.

### Running Shoelaces

- `./shoelaces --config configs/shoelaces.toml` runs with the example config.
- The default web UI is available at `localhost:8081`.

## Project Architecture

Shoelaces is a lightweight server bootstrapping tool. It serves iPXE boot
scripts, cloud-init configuration, and other configuration files over HTTP. It
also provides a web UI for deployment state and supports environment-based
configuration overrides.

### Core Components

- `cmd/shoelaces/` defines the CLI entrypoint using `urfave/cli/v3`.
- `environment/` owns central configuration and runtime state,
  including templates, mappings, server states, logging, and TFTP settings.
- `handlers/` contains HTTP handlers for the UI, API endpoints, and
  boot script/config serving.
- `router/` wires routes with Gorilla Mux.
- `templates/` parses and renders Go templates with environment
  override support.
- `mappings/` loads IP/hostname-to-boot-script mappings from YAML.
- `tftpserver/` implements the optional embedded TFTP server.

### Boot Flow

1. An iPXE agent hits `/start`.
2. The agent enters the polling loop at `/poll/1/{mac}`.
3. Shoelaces matches the server against network or hostname mappings, or waits
   for manual selection in the UI.
4. Templates are rendered with environment-specific overrides when configured.

### Template and Config Layout

- Templates use the `.slc` extension by default.
- Base templates live under `data-dir/`.
- Environment overrides live under `env_overrides/{environment}/` and preserve
  the base directory structure.
- Typical `data-dir/` contents include:
  - `ipxe/`: iPXE boot scripts.
  - `cloud-config/`: cloud-init configurations.
  - `preseed/`: Debian/Ubuntu preseed files.
  - `kickstart/`: CentOS/RHEL kickstart files.
  - `static/`: static files served without templating.
  - `mappings.yaml`: IP/hostname mapping configuration.

## Working Style

- Prefer minimal, targeted changes that preserve the existing code style.
- Run relevant tests for the area you touch when practical.
- Commit in small logical chunks when creating commits. Each commit should be
  self-reviewable: one coherent change with its relevant tests/docs, and no
  unrelated cleanup.
- Do not introduce a second commit-title convention; keep commit types aligned
  with `cliff.toml` and `.github/workflows/commits.yml`.
- If working from a checklist or plan, update checklist items incrementally as
  work completes, not only at the end.
- Prefer table-driven tests when they keep related cases easier to review.
- Add comments for non-obvious logic, especially production constraints,
  compatibility behavior, workarounds, performance tradeoffs, safety guardrails,
  or surprising behavior.
- Keep comments succinct and useful for review. Avoid comments that merely
  restate the next line of code.
