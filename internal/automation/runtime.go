package automation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type actorAgentContextKey struct{}

// WithActorAgentID 标记本次自动化管理动作由哪个 Agent 发起。
func WithActorAgentID(ctx context.Context, agentID string) context.Context {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ctx
	}
	return context.WithValue(ctx, actorAgentContextKey{}, agentID)
}

// ActorAgentID 从上下文读取本次自动化管理动作的发起 Agent。
func ActorAgentID(ctx context.Context) (string, bool) {
	agentID, _ := ctx.Value(actorAgentContextKey{}).(string)
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", false
	}
	return agentID, true
}

// JobRuntimeState 是进程内的自动化任务运行态。
type JobRuntimeState struct {
	Job                types.ScheduledTask
	Running            bool
	RunningCount       int
	NextRunAt          *time.Time
	RunningRunID       string
	RunningStartedAt   *time.Time
	LastRunAt          *time.Time
	LastRunStatus      string
	FailureStreak      int
	LastError          *string
	LastDeliveryStatus string
}

// HeartbeatRuntimeState 是进程内的 heartbeat 运行态。
type HeartbeatRuntimeState struct {
	Config          types.HeartbeatConfig
	Running         bool
	PendingWake     bool
	NextRunAt       *time.Time
	LastHeartbeatAt *time.Time
	LastAckAt       *time.Time
	DeliveryError   *string
}

// HeartbeatWakeRequest 表示待合并进 heartbeat 指令的一次唤醒请求。
type HeartbeatWakeRequest struct {
	AgentID    string
	SessionKey string
	WakeMode   string
	Text       string
}

var retryBackoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

// RetryBackoffFor 返回连续失败后的短重试退避。
func RetryBackoffFor(streak int) (time.Duration, bool) {
	if streak <= 0 || streak > len(retryBackoffs) {
		return 0, false
	}
	return retryBackoffs[streak-1], true
}

// ResolveSessionKey 解析自动化任务的真实执行会话。
func ResolveSessionKey(job types.ScheduledTask, runID *string) (string, error) {
	switch strings.TrimSpace(job.SessionTarget.Kind) {
	case types.SessionTargetMain:
		return BuildMainSessionKey(job.AgentID), nil
	case types.SessionTargetBound:
		return strings.TrimSpace(job.SessionTarget.BoundSessionKey), nil
	case types.SessionTargetNamed:
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", strings.TrimSpace(job.SessionTarget.NamedSessionKey), ""), nil
	default:
		if runID == nil || strings.TrimSpace(*runID) == "" {
			return "", errors.New("isolated target requires run_id")
		}
		ref := fmt.Sprintf("scheduled-task:%s:%s", job.JobID, strings.TrimSpace(*runID))
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
