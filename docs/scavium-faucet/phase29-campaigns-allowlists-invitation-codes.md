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
  - `PUT /api/v1/admin/campaigns/{id}`
  - `POST /api/v1/admin/campaigns/{id}/disable`
  - `GET /api/v1/admin/campaigns/export.csv`
  - `POST /api/v1/admin/invitations`
  - `POST /api/v1/admin/allowlist`
- Admin mutation audit:
  - campaign create
  - campaign update
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

## Fix 3 verification notes

The third Phase 29 fix pass verified campaign reference validation and rollback edge cases after the previous audit rollback hardening. SQLite campaign invitation and allowlist writes now explicitly verify that the referenced campaign exists before insertion instead of relying on database foreign-key behavior. Campaign allowlist insertion is idempotent and no longer replaces an existing entry, preventing an unaudited admin retry from changing allowlist metadata if durable audit persistence fails after the mutation. Focused SQLite tests cover missing campaign references and idempotent allowlist insertion.

## Fix 4 verification

The fourth Phase 29 fix pass closed the remaining admin-control gap from the step 29.4 scope: campaigns can now be updated through an audited `PUT /api/v1/admin/campaigns/{id}` path. SQLite and in-memory admin services preserve campaign identity and historical attribution, validate the path/body id consistently, and roll back durable updates if admin audit persistence fails. Tests cover in-memory update behavior, SQLite update persistence, HTTP update wiring, and audit-failure rollback.

## Fix 5 verification

The fifth Phase 29 fix pass closed the HTTP test regression introduced with the audited campaign update endpoint. The campaign update endpoint test now sends a real `CampaignRequest` through the shared admin request helper instead of passing a prebuilt buffer that the helper marshaled as an unrelated JSON object. This keeps the test aligned with the actual public admin API contract and verifies create-then-update wiring end to end without changing runtime behavior.

## Fix 6 verification

The sixth Phase 29 fix pass closed a final invite-claim consistency edge case. If invitation validation succeeds before claim persistence but invitation consumption fails after the durable claim is created, the claim is now immediately marked `rejected` with `invalid_campaign` before returning the public rejection. This prevents an unqueued invite claim from remaining in a received/budget-counting state after a race with a max-use invitation code or another durable consume failure. A focused persistent-service test covers the post-create consume-failure path.

## Deferred items

The following remain intentionally deferred as Stage 4/professional-scale features:

- distributed campaign coordination
- HA/distributed locks
- external CRM integrations
- advanced segmentation beyond public/invite/allowlist scopes
- tamper-evident audit chains
- full reporting suite
