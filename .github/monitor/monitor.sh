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

cleanup_temp_dir() {
    if [ -n "${TEMP_REVIEW_DIR:-}" ] && [ -d "${TEMP_REVIEW_DIR}" ]; then
        rm -rf "${TEMP_REVIEW_DIR}"
    fi
}

restore_original_branch() {
    if [ -n "${CURRENT_BRANCH:-}" ] && [ "$(git branch --show-current 2>/dev/null || true)" != "${CURRENT_BRANCH}" ]; then
        git checkout "${CURRENT_BRANCH}" >/dev/null 2>&1 || true
    fi
}

finalize() {
    cleanup_temp_dir
    restore_original_branch
}

trap finalize EXIT

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
MERGE_METHOD="$(jq -r '.autoMerge.mergeMethod // "squash"' "${CONFIG}")"
AUTO_MERGE_ENABLED="$(jq -r '.autoMerge.enabled // true' "${CONFIG}")"
CODEX_ENABLED="$(jq -r '.codex.enabled // true' "${CONFIG}")"
REVIEW_PROMPT_RAW="$(jq -r '.codex.reviewPrompt // empty' "${CONFIG}")"

if [ -z "${REVIEW_PROMPT_RAW}" ] || [ "${REVIEW_PROMPT_RAW}" = "null" ]; then
    REVIEW_PROMPT_RAW="Review this code change for bugs, security issues, code quality problems, regressions, and breaking changes. Reply with exactly one first line in the form VERDICT: APPROVE, VERDICT: REJECT, or VERDICT: NEEDS_HUMAN_REVIEW. Then include a short rationale and any concrete blocking issues."
fi

REVIEW_PROMPT="${REVIEW_PROMPT_RAW}

Output contract:
- First line must be exactly one of:
  - VERDICT: APPROVE
  - VERDICT: REJECT
  - VERDICT: NEEDS_HUMAN_REVIEW
- Keep the rest concise and concrete.
- If rejecting, include at least one specific blocking issue.
"

COMMENT_TAG="<!-- github-monitor:codex-review -->"

cd "${REPO_DIR}"

CURRENT_BRANCH="$(git branch --show-current)"

log "Fetching latest changes..."
git fetch --all --prune 2>&1 | grep -v "From https" || true

get_pr_state_json() {
    local pr_num="$1"
    gh pr view "$pr_num" --repo "${REPO}" --json number,state,mergedAt,closedAt,mergeable,mergeStateStatus,isDraft,headRefName,baseRefName,title,url
}

is_pr_open_and_mergeable() {
    local pr_json="$1"
    local state mergeable is_draft base_ref

    state="$(echo "${pr_json}" | jq -r '.state')"
    mergeable="$(echo "${pr_json}" | jq -r '.mergeable')"
    is_draft="$(echo "${pr_json}" | jq -r '.isDraft')"
    base_ref="$(echo "${pr_json}" | jq -r '.baseRefName')"

    [ "${state}" = "OPEN" ] && [ "${is_draft}" = "false" ] && [ "${base_ref}" = "${MAIN_BRANCH}" ] && [ "${mergeable}" = "MERGEABLE" ]
}

run_codex_review() {
    local pr_num="$1"
    local pr_branch="$2"

    TEMP_REVIEW_DIR="$(mktemp -d)"
    git clone --quiet "${REPO_DIR}" "${TEMP_REVIEW_DIR}/repo" >/dev/null 2>&1

    (
        cd "${TEMP_REVIEW_DIR}/repo"
        git fetch --all --prune >/dev/null 2>&1
        git checkout -q "origin/${MAIN_BRANCH}"
        git checkout -q -B "review/${pr_num}" "origin/${pr_branch}"
        printf '%s
' "${REVIEW_PROMPT}" | codex review --base "origin/${MAIN_BRANCH}" 2>&1
    )
}

extract_verdict() {
    local review_text="$1"
    local verdict

    verdict="$(printf '%s
' "${review_text}" | sed -n 's/^VERDICT:[[:space:]]*//p' | head -n1 | tr '[:lower:]' '[:upper:]' | tr -d '\r')"

    case "${verdict}" in
        APPROVE|REJECT|NEEDS_HUMAN_REVIEW)
            printf '%s
' "${verdict}"
            ;;
        *)
            printf '%s
' "NEEDS_HUMAN_REVIEW"
            ;;
    esac
}

truncate_review_body() {
    local review_text="$1"
    printf '%s
' "${review_text}" | awk 'NR<=40 { print } NR==41 { print "..." }'
}

comment_already_posted() {
    local pr_num="$1"
    local body_hash="$2"

    gh pr view "$pr_num" --repo "${REPO}" --json comments \
      --jq ".comments[].body" 2>/dev/null | grep -Fq "${COMMENT_TAG} hash:${body_hash}" || return 1
}

post_review_comment_if_needed() {
    local pr_num="$1"
    local verdict="$2"
    local review_text="$3"
    local summarized hash body

    summarized="$(truncate_review_body "${review_text}")"
    hash="$(printf '%s' "${verdict}
${summarized}" | shasum -a 256 | awk '{print $1}')"

    if comment_already_posted "${pr_num}" "${hash}"; then
        log "PR #${pr_num} already has identical monitor comment, skipping"
        return 0
    fi

    body="${COMMENT_TAG} hash:${hash}
🤖 自动巡检结论：**${verdict}**

\\`\\`\\`
${summarized}
\\`\\`\\`
"

    gh pr comment "$pr_num" --repo "${REPO}" --body "${body}" >/dev/null
}

merge_pr() {
    local pr_num="$1"
    case "${MERGE_METHOD}" in
        merge)
            gh pr merge "$pr_num" --repo "${REPO}" --merge --delete-branch
            ;;
        rebase)
            gh pr merge "$pr_num" --repo "${REPO}" --rebase --delete-branch
            ;;
        squash|*)
            gh pr merge "$pr_num" --repo "${REPO}" --squash --delete-branch
            ;;
    esac
}

log "Checking for open PRs..."
PRS="$(gh pr list --repo "${REPO}" --state open --json number,title,headRefName,baseRefName,mergeable,mergeStateStatus,isDraft 2>&1)"

if [ "${PRS}" != "[]" ] && [ -n "${PRS}" ]; then
    log "Found open PRs"

    echo "${PRS}" | jq -c '.[]' | while read -r pr; do
        PR_NUM="$(echo "${pr}" | jq -r '.number')"
        PR_TITLE="$(echo "${pr}" | jq -r '.title')"
        PR_BRANCH="$(echo "${pr}" | jq -r '.headRefName')"

        log "Processing PR #${PR_NUM}: ${PR_TITLE}"

        PR_STATE_JSON="$(get_pr_state_json "${PR_NUM}")"
        if ! is_pr_open_and_mergeable "${PR_STATE_JSON}"; then
            log "PR #${PR_NUM} skipped (state=$(echo "${PR_STATE_JSON}" | jq -r '.state'), mergeable=$(echo "${PR_STATE_JSON}" | jq -r '.mergeable'), draft=$(echo "${PR_STATE_JSON}" | jq -r '.isDraft'))"
            continue
        fi

        if [ "${CODEX_ENABLED}" != "true" ]; then
            log "Codex review disabled; skipping PR #${PR_NUM}"
            continue
        fi

        log "Reviewing PR #${PR_NUM} with Codex..."
        REVIEW_OUTPUT="$(run_codex_review "${PR_NUM}" "${PR_BRANCH}" || true)"
        VERDICT="$(extract_verdict "${REVIEW_OUTPUT}")"
        cleanup_temp_dir
        TEMP_REVIEW_DIR=""

        log "PR #${PR_NUM} review verdict: ${VERDICT}"

        # Review can take time; refresh state before taking action.
        PR_STATE_JSON="$(get_pr_state_json "${PR_NUM}")"
        if ! is_pr_open_and_mergeable "${PR_STATE_JSON}"; then
            log "PR #${PR_NUM} changed state during review; skipping action (state=$(echo "${PR_STATE_JSON}" | jq -r '.state'), mergedAt=$(echo "${PR_STATE_JSON}" | jq -r '.mergedAt'))"
            continue
        fi

        case "${VERDICT}" in
            APPROVE)
                if [ "${AUTO_MERGE_ENABLED}" = "true" ]; then
                    log "PR #${PR_NUM} approved, merging..."
                    if merge_pr "${PR_NUM}" >/dev/null 2>&1; then
                        log "PR #${PR_NUM} merged successfully"
                    else
                        log "Merge PR #${PR_NUM} failed; leaving for manual follow-up"
                    fi
                else
                    log "PR #${PR_NUM} approved, but auto-merge is disabled"
                fi
                ;;
            REJECT)
                log "PR #${PR_NUM} rejected with concrete issues"
                post_review_comment_if_needed "${PR_NUM}" "${VERDICT}" "${REVIEW_OUTPUT}" || true
                ;;
            NEEDS_HUMAN_REVIEW|*)
                log "PR #${PR_NUM} requires human review; no auto-comment posted"
                ;;
        esac
    done
else
    log "No open PRs found"
fi

log "Checking branch status..."
OPEN_PRS_JSON="$(gh pr list --repo "${REPO}" --state open --json number,headRefName,title 2>/dev/null || echo '[]')"

git for-each-ref --format='%(refname:short)' refs/remotes/origin | sed 's#^origin/##' | while read -r branch; do
    if [ -z "${branch}" ] || [ "${branch}" = "HEAD" ] || [ "${branch}" = "${MAIN_BRANCH}" ]; then
        continue
    fi

    ahead="$(git rev-list --count "origin/${MAIN_BRANCH}..origin/${branch}" 2>/dev/null || echo "0")"
    if [ "${ahead}" -le 0 ]; then
        continue
    fi

    matching_pr="$(echo "${OPEN_PRS_JSON}" | jq -r --arg branch "${branch}" '.[] | select(.headRefName == $branch) | "#\(.number) \(.title)"' | head -n1)"
    if [ -n "${matching_pr}" ]; then
        log "Branch ${branch} is ${ahead} commits ahead of ${MAIN_BRANCH} (open PR ${matching_pr})"
    else
        log "Branch ${branch} is ${ahead} commits ahead of ${MAIN_BRANCH} (no open PR; branch may be stale or already squash-merged)"
    fi
done

log "Monitor check complete"
