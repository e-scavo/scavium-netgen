#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'

usage() {
    cat <<'USAGE'
Usage:
  scripts/scavium-faucet-backup.sh [--plan|--execute|--verify]

Purpose:
  Create an operator-controlled backup bundle for the faucet SQLite database and
  reviewed runtime configuration. The default mode is --plan and performs no writes.

Environment:
  SCAVIUM_FAUCET_DATABASE_PATH   SQLite DB path. Default: /var/lib/scavium-faucet/scavium-faucet.db
  SCAVIUM_FAUCET_ENV_FILE        Reviewed env file to copy when present. Default: /etc/scavium-faucet/scavium-faucet.env
  SCAVIUM_FAUCET_BACKUP_DIR      Backup output directory. Default: ./scavium-faucet-backups
  SCAVIUM_FAUCET_BACKUP_ID       Backup id. Default: UTC timestamp
  SCAVIUM_FAUCET_BACKUP_FILE     Existing .tar.gz bundle for --verify mode.

Safety:
  - --plan prints what would happen.
  - --execute creates a local tar.gz bundle and never uploads it.
  - --verify lists entries, rejects unsafe archive paths, and validates SHA256SUMS.
  - Secrets may exist in the copied env file; store bundles in restricted storage.
USAGE
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

mode="${1:---plan}"
case "$mode" in
    --plan|--execute|--verify) ;;
    -h|--help)
        usage
        exit 0
        ;;
    *)
        usage
        die "unsupported mode: $mode"
        ;;
esac

DB_PATH="${SCAVIUM_FAUCET_DATABASE_PATH:-/var/lib/scavium-faucet/scavium-faucet.db}"
ENV_FILE="${SCAVIUM_FAUCET_ENV_FILE:-/etc/scavium-faucet/scavium-faucet.env}"
BACKUP_DIR="${SCAVIUM_FAUCET_BACKUP_DIR:-./scavium-faucet-backups}"
BACKUP_ID="${SCAVIUM_FAUCET_BACKUP_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
BUNDLE_PATH="${BACKUP_DIR}/scavium-faucet-backup-${BACKUP_ID}.tar.gz"
WORK_DIR="${BACKUP_DIR}/.work-${BACKUP_ID}"
VERIFY_FILE="${SCAVIUM_FAUCET_BACKUP_FILE:-$BUNDLE_PATH}"

validate_bundle_listing() {
    local bundle="$1"
    local entry
    while IFS= read -r entry; do
        case "$entry" in
            /*|../*|*/../*|*"/.."|"..")
                die "unsafe path in backup bundle: $entry"
                ;;
        esac
    done < <(tar -tzf "$bundle")
}

verify_bundle() {
    local bundle="$1"
    local tmp_dir
    require_cmd tar
    require_cmd sha256sum
    [[ -f "$bundle" ]] || die "backup bundle not found: $bundle"
    echo "[backup] verifying bundle: $bundle"
    validate_bundle_listing "$bundle"
    tmp_dir="$(mktemp -d)"
    trap 'rm -rf "$tmp_dir"' RETURN
    tar -C "$tmp_dir" -xzf "$bundle"
    [[ -f "$tmp_dir/db/scavium-faucet.db" ]] || die "bundle missing db/scavium-faucet.db"
    [[ -f "$tmp_dir/SHA256SUMS" ]] || die "bundle missing SHA256SUMS"
    (cd "$tmp_dir" && sha256sum -c SHA256SUMS >/dev/null)
    tar -tzf "$bundle" | sed 's/^/[backup] contains: /'
    echo "[backup] checksum verification completed"
}

if [[ "$mode" == "--verify" ]]; then
    verify_bundle "$VERIFY_FILE"
    exit 0
fi

cat <<SUMMARY
[backup] mode:        $mode
[backup] database:    $DB_PATH
[backup] env file:    $ENV_FILE
[backup] backup dir:  $BACKUP_DIR
[backup] bundle:      $BUNDLE_PATH
SUMMARY

if [[ "$mode" == "--plan" ]]; then
    echo "[backup] plan only; no files will be written"
    if [[ -f "$DB_PATH" ]]; then
        echo "[backup] database exists and would be copied using sqlite3 .backup when available"
    else
        echo "[backup] database is not present; execute would fail until the path is corrected"
    fi
    if [[ -f "$ENV_FILE" ]]; then
        echo "[backup] env file exists and would be included as config/scavium-faucet.env"
    else
        echo "[backup] env file missing; execute would continue with a manifest note"
    fi
    exit 0
fi

[[ -f "$DB_PATH" ]] || die "database file not found: $DB_PATH"
require_cmd tar
require_cmd sha256sum

umask 077
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/db" "$WORK_DIR/config"

if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$DB_PATH" ".backup '${WORK_DIR}/db/scavium-faucet.db'"
else
    cp -p "$DB_PATH" "$WORK_DIR/db/scavium-faucet.db"
    for suffix in -wal -shm; do
        if [[ -f "${DB_PATH}${suffix}" ]]; then
            cp -p "${DB_PATH}${suffix}" "$WORK_DIR/db/scavium-faucet.db${suffix}"
        fi
    done
fi

if [[ -f "$ENV_FILE" ]]; then
    cp -p "$ENV_FILE" "$WORK_DIR/config/scavium-faucet.env"
fi

cat > "$WORK_DIR/MANIFEST.txt" <<MANIFEST
scavium-faucet backup
created_utc=$BACKUP_ID
database_source=$DB_PATH
env_source=$ENV_FILE
sqlite3_available=$(command -v sqlite3 >/dev/null 2>&1 && echo yes || echo no)
notes=Backup may contain secrets from the env file. Keep it encrypted/restricted.
MANIFEST

(
    cd "$WORK_DIR"
    find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS
)

mkdir -p "$BACKUP_DIR"
tar -C "$WORK_DIR" -czf "$BUNDLE_PATH" .
rm -rf "$WORK_DIR"

echo "[backup] created: $BUNDLE_PATH"
echo "[backup] verify with: SCAVIUM_FAUCET_BACKUP_FILE=$(printf '%q' "$BUNDLE_PATH") scripts/scavium-faucet-backup.sh --verify"
