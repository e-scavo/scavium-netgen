# Deployment

This package prepares a **manual, review-first** VPS deployment for `scavium-faucet`.

It does **not** assume automatic production execution from this repository. All files use placeholders such as `DOMAIN`, `DEPLOY_USER`, `APP_PATH`, and `SERVICE_NAME` so an operator can review and adapt them before use.

## Current deployment status (2026-05-04)

Phase 14 has been executed on a real VPS and is operational.

- OS: Debian GNU/Linux 13 (trixie)
- Public domain: `faucet.testnet.scavium.network`
- TLS: active via certbot
- Reverse proxy: nginx active
- Service manager: systemd unit enabled and running
- Persistence: SQLite active at `/var/lib/scavium-faucet/scavium-faucet.db`
- Backend bind: loopback-only (`127.0.0.1:18080`)

Verified post-deploy behavior:

- `/health` and `/ready` respond successfully through the deployed topology
- claim lifecycle verified end-to-end (`queued` -> `sending` -> `confirmed`)
- rate limiting verified with `429` responses
- CORS policy verified
- request logging verified
- RPC connectivity and transaction sending verified

## Production Deployment Status (May 2026)

Final production hardening validation confirms the current live posture:

- TLS auto-renewal fully verified (`certbot renew --dry-run` completed successfully)
- `certbot.timer` is active for scheduled renewal checks
- renewal deploy hook is present at `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh`
- UFW is installed and active with default deny incoming
- allowed UFW ports are limited to `22/tcp`, `80/tcp`, and `443/tcp`
- backend remains isolated on loopback-only bind (`127.0.0.1:18080`)

## Current runtime constraints

Keep the deployment aligned with the current binary:

- claim state is persisted in SQLite; restarting the process does not lose queued claims
- admin routes are active when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set
- readiness checks are real DB/queue probes; RPC/wallet probes activate in non-dry-run mode
- the background worker processes the claim queue automatically (enabled by default)
- CORS is disabled by default; set `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` to a comma-separated list of exact origins if browser clients need cross-origin access
- daily distribution can be capped with `SCAVIUM_FAUCET_DAILY_BUDGET_WEI`; leave unset for unlimited
- the binary writes structured JSON access logs to stdout; route stdout to `journald` or a log aggregator

Store the SQLite database on a **persistent volume outside the release directory** so it survives deployments and rollbacks. Example path: `/var/lib/scavium-faucet/scavium-faucet.db`.

### CORS and daily budget examples

To enable CORS for a browser frontend and cap daily distribution, add to the environment file:

```ini
# Allow only the production frontend origin.
SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS=https://faucet.example.com

# Limit to 100 ether per UTC day (18-decimal token).
SCAVIUM_FAUCET_DAILY_BUDGET_WEI=100000000000000000000
```

Both settings are optional. An empty `CORS_ALLOWED_ORIGINS` disables CORS entirely. An unset `DAILY_BUDGET_WEI` means unlimited.

## Suggested server layout

```text
APP_PATH/
  current -> APP_PATH/releases/RELEASE_ID
  releases/
    RELEASE_ID/
      scavium-faucet
  review/
    RELEASE_ID/
      scavium-faucet.env.example
      scavium-faucet.service.template
      scavium-faucet.nginx.conf.template
```

Recommended placeholder mapping:

| Placeholder | Meaning |
|---|---|
| `DOMAIN` | public hostname, for example `faucet.example.com` |
| `DEPLOY_HOST` | target VPS hostname or IP |
| `DEPLOY_USER` | non-root SSH user used for deploys |
| `DEPLOY_GROUP` | service group on the VPS |
| `APP_PATH` | release root, for example `/opt/scavium-faucet` |
| `SERVICE_NAME` | systemd unit name, for example `scavium-faucet` |

## Files in this package

| File | Purpose |
|---|---|
| `deployment/scavium-faucet.service.template` | systemd unit template |
| `deployment/scavium-faucet.nginx.conf.template` | nginx server block template |
| `deployment/scavium-faucet.env.example` | example environment file with placeholders only |
| `deployment-certbot.md` | manual ACME and certbot guide |
| `deployment-firewall.md` | VPS and edge firewall guide |
| `deployment-rollback.md` | rollback procedure |
| `../../scripts/deploy-scavium-faucet-safe.sh` | safe deploy helper; review mode by default |
| `../../scripts/migrate-scavium-faucet-phase30.sh` | Phase 30 production migration helper that stages through a user-writable temp dir, executes privileged VPS changes through `REMOTE_SUDO`, preserves live env/nginx/systemd config, prints an advisory Phase 30 config audit, verifies pre-migration backup, archives the previous direct binary when needed, supports release-symlink and legacy direct-binary layouts, runs smoke checks from the VPS through the configured nginx/TLS URL with separate public/admin timeouts, and rolls back the symlink or binary on failure |

## Review-first deployment flow

For an existing production faucet moving to the Phase 30 binary, prefer the dedicated [Phase 30 production migration runbook](deployment-phase30-migration.md). It wraps the same release-layout assumptions with a verified pre-migration SQLite/config backup, post-activation smoke checks, and automatic symlink rollback if validation fails.

1. Build the binary outside the server and decide a `RELEASE_ID`.
2. Review and fill the environment example with real values **outside the repository**. Existing production env files are not overwritten by migration helpers.
3. Review and render the systemd and nginx templates with your final paths and domain only for first install or intentional config replacement. The Phase 30 binary migrator does not install these templates.
4. Use `scripts/deploy-scavium-faucet-safe.sh --plan` to inspect the exact staging commands.
5. If the plan looks correct, run the same script with `--execute`.
6. Install the reviewed systemd and nginx files manually on the VPS.
7. Follow the certbot guide only after DNS points to the VPS and nginx is syntactically valid.
8. Keep the Go service bound to loopback and expose only nginx on the public interface.

The environment in this repository is now both a review-first reference and a record of a successful real deployment on Debian 13 for `faucet.testnet.scavium.network`.

## Manual systemd installation

Template source:

```text
docs/scavium-faucet/deployment/scavium-faucet.service.template
```

Manual operator steps after review:

```bash
sudo install -o root -g root -m 0644 \
  ./scavium-faucet.service \
  /etc/systemd/system/SERVICE_NAME.service

sudo systemctl daemon-reload
sudo systemctl enable SERVICE_NAME.service
sudo systemctl restart SERVICE_NAME.service
sudo systemctl status SERVICE_NAME.service --no-pager
```

## Manual nginx installation

Template source:

```text
docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template
```

Manual operator steps after review:

```bash
sudo install -o root -g root -m 0644 \
  ./scavium-faucet.nginx.conf \
  /etc/nginx/sites-available/SERVICE_NAME.conf

sudo ln -sfn \
  /etc/nginx/sites-available/SERVICE_NAME.conf \
  /etc/nginx/sites-enabled/SERVICE_NAME.conf

sudo nginx -t
sudo systemctl reload nginx
```

Do not enable HSTS until HTTPS is working correctly and you are sure the hostname will stay permanent.

## Environment handling

Keep the real environment file outside the repository, for example:

```text
/etc/scavium-faucet/scavium-faucet.env
```

Start from:

```text
docs/scavium-faucet/deployment/scavium-faucet.env.example
```

The checked-in example intentionally keeps secret values as placeholders.

## Related guides

- [Certbot / ACME](deployment-certbot.md)
- [Firewall](deployment-firewall.md)
- [Rollback](deployment-rollback.md)

## Phase 23 backup and restore deployment hooks

Before each production deployment, rollback, or wallet-rotation maintenance window, create and verify a local backup bundle:

```bash
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
SCAVIUM_FAUCET_ENV_FILE=/etc/scavium-faucet/scavium-faucet.env \
SCAVIUM_FAUCET_BACKUP_DIR=/secure/offline/scavium-faucet-backups \
scripts/scavium-faucet-backup.sh --execute
```

Then verify it:

```bash
SCAVIUM_FAUCET_BACKUP_FILE=/secure/offline/scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
scripts/scavium-faucet-backup.sh --verify
```

Rollback verification after switching `current` back to an older release must include `/health`, `/ready`, `/api/v1/admin/runtime`, `/api/v1/admin/wallet`, and `scripts/scavium-faucet-operator-smoke.sh`. Keep the service in paused or maintenance mode until DB, queue, RPC, and wallet state are all coherent.
