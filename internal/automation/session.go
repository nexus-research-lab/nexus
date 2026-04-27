package automation

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ResolveSessionKey 解析自动化任务的真实执行会话。
func ResolveSessionKey(job protocol.CronJob, runID *string) (string, error) {
	switch strings.TrimSpace(job.SessionTarget.Kind) {
	case protocol.SessionTargetMain:
		return BuildMainSessionKey(job.AgentID), nil
	case protocol.SessionTargetBound:
		return strings.TrimSpace(job.SessionTarget.BoundSessionKey), nil
	case protocol.SessionTargetNamed:
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", strings.TrimSpace(job.SessionTarget.NamedSessionKey), ""), nil
	default:
		if runID == nil || strings.TrimSpace(*runID) == "" {
			return "", errors.New("isolated target requires run_id")
		}
		ref := fmt.Sprintf("cron:%s:%s", job.JobID, strings.TrimSpace(*runID))
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", ref, ""), nil
	}
}

// BuildMainSessionKey 构建 agent 的自动化主会话 key。
func BuildMainSessionKey(agentID string) string {
	return protocol.BuildAgentSessionKey(strings.TrimSpace(agentID), "automation", "dm", "main", "")
}

// NewID 创建自动化内部 id。
func NewID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}
