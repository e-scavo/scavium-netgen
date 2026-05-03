# Security

Security model, hardening checklist, and secret management guidelines for `scavium-faucet`.

---

## Threat model

| Threat | Mitigation |
|---|---|
| Bot draining funds | Captcha + IP rate-limit + address cooldown + daily budget |
| Single IP rotating wallets | IP rate-limit + fingerprint (future) |
| Single wallet rotating IPs | Address cooldown + per-address daily limit |
| Stolen admin token | HTTPS only; token never logged; rotate and restart on suspected compromise |
| Stolen private key | Key stored only in `EnvironmentFile` with `chmod 640`; never logged; use treasury wallet with limited balance |
| RPC endpoint abuse | Backend RPC not exposed externally; only the Go binary talks to it |
| HTTP abuse | nginx `limit_req`, connection limits, and request body limit |
| Credential exposure in logs | All secret fields sanitised; never printed in structured logs |

---

## Network hardening

- The Go binary binds to `127.0.0.1` only — never to `0.0.0.0`.
- nginx terminates TLS and proxies to the loopback interface.
- The Besu RPC node must also bind to loopback or a private interface.
- Firewall rules must allow port `443` inbound and block `18080`, `18545` from external traffic.

```bash
# UFW example
ufw allow 22/tcp   # SSH
ufw allow 443/tcp  # HTTPS (nginx)
ufw deny  18080    # faucet backend — loopback only
ufw deny  18545    # Besu RPC — loopback only
ufw enable
```

---

## systemd hardening

Recommended security directives for the systemd unit:

```ini
[Service]
User=scavium-faucet
Group=scavium-faucet
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ReadWritePaths=/var/lib/scavium-faucet
EnvironmentFile=/etc/scavium-faucet/env
```

The `EnvironmentFile` must be owned by `root` and readable only by the service user:

```bash
chown root:scavium-faucet /etc/scavium-faucet/env
chmod 640 /etc/scavium-faucet/env
```

---

## Secret management

### Private key (`SCAVIUM_FAUCET_PRIVATE_KEY`)

- Never commit to source control.
- Store only in the `EnvironmentFile` with strict file permissions.
- Use a dedicated faucet wallet — never the treasury or validator keys.
- Keep the faucet wallet balance at the minimum needed (e.g. 1–7 days of budget).
- Refill from treasury on a schedule or when the balance guard triggers.

### Admin token (`SCAVIUM_FAUCET_ADMIN_TOKEN`)

- Generate with a cryptographically-secure PRNG: `openssl rand -hex 32`.
- Rotate immediately if compromised: update `EnvironmentFile`, restart service.
- Never share over plaintext channels.

### Captcha secret (`SCAVIUM_FAUCET_CAPTCHA_SECRET`)

- Keep in `EnvironmentFile` only.
- Rotate through the captcha provider dashboard if compromised.

---

## HTTP security headers

nginx must set the following headers for all responses:

```nginx
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
add_header X-Frame-Options DENY always;
add_header X-Content-Type-Options nosniff always;
add_header Referrer-Policy strict-origin-when-cross-origin always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';" always;
```

---

## CORS

The faucet sets CORS headers based on `SCAVIUM_FAUCET_PUBLIC_BASE_URL`. Only the configured origin is allowed. Do not set `Access-Control-Allow-Origin: *` in production.

---

## Trusted proxy configuration

If the faucet is behind nginx on the same host:

```bash
SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
```

This ensures `X-Forwarded-For` headers are trusted only from the loopback interface, preventing IP spoofing by external clients.

---

## Rate limiting layers

| Layer | Variable | Default |
|---|---|---|
| IP per hour | `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` | 10 |
| Address per day | `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` | 3 |
| Address cooldown | `SCAVIUM_FAUCET_COOLDOWN_SECONDS` | 86400 |
| Daily budget | `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` | unlimited |
| nginx rate limit | `limit_req_zone` | operator-defined |

All layers operate independently. A request must pass all active layers.

---

## Pre-production security checklist

- [ ] `SCAVIUM_FAUCET_DRY_RUN=false`
- [ ] `SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080` (loopback only)
- [ ] nginx TLS enabled with valid certificate
- [ ] nginx sets all security headers
- [ ] `EnvironmentFile` permissions: `chmod 640`, `chown root:scavium-faucet`
- [ ] Private key is a dedicated faucet wallet, not treasury/validator
- [ ] `SCAVIUM_FAUCET_TRUSTED_PROXY` set correctly
- [ ] `SCAVIUM_FAUCET_ADMIN_TOKEN` is a random 32-byte hex value
- [ ] Captcha provider is not `disabled` and not `dev`
- [ ] Firewall blocks direct access to ports `18080` and `18545`
- [ ] systemd hardening directives applied
- [ ] Wallet balance guard configured (`SCAVIUM_FAUCET_DAILY_BUDGET_WEI`)
- [ ] Log sanitisation verified (no secrets in `journalctl` output)
- [ ] Admin API tested only over HTTPS
