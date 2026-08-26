// INPUT: 可信 runtime round、Goal/Execution/Automation services、exact WorkGraph preview binding 与 runtime permission context。
// OUTPUT: 按领域读写边界拆分的 round-scoped Nexus MCP tools、语义结果与 typed mutation receipt。
// POS: Agent-facing Nexus MCP adapter；工具 schema 就是模型合同，身份、责任、preview、Plan Mode 与 Automation 真人确认均由宿主固定。
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
		definitions := runtimeCommandMCPTools(
			ctx,
			automation,
			goals,
			execution,
			permissions,
			actor,
			workflowServices...,
		)
		if len(definitions) == 0 {
			return nil, nil
		}
		server := sdktool.NewSimpleSDKMCPServer(
			protocol.NexusMCPServerName,
			"1.0.0",
			definitions,
		)
		return map[string]sdkmcp.ServerConfig{
			protocol.NexusMCPServerName: sdkmcp.SDKServerConfig{
				Name: protocol.NexusMCPServerName, Instance: server,
			},
		}, nil
	}
}

func runtimeCommandMCPTools(
	ctx context.Context,
	automation *automationsvc.Service,
	goals goalcontract.Service,
	execution executioncontract.Service,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
	workflowServices ...executioncontract.WorkflowService,
) []sdktool.Tool {
	definitions := make([]sdktool.Tool, 0, len(protocol.NexusManagedToolNames()))
	goalOperations, _ := goalRuntimeOperations(goals, actor)
	definitions = appendSemanticRuntimeTools(
		definitions,
		protocol.NexusGoalReadToolName,
		protocol.NexusGoalWriteToolName,
		"读取当前会话的权威 Goal。",
		"创建、重定向、审计或更新当前 Goal。",
		goalOperations,
		func(callCtx context.Context, operation string, input map[string]any, write bool, requestID string) (any, error) {
			fresh, err := goalRuntimeOperations(goals, actor)
			if err != nil {
				return nil, err
			}
			return invokeRuntimeSemanticOperation(callCtx, actor, runtimecommand.DomainGoal, fresh, operation, input, write, requestID)
		},
	)

	executionOperations, _, _ := executionRuntimeOperations(ctx, execution, actor, workflowServices...)
	definitions = appendSemanticRuntimeTools(
		definitions,
		protocol.NexusExecutionReadToolName,
		protocol.NexusExecutionWriteToolName,
		"读取当前 Execution、WorkGraph 与草图状态。",
		"规划、分派、提交、审查或修改 Execution 与 WorkGraph。",
		executionOperations,
		func(callCtx context.Context, operation string, input map[string]any, write bool, requestID string) (any, error) {
			fresh, _, err := executionRuntimeOperations(callCtx, execution, actor, workflowServices...)
			if err != nil {
				return nil, err
			}
			return invokeRuntimeSemanticOperation(callCtx, actor, runtimecommand.DomainExecution, fresh, operation, input, write, requestID)
		},
	)

	if automation == nil {
		return definitions
	}
	contract, err := automation.RuntimeCommandContract(ctx, actor)
	if err != nil {
		return definitions
	}
	definitions = appendAutomationRuntimeTools(definitions, automation, permissions, actor, contract)
	return definitions
}

type semanticRuntimeToolHandler func(context.Context, string, map[string]any, bool, string) (any, error)

func appendSemanticRuntimeTools(
	definitions []sdktool.Tool,
	readName string,
	writeName string,
	readDescription string,
	writeDescription string,
	operations []runtimecommand.Operation,
	handler semanticRuntimeToolHandler,
) []sdktool.Tool {
	reads, writes := splitRuntimeOperations(operations)
	if len(reads) > 0 {
		definitions = append(definitions, semanticRuntimeTool(
			readName,
			readDescription,
			reads,
			true,
			handler,
		))
	}
	if len(writes) > 0 {
		definitions = append(definitions, semanticRuntimeTool(
			writeName,
			writeDescription,
			writes,
			false,
			handler,
		))
	}
	return definitions
}

func semanticRuntimeTool(
	name string,
	description string,
	operations []runtimecommand.Operation,
	readOnly bool,
	handler semanticRuntimeToolHandler,
) sdktool.Tool {
	schema := runtimeOperationToolSchema(operations)
	annotationReadOnly := readOnly
	for _, operation := range operations {
		annotationReadOnly = annotationReadOnly && runtimeOperationReadOnly(operation)
	}
	fixedOperation := ""
	if name == protocol.NexusGoalReadToolName && len(operations) == 1 &&
		operations[0].Name == "get_goal" {
		fixedOperation = operations[0].Name
		schema = operations[0].InputSchema
	}
	return sdktool.Tool{
		Name:        name,
		Description: description,
		SearchHint:  runtimeOperationSearchHint(operations),
		AlwaysLoad:  true,
		InputSchema: schema,
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly: annotationReadOnly, ReadOnlyHint: annotationReadOnly, Destructive: !readOnly,
		},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			requestID := ""
			if !readOnly {
				requestID = runtimeMCPRequestID(callContext)
				if !runtimecommand.ValidRequestID(requestID) {
					return runtimeCommandMCPError(errors.New("runtime 未提供有效的 tool_use_id；请升级 Agent SDK Bridge")), nil
				}
			}
			operation := stringValue(input["operation"])
			if fixedOperation != "" {
				operation = fixedOperation
			}
			result, err := handler(
				ctx,
				operation,
				runtimeOperationInput(input),
				!readOnly,
				requestID,
			)
			if err != nil {
				return runtimeCommandMCPError(err), nil
			}
			return runtimeCommandMCPResult(result), nil
		},
	}
}

func splitRuntimeOperations(operations []runtimecommand.Operation) ([]runtimecommand.Operation, []runtimecommand.Operation) {
	reads := make([]runtimecommand.Operation, 0, len(operations))
	writes := make([]runtimecommand.Operation, 0, len(operations))
	for _, operation := range operations {
		if runtimeOperationReadGroup(operation) {
			reads = append(reads, operation)
		} else {
			writes = append(writes, operation)
		}
	}
	return reads, writes
}

func runtimeOperationReadOnly(operation runtimecommand.Operation) bool {
	return operation.ReadOnly || operation.Annotations != nil &&
		(operation.Annotations.ReadOnly || operation.Annotations.ReadOnlyHint)
}

func runtimeOperationReadGroup(operation runtimecommand.Operation) bool {
	return operation.Name == "get_execution" || runtimeOperationReadOnly(operation)
}

func runtimeOperationToolSchema(operations []runtimecommand.Operation) map[string]any {
	properties := map[string]any{}
	names := make([]string, 0, len(operations))
	alternatives := make([]any, 0, len(operations))
	for _, operation := range operations {
		name := strings.TrimSpace(operation.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		branchProperties := map[string]any{
			"operation": map[string]any{
				"type": "string", "enum": []string{name}, "description": operation.Description,
			},
		}
		required := []string{"operation"}
		if schemaProperties, ok := operation.InputSchema["properties"].(map[string]any); ok {
			for field, schema := range schemaProperties {
				branchProperties[field] = schema
				if _, exists := properties[field]; !exists {
					properties[field] = map[string]any{}
				}
			}
		}
		required = append(required, schemaStringSlice(operation.InputSchema["required"])...)
		alternatives = append(alternatives, map[string]any{
			"properties":           branchProperties,
			"required":             required,
			"additionalProperties": false,
		})
	}
	properties["operation"] = map[string]any{"type": "string", "enum": names}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"anyOf":                alternatives,
		"required":             []string{"operation"},
		"additionalProperties": false,
	}
}

func runtimeOperationSearchHint(operations []runtimecommand.Operation) string {
	parts := make([]string, 0, len(operations))
	for _, operation := range operations {
		parts = append(parts, strings.TrimSpace(operation.Name+" "+operation.SearchHint))
	}
	return strings.Join(parts, " ")
}

func runtimeOperationInput(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		if key != "operation" {
			result[key] = value
		}
	}
	return result
}

func runtimeMCPRequestID(callContext *sdktool.CallContext) string {
	if callContext == nil {
		return ""
	}
	return strings.TrimSpace(callContext.ToolUseID)
}

func schemaStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func appendAutomationRuntimeTools(
	definitions []sdktool.Tool,
	svc *automationsvc.Service,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
	contract automationdomain.AutomationCommandContract,
) []sdktool.Tool {
	if len(contract.QueryOperations) > 0 {
		definitions = append(definitions, automationRuntimeTool(
			protocol.NexusAutomationReadToolName,
			"查询当前 Actor 可见的自动化任务、运行记录与报告。",
			runtimecommand.ActionInspect,
			contract.QueryOperations,
			contract,
			svc,
			permissions,
			actor,
		))
	}
	if len(contract.MutationOperations) == 0 {
		return definitions
	}
	definitions = append(definitions,
		automationRuntimeTool(
			protocol.NexusAutomationPlanToolName,
			"生成自动化变更计划；不写入状态。",
			runtimecommand.ActionPlan,
			contract.MutationOperations,
			contract,
			svc,
			permissions,
			actor,
		),
		automationRuntimeTool(
			protocol.NexusAutomationApplyToolName,
			"按 plan 返回的 revision 与 digest 应用自动化变更，并在需要时请求真人确认。",
			runtimecommand.ActionApply,
			contract.MutationOperations,
			contract,
			svc,
			permissions,
			actor,
		),
	)
	return definitions
}

func automationRuntimeTool(
	name string,
	description string,
	action string,
	operations []string,
	contract automationdomain.AutomationCommandContract,
	svc *automationsvc.Service,
	permissions *permissionctx.Context,
	actor runtimecommand.Actor,
) sdktool.Tool {
	schema := automationRuntimeToolSchema(contract, operations, action == runtimecommand.ActionApply)
	readOnly := action != runtimecommand.ActionApply
	return sdktool.Tool{
		Name:        name,
		Description: description,
		SearchHint:  "automation scheduled task reminder schedule run history report 定时任务 提醒 计划 执行 历史 报告",
		AlwaysLoad:  true,
		InputSchema: schema,
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly: readOnly, ReadOnlyHint: readOnly, Destructive: !readOnly,
		},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			if err := runtimecommand.ValidateInput(schema, input); err != nil {
				return runtimeCommandMCPError(err), nil
			}
			requestID := ""
			if action == runtimecommand.ActionApply {
				requestID = runtimeMCPRequestID(callContext)
				if !runtimecommand.ValidRequestID(requestID) {
					return runtimeCommandMCPError(errors.New("runtime 未提供有效的 tool_use_id；请升级 Agent SDK Bridge")), nil
				}
			}
			operationInput := runtimeOperationInput(input)
			delete(operationInput, "expected_revision")
			delete(operationInput, "plan_digest")
			result, err := handleAutomationRuntimeCommand(ctx, svc, permissions, actor, runtimecommand.Request{
				Domain:           runtimecommand.DomainAutomation,
				Action:           action,
				Operation:        stringValue(input["operation"]),
				Input:            operationInput,
				RequestID:        requestID,
				ExpectedRevision: stringValue(input["expected_revision"]),
				PlanDigest:       stringValue(input["plan_digest"]),
			})
			if err != nil {
				return runtimeCommandMCPError(err), nil
			}
			return runtimeCommandMCPResult(result), nil
		},
	}
}

func automationRuntimeToolSchema(
	contract automationdomain.AutomationCommandContract,
	operations []string,
	apply bool,
) map[string]any {
	allProperties := automationRuntimeInputProperties()
	expectedRevisionSchema := map[string]any{
		"type": "string", "description": "Required. current_revision returned by automation_plan.",
	}
	planDigestSchema := map[string]any{
		"type": "string", "description": "Required. plan_digest returned by automation_plan.",
	}
	properties := map[string]any{}
	names := make([]string, 0, len(operations))
	alternatives := make([]any, 0, len(operations))
	for _, name := range operations {
		operationContract, ok := contract.Operations[name]
		if !ok {
			continue
		}
		names = append(names, name)
		description := strings.Join(operationContract.Notes, " ")
		branchProperties := map[string]any{
			"operation": map[string]any{
				"type": "string", "enum": []string{name}, "description": description,
			},
		}
		fields := append(append([]string{}, operationContract.Required...), operationContract.Optional...)
		for _, field := range fields {
			if schema, exists := allProperties[field]; exists {
				branchProperties[field] = schema
				properties[field] = schema
			}
		}
		required := append([]string{"operation"}, operationContract.Required...)
		if apply {
			branchProperties["expected_revision"] = expectedRevisionSchema
			branchProperties["plan_digest"] = planDigestSchema
			required = append(required, "expected_revision", "plan_digest")
		}
		alternatives = append(alternatives, map[string]any{
			"properties":           branchProperties,
			"required":             required,
			"additionalProperties": false,
		})
	}
	properties["operation"] = map[string]any{"type": "string", "enum": names}
	if apply {
		properties["expected_revision"] = expectedRevisionSchema
		properties["plan_digest"] = planDigestSchema
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"anyOf":                alternatives,
		"required":             []string{"operation"},
		"additionalProperties": false,
	}
}

func automationRuntimeInputProperties() map[string]any {
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	integer := func(description string) map[string]any {
		return map[string]any{"type": "integer", "description": description}
	}
	boolean := func(description string) map[string]any {
		return map[string]any{"type": "boolean", "description": description}
	}
	return map[string]any{
		"job_id":             text("Exact scheduled task ID."),
		"run_id":             text("Exact task run ID."),
		"query":              text("Task search query used only when an exact ID is unavailable."),
		"agent_id":           text("Target Agent ID when the current Actor has cross-Agent authority."),
		"name":               text("Task display name."),
		"instruction":        text("Complete task instruction."),
		"instruction_append": text("Text appended to the existing instruction."),
		"schedule": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":           map[string]any{"type": "string", "enum": []string{"single", "daily", "interval", "cron"}},
				"run_at":         text("One-time RFC3339 execution time."),
				"daily_time":     text("Daily local time."),
				"weekdays":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"interval_value": integer("Positive interval value."),
				"interval_unit":  text("Interval unit."),
				"expr":           text("Cron expression."),
				"timezone":       text("IANA timezone."),
			},
			"required":             []string{"kind"},
			"additionalProperties": false,
		},
		"execution_kind":             text("Execution kind."),
		"permission_mode":            text("Runtime permission mode."),
		"context_mode":               text("Task context mode."),
		"deliver_result":             boolean("Whether to deliver the task result."),
		"overlap_policy":             text("Concurrent run policy."),
		"expires_at":                 text("Optional RFC3339 expiration time."),
		"clear_expires_at":           boolean("Clear the current expiration time."),
		"enabled":                    boolean("Whether the task is enabled."),
		"cancel_active_run":          boolean("Cancel an active run when required by the operation."),
		"execution_mode":             text("Host-authorized execution route."),
		"reply_mode":                 text("Host-authorized reply route."),
		"selected_session_key":       text("Host-authorized selected execution Session."),
		"named_session_key":          text("Host-authorized named execution Session."),
		"selected_reply_session_key": text("Host-authorized selected reply Session."),
		"reply_session_key":          text("Host-authorized reply Session."),
		"include_active":             boolean("Include active tasks."),
		"include_deleted":            boolean("Include deleted task history."),
		"limit":                      integer("Maximum task count."),
		"run_limit":                  integer("Maximum run count."),
		"event_limit":                integer("Maximum event count."),
		"date":                       text("Report date."),
		"timezone":                   text("IANA timezone."),
		"every_seconds":              integer("Heartbeat interval in seconds."),
		"target_mode":                text("Heartbeat delivery target mode."),
		"ack_max_chars":              integer("Heartbeat acknowledgement limit."),
		"mode":                       text("Wake mode."),
		"text":                       text("Optional wake text."),
	}
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
	message := "Nexus 工具调用失败"
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
	operations, err := goalRuntimeOperations(svc, actor)
	if err != nil {
		return nil, err
	}
	return handleSemanticRuntimeCommand(
		ctx, actor, runtimecommand.DomainGoal, "get_goal", operations, command,
	)
}

func goalRuntimeOperations(
	svc goalcontract.Service,
	actor runtimecommand.Actor,
) ([]runtimecommand.Operation, error) {
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
	return goaloperation.BuildAll(svc, sctx), nil
}

func handleExecutionRuntimeCommand(
	ctx context.Context,
	svc executioncontract.Service,
	actor runtimecommand.Actor,
	command runtimecommand.Request,
	workflowServices ...executioncontract.WorkflowService,
) (any, error) {
	operations, inspectOperation, err := executionRuntimeOperations(
		ctx, svc, actor, workflowServices...,
	)
	if err != nil {
		return nil, err
	}
	return handleSemanticRuntimeCommand(
		ctx,
		actor,
		runtimecommand.DomainExecution,
		inspectOperation,
		operations,
		command,
	)
}

func executionRuntimeOperations(
	ctx context.Context,
	svc executioncontract.Service,
	actor runtimecommand.Actor,
	workflowServices ...executioncontract.WorkflowService,
) ([]runtimecommand.Operation, string, error) {
	if actor.SourceContextType == protocol.SessionPurposeWorkGraphDistillation {
		if len(workflowServices) == 0 || workflowServices[0] == nil {
			return nil, "", errors.New("WorkGraph distillation command service 尚未装配")
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
		return executionoperation.BuildWorkGraphDistillation(workflowServices[0], sctx), "", nil
	}
	if actor.SourceContextType == protocol.SessionPurposeWorkGraphEditor {
		if len(workflowServices) == 0 || workflowServices[0] == nil {
			return nil, "", errors.New("WorkGraph editor command service 尚未装配")
		}
		editorService, ok := workflowServices[0].(executioncontract.WorkflowEditorService)
		if !ok || !editorService.RuntimeEditorActive(actor.OwnerUserID, actor.SessionKey) {
			return nil, "", errors.New("当前 round 没有有效的 WorkGraph editor command identity")
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
		return executionoperation.BuildWorkGraphEditor(editorService, sctx), "", nil
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
			return authoringOperations, "", nil
		}
		return nil, "", errors.New("Execution command service 尚未装配")
	}
	// Goal create/retarget can advance exact authority during this physical
	// round. Execution must consume the same host-owned state instead of the
	// immutable launch snapshot, or Goal+WorkGraph will self-conflict.
	roundContext.GoalAuthority = actor.GoalMutationAuthority
	roundContext.ResponsibilityAuthority = actor.GoalResponsibilityState
	sctx, ok := resolveExecutionCommandContext(ctx, svc, roundContext)
	if !ok {
		if len(authoringOperations) > 0 {
			return authoringOperations, "", nil
		}
		return nil, "", errors.New("当前 round 没有有效的 Execution command identity")
	}
	sctx.CommandAttempts = actor.Round.Attempts
	operations := executionoperation.BuildAll(svc, sctx, workflowServices...)
	return operations, "get_execution", nil
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
		if _, ok := runtimecommand.FindOperation(operations, inspectOperation); !ok {
			return nil, fmt.Errorf("%s inspect operation 未装配", domain)
		}
		return invokeRuntimeSemanticOperation(
			ctx, actor, domain, operations, inspectOperation, command.Input, false, "",
		)
	case runtimecommand.ActionInvoke:
		operation, ok := runtimecommand.FindOperation(operations, command.Operation)
		if !ok || operation.Name == inspectOperation {
			return nil, fmt.Errorf("未知或不可 invoke 的 %s operation %q", domain, command.Operation)
		}
		write := !runtimeOperationReadOnly(operation)
		if write && !runtimecommand.ValidRequestID(command.RequestID) {
			return nil, errors.New("invoke request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
		}
		return invokeRuntimeSemanticOperation(
			ctx, actor, domain, operations, operation.Name, command.Input, write, command.RequestID,
		)
	default:
		return nil, fmt.Errorf("%s 只支持 contract、inspect、invoke", domain)
	}
}

func invokeRuntimeSemanticOperation(
	ctx context.Context,
	actor runtimecommand.Actor,
	domain string,
	operations []runtimecommand.Operation,
	operationName string,
	input map[string]any,
	write bool,
	requestID string,
) (any, error) {
	operation, ok := runtimecommand.FindOperation(operations, operationName)
	if !ok || runtimeOperationReadGroup(operation) == write {
		kind := "read"
		if write {
			kind = "write"
		}
		return nil, fmt.Errorf("未知或不可 %s 的 %s operation %q", kind, domain, operationName)
	}
	result, err := operation.Invoke(ctx, input, &runtimecommand.CallContext{
		RequestID: requestID,
		SessionID: actor.Round.CommandContext.CurrentSDKSessionID(),
	})
	if write && err == nil {
		recordRuntimeSemanticReceipt(actor, requestID, domain, operation.Name, input, result)
	}
	return result, err
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
