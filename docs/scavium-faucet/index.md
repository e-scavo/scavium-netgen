# scavium-faucet

`scavium-faucet` is the faucet service that currently ships from this repository. The binary is a single Go HTTP server with an embedded web UI, public JSON endpoints, request IDs, structured logs, SQLite-backed persistent claim state, a background worker, and real readiness probes.

This directory documents the **implemented project surface**, not the full roadmap. The broader feature backlog remains in [`docs/scavium_faucet_public_features.md`](../scavium_faucet_public_features.md), which stays untouched and should be treated as the source roadmap document.

## Current state

`scavium-faucet` is deployed and operational on Debian 13 at `https://faucet.testnet.scavium.network` behind nginx with certbot-managed TLS and a systemd-managed backend process.

Phase 14 deployment work is COMPLETED for the testnet public faucet target. Phase 15 Abuse Protection is CLOSED for the public testnet scope, with captcha, durable abuse signals, progressive enforcement, and retention documented as the active production baseline. Phase 16 Observability & Operations is CLOSED as the first operator-facing visibility layer over the same deployed service. Phase 17 Token Support is CLOSED as the complete multi-token faucet layer, including config-driven native/ERC20 registration, strict token-aware claim validation, token-scoped enforcement and metrics, browser-side token selection, and the Phase 17.5 post-audit fixes. Phase 18 Admin Control is CLOSED after the Phase 18.7 post-audit fix pass: the admin surface exposes metrics, runtime visibility, queue visibility/control, claim retry/cancel, blocklist management, mode control, and audit trail behavior while preserving public API compatibility. Phase 30 Wallet Integration is CLOSED through fix 5, adding optional wallet challenges/proofs with SQLite persistence, replay resistance, fallback parity, and legacy claim compatibility.

The service is production-ready for the current testnet scope, including validated TLS auto-renewal, active firewall policy, loopback-isolated backend exposure, request correlation, structured claim-flow logs, admin-protected runtime metrics, admin-protected Prometheus-compatible metrics text export, admin runtime/queue visibility, admin queue controls, runtime-effective faucet mode control, alerting guidance, local smoke tests, and enriched health/readiness probes, review-first SQLite/config backup and restore scripts, and wallet refill/rotation runbooks.

## Documentation

| Document | Description |
|---|---|
| [architecture.md](architecture.md) | Actual runtime wiring, package roles, and state model |
| [api.md](api.md) | Public API reference plus the handler-level admin contract |
| [openapi.yaml](openapi.yaml) | Lightweight Phase 25 OpenAPI contract for implemented stable surfaces |
| [configuration.md](configuration.md) | Environment variables, defaults, and what is wired today |
| [deployment.md](deployment.md) | Review-first VPS deployment package with systemd, nginx, env, certbot, firewall, and rollback assets |
| [deployment-certbot.md](deployment-certbot.md) | Manual ACME and certbot guide for TLS issuance and renewal |
| [deployment-firewall.md](deployment-firewall.md) | Public exposure and firewall policy for VPS and cloud edge |
| [deployment-rollback.md](deployment-rollback.md) | Rollback procedure for release symlinks and service recovery |
| [runbook.md](runbook.md) | Build, run, health checks, backup/restore, wallet refill/rotation, and operational caveats |
| [security.md](security.md) | Current security properties, gaps, and deployment guidance |
| [token-registration.md](token-registration.md) | Phase 17.2 testnet token registration guide for native and ERC20 faucet assets |
| [phase26-public-frontend-completion.md](phase26-public-frontend-completion.md) | Phase 26 public frontend implementation narrative |
| [phase26-public-frontend-closure-audit.md](phase26-public-frontend-closure-audit.md) | Phase 26 closure audit and deferred-work boundary |
| [phase27-advanced-anti-abuse-closure-audit.md](phase27-advanced-anti-abuse-closure-audit.md) | Phase 27 advanced anti-abuse implementation and closure audit |
| [phase27-fix4-completion.md](phase27-fix4-completion.md) | Phase 27 Fix4 completion note for manual-review/risk-rejection ordering |
| [phase30-wallet-integration.md](phase30-wallet-integration.md) | Phase 30 wallet challenge/proof closure note and deferred wallet backlog |

## Current implementation snapshot

- The binary loads environment config and listens on `127.0.0.1:18080` by default.
- Non-API paths serve the embedded frontend; `/api/*` paths return JSON.
- Public endpoints support health, readiness, status, config, token catalog discovery, wallet challenge issuance, claim creation, claim lookup, address eligibility, and version.
- Claim data, wallet challenges, and abuse-signal observations are persisted in SQLite (WAL mode). Restarting the process does not lose queued or in-flight claims, short-lived wallet challenges, or recorded claim-intake signals.
- The background worker processes the SQLite claim queue and dispatches the configured sender (dry-run or real).
- Readiness checks are real probes against the database and queue; RPC and wallet checks activate when not in dry-run mode.
- `AdminToken` is wired from config into the HTTP handler; setting `SCAVIUM_FAUCET_ADMIN_TOKEN` enables the `/api/v1/admin/*` endpoints.
- Captcha verification, durable abuse signal capture, trusted-proxy IP extraction, user-agent forwarding, and persistent rate limits (IP per hour, address per day) are active in claim creation.
- CORS is configurable via `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS`; exact-origin matching only, wildcard `*` rejected at startup, admin paths always excluded. Empty (default) disables CORS headers entirely.
- Daily distribution is optionally capped by `SCAVIUM_FAUCET_DAILY_BUDGET_WEI`; the limit is enforced atomically in SQLite and resets at UTC midnight.
- Each request produces a structured JSON access log line on stdout containing `request_id`, `correlation_id`, `method`, `path`, `status`, `duration`, and `remote_ip`; no secrets or request bodies are logged.
- `X-Request-ID` and `X-Correlation-ID` are echoed on responses; when no correlation ID is supplied, the request ID is used as the correlation ID.
- The claim path emits safe structured events for accepted and rejected claims without logging addresses, raw fingerprints, captcha tokens, request bodies, secrets, or idempotency-key values.
- `GET /api/v1/admin/metrics` exposes lightweight in-process runtime counters and process metrics when `SCAVIUM_FAUCET_ADMIN_TOKEN` is configured.
- `GET /api/v1/admin/metrics/prometheus` exposes the same safe process-local metrics as Prometheus-compatible text behind the same admin bearer-token middleware.
- `GET /api/v1/admin/runtime` aggregates dashboard, readiness, metrics, and timestamp data for operator inspection.
- `GET /api/v1/admin/queue` exposes bounded queue visibility; admin queue retry/cancel endpoints provide limited operational control.
- `POST /api/v1/admin/faucet/mode` accepts only `active`, `paused`, or `maintenance` and propagates the selected mode into the live faucet runtime.
- Admin audit entries and structured admin-action logs avoid admin-token leakage; actor attribution uses trusted-proxy-aware real IP extraction.
- `/health` includes uptime and build metadata; `/ready` includes per-check duration and aggregate readiness summary while keeping the real DB/queue/RPC/wallet probes.
- `scripts/scavium-faucet-backup.sh` and `scripts/scavium-faucet-restore.sh` provide plan-first backup/restore flows for SQLite and reviewed configuration.
- Wallet refill and rotation remain manual runbook operations; no automatic treasury refill or fund-transfer automation is introduced.

## Quick start

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-dev.db \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
go run ./cmd/scavium-faucet
```

See [configuration.md](configuration.md) for the environment reference and [runbook.md](runbook.md) for operational notes.


## Phase 15.3 — Progressive Abuse Enforcement

Phase 15.3 builds directly on the Phase 15.2 abuse signal ledger. The faucet now evaluates recent negative signals during claim intake and can reject a request when the configured IP, address, or fingerprint threshold is reached within the enforcement window.

The implementation remains production-safe and contract-preserving: no public endpoint changes were introduced, no schema migration was required, and rejected requests reuse the existing `claim_rejected` envelope. Each enforcement rejection is also recorded back into `abuse_signals`, keeping the audit trail cumulative for later admin controls, blocklists, and adaptive rate limiting.


## Phase 15.4 — Abuse Operations & Retention

Phase 15.4 completes the operational side of Abuse Protection. The faucet now prunes old `abuse_signals` at startup according to `SCAVIUM_FAUCET_ABUSE_SIGNAL_RETENTION_DAYS`, defaulting to 30 days and allowing `0` as an explicit opt-out.

This keeps the abuse ledger useful for progressive enforcement and investigation without allowing unbounded SQLite growth. Internal aggregate summaries by signal kind were also added for future observability/admin work, but no new public endpoint was exposed and existing API contracts remain unchanged.

## Phase 15.close — Abuse Protection Closure

Phase 15 is now closed as a cumulative abuse-protection layer for the production-ready public faucet. The implemented path covers human verification, durable claim-intake signals, conservative progressive enforcement, and bounded retention for the abuse ledger.

The closure is documentary only: no runtime code, public API contract, deployment topology, nginx exposure model, or database schema changes are required. Phase 16 can now build observability and operator feedback loops on top of the stable abuse dataset and existing structured request logs.

## Phase 16.close — Observability & Operations Closure

Phase 16 is closed as an incremental observability layer over the production faucet. Request correlation, safe structured logging, runtime counters, admin-protected metrics, and enriched health/readiness responses are now part of the active operational baseline.

The closure remains contract-preserving: no public claim response shape changed, no backend exposure model changed, no database migration was added, and no external metrics service was required. Phase 17 can now extend token support with enough runtime visibility to diagnose claim intake, rejection classes, readiness degradation, and deployed build identity.


## Phase 17.2.2 — Testnet Token Registration Guidance

Phase 17.2.2 documents the production-safe operator path for registering SCAVIUM testnet faucet assets through configuration. The new guide keeps Phase 17.1 and 17.2.1 behavior intact: token ids remain config-driven, `POST /api/v1/claim` remains backward-compatible, and public catalog discovery continues through `GET /api/v1/tokens` and `GET /api/v1/faucet/tokens`.

The guidance covers native-only operation, native + ERC20 testnet registration, faucet wallet balance checks, decimals and base-unit conversion, post-restart catalog validation, and explicit `token_id` claim testing. No runtime admin mutation, frontend selector, database-backed token catalog, or public contract change is introduced in this subphase.

## Phase 17.2.close — Token Registration Closure

Phase 17.2 is closed as a configuration-driven token registration layer for the public testnet faucet. The implemented scope now covers the Phase 17.1 multi-token foundation, the Phase 17.2.1 public token catalog endpoints, and the Phase 17.2.2 operator registration guide for native and ERC20 testnet assets.

The closure is documentary only and preserves the production contract: existing clients may still call `POST /api/v1/claim` without `token_id`, the configured default token remains the fallback, token metadata remains claim-safe, and no runtime admin mutation or database-backed token catalog is introduced. Phase 17.3 can now harden token-aware claim validation on top of a stable registration and discovery baseline.



## Phase 17.3.1 — Token Validation Layer

Phase 17.3.1 hardens the token-aware claim path introduced in Phase 17.1 and operationalized in Phase 17.2. Claims now resolve and validate `token_id` before captcha, risk, cooldown, rate-limit, daily-budget, persistence, and queue processing. Unknown or non-executable token selections are rejected through the existing `claim_rejected` contract with `invalid_token` as the reason, preserving the public error envelope while making token validation explicit.

The subphase also records invalid token attempts as durable abuse signals and exposes `claims.invalid_token` in admin metrics. Backward compatibility is unchanged: clients that omit `token_id` continue to receive the configured default token.

## Phase 17.3.2 — Token-Aware Abuse & Rate-Limit Scope

Phase 17.3.2 narrows claim-intake enforcement scopes so that cooldown and rate-limit decisions are evaluated against the selected token rather than only against the wallet/IP/fingerprint globally. This keeps the Phase 17.3.1 validation boundary intact while preventing one configured faucet asset from unintentionally consuming another asset's eligibility window.

The change remains backward-compatible with the existing claim API. Clients still submit the same optional `token_id`; omitted token ids resolve to the configured default token before enforcement. Daily budgets were already token-aware from the Phase 17 foundation and remain unchanged.

## Phase 17.3.3 — Token-Aware Observability Alignment

Phase 17.3.3 aligns the Phase 16 observability surface with the token-aware claim pipeline introduced across Phase 17. Claims now increment token-scoped runtime counters in addition to the existing aggregate metrics, allowing operators to distinguish default-token traffic, ERC20 token traffic, rate-limit pressure, daily-budget pressure, and invalid token attempts from `/api/v1/admin/metrics` without introducing an external metrics dependency.

The logging posture remains production-safe. Accepted claim-flow logs include the resolved `token_id` and a token claim event marker, while rejected claim-flow logs include only the supplied token scope or the `default` bucket for omitted `token_id`. Addresses, raw fingerprints, captcha tokens, request bodies, secrets, and idempotency-key values remain excluded from structured logs.

## Phase 17.3.close — Claim Validation Hardening Closure

Phase 17.3 is closed as the token-aware claim validation and enforcement hardening layer for the public testnet faucet. The implemented scope covers strict claim-time token validation, token-scoped cooldown and rate-limit evaluation, and token-aware observability over the existing aggregate runtime metrics.

The closure is documentary only and preserves the production contract. `POST /api/v1/claim` remains backward-compatible, omitted `token_id` still resolves to the configured default token, invalid token selections continue to use the existing `claim_rejected` envelope with `invalid_token` as the reason, and no frontend selector, runtime admin mutation, database-backed token catalog, or external metrics backend is introduced. Phase 18 can now build admin/control-plane capabilities on top of a stable token-aware claim pipeline.

## Phase 17.4.1 — Public Token Catalog Consumption

Phase 17.4.1 starts the frontend side of token support by consuming the public token catalog already exposed by Phase 17.2. The embedded HTML/JS faucet UI now loads `GET /api/v1/tokens`, renders a token selector from claim-safe metadata, and submits the selected token id as optional `token_id` in `POST /api/v1/claim`.

The implementation remains backward-compatible and failure-safe: if the token catalog cannot be loaded, the selector stays hidden and the claim payload continues to omit `token_id`, preserving the existing default-token path. No admin mutation, database-backed catalog, new dependency, or public claim contract change is introduced.

## Phase 17.4.2 — Token Selector UX Hardening

Phase 17.4.2 hardens the browser-side token selector introduced in Phase 17.4.1 without changing any backend contract. The frontend now exposes explicit catalog loading and fallback states, keeps default-token claim behavior visible when catalog discovery fails, and renders selected-token details using claim-safe metadata from `GET /api/v1/tokens`.

The UI remains dependency-free and CSP-compatible. Token labels and detail cards use only public metadata (`id`, `symbol`, `type`, `decimals`, and `amount_wei`), while the claim payload continues to send only the optional selected `token_id`. No token icons, balance checks, runtime admin mutation, or database-backed catalog behavior is introduced.

## Phase 17.4.3 — Token Claim Result UX Alignment

Phase 17.4.3 aligns the browser-side claim result panel with the token-aware claim pipeline already stabilized in Phase 17.3. Successful and polled claim responses now render a compact token-aware summary above the existing key/value details, using the returned token metadata to show the selected asset, resolved amount, status, token type, and explorer action more clearly.

The change remains frontend-only and contract-preserving. It does not introduce new API fields, backend behavior, external JavaScript dependencies, token icons, or admin mutation. If a response represents the native/default path or lacks token metadata, the UI falls back to the existing default-token wording and still displays the raw claim details.

## Phase 17.4.close — Token-Aware Frontend Closure

Phase 17.4 is closed as the browser-facing token selection and claim-result alignment layer for the public testnet faucet. The implemented scope covers public catalog consumption in the embedded frontend, visible selector loading/fallback states, selected-token detail rendering, optional `token_id` submission, and token-aware claim-result summaries for accepted and polled claims.

The closure is documentary only and preserves the production contract: `POST /api/v1/claim` remains backward-compatible, omitted `token_id` values still use the configured default token, catalog failures still fall back to legacy/default claim behavior, and no backend runtime token mutation, database-backed token catalog, or new frontend dependency is introduced. Phase 18 can now build admin control and operational management features on top of a stable token-aware user surface.

## Phase 17.5.close — Post-Audit Fix Closure

Phase 17.5 is closed as the post-audit correction pass for the completed token-support phase. The implemented fixes resolve the Phase 17 audit findings around frontend status rendering, cooldown retry display, token-aware metric bucket consistency for clients that omit `token_id`, and defensive sanitization of user-supplied token ids before rejection logging/metrics paths.

The closure is documentary only. The public API contract remains unchanged: `POST /api/v1/claim` still accepts optional `token_id`, omitted values continue to use the configured default-token path, error envelopes remain stable, and no runtime token mutation or database-backed token catalog is introduced. With the audit fixes validated by `go test ./...`, Phase 18 can start from a production-safe token-aware baseline.

## Phase 18.close — Admin Control Closure

Phase 18 is closed as the first production-safe admin control plane over the public faucet. The implemented scope is intentionally incremental: it adds operator visibility and bounded controls without changing the public claim contract, introducing new external services, changing the SQLite schema, or moving configuration into a database.

The closed Phase 18 baseline includes admin-protected runtime metrics, composite runtime visibility, queue summary visibility, queue retry/cancel endpoints, claim retry/cancel endpoints, blocklist management, faucet mode control, and audit trail behavior. The Phase 18.7 post-audit pass corrected the critical runtime gap by ensuring admin mode changes affect the live faucet service instead of only an isolated admin summary. It also aligned audit actor attribution with trusted-proxy real IP extraction and capped admin queue listing size.

The closure deliberately defers broader control-plane expansion. Dynamic budget/config editing, database-backed admin catalogs, CSV/export workflows, role-based admin accounts, 2FA, allowlist/campaign controls, and durable audit persistence remain later-phase work. Phase 19 can now focus on production hardening from a verified admin-control baseline.

## Phase 19.close — Production Hardening Closure

Phase 19 is closed as a conservative production-hardening pass over the already-stable faucet runtime. The implemented scope covers backend security headers, defensive rate-limit edge cases, explicit request/header/body/time limits, and deterministic graceful shutdown.

The closure preserves the public contract for `POST /api/v1/claim`, error envelopes, request/correlation headers, token-aware behavior, and admin bearer authentication. Phase 20 subsequently completed the planned SQLite-backed admin claim/queue controls, durable audit persistence, and persisted blocklist enforcement without changing those public/admin endpoint contracts.

With Phase 20 closed, the documentation is aligned to the current durable admin-state baseline while keeping later deferred items (dynamic config mutation, campaigns/allowlists, runtime token mutation, advanced resilience) explicit.


## Phase 21 closure

Phase 21 is closed as the operator observability and alerting baseline. The implementation keeps the existing production topology and dependency profile: metrics remain process-local, admin-protected, and safe for operator use; a bounded Prometheus-compatible text endpoint was added without exposing sensitive labels; worker and watcher runtime counters now cover queue and blockchain outcomes; and the runbook now includes alert thresholds plus a local non-mutating smoke-test script for deploy and rollback checks.

## Phase 22 closure

Phase 22 is closed as the conservative blockchain/runtime resilience layer. The faucet now supports startup-only RPC failover with chain-ID validation and exposes admin-protected wallet visibility for signer address, native balance, pending nonce, and configured token balances. It does not add load balancing, automatic endpoint rotation during transactions, or fund movement automation.

## Phase 23 closure

Phase 23 is closed as the operational runbook and backup/restore layer. Operators now have plan-first SQLite/config backup and restore helpers, including verified optional SQLite WAL/SHM companion restoration, documented restore drills, deployment rollback verification commands, and manual wallet refill/rotation procedures. The closure is intentionally non-automatic: no treasury refill, private-key rotation, or fund transfer script was added.

## Phase 24 closure

Phase 25 closes public API completion for address history, wallet/address eligibility status completion, pagination conventions, and the lightweight OpenAPI baseline.

Phase 24 closes the post-Phase-14 roadmap. The closure audit is recorded in `phase24-post14-closure-audit.md`, and the Copilot/manual re-audit prompt is available at `cmd/scavium-faucet/.agent/phase24-closure-audit-prompt.md`. Future work should begin from Phase 25 or another explicitly scheduled backlog item in `docs/scavium_faucet_public_features.md`; Stage 4 remains deferred until intentionally planned.


## Phase 26 closure

Phase 26 is closed as the public frontend completion pass over the Phase 25 API baseline. The embedded browser UI now exposes address eligibility and bounded public address history, keeps explorer links optional and defensively validated, adds in-page privacy and terms links with safe testnet copy, and improves accessibility/mobile behavior without introducing a frontend framework, inline event handlers, backend contract changes, or admin-surface changes.

The implementation narrative is `phase26-public-frontend-completion.md`, and the closure audit is `phase26-public-frontend-closure-audit.md`.


## Phase 27 closure

Phase 27 is closed as an advanced anti-abuse increment over persisted abuse signals. The implementation adds deterministic risk score composition, same-IP burst detection across successful and failed intake signals, rotating-IP fingerprint heuristics, address clustering, disabled-by-default honeypot handling, and persisted internal manual-review hints without changing public claim envelopes or introducing new external services.


## Phase 28 closure

Phase 28 is closed as a durable config and budget control increment. Operators can now edit a minimal non-secret runtime policy subset through admin-protected endpoints, with persisted SQLite overrides, durable audit summaries, and immediate claim-time application. The detailed implementation narrative is `phase28-config-budget-control.md`. Campaigns, allowlists, runtime token catalog mutation, and all secret-bearing configuration remain out of scope for later phases.


## Phase 29 closure — Campaigns, allowlists, and invitation codes

Phase 29 is closed as a production-safe campaign distribution increment. The faucet now persists campaigns, invitation codes, allowlist entries, and claim-level campaign attribution in SQLite; optional public claim fields remain backward compatible; admin-only campaign controls and CSV export are protected by the existing bearer-token middleware. The final fix passes also align campaign admin mutations with the Phase 28 audit-safety rule by rolling back create/disable/invitation/allowlist writes when durable audit append fails, and reject any durable invite claim if post-create invitation consumption fails. The final coverage audit adds focused persistent-service tests for public campaigns, invalid invite codes, exhausted campaign budgets, allowlist-approved claims, invite idempotency, and admin HTTP contract coverage for auth, invalid bodies, unsupported methods, CSV limits, and CSV formula hardening. Details are maintained in `phase29-campaigns-allowlists-invitation-codes.md`.
