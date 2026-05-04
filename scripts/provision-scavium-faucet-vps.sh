#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat <<'EOF'
Provision SCAVIUM faucet host (Debian 13 trixie).

Usage:
    sudo scripts/provision-scavium-faucet-vps.sh --binary /path/to/scavium-faucet [--execute] [--with-ufw] [--start-service]

Options:
  --binary PATH   Compiled scavium-faucet binary to install (required)
  --execute       Perform actions (default is --plan)
  --plan          Print planned commands only
  --with-ufw      Configure UFW rules (OpenSSH, 80, 443)
    --start-service Start/restart scavium-faucet only when env has no REPLACE_WITH placeholders
  -h, --help      Show this help

Notes:
  - Backend remains loopback-only via SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080.
  - This script does not run certbot; it prints manual next steps.
EOF
}

MODE="--plan"
WITH_UFW="no"
START_SERVICE="no"
BINARY_PATH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary)
            BINARY_PATH="${2:-}"
            shift 2
            ;;
        --execute)
            MODE="--execute"
            shift
            ;;
        --plan)
            MODE="--plan"
            shift
            ;;
        --with-ufw)
            WITH_UFW="yes"
            shift
            ;;
        --start-service)
            START_SERVICE="yes"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1" >&2
            usage
            exit 1
            ;;
    esac
done

if [[ -z "$BINARY_PATH" ]]; then
    echo "ERROR: --binary PATH is required" >&2
    usage
    exit 1
fi

if [[ ! -f "$BINARY_PATH" ]]; then
    echo "ERROR: binary not found: $BINARY_PATH" >&2
    exit 1
fi

if [[ "$EUID" -ne 0 ]]; then
    echo "ERROR: run as root (use sudo)" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

ENV_TEMPLATE="$REPO_ROOT/docs/scavium-faucet/deployment/scavium-faucet.env.example"
SERVICE_TEMPLATE="$REPO_ROOT/docs/scavium-faucet/deployment/scavium-faucet.service.template"
NGINX_TEMPLATE="$REPO_ROOT/docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template"

for f in "$ENV_TEMPLATE" "$SERVICE_TEMPLATE" "$NGINX_TEMPLATE"; do
    if [[ ! -f "$f" ]]; then
        echo "ERROR: required file missing: $f" >&2
        exit 1
    fi
done

run() {
    if [[ "$MODE" == "--plan" ]]; then
        echo "+ $*"
    else
        eval "$*"
    fi
}

env_has_placeholders() {
    local env_file="$1"
    if [[ ! -f "$env_file" ]]; then
        return 0
    fi
    grep -q "REPLACE_WITH" "$env_file"
}

write_bootstrap_nginx_config() {
    cat >/etc/nginx/sites-available/scavium-faucet <<'EOF'
limit_req_zone $binary_remote_addr zone=faucet_req:10m rate=10r/s;
limit_conn_zone $binary_remote_addr zone=faucet_conn:10m;

server {
    listen 80;
    listen [::]:80;
    server_name faucet.testnet.scavium.network;

    set_real_ip_from 127.0.0.1;
    real_ip_header X-Forwarded-For;
    real_ip_recursive on;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        limit_req zone=faucet_req burst=20 nodelay;
        limit_conn faucet_conn 20;
        add_header Retry-After 300 always;
        return 503;
    }

    error_page 503 @bootstrap_maintenance;
    location @bootstrap_maintenance {
        default_type text/plain;
        return 503 'TLS bootstrap in progress. Retry after certificate installation.';
    }
}
EOF
}

echo "======================================"
echo "SCAVIUM FAUCET VPS PROVISION"
echo "Mode: $MODE"
echo "Binary: $BINARY_PATH"
echo "With UFW: $WITH_UFW"
echo "Start service: $START_SERVICE"
echo "Domain: faucet.testnet.scavium.network"
echo "======================================"

# Packages
run "apt-get update"
run "apt-get install -y nginx certbot python3-certbot-nginx ca-certificates curl ufw"

# Service account
run "getent group scavium-faucet >/dev/null || groupadd --system scavium-faucet"
run "id scavium-faucet >/dev/null 2>&1 || useradd --system --gid scavium-faucet --home-dir /nonexistent --shell /usr/sbin/nologin scavium-faucet"

# Directories
run "install -d -o root -g root -m 0755 /opt/scavium-faucet"
run "install -d -o root -g root -m 0755 /opt/scavium-faucet/bin"
run "install -d -o root -g scavium-faucet -m 0750 /etc/scavium-faucet"
run "install -d -o scavium-faucet -g scavium-faucet -m 0750 /var/lib/scavium-faucet"
run "install -d -o root -g root -m 0755 /var/www/certbot"

# Binary
run "install -o root -g scavium-faucet -m 0750 '$BINARY_PATH' /opt/scavium-faucet/bin/scavium-faucet"

# Environment file (never overwrite existing)
run "if [[ -f /etc/scavium-faucet/scavium-faucet.env ]]; then cp -a /etc/scavium-faucet/scavium-faucet.env /etc/scavium-faucet/scavium-faucet.env.bak.$(date +%Y%m%d%H%M%S); fi"
run "if [[ ! -f /etc/scavium-faucet/scavium-faucet.env ]]; then install -o root -g scavium-faucet -m 0640 '$ENV_TEMPLATE' /etc/scavium-faucet/scavium-faucet.env; else install -o root -g scavium-faucet -m 0640 '$ENV_TEMPLATE' /etc/scavium-faucet/scavium-faucet.env.new; fi"

# systemd + nginx
run "install -o root -g root -m 0644 '$SERVICE_TEMPLATE' /etc/systemd/system/scavium-faucet.service"
# Install HTTPS-capable template as a reviewed reference, but bootstrap nginx with
# an HTTP-only config so first nginx -t does not depend on certbot cert files.
run "install -o root -g root -m 0644 '$NGINX_TEMPLATE' /etc/nginx/sites-available/scavium-faucet.https.template"
if [[ "$MODE" == "--plan" ]]; then
    echo "+ write HTTP-only bootstrap nginx config to /etc/nginx/sites-available/scavium-faucet"
else
    write_bootstrap_nginx_config
fi
run "ln -sfn /etc/nginx/sites-available/scavium-faucet /etc/nginx/sites-enabled/scavium-faucet"
run "if [[ -L /etc/nginx/sites-enabled/default ]]; then rm -f /etc/nginx/sites-enabled/default; fi"
run "nginx -t"

run "systemctl daemon-reload"
run "systemctl enable scavium-faucet.service"
run "systemctl reload nginx"

if [[ "$START_SERVICE" == "yes" ]]; then
    if [[ "$MODE" == "--plan" ]]; then
        echo "+ validate /etc/scavium-faucet/scavium-faucet.env has no REPLACE_WITH placeholders"
        echo "+ systemctl restart scavium-faucet.service"
    else
        if env_has_placeholders "/etc/scavium-faucet/scavium-faucet.env"; then
            echo "ERROR: /etc/scavium-faucet/scavium-faucet.env still contains REPLACE_WITH placeholders; refusing to start service" >&2
            exit 1
        fi
        systemctl restart scavium-faucet.service
    fi
fi

if [[ "$WITH_UFW" == "yes" ]]; then
    run "ufw allow OpenSSH"
    run "ufw allow 80/tcp"
    run "ufw allow 443/tcp"
    run "ufw deny 18080/tcp"
    run "ufw deny 18545/tcp"
    run "ufw --force enable"
    run "ufw status verbose"
fi

cat <<'EOF'

Manual next steps:
1. Edit /etc/scavium-faucet/scavium-faucet.env and fill REPLACE_WITH_* values.
2. Start the service only after the env file is complete:
    Option A: rerun this script with --execute --start-service
    Option B: systemctl start scavium-faucet.service
3. If the first boot fails, test by temporarily commenting MemoryDenyWriteExecute=true in /etc/systemd/system/scavium-faucet.service, then run:
    systemctl daemon-reload
    systemctl restart scavium-faucet.service
4. Confirm DNS A/AAAA for faucet.testnet.scavium.network points to this host.
5. Run certbot dry-run:
    certbot --nginx -d faucet.testnet.scavium.network --agree-tos --email OPS_EMAIL --redirect --dry-run
6. Run certbot live:
    certbot --nginx -d faucet.testnet.scavium.network --agree-tos --email OPS_EMAIL --redirect
7. After certbot succeeds, switch nginx to HTTPS template and reload:
    sudo cp /etc/nginx/sites-available/scavium-faucet.https.template /etc/nginx/sites-available/scavium-faucet
    sudo nginx -t && sudo systemctl reload nginx
8. Smoke checks after the service is running:
   curl -fsS http://127.0.0.1:18080/health
   curl -fsS http://127.0.0.1:18080/ready
   curl -fsS https://faucet.testnet.scavium.network/health
9. Logs:
   journalctl -u scavium-faucet.service -f
EOF
