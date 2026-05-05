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

### 18.1 — Admin Interface
- Pause/resume faucet
- Stats overview

### 18.2 — Dynamic Budget Control
- Runtime config changes

### 18.3 — Claim Audit
- History
- Export

---

## Phase 19 — Production Readiness (Mainnet)

### 19.1 — RPC Failover
- Multiple node support

### 19.2 — Wallet Hardening
- Hot/cold separation

### 19.3 — Funding Automation
- Auto refill logic

### 19.4 — Security Hardening
- Final production posture

---

## Conclusion

The faucet has reached a stable production baseline.

Remaining phases focus on:
- Observability and operator feedback loops
- Feature expansion
- Admin/control-plane maturation
- Mainnet readiness

