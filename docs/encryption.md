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

When TPM unlock is enabled, Shoelaces:

- installs `dracut-core`, `dracut-config-generic`, and `tpm2-tools`;
- verifies `systemd-cryptenroll --tpm2-device=list` sees a TPM device;
- fails install when `requireSha256Bank: true` and
  `/sys/class/tpm/tpm0/pcr-sha256` is missing;
- enrolls the LUKS backing partition with `systemd-cryptenroll` using a
  temporary root-only passphrase file;
- writes `tpm2-device=<device>` into `/etc/crypttab`;
- builds a host-only dracut initramfs with systemd cryptsetup and TPM2 modules;
- creates a GRUB entry for the dracut image and makes it the default.

Debian initramfs-tools does not honor `tpm2-device=auto` for this flow, so
`initramfs: dracut` is required. Shoelaces still builds the standard
passphrase-capable initramfs as a fallback.

## Secret Handling

Preseeded encryption is unattended, so the rendered installer config contains
the LUKS passphrase. Use boot references and environment-backed passphrases for
real installs. Shoelaces redacts `storage.encryption.passphrase` in logs and
events, and the TPM enrollment path writes the passphrase only to a `0600`
temporary file under `/target/run`, removes it after enrollment, and does not
pass it as a command-line argument.

Treat any machine that can fetch installer configs during provisioning as able
to access the install secret for that boot session.
