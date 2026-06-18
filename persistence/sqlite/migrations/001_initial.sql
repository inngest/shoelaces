-- +goose Up
CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type INTEGER NOT NULL,
  occurred_at_unix_nano INTEGER NOT NULL,
  mac TEXT NOT NULL,
  ip TEXT NOT NULL DEFAULT '',
  hostname TEXT NOT NULL DEFAULT '',
  boot_type TEXT NOT NULL DEFAULT '',
  script TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  params_json BLOB NOT NULL DEFAULT X'7B7D'
);

CREATE INDEX IF NOT EXISTS events_occurred_at_idx
  ON events (occurred_at_unix_nano, id);

CREATE INDEX IF NOT EXISTS events_mac_idx
  ON events (mac, occurred_at_unix_nano, id);

CREATE TABLE IF NOT EXISTS server_states (
  mac TEXT PRIMARY KEY,
  ip TEXT NOT NULL DEFAULT '',
  hostname TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL DEFAULT '',
  environment TEXT NOT NULL DEFAULT '',
  params_json BLOB NOT NULL DEFAULT X'7B7D',
  users_json BLOB NOT NULL DEFAULT X'7B7D',
  provisioning_json BLOB NOT NULL DEFAULT X'7B7D',
  allowed_targets_json BLOB NOT NULL DEFAULT X'5B5D',
  retry INTEGER NOT NULL DEFAULT 0,
  last_access_unix_nano INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS server_states_last_access_idx
  ON server_states (last_access_unix_nano);

CREATE TABLE IF NOT EXISTS boot_sessions (
  ref TEXT PRIMARY KEY,
  mac TEXT NOT NULL,
  ip TEXT NOT NULL DEFAULT '',
  hostname TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL DEFAULT '',
  environment TEXT NOT NULL DEFAULT '',
  params_json BLOB NOT NULL DEFAULT X'7B7D',
  users_json BLOB NOT NULL DEFAULT X'7B7D',
  provisioning_json BLOB NOT NULL DEFAULT X'7B7D',
  created_at_unix_nano INTEGER NOT NULL,
  expires_at_unix_nano INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS boot_sessions_mac_idx
  ON boot_sessions (mac);

CREATE INDEX IF NOT EXISTS boot_sessions_expires_at_idx
  ON boot_sessions (expires_at_unix_nano);

-- +goose Down
DROP INDEX IF EXISTS boot_sessions_expires_at_idx;
DROP INDEX IF EXISTS boot_sessions_mac_idx;
DROP TABLE IF EXISTS boot_sessions;
DROP INDEX IF EXISTS server_states_last_access_idx;
DROP TABLE IF EXISTS server_states;
DROP INDEX IF EXISTS events_mac_idx;
DROP INDEX IF EXISTS events_occurred_at_idx;
DROP TABLE IF EXISTS events;
