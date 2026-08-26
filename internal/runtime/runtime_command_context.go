// INPUT: 当前 DM/Room round 的完整 actor、scope、permission、Automation、WorkGraph 保存绑定、统一 Responsibility authority 与 runtime identity。
// OUTPUT: Goal/Execution/Automation command 共用的 producer/reviewer trusted authority、exact preview 保存绑定、动态责任身份与调用时 SDK Session identity。
// POS: round-scoped nexus_runtime server 的可信上下文；不接受模型输入覆盖。
package runtime

import (
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// SDKSessionIdentityState 保存当前物理 round 已由宿主确认的 provider Session
// identity。command server 可能早于 provider init/fork 事件创建，因此调用时必须读取
// 这份动态状态，不能把启动阶段的空值或旧值固化进 round actor。
type SDKSessionIdentityState struct {
	mu        sync.RWMutex
	sessionID string
}

// NewSDKSessionIdentityState 创建可被同一 round 后续 init/fork 事件更新的身份状态。
func NewSDKSessionIdentityState(sessionID string) *SDKSessionIdentityState {
	return &SDKSessionIdentityState{sessionID: strings.TrimSpace(sessionID)}
}

// Set 原子替换当前 provider Session identity；空值用于明确撤销失效身份。
func (s *SDKSessionIdentityState) Set(sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sessionID = strings.TrimSpace(sessionID)
	s.mu.Unlock()
}

// Load 返回当前 provider Session identity。
func (s *SDKSessionIdentityState) Load() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

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
	SDKSessionIdentity      *SDKSessionIdentityState
	AutomationRun           *protocol.AutomationRunContext
	WorkGraphPreviewID      string
}

// CurrentSDKSessionID 在 command 真正调用时读取动态 provider identity。
func (c RuntimeCommandContext) CurrentSDKSessionID() string {
	if c.SDKSessionIdentity == nil {
		return ""
	}
	return c.SDKSessionIdentity.Load()
}
