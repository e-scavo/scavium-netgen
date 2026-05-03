CREATE TABLE IF NOT EXISTS requests (
	id TEXT PRIMARY KEY,
	address TEXT NOT NULL,
	amount_wei TEXT NOT NULL,
	status TEXT NOT NULL,
	reason TEXT NOT NULL DEFAULT '',
	idempotency_key TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_requests_address ON requests(address);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);

CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id TEXT NOT NULL,
	tx_hash TEXT NOT NULL UNIQUE,
	from_address TEXT NOT NULL,
	to_address TEXT NOT NULL,
	value_wei TEXT NOT NULL,
	status TEXT NOT NULL,
	block_number INTEGER NOT NULL DEFAULT 0,
	gas_used INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(request_id) REFERENCES requests(id)
);

CREATE INDEX IF NOT EXISTS idx_transactions_request_id ON transactions(request_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);

CREATE TABLE IF NOT EXISTS rate_limits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	limit_key TEXT NOT NULL,
	window_start TEXT NOT NULL,
	window_seconds INTEGER NOT NULL,
	count INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(limit_key, window_start, window_seconds)
);

CREATE INDEX IF NOT EXISTS idx_rate_limits_key ON rate_limits(limit_key);
CREATE INDEX IF NOT EXISTS idx_rate_limits_window_start ON rate_limits(window_start);

CREATE TABLE IF NOT EXISTS config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
