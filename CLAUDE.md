# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Shoelaces is a lightweight server bootstrapping tool that serves iPXE boot scripts, cloud-init configuration, and other configuration files over HTTP. It provides a web UI for managing server deployments and supports environment-based configurations.

## Development Commands

### Building and Testing
- `go build` - Build the shoelaces binary
- `make` or `make all` - Build the binary (same as go build)
- `make test` - Run unit tests and integration tests (includes formatting)
- `go fmt` or `make fmt` - Format Go code
- `make clean` - Clean build artifacts

### Integration Testing
- `./test/integ-test/integ_test.py -vv` - Run integration tests with verbose output
- Integration tests require the binary to be built first (`go build`)
- Tests run on port 18888 and use fixtures in `test/integ-test/expected-results/`

### Running Shoelaces
- `./shoelaces -config configs/shoelaces.conf` - Run with example configuration
- Default web UI available at `localhost:8081`

## Architecture

### Core Components

**Environment (`internal/environment/`)**: Central configuration and state management. The Environment struct holds all global data including templates, mappings, server states, and configuration parameters.

**Handlers (`internal/handlers/`)**: HTTP request handlers for web UI, API endpoints, and boot script serving. Key handlers include:
- Boot flow: `/start` → `/poll/1/{mac}` for iPXE agents
- UI endpoints: `/`, `/events`, `/mappings`
- Template serving: `/configs/` for dynamic templates, `/configs/static/` for static files

**Router (`internal/router/`)**: HTTP routing using Gorilla Mux, defines all endpoints and maps them to handlers.

**Templates (`internal/templates/`)**: Go template system for dynamic configuration generation with environment overrides support.

**Mappings (`internal/mappings/`)**: IP/hostname-to-boot-script mapping system using YAML configuration.

### Boot Flow
1. iPXE agent hits `/start` endpoint
2. Agent enters polling loop via `/poll/1/{mac}` 
3. Server matches against network/hostname mappings or waits for manual selection
4. Templates are rendered with environment-specific overrides if configured

### Template System
- Templates use `.slc` extension by default
- Support for environment overrides in `env_overrides/{environment}/` directories
- Base templates in `data-dir/`, overrides preserve directory structure
- Go template language with custom functions for dynamic configuration

### Configuration Structure
- `data-dir/`: Root directory for templates and configurations
  - `ipxe/`: iPXE boot scripts
  - `cloud-config/`: Cloud-init configurations  
  - `preseed/`: Debian/Ubuntu preseed files
  - `kickstart/`: CentOS/RHEL kickstart files
  - `static/`: Static files served without templating
  - `mappings.yaml`: IP/hostname to script mappings
- `env_overrides/{env}/`: Environment-specific template overrides

## Testing Strategy

The project uses both unit tests (Go) and integration tests (Python). Integration tests validate the complete HTTP API by starting a real server instance and making requests against expected fixtures.

Integration test fixtures are in `test/integ-test/expected-results/` and test configurations in `test/integ-test/integ-test-configs/`.