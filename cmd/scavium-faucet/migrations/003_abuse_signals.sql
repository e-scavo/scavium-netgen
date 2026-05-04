CREATE TABLE IF NOT EXISTS abuse_signals (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,
	address TEXT NOT NULL DEFAULT '',
	remote_ip TEXT NOT NULL DEFAULT '',
	fingerprint TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	claim_id TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	score INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_abuse_signals_kind ON abuse_signals(kind);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_address ON abuse_signals(address);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_remote_ip ON abuse_signals(remote_ip);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_fingerprint ON abuse_signals(fingerprint);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_claim_id ON abuse_signals(claim_id);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_created_at ON abuse_signals(created_at);
