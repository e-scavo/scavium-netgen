CREATE TABLE IF NOT EXISTS admin_blocklist (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	key_type TEXT NOT NULL CHECK (key_type IN ('ip', 'address', 'fingerprint')),
	key_value TEXT NOT NULL,
	reason TEXT NOT NULL DEFAULT '',
	blocked_at TEXT NOT NULL,
	UNIQUE(key_type, key_value)
);

CREATE INDEX IF NOT EXISTS idx_admin_blocklist_type_value ON admin_blocklist(key_type, key_value);
CREATE INDEX IF NOT EXISTS idx_admin_blocklist_blocked_at ON admin_blocklist(blocked_at);
