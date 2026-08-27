// INPUT: 可信 runtime round、Nexus 内建工具组、Goal/Execution/Automation services 与 runtime permission context。
// OUTPUT: 单一 round-scoped nexus MCP server、按需 contract、语义调用结果与 typed mutation receipt。
// POS: Nexus 内建工具的唯一 server 装配点；身份、责任、preview、Plan Mode 与真人确认均由宿主固定。
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	serverexecution "github.com/nexus-research-lab/nexus/internal/app/server/execution"
	servergoal "github.com/nexus-research-lab/nexus/internal/app/server/goal"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	executioncontract "github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	executionoperation "github.com/nexus-research-lab/nexus/internal/mcp/command/execution/operation"
	goalcontract "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
	goaloperation "github.com/nexus-research-lab/nexus/internal/mcp/command/goal/operation"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

const nexusMCPServerName = "nexus"

// ToolBuilder 为一个 physical round 构建某个内建能力的工具定义。
type ToolBuilder func(context.Context, nexusmcp.RoundContext) []sdktool.Tool

// CombineToolBuilders 合并同一 physical round 的内建工具。
func CombineToolBuilders(builders ...ToolBuilder) ToolBuilder {
	return func(ctx context.Context, round nexusmcp.RoundContext) []sdktool.Tool {
		var tools []sdktool.Tool
		for _, builder := range builders {
			if builder != nil {
				tools = append(tools, builder(ctx, round)...)
			}
		}
		return tools
	}
}

// NewServerBuilder 构建每个 round 唯一的 Nexus MCP server。
func NewServerBuilder(
	cfg config.Config,
	agents AgentResolver,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	builtInTools ToolBuilder,
	workflowServices ...executioncontract.WorkflowService,
) func(context.Context, nexusmcp.RoundContext) (map[string]sdkmcp.ServerConfig, error) {
	return func(ctx context.Context, round nexusmcp.RoundContext) (map[string]sdkmcp.ServerConfig, error) {
		definitions := make([]sdktool.Tool, 0, 1)
		commandTool, ok, err := buildNexusCommandMCPTool(
			ctx,
			cfg,
			agents,
			automation,
			goals,
			execution,
			permissions,
			round,
			workflowServices...,
		)
		if err != nil {
			return nil, err
		}
		if ok {
			definitions = append(definitions, commandTool)
		}
		if builtInTools != nil && !isolatedWorkGraphMCPRound(round.SourceContextType) {
			definitions = append(definitions, builtInTools(ctx, round)...)
		}
		if len(definitions) == 0 {
			return nil, nil
		}
		server := newRoundScopedMCPServer(
			ctx,
			sdktool.NewSimpleSDKMCPServer(
				nexusMCPServerName,
				"1.0.0",
				definitions,
			),
		)
		return map[string]sdkmcp.ServerConfig{
			nexusMCPServerName: sdkmcp.SDKServerConfig{
				Name: nexusMCPServerName, Instance: server,
			},
		}, nil
	}
}

// roundScopedMCPServer 把宿主已核验的真人身份带回 bridge 发起的工具调用，
// 同时保留控制请求自身的取消信号。服务层仍会重新验证 Session、角色与 round lease。
type roundScopedMCPServer struct {
	server              sdkmcp.SDKMCPServer
	principal           *authctx.Principal
	humanEvidenceSource string
}

func newRoundScopedMCPServer(
	ctx context.Context,
	server sdkmcp.SDKMCPServer,
) sdkmcp.SDKMCPServer {
	scoped := &roundScopedMCPServer{
		server:    server,
		principal: clonePrincipal(authctx.PrincipalFromContext(ctx)),
	}
	if evidence, ok := authctx.InteractiveHumanEvidenceFromContext(ctx); ok {
		scoped.humanEvidenceSource = evidence.Source
	}
	return scoped
}

func (s *roundScopedMCPServer) HandleMessage(
	ctx context.Context,
	message map[string]any,
) (map[string]any, error) {
	ctx = authctx.WithPrincipal(ctx, clonePrincipal(s.principal))
	ctx = authctx.WithInteractiveHumanEvidence(ctx, s.humanEvidenceSource)
	return s.server.HandleMessage(ctx, message)
}

func clonePrincipal(principal *authctx.Principal) *authctx.Principal {
	if principal == nil {
		return nil
	}
	cloned := *principal
	if principal.SessionID != nil {
		sessionID := *principal.SessionID
		cloned.SessionID = &sessionID
	}
	return &cloned
}

func isolatedWorkGraphMCPRound(sourceContextType string) bool {
	switch strings.TrimSpace(sourceContextType) {
	case protocol.SessionPurposeWorkGraphEditor, protocol.SessionPurposeWorkGraphDistillation:
		return true
	default:
		return false
	}
}

func buildNexusCommandMCPTool(
	ctx context.Context,
	cfg config.Config,
	agents AgentResolver,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	round nexusmcp.RoundContext,
	workflowServices ...executioncontract.WorkflowService,
) (sdktool.Tool, bool, error) {
	agentValue := round.CommandContext.Agent
	if agents == nil || agentValue == nil || round.CommandReceipts == nil {
		return sdktool.Tool{}, false, nil
	}
	agentID := strings.TrimSpace(agentValue.AgentID)
	lease, hasLease := runtimectx.RuntimeRoundLeaseFromContext(ctx)
	if agentID == "" || !hasLease || strings.TrimSpace(lease.SessionKey) == "" ||
		strings.TrimSpace(lease.RoundID) == "" {
		return sdktool.Tool{}, false, nil
	}
	if round.CommandAttempts == nil {
		round.CommandAttempts = nexusmcp.NewCommandAttemptState()
	}
	record, err := agents.GetAgent(ctx, agentID)
	if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" ||
		strings.TrimSpace(record.AgentID) != agentID {
		return sdktool.Tool{}, false, err
	}
	actor := command.Actor{
		OwnerUserID: strings.TrimSpace(record.OwnerUserID),
		AgentID:     agentID, AgentName: strings.TrimSpace(record.Name),
		SessionKey: strings.TrimSpace(round.SessionKey), RoundID: strings.TrimSpace(round.RoundID),
		LeaseSessionKey: strings.TrimSpace(lease.SessionKey), LeaseRoundID: strings.TrimSpace(lease.RoundID),
		SourceContextType:  strings.ToLower(strings.TrimSpace(round.SourceContextType)),
		SourceContextID:    strings.TrimSpace(round.SourceContextID),
		SourceContextLabel: strings.TrimSpace(round.SourceContextLabel),
		SessionLabel:       strings.TrimSpace(round.SourceContextLabel),
		DefaultTimezone:    strings.TrimSpace(cfg.DefaultTimezone),
		Round:              round,
	}
	if normalized := normalizedAutomationRunContext(round.CommandContext.AutomationRun); normalized != nil {
		actor.SourceContextType = "automation_run"
		actor.SourceContextID = normalized.JobID
		actor.SourceContextLabel = normalized.JobName
		actor.CurrentJobID = normalized.JobID
		actor.CurrentRunID = normalized.RunID
	} else if !trustedCommandActor(ctx, record, actor) {
		actor.SourceContextType = strings.TrimSuffix(actor.SourceContextType, "_untrusted") + "_untrusted"
	}
	actor.IsMainAgent = record.IsMain && trustedMainCommandActor(actor)
	actor.GoalMutationAuthority = servergoal.ResolveCommandMutationAuthority(
		ctx,
		goals,
		servergoal.ResolveCommandSessionKey(actor.SessionKey, actor.SourceContextType),
		actor.SourceContextType,
		record,
		round.CommandContext.GoalAuthority,
	)
	actor.GoalResponsibilityState = round.CommandContext.ResponsibilityAuthority
	if actor.GoalMutationAuthority != round.CommandContext.GoalAuthority {
		actor.GoalResponsibilityState = nil
	}
	return command.NewTool(func(callCtx context.Context, request command.Request) (any, error) {
		return dispatchNexusCommand(
			callCtx,
			automation,
			goals,
			execution,
			permissions,
			actor,
			request,
			workflowServices...,
		)
	}), true, nil
}

func normalizedAutomationRunContext(value *protocol.AutomationRunContext) *protocol.AutomationRunContext {
	if value == nil {
		return nil
	}
	normalized := value.Normalized()
	if !normalized.Valid() {
		return nil
	}
	return &normalized
}

func trustedCommandActor(ctx context.Context, agent *protocol.Agent, actor command.Actor) bool {
	if agent == nil || actor.SessionKey == "" || actor.RoundID == "" || actor.RoundID != actor.LeaseRoundID {
		return false
	}
	switch actor.SourceContextType {
	case protocol.SessionPurposeWorkGraphEditor:
		if _, _, _, ok := trustedPrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		parsed := protocol.ParseSessionKey(actor.SessionKey)
		return actor.SessionKey == actor.LeaseSessionKey &&
			parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
			parsed.Channel == protocol.SessionChannelWebSocketSegment &&
			parsed.ChatType == protocol.RoomTypeDM &&
			strings.TrimSpace(parsed.AgentID) == actor.AgentID &&
			actor.SourceContextID == actor.AgentID
	case protocol.SessionPurposeWorkGraphDistillation:
		if _, _, _, ok := trustedPrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		parsed := protocol.ParseSessionKey(actor.SessionKey)
		return actor.SessionKey == actor.LeaseSessionKey &&
			parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
			parsed.Channel == protocol.SessionChannelInternalSegment &&
			parsed.ChatType == protocol.RoomTypeDM &&
			strings.TrimSpace(parsed.AgentID) == actor.AgentID &&
			actor.SourceContextID == actor.AgentID &&
			strings.TrimSpace(actor.Round.CommandContext.ScopeSessionKey) != "" &&
			strings.TrimSpace(actor.Round.CommandContext.WorkGraphPreviewID) != ""
	case "agent":
		if _, _, _, ok := trustedPrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRoute(
			actor.AgentID, "agent", actor.SessionKey, actor.RoundID,
			actor.LeaseSessionKey, actor.LeaseRoundID,
		)
		return ok && actor.SourceContextID == actor.AgentID
	case "room":
		if _, _, _, ok := trustedPrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRoute(
			actor.AgentID, "room", actor.SessionKey, actor.RoundID,
			actor.LeaseSessionKey, actor.LeaseRoundID,
		)
		return ok && actor.SourceContextID != ""
	case "agent_paired":
		parsed := protocol.ParseSessionKey(actor.SessionKey)
		return actor.SessionKey == actor.LeaseSessionKey && parsed.IsStructured &&
			parsed.Kind == protocol.SessionKeyKindAgent && parsed.ChatType == protocol.RoomTypeDM &&
			strings.TrimSpace(parsed.AgentID) == actor.AgentID &&
			protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelWebSocket &&
			protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelInternalSegment
	default:
		return false
	}
}

func trustedMainCommandActor(actor command.Actor) bool {
	if actor.SourceContextType != "agent" || actor.SessionKey != actor.LeaseSessionKey {
		return false
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	return parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment && parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(actor.AgentID)
}

func dispatchNexusCommand(
	ctx context.Context,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	actor command.Actor,
	request command.Request,
	workflowServices ...executioncontract.WorkflowService,
) (any, error) {
	if (actor.SourceContextType == protocol.SessionPurposeWorkGraphEditor ||
		actor.SourceContextType == protocol.SessionPurposeWorkGraphDistillation) &&
		strings.ToLower(strings.TrimSpace(request.Domain)) != command.DomainExecution {
		return nil, errors.New("临时 WorkGraph Session 只允许 execution domain")
	}
	switch strings.ToLower(strings.TrimSpace(request.Domain)) {
	case command.DomainAutomation:
		return handleAutomationCommand(ctx, automation, permissions, actor, request)
	case command.DomainGoal:
		return HandleGoalCommand(ctx, goals, actor, request)
	case command.DomainExecution:
		return HandleExecutionCommand(ctx, execution, actor, request, workflowServices...)
	default:
		return nil, fmt.Errorf("未知 Nexus command domain %q", request.Domain)
	}
}

func handleAutomationCommand(
	ctx context.Context,
	svc *automationsvc.Service,
	permissions *permissionctx.Context,
	actor command.Actor,
	request command.Request,
) (any, error) {
	if svc == nil {
		return nil, errors.New("Automation service 尚未装配")
	}
	input, err := decodeAutomationCommandInput(request.Input)
	if err != nil {
		return nil, err
	}
	automationRequest := automationdomain.AutomationCommandRequest{
		Action: request.Action, Operation: request.Operation, Input: input,
		RequestID: request.RequestID, ExpectedRevision: request.ExpectedRevision,
		PlanDigest: request.PlanDigest,
	}
	switch strings.ToLower(strings.TrimSpace(request.Action)) {
	case command.ActionContract:
		return svc.RuntimeCommandContract(ctx, actor)
	case command.ActionInspect:
		return svc.InspectRuntimeCommand(ctx, actor, request.Operation, input)
	case command.ActionPlan:
		return svc.PlanRuntimeCommand(ctx, actor, request.Operation, input)
	case command.ActionReplay:
		replayed, found, replayErr := svc.ReplayRuntimeCommand(ctx, actor, automationRequest)
		return automationdomain.AutomationCommandReplayResult{Found: found, Result: replayed}, replayErr
	case command.ActionApply:
		replayed, found, replayErr := svc.ReplayRuntimeCommand(ctx, actor, automationRequest)
		if replayErr != nil || found {
			return replayed, replayErr
		}
		plan, planErr := svc.PlanRuntimeCommand(ctx, actor, request.Operation, input)
		if planErr != nil {
			return nil, planErr
		}
		approvalRequestID, approvalErr := requireAutomationConfirmation(ctx, permissions, actor, *plan)
		if approvalErr != nil {
			return nil, approvalErr
		}
		return svc.ApplyRuntimeCommand(ctx, actor, automationRequest, automationsvc.RuntimeCommandApplyOptions{
			HumanConfirmed: true, HumanApprovalRequestID: approvalRequestID,
		})
	default:
		return nil, errors.New("未知 Nexus Automation command action")
	}
}

func decodeAutomationCommandInput(input map[string]any) (automationdomain.AutomationCommandInput, error) {
	if input == nil {
		input = map[string]any{}
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return automationdomain.AutomationCommandInput{}, err
	}
	var result automationdomain.AutomationCommandInput
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("Automation input 无效: %w", err)
	}
	return result, nil
}

// HandleGoalCommand 执行已绑定身份的 Goal 命令。
func HandleGoalCommand(
	ctx context.Context,
	svc goalcontract.Service,
	actor command.Actor,
	request command.Request,
) (any, error) {
	if svc == nil {
		return nil, errors.New("Goal command service 尚未装配")
	}
	sctx := goalcontract.Context{
		OwnerUserID:       actor.OwnerUserID,
		CurrentSessionKey: servergoal.ResolveCommandSessionKey(actor.SessionKey, actor.SourceContextType),
		CurrentRoundID:    actor.RoundID, CurrentAgentID: actor.AgentID,
		GoalAuthority:           actor.GoalMutationAuthority,
		ResponsibilityAuthority: actor.GoalResponsibilityState,
		AllowUserRetarget:       servergoal.AllowsTrustedUserRetarget(actor.SourceContextType),
		PlanMode:                permissionctx.NormalizeMode(actor.Round.CommandContext.PermissionMode) == sdkpermission.ModePlan,
	}
	operations := goaloperation.BuildAll(svc, sctx)
	return command.HandleSemantic(
		ctx, actor, command.DomainGoal, "get_goal", operations, request,
	)
}

// HandleExecutionCommand 执行已绑定身份的 Execution 命令。
func HandleExecutionCommand(
	ctx context.Context,
	svc executioncontract.Service,
	actor command.Actor,
	request command.Request,
	workflowServices ...executioncontract.WorkflowService,
) (any, error) {
	if actor.SourceContextType == protocol.SessionPurposeWorkGraphDistillation {
		if len(workflowServices) == 0 || workflowServices[0] == nil {
			return nil, errors.New("WorkGraph distillation command service 尚未装配")
		}
		roundContext := actor.Round.CommandContext
		sctx := executioncontract.Context{
			OwnerUserID:        actor.OwnerUserID,
			AgentID:            actor.AgentID,
			ScopeSessionKey:    strings.TrimSpace(roundContext.ScopeSessionKey),
			RuntimeSessionKey:  actor.SessionKey,
			RootRoundID:        actor.RoundID,
			RuntimeRoundID:     actor.LeaseRoundID,
			AgentRoundID:       strings.TrimSpace(roundContext.AgentRoundID),
			CommandAttempts:    actor.Round.CommandAttempts,
			WorkGraphPreviewID: strings.TrimSpace(roundContext.WorkGraphPreviewID),
		}
		return command.HandleSemantic(
			ctx,
			actor,
			command.DomainExecution,
			"",
			executionoperation.BuildWorkGraphDistillation(workflowServices[0], sctx),
			request,
		)
	}
	if actor.SourceContextType == protocol.SessionPurposeWorkGraphEditor {
		if len(workflowServices) == 0 || workflowServices[0] == nil {
			return nil, errors.New("WorkGraph editor command service 尚未装配")
		}
		editorService, ok := workflowServices[0].(executioncontract.WorkflowEditorService)
		if !ok || !editorService.RuntimeEditorActive(actor.OwnerUserID, actor.SessionKey) {
			return nil, errors.New("当前 round 没有有效的 WorkGraph editor command identity")
		}
		sctx := executioncontract.Context{
			OwnerUserID:       actor.OwnerUserID,
			AgentID:           actor.AgentID,
			ScopeSessionKey:   actor.SessionKey,
			RuntimeSessionKey: actor.SessionKey,
			RootRoundID:       actor.RoundID,
			RuntimeRoundID:    actor.LeaseRoundID,
			AgentRoundID:      actor.Round.CommandContext.AgentRoundID,
			CommandAttempts:   actor.Round.CommandAttempts,
		}
		return command.HandleSemantic(
			ctx,
			actor,
			command.DomainExecution,
			"",
			executionoperation.BuildWorkGraphEditor(editorService, sctx),
			request,
		)
	}
	roundContext := actor.Round.CommandContext
	authoringScopeSessionKey := strings.TrimSpace(roundContext.ScopeSessionKey)
	if authoringScopeSessionKey == "" {
		authoringScopeSessionKey = strings.TrimSpace(actor.SessionKey)
	}
	var authoringOperations []command.Operation
	if len(workflowServices) > 0 {
		if authoring, ok := workflowServices[0].(executioncontract.WorkflowAuthoringService); ok {
			authoringOperations = executionoperation.BuildWorkGraphAuthoring(
				authoring,
				executioncontract.Context{
					OwnerUserID: actor.OwnerUserID, AgentID: actor.AgentID,
					ScopeSessionKey: authoringScopeSessionKey, RuntimeSessionKey: actor.SessionKey,
					RootRoundID: actor.RoundID, RuntimeRoundID: actor.LeaseRoundID,
					AgentRoundID:    strings.TrimSpace(roundContext.AgentRoundID),
					CommandAttempts: actor.Round.CommandAttempts,
				},
			)
		}
	}
	if svc == nil {
		if len(authoringOperations) > 0 {
			return command.HandleSemantic(
				ctx, actor, command.DomainExecution, "", authoringOperations, request,
			)
		}
		return nil, errors.New("Execution command service 尚未装配")
	}
	// Goal create/retarget can advance exact authority during this physical
	// round. Execution must consume the same host-owned state instead of the
	// immutable launch snapshot, or Goal+WorkGraph will self-conflict.
	roundContext.GoalAuthority = actor.GoalMutationAuthority
	roundContext.ResponsibilityAuthority = actor.GoalResponsibilityState
	sctx, ok := serverexecution.ResolveCommandContext(ctx, svc, roundContext)
	if !ok {
		if len(authoringOperations) > 0 {
			return command.HandleSemantic(
				ctx, actor, command.DomainExecution, "", authoringOperations, request,
			)
		}
		return nil, errors.New("当前 round 没有有效的 Execution command identity")
	}
	sctx.CommandAttempts = actor.Round.CommandAttempts
	operations := executionoperation.BuildAll(svc, sctx, workflowServices...)
	return command.HandleSemantic(
		ctx, actor, command.DomainExecution, "get_execution", operations, request,
	)
}

func requireAutomationConfirmation(
	ctx context.Context,
	permissions *permissionctx.Context,
	actor command.Actor,
	plan automationdomain.AutomationCommandPlan,
) (string, error) {
	if !plan.RequiresConfirmation {
		return "not-required", nil
	}
	if permissions == nil {
		return "", errors.New("Automation 真人确认服务尚未装配")
	}
	decision, requestID, err := permissions.RequestPermissionWithID(ctx, actor.LeaseSessionKey, sdkpermission.Request{
		ToolName: "nexus_automation_apply",
		Input:    automationConfirmationInput(plan),
		Title:    "确认 Nexus 自动化变更", DisplayName: "Nexus Automation",
		Description: plan.Summary,
	})
	if err != nil {
		return requestID, err
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		message := strings.TrimSpace(decision.Message)
		if message == "" {
			message = "用户未批准 Automation 变更"
		}
		return requestID, errors.New(message)
	}
	return requestID, nil
}

func automationConfirmationInput(plan automationdomain.AutomationCommandPlan) map[string]any {
	input := map[string]any{
		"operation": plan.Operation, "target": plan.Target, "summary": plan.Summary,
		"risk": plan.Risk, "revision": plan.CurrentRevision, "plan_digest": plan.PlanDigest,
	}
	changes := map[string]any{}
	add := func(name string, value any, present bool) {
		if present {
			changes[name] = value
		}
	}
	command := plan.Input
	add("name", command.Name, strings.TrimSpace(command.Name) != "")
	add("instruction", command.Instruction, strings.TrimSpace(command.Instruction) != "")
	add("instruction_append", command.InstructionAdd, strings.TrimSpace(command.InstructionAdd) != "")
	add("schedule", command.Schedule, command.Schedule != nil)
	add("agent_id", command.AgentID, strings.TrimSpace(command.AgentID) != "")
	add("context_mode", command.ContextMode, strings.TrimSpace(command.ContextMode) != "")
	add("deliver_result", command.DeliverResult, command.DeliverResult != nil)
	add("permission_mode", command.PermissionMode, strings.TrimSpace(command.PermissionMode) != "")
	add("overlap_policy", command.OverlapPolicy, strings.TrimSpace(command.OverlapPolicy) != "")
	add("expires_at", command.ExpiresAt, strings.TrimSpace(command.ExpiresAt) != "")
	add("clear_expires_at", command.ClearExpiresAt, command.ClearExpiresAt)
	add("enabled", command.Enabled, command.Enabled != nil)
	add("cancel_active_run", command.CancelActiveRun, command.CancelActiveRun)
	add("run_id", command.RunID, strings.TrimSpace(command.RunID) != "")
	add("every_seconds", command.EverySeconds, command.EverySeconds > 0)
	add("target_mode", command.TargetMode, strings.TrimSpace(command.TargetMode) != "")
	add("ack_max_chars", command.AckMaxChars, command.AckMaxChars != nil)
	add("wake_mode", command.Mode, strings.TrimSpace(command.Mode) != "")
	add("text", command.Text, strings.TrimSpace(command.Text) != "")
	if len(changes) > 0 {
		input["changes"] = changes
	}
	return input
}
