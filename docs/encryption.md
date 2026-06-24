# Storage Encryption

Shoelaces supports structured Debian LUKS installs through
`storage.encryption`. Embedded encryption rendering is Debian-only; other
installer families should use custom templates until they have native support.
The regular plain-LUKS and TPM unlock paths are currently verified on Debian 13;
other OS installers have not been tested for this behavior yet.

## Basic LUKS

Use `storage.encryption.enabled: true` with a passphrase. Prefer an environment
reference so secrets stay out of `mappings.yaml`.

```yaml
storage:
  mode: regular
  encryption:
    enabled: true
    passphrase:
      env: SHOELACES_LUKS_HOST_IAD_20_PASSPHRASE
```

Optional LUKS parameters:

- `cipher`: defaults to `aes-xts-plain64`.
- `keySize`: defaults to `512`.
- `hash`: defaults to `sha512`.

Debian `regular` encrypted storage creates ESP, `/boot`, and one LUKS root
partition. `/` is formatted directly on the opened mapper, and swap is created
as `/swapfile` inside encrypted root. Debian `lvm` and `raid` encrypted modes
remain supported, but TPM unlock currently applies only to `regular`.

## TPM Unlock

TPM-backed unlock is configured under `storage.encryption.tpm`:

```yaml
storage:
  mode: regular
  encryption:
    enabled: true
    passphrase:
      env: SHOELACES_LUKS_HOST_IAD_20_PASSPHRASE
    tpm:
      enabled: true
      device: auto
      pcrs: "7"
      requireSha256Bank: true
      initramfs: dracut
```

When TPM unlock is enabled, Shoelaces keeps the Debian preseed `late_command`
small. The preseed downloads a ref-scoped generated helper from
`/configs/generated/debian/luks-tpm-setup.sh?ref=...`, executes it, removes it,
and then appends any `installer.lateCommands`.

The generated installer helper:

- installs first-boot TPM reenrollment dependencies: `cryptsetup-initramfs`,
  `systemd-cryptsetup`, `dracut-core`, `dracut-config-generic`, `tpm2-tools`,
  and `util-linux`;
- verifies `systemd-cryptenroll --tpm2-device=list` sees a TPM device;
- fails install when `requireSha256Bank: true` and
  `/sys/class/tpm/tpm0/pcr-sha256` is missing;
- fetches the resolved LUKS passphrase from the boot-reference-scoped generated
  passphrase endpoint;
- enrolls a temporary TPM token with `systemd-cryptenroll --tpm2-pcrs=` so the
  first installed disk boot can unlock unattended;
- writes `tpm2-device=<device>` into `/etc/crypttab`;
- builds a host-only dracut initramfs with systemd cryptsetup and TPM2 modules;
- creates a GRUB entry for the dracut image and makes it the default;
- installs a dedicated Shoelaces TPM re-enrollment service and stores a
  root-only copy of the passphrase at
  `/var/lib/shoelaces/luks-tpm.passphrase` inside the encrypted target root.

The temporary installer enrollment intentionally has no PCR binding. PCR 7
during PXE installer boot can differ from PCR 7 during installed disk boot, so
the final PCR-bound token is created by
`shoelaces-luks-tpm-reenroll.service`. On the first installed boot, the
generated service runs a local `reenroll-luks-tpm` phase before network startup
and before a downstream `firstboot.service` if one is present. It uses
Shoelaces-specific paths so site-provided firstboot units, defaults, and
scripts can coexist. If `/var/lib/shoelaces/luks-tpm.passphrase` exists, the
reenrollment service:

- resolves the current root mapper and backing LUKS device;
- wipes the temporary `systemd-tpm2` token;
- enrolls a new TPM token with `--tpm2-pcrs="$SHOELACES_LUKS_TPM_PCRS"`;
- verifies `systemd-tpm2`, `tpm2-hash-pcrs`, and SHA256 PCR bank metadata in
  `cryptsetup luksDump`;
- deletes `/var/lib/shoelaces/luks-tpm.passphrase`.

Defaults:

```text
SHOELACES_LUKS_TPM_PASSPHRASE_FILE=/var/lib/shoelaces/luks-tpm.passphrase
SHOELACES_LUKS_TPM_PCRS=7
```

Debian initramfs-tools does not honor `tpm2-device=auto` for this flow, so
`initramfs: dracut` is required. Shoelaces still builds the standard
passphrase-capable initramfs as a fallback.

## Secret Handling

Preseeded encryption is unattended, so the rendered installer config contains
the LUKS passphrase. Use boot references and environment-backed passphrases for
real installs. Shoelaces redacts `storage.encryption.passphrase` in logs and
events. The TPM installer helper writes the passphrase to a `0600` temporary
file under `/target/run`, removes that transient file on exit, and does not pass
the passphrase as a command-line argument. For the two-phase TPM flow it also
leaves `/target/var/lib/shoelaces/luks-tpm.passphrase` at mode `0600`; the
Shoelaces reenrollment service removes that file after successful PCR-bound
reenrollment.

Treat any machine that can fetch installer configs during provisioning as able
to access the install secret for that boot session.
