// Package contract 定义 nexus_automation MCP 子包之间共享的契约：
// Service 接口、ServerContext 上下文、ServerName 常量。
// 放在独立叶子包里避免 tool / internal 子包反向依赖 mcp 顶层。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package contract

import (
	"context"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// ServerName 是 MCP server 的注册名。
const ServerName = "nexus_automation"

// ServerContext 承载当前会话与智能体的运行时上下文。
type ServerContext struct {
	CurrentAgentID      string
	CurrentAgentName    string
	OwnerUserID         string
	CurrentSessionKey   string
	CurrentSessionLabel string
	// SourceContextType 的 "agent"/"room" 与经过实时 pairing 复核的
	// "agent_paired" 表示可信交互上下文；其他带 channel/external/
	// automation/queue/internal 等后缀的来源只能获得只读执行权限。
	// 该字段也影响 reply_mode=execution 的解析。
	SourceContextType string
	// SourceContextID/Label 对齐前端任务来源快照，用于让 Agent 创建的 Room 任务
	// 后续仍能在任务管理 UI 里按 Room 维度编辑。
	SourceContextID    string
	SourceContextLabel string
	// StableInteractiveSurface 只决定当前 Session 的工具表；真实
	// mutation 权限仍只由 SourceContextType 在每次调用时判定。
	StableInteractiveSurface bool
	// IsMainAgent 只由应用层为主智能体自己的可信私有 DM 签发。该上下文可在
	// owner scope 内跨 Agent 管理；Room、外部和后台来源即使运行主智能体也必须为 false。
	IsMainAgent bool
	// DefaultTimezone 是用户未显式指定 schedule.timezone 时使用的回退时区（IANA）。
	DefaultTimezone string
	// CurrentJobID/CurrentRunID 仅由 runtime Execution MCP overlay 从服务端
	// AutomationRunContext 签发。后台任务的只读工具严格收窄到该任务。
	CurrentJobID string
	CurrentRunID string
}

// Service 是 MCP server 依赖的 automation 服务子集。
type Service interface {
	ListTasks(ctx context.Context, agentID string) ([]automationdomain.ScheduledTask, error)
	GetTask(ctx context.Context, jobID string) (*automationdomain.ScheduledTask, error)
	CreateTask(ctx context.Context, input automationdomain.CreateJobInput) (*automationdomain.ScheduledTask, error)
	UpdateTask(ctx context.Context, jobID string, input automationdomain.UpdateJobInput) (*automationdomain.ScheduledTask, error)
	UpdateTaskAtVersion(ctx context.Context, jobID string, expectedVersion int64, input automationdomain.UpdateJobInput) (*automationdomain.ScheduledTask, error)
	DeleteTask(ctx context.Context, jobID string) (*automationdomain.DeleteJobResult, error)
	DeleteTaskAtVersion(ctx context.Context, jobID string, expectedVersion int64) (*automationdomain.DeleteJobResult, error)
	RunTaskNow(ctx context.Context, jobID string) (*automationdomain.ExecutionResult, error)
	ListTaskRuns(ctx context.Context, jobID string) ([]automationdomain.ScheduledTaskRun, error)
	ListTaskEvents(ctx context.Context, jobID string, limit int) ([]automationdomain.ScheduledTaskEvent, error)
	SearchTaskHistory(ctx context.Context, input automationdomain.ScheduledTaskHistorySearchInput) ([]automationdomain.ScheduledTaskHistoryItem, error)
	GetTaskStatus(ctx context.Context, jobID string, runLimit int, eventLimit int) (*automationdomain.ScheduledTaskStatus, error)
	GetDailyReport(ctx context.Context, input automationdomain.ScheduledTaskDailyReportInput) (*automationdomain.ScheduledTaskDailyReport, error)
	RetryRunDelivery(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error)
	RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*automationdomain.ScheduledTask, error)
	GetHeartbeatStatus(ctx context.Context, agentID string) (*automationdomain.HeartbeatStatus, error)
	UpdateHeartbeatAtVersion(ctx context.Context, agentID string, expectedVersion int64, input automationdomain.HeartbeatUpdateInput) (*automationdomain.HeartbeatStatus, error)
	WakeHeartbeat(ctx context.Context, agentID string, input automationdomain.HeartbeatWakeInput) (*automationdomain.HeartbeatWakeResult, error)
}
