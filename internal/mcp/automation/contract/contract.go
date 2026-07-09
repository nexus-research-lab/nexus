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

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
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
	// SourceContextType 取值 "agent" 或 "room"，影响 reply_mode=execution 的解析。
	SourceContextType string
	// SourceContextID/Label 对齐前端任务来源快照，用于让 Agent 创建的 Room 任务
	// 后续仍能在任务管理 UI 里按 Room 维度编辑。
	SourceContextID    string
	SourceContextLabel string
	// IsMainAgent 标识当前调用方是否为主智能体。主智能体豁免 agent_id scope 限制，
	// 可以查看/管理任意智能体的定时任务；普通 Agent 只能 CRUD 自己的任务。
	IsMainAgent bool
	// DefaultTimezone 是用户未显式指定 schedule.timezone 时使用的回退时区（IANA）。
	DefaultTimezone string
}

// Service 是 MCP server 依赖的 automation 服务子集。
type Service interface {
	ListTasks(ctx context.Context, agentID string) ([]automationdomain.CronJob, error)
	GetTask(ctx context.Context, jobID string) (*automationdomain.CronJob, error)
	CreateTask(ctx context.Context, input automationdomain.CreateJobInput) (*automationdomain.CronJob, error)
	UpdateTask(ctx context.Context, jobID string, input automationdomain.UpdateJobInput) (*automationdomain.CronJob, error)
	UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*automationdomain.CronJob, error)
	DeleteTask(ctx context.Context, jobID string) (*automationdomain.DeleteJobResult, error)
	RunTaskNow(ctx context.Context, jobID string) (*automationdomain.ExecutionResult, error)
	ListTaskRuns(ctx context.Context, jobID string) ([]automationdomain.CronRun, error)
	ListTaskEvents(ctx context.Context, jobID string, limit int) ([]automationdomain.CronTaskEvent, error)
	SearchTaskHistory(ctx context.Context, input automationdomain.CronTaskHistorySearchInput) ([]automationdomain.CronTaskHistoryItem, error)
	GetTaskStatus(ctx context.Context, jobID string, runLimit int, eventLimit int) (*automationdomain.CronTaskStatus, error)
	GetDailyReport(ctx context.Context, input automationdomain.CronDailyReportInput) (*automationdomain.CronDailyReport, error)
	RetryRunDelivery(ctx context.Context, jobID string, runID string) (*automationdomain.CronRun, error)
	RecoverTaskRunningRun(ctx context.Context, jobID string, runID string) (*automationdomain.CronJob, error)
}
