// INPUT: 可信 runtime round、结构化 nexus.command 调用、Goal/Execution/Automation services、exact WorkGraph preview binding 与 runtime permission context。
// OUTPUT: 三个领域共用的 round-scoped MCP server、按需 contract、语义调用结果与 typed mutation receipt。
// POS: Agent-facing Nexus command adapter；身份、责任、preview、Plan Mode 与 Automation 真人确认均由宿主固定。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	executioncontract "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	executionoperation "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/operation"
	goalcontract "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/goal/contract"
	goaloperation "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/goal/operation"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

const (
	runtimeCommandMCPServerName = "nexus"
	runtimeCommandMCPToolName   = "command"
)

func newRuntimeCommandMCPServerBuilder(
	cfg config.Config,
	agents runtimeAgentResolver,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	workflowServices ...executioncontract.WorkflowService,
) func(context.Context, runtimecommand.RoundContext) (map[string]sdkmcp.ServerConfig, error) {
	return func(ctx context.Context, round runtimecommand.RoundContext) (map[string]sdkmcp.ServerConfig, error) {
		agentValue := round.CommandContext.Agent
		if agents == nil || agentValue == nil || round.Receipts == nil {
			return nil, nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		lease, hasLease := runtimectx.RuntimeRoundLeaseFromContext(ctx)
		if agentID == "" || !hasLease || strings.TrimSpace(lease.SessionKey) == "" ||
			strings.TrimSpace(lease.RoundID) == "" {
			return nil, nil
		}
		if round.Attempts == nil {
			round.Attempts = runtimecommand.NewAttemptState()
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" ||
			strings.TrimSpace(record.AgentID) != agentID {
			return nil, err
		}
		actor := runtimecommand.Actor{
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
		} else if !trustedRuntimeCommandActor(ctx, record, actor) {
			actor.SourceContextType = strings.TrimSuffix(actor.SourceContextType, "_untrusted") + "_untrusted"
		}
		actor.IsMainAgent = record.IsMain && trustedMainRuntimeCommandActor(actor)
		actor.GoalMutationAuthority = resolveGoalCommandMutationAuthority(
			ctx,
			goals,
			resolveGoalCommandSessionKey(actor.SessionKey, actor.SourceContextType),
			actor.SourceContextType,
			record,
			round.CommandContext.GoalAuthority,
		)
		actor.GoalResponsibilityState = round.CommandContext.ResponsibilityAuthority
		if actor.GoalMutationAuthority != round.CommandContext.GoalAuthority {
			actor.GoalResponsibilityState = nil
		}
		inputSchema := runtimeCommandMCPInputSchema()
		server := sdktool.NewSimpleSDKMCPServer(
			runtimeCommandMCPServerName,
			"1.0.0",
			[]sdktool.Tool{{
				Name: runtimeCommandMCPToolName,
				Description: "调用 Nexus 托管的 Goal、Execution 或 Automation 命令。" +
					"先用 contract 获取操作目录和精确输入 schema，再用 inspect/invoke 或 plan/apply 执行。",
				SearchHint:  "nexus goal execution automation contract inspect invoke plan apply 目标 执行 自动化",
				AlwaysLoad:  true,
				InputSchema: inputSchema,
				Annotations: &sdktool.ToolAnnotations{Destructive: true},
				Handler: func(callCtx context.Context, input map[string]any) (sdktool.ToolResult, error) {
					if err := runtimecommand.ValidateInput(inputSchema, input); err != nil {
						return runtimeCommandMCPError(err), nil
					}
					command := runtimeCommandRequestFromMCP(input)
					result, err := dispatchRuntimeCommand(
						callCtx,
						automation,
						goals,
						execution,
						permissions,
						actor,
						command,
						workflowServices...,
					)
					if err != nil {
						return runtimeCommandMCPError(err), nil
					}
					return runtimeCommandMCPResult(result), nil
				},
			}},
		)
		return map[string]sdkmcp.ServerConfig{
			runtimeCommandMCPServerName: sdkmcp.SDKServerConfig{
				Name: runtimeCommandMCPServerName, Instance: server,
			},
		}, nil
	}
}

func runtimeCommandMCPInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"domain": map[string]any{
				"type": "string", "enum": []string{
					runtimecommand.DomainAutomation,
					runtimecommand.DomainGoal,
					runtimecommand.DomainExecution,
				},
			},
			"action": map[string]any{
				"type": "string", "enum": []string{
					runtimecommand.ActionContract,
					runtimecommand.ActionInspect,
					runtimecommand.ActionInvoke,
					runtimecommand.ActionPlan,
					runtimecommand.ActionApply,
				},
			},
			"operation":         map[string]any{"type": "string"},
			"input":             map[string]any{"type": "object", "additionalProperties": true},
			"request_id":        map[string]any{"type": "string"},
			"expected_revision": map[string]any{"type": "string"},
			"plan_digest":       map[string]any{"type": "string"},
		},
		"required":             []string{"domain", "action"},
		"additionalProperties": false,
	}
}

func runtimeCommandRequestFromMCP(input map[string]any) runtimecommand.Request {
	command := runtimecommand.Request{
		Domain:           stringValue(input["domain"]),
		Action:           stringValue(input["action"]),
		Operation:        stringValue(input["operation"]),
		RequestID:        stringValue(input["request_id"]),
		ExpectedRevision: stringValue(input["expected_revision"]),
		PlanDigest:       stringValue(input["plan_digest"]),
	}
	command.Input, _ = input["input"].(map[string]any)
	return command
}

func runtimeCommandMCPResult(value any) sdktool.ToolResult {
	if result, ok := value.(runtimecommand.Result); ok {
		return sdktool.ToolResult{
			Content: result.Content, StructuredContent: result.StructuredContent, IsError: result.IsError,
		}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return runtimeCommandMCPError(err)
	}
	structured := map[string]any{}
	if value != nil {
		if err = json.Unmarshal(payload, &structured); err != nil {
			return runtimeCommandMCPError(err)
		}
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": string(payload)}},
		StructuredContent: structured,
	}
}

func runtimeCommandMCPError(err error) sdktool.ToolResult {
	message := "Nexus command 失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": message}},
		StructuredContent: map[string]any{"error": message},
		IsError:           true,
	}
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

func trustedRuntimeCommandActor(ctx context.Context, agent *protocol.Agent, actor runtimecommand.Actor) bool {
	if agent == nil || actor.SessionKey == "" || actor.RoundID == "" || actor.RoundID != actor.LeaseRoundID {
		return false
	}
	switch actor.SourceContextType {
	case protocol.SessionPurposeWorkGraphEditor:
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
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
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
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
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRuntimeRoute(
			actor.AgentID, "agent", actor.SessionKey, actor.RoundID,
			actor.LeaseSessionKey, actor.LeaseRoundID,
		)
		return ok && actor.SourceContextID == actor.AgentID
	case "room":
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRuntimeRoute(
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

func trustedMainRuntimeCommandActor(actor runtimecommand.Actor) bool {
	if actor.SourceContextType != "agent" || actor.SessionKey != actor.LeaseSessionKey {
		return false
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	return parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment && parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(actor.AgentID)
}

func dispatchRuntimeCommand(
	ctx context.Context,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
	command runtimecommand.Request,
	workflowServices ...executioncontract.WorkflowService,
) (any, error) {
	if (actor.SourceContextType == protocol.SessionPurposeWorkGraphEditor ||
		actor.SourceContextType == protocol.SessionPurposeWorkGraphDistillation) &&
		strings.ToLower(strings.TrimSpace(command.Domain)) != runtimecommand.DomainExecution {
		return nil, errors.New("临时 WorkGraph Session 只允许 execution domain")
	}
	switch strings.ToLower(strings.TrimSpace(command.Domain)) {
	case runtimecommand.DomainAutomation:
		return handleAutomationRuntimeCommand(ctx, automation, permissions, actor, command)
	case runtimecommand.DomainGoal:
		return handleGoalRuntimeCommand(ctx, goals, actor, command)
	case runtimecommand.DomainExecution:
		return handleExecutionRuntimeCommand(ctx, execution, actor, command, workflowServices...)
	default:
		return nil, fmt.Errorf("未知 Nexus command domain %q", command.Domain)
	}
}

func handleAutomationRuntimeCommand(
	ctx context.Context,
	svc *automationsvc.Service,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
	command runtimecommand.Request,
) (any, error) {
	if svc == nil {
		return nil, errors.New("Automation service 尚未装配")
	}
	input, err := decodeAutomationCommandInput(command.Input)
	if err != nil {
		return nil, err
	}
	request := automationdomain.AutomationCommandRequest{
		Action: command.Action, Operation: command.Operation, Input: input,
		RequestID: command.RequestID, ExpectedRevision: command.ExpectedRevision,
		PlanDigest: command.PlanDigest,
	}
	switch strings.ToLower(strings.TrimSpace(command.Action)) {
	case runtimecommand.ActionContract:
		return svc.RuntimeCommandContract(ctx, actor)
	case runtimecommand.ActionInspect:
		return svc.InspectRuntimeCommand(ctx, actor, command.Operation, input)
	case runtimecommand.ActionPlan:
		return svc.PlanRuntimeCommand(ctx, actor, command.Operation, input)
	case runtimecommand.ActionReplay:
		replayed, found, replayErr := svc.ReplayRuntimeCommand(ctx, actor, request)
		return automationdomain.AutomationCommandReplayResult{Found: found, Result: replayed}, replayErr
	case runtimecommand.ActionApply:
		replayed, found, replayErr := svc.ReplayRuntimeCommand(ctx, actor, request)
		if replayErr != nil || found {
			return replayed, replayErr
		}
		plan, planErr := svc.PlanRuntimeCommand(ctx, actor, command.Operation, input)
		if planErr != nil {
			return nil, planErr
		}
		approvalRequestID, approvalErr := requireRuntimeAutomationConfirmation(ctx, permissions, actor, *plan)
		if approvalErr != nil {
			return nil, approvalErr
		}
		return svc.ApplyRuntimeCommand(ctx, actor, request, automationsvc.RuntimeCommandApplyOptions{
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

func handleGoalRuntimeCommand(
	ctx context.Context,
	svc goalcontract.Service,
	actor runtimecommand.Actor,
	command runtimecommand.Request,
) (any, error) {
	if svc == nil {
		return nil, errors.New("Goal command service 尚未装配")
	}
	sctx := goalcontract.Context{
		OwnerUserID:       actor.OwnerUserID,
		CurrentSessionKey: resolveGoalCommandSessionKey(actor.SessionKey, actor.SourceContextType),
		CurrentRoundID:    actor.RoundID, CurrentAgentID: actor.AgentID,
		GoalAuthority:           actor.GoalMutationAuthority,
		ResponsibilityAuthority: actor.GoalResponsibilityState,
		AllowUserRetarget:       allowsTrustedUserGoalRetarget(actor.SourceContextType),
		PlanMode:                permissionctx.NormalizeMode(actor.Round.CommandContext.PermissionMode) == sdkpermission.ModePlan,
	}
	operations := goaloperation.BuildAll(svc, sctx)
	return handleSemanticRuntimeCommand(
		ctx, actor, runtimecommand.DomainGoal, "get_goal", operations, command,
	)
}

func handleExecutionRuntimeCommand(
	ctx context.Context,
	svc executioncontract.Service,
	actor runtimecommand.Actor,
	command runtimecommand.Request,
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
			CommandAttempts:    actor.Round.Attempts,
			WorkGraphPreviewID: strings.TrimSpace(roundContext.WorkGraphPreviewID),
		}
		return handleSemanticRuntimeCommand(
			ctx,
			actor,
			runtimecommand.DomainExecution,
			"",
			executionoperation.BuildWorkGraphDistillation(workflowServices[0], sctx),
			command,
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
			CommandAttempts:   actor.Round.Attempts,
		}
		return handleSemanticRuntimeCommand(
			ctx,
			actor,
			runtimecommand.DomainExecution,
			"",
			executionoperation.BuildWorkGraphEditor(editorService, sctx),
			command,
		)
	}
	roundContext := actor.Round.CommandContext
	authoringScopeSessionKey := strings.TrimSpace(roundContext.ScopeSessionKey)
	if authoringScopeSessionKey == "" {
		authoringScopeSessionKey = strings.TrimSpace(actor.SessionKey)
	}
	var authoringOperations []runtimecommand.Operation
	if len(workflowServices) > 0 {
		if authoring, ok := workflowServices[0].(executioncontract.WorkflowAuthoringService); ok {
			authoringOperations = executionoperation.BuildWorkGraphAuthoring(
				authoring,
				executioncontract.Context{
					OwnerUserID: actor.OwnerUserID, AgentID: actor.AgentID,
					ScopeSessionKey: authoringScopeSessionKey, RuntimeSessionKey: actor.SessionKey,
					RootRoundID: actor.RoundID, RuntimeRoundID: actor.LeaseRoundID,
					AgentRoundID:    strings.TrimSpace(roundContext.AgentRoundID),
					CommandAttempts: actor.Round.Attempts,
				},
			)
		}
	}
	if svc == nil {
		if len(authoringOperations) > 0 {
			return handleSemanticRuntimeCommand(
				ctx, actor, runtimecommand.DomainExecution, "", authoringOperations, command,
			)
		}
		return nil, errors.New("Execution command service 尚未装配")
	}
	// Goal create/retarget can advance exact authority during this physical
	// round. Execution must consume the same host-owned state instead of the
	// immutable launch snapshot, or Goal+WorkGraph will self-conflict.
	roundContext.GoalAuthority = actor.GoalMutationAuthority
	roundContext.ResponsibilityAuthority = actor.GoalResponsibilityState
	sctx, ok := resolveExecutionCommandContext(ctx, svc, roundContext)
	if !ok {
		if len(authoringOperations) > 0 {
			return handleSemanticRuntimeCommand(
				ctx, actor, runtimecommand.DomainExecution, "", authoringOperations, command,
			)
		}
		return nil, errors.New("当前 round 没有有效的 Execution command identity")
	}
	sctx.CommandAttempts = actor.Round.Attempts
	operations := executionoperation.BuildAll(svc, sctx, workflowServices...)
	return handleSemanticRuntimeCommand(
		ctx, actor, runtimecommand.DomainExecution, "get_execution", operations, command,
	)
}

func handleSemanticRuntimeCommand(
	ctx context.Context,
	actor runtimecommand.Actor,
	domain string,
	inspectOperation string,
	operations []runtimecommand.Operation,
	command runtimecommand.Request,
) (any, error) {
	action := strings.ToLower(strings.TrimSpace(command.Action))
	switch action {
	case runtimecommand.ActionContract:
		return runtimecommand.BuildContract(domain, inspectOperation, command.Operation, operations)
	case runtimecommand.ActionInspect:
		if strings.TrimSpace(command.Operation) != "" {
			return nil, errors.New("inspect operation 由领域固定，不能覆盖")
		}
		operation, ok := runtimecommand.FindOperation(operations, inspectOperation)
		if !ok {
			return nil, fmt.Errorf("%s inspect operation 未装配", domain)
		}
		return operation.Invoke(ctx, command.Input, nil)
	case runtimecommand.ActionInvoke:
		if !runtimecommand.ValidRequestID(command.RequestID) {
			return nil, errors.New("invoke request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
		}
		operation, ok := runtimecommand.FindOperation(operations, command.Operation)
		if !ok || operation.Name == inspectOperation {
			return nil, fmt.Errorf("未知或不可 invoke 的 %s operation %q", domain, command.Operation)
		}
		result, err := operation.Invoke(ctx, command.Input, &runtimecommand.CallContext{
			RequestID: command.RequestID,
			SessionID: actor.Round.CommandContext.CurrentSDKSessionID(),
		})
		if err == nil {
			recordRuntimeSemanticReceipt(
				actor, command.RequestID, domain, operation.Name, command.Input, result,
			)
		}
		return result, err
	default:
		return nil, fmt.Errorf("%s 只支持 contract、inspect、invoke", domain)
	}
}

func recordRuntimeSemanticReceipt(
	actor runtimecommand.Actor,
	requestID string,
	domain string,
	operation string,
	input map[string]any,
	result runtimecommand.Result,
) {
	if result.IsError || actor.Round.Receipts == nil {
		return
	}
	receipt := runtimecommand.Receipt{
		RequestID: requestID, Domain: domain, Operation: operation,
	}
	if result.StructuredContent != nil {
		receipt.Outcome = stringValue(result.StructuredContent["outcome"])
		receipt.Message = stringValue(result.StructuredContent["message"])
		receipt.ReasonCode = stringValue(result.StructuredContent["reason_code"])
		receipt.ExecutionID = stringValue(result.StructuredContent["execution_id"])
		receipt.Changed = stringSliceValue(result.StructuredContent["changed"])
		receipt.SnapshotRevision = int64Value(result.StructuredContent["snapshot_revision"])
		if goalValue, ok := result.StructuredContent["goal"].(map[string]any); ok {
			receipt.GoalStatus = stringValue(goalValue["status"])
		}
		if receipt.GoalID == "" {
			receipt.GoalID = stringValue(result.StructuredContent["goalId"])
		}
	}
	authority, ok := actor.GoalMutationAuthority.Load()
	if actor.GoalResponsibilityState != nil {
		if shared, sharedOK := actor.GoalResponsibilityState.LoadGoalAuthority(); sharedOK {
			authority, ok = shared, true
		}
	}
	if ok {
		receipt.GoalID = authority.GoalID
		receipt.GoalBound = strings.TrimSpace(authority.GoalID) != "" &&
			strings.TrimSpace(authority.ExecutionID) != ""
	}
	if responsibilityState := actor.Round.CommandContext.ResponsibilityAuthority; responsibilityState != nil {
		if responsibility, loaded := responsibilityState.Load(); loaded {
			if receipt.ExecutionID == "" {
				receipt.ExecutionID = strings.TrimSpace(responsibility.ExecutionID)
			}
			if binding := responsibility.WorkBinding; binding != nil {
				receipt.WorkItemID = binding.WorkItemID
				receipt.AssignmentID = binding.AssignmentID
				receipt.AttemptID = binding.AttemptID
			}
		}
	}
	if operation == "update_goal" && receipt.GoalStatus == "" {
		receipt.GoalStatus = stringValue(input["status"])
	}
	actor.Round.Receipts.Record(receipt)
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringValue(item); value != "" {
				result = append(result, value)
			}
		}
		return result
	default:
		return nil
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}

func requireRuntimeAutomationConfirmation(
	ctx context.Context,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
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
		Input:    runtimeAutomationConfirmationInput(plan),
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

func runtimeAutomationConfirmationInput(plan automationdomain.AutomationCommandPlan) map[string]any {
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
