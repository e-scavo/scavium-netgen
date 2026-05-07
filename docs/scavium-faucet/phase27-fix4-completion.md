# Phase 27 Fix4 Completion — Manual Review and Risk Rejection Ordering

Phase 27 Fix4 is closed as a targeted correction on top of the advanced anti-abuse implementation. The fix preserves the Phase 27 public API contract while making the internal abuse-signal ledger deterministic for operator diagnostics and tests.

## Closure scope

- Rejection decisions that also request manual review now persist the bounded `manual_review` signal before the terminal `risk_rejected` signal.
- The final persisted signal for a rejected claim remains `risk_rejected`, preserving existing diagnostics that inspect the latest denial event and keeping progressive enforcement history stable.
- Allowed decisions with `Review=true` continue to persist `risk_allowed` followed by `manual_review`, so low-score suspicious activity remains visible without denying the user.
- Public callers still receive the normalized `claim_rejected` envelope for rejected claims; raw IPs, addresses, fingerprints, user agents, honeypot contents, captcha tokens, idempotency keys, and private operational details are not exposed.

## Verification notes

The closure is covered by the persistent faucet service tests:

- `TestPersistentReadServiceProgressiveAbuseEnforcementRejects` verifies that progressive abuse rejection records `risk_rejected` as the last signal with the expected score.
- `TestPersistentReadServicePersistsManualReviewHints` verifies that allowed review hints are persisted as `manual_review` with the bounded score and reason.
- The broader Phase 27 enforcer tests cover honeypot rejection, burst detection over failed intake signals, rotating-IP scoring, and address-cluster scoring.

## Deferred work

No runtime policy editor, campaign system, allowlist, invitation-code flow, ASN/geolocation reputation check, external reputation service, or distributed anti-abuse backend is introduced by this fix. Those remain deferred to later scheduled phases.
