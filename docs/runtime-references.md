# Runtime References

Shoelaces persists runtime data so later requests can reuse boot decisions
without carrying full structured config in URLs.

## Reference Types

| Record | Lookup | User-facing API | Notes |
|--------|--------|-----------------|-------|
| Boot sessions | `ref` | `GET /ajax/boot-sessions/{ref}` | Opaque boot/config references used by `/configs/*?ref=...`. The API returns redacted metadata only. |
| Event history | event ULID | `GET /ajax/events/{id}` and `GET /ajax/events` | Events are persisted with redacted params before storage. |
| Waiting/manual server state | MAC address | `GET /ajax/servers` | Shows only hosts currently waiting for manual selection. Selected state is reused internally on the next poll. |
| Host observations | MAC/IP/hostname in events and state | Existing event/server APIs | No separate host inventory API exists yet. |

## Redaction

`/configs/*` is the only route that resolves a boot `ref` into the full stored
template context. UI/AJAX APIs must use redacted views:

- sensitive params such as tokens, passwords, private keys, and `boot_ref`
  values are returned as `[REDACTED]`;
- structured users keep useful non-secret fields but redact password hashes and
  SSH authorized keys;
- structured provisioning keeps useful installer context but redacts nested
  sensitive values.

The current HTTP API assumes the same operator trust boundary as the existing
Shoelaces UI. It does not add per-reference authorization.
