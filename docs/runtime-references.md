# Runtime References

Shoelaces persists runtime data so later requests can reuse boot decisions
without carrying full structured config in URLs.

## Reference Types

| Record | Lookup | User-facing API | Notes |
|--------|--------|-----------------|-------|
| Boot sessions | `ref` | `GET /ajax/boot-sessions/{ref}` or `shoelaces boot-sessions get <ref>` | Opaque boot/config references used by `/configs/*?ref=...`. Operator lookups return redacted metadata only. |
| Event history | event ULID | `GET /ajax/events/{id}`, `GET /ajax/events`, `shoelaces events list`, or `shoelaces events get <event-id>` | Events are persisted with redacted params before storage. |
| Waiting/manual server state | MAC address | `GET /ajax/servers`, `shoelaces servers list`, or `shoelaces servers get <mac>` | Shows persisted waiting and selected state. Selected state is reused internally on the next poll. |
| Host observations | MAC/IP/hostname in events and state | Existing event/server APIs | No separate host inventory API exists yet. |

## Redaction

`/configs/*` is the only route that resolves a boot `ref` into the full stored
template context. UI/AJAX APIs must use redacted views:

- sensitive params such as tokens, passwords, private keys, and `boot_ref`
  values are returned as `[REDACTED]`;
- structured users keep useful non-secret fields but redact password hashes and
  SSH authorized keys;
- structured provisioning keeps useful installer context but redacts nested
  sensitive values such as `storage.encryption.passphrase`.

Secret-bearing installer values are still present in the persisted boot-session
record until the ref expires or is swept. A missing or expired ref returns
`404 boot reference not found`; the host should poll again to receive a fresh
boot script and `/configs/*?ref=...` URL.

The current HTTP API assumes the same operator trust boundary as the existing
Shoelaces UI. It does not add per-reference authorization.
