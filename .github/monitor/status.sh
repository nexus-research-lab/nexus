#!/bin/bash
# GitHub Monitor Status

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${SCRIPT_DIR}/monitor.log"
CONFIG_FILE="${SCRIPT_DIR}/config.json"

echo "GitHub Monitor Status"
echo "===================="
echo ""

if [ -f "${CONFIG_FILE}" ]; then
    echo "📋 Configuration:"
    cat "${CONFIG_FILE}"
else
    echo "⚠️  No config file found"
fi

echo ""

if [ -f "${LOG_FILE}" ]; then
    echo "🕒 Last log line:"
    tail -1 "${LOG_FILE}"
else
    echo "⚠️  No monitor log found"
fi

echo ""
echo "💡 Commands:"
echo "  - Run now:   bash ${SCRIPT_DIR}/monitor.sh"
echo "  - View logs: tail -f ${LOG_FILE}"
