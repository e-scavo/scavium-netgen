# Phase 26 — Public Frontend Completion

Phase 26 closes the public browser UX work on top of the Phase 25 API surface. The implementation is intentionally frontend-only except for package tests and documentation. It does not change the `POST /api/v1/claim` request or response contract, does not add dependencies, and keeps the embedded assets CSP-compatible by preserving the external `/static/faucet.js` script model.

## Implemented scope

- Claim submission remains token-aware and idempotent.
- Public faucet mode rendering now consumes the documented `status` field from `/api/v1/status`, while keeping a fallback for older `mode` payloads.
- Users can check wallet/address eligibility from the same address input through `GET /api/v1/address/{address}/status`.
- Users can view bounded public claim history through `GET /api/v1/address/{address}/history?limit=10&offset=0`.
- Claim result and history explorer actions render only when both a configured explorer transaction URL and a syntactically valid transaction hash are present.
- Privacy and terms links are available in-page with safe default public wording for a testnet faucet. Operators that require jurisdiction-specific copy should replace the static HTML text during deployment packaging or serve external legal pages through the reverse proxy.
- Accessibility and mobile polish were limited to semantic regions, live status areas, focus-visible states, busy button states, keyboard-friendly buttons, responsive stacking, and non-inline JavaScript.

## Explicit non-goals

Phase 26 does not add account login, wallet signatures, advanced anti-abuse, runtime legal-link configuration, external analytics, frontend framework dependencies, or admin-facing behavior. It also does not expose private abuse signals, fingerprints, captcha material, idempotency keys, blocklist notes, internal queue controls, or admin audit data in the browser.

## Runtime privacy notes

The status and history panels reuse the public-safe backend contracts created in Phase 25. Address history is scoped to the address typed by the user and only displays public claim fields. Explorer links are treated as optional convenience links; malformed templates or malformed transaction hashes suppress link rendering instead of building unsafe URLs.

## Manual UX checks

Operators should verify after deployment:

1. Loading the homepage still serves `index.html` with `/static/faucet.js` and no inline event handlers.
2. Claim submission still works with and without `token_id` depending on token catalog availability.
3. Maintenance, paused, and no-funds states disable only the submit action and show a visible banner.
4. The eligibility button rejects malformed addresses client-side and renders the public status response for valid addresses.
5. The history button rejects malformed addresses client-side and renders an empty state or recent claims for valid addresses.
6. Explorer links appear only for configured explorer templates containing `{txHash}` and 32-byte EVM transaction hashes.
7. Keyboard tab order reaches the address input, token selector, captcha controls, submit, refresh, eligibility, history, privacy, and terms links.
8. The layout stacks controls cleanly on narrow screens.
