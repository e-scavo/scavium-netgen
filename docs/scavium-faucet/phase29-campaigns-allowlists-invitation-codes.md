# Phase 29 — Campaigns, Allowlists, and Invitation Codes

## Implemented baseline

Phase 29 introduces a production-safe baseline for campaign-aware faucet distribution:

- campaign domain models and scopes
- invitation-code domain model
- optional public claim request fields:
  - `campaign_id`
  - `invitation_code`
- backward-compatible request handling for legacy clients
- preparatory interfaces for durable SQLite-backed campaign persistence

## Scope boundaries

The implementation intentionally preserves:

- existing normalized error envelopes
- existing claim flow contracts
- existing admin authentication requirements
- Stage 4/professional-scale deferrals

## Deferred items

The following remain intentionally deferred:

- distributed campaign coordination
- HA/distributed locks
- external CRM integrations
- advanced allowlist segmentation
- automatic wallet refill orchestration
