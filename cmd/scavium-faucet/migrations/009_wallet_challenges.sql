CREATE TABLE IF NOT EXISTS wallet_challenges (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    nonce TEXT NOT NULL,
    message TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wallet_challenges_address ON wallet_challenges(address);
CREATE INDEX IF NOT EXISTS idx_wallet_challenges_expires_at ON wallet_challenges(expires_at);
