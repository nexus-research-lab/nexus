// INPUT: 领域 command 的应用结果、拒绝原因与最新 snapshot。
// OUTPUT: 所有 execution MCP mutation 共用的可恢复结果 envelope 与仅进程内使用的 confirmed Goal authority receipt。
// POS: 服务状态机到模型工具结果的稳定语义边界。
package orchestration

import (
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MutationOutcome 复用跨消息、UI 与 loop guard 的协议语义。
type MutationOutcome = protocol.MutationResultOutcome

const (
	MutationApplied    = protocol.MutationResultApplied
	MutationNoOp       = protocol.MutationResultNoOp
	MutationRejected   = protocol.MutationResultRejected
	MutationSuperseded = protocol.MutationResultSuperseded
)

// NextAction 是基于最新 snapshot 的有序、非授权性恢复建议。
type NextAction struct {
	Tool       string `json:"tool"`
	WorkItemID string `json:"work_item_id,omitempty"`
	LogicalKey string `json:"logical_key,omitempty"`
	Reason     string `json:"reason"`
}

// GoalAuthorityReceipt proves that the Goal-side reverse binding and the
// Execution-side forward binding are both durable for this exact revision.
// It is an in-process capability receipt and is never projected to the model.
type GoalAuthorityReceipt struct {
	GoalID            string
	ObjectiveRevision int64
	ExecutionID       string
}

// WorkBindingReceipt 是 Room host 在持久化 self Assignment 后签发的进程内
// capability receipt；Clear 表示责任已完成并回到同轮 coordination。
// 两者都不投影给模型，DM 也不会消费该 receipt。
type WorkBindingReceipt struct {
	Binding *protocol.ExecutionWorkBinding
	Clear   bool
}

// ResponsibilityAuthorityReceipt 是 durable mutation 已经完成后签发的宿主内
// capability 转场。ExecutionID 绑定同一物理 round 的当前 coordination lane，
// 并原子撤销先前 Work/Review lane；它不投影给模型。
type ResponsibilityAuthorityReceipt struct {
	ExecutionID string
}

// GoalConfirmationStatus is projected to the model only when a mutation
// crosses the Execution/Goal durable confirmation boundary.
type GoalConfirmationStatus string

const (
	GoalConfirmationPending   GoalConfirmationStatus = "pending"
	GoalConfirmationConfirmed GoalConfirmationStatus = "confirmed"
)

// MutationResult 是 service 内部的统一 mutation 结果。Snapshot 支持同进程协调
// 与非模型消费者；MCP adapter 必须只投影紧凑字段，不能把它与
// ExecutionContext 重复发送给模型。
type MutationResult struct {
	Outcome                 MutationOutcome                 `json:"outcome"`
	ReasonCode              ErrorCode                       `json:"reason_code,omitempty"`
	Message                 string                          `json:"message,omitempty"`
	ExecutionID             string                          `json:"execution_id,omitempty"`
	SnapshotRevision        int64                           `json:"snapshot_revision,omitempty"`
	ExecutionContext        string                          `json:"execution_context,omitempty"`
	ContextStatus           string                          `json:"context_status,omitempty"`
	Changed                 []string                        `json:"changed,omitempty"`
	NextActions             []NextAction                    `json:"next_actions,omitempty"`
	GoalConfirmation        GoalConfirmationStatus          `json:"goal_confirmation_status,omitempty"`
	Snapshot                *protocol.ExecutionSnapshot     `json:"snapshot,omitempty"`
	GoalAuthority           *GoalAuthorityReceipt           `json:"-"`
	WorkBinding             *WorkBindingReceipt             `json:"-"`
	ResponsibilityAuthority *ResponsibilityAuthorityReceipt `json:"-"`
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

// SupersededResult 表示 command 属于已经被 retarget/replace 关闭的旧责任。
func SupersededResult(
	snapshot *protocol.ExecutionSnapshot,
	err error,
) MutationResult {
	result := mutationResultFromSnapshot(snapshot)
	result.Outcome = MutationSuperseded
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		result.ReasonCode = domainErr.Code
		result.Message = domainErr.Message
		return result
	}
	result.ReasonCode = ErrorCodeExecutionTerminal
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

func withConfirmedGoalAuthority(result MutationResult) MutationResult {
	if result.Snapshot == nil {
		return result
	}
	execution := result.Snapshot.Execution
	if strings.TrimSpace(execution.ID) == "" ||
		strings.TrimSpace(execution.GoalID) == "" ||
		execution.GoalObjectiveRevision <= 0 {
		return result
	}
	result.GoalAuthority = &GoalAuthorityReceipt{
		GoalID:            strings.TrimSpace(execution.GoalID),
		ObjectiveRevision: execution.GoalObjectiveRevision,
		ExecutionID:       strings.TrimSpace(execution.ID),
	}
	result.GoalConfirmation = GoalConfirmationConfirmed
	return result
}

func withPendingGoalConfirmation(
	result MutationResult,
	message string,
	next NextAction,
) MutationResult {
	result.GoalConfirmation = GoalConfirmationPending
	result.Message = strings.TrimSpace(message)
	result.NextActions = normalizeNextActions(append(result.NextActions, next))
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
