#!/bin/bash
# GitHub Monitor Status

echo "GitHub Monitor Status"
echo "===================="
echo ""

# Check OpenClaw cron status
echo "📅 OpenClaw Cron Jobs:"
openclaw cron list 2>&1 | grep -A 1 "github-monitor" || echo "  No cron job found"

echo ""

# Check last run time
if [ -f "/Users/aibox/.openclaw/workspace/PROJECTS/nexus-core/.github-monitor/monitor.log" ]; then
    LAST_RUN=$(tail -1 /Users/aibox/.openclaw/workspace/PROJECTS/nexus-core/.github-monitor/monitor.log 2>/dev/null | grep -o '\[.*\]' | head -1)
    echo "Last check: $LAST_RUN"
else
    echo "⚠️  No monitor log found"
fi

echo ""
echo "📋 Configuration:"
echo "  - Repo: nexus-research-lab/nexus-core"
echo "  - Schedule: Every 15 minutes"
echo "  - Method: OpenClaw Cron"
echo ""
echo "📁 Logs:"
echo "  - Monitor: /Users/aibox/.openclaw/workspace/PROJECTS/nexus-core/.github-monitor/monitor.log"
echo ""
echo "💡 Management:"
echo "  - Status:    bash $0"
echo "  - Run now:   openclaw cron run a368c987-4aa8-4a28-a818-9ca8d5b334f8"
echo "  - View logs: tail -f /Users/aibox/.openclaw/workspace/PROJECTS/nexus-core/.github-monitor/monitor.log"
echo "  - List jobs: openclaw cron list"
echo "  - Disable:   openclaw cron disable a368c987-4aa8-4a28-a818-9ca8d5b334f8"
echo "  - Enable:    openclaw cron enable a368c987-4aa8-4a28-a818-9ca8d5b334f8"
