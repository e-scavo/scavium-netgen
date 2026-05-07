ALTER TABLE requests ADD COLUMN campaign_id TEXT NOT NULL DEFAULT '';
ALTER TABLE requests ADD COLUMN invitation_code TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_requests_campaign_id ON requests(campaign_id);

CREATE TABLE IF NOT EXISTS campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token_id TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL CHECK (scope IN ('public', 'invite', 'allowlist')),
    budget_wei TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    starts_at TEXT,
    ends_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_campaigns_enabled ON campaigns(enabled);
CREATE INDEX IF NOT EXISTS idx_campaigns_token_id ON campaigns(token_id);

CREATE TABLE IF NOT EXISTS campaign_allowlist (
    campaign_id TEXT NOT NULL,
    address TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(campaign_id, address),
    FOREIGN KEY(campaign_id) REFERENCES campaigns(id)
);

CREATE INDEX IF NOT EXISTS idx_campaign_allowlist_address ON campaign_allowlist(address);

CREATE TABLE IF NOT EXISTS invitation_codes (
    code TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL,
    max_uses INTEGER NOT NULL DEFAULT 1,
    uses INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(campaign_id) REFERENCES campaigns(id)
);

CREATE INDEX IF NOT EXISTS idx_invitation_codes_campaign_id ON invitation_codes(campaign_id);
CREATE INDEX IF NOT EXISTS idx_invitation_codes_enabled ON invitation_codes(enabled);
