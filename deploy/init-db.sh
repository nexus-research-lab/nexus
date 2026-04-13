#!/usr/bin/env bash

set -euo pipefail

: "${DATABASE_DRIVER:=sqlite}"
: "${DATABASE_URL:=sqlite:////home/agent/.nexus/data/nexus.db}"
export DATABASE_DRIVER
export DATABASE_URL

if [[ "${DATABASE_URL}" == sqlite:///* ]]; then
    DB_PATH="${DATABASE_URL#sqlite:///}"
    DB_PATH="${DB_PATH/#\~/${HOME}}"
    mkdir -p "$(dirname "${DB_PATH}")"
elif [[ "${DATABASE_URL}" == ~/* ]]; then
    DB_PATH="${DATABASE_URL/#\~/${HOME}}"
    mkdir -p "$(dirname "${DB_PATH}")"
fi

echo "Applying database migrations..."
/usr/local/bin/nexus-migrate up
echo "Database migration completed."

exec "$@"
