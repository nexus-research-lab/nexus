#!/bin/sh
set -eu

usage() {
    cat <<'EOF'
Usage: deploy/ssl-certbot.sh <issue|renew|dry-run|check>

Environment:
  ENV_FILE             .env path, default: <repo>/.env
  HOST_DATA_DIR        Host data dir, default: ./data
  NGINX_SERVER_NAME    Nginx server_name fallback for SSL_DOMAINS
  SSL_DOMAINS          Space or comma separated domains, for example: www.example.com example.com
  SSL_EMAIL            Optional Let's Encrypt account email
  SSL_CERT_NAME        Optional certbot lineage name, default: first domain
  SSL_FORCE_RENEWAL    true/false, force issue command to replace current cert
  SSL_STAGING          true/false, use Let's Encrypt staging
  SSL_RELOAD_NGINX     true/false, default: true
  CERTBOT_IMAGE        default: certbot/certbot:latest
EOF
}

die() {
    echo "Error: $*" >&2
    exit 1
}

is_true() {
    case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|on) return 0 ;;
        *) return 1 ;;
    esac
}

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname "$script_dir")
env_file=${ENV_FILE:-"$repo_root/.env"}

dotenv_value() {
    key=$1
    [ -f "$env_file" ] || return 0
    awk -v key="$key" '
        $0 ~ /^[[:space:]]*(#|$)/ { next }
        index($0, key "=") == 1 {
            sub("^[^=]*=", "")
            print
            exit
        }
    ' "$env_file"
}

strip_quotes() {
    value=$1
    case "$value" in
        \"*\") value=${value#\"}; value=${value%\"} ;;
        \'*\') value=${value#\'}; value=${value%\'} ;;
    esac
    printf '%s' "$value"
}

env_or_default() {
    key=$1
    fallback=$2
    current=$(printenv "$key" 2>/dev/null || true)
    if [ -n "$current" ]; then
        printf '%s' "$current"
        return
    fi
    value=$(dotenv_value "$key" || true)
    value=$(strip_quotes "$value")
    if [ -n "$value" ]; then
        printf '%s' "$value"
    else
        printf '%s' "$fallback"
    fi
}

resolve_host_data_dir() {
    value=$1
    case "$value" in
        /*) printf '%s' "$value" ;;
        ~|~/*) printf '%s%s' "$HOME" "${value#\~}" ;;
        *) printf '%s/%s' "$script_dir" "${value#./}" ;;
    esac
}

command=${1:-}
[ -n "$command" ] || {
    usage
    exit 2
}

host_data_dir=$(env_or_default HOST_DATA_DIR "./data")
nginx_server_name=$(env_or_default NGINX_SERVER_NAME "_")
ssl_domains=$(env_or_default SSL_DOMAINS "$nginx_server_name")
ssl_domains=$(printf '%s' "$ssl_domains" | tr ',' ' ')
ssl_email=$(env_or_default SSL_EMAIL "")
ssl_cert_name=$(env_or_default SSL_CERT_NAME "")
ssl_force_renewal=$(env_or_default SSL_FORCE_RENEWAL "false")
ssl_staging=$(env_or_default SSL_STAGING "false")
ssl_reload_nginx=$(env_or_default SSL_RELOAD_NGINX "true")
certbot_image=$(env_or_default CERTBOT_IMAGE "certbot/certbot:latest")

data_dir=$(resolve_host_data_dir "$host_data_dir")
certs_dir=$data_dir/certs
acme_dir=$data_dir/acme
challenge_dir=$acme_dir/.well-known/acme-challenge
compose_file=$script_dir/docker-compose.yml

domains=
for domain in $ssl_domains; do
    [ "$domain" = "_" ] && continue
    [ -n "$domain" ] || continue
    case "$domain" in
        "*."*) die "HTTP-01 不支持通配符域名: $domain" ;;
    esac
    domains="$domains $domain"
    if [ -z "$ssl_cert_name" ]; then
        ssl_cert_name=$domain
    fi
done

[ -n "$domains" ] || die "请设置 SSL_DOMAINS 或 NGINX_SERVER_NAME"

prepare_dirs() {
    mkdir -p "$certs_dir" "$challenge_dir"
}

run_certbot() {
    docker run --rm \
        -v "$certs_dir:/etc/letsencrypt" \
        -v "$acme_dir:/var/www/acme" \
        "$certbot_image" "$@"
}

reload_nginx() {
    is_true "$ssl_reload_nginx" || return 0
    [ -f "$compose_file" ] || return 0
    set -- -f "$compose_file"
    if [ -f "$env_file" ]; then
        set -- --env-file "$env_file" "$@"
    fi
    # 中文注释：证书首次出现后需要重新生成 nginx 配置，再 reload 才会开始监听 HTTPS。
    container_id=$(docker compose "$@" ps -q nginx 2>/dev/null || true)
    [ -n "$container_id" ] || return 0
    docker compose "$@" exec -T nginx \
        sh -lc '/docker-entrypoint.d/10-nexus-nginx.sh && nginx -s reload'
}

certbot_issue() {
    prepare_dirs
    set -- certonly --webroot --webroot-path /var/www/acme \
        --cert-name "$ssl_cert_name" \
        --agree-tos --non-interactive --keep-until-expiring
    if is_true "$ssl_force_renewal"; then
        set -- "$@" --force-renewal
    fi
    if [ -n "$ssl_email" ]; then
        set -- "$@" --email "$ssl_email"
    else
        set -- "$@" --register-unsafely-without-email
    fi
    if is_true "$ssl_staging"; then
        set -- "$@" --staging
    fi
    for domain in $domains; do
        set -- "$@" -d "$domain"
    done
    run_certbot "$@"
    reload_nginx
}

certbot_renew() {
    prepare_dirs
    set -- renew --authenticator webroot --webroot-path /var/www/acme --non-interactive
    if is_true "$ssl_staging"; then
        set -- "$@" --staging
    fi
    run_certbot "$@"
    reload_nginx
}

certbot_dry_run() {
    prepare_dirs
    run_certbot renew --authenticator webroot --webroot-path /var/www/acme --dry-run
}

check_challenge() {
    prepare_dirs
    token=nexus-ssl-check-$$
    printf 'ok\n' > "$challenge_dir/$token"
    trap 'rm -f "$challenge_dir/$token"' EXIT INT TERM
    for domain in $domains; do
        url="http://$domain/.well-known/acme-challenge/$token"
        printf 'Checking %s ... ' "$url"
        body=$(curl -fsS "$url" || true)
        [ "$body" = "ok" ] || die "ACME challenge 不通: $url"
        echo "ok"
    done
}

case "$command" in
    issue) certbot_issue ;;
    renew) certbot_renew ;;
    dry-run) certbot_dry_run ;;
    check) check_challenge ;;
    -h|--help|help) usage ;;
    *) usage; exit 2 ;;
esac
