// INPUT: 可信 runtime round、loopback nexus CLI 请求、Goal/Execution/Automation services 与 runtime permission context。
// OUTPUT: 三个领域共用的 command capability 环境、按需 contract、语义调用结果与 typed mutation receipt。
// POS: Agent-facing Nexus command broker；身份、责任绑定、Plan Mode 与 Automation 真人确认均由宿主固定。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
	executioncontract "github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	executionoperation "github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/operation"
	goalcontract "github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
	goaloperation "github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/operation"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

const maxRuntimeCommandRequestBytes = 1 << 20

func newRuntimeCommandEnvironmentBuilder(
	cfg config.Config,
	registry *runtimecommand.Registry,
	agents runtimeAgentResolver,
	goals goalcontract.Service,
) func(context.Context, runtimecommand.RoundContext) (map[string]string, error) {
	endpoint := runtimeCommandBrokerURL(cfg)
	return func(ctx context.Context, round runtimecommand.RoundContext) (map[string]string, error) {
		agentValue := round.CommandContext.Agent
		if registry == nil || agents == nil || agentValue == nil || round.Receipts == nil {
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
		token, err := registry.Issue(actor)
		if err != nil {
			return nil, err
		}
		environment := map[string]string{
			protocol.NexusCommandBrokerURLEnvName:       endpoint,
			protocol.NexusCommandCapabilityTokenEnvName: token,
		}
		inputPath, cleanup, stagingErr := prepareRuntimeCommandInput(
			actor.OwnerUserID, actor.LeaseRoundID, token,
		)
		if stagingErr != nil {
			return nil, stagingErr
		}
		if round.Resources == nil {
			cleanup()
			return nil, errors.New("runtime command physical round 缺少资源所有者")
		}
		round.Resources.Add(cleanup)
		environment[protocol.NexusCommandInputPathEnvName] = inputPath
		return environment, nil
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

func runtimeCommandBrokerURL(cfg config.Config) string {
	prefix := "/" + strings.Trim(strings.TrimSpace(cfg.APIPrefix), "/")
	if prefix == "/" {
		prefix = ""
	}
	return (&url.URL{
		Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Path: prefix + "/internal/runtime/command",
	}).String()
}

func newRuntimeCommandHandler(
	registry *runtimecommand.Registry,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if registry == nil || !runtimeConfigurationLoopbackRequest(request) {
			writeRuntimeCommandError(writer, http.StatusForbidden, "runtime command broker 只接受本机请求")
			return
		}
		actor, err := registry.Resolve(request.Header.Get(protocol.NexusCommandCapabilityHeader))
		if err != nil {
			writeRuntimeCommandError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		var command runtimecommand.Request
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRuntimeCommandRequestBytes))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&command); err != nil {
			writeRuntimeCommandError(writer, http.StatusBadRequest, "请求参数错误: "+err.Error())
			return
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeRuntimeCommandError(writer, http.StatusBadRequest, "请求只能包含一个 JSON 对象")
			return
		}
		var result any
		switch strings.ToLower(strings.TrimSpace(command.Domain)) {
		case runtimecommand.DomainAutomation:
			result, err = handleAutomationRuntimeCommand(request.Context(), automation, permissions, actor, command)
		case runtimecommand.DomainGoal:
			result, err = handleGoalRuntimeCommand(request.Context(), goals, actor, command)
		case runtimecommand.DomainExecution:
			result, err = handleExecutionRuntimeCommand(request.Context(), execution, actor, command)
		default:
			err = fmt.Errorf("未知 Nexus runtime command domain %q", command.Domain)
		}
		if err != nil {
			writeRuntimeCommandError(writer, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeRuntimeCommandJSON(writer, http.StatusOK, map[string]any{"success": true, "data": result})
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
) (any, error) {
	if svc == nil {
		return nil, errors.New("Execution command service 尚未装配")
	}
	roundContext := actor.Round.CommandContext
	// Goal create/retarget can advance exact authority during this physical
	// round. Execution must consume the same host-owned state instead of the
	// immutable launch snapshot, or Goal+WorkGraph will self-conflict.
	roundContext.GoalAuthority = actor.GoalMutationAuthority
	roundContext.ResponsibilityAuthority = actor.GoalResponsibilityState
	sctx, ok := resolveExecutionCommandContext(ctx, svc, roundContext)
	if !ok {
		return nil, errors.New("当前 round 没有有效的 Execution command identity")
	}
	sctx.CommandAttempts = actor.Round.Attempts
	operations := executionoperation.BuildAll(svc, sctx)
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
		result, err := operation.Invoke(ctx, command.Input, &runtimecommand.CallContext{RequestID: command.RequestID})
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

func writeRuntimeCommandError(writer http.ResponseWriter, status int, message string) {
	writeRuntimeCommandJSON(writer, status, map[string]any{
		"success": false, "error": map[string]any{"message": strings.TrimSpace(message)},
	})
}

func writeRuntimeCommandJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
