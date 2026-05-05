ALTER TABLE requests ADD COLUMN token_id TEXT NOT NULL DEFAULT 'native';
ALTER TABLE requests ADD COLUMN token_symbol TEXT NOT NULL DEFAULT 'SCAV';
ALTER TABLE requests ADD COLUMN token_type TEXT NOT NULL DEFAULT 'native';
ALTER TABLE requests ADD COLUMN token_address TEXT NOT NULL DEFAULT '';
ALTER TABLE requests ADD COLUMN token_decimals INTEGER NOT NULL DEFAULT 18;

CREATE INDEX IF NOT EXISTS idx_requests_token_id ON requests(token_id);

ALTER TABLE transactions ADD COLUMN token_id TEXT NOT NULL DEFAULT 'native';
ALTER TABLE transactions ADD COLUMN token_symbol TEXT NOT NULL DEFAULT 'SCAV';
ALTER TABLE transactions ADD COLUMN token_type TEXT NOT NULL DEFAULT 'native';
ALTER TABLE transactions ADD COLUMN token_address TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN token_decimals INTEGER NOT NULL DEFAULT 18;

CREATE INDEX IF NOT EXISTS idx_transactions_token_id ON transactions(token_id);
