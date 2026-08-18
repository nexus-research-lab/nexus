// INPUT: 当前 DM/Room round 的完整 actor、scope、permission、Automation、统一 Responsibility authority 与 runtime identity。
// OUTPUT: Goal/Execution/Automation command 共用的 producer/reviewer trusted capability 与动态责任身份。
// POS: round-scoped nexus command broker 的可信上下文；不接受模型输入覆盖。
package runtime

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// RuntimeCommandContext 是领域 command 每轮重新读取权威 snapshot 所需的身份边界。
// 它不携带 snapshot 或 expected version，避免同一轮连续 mutation 读取缓存状态。
type RuntimeCommandContext struct {
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
	SourceContextLabel      string
	RoomID                  string
	ConversationID          string
	PermissionMode          sdkpermission.Mode
	GoalAuthority           *GoalAuthorityState
	ResponsibilityAuthority *ResponsibilityAuthorityState
	AutomationRun           *protocol.AutomationRunContext
}
