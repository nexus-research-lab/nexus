#!/bin/bash
# GitHub PR & Branch Monitor for nexus-core

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CONFIG="${SCRIPT_DIR}/config.json"
LOG="${SCRIPT_DIR}/monitor.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "${LOG}"
}

require_bin() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "缺少依赖命令: $1" >&2
        exit 1
    fi
}

require_bin git
require_bin gh
require_bin jq
require_bin codex

if [ ! -f "${CONFIG}" ]; then
    echo "缺少配置文件: ${CONFIG}" >&2
    exit 1
fi

REPO="$(jq -r '.repo' "${CONFIG}")"
MAIN_BRANCH="$(jq -r '.mainBranch' "${CONFIG}")"

cd "${REPO_DIR}"

log "Fetching latest changes..."
git fetch --all --prune 2>&1 | grep -v "From https" || true

CURRENT_BRANCH="$(git branch --show-current)"

log "Checking for open PRs..."
PRS="$(gh pr list --repo "${REPO}" --state open --json number,title,headRefName,baseRefName,mergeable,mergeStateStatus 2>&1)"

if [ "${PRS}" != "[]" ] && [ -n "${PRS}" ]; then
    log "Found open PRs"

    echo "${PRS}" | jq -c '.[]' | while read -r pr; do
        PR_NUM="$(echo "${pr}" | jq -r '.number')"
        PR_TITLE="$(echo "${pr}" | jq -r '.title')"
        PR_BRANCH="$(echo "${pr}" | jq -r '.headRefName')"
        MERGEABLE="$(echo "${pr}" | jq -r '.mergeable')"
        MERGE_STATE="$(echo "${pr}" | jq -r '.mergeStateStatus')"

        log "Processing PR #${PR_NUM}: ${PR_TITLE}"

        if [ "${MERGEABLE}" != "MERGEABLE" ]; then
            log "PR #${PR_NUM} not mergeable (state: ${MERGE_STATE})"
            continue
        fi

        git stash push -u -m "github-monitor-temp" >/dev/null 2>&1 || true
        git checkout "${PR_BRANCH}" >/dev/null 2>&1 || true
        git pull origin "${PR_BRANCH}" 2>&1 | grep -v "From https" || true

        log "Reviewing PR #${PR_NUM} with Codex..."
        REVIEW="$(codex review --base "${MAIN_BRANCH}" 2>&1 || echo "REVIEW_FAILED")"

        if echo "${REVIEW}" | grep -qiE "(approve|approved|looks good|no issues|lgtm|ready to merge)"; then
            log "PR #${PR_NUM} approved, merging..."
            gh pr merge "${PR_NUM}" --repo "${REPO}" --squash --delete-branch 2>&1 || log "Merge PR #${PR_NUM} failed"
        else
            log "PR #${PR_NUM} rejected"
            gh pr comment "${PR_NUM}" --repo "${REPO}" --body "🤖 自动巡检未通过，请根据 Codex review 输出继续处理。" 2>&1 || true
        fi

        git checkout "${MAIN_BRANCH}" >/dev/null 2>&1 || true
        git stash pop >/dev/null 2>&1 || true
    done
else
    log "No open PRs found"
fi

log "Checking branch status..."
git branch -r | grep -v HEAD | grep -v "${MAIN_BRANCH}" | sed 's/.*origin\///' | while read -r branch; do
    if [ -z "${branch}" ]; then
        continue
    fi

    ahead="$(git rev-list --count "origin/${MAIN_BRANCH}..origin/${branch}" 2>/dev/null || echo "0")"
    if [ "${ahead}" -le 0 ]; then
        continue
    fi

    log "Branch ${branch} is ${ahead} commits ahead of ${MAIN_BRANCH}"
done

if [ -n "${CURRENT_BRANCH}" ] && [ "${CURRENT_BRANCH}" != "$(git branch --show-current)" ]; then
    git checkout "${CURRENT_BRANCH}" >/dev/null 2>&1 || true
fi

log "Monitor check complete"
