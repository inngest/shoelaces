-- name: UpsertServerState :exec
INSERT INTO server_states (
  mac,
  ip,
  hostname,
  target,
  environment,
  params_json,
  users_json,
  provisioning_json,
  allowed_targets_json,
  retry,
  last_access_unix_nano
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(mac) DO UPDATE SET
  ip = excluded.ip,
  hostname = excluded.hostname,
  target = excluded.target,
  environment = excluded.environment,
  params_json = excluded.params_json,
  users_json = excluded.users_json,
  provisioning_json = excluded.provisioning_json,
  allowed_targets_json = excluded.allowed_targets_json,
  retry = excluded.retry,
  last_access_unix_nano = excluded.last_access_unix_nano;

-- name: GetServerState :one
SELECT
  mac,
  ip,
  hostname,
  target,
  environment,
  params_json,
  users_json,
  provisioning_json,
  allowed_targets_json,
  retry,
  last_access_unix_nano
FROM server_states
WHERE mac = ?;

-- name: ListServerStates :many
SELECT
  mac,
  ip,
  hostname,
  target,
  environment,
  params_json,
  users_json,
  provisioning_json,
  allowed_targets_json,
  retry,
  last_access_unix_nano
FROM server_states
ORDER BY mac ASC;

-- name: DeleteServerState :execrows
DELETE FROM server_states
WHERE mac = ?;

-- name: DeleteServerStatesBefore :execrows
DELETE FROM server_states
WHERE last_access_unix_nano < ?;
