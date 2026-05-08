#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'

usage() {
    cat <<'USAGE'
Usage:
  DEPLOY_HOST=host \
  DEPLOY_USER=user \
  APP_PATH=/opt/scavium-faucet \
  LOCAL_BINARY=./scavium-faucet \
  scripts/migrate-scavium-faucet-phase30.sh [--plan|--execute]

Purpose:
  Operator-controlled migration helper for upgrading an existing production
  scavium-faucet service to a Phase 30-capable binary. The default mode is
  --plan and performs no remote writes.

Required environment:
  DEPLOY_HOST                 Target VPS hostname or IP.
  DEPLOY_USER                 SSH user.
  APP_PATH                    Release root, e.g. /opt/scavium-faucet.
  LOCAL_BINARY                Already-built new scavium-faucet binary.

Optional environment:
  SERVICE_NAME                systemd service name. Default: scavium-faucet
  RELEASE_ID                  Immutable release id. Default: UTC timestamp
  REMOTE_ENV_FILE             Reviewed env file. Default: /etc/scavium-faucet/scavium-faucet.env
  REMOTE_DB_PATH              SQLite DB path. Default: /var/lib/scavium-faucet/scavium-faucet.db
  REMOTE_BACKUP_DIR           Remote backup dir. Default: /var/backups/scavium-faucet
  LOCAL_BASE_URL              Health base URL on VPS. Default: http://127.0.0.1:18080
  SMOKE_ADMIN_CHECKS          yes/no. Default: yes when ADMIN token can be read by root from REMOTE_ENV_FILE
  POST_ACTIVATION_SLEEP       Seconds to wait before smoke. Default: 2
  MIGRATION_CONFIRM           Must be yes for --execute.
  KEEP_FAILED_RELEASE         yes/no. Default: yes

Safety:
  - --plan prints the remote plan and does not copy or modify anything.
  - --execute requires MIGRATION_CONFIRM=yes.
  - The current symlink target is captured before activation.
  - A remote backup is created and verified before activation.
  - The DB remains outside the release directory.
  - If post-activation smoke fails, the previous symlink is restored and the service is restarted.
  - This script does not edit nginx, certbot, firewall, or secrets.
USAGE
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

require_file() {
    local path="$1"
    [[ -f "$path" ]] || die "required file not found: $path"
}

require_value() {
    local name="$1"
    local value="${!name:-}"
    [[ -n "$value" ]] || die "required environment variable is empty: $name"
    case "$value" in
        DEPLOY_HOST|DEPLOY_USER|APP_PATH|LOCAL_BINARY|SERVICE_NAME)
            die "replace placeholder value before use: $name=$value"
            ;;
    esac
}

quote_sq() {
    printf "'%s'" "${1//\'/\'\\\'\'}"
}

run_remote() {
    local remote_cmd="$1"
    if [[ "$MODE" == "--plan" ]]; then
        printf '+ ssh %q %s\n' "$REMOTE" "$(quote_sq "$remote_cmd")"
        return
    fi
    ssh "$REMOTE" "$remote_cmd"
}

copy_remote() {
    local source="$1"
    local destination="$2"
    if [[ "$MODE" == "--plan" ]]; then
        printf '+ scp %q %q\n' "$source" "$REMOTE:$destination"
        return
    fi
    scp "$source" "$REMOTE:$destination"
}

MODE="${1:---plan}"
case "$MODE" in
    --plan|--execute) ;;
    -h|--help)
        usage
        exit 0
        ;;
    *)
        usage
        die "unsupported mode: $MODE"
        ;;
esac

if [[ "$MODE" == "--execute" ]]; then
    require_cmd ssh
    require_cmd scp
fi

DEPLOY_HOST="${DEPLOY_HOST:-DEPLOY_HOST}"
DEPLOY_USER="${DEPLOY_USER:-DEPLOY_USER}"
APP_PATH="${APP_PATH:-APP_PATH}"
LOCAL_BINARY="${LOCAL_BINARY:-./scavium-faucet}"
SERVICE_NAME="${SERVICE_NAME:-scavium-faucet}"
RELEASE_ID="${RELEASE_ID:-$(date -u +%Y%m%dT%H%M%SZ)-phase30}"
REMOTE_ENV_FILE="${REMOTE_ENV_FILE:-/etc/scavium-faucet/scavium-faucet.env}"
REMOTE_DB_PATH="${REMOTE_DB_PATH:-/var/lib/scavium-faucet/scavium-faucet.db}"
REMOTE_BACKUP_DIR="${REMOTE_BACKUP_DIR:-/var/backups/scavium-faucet}"
LOCAL_BASE_URL="${LOCAL_BASE_URL:-http://127.0.0.1:18080}"
SMOKE_ADMIN_CHECKS="${SMOKE_ADMIN_CHECKS:-yes}"
POST_ACTIVATION_SLEEP="${POST_ACTIVATION_SLEEP:-2}"
KEEP_FAILED_RELEASE="${KEEP_FAILED_RELEASE:-yes}"

require_value DEPLOY_HOST
require_value DEPLOY_USER
require_value APP_PATH
require_value LOCAL_BINARY
require_file "$LOCAL_BINARY"

if [[ "$MODE" == "--execute" ]]; then
    [[ "${MIGRATION_CONFIRM:-no}" == "yes" ]] || die "set MIGRATION_CONFIRM=yes to execute migration"
fi

REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
RELEASE_PATH="${APP_PATH}/releases/${RELEASE_ID}"
CURRENT_PATH="${APP_PATH}/current"
REMOTE_BINARY="${RELEASE_PATH}/scavium-faucet"
REMOTE_BACKUP_ID="${RELEASE_ID}-pre"
REMOTE_BACKUP_BUNDLE="${REMOTE_BACKUP_DIR}/scavium-faucet-backup-${REMOTE_BACKUP_ID}.tar.gz"

cat <<SUMMARY
======================================
SCAVIUM FAUCET PHASE 30 MIGRATION
Mode:                 $MODE
Remote:               $REMOTE
Service:              $SERVICE_NAME
Release ID:           $RELEASE_ID
Release path:         $RELEASE_PATH
Current symlink path: $CURRENT_PATH
DB path:              $REMOTE_DB_PATH
Env file:             $REMOTE_ENV_FILE
Backup bundle:        $REMOTE_BACKUP_BUNDLE
Smoke base URL:       $LOCAL_BASE_URL
======================================
SUMMARY

REMOTE_SCRIPT=$(cat <<'REMOTE'
set -euo pipefail

svc="__SERVICE_NAME__"
app_path="__APP_PATH__"
release_path="__RELEASE_PATH__"
current_path="__CURRENT_PATH__"
remote_binary="__REMOTE_BINARY__"
db_path="__REMOTE_DB_PATH__"
env_file="__REMOTE_ENV_FILE__"
backup_dir="__REMOTE_BACKUP_DIR__"
backup_id="__REMOTE_BACKUP_ID__"
backup_bundle="__REMOTE_BACKUP_BUNDLE__"
base_url="__LOCAL_BASE_URL__"
smoke_admin="__SMOKE_ADMIN_CHECKS__"
post_sleep="__POST_ACTIVATION_SLEEP__"
keep_failed="__KEEP_FAILED_RELEASE__"

fail() {
    echo "[migrate] ERROR: $*" >&2
    exit 1
}

curl_probe() {
    local name="$1"
    local url="$2"
    shift 2
    echo "[migrate] smoke: $name"
    curl -fsS --max-time 8 "$@" "$url" >/dev/null
}

[[ -x "$remote_binary" ]] || fail "new binary is missing or not executable: $remote_binary"
[[ -f "$env_file" ]] || fail "env file missing: $env_file"
[[ -f "$db_path" ]] || fail "database missing: $db_path"
command -v tar >/dev/null 2>&1 || fail "tar not installed"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum not installed"
command -v curl >/dev/null 2>&1 || fail "curl not installed"

previous_target=""
if [[ -L "$current_path" ]]; then
    previous_target="$(readlink -f "$current_path")"
elif [[ -e "$current_path" ]]; then
    fail "$current_path exists but is not a symlink"
fi
[[ -n "$previous_target" ]] || fail "current release symlink does not exist: $current_path"
[[ -x "$previous_target/scavium-faucet" ]] || fail "previous release binary is not executable: $previous_target/scavium-faucet"

echo "[migrate] previous release: $previous_target"

work_dir="${backup_dir}/.work-${backup_id}"
rm -rf "$work_dir"
umask 077
mkdir -p "$work_dir/db" "$work_dir/config" "$backup_dir"

if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$db_path" ".backup '${work_dir}/db/scavium-faucet.db'"
else
    cp -p "$db_path" "$work_dir/db/scavium-faucet.db"
    for suffix in -wal -shm; do
        if [[ -f "${db_path}${suffix}" ]]; then
            cp -p "${db_path}${suffix}" "$work_dir/db/scavium-faucet.db${suffix}"
        fi
    done
fi
cp -p "$env_file" "$work_dir/config/scavium-faucet.env"
cat > "$work_dir/MANIFEST.txt" <<MANIFEST
scavium-faucet phase30 pre-migration backup
created_utc=$backup_id
database_source=$db_path
env_source=$env_file
previous_release=$previous_target
new_release=$release_path
service=$svc
notes=Backup may contain secrets from the env file. Keep it encrypted/restricted.
MANIFEST
(
    cd "$work_dir"
    find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS
)
tar -C "$work_dir" -czf "$backup_bundle" .
rm -rf "$work_dir"
(
    verify_dir="$(mktemp -d)"
    trap 'rm -rf "$verify_dir"' EXIT
    tar -C "$verify_dir" -xzf "$backup_bundle"
    (cd "$verify_dir" && sha256sum -c SHA256SUMS >/dev/null)
)
echo "[migrate] backup verified: $backup_bundle"

ln -sfn "$release_path" "$current_path"
sudo systemctl restart "${svc}.service"
sudo systemctl is-active --quiet "${svc}.service"
sleep "$post_sleep"

set +e
curl_probe health "$base_url/health"
health_rc=$?
curl_probe ready "$base_url/ready"
ready_rc=$?
curl_probe status "$base_url/api/v1/status"
status_rc=$?
curl_probe tokens "$base_url/api/v1/tokens"
tokens_rc=$?
admin_rc=0
if [[ "$smoke_admin" == "yes" ]]; then
    admin_token=""
    if [[ -r "$env_file" ]]; then
        admin_token="$(grep -E '^SCAVIUM_FAUCET_ADMIN_TOKEN=' "$env_file" | tail -1 | cut -d= -f2- || true)"
    fi
    if [[ -n "$admin_token" ]]; then
        curl_probe admin-runtime "$base_url/api/v1/admin/runtime" -H "Authorization: Bearer ${admin_token}"
        admin_rc=$?
        if [[ "$admin_rc" -eq 0 ]]; then
            curl_probe admin-wallet "$base_url/api/v1/admin/wallet" -H "Authorization: Bearer ${admin_token}"
            admin_rc=$?
        fi
    else
        echo "[migrate] admin smoke skipped: token was not readable from env file"
    fi
fi
set -e

if [[ "$health_rc" -ne 0 || "$ready_rc" -ne 0 || "$status_rc" -ne 0 || "$tokens_rc" -ne 0 || "$admin_rc" -ne 0 ]]; then
    echo "[migrate] smoke failed; rolling back symlink to $previous_target" >&2
    ln -sfn "$previous_target" "$current_path"
    sudo systemctl restart "${svc}.service"
    sudo systemctl is-active --quiet "${svc}.service" || true
    if [[ "$keep_failed" != "yes" ]]; then
        rm -rf "$release_path"
    fi
    fail "migration failed and previous release was reactivated"
fi

echo "[migrate] migration completed"
echo "[migrate] active release: $(readlink -f "$current_path")"
echo "[migrate] rollback target: $previous_target"
echo "[migrate] backup bundle: $backup_bundle"
REMOTE
)

REMOTE_SCRIPT=${REMOTE_SCRIPT//__SERVICE_NAME__/$SERVICE_NAME}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__APP_PATH__/$APP_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__RELEASE_PATH__/$RELEASE_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__CURRENT_PATH__/$CURRENT_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BINARY__/$REMOTE_BINARY}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_DB_PATH__/$REMOTE_DB_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_ENV_FILE__/$REMOTE_ENV_FILE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_DIR__/$REMOTE_BACKUP_DIR}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_ID__/$REMOTE_BACKUP_ID}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_BUNDLE__/$REMOTE_BACKUP_BUNDLE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__LOCAL_BASE_URL__/$LOCAL_BASE_URL}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_ADMIN_CHECKS__/$SMOKE_ADMIN_CHECKS}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__POST_ACTIVATION_SLEEP__/$POST_ACTIVATION_SLEEP}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__KEEP_FAILED_RELEASE__/$KEEP_FAILED_RELEASE}

run_remote "mkdir -p $(quote_sq "$RELEASE_PATH")"
copy_remote "$LOCAL_BINARY" "$REMOTE_BINARY"
run_remote "chmod 0755 $(quote_sq "$REMOTE_BINARY")"

if [[ "$MODE" == "--plan" ]]; then
    printf '+ ssh %q %s\n' "$REMOTE" "'bash -s' <<'REMOTE_MIGRATION'"
    printf '%s\n' "$REMOTE_SCRIPT"
    printf '%s\n' 'REMOTE_MIGRATION'
    echo ""
    echo "Plan only. No remote changes were made."
    exit 0
fi

ssh "$REMOTE" 'bash -s' <<<"$REMOTE_SCRIPT"
