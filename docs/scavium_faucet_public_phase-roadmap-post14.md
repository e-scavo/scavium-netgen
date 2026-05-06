# SCAVIUM Faucet — Post Phase 14 Roadmap

## Current Status

The SCAVIUM Faucet is fully deployed and production-ready on testnet:

- Backend (Go) operational
- nginx reverse proxy (HTTPS)
- TLS with certbot (auto-renew verified)
- systemd service active
- SQLite persistence
- RPC connected (txpool enabled)
- Claims working end-to-end
- Rate limiting active
- Frontend operational (CSP-safe)
- Firewall (UFW) active

---

## Phase 15 — Public Faucet Hardening & Abuse Protection

**Status:** CLOSED. Phase 15 is complete as the faucet abuse-protection baseline for the current public testnet deployment. The phase introduced human verification, durable abuse telemetry, conservative enforcement, and operational retention without changing the existing public API contract or exposing the backend directly.

### 15.1 — Captcha Integration
- hCaptcha or Cloudflare Turnstile
- Frontend widget integration
- Backend verification

### 15.2 — Claim Abuse Signals
- Durable SQLite-backed abuse signal capture
- IP + User-Agent + address + fingerprint correlation
- Captcha, risk, cooldown, rate-limit, budget, and accepted-claim observations
- Non-blocking telemetry layer for later adaptive policy and admin review

### 15.3 — Progressive Abuse Enforcement
- Conservative signal-based enforcement using the Phase 15.2 abuse signal ledger
- Temporary claim rejection when recent negative signals exceed configured IP, address, or fingerprint thresholds
- Runtime-configurable enforcement window and thresholds
- No public API contract changes; reuses existing `claim_rejected` error envelope

### 15.4 — Abuse Operations & Retention
- Configurable abuse signal retention window
- Startup pruning of expired abuse signals
- Internal aggregate summaries by signal kind for future operator surfaces
- Operational tuning documentation before Phase 16 metrics/admin exposure

### 15.close — Phase Closure
- Abuse Protection consolidated as a closed production-ready phase
- 15.1 through 15.4 recorded as cumulative, deploy-compatible increments
- No code, API, schema, nginx, systemd, or runtime configuration changes required for closure
- Phase 16 explicitly starts from the structured abuse dataset, request logs, and operational boundaries already established

---

## Phase 16 — Observability & Operations

**Status:** CLOSED. Phase 16 is complete as the first production observability layer for the public faucet. The phase added request correlation, safe structured claim-flow logs, lightweight runtime counters, an admin-protected metrics endpoint, and richer liveness/readiness payloads without changing the public claim contract or introducing external observability dependencies.

### 16.1 — Structured Logging + Request Correlation
- `X-Request-ID` preserved or generated for every request
- `X-Correlation-ID` accepted and echoed, falling back to the request ID when absent
- Structured JSON access logs now include both identifiers
- Claim acceptance and rejection emit safe, structured diagnostic events
- Request bodies, captcha tokens, raw fingerprints, secrets, addresses, and idempotency-key values are not logged

### 16.2 — Metrics Endpoint + Internal Runtime Counters
- Lightweight in-process runtime counters in `internal/observability`
- Admin-protected `GET /api/v1/admin/metrics` endpoint
- Claims accepted/rejected, captcha failures, rate-limit hits, daily-budget exceedances, faucet-unavailable, claim-unavailable, and risk-rejection counters
- Build metadata and uptime included in the metrics snapshot
- No Prometheus dependency or new external service requirement introduced in this phase

### 16.3 — Health / Readiness Observability Enrichment
- `/health` now includes uptime and build metadata while remaining a liveness endpoint
- `/ready` keeps the existing DB, queue, RPC, and wallet probes and now includes per-check duration plus aggregate summary counts
- Dry-run readiness continues to use DB and queue checks only
- Non-dry-run readiness continues to include RPC and wallet checks

### 16.4 — Operational Documentation + Observability Closure
- Phase 16 endpoints, headers, logs, runtime counters, and operator commands documented cumulatively
- Existing trunk documentation extended without rewriting historical Phase 14/15 deployment and abuse-protection context
- Phase 17 can now start from an observable and operator-readable faucet baseline

---

## Phase 17 — Token Faucet Extension

### 17.1 — Multi-token Support
**Status:** IMPLEMENTED in the Phase 17.1 foundation pass. The claim contract remains backward-compatible: clients may omit `token_id` and receive the configured default native token exactly as before.

- Config-driven token catalog via `SCAVIUM_FAUCET_TOKENS_JSON`
- Backward-compatible native-token fallback from `SCAVIUM_FAUCET_SYMBOL` and `SCAVIUM_FAUCET_AMOUNT_WEI`
- Optional `token_id` claim input
- Token metadata persisted with each claim and transaction
- ERC20 transfer path prepared in `chain.EthSender` using `transfer(address,uint256)` calldata
- SQLite migration `004_token_claim_metadata.sql` adds token fields without altering existing claim statuses or error codes

### 17.2 — Token Registration (testnet)
**Status:** CLOSED. The faucet exposes a public token catalog derived from validated runtime token configuration, and operators now have a documented testnet registration path for native and ERC20 assets without introducing runtime mutation or changing the claim contract.

#### 17.2.1 — Public Token Catalog Endpoint
**Status:** IMPLEMENTED.

Implemented:
- `GET /api/v1/tokens`
- `GET /api/v1/faucet/tokens`
- Public response envelope: `{ "tokens": [...] }`
- Native and ERC20 metadata exposure using the Phase 17.1 token model
- GET-only method handling through the existing `method_not_allowed` error envelope
- No changes to `POST /api/v1/claim`, claim statuses, error codes, idempotency semantics, queue behavior, or admin authentication


#### 17.2.2 — Testnet Token Registration Guidance
**Status:** IMPLEMENTED.

Implemented:
- New `docs/scavium-faucet/token-registration.md` guide for native-only and native + ERC20 testnet registration
- One-line `SCAVIUM_FAUCET_TOKENS_JSON` examples suitable for systemd environment files
- Operator checklist covering ERC20 contract address, decimals, faucet signer gas, ERC20 balances, stable token ids, and default-token validation
- Post-restart validation using `GET /api/v1/tokens` and `GET /api/v1/faucet/tokens`
- Explicit claim test example using `token_id` and `Idempotency-Key`
- Troubleshooting matrix for configuration, startup, catalog, and ERC20 send failures

No runtime code, schema, endpoint, or claim contract change was introduced in 17.2.2.

Closure notes for 17.2:
- Token registration is configuration-driven and restart-applied.
- Public catalog discovery is the canonical post-restart validation path.
- Runtime admin token mutation, database-backed catalogs, frontend token selection, and hot reload remain outside 17.2 and move to later phases when explicitly scheduled.

#### 17.2.close — Token Registration Closure
**Status:** CLOSED.

Implemented:
- Documentary closure of the configuration-driven token registration model
- Consolidated boundary that 17.2 does not add runtime token mutation, DB-backed token catalogs, or frontend selection
- Preserved claim compatibility: omitted `token_id` still resolves to the configured default token
- Confirmed that Phase 17.3 can start from a stable registration/catalog baseline

### 17.3 — Claim Validation Hardening (token-aware)

#### 17.3.1 — Token Validation Layer
**Status:** IMPLEMENTED.

Implemented:
- Strict claim-time token validation before captcha, risk, cooldown, rate-limit, daily-budget, and queue processing
- Unknown `token_id` values are rejected with the existing `claim_rejected` public contract and `invalid_token` reason
- Defensive validation confirms resolved token metadata remains executable before creating/enqueuing a claim
- Persistent abuse signal `invalid_token` records rejected token-selection attempts
- Runtime metrics now expose `claims.invalid_token` without changing the existing admin metrics envelope
- Existing backward compatibility is preserved: omitted `token_id` still resolves to the configured default token


#### 17.3.2 — Token-Aware Abuse & Rate-Limit Scope
**Status:** IMPLEMENTED.

Implemented:
- Cooldown checks during claim creation now resolve against the selected token id
- Rate-limit keys for IP, wallet address, and fingerprint now include the resolved token scope
- Distinct configured tokens no longer consume each other's cooldown/rate-limit windows
- Daily budget enforcement remains token-aware through the existing token budget path
- Public claim contracts and error envelopes remain unchanged

Deferred:
- Frontend token selector UI
- Runtime admin token mutation
- Database-backed token catalogs

#### 17.3.3 — Token-Aware Observability Alignment
**Status:** IMPLEMENTED.

Implemented:
- Admin runtime metrics now include token-scoped claim counters under `tokens`
- Accepted claim metrics are counted against the resolved token id returned by the claim service
- Rejected claim metrics are counted against the supplied token id, or the `default` bucket when the request omitted `token_id`
- Invalid token attempts remain under the existing aggregate `claims.invalid_token` counter and are also visible in the token-scoped bucket
- Claim-flow logs include production-safe token context without exposing wallet addresses, raw fingerprints, captcha tokens, request bodies, secrets, or idempotency-key values

Deferred:
- Durable per-token analytics
- External metrics backends
- Admin UI visualizations

#### 17.3.close — Claim Validation Hardening Closure
**Status:** CLOSED.

Implemented:
- Closed Phase 17.3 as the token-aware claim validation hardening layer
- Consolidated strict token validation, token-scoped enforcement, and token-aware observability as the active claim-intake baseline
- Confirmed that omitted `token_id` remains backward-compatible through the configured default token path
- Confirmed that invalid token selections continue to reuse the existing `claim_rejected` public envelope with `invalid_token` reason
- Kept deferred items out of scope: frontend token selector UI, runtime admin token mutation, database-backed token catalogs, durable per-token analytics, and external metrics backends

Outcome:
- Phase 18 can start from a stable token-aware pipeline suitable for admin/control-plane capabilities without revisiting the public claim contract.

Deferred:
- Frontend token selector UI
- Runtime admin token mutation
- Database-backed token catalogs
- Durable per-token analytics
- External metrics backends

### 17.4 — Token-Aware Frontend Claim Selection

#### 17.4.1 — Public Token Catalog Consumption
**Status:** IMPLEMENTED.

Implemented:
- Public embedded frontend now calls `GET /api/v1/tokens` during startup
- Token selector is rendered from claim-safe catalog metadata
- Selected token id is submitted as optional `token_id` in `POST /api/v1/claim`
- Claim result rendering includes token id/symbol/type and base-unit amount when returned by the API
- Catalog-load failures preserve backward compatibility by hiding the selector and omitting `token_id`

#### 17.4.2 — Token Selector UX Hardening
**Status:** IMPLEMENTED.

Implemented:
- Token catalog loading and fallback states are visible in the embedded frontend
- Selected-token detail cards show claim-safe amount/type/decimals metadata
- Catalog failure preserves default-token claim behavior by omitting `token_id`
- No backend contract or dependency change

#### 17.4.3 — Token Claim Result UX Alignment
**Status:** IMPLEMENTED.

Implemented:
- Claim result panel renders a token-aware summary for accepted and polled claims
- Returned `amount_wei` is formatted using token decimals when metadata is available
- Raw base-unit amount and existing claim details remain visible
- Explorer action copy is clarified without changing explorer URL configuration
- Native/default responses remain backward-compatible

Deferred:
- Advanced token balance display
- Token icons and richer branding metadata
- Runtime admin token mutation
- Database-backed token catalogs


#### 17.4.close — Token-Aware Frontend Closure
**Status:** CLOSED.

Closed scope:
- Closed Phase 17.4 as the browser-facing token selection and claim-result alignment layer
- Confirmed public catalog consumption, selector fallback behavior, selected-token metadata rendering, and token-aware claim-result summaries as the active frontend baseline
- Preserved the existing claim API contract: `token_id` remains optional and default-token fallback remains intact
- Confirmed no backend runtime token mutation, database-backed catalog, token icons, or new frontend dependency is introduced by this closure

Phase 18 can now introduce admin control features on top of a stable token-aware backend and frontend surface.

#### 17.close — Token Support Closure
**Status:** CLOSED.

Closed scope:
- Closed Phase 17 as the complete token-support layer for the current public testnet faucet scope
- Consolidated multi-token backend execution, configuration-driven registration, public catalog discovery, token-aware claim validation, token-scoped enforcement, token-aware metrics, and embedded frontend token selection/result presentation
- Preserved the existing public claim contract: `token_id` remains optional and omitted values continue to use the configured default token
- Kept runtime token mutation, database-backed catalogs, durable per-token analytics, token icons, and richer admin-control capabilities outside Phase 17

Outcome:
- Phase 18 can start from a stable token-aware backend, API, operations, and frontend baseline.



#### 17.5.close — Post-Audit Fix Closure
**Status:** CLOSED.

Closed scope:
- Closed the Phase 17 post-audit correction pass after the full token-support audit
- Confirmed the frontend status banner now reads the backend `status` field instead of the non-existent `mode` field
- Confirmed cooldown UI copy uses the existing `retry_after_seconds` error detail
- Confirmed accepted and rejected legacy-client claims that omit `token_id` now land in the same `default` token metrics bucket
- Confirmed user-supplied token ids are defensively sanitized before rejection logs and token-scoped metrics paths

Validation:
- `go test ./...` passes after the post-audit fixes

Outcome:
- Phase 18 can start from an audited and corrected token-aware backend, API, operations, and frontend baseline.

---

## Phase 18 — Admin & Control Plane
**Status:** CLOSED after post-audit fixes and documentation closure.

Implemented closure scope:

### 18.1 — Admin Metrics Expansion
- Expanded admin metrics with runtime/process visibility
- Preserved existing admin metrics response compatibility by only adding fields
- Kept counters process-local and diagnostic rather than durable accounting

### 18.2 — Admin Runtime Visibility
- Added `GET /api/v1/admin/runtime` as a composite admin view
- Aggregates dashboard, readiness, metrics, and response time in one operator-safe response
- Does not change public claim/status contracts

### 18.3 — Admin Queue Visibility
- Added `GET /api/v1/admin/queue` for operator queue snapshots
- Exposes counts, ready/delayed/in-flight/pending/terminal summaries, and limited admin-safe items
- Omits wallet addresses, idempotency keys, captcha tokens, request bodies, and transaction internals

### 18.4 — Admin Queue Control
- Added `POST /api/v1/admin/queue/retry` and `POST /api/v1/admin/queue/cancel` request surfaces
- Reused existing admin claim-control semantics and error mapping
- Kept the public `/api/v1/claim` contract unchanged

### 18.5 — Admin Operational Audit Trail
- Added structured admin audit logs for sensitive admin actions
- Avoids logging admin bearer tokens, raw blocklist values, request bodies, captcha tokens, and secrets
- Covers mode changes, queue/claim retry/cancel, and blocklist add/remove

### 18.6 — Admin Control Closure Audit
- Tightened mode validation to `active`, `paused`, and `maintenance`
- Rejected invalid mode changes with `400 invalid_mode`
- Added tests for validation and HTTP error behavior

### 18.7 — Post-Audit Admin Fixes
- Propagated accepted admin mode changes into the live faucet runtime
- Switched admin audit actor attribution to trusted-proxy-aware real IP handling
- Capped queue list limits to avoid unbounded admin responses

### 18.8 — Final Closure Notes/Fixes
- Aligned admin API documentation with the real queue response shape
- Documented the then-current in-memory scope for queue/claim control and blocklist surfaces (later closed in Phase 20)
- Validated blocklist `key_type` against `ip`, `address`, and `fingerprint`
- Capped admin claims and audit list limits at `500`

Deferred beyond Phase 18:
- Dynamic budget/config editing
- CSV/export workflows
- Allowlist/campaign management
- SQLite-backed admin claim control and durable admin audit persistence
- Runtime token mutation and database-backed token catalogs

---

## Phase 19 — Production Hardening (Closed)

Phase 19 was completed as a conservative production-hardening pass rather than a mainnet feature-expansion phase. The closed scope intentionally preserves the public API, admin contracts, SQLite schema, deployment topology, and dependency footprint.

### 19.1 — HTTP Security Headers / Hardening Pass
- Backend security headers are applied uniformly across API, admin, health/readiness, and frontend responses.
- HSTS remains owned by the TLS-terminating reverse proxy.

### 19.2 — Rate-Limit Refinements / Abuse Edge Cases
- Rate-limit scopes are built from non-empty canonical values.
- Fingerprint scope is normalized.
- Public/admin abuse reasons avoid exposing raw limiter keys.

### 19.3 — Performance Safety / Request Body & Timeout Hardening
- Server timeouts and header caps are explicit.
- JSON write request bodies are limited to `1 MiB` before routing when possible and during decode for streaming bodies.

### 19.4 — Operational Resilience / Graceful Shutdown & Runtime Safety
- `SIGINT`/`SIGTERM` use bounded graceful shutdown.
- Application cleanup is idempotent and closes owned resources once.

### 19.5 — Final Production Hardening Audit / Documentation Closure
- Documentation is aligned with the actual implemented Phase 19 scope.
- Deferred items remain explicit: RPC failover, hot/cold-wallet automation, funding refill workflows, durable admin audit persistence, and SQLite-backed admin claim/control surfaces.

---

## Phase 20 — SQLite-backed Admin State and Enforcement (Closed)

Phase 20 is the next required roadmap phase after Phase 19 closure. Its purpose is to remove the largest documented Phase 18 limitation: queue visibility/control, claim lookup/control, blocklist management, and admin audit history currently operate through the production-safe in-memory admin service. Phase 20 must move those operator surfaces onto SQLite-backed production state while preserving public contracts and existing admin endpoint shapes.

Closure status:

- 20.1: admin claim listing/detail and admin queue snapshots read persisted SQLite claim/queue state.
- 20.2: retry/cancel controls persist eligible transitions in SQLite claim state.
- 20.3: admin audit history is persisted and read from SQLite.
- 20.4: admin blocklist entries are persisted and enforced in public claim intake for `ip`, `address`, and `fingerprint` scopes.
- 20.5: documentation and roadmap alignment completed with deferred items explicitly carried forward.

Delivered closure scope:

- SQLite-backed admin claim listing and detail lookup.
- SQLite-backed queue snapshots derived from persisted claim/queue state.
- SQLite-backed retry and cancel control for eligible persisted claims.
- Durable admin audit history.
- Persisted blocklist storage and claim-path enforcement for IP, address, and fingerprint.
- Documentation that distinguishes durable Phase 20 behavior from still-deferred dynamic config, campaigns, runtime token mutation, and multi-instance control.

---

## Phase 21 — Operator Observability and Alerting Baseline (Planned)

Phase 21 must turn the existing Phase 16/19 observability baseline into operator feedback loops suitable for public operation without requiring a heavy external stack.

Required closure scope:

- Stable protected metrics export plan, preferably Prometheus-compatible without dependency bloat.
- Queue, worker, watcher, blockchain, abuse, blocklist, and token metrics aligned with real behavior.
- Alert threshold documentation for low balance, RPC failure, stuck queue, high failure rate, captcha spike, and blocklist spike.
- Nginx, journald, request ID, and correlation ID operational guidance.
- Smoke-test commands for operators.

---

## Phase 22 — Blockchain and Runtime Resilience (Planned)

Phase 22 must harden the transaction path and runtime blockchain dependencies for longer production operation.

Required closure scope:

- Conservative RPC failover configuration and tests.
- Wallet balance and nonce runtime visibility for operators.
- Stuck transaction reconciliation controls where safe.
- Reorg/min-confirmation and replacement-policy documentation.
- No high-availability or distributed-lock redesign in this phase.

---

## Phase 23 — Operational Runbooks, Backup/Restore, and Wallet Procedures (Planned)

Phase 23 must make production operation repeatable after the durable admin and observability layers are complete.

Required closure scope:

- SQLite backup/restore scripts or documented commands with dry-run validation.
- Configuration backup checklist.
- Wallet refill and wallet rotation runbooks.
- Deployment rollback verification.
- Production checklist update.

---

## Phase 24 — Post-14 Roadmap Closure Audit (Planned)

Phase 24 must close the post-Phase-14 roadmap before any broader feature-list expansion begins.

Required closure scope:

- Audit Phases 15 through 23 against code, tests, docs, scripts, and deployment templates.
- Confirm all post-14 deferred items are implemented or explicitly moved into the broader feature backlog.
- Update this roadmap with closure status.
- Produce a Copilot audit prompt and manual validation checklist.

---

## Conclusion

The faucet has reached a stable production baseline.

Remaining phases focus on:
- Observability and operator feedback loops
- Feature expansion
- Admin/control-plane maturation
- Mainnet readiness

