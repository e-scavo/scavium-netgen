# Phase 29 — Campaigns, Allowlists, and Invitation Codes

## Completion status

Phase 29 is implemented as a production-safe, single-instance SQLite increment. It adds durable campaign distribution controls without changing the legacy public claim contract: clients that do not send campaign fields continue to claim through the normal faucet policy path.

## Implemented behavior

- Durable SQLite schema for:
  - campaigns
  - campaign allowlist entries
  - invitation codes
  - claim-level `campaign_id` and `invitation_code` attribution
- Public claim integration:
  - optional `campaign_id`
  - optional `invitation_code`
  - legacy clients remain backward compatible
  - private campaign failures use a generic `invalid_campaign` rejection reason
- Campaign scopes:
  - `public`: any otherwise eligible claimant can use the campaign
  - `invite`: claimant must provide a valid enabled invitation code for the campaign
  - `allowlist`: claimant wallet must be explicitly allowlisted for the campaign
- Campaign controls:
  - campaign enable/disable state
  - optional campaign start/end windows
  - optional token scoping
  - optional campaign budget enforced against persisted claim usage
- Admin API:
  - `GET /api/v1/admin/campaigns`
  - `POST /api/v1/admin/campaigns`
  - `POST /api/v1/admin/campaigns/{id}/disable`
  - `GET /api/v1/admin/campaigns/export.csv`
  - `POST /api/v1/admin/invitations`
  - `POST /api/v1/admin/allowlist`
- Admin mutation audit:
  - campaign create
  - campaign disable
  - invitation create
  - allowlist add
- CSV export:
  - standard-library CSV generation
  - bounded admin list limit
  - spreadsheet-formula injection hardening for user-controlled string fields

## Runtime ordering

Campaign validation runs after token and blocklist validation and before captcha/risk/cooldown/rate-limit/claim persistence. Invitation code consumption occurs only for newly created invite-scoped claims after durable claim creation succeeds; idempotent replays return the existing claim without consuming another invitation use.

## Security and privacy notes

- Admin routes remain protected by the existing bearer-token middleware.
- Public errors do not reveal whether a private allowlist entry or invitation campaign exists.
- Campaign IDs and token IDs are sanitized at service boundaries and never used as unbounded metric labels.
- CSV export prefixes dangerous leading spreadsheet characters with a single quote.

## Fix 1 verification notes

The Phase 29 fix pass verified the runtime/admin fallback path in addition to the SQLite-backed production path. `InMemoryAdminService` now preserves campaign state across create/list/disable calls, validates invitation and allowlist references against known campaigns, stores invitation codes and allowlist entries for standalone/test wiring, and keeps the same audit actions as the durable admin service. This prevents fallback behavior from appearing successful while losing Phase 29 campaign controls.

## Fix 2 verification notes

The second Phase 29 fix pass verified durable admin mutation/audit consistency. SQLite-backed campaign create, campaign disable, invitation creation, and allowlist insertion now use best-effort rollback primitives if durable audit append fails after the mutation, mirroring the Phase 28 runtime-policy safety rule that unaudited admin changes must not remain active. Focused admin tests cover rollback for campaign creation, campaign disable, invitation creation, and allowlist insertion, while SQLite store helpers provide the rollback primitives used by the admin service.

## Deferred items

The following remain intentionally deferred as Stage 4/professional-scale features:

- distributed campaign coordination
- HA/distributed locks
- external CRM integrations
- advanced segmentation beyond public/invite/allowlist scopes
- tamper-evident audit chains
- full reporting suite
