// INPUT: 当前 owner/Agent/scope/session/round identity、trusted WorkBinding/ReviewBinding、共享 Goal authority 与 Orchestration 应用服务。
// OUTPUT: 十二个模型语义工具共用、每次调用动态读取 Goal identity 的权威 actor context 和窄服务接口。
// POS: MCP tool adapter 与 service/orchestration 之间不接受模型伪造业务身份的消费侧契约。
package contract

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

// ServerName is the built-in Execution Orchestration MCP server name.
const ServerName = "nexus_execution"

// ServerVersion changes whenever the model-facing tool set or schema breaks compatibility.
const ServerVersion = "2.0.0"

// Service is the complete model-facing semantic command surface.
//
// Machine bookkeeping such as starting Attempts is deliberately absent.
type Service interface {
	GetCurrent(context.Context, orchestration.ActorContext) (*protocol.ExecutionSnapshot, error)
	GetSnapshot(context.Context, orchestration.ActorContext, string) (*protocol.ExecutionSnapshot, error)
	PreparePlanExecution(context.Context, orchestration.ActorContext, orchestration.PreparePlanExecutionInput) (*protocol.ExecutionPlanProposal, error)
	MaterializePlanExecution(context.Context, orchestration.ActorContext, orchestration.MaterializePlanExecutionInput) (orchestration.MutationResult, error)
	AbandonExecution(context.Context, orchestration.ActorContext, orchestration.AbandonExecutionInput) (orchestration.MutationResult, error)
	AssignWork(context.Context, orchestration.ActorContext, orchestration.AssignWorkInput) (orchestration.MutationResult, error)
	SubmitWork(context.Context, orchestration.ActorContext, orchestration.SubmitWorkInput) (orchestration.MutationResult, error)
	ReviewWork(context.Context, orchestration.ActorContext, orchestration.ReviewWorkInput) (orchestration.MutationResult, error)
	BlockWork(context.Context, orchestration.ActorContext, orchestration.BlockWorkInput) (orchestration.MutationResult, error)
	ResumeWork(context.Context, orchestration.ActorContext, orchestration.ResumeWorkInput) (orchestration.MutationResult, error)
	TakeOverWork(context.Context, orchestration.ActorContext, orchestration.TakeOverWorkInput) (orchestration.MutationResult, error)
	AuditExecutionAlignment(
		context.Context,
		orchestration.ActorContext,
		orchestration.AuditExecutionAlignmentInput,
	) (orchestration.MutationResult, error)
	PromoteExecutionToGoal(
		context.Context,
		orchestration.ActorContext,
		orchestration.PromoteExecutionToGoalInput,
	) (orchestration.MutationResult, error)
}

// ServerContext contains authoritative runtime identity. None of these fields
// are accepted from model tool input.
type ServerContext struct {
	OwnerUserID       string
	AgentID           string
	Role              orchestration.ExecutionActorRole
	ActorKind         protocol.ExecutionActorKind
	ScopeKind         protocol.ExecutionScopeKind
	ScopeSessionKey   string
	RuntimeSessionKey string
	ExecutionID       string
	WorkBinding       *protocol.ExecutionWorkBinding
	WorkBindingState  *runtimectx.WorkBindingState
	ReviewBinding     *protocol.ExecutionReviewBinding
	GoalAuthority     *runtimectx.GoalAuthorityState
	RootRoundID       string
	RuntimeRoundID    string
	AgentRoundID      string
	RoomID            string
	ConversationID    string
	RoomSessionID     string
	PlanMode          bool
}

// Actor projects MCP runtime identity into the application service authority
// boundary.
func (c ServerContext) Actor() orchestration.ActorContext {
	goalAuthority, _ := c.GoalAuthority.Load()
	workBinding := cloneExecutionWorkBinding(c.WorkBinding)
	if c.WorkBindingState != nil {
		workBinding, _ = c.WorkBindingState.Load()
	}
	executionID := strings.TrimSpace(c.ExecutionID)
	if workBinding != nil {
		executionID = strings.TrimSpace(workBinding.ExecutionID)
	}
	if executionID == "" {
		executionID = strings.TrimSpace(goalAuthority.ExecutionID)
	}
	return orchestration.ActorContext{
		OwnerUserID:           strings.TrimSpace(c.OwnerUserID),
		SessionKey:            strings.TrimSpace(c.ScopeSessionKey),
		ExecutionID:           executionID,
		WorkBinding:           workBinding,
		ReviewBinding:         cloneExecutionReviewBinding(c.ReviewBinding),
		GoalID:                goalAuthority.GoalID,
		GoalObjectiveRevision: goalAuthority.ObjectiveRevision,
		AgentID:               strings.TrimSpace(c.AgentID),
		Role:                  c.Role,
		ActorKind:             c.ActorKind,
		ScopeKind:             c.ScopeKind,
		RoomID:                strings.TrimSpace(c.RoomID),
		ConversationID:        strings.TrimSpace(c.ConversationID),
		RootRoundID:           strings.TrimSpace(c.RootRoundID),
		RuntimeRoundID:        strings.TrimSpace(c.RuntimeRoundID),
		AgentRoundID:          strings.TrimSpace(c.AgentRoundID),
		PlanMode:              c.PlanMode,
	}
}

func cloneExecutionReviewBinding(
	binding *protocol.ExecutionReviewBinding,
) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func cloneExecutionWorkBinding(
	binding *protocol.ExecutionWorkBinding,
) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}
