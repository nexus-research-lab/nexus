// Package contract 定义 nexus_automation MCP 子包之间共享的契约：
// Service 接口、ServerContext 上下文、ServerName 常量。
// 放在独立叶子包里避免 tool / internal 子包反向依赖 mcp 顶层。
package contract

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ServerName 是 MCP server 的注册名。
const ServerName = "nexus_automation"

// ServerContext 承载当前会话与智能体的运行时上下文。
type ServerContext struct {
	CurrentAgentID      string
	CurrentAgentName    string
	CurrentSessionKey   string
	CurrentSessionLabel string
	// SourceContextType 取值 "agent" 或 "chat"，影响 reply_mode=execution 的解析。
	SourceContextType string
	// IsMainAgent 标识当前调用方是否为主智能体。主智能体豁免 agent_id scope 限制，
	// 可以查看/管理任意智能体的定时任务；普通 Agent 只能 CRUD 自己的任务。
	IsMainAgent bool
	// DefaultTimezone 是用户未显式指定 schedule.timezone 时使用的回退时区（IANA）。
	DefaultTimezone string
}

// Service 是 MCP server 依赖的 automation 服务子集。
type Service interface {
	ListTasks(ctx context.Context, agentID string) ([]protocol.CronJob, error)
	GetTask(ctx context.Context, jobID string) (*protocol.CronJob, error)
	CreateTask(ctx context.Context, input protocol.CreateJobInput) (*protocol.CronJob, error)
	UpdateTask(ctx context.Context, jobID string, input protocol.UpdateJobInput) (*protocol.CronJob, error)
	UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*protocol.CronJob, error)
	DeleteTask(ctx context.Context, jobID string) error
	RunTaskNow(ctx context.Context, jobID string) (*protocol.ExecutionResult, error)
	ListTaskRuns(ctx context.Context, jobID string) ([]protocol.CronRun, error)
}
