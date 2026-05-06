#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'

usage() {
    cat <<'USAGE'
Usage:
  SCAVIUM_FAUCET_RESTORE_BUNDLE=./scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
  scripts/scavium-faucet-restore.sh [--plan|--execute]

Purpose:
  Restore a previously created faucet backup bundle to an operator-selected
  SQLite database path and, optionally, a separate env file path.

Environment:
  SCAVIUM_FAUCET_RESTORE_BUNDLE      Required backup .tar.gz bundle.
  SCAVIUM_FAUCET_DATABASE_PATH       Restore DB target. Default: /var/lib/scavium-faucet/scavium-faucet.db
  SCAVIUM_FAUCET_ENV_FILE            Optional env restore target. Default: /etc/scavium-faucet/scavium-faucet.env
  SCAVIUM_FAUCET_RESTORE_CONFIG      yes/no. Default: no
  SCAVIUM_FAUCET_SERVICE_NAME        systemd service guard. Default: scavium-faucet
  SCAVIUM_FAUCET_RESTORE_CONFIRM     Must be yes for --execute.
  SCAVIUM_FAUCET_ALLOW_LIVE_RESTORE  Must be yes to bypass active-service guard.

Safety:
  - Default mode is --plan and performs no writes.
  - --execute requires SCAVIUM_FAUCET_RESTORE_CONFIRM=yes.
  - Restore should be performed while the service is stopped.
  - This script never starts, stops, or restarts systemd by itself.
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
    --plan|--execute) ;;
    -h|--help)
        usage
        exit 0
        ;;
    *)
        usage
        die "unsupported mode: $mode"
        ;;
esac

BUNDLE="${SCAVIUM_FAUCET_RESTORE_BUNDLE:-}"
DB_PATH="${SCAVIUM_FAUCET_DATABASE_PATH:-/var/lib/scavium-faucet/scavium-faucet.db}"
ENV_FILE="${SCAVIUM_FAUCET_ENV_FILE:-/etc/scavium-faucet/scavium-faucet.env}"
RESTORE_CONFIG="${SCAVIUM_FAUCET_RESTORE_CONFIG:-no}"
SERVICE_NAME="${SCAVIUM_FAUCET_SERVICE_NAME:-scavium-faucet}"
ALLOW_LIVE_RESTORE="${SCAVIUM_FAUCET_ALLOW_LIVE_RESTORE:-no}"

[[ -n "$BUNDLE" ]] || die "SCAVIUM_FAUCET_RESTORE_BUNDLE is required"
[[ -f "$BUNDLE" ]] || die "restore bundle not found: $BUNDLE"
require_cmd tar
require_cmd sha256sum

cat <<SUMMARY
[restore] mode:           $mode
[restore] bundle:         $BUNDLE
[restore] database target:$DB_PATH
[restore] env target:     $ENV_FILE
[restore] restore config: $RESTORE_CONFIG
[restore] service guard:  $SERVICE_NAME
SUMMARY

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

tar -C "$TMP_DIR" -xzf "$BUNDLE"
[[ -f "$TMP_DIR/db/scavium-faucet.db" ]] || die "bundle does not contain db/scavium-faucet.db"

if [[ -f "$TMP_DIR/SHA256SUMS" ]]; then
    (cd "$TMP_DIR" && sha256sum -c SHA256SUMS >/dev/null)
fi

if [[ "$mode" == "--plan" ]]; then
    echo "[restore] plan only; no files will be written"
    echo "[restore] would install db/scavium-faucet.db to $DB_PATH"
    if [[ "$RESTORE_CONFIG" == "yes" && -f "$TMP_DIR/config/scavium-faucet.env" ]]; then
        echo "[restore] would install config/scavium-faucet.env to $ENV_FILE"
    fi
    echo "[restore] recommended sequence: stop service, execute restore, start service, check /ready and /api/v1/admin/wallet"
    exit 0
fi

[[ "${SCAVIUM_FAUCET_RESTORE_CONFIRM:-no}" == "yes" ]] || die "set SCAVIUM_FAUCET_RESTORE_CONFIRM=yes to execute restore"

if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && [[ "$ALLOW_LIVE_RESTORE" != "yes" ]]; then
        die "service $SERVICE_NAME is active; stop it first or set SCAVIUM_FAUCET_ALLOW_LIVE_RESTORE=yes intentionally"
    fi
fi

umask 077
mkdir -p "$(dirname "$DB_PATH")"
if [[ -f "$DB_PATH" ]]; then
    cp -p "$DB_PATH" "${DB_PATH}.pre-restore.$(date -u +%Y%m%dT%H%M%SZ)"
fi
install -m 0600 "$TMP_DIR/db/scavium-faucet.db" "$DB_PATH"

if [[ "$RESTORE_CONFIG" == "yes" ]]; then
    [[ -f "$TMP_DIR/config/scavium-faucet.env" ]] || die "config restore requested but bundle has no config/scavium-faucet.env"
    mkdir -p "$(dirname "$ENV_FILE")"
    if [[ -f "$ENV_FILE" ]]; then
        cp -p "$ENV_FILE" "${ENV_FILE}.pre-restore.$(date -u +%Y%m%dT%H%M%SZ)"
    fi
    install -m 0600 "$TMP_DIR/config/scavium-faucet.env" "$ENV_FILE"
fi

echo "[restore] completed"
echo "[restore] start the service manually, then verify /ready and /api/v1/admin/wallet"
