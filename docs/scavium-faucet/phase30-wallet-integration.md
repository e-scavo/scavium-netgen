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

Static formatting and shell/script validation were completed. Go tests/build could not run in this environment because the local Go version is `go1.23.2` and the module requires Go `1.24.0`; the toolchain download from `proxy.golang.org` failed due network/DNS restrictions. The implementation was still reviewed for runtime wiring, persistence, fallback behavior, public API compatibility, defensive validation, documentation, and migration consistency.
