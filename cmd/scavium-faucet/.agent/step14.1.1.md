# Step 14.1.1 — Generate deployment templates and scripts

## Recommended executor

Codex in VSCode.

## Goal

Create or update deployment assets for Debian 13 full hardening.

## Scope

Templates/scripts only. Do not modify Go business logic.

## Expected files

Prefer this layout unless existing repo conventions differ:

```text
docs/scavium-faucet/deployment/
├── scavium-faucet.env.example
├── scavium-faucet.service
├── nginx-scavium-faucet.conf
├── install-debian13.sh
├── rollback.sh
├── smoke-test.sh
└── README.md
```

If some already exist, update them incrementally.

## Required env example

Must include placeholders and comments for at least:

```text
SCAVIUM_FAUCET_HOST=127.0.0.1
SCAVIUM_FAUCET_PORT=18080
SCAVIUM_FAUCET_PUBLIC_BASE_URL=https://faucet.testnet.scavium.network
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db
SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS=https://faucet.testnet.scavium.network
SCAVIUM_FAUCET_DRY_RUN=false
SCAVIUM_FAUCET_RPC_URL=
SCAVIUM_FAUCET_CHAIN_ID=
SCAVIUM_FAUCET_PRIVATE_KEY=
SCAVIUM_FAUCET_CAPTCHA_PROVIDER=
SCAVIUM_FAUCET_CAPTCHA_SECRET=
SCAVIUM_FAUCET_DAILY_BUDGET_WEI=
SCAVIUM_FAUCET_ADMIN_TOKEN=
```

Secrets must be placeholders only.

## Required systemd behavior

- Run as dedicated non-root user, preferably `scavium-faucet`.
- Use `/etc/scavium-faucet/scavium-faucet.env`.
- Restart on failure.
- Bind only to localhost through env.
- Hardening:
  - `NoNewPrivileges=true`
  - `PrivateTmp=true`
  - `ProtectSystem=strict`
  - `ProtectHome=true`
  - `ReadWritePaths=/var/lib/scavium-faucet`
  - `StateDirectory=scavium-faucet` if compatible
  - sensible `CapabilityBoundingSet=`
  - sensible `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`
- Do not prevent outbound RPC connectivity.

## Required nginx behavior

- Domain: `faucet.testnet.scavium.network`
- Proxy to `http://127.0.0.1:18080`
- Correct headers:
  - `Host`
  - `X-Real-IP`
  - `X-Forwarded-For`
  - `X-Forwarded-Proto`
  - `X-Request-ID`
- Body limit small.
- Timeouts.
- Basic rate limiting and connection limiting.
- Security headers:
  - HSTS only in HTTPS server block
  - `X-Content-Type-Options`
  - `X-Frame-Options`
  - `Referrer-Policy`
  - conservative CSP if safe
- Include HTTP->HTTPS redirect server block.
- Leave Certbot managed certificate paths as placeholders or standard `/etc/letsencrypt/live/faucet.testnet.scavium.network/...`.

## Required install script

For Debian 13:

- apt update
- install nginx, certbot, python3-certbot-nginx, ca-certificates, curl, ufw
- create user and directories
- install binary from a supplied path argument or local build artifact
- install env example if missing, never overwrite real env without backup
- install systemd unit
- install nginx config
- test nginx config
- reload systemd
- enable service
- print next manual steps for secrets, certbot, firewall and smoke tests

## Required firewall guidance

The script may configure UFW only if explicitly invoked with a flag. It must never lock out SSH by default.

If adding optional UFW commands, ensure:

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
```

## Required smoke test script

- curl `/health`
- curl `/ready`
- optionally claim only when explicitly passed an address
- do not require secrets
- print useful diagnostics

## Validation

Run or suggest:

```bash
shellcheck docs/scavium-faucet/deployment/*.sh || true
bash -n docs/scavium-faucet/deployment/*.sh
grep -R "SCAVIUM_FAUCET_PRIVATE_KEY=.*[^<]" docs/scavium-faucet/deployment || true
```

Also run:

```bash
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not modify Go business logic.
- Do not hardcode real secrets.
- Do not touch mainnet domain beyond future note.
- Do not read or use `cmd/scavium-faucet-v0`.
