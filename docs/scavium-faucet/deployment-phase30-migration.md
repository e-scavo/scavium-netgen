# Phase 30 production migration runbook

This runbook upgrades an existing production `scavium-faucet` deployment to the Phase 30 binary while preserving the durable SQLite database, the reviewed environment file, the nginx/TLS perimeter, and the previous binary release for rollback.

Phase 30 adds optional wallet challenge/proof functionality. It does **not** break legacy claim clients: requests that omit `wallet_challenge_id` and `wallet_signature` continue through the existing claim path. Browser wallet integrations must use an explicitly allowed origin when the operator enables `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS`; native, desktop, mobile, CLI, and server-to-server clients that do not send `Origin` remain supported.

## Operator assumptions

- The current production service may use either supported binary layout:
  - release symlink: `APP_PATH/current -> APP_PATH/releases/<release>`
  - legacy/direct binary: `APP_PATH/bin/scavium-faucet` with no `APP_PATH/current` symlink
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
   ACTIVE_BINARY_PATH=/opt/scavium-faucet/bin/scavium-faucet \
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
ACTIVE_BINARY_PATH=/opt/scavium-faucet/bin/scavium-faucet \
MIGRATION_CONFIRM=yes \
scripts/migrate-scavium-faucet-phase30.sh --execute
```

The helper performs the following sequence:

1. stages the new binary and generated migration script in a user-writable remote temp directory (`REMOTE_STAGE_DIR`)
2. enters the privileged section through `REMOTE_SUDO`
3. installs the new binary into `APP_PATH/releases/RELEASE_ID/scavium-faucet` for auditability
4. detects activation mode:
   - `release_symlink` when `APP_PATH/current` exists and points to a previous release
   - `direct_binary` when `APP_PATH/current` is absent and `ACTIVE_BINARY_PATH` is executable
5. creates and verifies a remote pre-migration backup bundle containing:
   - SQLite database copied through `sqlite3 .backup` when available
   - WAL/SHM companions when a direct copy fallback is needed
   - the reviewed environment file
   - previous direct binary copy when the server uses `direct_binary` mode
   - manifest with activation mode and previous/new release metadata
6. activates the new binary:
   - repoints `current` to the new release in `release_symlink` mode
   - atomically replaces `ACTIVE_BINARY_PATH` in `direct_binary` mode
7. restarts systemd
8. smoke-checks `/health`, `/ready`, `/api/v1/status`, and `/api/v1/tokens`
9. when the admin token is readable from the env file, smoke-checks `/api/v1/admin/runtime` and `/api/v1/admin/wallet`
10. automatically restores the previous symlink or previous direct binary and restarts the service if smoke fails


## Existing server without `/opt/scavium-faucet/current`

Some already-running VPS installs use the systemd template path `ExecStart=/opt/scavium-faucet/bin/scavium-faucet` and do not yet have a `current` release symlink. This is supported. Leave `APP_PATH=/opt/scavium-faucet` and either rely on the default `ACTIVE_BINARY_PATH=/opt/scavium-faucet/bin/scavium-faucet` or set it explicitly. In this mode the helper keeps the immutable release copy for auditability, backs up the current direct binary, atomically replaces the direct binary, and restores that previous binary on failed smoke checks.

Do not create `/opt/scavium-faucet/current` manually just to satisfy the migration; the script detects the real active layout and uses the matching rollback strategy.

## Sudo and non-root VPS users

By default the helper assumes the VPS is accessed with a normal SSH user and escalates the privileged phase with `REMOTE_SUDO=sudo`. The local binary and generated migration script are copied first to `REMOTE_STAGE_DIR` under `/tmp`, avoiding direct `scp` writes to `/opt`, `/etc`, or `/var`. The privileged phase then installs the binary into the release directory, creates the backup, activates the detected layout, restarts systemd, runs smoke checks, and performs rollback if needed.

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

## Post-migration validation

After the helper succeeds, perform a second manual validation pass:

```bash
curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:18080/ready
curl -fsS http://127.0.0.1:18080/api/v1/status
curl -fsS http://127.0.0.1:18080/api/v1/tokens
curl -fsS http://127.0.0.1:18080/api/v1/admin/runtime \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
curl -fsS http://127.0.0.1:18080/api/v1/admin/wallet \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

For wallet proof readiness, verify that challenge issuance works from an allowed browser origin:

```bash
curl -fsS http://127.0.0.1:18080/api/v1/wallet/challenge \
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
curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:18080/ready
```

If data/config restoration is required, use the pre-migration backup bundle reported by the helper with `scripts/scavium-faucet-restore.sh` from the same source tree, while the service is stopped.

## Edge cases to watch

- If `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS` is set, browser challenge/proof requests from non-listed origins should fail; legacy claims without wallet proof fields should not be blocked by this setting.
- If `/ready` fails after activation, inspect DB path permissions, RPC chain-id compatibility, faucet wallet balance, and watcher/worker logs before retrying.
- If admin smoke is skipped because the env file is not readable by the SSH user, validate admin endpoints manually with the token from the operator secret store.
- Do not move the SQLite database into the release directory; rollbacks must switch binaries without losing queue, challenge, audit, campaign, allowlist, invitation, runtime policy, or claim state.
