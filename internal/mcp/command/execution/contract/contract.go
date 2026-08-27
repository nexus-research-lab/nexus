// INPUT: 当前 owner/Agent/scope/session/round identity、exact WorkGraph preview、统一动态 Responsibility authority 与 Orchestration 应用服务。
// OUTPUT: 模型语义工具共用、每次调用原子读取 Goal/Execution/Work/Review、preview 与 durable Plan proposal binding 的权威 context 和窄服务接口。
// POS: command operation adapter 与 service/orchestration 之间不接受模型伪造业务身份、preview 或 proposal 选择的消费侧契约。
package contract

import (
	"context"
	"strings"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

// Service is the complete model-facing semantic command surface.
//
// Machine bookkeeping such as starting Attempts is deliberately absent.
type Service interface {
	GetCurrent(context.Context, orchestration.ActorContext) (*protocol.ExecutionSnapshot, error)
	GetSnapshot(context.Context, orchestration.ActorContext, string) (*protocol.ExecutionSnapshot, error)
	ReadCurrent(context.Context, orchestration.ActorContext) (*protocol.ExecutionSnapshot, error)
	ReadSnapshot(context.Context, orchestration.ActorContext, string) (*protocol.ExecutionSnapshot, error)
	PreparePlanExecution(context.Context, orchestration.ActorContext, orchestration.PreparePlanExecutionInput) (*protocol.ExecutionPlanProposal, error)
	ResolvePlanExecutionProposal(context.Context, orchestration.ActorContext) (*protocol.ExecutionPlanProposal, error)
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

// WorkflowService is the owner-scoped semantic library used by the
// named WorkGraph save operation. Source session and owner identity stay host-owned.
type WorkflowService interface {
	SavePreview(
		context.Context,
		string,
		protocol.SaveWorkGraphWorkflowRequest,
	) (*protocol.WorkGraphWorkflow, error)
}

// WorkflowAuthoringService 是普通 DM/Room 中由 execution-orchestrator Skill 使用的统一草图能力。
// owner 与 source Session identity 由 runtime capability 固定；模型只能选择该 Session 内的 source/preview/version。
type WorkflowAuthoringService interface {
	InspectLibrary(context.Context, string, string) (*protocol.WorkGraphWorkflowLibrary, error)
	PreviewFromExecution(context.Context, string, protocol.PreviewWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflowPreview, error)
	GetDraft(context.Context, string, string, string) (*protocol.WorkGraphWorkflowDraft, error)
	ReviseDraftPreview(context.Context, string, string, protocol.ReviseWorkGraphWorkflowDraftRequest) (*protocol.WorkGraphWorkflowDraft, error)
	SelectDraftRevision(context.Context, string, string, string, int64, int64) (*protocol.WorkGraphWorkflowDraft, error)
	SavePreview(context.Context, string, protocol.SaveWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflow, error)
}

// WorkflowEditorService 是隐藏专用 DM 中唯一可写的草图 revision/selection 边界。
// owner 与 editor Session identity 只来自 runtime capability，不进入模型输入。
type WorkflowEditorService interface {
	RuntimeEditorActive(string, string) bool
	ReviseEditorPreview(
		context.Context,
		string,
		string,
		protocol.ReviseWorkGraphWorkflowPreviewRequest,
	) (*protocol.WorkGraphWorkflowEditorSession, error)
	SelectEditorVersionBySession(context.Context, string, string, int64, int64) (*protocol.WorkGraphWorkflowEditorSession, error)
}

// Context contains authoritative runtime identity. None of these fields
// are accepted from command input.
type Context struct {
	OwnerUserID             string
	AgentID                 string
	Role                    orchestration.ExecutionActorRole
	ActorKind               protocol.ExecutionActorKind
	ScopeKind               protocol.ExecutionScopeKind
	ScopeSessionKey         string
	RuntimeSessionKey       string
	ExecutionID             string
	WorkBinding             *protocol.ExecutionWorkBinding
	WorkBindingState        *runtimectx.WorkBindingState
	ReviewBinding           *protocol.ExecutionReviewBinding
	GoalAuthority           *runtimectx.GoalAuthorityState
	ResponsibilityAuthority *runtimectx.ResponsibilityAuthorityState
	RootRoundID             string
	RuntimeRoundID          string
	AgentRoundID            string
	RoomID                  string
	ConversationID          string
	RoomSessionID           string
	PlanMode                bool
	CommandAttempts         *nexusmcp.CommandAttemptState
	WorkGraphPreviewID      string
}

// Actor 把 nexus.command 的可信身份投影到应用服务权限边界。
func (c Context) Actor() orchestration.ActorContext {
	goalAuthority := runtimectx.GoalAuthority{}
	workBinding := cloneExecutionWorkBinding(c.WorkBinding)
	reviewBinding := cloneExecutionReviewBinding(c.ReviewBinding)
	executionID := strings.TrimSpace(c.ExecutionID)
	if c.ResponsibilityAuthority != nil {
		authority, _ := c.ResponsibilityAuthority.Load()
		workBinding = cloneExecutionWorkBinding(authority.WorkBinding)
		reviewBinding = cloneExecutionReviewBinding(authority.ReviewBinding)
		executionID = strings.TrimSpace(authority.ExecutionID)
		goalAuthority.GoalID = authority.GoalID
		goalAuthority.ObjectiveRevision = authority.ObjectiveRevision
		goalAuthority.ExecutionID = authority.ExecutionID
	} else {
		goalAuthority, _ = c.GoalAuthority.Load()
	}
	if c.ResponsibilityAuthority == nil && c.WorkBindingState != nil {
		workBinding, _ = c.WorkBindingState.Load()
	}
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
		ReviewBinding:         reviewBinding,
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
