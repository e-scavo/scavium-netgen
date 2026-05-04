# Step 14.1.4 — Post-deploy smoke validation

## Recommended executor

Human/operator, with Copilot Chat assisting analysis if needed.

## Goal

Validate deployed faucet without making unsafe assumptions.

## Commands

From VPS:

```bash
systemctl status scavium-faucet --no-pager
journalctl -u scavium-faucet -n 100 --no-pager
nginx -t
ss -lntp
curl -sS http://127.0.0.1:18080/health
curl -sS http://127.0.0.1:18080/ready
curl -sS https://faucet.testnet.scavium.network/health
curl -sS https://faucet.testnet.scavium.network/ready
```

Optional claim only if faucet has safe funds and captcha/dev policy is intentionally configured:

```bash
curl -sS -X POST https://faucet.testnet.scavium.network/api/v1/claim \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: deploy-smoke-1' \
  -d '{"address":"0x0000000000000000000000000000000000000001"}'
```

## Verify

- backend listens only on localhost
- public HTTPS works
- `/ready` returns expected state
- logs show request entries
- nginx forwards real IP
- Certbot renewal dry-run works:
  - `certbot renew --dry-run`

## Output

- pass/fail
- observed responses
- any required fix
