// INPUT: 宿主绑定的 physical-round Actor 与 Goal/Execution/Automation 领域分发函数。
// OUTPUT: 单一 nexus.command MCP 工具、动态 contract 调用结果与宿主可信 typed receipt。
// POS: 模型工具协议与 Nexus 领域 command adapter 之间的唯一 MCP 边界。
package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

const ToolName = "command"

// Handler 接收已经通过 MCP envelope 校验的 command 请求。
type Handler func(context.Context, Request) (any, error)

// NewTool 构建统一 nexus server 中的 command 工具定义。
func NewTool(handler Handler) sdktool.Tool {
	inputSchema := inputSchema()
	return sdktool.Tool{
		Name: ToolName,
		Description: "调用 Nexus 托管的 Goal、Execution 或 Automation 命令。" +
			"先用 contract 获取操作目录和精确输入 schema，再用 inspect/invoke 或 plan/apply 执行。",
		SearchHint:  "nexus goal execution automation contract inspect invoke plan apply 目标 执行 自动化",
		AlwaysLoad:  true,
		InputSchema: inputSchema,
		Annotations: &sdktool.ToolAnnotations{Destructive: true},
		Handler: func(ctx context.Context, input map[string]any) (sdktool.ToolResult, error) {
			if err := ValidateInput(inputSchema, input); err != nil {
				return errorResult(err), nil
			}
			if handler == nil {
				return errorResult(errors.New("Nexus command handler 尚未装配")), nil
			}
			result, err := handler(ctx, requestFromInput(input))
			if err != nil {
				return errorResult(err), nil
			}
			return toolResult(result), nil
		},
	}
}

func inputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"domain": map[string]any{
				"type": "string", "enum": []string{DomainAutomation, DomainGoal, DomainExecution},
			},
			"action": map[string]any{
				"type": "string", "enum": []string{
					ActionContract, ActionInspect, ActionInvoke, ActionPlan, ActionApply,
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

func requestFromInput(input map[string]any) Request {
	request := Request{
		Domain:           stringValue(input["domain"]),
		Action:           stringValue(input["action"]),
		Operation:        stringValue(input["operation"]),
		RequestID:        stringValue(input["request_id"]),
		ExpectedRevision: stringValue(input["expected_revision"]),
		PlanDigest:       stringValue(input["plan_digest"]),
	}
	request.Input, _ = input["input"].(map[string]any)
	return request
}

func toolResult(value any) sdktool.ToolResult {
	if result, ok := value.(Result); ok {
		return sdktool.ToolResult{
			Content: result.Content, StructuredContent: result.StructuredContent, IsError: result.IsError,
		}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return errorResult(err)
	}
	structured := map[string]any{}
	if value != nil {
		if err = json.Unmarshal(payload, &structured); err != nil {
			return errorResult(err)
		}
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": string(payload)}},
		StructuredContent: structured,
	}
}

func errorResult(err error) sdktool.ToolResult {
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

// HandleSemantic 执行 contract/inspect/invoke，并为成功的 mutation 记录可信回执。
func HandleSemantic(
	ctx context.Context,
	actor Actor,
	domain string,
	inspectOperation string,
	operations []Operation,
	request Request,
) (any, error) {
	action := strings.ToLower(strings.TrimSpace(request.Action))
	switch action {
	case ActionContract:
		return BuildContract(domain, inspectOperation, request.Operation, operations)
	case ActionInspect:
		if strings.TrimSpace(request.Operation) != "" {
			return nil, errors.New("inspect operation 由领域固定，不能覆盖")
		}
		operation, ok := FindOperation(operations, inspectOperation)
		if !ok {
			return nil, fmt.Errorf("%s inspect operation 未装配", domain)
		}
		return operation.Invoke(ctx, request.Input, nil)
	case ActionInvoke:
		if !ValidRequestID(request.RequestID) {
			return nil, errors.New("invoke request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
		}
		operation, ok := FindOperation(operations, request.Operation)
		if !ok || operation.Name == inspectOperation {
			return nil, fmt.Errorf("未知或不可 invoke 的 %s operation %q", domain, request.Operation)
		}
		result, err := operation.Invoke(ctx, request.Input, &CallContext{
			RequestID: request.RequestID,
			SessionID: actor.Round.CommandContext.CurrentSDKSessionID(),
		})
		if err == nil {
			recordReceipt(actor, request.RequestID, domain, operation.Name, request.Input, result)
		}
		return result, err
	default:
		return nil, fmt.Errorf("%s 只支持 contract、inspect、invoke", domain)
	}
}

func recordReceipt(
	actor Actor,
	requestID string,
	domain string,
	operation string,
	input map[string]any,
	result Result,
) {
	if result.IsError || actor.Round.CommandReceipts == nil {
		return
	}
	receipt := nexusmcp.CommandReceipt{
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
	if state := actor.Round.CommandContext.ResponsibilityAuthority; state != nil {
		if responsibility, loaded := state.Load(); loaded {
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
	if operation == GoalOperationUpdate && receipt.GoalStatus == "" {
		receipt.GoalStatus = stringValue(input["status"])
	}
	actor.Round.CommandReceipts.Record(receipt)
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
