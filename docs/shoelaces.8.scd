shoelaces(8)

# NAME

shoelaces - automated server bootstrapping

# SYNOPSIS

*shoelaces* [options...]

# OPTIONS

*--base-url* <string>
	Optional parameter. Specifies the base address that will be used when
	generating URLs.
	If it's not specified, the value of "--bind-addr" will be used.

*--bind-addr* <host:port>
	The address where Shoelaces will listen for requests. Defaults to
	"localhost:8081".

*--config* <config>
	Specifies a config file. Most options can be specified in the config;
	*--debug* is CLI-only.

*--data-dir* <directory>
	Specifies a directory with mappings, configs, templates, etc.

*--debug*
	Enables debug mode. This flag is CLI-only; use *log-level = "debug"* in
	config files.

*--env-dir* <directory>
	Specifies a directory with environment overrides. Refer to the README of
	the project for more information about environment overrides.

*--mappings-file* <file>
	Specifies a mappings YAML file. Defaults to "mappings.yaml". Refer to the
	README of the project for more information about mappings.

*--ui-dir* <directory>
	Optional parameter. Specifies a custom UI directory containing web
	templates and frontend assets. If omitted, Shoelaces uses UI assets
	embedded in the binary.

*--static-dir* <directory>
	Deprecated alias for "--ui-dir". This option is only for UI assets and
	does not control provisioning static files.

*--template-extension* <extension>
	Shoelaces template extension. Defaults to ".slc".

*--tftp-enabled*
	Enables the embedded TFTP server. Disabled by default.

*--tftp-addr* <host:port>
	The embedded TFTP server UDP listen address. Defaults to ":69".

*--tftp-root* <directory>
	Directory to serve with the embedded TFTP server. Defaults to
	"data-dir/tftp" when left unset.

*--tftp-readonly*
	Disables TFTP uploads. Defaults to true.

*--tftp-timeout* <duration>
	Per-request embedded TFTP timeout. Defaults to "5s".

*--version*
	Prints version information and exits.

# DESCRIPTION

Shoelaces serves over HTTP iPXE boot scripts, cloud-init configuration, and
any other configuration files to baremetal or virtual machines booting
using iPXE. It also does a few other things to make easier managing server
deployments:

- Has a simple but nice UI to show the current configuration, and history
  of servers that booted.
- Uses Go templates to handle complex configurations.
- Allows specifying the boot target for a given server based on its MAC
  address, IP address, DNS PTR record, or CIDR network.
- Supports the notion of environments for Development and Production
  environment configurations, while trying to minimize template
  duplication.
- Puts unknown servers into iPXE script boot retry loop, while at the same
  time showing them in the UI allowing the user to select a specific boot
  configuration.

Shoelaces is better when used along a DHCP server. Refer to the
*CONFIGURATION* section for examples on how to configure a DHCP server for
hooking it with Shoelaces.

# CONFIGURATION

Shoelaces searches for a config file whenever the *--config* flag is
specified.

Options can be specified as command-line flags, environment variables, or
config file entries. Command-line flags have the highest precedence, then
environment variables, then config file values, then defaults. Environment
variable names are uppercase flag names with hyphens converted to underscores,
such as *DATA_DIR*, *BIND_ADDR*, and *TFTP_ENABLED*.

Config files can be TOML, YAML, or JSON. The parser is selected from the file
extension: ".toml", ".yaml", ".yml", or ".json". Nested TFTP settings use a
"tftp" object/table and map to the "tftp-*" CLI flags.

The Shoelaces web UI templates and frontend assets are embedded in the binary
by default. Use *--ui-dir* only when intentionally overriding those UI assets.
Generic provisioning templates are also embedded. Disk templates under
"data-dir" override embedded provisioning templates, and disk environment
overrides under "data-dir/env_overrides/<env>" override both. Provisioning
static files are separate: files under "data-dir/static" are served first from
the "/configs/static/" URL path, then embedded generic provisioning static
defaults are used as a fallback. The UI "/static/" path never serves
provisioning files.

"mappings.yaml" remains external site policy. It is not embedded and must be
provided through *--data-dir*. See "docs/provisioning-overrides.md" in the
source tree for full-template overrides, structured provisioning fields, and
*installer.extraTemplate* snippets for native installer behavior such as Debian
late_command, kickstart %post, or cloud-init runcmd.

Here is an example TOML config file:

```
bind-addr = "localhost:8081"
data-dir = "/etc/shoelaces/data-dir/"
template-extension = ".slc"
mappings-file = "mappings.yaml"
log-level = "debug"

[tftp]
enabled = true
address = ":69"
root = "/var/lib/shoelaces/tftp"
readonly = true
timeout = "5s"
```

Equivalent examples are kept in *configs/shoelaces.toml*,
*configs/shoelaces.yaml*, and *configs/shoelaces.json*.

# MAPPINGS

Shoelaces reads runtime boot policy from the YAML file selected by
*--mappings-file*, relative to *--data-dir*. The mapping file defines named
*targets* and then references those targets from *networkMaps*, *hostnameMaps*,
*macMaps*, and *ipMaps*.

Each target has a required *script*, optional *label*, optional *environment*,
structured provisioning fields, and optional escape-hatch *params*. Mapping
rules can define *targets* to restrict manual choices and *defaultTarget* to
select a target automatically. If a matched rule has no *defaultTarget*,
Shoelaces queues the host for manual choice in the UI and only exposes the
rule's allowed targets.

Prefer structured sections such as *repos*, *storage*, *boot*, *packages*,
*network*, *locale*, *time*, *users*, and *installer* for built-in provisioning
behavior. *params* remains available for custom site templates, low-level
compatibility values, and explicit manual/request overrides. Parameter merge
order for *params* is global defaults, target params, matched mapping params,
manual/request params, then generated values such as *hostname* and *baseURL*.
Sensitive params can be loaded explicitly from the Shoelaces process environment:

```
params:
  root_password_crypted:
    env: SHOELACES_ROOT_PASSWORD_CRYPTED
```

When a built-in field is present in both structured config and legacy mapping
params, structured mapping policy is canonical for mapping-resolved boots.
Explicit manual/request params can still override for one-off renders.

The old *networkMaps[].script* and *hostnameMaps[].script* mapping schema is no
longer supported. Define named targets and reference them from mapping rules
instead. See *configs/data-dir/mappings.yaml* for a complete example including
Debian and Ubuntu targets.

*dhcpd*(8) can be hooked with Shoelaces by adding the following snippet in
*dhcpd.conf*(5):

```
# dhcp.conf
next-server <your-tftp-server>;
if exists user-class and option user-class = "iPXE" {
  filename "http://<shoelaces-server>/start";
} else {
  filename "undionly.kpxe";
}
```

Similarly, *dnsmasq*(8) can be configured for working with Shoelaces with
the following snippet:

```
dhcp-match=set:ipxe,175 # iPXE sends a 175 option.
dhcp-boot=tag:!ipxe,undionly.kpxe
dhcp-boot=http://<shoelaces-server>/start
```

A TFTP server such as *tftpd*(8), or Shoelaces' embedded TFTP server, must be
configured to serve the iPXE executable, *undionly.kpxe*.

# SEE ALSO

*dhcpd*(8) *dhcpd.conf*(5) *dnsmasq*(8) *tftpd*(8)

# AUTHORS

Maintained by ThousandEyes Inc. <opensource@thousandeyes.com>.

Up-to-date sources can be found at https://github.com/inngest/shoelaces.

Bugs reports and patches can be submitted via GitHub's interface or
contacting the maintainers directly via email.
