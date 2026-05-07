# Phase 26 — Public Frontend Completion Closure Audit

## Scope closed

Phase 26 completes the scheduled public frontend work from `implementation-roadmap-after-phase19.md` on top of the Phase 25 public API baseline. It remains a browser/UI pass only and does not start Phase 27 anti-abuse work.

Closed items:

- Claim submission remains backward-compatible with `POST /api/v1/claim`, keeps `Idempotency-Key`, and preserves the normalized error-envelope behavior.
- The embedded frontend serves `index.html` plus external `/static/faucet.js`; no inline event handlers or inline JavaScript are introduced.
- Public faucet mode UX consumes `GET /api/v1/status`, using the documented `status` field and keeping a fallback for older `mode` payloads.
- Address eligibility UX calls `GET /api/v1/address/{address}/status` from the existing address input and renders only public-safe fields, including token-aware and daily-budget status when present.
- Address history UX calls `GET /api/v1/address/{address}/history?limit=10&offset=0`, announces loading/empty/error states in the history panel itself, moves focus to the history panel, and displays bounded public claim history only.
- Transaction explorer actions render only when the configured explorer transaction template is absolute HTTP(S), contains `{txHash}`, and the returned transaction hash is a syntactically valid 32-byte EVM hash.
- Privacy and terms links are present as in-page, safe testnet defaults; operators that require jurisdiction-specific copy can replace the static HTML or serve reviewed pages through nginx.
- Accessibility and responsive polish cover labels, semantic sections, live status areas, busy button states, keyboard-reachable controls, focus-visible styling, focus movement for status/history panels, and small-screen stacking.

## Contract safety

Phase 26 does not change backend persistence, migrations, admin routes, claim envelopes, public route names, CORS behavior, request IDs, correlation IDs, rate-limit semantics, captcha verification, or token validation. The browser never renders private abuse signals, raw fingerprints, captcha tokens, idempotency keys, blocklist notes, internal queue controls, admin audit entries, bearer tokens, secrets, or unbounded metric labels.

Explorer URLs are treated as optional presentation data derived from runtime status configuration. Missing, relative, malformed, or non-HTTP(S) explorer templates suppress links instead of constructing relative or attacker-controlled URLs. Transaction hashes are validated before any explorer anchor is rendered, and the static HTML does not hardcode an explorer host.

## Runtime validation notes

The frontend package tests assert that the embedded UI serves the external script, avoids inline `onclick` handlers, includes the Phase 26 status/history/legal UX markers, uses the public Phase 25 APIs, and keeps explorer-link construction guarded by absolute URL and transaction-hash checks. Operator smoke checks in `runbook.md` cover browser behavior that cannot be fully proven by package tests, including keyboard flow, maintenance-mode UX, empty history, explorer suppression, and narrow viewport layout.

The deployable source tree contains a single embedded public frontend under `cmd/scavium-faucet/internal/frontend/web`. There is no parallel legacy `cmd/scavium-faucet/web` asset tree in this ZIP, avoiding static frontend drift.

## Deferred work

Advanced anti-abuse, risk scoring, burst detection, address clustering, rotating IP heuristics, optional honeypot/JS challenges, dynamic runtime budget/config editing, allowlists, campaigns, invitation codes, legal-page runtime configuration, analytics, frontend frameworks, and wallet-signature flows remain later scheduled phases or explicit operator customizations.
