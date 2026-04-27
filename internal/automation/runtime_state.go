package automation

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// JobRuntimeState 是进程内的自动化任务运行态。
type JobRuntimeState struct {
	Job           protocol.CronJob
	Running       bool
	NextRunAt     *time.Time
	LastRunAt     *time.Time
	FailureStreak int
}

// HeartbeatRuntimeState 是进程内的 heartbeat 运行态。
type HeartbeatRuntimeState struct {
	Config          protocol.HeartbeatConfig
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
