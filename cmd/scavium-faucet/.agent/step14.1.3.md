# Step 14.1.3 — Manual VPS deployment procedure

## Recommended executor

Human/operator on VPS.

## Goal

Deploy the faucet on Debian 13 VPS.

## Important

This step is manual. Do not ask Codex to run commands against the VPS unless explicitly connected to the VPS terminal.

## Pre-flight

On local repo:

```bash
git status --short
go test ./...
go build -o ./dist/scavium-faucet ./cmd/scavium-faucet
```

Copy binary and deployment assets to VPS.

## VPS procedure outline

1. Install packages:
   - nginx
   - certbot
   - python3-certbot-nginx
   - ca-certificates
   - curl
   - ufw

2. Create user/dirs:
   - `scavium-faucet`
   - `/opt/scavium-faucet/bin`
   - `/etc/scavium-faucet`
   - `/var/lib/scavium-faucet`

3. Install binary:
   - `/opt/scavium-faucet/bin/scavium-faucet`

4. Install env:
   - `/etc/scavium-faucet/scavium-faucet.env`
   - permissions `0600`
   - owner `root:scavium-faucet` or equivalent secure setup

5. Install systemd unit:
   - `/etc/systemd/system/scavium-faucet.service`

6. Install nginx config:
   - `/etc/nginx/sites-available/scavium-faucet`
   - symlink to `sites-enabled`

7. Obtain TLS:
   - `certbot --nginx -d faucet.testnet.scavium.network`

8. Enable firewall safely:
   - allow SSH first
   - allow 80/443
   - enable UFW

9. Start service:
   - `systemctl daemon-reload`
   - `systemctl enable --now scavium-faucet`
   - `systemctl status scavium-faucet`

10. Smoke:
   - `/health`
   - `/ready`
   - claim if configured safely

## Required operator notes

- Do not put private key in shell history.
- Use editor with secure permissions for env file.
- Confirm backend is not listening publicly:
  - `ss -lntp | grep 18080`
  - should show `127.0.0.1:18080`.

## Output to capture

- OS version
- nginx test result
- certbot result
- systemd status
- health result
- ready result
- firewall status
