// INPUT: 当前 DM/Room round 的完整 actor、scope、permission、Automation、统一 Responsibility authority 与 runtime identity。
// OUTPUT: 保留 Automation run、producer/reviewer trusted capability，并让 Execution MCP 动态读取同一份 Goal/Execution/Work/Review identity。
// POS: 保持通用 MCPServerBuilder 稳定，同时给编排工具提供不丢失语义的 trusted round fence。
package runtime

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// ExecutionToolContext 是 Execution 工具每轮重新读取权威 snapshot 所需的身份边界。
// 它不携带 snapshot 或 expected version，避免同一轮连续 mutation 读取缓存状态。
type ExecutionToolContext struct {
	Agent                   *protocol.Agent
	ScopeSessionKey         string
	RuntimeSessionKey       string
	ExecutionID             string
	WorkBinding             *protocol.ExecutionWorkBinding
	WorkBindingState        *WorkBindingState
	ReviewBinding           *protocol.ExecutionReviewBinding
	CoordinatorAgentID      string
	RootRoundID             string
	AgentRoundID            string
	SourceContextType       string
	SourceContextID         string
	RoomID                  string
	ConversationID          string
	PermissionMode          sdkpermission.Mode
	GoalAuthority           *GoalAuthorityState
	ResponsibilityAuthority *ResponsibilityAuthorityState
	AutomationRun           *protocol.AutomationRunContext
}

// ExecutionMCPServerBuilder 构造只服务当前 round identity 的 Execution MCP overlay。
type ExecutionMCPServerBuilder func(
	context.Context,
	ExecutionToolContext,
) map[string]sdkmcp.ServerConfig
