# Step 13.2.1 — API documentation alignment

Modify:
docs/scavium-faucet/api.md

Goals:

1) Update POST /api/v1/claim errors

Add full table:

| HTTP | Code | Meaning |
|------|------|--------|
| 422 | captcha_failed |
| 403 | claim_rejected |
| 429 | rate_limited |
| 429 | daily_budget_exceeded |
| 503 | faucet_unavailable |
| 500 | claim_unavailable |

2) Document `retry_after_seconds` in response body

3) Clarify that:
- rate limiting includes IP, address, fingerprint
- daily budget is global limit

4) Add note:
"Retry-After header may be added in future versions"

Do not modify unrelated sections.
Preserve structure and narrative.
