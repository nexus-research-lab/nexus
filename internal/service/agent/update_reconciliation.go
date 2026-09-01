// INPUT: Agent 配置已经提交后发生的 workspace 投影或读取补全失败。
// OUTPUT: 可由 HTTP 边界可靠识别的“主写入已提交”证据。
// POS: Agent 更新的提交后阶段错误；不改变仓储事务、Agent 身份或重试行为。
package agent

import (
	"errors"
	"fmt"
)

// UpdateReconcileError 表示 Agent 配置已经持久化，但提交后的本地投影或读取补全没有完成。
type UpdateReconcileError struct {
	cause error
}

func (e *UpdateReconcileError) Error() string {
	if e == nil || e.cause == nil {
		return "Agent 配置已保存，但提交后同步需要 reconcile"
	}
	return fmt.Sprintf("Agent 配置已保存，但提交后同步需要 reconcile: %v", e.cause)
}

func (e *UpdateReconcileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// AgentUpdateCommitted 只依据服务层显式提交证据判断，不能根据错误文案猜测。
func AgentUpdateCommitted(err error) bool {
	var committed *UpdateReconcileError
	return errors.As(err, &committed)
}
