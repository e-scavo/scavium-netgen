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
  DEPLOY_USER                 SSH user. May be a non-root sudo-capable user.
  APP_PATH                    Release root, e.g. /opt/scavium-faucet.
  LOCAL_BINARY                Already-built new scavium-faucet binary.

Optional environment:
  SERVICE_NAME                systemd service name. Default: scavium-faucet
  RELEASE_ID                  Immutable release id. Default: UTC timestamp
  REMOTE_ENV_FILE             Reviewed env file. Default: /etc/scavium-faucet/scavium-faucet.env
  REMOTE_DB_PATH              SQLite DB path. Default: /var/lib/scavium-faucet/scavium-faucet.db
  REMOTE_BACKUP_DIR           Remote backup dir. Default: /var/backups/scavium-faucet
  REMOTE_STAGE_DIR            User-writable remote staging dir. Default: /tmp/scavium-faucet-migration-RELEASE_ID
  REMOTE_SUDO                 Privilege escalation command. Default: sudo
                              Use "sudo -n" for non-interactive sudo, or empty when DEPLOY_USER is root.
  SMOKE_BASE_URL              Smoke URL used from the VPS. Default: https://DEPLOY_HOST
  LOCAL_BASE_URL              Backward-compatible alias for SMOKE_BASE_URL.
                              Set explicitly to http://127.0.0.1:18080 only when the service is known to listen there.
  SMOKE_ADMIN_CHECKS          yes/no. Default: yes when ADMIN token can be read by root from REMOTE_ENV_FILE
  POST_ACTIVATION_SLEEP       Seconds to wait before smoke. Default: 2
  FAILURE_JOURNAL_LINES       Journal lines printed before rollback on smoke failure. Default: 80
  SMOKE_TIMEOUT_SECONDS       Timeout for public smoke requests. Default: 15
  SMOKE_ADMIN_TIMEOUT_SECONDS Timeout for admin smoke requests. Default: 30
  SMOKE_RETRIES               Retry count for transient smoke curl failures. Default: 1
  CONFIG_AUDIT                yes/no. Default: yes. Backward-compatible alias for config candidate generation.
  CONFIG_CANDIDATES           yes/no. Default: CONFIG_AUDIT. Stages current repository env/nginx templates and generates VPS-side candidates.
  APPLY_ENV_CANDIDATE         yes/no. Default: no. When yes, installs the generated env candidate after backup and before restart.
  APPLY_NGINX_CANDIDATE       yes/no. Default: no. When yes, installs the staged nginx candidate, validates nginx -t, and reloads nginx.
  REMOTE_NGINX_SITE           Active nginx site to compare/apply. Default: /etc/nginx/sites-available/scavium-faucet
  LOCAL_ENV_EXAMPLE           Local env reference used for candidate merge. Default: docs/scavium-faucet/deployment/scavium-faucet.env.example
  LOCAL_NGINX_TEMPLATE        Local nginx reference staged as candidate. Default: docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template
  PHASE30_ENV_KEYS            Space-separated env keys to audit. Default: SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS
  MIGRATION_CONFIRM           Must be yes for --execute.
  KEEP_FAILED_RELEASE         yes/no. Default: yes

Safety:
  - --plan prints the remote plan and does not copy or modify anything.
  - --execute requires MIGRATION_CONFIRM=yes.
  - The local binary and generated migration script are first copied to REMOTE_STAGE_DIR.
  - Privileged filesystem/systemd work is performed remotely through REMOTE_SUDO.
  - The current symlink target is captured before activation when the server uses a release layout.
  - Legacy/direct binary deployments using APP_PATH/bin/scavium-faucet are supported and backed up before replacement.
  - A remote backup is created and verified before activation.
  - The DB remains outside the release directory.
  - If post-activation smoke fails, the previous symlink or direct binary is restored and the service is restarted.
  - By default this script does not overwrite nginx, certbot, firewall, systemd templates, env files, or secrets.
  - It can generate root-readable env/nginx candidate files on the VPS so operators can review exact differences without copying secrets back locally.
  - Env candidates preserve active values for keys still present in the new template, keep new template keys as-is, and append legacy active keys as comments when they no longer exist in the new template.
  - Env/nginx candidates are applied only with APPLY_ENV_CANDIDATE=yes and/or APPLY_NGINX_CANDIDATE=yes.
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

copy_text_remote() {
    local text="$1"
    local destination="$2"
    if [[ "$MODE" == "--plan" ]]; then
        printf '+ scp <generated migration script> %q\n' "$REMOTE:$destination"
        return
    fi
    local tmp_file
    tmp_file="$(mktemp)"
    trap 'rm -f "$tmp_file"' RETURN
    printf '%s\n' "$text" > "$tmp_file"
    scp "$tmp_file" "$REMOTE:$destination"
    rm -f "$tmp_file"
    trap - RETURN
}

run_remote_privileged_script() {
    local remote_script_path="$1"
    local remote_cmd
    if [[ -n "$REMOTE_SUDO" ]]; then
        remote_cmd="$REMOTE_SUDO bash $(quote_sq "$remote_script_path")"
    else
        remote_cmd="bash $(quote_sq "$remote_script_path")"
    fi

    if [[ "$MODE" == "--plan" ]]; then
        printf '+ ssh -tt %q %s\n' "$REMOTE" "$(quote_sq "$remote_cmd")"
        return
    fi

    # -tt allows sudo password prompts for operators whose VPS user is not passwordless sudo.
    ssh -tt "$REMOTE" "$remote_cmd"
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
    require_cmd mktemp
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
REMOTE_STAGE_DIR="${REMOTE_STAGE_DIR:-/tmp/scavium-faucet-migration-${RELEASE_ID}}"
REMOTE_SUDO="${REMOTE_SUDO-sudo}"
SMOKE_BASE_URL="${SMOKE_BASE_URL:-${LOCAL_BASE_URL:-https://${DEPLOY_HOST}}}"
SMOKE_ADMIN_CHECKS="${SMOKE_ADMIN_CHECKS:-yes}"
POST_ACTIVATION_SLEEP="${POST_ACTIVATION_SLEEP:-2}"
KEEP_FAILED_RELEASE="${KEEP_FAILED_RELEASE:-yes}"
FAILURE_JOURNAL_LINES="${FAILURE_JOURNAL_LINES:-80}"
SMOKE_TIMEOUT_SECONDS="${SMOKE_TIMEOUT_SECONDS:-15}"
SMOKE_ADMIN_TIMEOUT_SECONDS="${SMOKE_ADMIN_TIMEOUT_SECONDS:-30}"
SMOKE_RETRIES="${SMOKE_RETRIES:-1}"
CONFIG_AUDIT="${CONFIG_AUDIT:-yes}"
CONFIG_CANDIDATES="${CONFIG_CANDIDATES:-$CONFIG_AUDIT}"
APPLY_ENV_CANDIDATE="${APPLY_ENV_CANDIDATE:-no}"
APPLY_NGINX_CANDIDATE="${APPLY_NGINX_CANDIDATE:-no}"
REMOTE_NGINX_SITE="${REMOTE_NGINX_SITE:-/etc/nginx/sites-available/scavium-faucet}"
LOCAL_ENV_EXAMPLE="${LOCAL_ENV_EXAMPLE:-docs/scavium-faucet/deployment/scavium-faucet.env.example}"
LOCAL_NGINX_TEMPLATE="${LOCAL_NGINX_TEMPLATE:-docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template}"
PHASE30_ENV_KEYS="${PHASE30_ENV_KEYS:-SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS}"

require_value DEPLOY_HOST
require_value DEPLOY_USER
require_value APP_PATH
require_value LOCAL_BINARY
require_file "$LOCAL_BINARY"
if [[ "$CONFIG_CANDIDATES" == "yes" ]]; then
    require_file "$LOCAL_ENV_EXAMPLE"
    require_file "$LOCAL_NGINX_TEMPLATE"
fi

if [[ "$MODE" == "--execute" ]]; then
    [[ "${MIGRATION_CONFIRM:-no}" == "yes" ]] || die "set MIGRATION_CONFIRM=yes to execute migration"
fi

REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
RELEASE_PATH="${APP_PATH}/releases/${RELEASE_ID}"
CURRENT_PATH="${APP_PATH}/current"
REMOTE_BINARY="${RELEASE_PATH}/scavium-faucet"
LEGACY_BINARY_PATH="${APP_PATH}/bin/scavium-faucet"
REMOTE_STAGED_BINARY="${REMOTE_STAGE_DIR}/scavium-faucet"
REMOTE_STAGED_SCRIPT="${REMOTE_STAGE_DIR}/phase30-migration.sh"
REMOTE_STAGED_ENV_EXAMPLE="${REMOTE_STAGE_DIR}/scavium-faucet.env.example"
REMOTE_STAGED_NGINX_TEMPLATE="${REMOTE_STAGE_DIR}/scavium-faucet.nginx.conf.template"
REMOTE_BACKUP_ID="${RELEASE_ID}-pre"
REMOTE_BACKUP_BUNDLE="${REMOTE_BACKUP_DIR}/scavium-faucet-backup-${REMOTE_BACKUP_ID}.tar.gz"

cat <<SUMMARY
======================================
SCAVIUM FAUCET PHASE 30 MIGRATION
Mode:                 $MODE
Remote:               $REMOTE
Remote sudo:          ${REMOTE_SUDO:-<none>}
Remote stage dir:     $REMOTE_STAGE_DIR
Service:              $SERVICE_NAME
Release ID:           $RELEASE_ID
Release path:         $RELEASE_PATH
Current symlink path: $CURRENT_PATH
Legacy binary path:   $LEGACY_BINARY_PATH
DB path:              $REMOTE_DB_PATH
Env file:             $REMOTE_ENV_FILE
Backup bundle:        $REMOTE_BACKUP_BUNDLE
Smoke base URL:       $SMOKE_BASE_URL
Failure journal:      $FAILURE_JOURNAL_LINES lines
Smoke timeout:        ${SMOKE_TIMEOUT_SECONDS}s public / ${SMOKE_ADMIN_TIMEOUT_SECONDS}s admin
Smoke retries:        $SMOKE_RETRIES
Config candidates:    $CONFIG_CANDIDATES
Apply env candidate:  $APPLY_ENV_CANDIDATE
Apply nginx candidate:$APPLY_NGINX_CANDIDATE
Nginx site:           $REMOTE_NGINX_SITE
Phase 30 env keys:    $PHASE30_ENV_KEYS
======================================
SUMMARY

REMOTE_SCRIPT=$(cat <<'REMOTE'
set -euo pipefail

svc="__SERVICE_NAME__"
app_path="__APP_PATH__"
release_path="__RELEASE_PATH__"
current_path="__CURRENT_PATH__"
remote_binary="__REMOTE_BINARY__"
legacy_binary="__LEGACY_BINARY_PATH__"
staged_binary="__REMOTE_STAGED_BINARY__"
db_path="__REMOTE_DB_PATH__"
env_file="__REMOTE_ENV_FILE__"
backup_dir="__REMOTE_BACKUP_DIR__"
backup_id="__REMOTE_BACKUP_ID__"
backup_bundle="__REMOTE_BACKUP_BUNDLE__"
base_url="__SMOKE_BASE_URL__"
smoke_admin="__SMOKE_ADMIN_CHECKS__"
post_sleep="__POST_ACTIVATION_SLEEP__"
keep_failed="__KEEP_FAILED_RELEASE__"
remote_stage_dir="__REMOTE_STAGE_DIR__"
failure_journal_lines="__FAILURE_JOURNAL_LINES__"
smoke_timeout="__SMOKE_TIMEOUT_SECONDS__"
smoke_admin_timeout="__SMOKE_ADMIN_TIMEOUT_SECONDS__"
smoke_retries="__SMOKE_RETRIES__"
config_audit="__CONFIG_AUDIT__"
staged_env_example="__REMOTE_STAGED_ENV_EXAMPLE__"
staged_nginx_template="__REMOTE_STAGED_NGINX_TEMPLATE__"
remote_nginx_site="__REMOTE_NGINX_SITE__"
config_candidates="__CONFIG_CANDIDATES__"
apply_env_candidate="__APPLY_ENV_CANDIDATE__"
apply_nginx_candidate="__APPLY_NGINX_CANDIDATE__"
phase30_env_keys="__PHASE30_ENV_KEYS__"
activation_started_utc=""
env_candidate=""
nginx_candidate=""
env_pre_apply_backup="${remote_stage_dir}/scavium-faucet.env.pre-apply"
nginx_pre_apply_backup="${remote_stage_dir}/scavium-faucet.nginx.pre-apply"
env_candidate_applied="no"
nginx_candidate_applied="no"

fail() {
    echo "[migrate] ERROR: $*" >&2
    exit 1
}

curl_probe() {
    local name="$1"
    local timeout="$2"
    local url="$3"
    shift 3
    local attempt=0
    local rc=0

    echo "[migrate] smoke: $name"
    while true; do
        rc=0
        curl -fsS --max-time "$timeout" "$@" "$url" >/dev/null || rc=$?
        if [[ "$rc" -eq 0 ]]; then
            return 0
        fi
        if [[ "$attempt" -ge "$smoke_retries" ]]; then
            echo "[migrate] smoke failed: ${name} rc=${rc} url=${url} timeout=${timeout}s attempts=$((attempt + 1))" >&2
            return "$rc"
        fi
        attempt=$((attempt + 1))
        echo "[migrate] smoke retry: ${name} attempt=$((attempt + 1))" >&2
        sleep 1
    done
}

env_key_present() {
    local file="$1"
    local key="$2"
    grep -Eq "^[[:space:]]*(export[[:space:]]+)?${key}=" "$file"
}

print_config_audit() {
    [[ "$config_candidates" == "yes" ]] || return 0

    echo "[migrate] config candidates: production env/nginx/systemd are preserved unless APPLY_ENV_CANDIDATE=yes or APPLY_NGINX_CANDIDATE=yes"
    if [[ -f "$staged_env_example" ]]; then
        echo "[migrate] config candidates: env reference staged from repository: $staged_env_example"
    else
        echo "[migrate] config candidates warning: env reference not staged; skipping env candidate generation" >&2
    fi
    if [[ -f "$staged_nginx_template" ]]; then
        echo "[migrate] config candidates: nginx reference staged from repository: $staged_nginx_template"
    else
        echo "[migrate] config candidates warning: nginx reference not staged; skipping nginx candidate generation" >&2
    fi

    local key
    for key in $phase30_env_keys; do
        if env_key_present "$env_file" "$key"; then
            echo "[migrate] config audit: present in production env: $key"
        else
            echo "[migrate] config audit warning: missing optional Phase 30 env key: $key" >&2
            echo "[migrate] config audit warning: keep it unset for legacy/native-only rollout, or add exact browser origins before browser wallet proof flows go public" >&2
        fi
    done

    if command -v systemctl >/dev/null 2>&1; then
        systemctl cat "${svc}.service" 2>/dev/null | grep -E '^[[:space:]]*ExecStart=' | tail -1 | sed 's/^/[migrate] config audit: active systemd /' || true
    fi
}

generate_env_candidate() {
    [[ "$config_candidates" == "yes" ]] || return 0
    [[ -f "$staged_env_example" ]] || return 0

    env_candidate="${env_file}.phase30-candidate"
    awk '
    function extract_key(line, x, p) {
        x=line
        sub(/^[[:space:]]+/, "", x)
        if (x ~ /^#/) {
            sub(/^#[[:space:]]*/, "", x)
        }
        sub(/^export[[:space:]]+/, "", x)
        if (x ~ /^[A-Za-z_][A-Za-z0-9_]*=/) {
            p=index(x, "=")
            return substr(x, 1, p-1)
        }
        return ""
    }
    FNR==NR {
        k=extract_key($0)
        active=$0
        sub(/^[[:space:]]+/, "", active)
        if (k != "" && active !~ /^#/) {
            live[k]=$0
            live_order[++live_count]=k
        }
        next
    }
    {
        k=extract_key($0)
        if (k != "") {
            template[k]=1
            if ((k in live) && !used[k]) {
                print live[k]
                used[k]=1
                next
            }
        }
        print $0
    }
    END {
        legacy_header=0
        for (i=1; i<=live_count; i++) {
            k=live_order[i]
            if (!(k in template)) {
                if (!legacy_header) {
                    print ""
                    print "# Legacy active settings found in the previous production env but absent from the current repository template."
                    print "# They are intentionally commented here so operators can audit whether they are obsolete before deleting them."
                    legacy_header=1
                }
                print "# legacy: " live[k]
            }
        }
    }' "$env_file" "$staged_env_example" > "$env_candidate"
    chmod 0600 "$env_candidate"
    echo "[migrate] config candidate: generated env candidate at $env_candidate"
    echo "[migrate] config candidate: review on VPS with: sudo diff -u $env_file $env_candidate"
}

generate_nginx_candidate() {
    [[ "$config_candidates" == "yes" ]] || return 0
    [[ -f "$staged_nginx_template" ]] || return 0

    nginx_candidate="${remote_nginx_site}.phase30-candidate"
    install -m 0644 "$staged_nginx_template" "$nginx_candidate"
    echo "[migrate] config candidate: staged nginx candidate at $nginx_candidate"
    if [[ -f "$remote_nginx_site" ]]; then
        echo "[migrate] config candidate: review on VPS with: sudo diff -u $remote_nginx_site $nginx_candidate"
    else
        echo "[migrate] config candidate warning: active nginx site not found at $remote_nginx_site; candidate can be used for first install" >&2
    fi
}

apply_config_candidates_if_requested() {
    if [[ "$apply_env_candidate" == "yes" ]]; then
        [[ -n "${env_candidate:-}" && -f "$env_candidate" ]] || fail "env candidate missing; cannot apply"
        cp -p "$env_file" "$env_pre_apply_backup"
        install -m 0600 "$env_candidate" "$env_file"
        env_candidate_applied="yes"
        echo "[migrate] config apply: installed env candidate to $env_file"
    fi

    if [[ "$apply_nginx_candidate" == "yes" ]]; then
        [[ -n "${nginx_candidate:-}" && -f "$nginx_candidate" ]] || fail "nginx candidate missing; cannot apply"
        local nginx_backup="${remote_nginx_site}.pre-${backup_id}"
        if [[ -f "$remote_nginx_site" ]]; then
            cp -p "$remote_nginx_site" "$nginx_backup"
            cp -p "$remote_nginx_site" "$nginx_pre_apply_backup"
        fi
        install -m 0644 "$nginx_candidate" "$remote_nginx_site"
        if nginx -t; then
            systemctl reload nginx
            nginx_candidate_applied="yes"
            echo "[migrate] config apply: installed nginx candidate to $remote_nginx_site and reloaded nginx"
        else
            echo "[migrate] config apply: nginx -t failed; restoring previous nginx site" >&2
            if [[ -f "$nginx_backup" ]]; then
                install -m 0644 "$nginx_backup" "$remote_nginx_site"
                nginx -t >/dev/null 2>&1 && systemctl reload nginx || true
            fi
            fail "nginx candidate failed validation"
        fi
    fi
}

restore_applied_config_candidates() {
    if [[ "$env_candidate_applied" == "yes" && -f "$env_pre_apply_backup" ]]; then
        echo "[migrate] rollback: restoring previous env file to $env_file" >&2
        install -m 0600 "$env_pre_apply_backup" "$env_file"
    fi

    if [[ "$nginx_candidate_applied" == "yes" && -f "$nginx_pre_apply_backup" ]]; then
        echo "[migrate] rollback: restoring previous nginx site to $remote_nginx_site" >&2
        install -m 0644 "$nginx_pre_apply_backup" "$remote_nginx_site"
        nginx -t >/dev/null 2>&1 && systemctl reload nginx || true
    fi
}

print_failure_journal() {
    if [[ -n "$activation_started_utc" ]]; then
        echo "[migrate] service journal since activation ${activation_started_utc} (last ${failure_journal_lines} lines):" >&2
        journalctl -u "${svc}.service" --since "$activation_started_utc" -n "$failure_journal_lines" --no-pager >&2 || true
    else
        echo "[migrate] service journal before rollback (last ${failure_journal_lines} lines):" >&2
        journalctl -u "${svc}.service" -n "$failure_journal_lines" --no-pager >&2 || true
    fi
}

[[ "$(id -u)" -eq 0 ]] || fail "privileged migration script must run as root; set REMOTE_SUDO=sudo or run with a root DEPLOY_USER"
[[ -x "$staged_binary" ]] || fail "staged binary is missing or not executable: $staged_binary"
[[ -f "$env_file" ]] || fail "env file missing: $env_file"
[[ -f "$db_path" ]] || fail "database missing: $db_path"
command -v install >/dev/null 2>&1 || fail "install not installed"
command -v tar >/dev/null 2>&1 || fail "tar not installed"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum not installed"
command -v curl >/dev/null 2>&1 || fail "curl not installed"
command -v systemctl >/dev/null 2>&1 || fail "systemctl not installed"

print_config_audit
generate_env_candidate
generate_nginx_candidate

activation_mode=""
previous_target=""
previous_direct_backup="${remote_stage_dir}/previous-scavium-faucet"

if [[ -L "$current_path" ]]; then
    activation_mode="release_symlink"
    previous_target="$(readlink -f "$current_path")"
    [[ -x "$previous_target/scavium-faucet" ]] || fail "previous release binary is not executable: $previous_target/scavium-faucet"
elif [[ -e "$current_path" ]]; then
    fail "$current_path exists but is not a symlink"
elif [[ -x "$legacy_binary" ]]; then
    activation_mode="direct_binary"
    previous_target="$legacy_binary"
    cp -p "$legacy_binary" "$previous_direct_backup"
    chmod 0755 "$previous_direct_backup"
else
    fail "no supported active binary found; expected $current_path symlink or executable $legacy_binary"
fi

echo "[migrate] activation mode: $activation_mode"
echo "[migrate] previous target: $previous_target"

if [[ "$activation_mode" == "release_symlink" ]]; then
    install -d -m 0755 "$release_path"
    install -m 0755 "$staged_binary" "$remote_binary"
    [[ -x "$remote_binary" ]] || fail "new binary is missing or not executable after install: $remote_binary"
else
    # In direct-binary mode, do not replace the active on-disk binary until
    # after the pre-migration SQLite/env backup has been created and verified.
    install -d -m 0755 "$(dirname "$legacy_binary")"
fi

work_dir="${backup_dir}/.work-${backup_id}"
rm -rf "$work_dir"
umask 077
mkdir -p "$work_dir/db" "$work_dir/config" "$backup_dir"
if [[ "$activation_mode" == "direct_binary" ]]; then
    mkdir -p "$work_dir/binary"
    cp -p "$previous_direct_backup" "$work_dir/binary/scavium-faucet.previous"
    chmod 0755 "$work_dir/binary/scavium-faucet.previous"
fi

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
if [[ -f "$remote_nginx_site" ]]; then
    cp -p "$remote_nginx_site" "$work_dir/config/scavium-faucet.nginx.conf"
fi
if [[ -n "${env_candidate:-}" && -f "$env_candidate" ]]; then
    cp -p "$env_candidate" "$work_dir/config/scavium-faucet.env.phase30-candidate"
fi
if [[ -n "${nginx_candidate:-}" && -f "$nginx_candidate" ]]; then
    cp -p "$nginx_candidate" "$work_dir/config/scavium-faucet.nginx.conf.phase30-candidate"
fi
cat > "$work_dir/MANIFEST.txt" <<MANIFEST
scavium-faucet phase30 pre-migration backup
created_utc=$backup_id
database_source=$db_path
env_source=$env_file
nginx_source=$remote_nginx_site
config_candidates=$config_candidates
apply_env_candidate=$apply_env_candidate
apply_nginx_candidate=$apply_nginx_candidate
activation_mode=$activation_mode
previous_target=$previous_target
new_release=$release_path
legacy_binary=$legacy_binary
previous_direct_binary_backup=binary/scavium-faucet.previous when activation_mode=direct_binary
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
apply_config_candidates_if_requested

if [[ "$activation_mode" == "release_symlink" ]]; then
    ln -sfn "$release_path" "$current_path"
else
    install -m 0755 "$staged_binary" "$legacy_binary"
    [[ -x "$legacy_binary" ]] || fail "new binary is missing or not executable after direct install: $legacy_binary"
fi
activation_started_utc="$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
systemctl restart "${svc}.service"
systemctl is-active --quiet "${svc}.service"
sleep "$post_sleep"

set +e
curl_probe health "$smoke_timeout" "$base_url/health"
health_rc=$?
curl_probe ready "$smoke_timeout" "$base_url/ready"
ready_rc=$?
curl_probe status "$smoke_timeout" "$base_url/api/v1/status"
status_rc=$?
curl_probe tokens "$smoke_timeout" "$base_url/api/v1/tokens"
tokens_rc=$?
admin_rc=0
if [[ "$smoke_admin" == "yes" ]]; then
    admin_token=""
    if [[ -r "$env_file" ]]; then
        admin_token="$(grep -E '^SCAVIUM_FAUCET_ADMIN_TOKEN=' "$env_file" | tail -1 | cut -d= -f2- || true)"
    fi
    if [[ -n "$admin_token" ]]; then
        curl_probe admin-runtime "$smoke_admin_timeout" "$base_url/api/v1/admin/runtime" -H "Authorization: Bearer ${admin_token}"
        admin_rc=$?
        if [[ "$admin_rc" -eq 0 ]]; then
            curl_probe admin-wallet "$smoke_admin_timeout" "$base_url/api/v1/admin/wallet" -H "Authorization: Bearer ${admin_token}"
            admin_rc=$?
        fi
    else
        echo "[migrate] admin smoke skipped: token was not readable from env file"
    fi
fi
set -e

if [[ "$health_rc" -ne 0 || "$ready_rc" -ne 0 || "$status_rc" -ne 0 || "$tokens_rc" -ne 0 || "$admin_rc" -ne 0 ]]; then
    print_failure_journal
    restore_applied_config_candidates
    if [[ "$activation_mode" == "release_symlink" ]]; then
        echo "[migrate] smoke failed; rolling back symlink to $previous_target" >&2
        ln -sfn "$previous_target" "$current_path"
        rollback_message="previous release was reactivated"
    else
        echo "[migrate] smoke failed; restoring previous direct binary to $legacy_binary" >&2
        install -m 0755 "$previous_direct_backup" "$legacy_binary"
        rollback_message="previous direct binary was restored"
    fi
    systemctl restart "${svc}.service"
    systemctl is-active --quiet "${svc}.service" || true
    if [[ "$keep_failed" != "yes" && "$activation_mode" == "release_symlink" ]]; then
        rm -rf "$release_path"
    fi
    fail "migration failed and $rollback_message"
fi

rm -rf "$remote_stage_dir"

echo "[migrate] migration completed"
if [[ "$activation_mode" == "release_symlink" ]]; then
    echo "[migrate] active release: $(readlink -f "$current_path")"
else
    echo "[migrate] active binary: $legacy_binary"
fi
echo "[migrate] rollback target: $previous_target"
if [[ "$activation_mode" == "direct_binary" ]]; then
    echo "[migrate] previous direct binary archived in backup bundle: binary/scavium-faucet.previous"
fi
echo "[migrate] backup bundle: $backup_bundle"
REMOTE
)

REMOTE_SCRIPT=${REMOTE_SCRIPT//__SERVICE_NAME__/$SERVICE_NAME}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__APP_PATH__/$APP_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__RELEASE_PATH__/$RELEASE_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__CURRENT_PATH__/$CURRENT_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BINARY__/$REMOTE_BINARY}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__LEGACY_BINARY_PATH__/$LEGACY_BINARY_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_STAGED_BINARY__/$REMOTE_STAGED_BINARY}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_DB_PATH__/$REMOTE_DB_PATH}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_ENV_FILE__/$REMOTE_ENV_FILE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_DIR__/$REMOTE_BACKUP_DIR}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_ID__/$REMOTE_BACKUP_ID}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_BACKUP_BUNDLE__/$REMOTE_BACKUP_BUNDLE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_BASE_URL__/$SMOKE_BASE_URL}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_ADMIN_CHECKS__/$SMOKE_ADMIN_CHECKS}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__POST_ACTIVATION_SLEEP__/$POST_ACTIVATION_SLEEP}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__KEEP_FAILED_RELEASE__/$KEEP_FAILED_RELEASE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_STAGE_DIR__/$REMOTE_STAGE_DIR}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__FAILURE_JOURNAL_LINES__/$FAILURE_JOURNAL_LINES}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_TIMEOUT_SECONDS__/$SMOKE_TIMEOUT_SECONDS}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_ADMIN_TIMEOUT_SECONDS__/$SMOKE_ADMIN_TIMEOUT_SECONDS}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__SMOKE_RETRIES__/$SMOKE_RETRIES}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__CONFIG_AUDIT__/$CONFIG_AUDIT}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__CONFIG_CANDIDATES__/$CONFIG_CANDIDATES}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__APPLY_ENV_CANDIDATE__/$APPLY_ENV_CANDIDATE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__APPLY_NGINX_CANDIDATE__/$APPLY_NGINX_CANDIDATE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_NGINX_SITE__/$REMOTE_NGINX_SITE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_STAGED_ENV_EXAMPLE__/$REMOTE_STAGED_ENV_EXAMPLE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__REMOTE_STAGED_NGINX_TEMPLATE__/$REMOTE_STAGED_NGINX_TEMPLATE}
REMOTE_SCRIPT=${REMOTE_SCRIPT//__PHASE30_ENV_KEYS__/$PHASE30_ENV_KEYS}

run_remote "rm -rf $(quote_sq "$REMOTE_STAGE_DIR") && mkdir -p $(quote_sq "$REMOTE_STAGE_DIR") && chmod 0700 $(quote_sq "$REMOTE_STAGE_DIR")"
copy_remote "$LOCAL_BINARY" "$REMOTE_STAGED_BINARY"
run_remote "chmod 0755 $(quote_sq "$REMOTE_STAGED_BINARY")"
if [[ "$CONFIG_CANDIDATES" == "yes" ]]; then
    copy_remote "$LOCAL_ENV_EXAMPLE" "$REMOTE_STAGED_ENV_EXAMPLE"
    run_remote "chmod 0600 $(quote_sq "$REMOTE_STAGED_ENV_EXAMPLE")"
    copy_remote "$LOCAL_NGINX_TEMPLATE" "$REMOTE_STAGED_NGINX_TEMPLATE"
    run_remote "chmod 0644 $(quote_sq "$REMOTE_STAGED_NGINX_TEMPLATE")"
fi
copy_text_remote "$REMOTE_SCRIPT" "$REMOTE_STAGED_SCRIPT"
run_remote "chmod 0600 $(quote_sq "$REMOTE_STAGED_SCRIPT")"

if [[ "$MODE" == "--plan" ]]; then
    echo ""
    echo "The generated remote script will be executed through REMOTE_SUDO."
    echo "Use REMOTE_SUDO='sudo -n' for passwordless/non-interactive sudo checks, REMOTE_SUDO='sudo' for an interactive sudo prompt, or REMOTE_SUDO='' when DEPLOY_USER is root."
fi

run_remote_privileged_script "$REMOTE_STAGED_SCRIPT"

if [[ "$MODE" == "--plan" ]]; then
    echo ""
    echo "Plan only. No remote changes were made."
fi
