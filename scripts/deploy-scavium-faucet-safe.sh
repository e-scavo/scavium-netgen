#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'

usage() {
    cat <<'EOF'
Usage:
  DEPLOY_HOST=host \
  DEPLOY_USER=user \
  APP_PATH=/opt/scavium-faucet \
  LOCAL_BINARY=./scavium-faucet \
  scripts/deploy-scavium-faucet-safe.sh [--plan|--execute]

Defaults:
  --plan                     Print the exact commands and stop.
  --execute                  Run the staging commands.

Optional environment:
  SERVICE_NAME=scavium-faucet
  RELEASE_ID=manual-release-id
  ACTIVATE_RELEASE=no        Set to yes to switch current symlink and restart systemd.
  VERIFY_HEALTH=no           Set to yes to run curl http://127.0.0.1:18080/health after activation.
  INSTALL_REVIEW_BUNDLE=yes  Upload example env and reviewed templates into APP_PATH/review/RELEASE_ID.

Review bundle inputs:
  ENV_FILE_LOCAL=docs/scavium-faucet/deployment/scavium-faucet.env.example
  SYSTEMD_UNIT_LOCAL=docs/scavium-faucet/deployment/scavium-faucet.service.template
  NGINX_SITE_LOCAL=docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template

Safety notes:
  - This script never runs certbot or reloads nginx.
  - Activation is opt-in and disabled by default.
  - Fill placeholders before real use.
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

quote_sq() {
    printf "'%s'" "${1//\'/\'\\\'\'}"
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
        DEPLOY_HOST|DEPLOY_USER|APP_PATH|LOCAL_BINARY|DOMAIN|SERVICE_NAME|CHAIN_ID|NETWORK_NAME|SYMBOL)
            die "replace placeholder value before use: $name=$value"
            ;;
    esac
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

require_cmd ssh
require_cmd scp

DEPLOY_HOST="${DEPLOY_HOST:-DEPLOY_HOST}"
DEPLOY_USER="${DEPLOY_USER:-DEPLOY_USER}"
APP_PATH="${APP_PATH:-APP_PATH}"
LOCAL_BINARY="${LOCAL_BINARY:-./scavium-faucet}"
SERVICE_NAME="${SERVICE_NAME:-scavium-faucet}"
RELEASE_ID="${RELEASE_ID:-$(date -u +%Y%m%d%H%M%S)}"
ACTIVATE_RELEASE="${ACTIVATE_RELEASE:-no}"
VERIFY_HEALTH="${VERIFY_HEALTH:-no}"
INSTALL_REVIEW_BUNDLE="${INSTALL_REVIEW_BUNDLE:-yes}"

ENV_FILE_LOCAL="${ENV_FILE_LOCAL:-docs/scavium-faucet/deployment/scavium-faucet.env.example}"
SYSTEMD_UNIT_LOCAL="${SYSTEMD_UNIT_LOCAL:-docs/scavium-faucet/deployment/scavium-faucet.service.template}"
NGINX_SITE_LOCAL="${NGINX_SITE_LOCAL:-docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template}"

require_value DEPLOY_HOST
require_value DEPLOY_USER
require_value APP_PATH
require_value LOCAL_BINARY
require_file "$LOCAL_BINARY"

REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
RELEASE_PATH="${APP_PATH}/releases/${RELEASE_ID}"
REVIEW_PATH="${APP_PATH}/review/${RELEASE_ID}"
CURRENT_PATH="${APP_PATH}/current"
REMOTE_BINARY="${RELEASE_PATH}/scavium-faucet"

if [[ "$MODE" == "--execute" && "$ACTIVATE_RELEASE" == "yes" ]]; then
    [[ "${EXECUTE_DEPLOY:-no}" == "yes" ]] || die "set EXECUTE_DEPLOY=yes to allow activation"
fi

if [[ "$INSTALL_REVIEW_BUNDLE" == "yes" ]]; then
    require_file "$ENV_FILE_LOCAL"
    require_file "$SYSTEMD_UNIT_LOCAL"
    require_file "$NGINX_SITE_LOCAL"
fi

echo "======================================"
echo "SCAVIUM FAUCET SAFE DEPLOY"
echo "Mode:                 $MODE"
echo "Remote:               $REMOTE"
echo "Release ID:           $RELEASE_ID"
echo "Release path:         $RELEASE_PATH"
echo "Current symlink path: $CURRENT_PATH"
echo "Activate release:     $ACTIVATE_RELEASE"
echo "Verify health:        $VERIFY_HEALTH"
echo "Install review bundle:$INSTALL_REVIEW_BUNDLE"
echo "======================================"

run_remote "mkdir -p $(quote_sq "$RELEASE_PATH")"
copy_remote "$LOCAL_BINARY" "$REMOTE_BINARY"
run_remote "chmod 0755 $(quote_sq "$REMOTE_BINARY")"

if [[ "$INSTALL_REVIEW_BUNDLE" == "yes" ]]; then
    run_remote "mkdir -p $(quote_sq "$REVIEW_PATH")"
    copy_remote "$ENV_FILE_LOCAL" "${REVIEW_PATH}/scavium-faucet.env.example"
    copy_remote "$SYSTEMD_UNIT_LOCAL" "${REVIEW_PATH}/scavium-faucet.service.template"
    copy_remote "$NGINX_SITE_LOCAL" "${REVIEW_PATH}/scavium-faucet.nginx.conf.template"
fi

if [[ "$ACTIVATE_RELEASE" == "yes" ]]; then
    run_remote "ln -sfn $(quote_sq "$RELEASE_PATH") $(quote_sq "$CURRENT_PATH")"
    run_remote "sudo systemctl restart $(quote_sq "${SERVICE_NAME}.service")"
    run_remote "sudo systemctl status $(quote_sq "${SERVICE_NAME}.service") --no-pager"

    if [[ "$VERIFY_HEALTH" == "yes" ]]; then
        run_remote "curl -fsS http://127.0.0.1:18080/health"
    fi
fi

if [[ "$MODE" == "--plan" ]]; then
    echo ""
    echo "Plan only. No remote changes were made."
fi
