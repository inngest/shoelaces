-- name: CreateBootSession :exec
INSERT INTO boot_sessions (
  ref,
  mac,
  ip,
  hostname,
  target,
  environment,
  params_json,
  users_json,
  provisioning_json,
  created_at_unix_nano,
  expires_at_unix_nano
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetBootSession :one
SELECT
  ref,
  mac,
  ip,
  hostname,
  target,
  environment,
  params_json,
  users_json,
  provisioning_json,
  created_at_unix_nano,
  expires_at_unix_nano
FROM boot_sessions
WHERE ref = ?
  AND expires_at_unix_nano > ?;

-- name: DeleteBootSessionsBefore :execrows
DELETE FROM boot_sessions
WHERE expires_at_unix_nano < ?;
