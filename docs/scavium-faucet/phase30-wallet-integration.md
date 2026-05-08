# Phase 30 — SCAVIUM Wallet Integration Closure

Phase 30 closes the wallet-integration baseline without changing the legacy public claim contract.

## Implemented scope

- Added `POST /api/v1/wallet/challenge` and `/api/v1/faucet/wallet/challenge`.
- Persisted wallet challenges in SQLite through migration `009_wallet_challenges.sql`.
- Added in-memory fallback challenge storage for test/dev fallback parity.
- Added optional claim proof fields:
  - `wallet_challenge_id`
  - `wallet_signature`
- Verified signatures with the existing go-ethereum dependency using Ethereum personal-sign message hashing.
- Preserved legacy claim behavior when both wallet proof fields are omitted.
- Preserved idempotency behavior by returning existing claims before consuming a wallet challenge on retry.
- Added configurable browser app-origin defense-in-depth through `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS`.
- Kept missing `Origin` allowed for native wallet, CLI, and server-to-server clients.

## Runtime behavior

Wallet challenges are non-secret, short-lived, and valid for five minutes. A successful optional wallet proof consumes the challenge so replay attempts are rejected. Expired, consumed, wrong-address, malformed-signature, and signer-mismatch proofs are normalized as `claim_rejected` responses with public-safe reasons.

Origin checks are deliberately not used as an authentication mechanism. They only reduce accidental browser misuse when operators configure exact allowed application origins.

## Deferred backlog

The following remain outside Phase 30 and should be scheduled explicitly later:

- multi-wallet UX negotiation
- multi-network wallet routing
- hosted production origin presets
- external wallet-provider webhooks
- high-availability or distributed challenge stores beyond the current single-instance SQLite model
- broader Stage 4 professional-scale architecture items

## Validation notes

Phase 30 has been closure-audited through fix 7 post-Copilot audit. The latest operator baseline supplied with the Phase 30 fix series reports `go test ./...` passing on Go 1.24. Static review in this environment also rechecked formatting, script syntax, OpenAPI YAML parsing, backup plan wiring, runtime/API compatibility, SQLite persistence, in-memory fallback behavior, and wallet-origin semantics. Local `go test`/`go build` execution remains blocked in this ChatGPT environment because the module requires Go 1.24 and the toolchain download from `proxy.golang.org` is unavailable.

## Post-implementation audit fixes

- Fix 1: the Phase 30 wallet challenge persistence model was adjusted so the SQLite store owns only domain-level wallet challenge records and no longer imports the faucet service package. This keeps the production store wiring intact while removing the Go test import cycle between `internal/faucet` tests and `internal/store/sqlite`.
- Fix 2: the in-memory wallet proof path was adjusted to consume challenges while holding the existing write lock instead of re-entering the service through an additional read lock. This removed the `TestInMemoryWalletChallengeAllowsOptionalSignature` timeout/deadlock.
- Fix 3: wallet allowed-origin enforcement on `POST /api/v1/claim` was narrowed to requests that actually include wallet proof fields. Legacy claims without `wallet_challenge_id` and `wallet_signature` remain compatible even when `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS` is configured, while challenge issuance and proof-bearing claims remain protected by the wallet origin policy.

- Fix 4: closure documentation was refreshed after the fix 3 compatibility audit so the roadmap and Phase 30 notes accurately reflect the current implementation baseline and do not leave stale validation language behind.
- Fix 5: the post-fix documentation baseline was refreshed again after the full Phase 30 implementation bundle was re-applied, ensuring roadmap status and closure notes consistently describe the current operator-validated implementation with `go test ./...` passing on Go 1.24.
- Fix 6: final audit of the Phase 30 fix 5 implementation confirmed no remaining code-level feature gaps for challenge issuance, SQLite persistence, replay/expiry handling, optional personal-sign verification, legacy claim compatibility, fallback parity, app-origin semantics, OpenAPI coverage, and operational script compatibility. This fix only refreshes closure status so docs match the verified implementation baseline.

- Fix 7: the post-Copilot audit closed cross-phase consistency gaps discovered while validating Phases 25–30 together. Persistent `/api/v1/tokens` now reports runtime policy budget overrides like `/api/v1/config`, failed durable enqueue attempts best-effort mark the freshly persisted claim as `rejected` with `enqueue_failed` instead of leaving an orphan `received` claim, SQLite-backed duplicate campaign/invitation writes map constraint races to validated admin errors, and the in-memory fallback address status now reports cooldown eligibility with token-level cooldown metadata for closer SQLite parity. Fix 7 follow-up documentation also aligns the runbook and documentation index so the runtime-policy/token-catalog validation path and Phase 28/29 closure docs are discoverable from the maintained docs entry point.
