// INPUT: 宿主在 Goal continuation durable claim 成功后签发的 DM continuation 绑定。
// OUTPUT: 供同一 physical round 的 command builder 做 exact identity 校验的只读 authority。
// POS: Goal continuation 从调度器进入 runtime command capability 的不可伪造边界。
package runtime

import "strings"

// SourceContextGoalContinuation is the only source label that can upgrade a
// DM internal round after exact continuation authority validation.
const SourceContextGoalContinuation = "agent_goal_continuation"

// GoalContinuationAuthority 是 host-only 的 DM Goal continuation 绑定。
//
// 它不是 Goal 或 Execution 的业务状态，也不由模型输入构造。只有
// ValidateContinuationForDispatch 与 ClaimContinuationPlan 都成功后，DM
// 调度器才会把它放进本轮 Request。ExecutionID 在 Goal-only continuation
// 中可以为空；一旦存在，必须与同轮 Goal/Responsibility authority 完全一致。
type GoalContinuationAuthority struct {
	OwnerUserID       string
	AgentID           string
	ScopeSessionKey   string
	GoalID            string
	ObjectiveRevision int64
	ExecutionID       string
	RootRoundID       string
}

// Normalized 返回只清洗宿主已签发值的副本；它不会补全或推断任何 identity。
func (a GoalContinuationAuthority) Normalized() GoalContinuationAuthority {
	a.OwnerUserID = strings.TrimSpace(a.OwnerUserID)
	a.AgentID = strings.TrimSpace(a.AgentID)
	a.ScopeSessionKey = strings.TrimSpace(a.ScopeSessionKey)
	a.GoalID = strings.TrimSpace(a.GoalID)
	a.ExecutionID = strings.TrimSpace(a.ExecutionID)
	a.RootRoundID = strings.TrimSpace(a.RootRoundID)
	return a
}

// Valid 只判断 continuation authority 自身是否完整，不代表它已经匹配某个
// runtime context；匹配必须在 command builder 中再次逐字段完成。
func (a GoalContinuationAuthority) Valid() bool {
	a = a.Normalized()
	return a.OwnerUserID != "" && a.AgentID != "" &&
		a.ScopeSessionKey != "" && a.GoalID != "" &&
		a.ObjectiveRevision > 0 && a.RootRoundID != ""
}
