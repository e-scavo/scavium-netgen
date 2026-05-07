# Phase 27 — Advanced Anti-Abuse Closure Audit

Phase 27 is implemented as a conservative extension of the existing persisted `abuse_signals` ledger. It does not add external reputation services, does not add a schema migration, and does not change the public `POST /api/v1/claim` response envelope.

## Implemented scope

- Risk score expansion now preserves existing progressive IP/address/fingerprint thresholds as immediate rejection gates and adds bounded score contributions for the new Phase 27 heuristics.
- Burst detection scores repeated same-IP claim-intake activity over `SCAVIUM_FAUCET_ABUSE_BURST_WINDOW_SECONDS`.
- Rotating-IP heuristics count distinct recent `remote_ip` values for the same fingerprint using a hardcoded safe column selector.
- Address clustering counts distinct recent addresses for the same fingerprint, or the same IP when no fingerprint is available.
- Optional honeypot handling is disabled by default through `SCAVIUM_FAUCET_ABUSE_HONEYPOT_ENABLED=false` and uses the request field `website` for compatibility with legacy clients.
- Manual-review surfacing is internal to `domain.RiskDecision.Review`; public callers still receive only normalized errors.

## Safety notes

All risk reasons are bounded categories. Raw IPs, addresses, fingerprints, user agents, captcha tokens, idempotency keys, and honeypot contents are not emitted in public errors or Prometheus labels. Thresholds can be disabled individually with `0` except the time windows, which must remain positive.

## Deferred work

Runtime policy editing, campaign systems, allowlists, invitation codes, ASN/geolocation checks, and professional-scale distributed anti-abuse remain outside Phase 27 and continue in later roadmap phases only.
