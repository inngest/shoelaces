-- name: InsertEvent :one
INSERT INTO events (
  event_type,
  occurred_at_unix_nano,
  mac,
  ip,
  hostname,
  boot_type,
  script,
  message,
  params_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ListEvents :many
SELECT
  id,
  event_type,
  occurred_at_unix_nano,
  mac,
  ip,
  hostname,
  boot_type,
  script,
  message,
  params_json
FROM events
ORDER BY occurred_at_unix_nano ASC, id ASC;

-- name: DeleteEventsBefore :execrows
DELETE FROM events
WHERE occurred_at_unix_nano < ?;
