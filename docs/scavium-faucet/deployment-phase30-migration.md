# Phase 30 production migration runbook

This runbook upgrades an existing production `scavium-faucet` deployment to the Phase 30 binary while preserving the durable SQLite database, the reviewed environment file, the nginx/TLS perimeter, and the previous binary/release rollback material.

Phase 30 adds optional wallet challenge/proof functionality. It does **not** break legacy claim clients: requests that omit `wallet_challenge_id` and `wallet_signature` continue through the existing claim path. Browser wallet integrations must use an explicitly allowed origin when the operator enables `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS`; native, desktop, mobile, CLI, and server-to-server clients that do not send `Origin` remain supported.

## Operator assumptions

- The current production service uses one of the supported layouts:
  - preferred release layout: `APP_PATH/current -> APP_PATH/releases/<release>`
  - legacy/direct binary layout: `APP_PATH/bin/scavium-faucet` as referenced by systemd
  - SQLite at `/var/lib/scavium-faucet/scavium-faucet.db` or another durable path outside releases
  - real environment file outside the repository, normally `/etc/scavium-faucet/scavium-faucet.env`
- The new binary has already been built and reviewed locally.
- The service remains loopback-only behind nginx.
- The migration is performed from an operator workstation with SSH access to the VPS.
- The SSH account may be a normal user. Privileged remote filesystem and systemd operations are executed through `REMOTE_SUDO`, which defaults to `sudo`.

## Pre-migration checklist

1. Build the new binary from the Phase 30 source ZIP:

   ```bash
   go build -o ./scavium-faucet ./cmd/scavium-faucet
   ```

   If the local environment cannot download Go 1.24/toolchain dependencies, build in the existing trusted build environment instead; do not copy a partially built or stale binary.

2. Review the production environment file and add wallet-origin configuration only when browser wallet proof flows are going public:

   ```ini
   # Browser app origins allowed to issue wallet challenges and submit wallet proof claims.
   SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS=https://faucet.testnet.scavium.network
   ```

   Leave it unset if Phase 30 is being deployed only to preserve legacy claims and internal/native clients. Do not use wildcard origins.

3. Confirm durable paths on the VPS:

   ```bash
   systemctl status scavium-faucet.service --no-pager
   readlink -f /opt/scavium-faucet/current || test -x /opt/scavium-faucet/bin/scavium-faucet
   test -f /var/lib/scavium-faucet/scavium-faucet.db
   test -f /etc/scavium-faucet/scavium-faucet.env
   ```

4. Run the migration helper in plan mode:

   ```bash
   DEPLOY_HOST=faucet.testnet.scavium.network \
   DEPLOY_USER=deploy \
   APP_PATH=/opt/scavium-faucet \
   LOCAL_BINARY=./scavium-faucet \
   SERVICE_NAME=scavium-faucet \
   REMOTE_SUDO=sudo \
   SMOKE_BASE_URL=https://faucet.testnet.scavium.network \
   scripts/migrate-scavium-faucet-phase30.sh --plan
   ```

   Review every printed command before executing.

## Execute migration

Run the same command with explicit confirmation:

```bash
DEPLOY_HOST=faucet.testnet.scavium.network \
DEPLOY_USER=deploy \
APP_PATH=/opt/scavium-faucet \
LOCAL_BINARY=./scavium-faucet \
SERVICE_NAME=scavium-faucet \
REMOTE_SUDO=sudo \
SMOKE_BASE_URL=https://faucet.testnet.scavium.network \
MIGRATION_CONFIRM=yes \
scripts/migrate-scavium-faucet-phase30.sh --execute
```

The helper performs the following sequence:

1. stages the new binary and generated migration script in a user-writable remote temp directory (`REMOTE_STAGE_DIR`)
2. enters the privileged section through `REMOTE_SUDO`
3. detects the active layout:
   - release symlink: installs the new binary into `APP_PATH/releases/RELEASE_ID/scavium-faucet`
   - legacy/direct binary: backs up and replaces `APP_PATH/bin/scavium-faucet`
4. captures the previous symlink target or direct binary rollback target
5. creates and verifies a remote pre-migration backup bundle containing:
   - SQLite database copied through `sqlite3 .backup` when available
   - WAL/SHM companions when a direct copy fallback is needed
   - the reviewed environment file
   - manifest with previous/new release metadata
   - previous direct binary as `binary/scavium-faucet.previous` when migrating a legacy/direct deployment
6. repoints `current` to the new release when using the release layout; direct-binary deployments are already replaced in place
7. restarts systemd
8. smoke-checks `/health`, `/ready`, `/api/v1/status`, and `/api/v1/tokens` from the VPS against `SMOKE_BASE_URL` using `SMOKE_TIMEOUT_SECONDS` (default `15`) and `SMOKE_RETRIES` (default `1`)
9. when the admin token is readable from the env file, smoke-checks `/api/v1/admin/runtime` and `/api/v1/admin/wallet` with the larger `SMOKE_ADMIN_TIMEOUT_SECONDS` window (default `30`) because admin wallet reads can be slower than public health/catalog endpoints
10. if smoke fails, prints `journalctl -u scavium-faucet.service` lines from the activation window before rollback so startup failures such as SQLite migration errors are visible in the operator terminal without being buried under older crawler traffic
11. automatically restores the previous symlink or previous direct binary and restarts the service if smoke fails


## Sudo and non-root VPS users

By default the helper assumes the VPS is accessed with a normal SSH user and escalates the privileged phase with `REMOTE_SUDO=sudo`. The local binary and generated migration script are copied first to `REMOTE_STAGE_DIR` under `/tmp`, avoiding direct `scp` writes to `/opt`, `/etc`, or `/var`. The privileged phase then installs the binary into the release directory, creates the backup, flips the symlink, restarts systemd, runs smoke checks, and performs rollback if needed.

Use one of these modes explicitly:

```bash
# Normal interactive VPS account; sudo may prompt for a password.
REMOTE_SUDO=sudo

# CI/passwordless sudo; fail immediately if sudo would prompt.
REMOTE_SUDO='sudo -n'

# Root SSH account only.
REMOTE_SUDO=''
```

If the VPS requires a sudo password, run the script from an interactive terminal so the `ssh -tt` privileged step can display the prompt. If sudo is not permitted for the deploy user, provision the required sudoers access before migration rather than making release paths world-writable.


## Configuration candidate migration

The migration helper treats production configuration as versioned operational state. By default it still preserves `/etc/scavium-faucet/scavium-faucet.env`, nginx site files, certbot files, firewall rules, and the installed systemd unit, but it now stages the repository env/nginx references on the VPS and generates root-readable candidates next to the active files. This gives operators an exact diff without copying secrets back to the workstation.

For the env file, the candidate is produced from `docs/scavium-faucet/deployment/scavium-faucet.env.example`: active production values are carried forward when the same key exists in the current template, new template keys remain in their documented/default/commented form, and active production keys that no longer exist in the current template are appended as `# legacy:` comments. This leaves an audit trail of previous settings without silently keeping obsolete configuration live.

For nginx, the repository template is staged as `${REMOTE_NGINX_SITE}.phase30-candidate` so the operator can inspect security headers, CSP, upstream, timeout, and proxy differences. The systemd template remains first-install/reference material and is not generated or installed by the Phase 30 migrator unless a separate manual first-install flow is being followed.

Default candidate-only flow:

```bash
CONFIG_CANDIDATES=yes \
LOCAL_ENV_EXAMPLE=docs/scavium-faucet/deployment/scavium-faucet.env.example \
LOCAL_NGINX_TEMPLATE=docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template \
REMOTE_NGINX_SITE=/etc/nginx/sites-available/scavium-faucet \
PHASE30_ENV_KEYS=SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS
```

After a successful run, review on the VPS:

```bash
sudo diff -u /etc/scavium-faucet/scavium-faucet.env /etc/scavium-faucet/scavium-faucet.env.phase30-candidate
sudo diff -u /etc/nginx/sites-available/scavium-faucet /etc/nginx/sites-available/scavium-faucet.phase30-candidate
```

Optional apply mode is explicit:

```bash
APPLY_ENV_CANDIDATE=yes       # install generated env candidate before service restart
APPLY_NGINX_CANDIDATE=yes     # install staged nginx candidate, run nginx -t, then reload nginx
```

If smoke validation fails after candidates were applied, the helper restores the previous env/nginx files from staging before rolling back the binary/symlink and restarting the service. The pre-migration backup bundle also includes the live env, live nginx site when present, and generated candidates for later manual recovery/review. Missing `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS` remains non-fatal: keep it absent for legacy/native-only rollout, or add exact browser origins before enabling browser wallet challenge/proof flows.

## Post-migration validation

After the helper succeeds, perform a second manual validation pass:

```bash
curl -fsS https://faucet.testnet.scavium.network/health
curl -fsS https://faucet.testnet.scavium.network/ready
curl -fsS https://faucet.testnet.scavium.network/api/v1/status
curl -fsS https://faucet.testnet.scavium.network/api/v1/tokens
curl -fsS https://faucet.testnet.scavium.network/api/v1/admin/runtime \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
curl -fsS https://faucet.testnet.scavium.network/api/v1/admin/wallet \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

For wallet proof readiness, verify that challenge issuance works from an allowed browser origin:

```bash
curl -fsS https://faucet.testnet.scavium.network/api/v1/wallet/challenge \
  -H 'Content-Type: application/json' \
  -H 'Origin: https://faucet.testnet.scavium.network' \
  -d '{"address":"0x0000000000000000000000000000000000000000"}'
```

Use a real wallet address for live validation. The response should include a challenge id, message, expiry timestamp, and the normalized address. Do not reconstruct the message client-side; the wallet must sign the server-provided message.

## Rollback

If the helper detects a failed smoke check, it automatically reactivates the previous symlink or restores the previous direct binary and restarts the service.

Manual rollback remains available:

```bash
sudo ln -sfn /opt/scavium-faucet/releases/PREVIOUS_RELEASE_ID /opt/scavium-faucet/current
sudo systemctl restart scavium-faucet.service
curl -fsS https://faucet.testnet.scavium.network/health
curl -fsS https://faucet.testnet.scavium.network/ready
```

If data/config restoration is required, use the pre-migration backup bundle reported by the helper with `scripts/scavium-faucet-restore.sh` from the same source tree, while the service is stopped. For legacy/direct binary deployments, the same backup bundle also contains `binary/scavium-faucet.previous`; extract it and install it back to `/opt/scavium-faucet/bin/scavium-faucet` with mode `0755` if a later manual binary rollback is needed after a successful migration.

## Edge cases to watch

- If `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS` is set, browser challenge/proof requests from non-listed origins should fail; legacy claims without wallet proof fields should not be blocked by this setting.
- If all smoke checks return nginx `502`, treat it as a backend startup/listen failure first. Inspect the journal lines printed by the migration helper; a common Phase 30 failure mode on older production databases is a partially applied SQLite migration without the corresponding `schema_migrations` row. The Phase 30 migrator now tolerates already-existing `ALTER TABLE ... ADD COLUMN` columns and continues adding any missing columns before recording the migration.
- If `/ready` fails after activation while `/health` succeeds, inspect DB path permissions, RPC chain-id compatibility, faucet wallet balance, and watcher/worker logs before retrying.
- If admin smoke is skipped because the env file is not readable by the SSH user, validate admin endpoints manually with the token from the operator secret store.
- Do not move the SQLite database into the release directory; rollbacks must switch binaries without losing queue, challenge, audit, campaign, allowlist, invitation, runtime policy, or claim state.

## Smoke URL selection

The migration helper runs smoke checks from the VPS after `systemctl restart`. The default `SMOKE_BASE_URL` is `https://DEPLOY_HOST`, which validates the real nginx/TLS path used by public clients. Do not rely on `http://127.0.0.1:18080` unless the active systemd environment is known to bind the Go service to that loopback port. For this VPS topology, pass:

```bash
SMOKE_BASE_URL=https://faucet.testnet.scavium.network
```

`LOCAL_BASE_URL` remains accepted as a backward-compatible alias only.


## Smoke timeout tuning

The helper treats public and admin smoke checks differently. Public endpoints should answer quickly, while `/api/v1/admin/wallet` may require a slower wallet/RPC-backed read and can legitimately approach the old 8 second curl limit. The defaults are therefore:

```bash
SMOKE_TIMEOUT_SECONDS=15
SMOKE_ADMIN_TIMEOUT_SECONDS=30
SMOKE_RETRIES=1
```

If the journal shows a smoke endpoint returned `status:200` just before rollback but curl reported `Operation timed out`, rerun with the current helper or explicitly increase the admin timeout:

```bash
SMOKE_ADMIN_TIMEOUT_SECONDS=45 \
MIGRATION_CONFIRM=yes \
scripts/migrate-scavium-faucet-phase30.sh --execute
```

Do not work around a real startup failure by increasing timeouts. If nginx returns `502` or the journal contains `create application failed`, fix the backend startup issue first.

## SQLite migration idempotency note

Production databases may have been upgraded by older binaries before `schema_migrations` was fully populated. In that state, columns such as `requests.token_id` can already exist while `004_token_claim_metadata.sql` is still considered pending. Phase 30's SQLite migration runner executes pending migration statements in a transaction and treats duplicate-column errors from `ALTER TABLE ... ADD COLUMN` as already-applied column additions, then continues with the remaining statements and records the migration only after the full migration completes. Non-duplicate DDL errors remain fatal and trigger the normal binary rollback path.
