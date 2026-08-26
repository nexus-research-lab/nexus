// INPUT: 宿主验证过的 runtime physical round 与动态 authority state。
// OUTPUT: Goal/Execution/Automation command 共用的可信 Actor。
// POS: round-scoped nexus_runtime 的身份边界；模型输入不能声明 Actor 或责任绑定。
package runtimecommand

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// RoundContext 是一个 physical round 的完整 command identity 与共享动态 authority。
type RoundContext struct {
	SessionKey         string
	RoundID            string
	SourceContextType  string
	SourceContextID    string
	SourceContextLabel string
	CommandContext     runtimectx.RuntimeCommandContext
	Receipts           *ReceiptState
	Attempts           *AttemptState
}

// Actor 是宿主为一个 physical round 固定的 command 调用身份。
type Actor struct {
	OwnerUserID        string
	AgentID            string
	AgentName          string
	SessionKey         string
	SessionLabel       string
	RoundID            string
	LeaseSessionKey    string
	LeaseRoundID       string
	SourceContextType  string
	SourceContextID    string
	SourceContextLabel string
	DefaultTimezone    string
	IsMainAgent        bool
	CurrentJobID       string
	CurrentRunID       string
	Round              RoundContext

	// GoalMutationAuthority 可为持久 Goal owner 的私有 Goal-only snapshot；它
	// 不得反向成为 Execution authority。
	GoalMutationAuthority   *runtimectx.GoalAuthorityState
	GoalResponsibilityState *runtimectx.ResponsibilityAuthorityState
}

func (a Actor) normalized() Actor {
	result := a
	result.OwnerUserID = strings.TrimSpace(result.OwnerUserID)
	result.AgentID = strings.TrimSpace(result.AgentID)
	result.AgentName = strings.TrimSpace(result.AgentName)
	result.SessionKey = strings.TrimSpace(result.SessionKey)
	result.SessionLabel = strings.TrimSpace(result.SessionLabel)
	result.RoundID = strings.TrimSpace(result.RoundID)
	result.LeaseSessionKey = strings.TrimSpace(result.LeaseSessionKey)
	result.LeaseRoundID = strings.TrimSpace(result.LeaseRoundID)
	result.SourceContextType = strings.ToLower(strings.TrimSpace(result.SourceContextType))
	result.SourceContextID = strings.TrimSpace(result.SourceContextID)
	result.SourceContextLabel = strings.TrimSpace(result.SourceContextLabel)
	result.DefaultTimezone = strings.TrimSpace(result.DefaultTimezone)
	result.CurrentJobID = strings.TrimSpace(result.CurrentJobID)
	result.CurrentRunID = strings.TrimSpace(result.CurrentRunID)
	return result
}

func (a Actor) valid() bool {
	value := a.normalized()
	return value.OwnerUserID != "" && value.AgentID != "" &&
		value.SessionKey != "" && value.RoundID != "" &&
		value.LeaseSessionKey != "" && value.LeaseRoundID != ""
}

func (a Actor) Valid() bool { return a.valid() }

// MutationAllowed 只允许可信交互来源修改 Automation；Goal/Execution 另由各自
// exact authority 与 Plan Mode 门禁决定。
func (a Actor) MutationAllowed() bool {
	if strings.TrimSpace(a.CurrentJobID) != "" || strings.TrimSpace(a.CurrentRunID) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(a.SourceContextType)) {
	case "agent", "agent_paired", "room":
		return true
	default:
		return false
	}
}

func (a Actor) CrossAgentAllowed() bool {
	return a.IsMainAgent && strings.TrimSpace(a.SourceContextType) == "agent" && a.MutationAllowed()
}

func (a Actor) AutomationRun() *protocol.AutomationRunContext {
	return a.Round.CommandContext.AutomationRun
}
