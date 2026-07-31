// INPUT: 领域 command 的应用结果、拒绝原因与最新 snapshot。
// OUTPUT: 所有 execution MCP mutation 共用的可恢复结果 envelope。
// POS: 服务状态机到模型工具结果的稳定语义边界。
package orchestration

import (
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MutationOutcome 表示 command 是否改变了权威状态。
type MutationOutcome string

const (
	MutationApplied  MutationOutcome = "applied"
	MutationNoOp     MutationOutcome = "no_op"
	MutationRejected MutationOutcome = "rejected"
)

// NextAction 是基于最新 snapshot 的有序、非授权性恢复建议。
type NextAction struct {
	Tool       string `json:"tool"`
	WorkItemID string `json:"work_item_id,omitempty"`
	LogicalKey string `json:"logical_key,omitempty"`
	Reason     string `json:"reason"`
}

// MutationResult 是模型可见 execution mutation 的统一结果。
type MutationResult struct {
	Outcome          MutationOutcome             `json:"outcome"`
	ReasonCode       ErrorCode                   `json:"reason_code,omitempty"`
	Message          string                      `json:"message,omitempty"`
	ExecutionID      string                      `json:"execution_id,omitempty"`
	SnapshotRevision int64                       `json:"snapshot_revision,omitempty"`
	ExecutionContext string                      `json:"execution_context,omitempty"`
	ContextStatus    string                      `json:"context_status,omitempty"`
	Changed          []string                    `json:"changed,omitempty"`
	NextActions      []NextAction                `json:"next_actions,omitempty"`
	Snapshot         *protocol.ExecutionSnapshot `json:"snapshot,omitempty"`
}

// AppliedResult 生成成功 mutation 的稳定 envelope。
func AppliedResult(
	snapshot *protocol.ExecutionSnapshot,
	changed []string,
	nextActions []NextAction,
) MutationResult {
	result := mutationResultFromSnapshot(snapshot)
	result.Outcome = MutationApplied
	result.Changed = normalizeResultStrings(changed)
	result.NextActions = normalizeNextActions(nextActions)
	return result
}

// NoOpResult 表示重复 command 或已经满足的幂等结果。
func NoOpResult(snapshot *protocol.ExecutionSnapshot, message string) MutationResult {
	result := mutationResultFromSnapshot(snapshot)
	result.Outcome = MutationNoOp
	result.Message = strings.TrimSpace(message)
	return result
}

// RejectedResult 把 DomainError 投影成模型可恢复的稳定拒绝。
func RejectedResult(
	snapshot *protocol.ExecutionSnapshot,
	err error,
	nextActions []NextAction,
) MutationResult {
	result := mutationResultFromSnapshot(snapshot)
	result.Outcome = MutationRejected
	result.NextActions = normalizeNextActions(nextActions)
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		result.ReasonCode = domainErr.Code
		result.Message = domainErr.Message
		return result
	}
	result.ReasonCode = ErrorCodeInvalidInput
	if err != nil {
		result.Message = strings.TrimSpace(err.Error())
	}
	return result
}

func mutationResultFromSnapshot(snapshot *protocol.ExecutionSnapshot) MutationResult {
	result := MutationResult{Snapshot: snapshot}
	if snapshot == nil {
		return result
	}
	result.ExecutionID = snapshot.Execution.ID
	result.SnapshotRevision = snapshot.Execution.Version
	return result
}

func normalizeResultStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func normalizeNextActions(actions []NextAction) []NextAction {
	result := make([]NextAction, 0, len(actions))
	for _, action := range actions {
		action.Tool = strings.TrimSpace(action.Tool)
		action.WorkItemID = strings.TrimSpace(action.WorkItemID)
		action.LogicalKey = strings.TrimSpace(action.LogicalKey)
		action.Reason = strings.TrimSpace(action.Reason)
		if action.Tool == "" || action.Reason == "" {
			continue
		}
		result = append(result, action)
	}
	return result
}
