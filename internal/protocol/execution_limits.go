// INPUT: Execution Orchestration 中会被投影给模型的集合长度与 parent round exit 时间。
// OUTPUT: 单一集合上限、subagent 固定恢复宽限期与可跨 service/storage 稳定识别的校验。
// POS: MCP schema、领域校验、SQL 防御、runtime hook 与异常历史投影共用的限制真相源。
package protocol

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ExecutionProjectionCollectionLimit 是单个模型执行契约集合的最大成员数。
//
// 该上限适用于 completion/acceptance criteria、单个 Work Item 的直接依赖、
// input/output scopes、Submission refs/evidence、Acceptance criteria results
// 及其 evidence，以及 Resume evidence。
const ExecutionProjectionCollectionLimit = 32

// SubagentReconciliationGrace 是 parent physical round 退出后等待迟到 child
// lifecycle 终态的固定宽限期。runtime、service 与 storage 必须共享这一真相源。
const SubagentReconciliationGrace = 30 * time.Second

// ErrExecutionProjectionLimitExceeded 可由 service 与 storage 使用 errors.Is 分类。
var ErrExecutionProjectionLimitExceeded = errors.New("projection_limit_exceeded")

// ExecutionProjectionLimitError 保留越界字段、实际数量和协议上限。
type ExecutionProjectionLimitError struct {
	Field string
	Count int
	Limit int
}

func (e *ExecutionProjectionLimitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s: %s has %d items; maximum is %d",
		ErrExecutionProjectionLimitExceeded,
		strings.TrimSpace(e.Field),
		e.Count,
		e.Limit,
	)
}

func (e *ExecutionProjectionLimitError) Unwrap() error {
	return ErrExecutionProjectionLimitExceeded
}

// ValidateExecutionProjectionLimit 拒绝任何不能被无损模型投影的集合。
func ValidateExecutionProjectionLimit(field string, count int) error {
	if count <= ExecutionProjectionCollectionLimit {
		return nil
	}
	return &ExecutionProjectionLimitError{
		Field: strings.TrimSpace(field),
		Count: count,
		Limit: ExecutionProjectionCollectionLimit,
	}
}

// ValidSubagentReconciliationDeadline 只接受精确的 T+30s durable deadline。
func ValidSubagentReconciliationDeadline(parentRoundExitedAt, reconcileAfter time.Time) bool {
	if parentRoundExitedAt.IsZero() || reconcileAfter.IsZero() {
		return false
	}
	return reconcileAfter.UTC().Equal(
		parentRoundExitedAt.UTC().Add(SubagentReconciliationGrace),
	)
}
