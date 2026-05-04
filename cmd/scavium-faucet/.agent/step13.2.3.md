# Step 13.2.3 — Security documentation alignment

Modify:
docs/scavium-faucet/security.md

Update sections:

## Active protections

Add:

- Persistent rate limiting (IP, address, fingerprint)
- Captcha enforcement
- Risk engine blocking
- Daily budget protection
- Safe CORS (exact origin match)
- Admin API isolation (no CORS)
- Request logging without sensitive data

## Remaining gaps

Keep ONLY real gaps:

- CORS wildcard not supported (by design)
- Retry-After header not implemented
- No distributed budget enforcement (single node)

## Remove outdated:

- "not wired"
- "planned"
- "future feature"

## Update operator stance:

Now safe for:
- public faucet (with captcha enabled)
- production testnet usage

Preserve narrative tone.
