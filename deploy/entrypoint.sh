#!/bin/bash

set -euo pipefail

: "${DATABASE_URL:=sqlite+aiosqlite:////home/agent/.nexus/data/nexus.db}"
export DATABASE_URL

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

prepare_database_path() {
    if [[ "${DATABASE_URL}" == sqlite+aiosqlite:///* ]]; then
        DB_PATH="${DATABASE_URL#sqlite+aiosqlite:///}"
        DB_PATH="${DB_PATH/#\~/${HOME}}"
        mkdir -p "$(dirname "${DB_PATH}")"
        return
    fi

    if [[ "${DATABASE_URL}" == sqlite:///* ]]; then
        DB_PATH="${DATABASE_URL#sqlite:///}"
        DB_PATH="${DB_PATH/#\~/${HOME}}"
        mkdir -p "$(dirname "${DB_PATH}")"
    fi
}

print_environment_summary
prepare_claude_settings
prepare_database_path

echo "Applying database migrations..."
python -m alembic upgrade head
echo "Database migration completed."

exec "$@"
