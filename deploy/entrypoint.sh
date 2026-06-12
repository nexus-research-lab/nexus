#!/bin/bash

set -euo pipefail

: "${DATABASE_DRIVER:=sqlite}"
: "${DATABASE_URL:=sqlite:////home/agent/.nexus/data/nexus.db}"
: "${PNPM_REGISTRY:=https://registry.npmjs.org/}"
: "${BUN_CONFIG_REGISTRY:=${PNPM_REGISTRY}}"
: "${PIP_INDEX_URL:=https://pypi.tuna.tsinghua.edu.cn/simple}"
: "${PIP_BREAK_SYSTEM_PACKAGES:=1}"
: "${UV_DEFAULT_INDEX:=${PIP_INDEX_URL}}"
: "${UV_INDEX_URL:=${UV_DEFAULT_INDEX}}"
: "${UV_BREAK_SYSTEM_PACKAGES:=true}"
: "${AUTH_INIT_OWNER_PASSWORD:=}"
: "${CONNECTOR_CREDENTIALS_KEY:=}"
: "${NEXUS_DOCKER_REWRITE_LOOPBACK_PROXY:=true}"
export DATABASE_DRIVER
export DATABASE_URL
export PNPM_REGISTRY
export BUN_CONFIG_REGISTRY
export PIP_INDEX_URL
export PIP_BREAK_SYSTEM_PACKAGES
export UV_DEFAULT_INDEX
export UV_INDEX_URL
export UV_BREAK_SYSTEM_PACKAGES
export AUTH_INIT_OWNER_PASSWORD
export NEXUS_DOCKER_REWRITE_LOOPBACK_PROXY
export PATH="${HOME}/.local/bin:${HOME}/.bun/bin:${PATH}"

generate_connector_credentials_key() {
    python3 - <<'PY'
import base64
import os

print(base64.b64encode(os.urandom(32)).decode("ascii"))
PY
}

validate_connector_credentials_key() {
    local value="$1"
    CONNECTOR_CREDENTIALS_KEY_VALUE="${value}" python3 - <<'PY'
import base64
import os
import sys

value = os.environ.get("CONNECTOR_CREDENTIALS_KEY_VALUE", "").strip()
try:
    key = base64.b64decode(value, validate=True)
except Exception:
    sys.exit(1)
sys.exit(0 if len(key) == 32 else 1)
PY
}

prepare_connector_credentials_key() {
    local key_file="${CONNECTOR_CREDENTIALS_KEY_FILE:-${HOME}/.nexus/config/connector-credentials.key}"

    if [[ -z "${CONNECTOR_CREDENTIALS_KEY}" ]]; then
        mkdir -p "$(dirname "${key_file}")"
        if [[ -s "${key_file}" ]]; then
            CONNECTOR_CREDENTIALS_KEY="$(tr -d '[:space:]' < "${key_file}")"
        else
            CONNECTOR_CREDENTIALS_KEY="$(generate_connector_credentials_key)"
            local previous_umask
            previous_umask="$(umask)"
            umask 077
            printf '%s\n' "${CONNECTOR_CREDENTIALS_KEY}" > "${key_file}"
            umask "${previous_umask}"
            chmod 0600 "${key_file}"
            echo "Generated CONNECTOR_CREDENTIALS_KEY at ${key_file}"
        fi
    fi

    if ! validate_connector_credentials_key "${CONNECTOR_CREDENTIALS_KEY}"; then
        echo "ERROR: CONNECTOR_CREDENTIALS_KEY must be exactly 32 random bytes encoded as standard base64." >&2
        echo "Generate one with: openssl rand -base64 32" >&2
        echo "For Docker deployments, unset CONNECTOR_CREDENTIALS_KEY to let the entrypoint generate and persist one at ${key_file}." >&2
        exit 1
    fi

	export CONNECTOR_CREDENTIALS_KEY
}

rewrite_loopback_proxy_url() {
	local proxy_url="$1"
	PROXY_URL="${proxy_url}" python3 - <<'PY'
import os
import sys
from urllib.parse import urlsplit, urlunsplit

raw = os.environ.get("PROXY_URL", "").strip()
if not raw:
    print("")
    sys.exit(0)

had_scheme = "://" in raw
candidate = raw if had_scheme else f"http://{raw}"
try:
    parsed = urlsplit(candidate)
    host = parsed.hostname
    port = parsed.port
except ValueError:
    print(raw)
    sys.exit(0)

if host not in {"127.0.0.1", "localhost", "::1"}:
    print(raw)
    sys.exit(0)

userinfo = ""
netloc = parsed.netloc
if "@" in netloc:
    userinfo = netloc.rsplit("@", 1)[0] + "@"

new_netloc = userinfo + "host.docker.internal"
if port is not None:
    new_netloc += f":{port}"

rewritten = urlunsplit((parsed.scheme, new_netloc, parsed.path, parsed.query, parsed.fragment))
if not had_scheme and rewritten.startswith("http://"):
    rewritten = rewritten[len("http://"):]

print(rewritten)
PY
}

prepare_proxy_environment() {
	case "$(printf '%s' "${NEXUS_DOCKER_REWRITE_LOOPBACK_PROXY}" | tr '[:upper:]' '[:lower:]')" in
		1|true|yes|on) ;;
		*) return ;;
	esac

	local key value rewritten
	for key in HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy; do
		value="${!key:-}"
		if [[ -z "${value}" ]]; then
			continue
		fi

		rewritten="$(rewrite_loopback_proxy_url "${value}")"
		if [[ "${rewritten}" != "${value}" ]]; then
			printf -v "${key}" '%s' "${rewritten}"
			export "${key}"
			echo "Rewrote ${key} loopback proxy host to host.docker.internal"
		fi
	done
}

print_environment_summary() {
    echo "=== Environment Variables ==="
    while IFS='=' read -r key value; do
        # 中文注释：启动日志需要保留环境概览，但不能把敏感配置原样打进日志。
        if [[ "${key}" =~ (TOKEN|SECRET|PASSWORD|KEY) ]]; then
            if [[ -z "${value}" ]]; then
                value=""
            elif [[ ${#value} -le 8 ]]; then
                value="********"
            else
                value="${value:0:4}***${value: -4}"
            fi
        fi
        printf '%s=%s\n' "${key}" "${value}"
    done < <(env | sort)
    echo "============================="
    echo ""
}

add_env() {
    local key="$1"
    local value="${2:-}"
    if [[ -n "${value}" ]]; then
        SETTINGS_ENV="$(echo "${SETTINGS_ENV}" | jq --arg k "${key}" --arg v "${value}" '. + {($k): $v}')"
    fi
}

write_json_file_in_place() {
    local target_file="$1"
    local temp_file
    temp_file="$(mktemp /tmp/claude-json.XXXXXX)"
    cat > "${temp_file}"
    # 中文注释：.claude.json 可能是单文件 bind mount，不能用 mv 覆盖挂载点，只能原地写回。
    cat "${temp_file}" > "${target_file}"
    rm -f "${temp_file}"
}

extract_url_host() {
    local url="$1"
    local without_scheme="${url#*://}"
    without_scheme="${without_scheme%%/*}"
    without_scheme="${without_scheme%%\?*}"
    without_scheme="${without_scheme%%#*}"
    without_scheme="${without_scheme##*@}"
    without_scheme="${without_scheme%%:*}"
    printf '%s\n' "${without_scheme}"
}

prepare_runtime_toolchain_config() {
    local pip_host
    pip_host="$(extract_url_host "${PIP_INDEX_URL}")"

    mkdir -p \
        "${HOME}/.config/pip" \
        "${HOME}/.config/uv" \
        "${HOME}/.cache/pip"

    cat > "${HOME}/.npmrc" <<EOF
registry=${PNPM_REGISTRY}
EOF

    cat > "${HOME}/.bunfig.toml" <<EOF
[install]
registry = "${BUN_CONFIG_REGISTRY}"
EOF

    cat > "${HOME}/.config/uv/uv.toml" <<EOF
[[index]]
url = "${UV_DEFAULT_INDEX}"
default = true

[pip]
index-url = "${UV_INDEX_URL}"
EOF

    cat > "${HOME}/.config/pip/pip.conf" <<EOF
[global]
index-url = ${PIP_INDEX_URL}
break-system-packages = ${PIP_BREAK_SYSTEM_PACKAGES}
disable-pip-version-check = true
timeout = 60
EOF

    if [[ -n "${pip_host}" ]]; then
        cat >> "${HOME}/.config/pip/pip.conf" <<EOF
trusted-host = ${pip_host}
EOF
    fi
}

prepare_claude_settings() {
    mkdir -p "${HOME}/.claude"
    if [[ -d "${HOME}/.claude.json" ]]; then
        echo "ERROR: ${HOME}/.claude.json is a directory, expected a file"
        exit 1
    fi

    SETTINGS_ENV="{}"
    add_env "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS" "${CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS:-}"
    add_env "ENABLE_TOOL_SEARCH" "${ENABLE_TOOL_SEARCH:-}"

    SETTINGS="$(jq -n --argjson env_config "${SETTINGS_ENV}" '{env: $env_config}')"
    if [[ "${CLAUDE_DANGEROUSLY_SKIP_PERMISSIONS:-true}" == "true" ]]; then
        SETTINGS="$(echo "${SETTINGS}" | jq '. + {skipDangerousModePermissionPrompt: true}')"
    fi

    echo "${SETTINGS}" > "${HOME}/.claude/settings.json"
    echo "Settings written to ${HOME}/.claude/settings.json"

    if [[ ! -f "${HOME}/.claude.json" ]]; then
        echo '{}' > "${HOME}/.claude.json"
    fi

    jq '. + {hasCompletedOnboarding: true}' "${HOME}/.claude.json" | write_json_file_in_place "${HOME}/.claude.json"
}

resolve_sqlite_database_path() {
    local raw_path="$1"
    local normalized_path="${raw_path}"

    if [[ "${normalized_path}" == sqlite:///* ]]; then
        normalized_path="${normalized_path#sqlite:///}"
    fi

    if [[ "${normalized_path}" == \~/* ]]; then
        normalized_path="${HOME}/${normalized_path#\~/}"
    fi

    if [[ "${normalized_path}" == /* ]]; then
        printf '%s\n' "${normalized_path}"
    fi
}

prepare_database_path() {
    local database_driver
    database_driver="$(printf '%s' "${DATABASE_DRIVER}" | tr '[:upper:]' '[:lower:]')"
    case "${database_driver}" in
        sqlite|sqlite3)
            DB_PATH="$(resolve_sqlite_database_path "${DATABASE_URL}")"
            if [[ -n "${DB_PATH}" ]]; then
                # 中文注释：SQLite 文件型数据库需要先确保父目录存在，否则 server migration 会直接失败。
                mkdir -p "$(dirname "${DB_PATH}")"
            fi
            ;;
        *)
            return
            ;;
    esac
}

prepare_connector_credentials_key
prepare_proxy_environment
print_environment_summary
prepare_runtime_toolchain_config
prepare_claude_settings
prepare_database_path

exec "$@"
