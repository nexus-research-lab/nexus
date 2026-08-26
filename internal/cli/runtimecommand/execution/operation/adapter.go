// INPUT: MCP 业务字段、宿主 CallContext 与 session-bound Context。
// OUTPUT: strict typed intent、最新 Execution snapshot、服务端 fencing/idempotency 与紧凑模型结果。
// POS: 所有 Execution operation 共享的可靠性适配层。
package operation

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

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
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

func loadReadableSnapshot(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if svc == nil {
		return nil, errors.New("execution orchestration service is nil")
	}
	if executionID = strings.TrimSpace(executionID); executionID != "" {
		return svc.ReadSnapshot(ctx, actor, executionID)
	}
	if executionID = strings.TrimSpace(actor.ExecutionID); executionID != "" {
		return svc.ReadSnapshot(ctx, actor, executionID)
	}
	return svc.ReadCurrent(ctx, actor)
}

func commandID(
	sctx contract.Context,
	callContext *runtimecommand.CallContext,
	operationName string,
	input map[string]any,
	snapshotRevision int64,
) (string, error) {
	if callContext != nil {
		if toolUseID := strings.TrimSpace(callContext.RequestID); toolUseID != "" {
			return toolUseID, nil
		}
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("canonicalize operation input: %w", err)
	}
	parts := []string{
		strings.TrimSpace(sctx.ScopeSessionKey),
		strings.TrimSpace(sctx.RuntimeSessionKey),
		strings.TrimSpace(sctx.RootRoundID),
		strings.TrimSpace(sctx.AgentRoundID),
		strings.TrimSpace(operationName),
		string(canonical),
		strconv.FormatInt(snapshotRevision, 10),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "execution-" + hex.EncodeToString(digest[:]), nil
}

func jsonResult(payload any) runtimecommand.Result {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return transportErrorResult(err)
	}
	var structured map[string]any
	if err := json.Unmarshal(encoded, &structured); err != nil {
		return transportErrorResult(err)
	}
	return runtimecommand.Result{
		Content: []map[string]any{{
			"type": "text",
			"text": string(encoded),
		}},
		StructuredContent: structured,
	}
}

func mutationResult(result orchestration.MutationResult) runtimecommand.Result {
	payload := executionMutationResult{
		Outcome:          result.Outcome,
		ReasonCode:       result.ReasonCode,
		Message:          result.Message,
		ExecutionID:      result.ExecutionID,
		SnapshotRevision: result.SnapshotRevision,
		ExecutionContext: result.ExecutionContext,
		ContextStatus:    result.ContextStatus,
		Changed:          result.Changed,
		NextActions:      result.NextActions,
		GoalConfirmation: result.GoalConfirmation,
	}
	return jsonResult(payload)
}

// executionMutationResult 是 command 模型面的唯一 mutation 投影。完整 Snapshot
// 继续留在 service result 供同进程协调、HTTP/UI 和测试使用；不能与已经由它
// 派生出的 execution_context 重复发送给模型。
type executionMutationResult struct {
	Outcome          orchestration.MutationOutcome        `json:"outcome"`
	ReasonCode       orchestration.ErrorCode              `json:"reason_code,omitempty"`
	Message          string                               `json:"message,omitempty"`
	ExecutionID      string                               `json:"execution_id,omitempty"`
	SnapshotRevision int64                                `json:"snapshot_revision,omitempty"`
	ExecutionContext string                               `json:"execution_context,omitempty"`
	ContextStatus    string                               `json:"context_status,omitempty"`
	Changed          []string                             `json:"changed,omitempty"`
	NextActions      []orchestration.NextAction           `json:"next_actions,omitempty"`
	GoalConfirmation orchestration.GoalConfirmationStatus `json:"goal_confirmation_status,omitempty"`
}

type executionRuntimeContextReader interface {
	RuntimeContext(
		context.Context,
		orchestration.ActorContext,
	) (string, error)
}

const (
	executionContextInlineLimit = 12 * 1024
)

func withFreshExecutionContext(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	result orchestration.MutationResult,
) orchestration.MutationResult {
	if result.Snapshot != nil {
		switch result.Snapshot.Execution.Status {
		case protocol.ExecutionStatusActive,
			protocol.ExecutionStatusWaiting,
			protocol.ExecutionStatusPaused:
			actor.ExecutionID = strings.TrimSpace(result.ExecutionID)
		default:
			// Terminal mutation results describe immutable history. Refresh the
			// session's current unmanaged/successor context instead of pinning
			// RuntimeContext to the execution that just ended.
			actor.ExecutionID = ""
			actor.WorkBinding = nil
			actor.ReviewBinding = nil
		}
	} else if executionID := strings.TrimSpace(result.ExecutionID); executionID != "" {
		actor.ExecutionID = executionID
	}
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
	rendered = compactRuntimeCommandContext(rendered)
	if rendered == "" {
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
		if action.Operation == "get_execution" {
			return result
		}
	}
	result.NextActions = append([]orchestration.NextAction{{
		Domain: "execution", Operation: "get_execution",
		Reason: "refresh the authoritative allowed actions before another orchestration mutation",
	}}, result.NextActions...)
	return result
}

func snapshotResult(
	ctx context.Context,
	svc contract.Service,
	actor orchestration.ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) runtimecommand.Result {
	payload := map[string]any{
		"execution_id":      nil,
		"snapshot_revision": nil,
		"execution_context": nil,
		"context_status":    "refresh_required",
	}
	if snapshot != nil {
		payload["execution_id"] = snapshot.Execution.ID
		payload["snapshot_revision"] = snapshot.Execution.Version
		switch snapshot.Execution.Status {
		case protocol.ExecutionStatusSuperseded:
			payload["outcome"] = orchestration.MutationSuperseded
			payload["reason_code"] = orchestration.ErrorCodeExecutionTerminal
			payload["execution_status"] = snapshot.Execution.Status
		case protocol.ExecutionStatusCompleted,
			protocol.ExecutionStatusFailed,
			protocol.ExecutionStatusCancelled:
			payload["outcome"] = orchestration.MutationRejected
			payload["reason_code"] = orchestration.ErrorCodeExecutionTerminal
			payload["execution_status"] = snapshot.Execution.Status
		}
	}
	if reader, ok := any(svc).(executionRuntimeContextReader); ok {
		if rendered, err := reader.RuntimeContext(ctx, actor); err == nil &&
			strings.TrimSpace(rendered) != "" {
			if rendered = compactRuntimeCommandContext(rendered); rendered != "" {
				payload["execution_context"] = rendered
				payload["context_status"] = "authoritative"
			}
		}
	}
	return jsonResult(payload)
}

// compactRuntimeCommandContext keeps the current authority/action contract
// inline while removing observed Runtime Graph history that is already
// available through the WorkGraph read model. Re-embedding that history after
// every mutation makes the result recursively grow until the runtime has to
// externalize even the small outcome/next_actions control envelope.
func compactRuntimeCommandContext(rendered string) string {
	rendered = strings.TrimSpace(removeExecutionContextElement(rendered, "runtime_facts"))
	if len(rendered) <= executionContextInlineLimit {
		return rendered
	}
	// The graph digest is useful orientation, but the actionable assigned,
	// ready and review sections below it are the required continuation state.
	rendered = strings.TrimSpace(removeExecutionContextElement(rendered, "graph_digest"))
	// Responsibility, review, action and blocker sections are authoritative.
	// They may exceed this transport-size preference, but must never be erased
	// or replaced with a fabricated refresh state. The structured command transports the one
	// structured wire as-is; only the optional observed graph facts above are
	// eligible for compaction.
	return rendered
}

func removeExecutionContextElement(rendered string, element string) string {
	startMarker := "<" + element
	start := strings.Index(rendered, startMarker)
	if start < 0 {
		return rendered
	}
	if start > 0 && rendered[start-1] == '\n' {
		start--
	}
	openingEndOffset := strings.Index(rendered[start:], ">")
	if openingEndOffset < 0 {
		return rendered
	}
	openingEnd := start + openingEndOffset
	if openingEnd > start && rendered[openingEnd-1] == '/' {
		return rendered[:start] + rendered[openingEnd+1:]
	}
	closeMarker := "</" + element + ">"
	closeOffset := strings.Index(rendered[openingEnd+1:], closeMarker)
	if closeOffset < 0 {
		return rendered
	}
	end := openingEnd + 1 + closeOffset + len(closeMarker)
	return rendered[:start] + rendered[end:]
}

func rejectedResult(message string, actions ...orchestration.NextAction) runtimecommand.Result {
	return mutationResult(orchestration.RejectedResult(nil, errors.New(message), actions))
}

// recoverableMutationRejection 把已经到达服务端、但被当前权威状态拒绝的
// stale/terminal command 留在业务 mutation 语义，避免把 retarget 竞态误报成
// command transport failure。owner/session 等安全错误仍保持 IsError。
func recoverableMutationRejection(err error) (runtimecommand.Result, bool) {
	var domainErr *orchestration.DomainError
	if !errors.As(err, &domainErr) {
		return runtimecommand.Result{}, false
	}
	switch domainErr.Code {
	case orchestration.ErrorCodeExecutionTerminal:
		return mutationResult(orchestration.SupersededResult(nil, domainErr)), true
	case orchestration.ErrorCodeStaleExecution:
		return mutationResult(orchestration.RejectedResult(nil, domainErr, nil)), true
	default:
		return runtimecommand.Result{}, false
	}
}

func transportErrorResult(err error) runtimecommand.Result {
	message := "execution command failed"
	if err != nil {
		message = err.Error()
	}
	return runtimecommand.Result{
		Content: []map[string]any{{
			"type": "text",
			"text": message,
		}},
		StructuredContent: map[string]any{
			"outcome": "rejected",
			"message": message,
		},
		IsError: true,
	}
}
