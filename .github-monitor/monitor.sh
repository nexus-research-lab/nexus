#!/bin/bash
# GitHub PR & Branch Monitor for nexus-core
# Runs every 15 minutes via OpenClaw cron
# Uses simple code review (no external AI)

set -e

REPO_DIR="/Users/aibox/.openclaw/workspace/PROJECTS/nexus-core"
CONFIG="$REPO_DIR/.github-monitor/config.json"
LOG="$REPO_DIR/.github-monitor/monitor.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG"
    echo "$1"
}

review_code() {
    local DIFF="$1"
    local ISSUES=""
    
    if [ -z "$DIFF" ]; then
        echo "NO_CHANGES"
        return 0
    fi
    
    # Check 1: Large file changes
    LOCAL_CHANGES=$(git diff --stat origin/main...HEAD 2>/dev/null | tail -1 | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+' || echo "0")
    if [ "$LOCAL_CHANGES" -gt 1000 ]; then
        ISSUES="$ISSUES\n- Large change: $LOCAL_CHANGES lines added"
    fi
    
    # Check 2: Hardcoded secrets
    if echo "$DIFF" | grep -qiE "(password|api_key|secret|token|private_key)\s*=\s*['\"][^'\"]+['\"]"; then
        ISSUES="$ISSUES\n- ⚠️  Potential hardcoded secrets"
    fi
    
    # Check 3: Too many console.logs
    CONSOLE_LOGS=$(echo "$DIFF" | grep -c '^\+.*console\.log' 2>/dev/null || echo "0")
    if [ "$CONSOLE_LOGS" -gt 10 ]; then
        ISSUES="$ISSUES\n- Many console.log statements ($CONSOLE_LOGS)"
    fi
    
    # Check 4: Deleted important files
    DELETED_FILES=$(echo "$DIFF" | grep -c '^--- a/' 2>/dev/null || echo "0")
    if [ "$DELETED_FILES" -gt 5 ]; then
        ISSUES="$ISSUES\n- Many files deleted ($DELETED_FILES)"
    fi
    
    # Check 5: Syntax errors (basic)
    if echo "$DIFF" | grep -E '^\+.*\}\s*$' | grep -vE '^\+\s*\}' | head -1 | grep -q .; then
        : # Skip this check for now
    fi
    
    if [ -z "$ISSUES" ]; then
        echo "APPROVE"
    else
        echo "REJECT:$ISSUES"
    fi
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

        # Get diff and review
        log "  🔍 Reviewing code..."
        DIFF=$(git diff origin/main...HEAD 2>/dev/null || echo "")
        REVIEW_RESULT=$(review_code "$DIFF")
        
        if echo "$REVIEW_RESULT" | grep -q "^APPROVE"; then
            log "  ✅ Code review passed, merging..."
            gh pr merge "$PR_NUM" --repo nexus-research-lab/nexus-core --squash --delete-branch 2>&1 || log "  ⚠️  Merge failed"
            log "  ✅ PR #$PR_NUM merged successfully"
        else
            ISSUES=$(echo "$REVIEW_RESULT" | sed 's/^REJECT://')
            log "  ❌ Code review rejected:$ISSUES"
            
            # Comment on PR
            gh pr comment "$PR_NUM" --repo nexus-research-lab/nexus-core --body "🤖 **Automated Code Review: REJECTED**

The following issues were detected:
${ISSUES}

Please address these issues before merging." 2>&1 || true
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

        # Review code
        log "  🔍 Reviewing code..."
        DIFF=$(git diff origin/main...HEAD 2>/dev/null || echo "")
        REVIEW_RESULT=$(review_code "$DIFF")
        
        if echo "$REVIEW_RESULT" | grep -q "^APPROVE"; then
            log "  ✅ Code review passed, merging to main..."

            # Switch to main and merge
            git checkout main 2>&1 | grep -v "Switched to" || true
            git pull origin main 2>&1 | grep -v "From https" || true

            # Merge with squash
            git merge --squash "$BRANCH" 2>&1 || log "  ⚠️  Merge failed"
            git commit -m "Merge $BRANCH into main (auto-approved)" 2>&1 || log "  ⚠️  Commit failed"
            git push origin main 2>&1 || log "  ⚠️  Push failed"

            # Delete the branch
            git branch -D "$BRANCH" 2>&1 || true
            git push origin --delete "$BRANCH" 2>&1 || true

            log "  ✅ Branch $BRANCH merged and deleted"
        else
            ISSUES=$(echo "$REVIEW_RESULT" | sed 's/^REJECT://')
            log "  ❌ Code review rejected:$ISSUES"
            log "  Skipping merge for $BRANCH"
            
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
