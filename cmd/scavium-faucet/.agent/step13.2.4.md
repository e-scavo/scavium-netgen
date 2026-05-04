# Step 13.2.4 — Runbook and deployment alignment

Modify:
docs/scavium-faucet/runbook.md
docs/scavium-faucet/deployment.md

Changes:

## runbook.md

Add:

- CORS config section
- daily budget behavior
- logging behavior

Update troubleshooting:

- 429 responses (rate limit / budget)
- captcha failures
- blocked requests

## deployment.md

Add:

- CORS_ALLOWED_ORIGINS example
- DAILY_BUDGET_WEI example

Clarify:

- DB must be persistent volume
- logs should be collected externally

Do not touch certbot / firewall docs.
