#!/bin/bash
# GitHub PR & Branch Monitor for nexus-core
# Runs every 15 minutes via OpenClaw cron
# Uses Codex for intelligent code review

set -e

# Load Azure API key directly
export AZURE_OPENAI_API_KEY="8fSNQnGZFlGsPTUcbaky42yqpqa39VpC1hlsiukPaJzEGuCO8ZJQQJ99BGACHYHv6XJ3w3AAAAACOGDz5F"

REPO_DIR="/Users/aibox/.openclaw/workspace/PROJECTS/nexus-core"
CONFIG="$REPO_DIR/.github-monitor/config.json"
LOG="$REPO_DIR/.github-monitor/monitor.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG"
    echo "$1"
}

cd "$REPO_DIR"

# Update repository
log "Fetching latest changes..."
git fetch --all --prune 2>&1 | grep -v "From https" || true

# Store current branch
CURRENT_BRANCH=$(git branch --show-current)

# Check for new PRs
log "Checking for open PRs..."
PRS=$(gh pr list --repo nexus-research-lab/nexus-core --state open --json number,title,headRefName,baseRefName,mergeable,mergeStateStatus 2>&1)

if [ "$PRS" != "[]" ] && [ -n "$PRS" ]; then
    log "Found open PRs:"
    echo "$PRS" | jq -r '.[] | "  PR #\(.number): \(.title) (\(.headRefName) -> \(.baseRefName))"'

    # Process each PR
    echo "$PRS" | jq -c '.[]' | while read -r pr; do
        PR_NUM=$(echo "$pr" | jq -r '.number')
        PR_TITLE=$(echo "$pr" | jq -r '.title')
        PR_BRANCH=$(echo "$pr" | jq -r '.headRefName')
        MERGEABLE=$(echo "$pr" | jq -r '.mergeable')
        MERGE_STATE=$(echo "$pr" | jq -r '.mergeStateStatus')

        log "Processing PR #$PR_NUM: $PR_TITLE"

        # Check if mergeable
        if [ "$MERGEABLE" != "MERGEABLE" ]; then
            log "  ⚠️  PR not mergeable (state: $MERGE_STATE)"
            continue
        fi

        # Checkout PR branch
        log "  Checking out PR branch..."
        git stash 2>&1 | grep -v "Saved working directory" || true
        git checkout "$PR_BRANCH" 2>&1 | grep -v "Switched to" || true
        git pull origin "$PR_BRANCH" 2>&1 | grep -v "From https" || true

        # Use Codex to review the branch
        log "  🔍 Reviewing with Codex..."
        REVIEW=$(cd "$REPO_DIR" && codex review --base main 2>&1 || echo "REVIEW_FAILED")

        log "  Codex review output: ${REVIEW:0:300}..."

        # Check if approved
        if echo "$REVIEW" | grep -qiE "(approve|approved|✅|looks good|no issues|lgtm|ready to merge)"; then
            log "  ✅ Approved by Codex, merging..."
            gh pr merge "$PR_NUM" --repo nexus-research-lab/nexus-core --squash --delete-branch 2>&1 || log "  ⚠️  Merge failed"
            log "  ✅ PR #$PR_NUM merged successfully"
        else
            log "  ❌ Rejected by Codex"

            # Add a comment on the PR
            gh pr comment "$PR_NUM" --repo nexus-research-lab/nexus-core --body "🤖 **Codex Code Review: REJECTED**

This automated code review found potential issues that need to be addressed.

**Review Summary:**
${REVIEW:0:500}

Please review and address any concerns before merging." 2>&1 || true
        fi

        # Return to main
        git checkout main 2>&1 | grep -v "Switched to" || true
        git stash pop 2>&1 | grep -v "Dropped" || true
    done
else
    log "No open PRs found"
fi

# Check branch sync
log "Checking branch status..."
git branch -r | grep -v HEAD | grep -v main | sed 's/.*origin\///' | while read -r BRANCH; do
    if [ -z "$BRANCH" ]; then
        continue
    fi
    # Check if branch has new commits
    AHEAD=$(cd "$REPO_DIR" && git rev-list --count origin/main..origin/"$BRANCH" 2>/dev/null || echo "0")

    if [ "$AHEAD" -gt 0 ]; then
        log "  Branch $BRANCH is $AHEAD commits ahead of main"

        # Checkout the branch
        log "  Checking out $BRANCH..."
        git stash 2>&1 | grep -v "Saved working directory" || true
        git checkout "$BRANCH" 2>&1 | grep -v "Switched to" || true
        git pull origin "$BRANCH" 2>&1 | grep -v "From https" || true

        # Use Codex to review
        log "  🔍 Reviewing with Codex..."
        REVIEW=$(cd "$REPO_DIR" && codex review --base main 2>&1 || echo "REVIEW_FAILED")

        log "  Codex review output: ${REVIEW:0:300}..."

        # Check if approved
        if echo "$REVIEW" | grep -qiE "(approve|approved|✅|looks good|no issues|lgtm|ready to merge)"; then
            log "  ✅ Approved by Codex, merging to main..."

            # Switch to main and merge
            git checkout main 2>&1 | grep -v "Switched to" || true
            git pull origin main 2>&1 | grep -v "From https" || true

            # Merge with squash
            git merge --squash "$BRANCH" 2>&1 || log "  ⚠️  Merge failed"
            git commit -m "Merge $BRANCH into main (auto-approved by Codex)" 2>&1 || log "  ⚠️  Commit failed"
            git push origin main 2>&1 || log "  ⚠️  Push failed"

            # Delete the branch
            git branch -D "$BRANCH" 2>&1 || true
            git push origin --delete "$BRANCH" 2>&1 || true

            log "  ✅ Branch $BRANCH merged and deleted"
        else
            log "  ❌ Rejected by Codex, skipping merge"

            # Return to main
            git checkout main 2>&1 | grep -v "Switched to" || true
        fi

        git stash pop 2>&1 | grep -v "Dropped" || true
    fi
done

# Return to original branch
if [ -n "$CURRENT_BRANCH" ] && [ "$CURRENT_BRANCH" != "$(git branch --show-current)" ]; then
    git checkout "$CURRENT_BRANCH" 2>&1 | grep -v "Switched to" || true
fi
log "Monitor check complete ✅"
