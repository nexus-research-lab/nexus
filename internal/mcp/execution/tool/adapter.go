// INPUT: MCP raw JSON、CallContext 与 session-bound ServerContext。
// OUTPUT: strict typed intent、最新 Execution snapshot、服务端 fencing/idempotency 与双投影结果。
// POS: 所有 execution tool 共享的可靠性适配层。
package tool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	return nil
}

func loadSnapshot(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if svc == nil {
		return nil, errors.New("execution orchestration service is nil")
	}
	if executionID = strings.TrimSpace(executionID); executionID != "" {
		return svc.GetSnapshot(ctx, actor, executionID)
	}
	if executionID = strings.TrimSpace(actor.ExecutionID); executionID != "" {
		return svc.GetSnapshot(ctx, actor, executionID)
	}
	return svc.GetCurrent(ctx, actor)
}

func commandID(
	sctx contract.ServerContext,
	callContext *sdktool.CallContext,
	toolName string,
	input map[string]any,
	snapshotRevision int64,
) (string, error) {
	if callContext != nil {
		if toolUseID := strings.TrimSpace(callContext.ToolUseID); toolUseID != "" {
			return toolUseID, nil
		}
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("canonicalize tool input: %w", err)
	}
	parts := []string{
		strings.TrimSpace(sctx.ScopeSessionKey),
		strings.TrimSpace(sctx.RuntimeSessionKey),
		strings.TrimSpace(sctx.RootRoundID),
		strings.TrimSpace(sctx.AgentRoundID),
		strings.TrimSpace(toolName),
		string(canonical),
		strconv.FormatInt(snapshotRevision, 10),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "execution-" + hex.EncodeToString(digest[:]), nil
}

func jsonResult(payload any) sdktool.ToolResult {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return transportErrorResult(err)
	}
	var structured map[string]any
	if err := json.Unmarshal(encoded, &structured); err != nil {
		return transportErrorResult(err)
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": string(encoded),
		}},
		StructuredContent: structured,
	}
}

func mutationResult(result orchestration.MutationResult) sdktool.ToolResult {
	return jsonResult(result)
}

type executionRuntimeContextReader interface {
	RuntimeContext(
		context.Context,
		orchestration.ActorContext,
	) (string, error)
}

func withFreshExecutionContext(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	result orchestration.MutationResult,
) orchestration.MutationResult {
	reader, ok := any(svc).(executionRuntimeContextReader)
	if !ok {
		result.ContextStatus = "refresh_required"
		return withGetExecutionRecovery(result)
	}
	rendered, err := reader.RuntimeContext(ctx, actor)
	if err != nil || strings.TrimSpace(rendered) == "" {
		result.ContextStatus = "refresh_required"
		return withGetExecutionRecovery(result)
	}
	result.ExecutionContext = rendered
	result.ContextStatus = "authoritative"
	return result
}

func withGetExecutionRecovery(
	result orchestration.MutationResult,
) orchestration.MutationResult {
	for _, action := range result.NextActions {
		if action.Tool == "get_execution" {
			return result
		}
	}
	result.NextActions = append([]orchestration.NextAction{{
		Tool:   "get_execution",
		Reason: "refresh the authoritative allowed actions before another orchestration mutation",
	}}, result.NextActions...)
	return result
}

func snapshotResult(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) sdktool.ToolResult {
	payload := map[string]any{
		"execution_id":      nil,
		"snapshot_revision": nil,
		"execution_context": nil,
		"context_status":    "refresh_required",
		"snapshot":          snapshot,
	}
	if snapshot != nil {
		payload["execution_id"] = snapshot.Execution.ID
		payload["snapshot_revision"] = snapshot.Execution.Version
	}
	if reader, ok := any(svc).(executionRuntimeContextReader); ok {
		if rendered, err := reader.RuntimeContext(ctx, actor); err == nil &&
			strings.TrimSpace(rendered) != "" {
			payload["execution_context"] = rendered
			payload["context_status"] = "authoritative"
		}
	}
	return jsonResult(payload)
}

func rejectedResult(message string, actions ...orchestration.NextAction) sdktool.ToolResult {
	return mutationResult(orchestration.RejectedResult(nil, errors.New(message), actions))
}

func transportErrorResult(err error) sdktool.ToolResult {
	message := "execution tool failed"
	if err != nil {
		message = err.Error()
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": message,
		}},
		IsError: true,
	}
}
