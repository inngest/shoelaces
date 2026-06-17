# Shoelaces at Inngest

This repository is Inngest's maintained fork of [ThousandEyes Shoelaces](https://github.com/thousandeyes/shoelaces).
Shoelaces serves iPXE, configuration, and static provisioning assets for bare-metal provisioning.
At Inngest, the deployed Shoelaces binary is installed by Ansible from S3 release artifacts, so the release workflow publishes both GitHub release artifacts and the S3 compatibility path.

## Building and testing

Use the same Go toolchain selected by `go.mod`.
CI uses `actions/setup-go` with `go-version-file: go.mod`.

Local unit-test, build, and version checks:

```bash
go test ./...
go test -race ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o shoelaces ./cmd/shoelaces
./shoelaces --version
```

The `--version` flag prints the release version, commit, build date, and builder metadata embedded by GoReleaser, which is useful for verifying the exact binary installed on a Shoelaces host.
The `make unit` target runs formatted Go unit tests.
The `make test` target runs `make unit` and then the historical Python integration test at `test/integ-test/integ_test.py`; that integration path may require local Python dependencies that are not installed by the Go toolchain.

CI currently runs:

- `go test -race ./...`
- Linux amd64 binary build from `./cmd/shoelaces`
- `golangci-lint`
- a GoReleaser snapshot dry run

## Proposed release process

Shoelaces follows the same broad release pattern as Atlas: release PRs drive tags, and tags drive GoReleaser.
Every push to `master` creates or updates a `release/next` PR with a SemVer release tag in the PR title.
Merging that release PR creates the tag and publishes GitHub release plus S3 artifacts with GoReleaser.
The important Shoelaces-specific difference is that the current Ansible deployment still consumes the S3 `shoelaces/releases/.../shoelaces.tar.gz` feed, so the release workflow also publishes that compatibility path.

### Automatic releases

Merging ordinary changes to `master` runs the auto-release PR workflow.
It asks `git-cliff` for the next SemVer tag, updates `CHANGELOG.md`, and opens or updates `release/next`.
When `release/next` is merged, the release tag workflow creates the tag from the release PR title and publishes per-platform archives plus `checksums.txt` to the GitHub release with GoReleaser.
It also publishes the same archives and checksum file to S3 using GoReleaser's S3 blob publisher.
S3 publication writes both an immutable version prefix and the mutable `latest` prefix:

```text
s3://inngest-artifacts/shoelaces/releases/<release_tag>/shoelaces_<version>_<os>_<arch>.tar.gz
s3://inngest-artifacts/shoelaces/releases/<release_tag>/shoelaces_<version>_windows_<arch>.zip
s3://inngest-artifacts/shoelaces/releases/<release_tag>/checksums.txt
s3://inngest-artifacts/shoelaces/releases/latest/shoelaces_<version>_<os>_<arch>.tar.gz
s3://inngest-artifacts/shoelaces/releases/latest/shoelaces_<version>_windows_<arch>.zip
s3://inngest-artifacts/shoelaces/releases/latest/checksums.txt
```

After S3 publication, the user should run the Ansible deployment; agents must not run Inngest Ansible.

A follow-up Ansible ticket should switch Shoelaces installation to consume GitHub release artifacts directly.
Until that lands, GitHub releases are the canonical build output, but S3 remains the live deployment source.

## Upstream ThousandEyes README content

The following project overview and usage material is adapted from the upstream [ThousandEyes Shoelaces README](https://github.com/thousandeyes/shoelaces).
Some examples in this fork have diverged for Inngest's current data directory, DHCP, and release workflow.

## **Shoelaces:** lightweight and painless server bootstrapping

**Shoelaces** serves [iPXE](https://ipxe.org/) boot scripts,
[cloud-init](http://cloud-init.org/) configuration, and any other configuration
files over HTTP to hardware or virtual machines booting over iPXE. It also does
a few other things to make it easier to manage your server deployments:

* Has a simple but **nice UI** to show the current configuration, and history of
  servers that booted.
* Uses simple **Go based template language** to handle more complex configurations.
* Allows specifying the **boot entry point** for a given server based on its
  **IP** address or **DNS PTR** record.
* Supports the notion of **environments** for _Development_ and _Production_
  environment configurations, while trying to minimize template duplication.
* Puts unknown servers into iPXE script boot **retry loop**, while at the same
  time **showing them in the UI** allowing the user to select a specific boot
  configuration.

### How it works

As soon as Shoelaces starts, the service will be patiently waiting for servers
to boot. If no servers are detected, you'll simply see a spinner in the web UI,
as can be seen in the screenshot.

![Shoelaces frontend - Waiting for hosts](docs/screenshots/shoelaces-frontend-1.png)

The URL `localhost:8081` will actually point to wherever you are running your
Shoelaces instance. It must be reachable by the booting hosts.

The following picture shows a high level overview of how a server notifies
Shoelaces that it's ready for booting.

![Shoelaces overview](docs/screenshots/shoelaces-overview.png)

In this graph we can see that as soon as the server boots using network boot, we
instruct the machine to switch to an [iPXE](https://ipxe.org/) executable. We do this
because we need to be able to make HTTP requests to Shoelaces, and regular
[PXE](https://en.wikipedia.org/wiki/Preboot_Execution_Environment) does not
support that protocol.

So, when a server boots, the
[DHCP](https://en.wikipedia.org/wiki/Dynamic_Host_Configuration_Protocol) server
will instruct it to retrieve an iPXE executable from a
[TFTP](https://en.wikipedia.org/wiki/Trivial_File_Transfer_Protocol) server.
When the host receives the iPXE executable, it will chainload into it and trigger a new
DHCP request. Finally, the server will detect that the request comes from an
iPXE executable, allowing it to respond with an HTTP URL. This URL, as you may have
guessed, will be pointing to Shoelaces.

If there was no automated installation configured for the booting server, you'll
be able to select an option to bootstrap it in the Shoelaces UI.

![Shoelaces frontend - Host detected](docs/screenshots/shoelaces-frontend-2.png)

A couple of things can be said about this screenshot:

* When you select a task, a bunch of input boxes for filling with parameters
  will appear (in the picture, they are *release* and *hostname*). The
  parameters to complete will be dynamically loaded from the chosen task
  template.

* Hosts send their MAC address when they contact Shoelaces. From the HTTP
  request Shoelaces will extract the source IP and perform a reverse DNS lookup.
  If the DNS query is successful, the resolved hostname will be shown in the web
  UI. If no hostname was resolved, Shoelaces will show just the MAC and the IP.

### Setting up

#### Building Shoelaces

At the moment a binary package is not provided. The only way of running
Shoelaces is to compile it from source. Refer to the Go Programming
Language [Getting Started](https://golang.org/doc/install) guide to learn
how to compile Shoelaces.

Once that you have configured your Go, you can get and compile Shoelaces by
running:

    $ go get github.com/inngest/shoelaces
    $ cd $GOPATH/src/github.com/inngest/shoelaces
    $ go build

#### Running Shoelaces

You can quickly try Shoelaces after compiling it by using the example configuration file:

    ./shoelaces --config configs/shoelaces.toml

Head to [localhost:8081](http://localhost:8081) to checkout Shoelaces' frontend.

#### Shoelaces configuration file

Shoelaces accepts several parameters:

* `config`: the path to a configuration file.
* `bind-addr`: the address Shoelaces listens on. Defaults to `localhost:8081`.
* `base-url`: the base URL used when generating URLs. Defaults to `bind-addr`.
* `data-dir`: the path to the root directory with the templates. It's advised to
  manage the templates in a VCS, such as a git repository. Refer to the [example
  data directory](configs/data-dir/) for more information.
  Provisioning static files are served from `data-dir/static` at
  `/configs/static/*`.
* `ui-dir`: optional path to a custom UI directory containing web templates and
  frontend assets. By default, Shoelaces uses UI assets embedded in the binary.
* `static-dir`: deprecated alias for `ui-dir`, retained for compatibility. Do
  not use it for provisioning static files.
* `env-dir`: the environment overrides directory inside `data-dir`. Defaults to
  `env_overrides`.
* `mappings-file`: the path to the YAML mappings file, relative to the
  `data-dir` parameter. Defaults to `mappings.yaml`.
* `template-extension`: the filename extension for the templates. The default is
  `.slc`, so you can just stick with that.
* `debug`: enable debug messages.
* `tftp-enabled`: enable the embedded TFTP server.
* `tftp-addr`: the embedded TFTP listen address. Defaults to `:69`.
* `tftp-root`: the directory served over TFTP. Defaults to `data-dir/tftp` when
  not explicitly set.
* `tftp-readonly`: disable TFTP uploads. Defaults to `true`.
* `tftp-timeout`: the embedded TFTP per-request timeout. Defaults to `5s`.

The parameters can be specified in a configuration file, as environment
variables, or as parameters when running the Shoelaces binary. Command-line
flags have the highest precedence, then environment variables, then config
file values, then defaults. Environment variable names are uppercase flag names
with hyphens converted to underscores, such as `DATA_DIR`, `BIND_ADDR`, and
`TFTP_ENABLED`.

Configuration files can be TOML, YAML, or JSON. The parser is selected from
the config file extension: `.toml`, `.yaml`, `.yml`, or `.json`. Nested TFTP
settings use a `tftp` object/table and map to the `tftp-*` CLI flags, such as
`tftp.enabled`, `tftp.address`, and `tftp.timeout`.

Example TOML config:

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

Example files are available for [TOML](configs/shoelaces.toml),
[YAML](configs/shoelaces.yaml), and [JSON](configs/shoelaces.json).

#### Extra requirements

Along with your **Shoelaces** installation, you will need a LAN segment with
working [TFTP](https://en.wikipedia.org/wiki/Trivial_File_Transfer_Protocol) and
[DHCP](https://en.wikipedia.org/wiki/Dynamic_Host_Configuration_Protocol)
servers. Any TFTP server should work. The DHCP server will need to be able to
match the `user-class` of the boot client. In our example the configuration is
for the widely used [ISC DHCP Server](https://www.isc.org/downloads/dhcp/).
Shoelaces will happily coexist with the TFTP and DHCP servers on the same host.
The server you are going to bootstrap needs to be capable of booting over the
network using
[PXE](https://en.wikipedia.org/wiki/Preboot_Execution_Environment).

##### TFTP

The TFTP server is only used to chainload the iPXE boot loader, so setting it up
in `read-only` mode is sufficient. The loader we use (`undionly.kpxe`) can be
downloaded from the [ipxe.org](http://ipxe.org/howto/chainloading) website.

It is also possible to compile your own iPXE executable in order to customize the
booting of your servers. For example, it's useful to [add your own SSL
certificates](http://ipxe.org/crypto#trusted_root_certificates) in case you want
to boot using HTTPS.

##### DHCP

Drop this config in your **ISC DHCP** server, replacing the relevant sections
with your TFTP and Shoelaces server addresses.

```txt
# dhcp.conf
next-server <your-tftp-server>;
if exists user-class and option user-class = "iPXE" {
  filename "http://<shoelaces-server>/start";
} else {
  filename "undionly.kpxe";
}
```

For **dnsmasq** (v2.53 or above) you can add this to its existing config, e.g. by
putting it in `dnsmasq.d/ipxe.conf`:

```txt
dhcp-match=set:ipxe,175 # iPXE sends a 175 option.
dhcp-boot=tag:!ipxe,undionly.kpxe
dhcp-boot=http://<shoelaces-server>/start
```

The **${netX/mac:hexhyp}** strings represents the MAC address of the booting
host. iPXE will be in charge of replacing that string for the actual value.

*Note*: In case you are using a DHCP server that does not have this level of
flexibility for configuring it, you can always re-compile the iPXE executable for
[breaking the loop](https://ipxe.org/howto/chainloading#breaking_the_loop_with_an_embedded_script).

### Script discoverability

The purpose of Shoelaces is automation. The less input it receives from the
user, the better. When a server boots, Shoelaces resolves a named boot target
from `mappings.yaml`. A target points at an iPXE template, an optional
environment override, a UI label, and target-specific template parameters.

Mappings can select targets by MAC address, exact IP address, hostname regular
expression, or CIDR network. Match precedence is manual selection, MAC, IP,
hostname, network, then unmatched/manual queue. If a mapping has
`defaultTarget`, Shoelaces boots it automatically. If a mapping only has
`targets`, the host is queued in the UI and operators can choose from that
restricted target list. Unmatched hosts can choose from all configured targets.

Example mapping:

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

  ubuntu2404:
    script: ubuntu-minimal.ipxe
    label: Ubuntu 24.04 LTS
    params:
      release: noble

  ubuntu2604:
    script: ubuntu-minimal.ipxe
    label: Ubuntu 26.04 LTS
    params:
      release: resolute

networkMaps:
  - network: 104.225.9.96/27
    defaultTarget: debian12
    targets:
      - debian12
      - debian13
      - ubuntu2404
      - ubuntu2604
    params:
      hostnamePrefix: iad-

hostnameMaps:
  - hostname: '^debian13-[0-9]+\.example\.com$'
    defaultTarget: debian13
    targets:
      - debian13

macMaps:
  - mac: "0c:42:a1:c3:52:96"
    defaultTarget: ubuntu2604
    targets:
      - ubuntu2604
    params:
      hostname: ubuntu2604-example

ipMaps:
  - ip: 2001:db8::10
    defaultTarget: debian12
    targets:
      - debian12
      - debian13
```

Parameter merge order is global `defaults.params`, selected target `params`,
matched mapping `params`, manual/request params, then generated values such as
`hostname` and `baseURL`. Raw scalar values can be placed directly in params.
Sensitive values can also come from the Shoelaces process environment using an
explicit reference:

```yaml
params:
  root_password_crypted:
    env: SHOELACES_ROOT_PASSWORD_CRYPTED
```

This uses the environment of the running Shoelaces process, so systemd service
environment variables work without a separate env file. Missing referenced
environment variables fail the boot render clearly.

Shoelaces will read these mappings from the YAML file configured by
`mappings-file`, relative to `data-dir`. Refer to the [example mappings
file](configs/data-dir/mappings.yaml) for a complete example. The old
`networkMaps[].script` and `hostnameMaps[].script` schema is no longer
supported; define named `targets` and reference them from mapping rules instead.

### Environments

Shoelaces supports the notion of environments a.k.a. *env overrides*.
Consider the following `data-dir` directory structure:

```txt
├── cloud-config
│   └── coreos-cloud-config.yaml.slc
├── env_overrides
|   └── testing
|       └─── cloud-config
|            └── coreos-cloud-config.yaml.slc
├── ipxe
│   ├── coreos.ipxe.slc
│   └── ubuntu-minimal.ipxe.slc
├── mappings.yaml
├── preseed
│   └── common.preseed.slc
└── static
    ├── bootstrap.sh
    └── rc.local-bootstrap
```

In this case, hosts that resolve to a target with `environment: testing` in
`mappings.yaml` will be assigned the `testing` environment and they'll use the
`coreos-cloud-config.yaml.slc` template from the `env_overrides/testing
directory`, while the rest of the templates will be served from the base
directory. Everything except `mappings.yaml` can be put in `env_overrides/$env`
preserving the path.

The way this works, considering that **Shoelaces** is mostly stateless, is by
setting different `baseURL` depending on the environment set. Normal requests
would get `baseURL` set to `http://$shoelaces_host:$port` while an environment
request will have `http://$shoelaces_host:$port/env/$environment_name/`

*CORNER CASES*: It is not possible to boot a host in a non default environment
unless there is a main iPXE script in the respective override directory. This
means /ipxemenu will only present default and non-default **iPXE** entry points,
and if you have a template that's included later in the boot process as an
override you won't be able to select it.

### Contributing

Contributions to Shoelaces are very welcome! Take into account the following
guidelines:

* [File an issue](https://github.com/inngest/shoelaces/issues) if you find
  a bug or, even better, contribute with a pull request.
* We have a bunch of integration tests that can be run by executing `make test`.
  Ensure that all test pass before submitting your pull request.
