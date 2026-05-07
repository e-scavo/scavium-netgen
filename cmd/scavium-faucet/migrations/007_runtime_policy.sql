CREATE TABLE IF NOT EXISTS runtime_policy (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runtime_policy_updated_at ON runtime_policy(updated_at);
