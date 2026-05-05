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
**Status:** ADVANCED through 17.2.2. The faucet exposes a public token catalog derived from validated runtime token configuration, and operators now have a documented testnet registration path for native and ERC20 assets without introducing runtime mutation or changing the claim contract.

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

Remaining for 17.2:
- Optional live testnet deployment notes once real deployed token ids/addresses are selected
- Optional final 17.2 closure after operator validation on the VPS

### 17.3 — Frontend Token Selection
- Token selector UI
- Dynamic routing

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
