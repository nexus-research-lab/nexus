// INPUT: 宿主签发的 Goal/Execution identity、Work/Review binding 与可信 mutation receipt。
// OUTPUT: 同一物理 round 内 Goal、Execution、Work、Review responsibility 的单一并发安全 authority snapshot。
// POS: runtime identity 边界；已构造的 Goal/Execution MCP、runtime graph 与 hooks 只读此状态，模型输入不能直接推进。
package runtime

import (
	"context"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ResponsibilityLane 描述当前物理 round 唯一有效的责任 lane。它是宿主内部
// capability 状态，不投影给模型，也不替代 durable Execution 状态机。
type ResponsibilityLane string

const (
	ResponsibilityLaneUnbound      ResponsibilityLane = "unbound"
	ResponsibilityLanePlanning     ResponsibilityLane = "planning"
	ResponsibilityLaneExecution    ResponsibilityLane = "execution"
	ResponsibilityLaneCoordination ResponsibilityLane = "coordination"
	ResponsibilityLaneWork         ResponsibilityLane = "work"
	ResponsibilityLaneReview       ResponsibilityLane = "review"
)

// ResponsibilityAuthority 是一次原子读取所得的完整 capability snapshot。
// ReservedExecutionID 只表示 retarget 后已持久化的 successor reservation；在
// materialization receipt 到达前，它不能充当可操作 Execution identity。
type ResponsibilityAuthority struct {
	Generation          uint64
	GoalID              string
	ObjectiveRevision   int64
	ExecutionID         string
	ReservedExecutionID string
	Lane                ResponsibilityLane
	WorkBinding         *protocol.ExecutionWorkBinding
	ReviewBinding       *protocol.ExecutionReviewBinding
}

// ResponsibilityAuthorityState 把过去分别静态捕获的 ExecutionID、
// ReviewBinding 与动态 WorkBinding 合并到一个锁保护的 snapshot。goalAuthority
// 是同一状态链的 Goal 视图；Goal mutation 由 ApplyGoalMutation 在此锁内提交。
type ResponsibilityAuthorityState struct {
	mu                     sync.RWMutex
	value                  ResponsibilityAuthority
	goalAuthority          *GoalAuthorityState
	releasedExecutionID    string
	initialBindingsInvalid bool
}

// NewResponsibilityAuthorityState 从可信 round identity 初始化统一状态。相互
// 冲突或不完整的 Work/Review binding 会 fail closed，不会被降级成另一条 lane。
func NewResponsibilityAuthorityState(
	goalAuthority *GoalAuthorityState,
	executionID string,
	workBinding *protocol.ExecutionWorkBinding,
	reviewBinding *protocol.ExecutionReviewBinding,
) *ResponsibilityAuthorityState {
	state := &ResponsibilityAuthorityState{goalAuthority: goalAuthority}
	state.value.ExecutionID = strings.TrimSpace(executionID)
	state.value.Lane = ResponsibilityLaneUnbound
	if state.value.ExecutionID != "" {
		state.value.Lane = ResponsibilityLaneExecution
	}
	if authority, ok := goalAuthority.LoadBound(); ok {
		state.value.GoalID = authority.GoalID
		state.value.ObjectiveRevision = authority.ObjectiveRevision
		if state.value.ExecutionID == "" {
			state.value.ExecutionID = strings.TrimSpace(authority.ExecutionID)
		}
	}

	validWork := completeRuntimeWorkBinding(workBinding)
	validReview := completeRuntimeReviewBinding(reviewBinding)
	if workBinding != nil && !validWork ||
		reviewBinding != nil && !validReview ||
		validWork && validReview {
		// A round can own exactly one responsibility lane. Retaining only the
		// ambient Execution here would silently turn an incomplete/corrupted
		// binding into coordinator authority, so invalidate the whole snapshot.
		state.value.GoalID = ""
		state.value.ObjectiveRevision = 0
		state.value.ExecutionID = ""
		state.value.ReservedExecutionID = ""
		state.value.WorkBinding = nil
		state.value.ReviewBinding = nil
		state.value.Lane = ResponsibilityLaneUnbound
		state.initialBindingsInvalid = true
		return state
	}
	if validWork {
		binding := cloneRuntimeWorkBinding(workBinding)
		if state.value.ExecutionID != "" && state.value.ExecutionID != binding.ExecutionID {
			state.value = ResponsibilityAuthority{Lane: ResponsibilityLaneUnbound}
			state.initialBindingsInvalid = true
			return state
		}
		state.value.ExecutionID = binding.ExecutionID
		state.value.WorkBinding = binding
		state.value.Lane = ResponsibilityLaneWork
		return state
	}
	if validReview {
		binding := cloneRuntimeReviewBinding(reviewBinding)
		if state.value.ExecutionID != "" && state.value.ExecutionID != binding.ExecutionID {
			state.value = ResponsibilityAuthority{Lane: ResponsibilityLaneUnbound}
			state.initialBindingsInvalid = true
			return state
		}
		state.value.ExecutionID = binding.ExecutionID
		state.value.ReviewBinding = binding
		state.value.Lane = ResponsibilityLaneReview
	}
	return state
}

// GoalAuthorityState 返回与统一状态共同推进的 Goal authority 视图。
func (s *ResponsibilityAuthorityState) GoalAuthorityState() *GoalAuthorityState {
	if s == nil {
		return nil
	}
	return s.goalAuthority
}

// LoadGoalAuthority 从统一 snapshot 读取 Goal mutation fence；统一状态存在时，
// Goal MCP 与 Execution MCP 都不再分别读取 GoalAuthorityState 造成撕裂。
func (s *ResponsibilityAuthorityState) LoadGoalAuthority() (GoalAuthority, bool) {
	if s == nil {
		return GoalAuthority{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	authority := GoalAuthority{
		GoalID:            s.value.GoalID,
		ObjectiveRevision: s.value.ObjectiveRevision,
		ExecutionID:       s.value.ExecutionID,
	}
	return authority, authority.GoalID != "" && authority.ObjectiveRevision > 0
}

// GrantGoalAuthority 初始化 trusted continuation/Goal-bound work round 的 exact
// fence。它只补全相同 Goal/revision/Execution，不改变现有 Work/Review lane。
func (s *ResponsibilityAuthorityState) GrantGoalAuthority(
	goalID string,
	objectiveRevision int64,
	executionID string,
) bool {
	if s == nil || s.goalAuthority == nil {
		return false
	}
	goalID = strings.TrimSpace(goalID)
	executionID = strings.TrimSpace(executionID)
	if goalID == "" || objectiveRevision <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.value.GoalID != "" && s.value.GoalID != goalID {
		return false
	}
	if s.value.ObjectiveRevision > objectiveRevision {
		return false
	}
	if s.value.ExecutionID != "" && executionID != "" && s.value.ExecutionID != executionID {
		return false
	}
	if s.value.GoalID == goalID &&
		s.value.ObjectiveRevision == objectiveRevision &&
		(executionID == "" || s.value.ExecutionID == executionID) {
		return true
	}
	if !s.goalAuthority.Bind(goalID, objectiveRevision, executionID) {
		return false
	}
	s.value.GoalID = goalID
	s.value.ObjectiveRevision = objectiveRevision
	if s.value.ExecutionID == "" {
		s.value.ExecutionID = executionID
		if executionID != "" && s.value.Lane == ResponsibilityLaneUnbound {
			s.value.Lane = ResponsibilityLaneExecution
		}
	}
	s.value.Generation++
	return true
}

// ClearGoalAuthority 撤销整条 round authority；旧 Execution/Work/Review 不能在
// Goal usage 已清理后继续通过已构造的 MCP server 生效。
func (s *ResponsibilityAuthorityState) ClearGoalAuthority() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.goalAuthority != nil {
		s.goalAuthority.Clear()
	}
	s.value = ResponsibilityAuthority{Generation: s.value.Generation + 1}
	s.mu.Unlock()
}

// Load 返回隔离副本，使一次 Execution tool 调用不会混读不同 generation。
func (s *ResponsibilityAuthorityState) Load() (ResponsibilityAuthority, bool) {
	if s == nil {
		return ResponsibilityAuthority{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := s.value
	result.WorkBinding = cloneRuntimeWorkBinding(result.WorkBinding)
	result.ReviewBinding = cloneRuntimeReviewBinding(result.ReviewBinding)
	return result, true
}

// SeedExecution 补全构造 Room slot 时尚不可见的 round Execution identity。
// 已经进入其他 Execution 或 reserved successor 的状态不会被覆盖。
func (s *ResponsibilityAuthorityState) SeedExecution(executionID string) bool {
	if s == nil {
		return false
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.value.ExecutionID != "" {
		return s.value.ExecutionID == executionID
	}
	// A static orchestrationActor captured when the physical round started may
	// still call SeedExecution after terminal release. The terminal fence must
	// win over that stale construction-time identity.
	if s.releasedExecutionID == executionID && s.value.ReservedExecutionID == "" {
		return false
	}
	if s.value.ReservedExecutionID != "" && s.value.ReservedExecutionID != executionID {
		return false
	}
	s.value.ExecutionID = executionID
	s.value.ReservedExecutionID = ""
	s.releasedExecutionID = ""
	if s.value.Lane == ResponsibilityLaneUnbound || s.value.Lane == ResponsibilityLanePlanning {
		s.value.Lane = ResponsibilityLaneExecution
	}
	s.value.Generation++
	return true
}

// ApplyGoalMutation 原子推进共享 Goal view 与 Execution responsibility。retarget
// reservation 会在同一次加锁中撤销 predecessor 的 Work/Review/Execution capability。
func (s *ResponsibilityAuthorityState) ApplyGoalMutation(item protocol.Goal) bool {
	if s == nil || s.goalAuthority == nil {
		return false
	}
	goalID := strings.TrimSpace(item.ID)
	revision := item.ObjectiveRevision()
	bindingState := protocol.GoalExecutionBindingStateFromGoal(item)
	executionID := strings.TrimSpace(protocol.GoalReservedExecutionID(item))
	if goalID == "" || revision <= 0 {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	switch bindingState {
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		s.goalAuthority.Clear()
		s.value = ResponsibilityAuthority{Generation: s.value.Generation + 1}
		return true
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		if !s.goalAuthority.Bind(goalID, revision, "") {
			return false
		}
	case protocol.GoalExecutionBindingStateConfirmed:
		if executionID == "" || !s.goalAuthority.Bind(goalID, revision, executionID) {
			return false
		}
	default:
		return false
	}

	previousExecutionID := s.value.ExecutionID
	s.value.GoalID = goalID
	s.value.ObjectiveRevision = revision
	s.value.Generation++
	switch bindingState {
	case protocol.GoalExecutionBindingStateStandalone:
		s.value.ExecutionID = ""
		s.value.ReservedExecutionID = ""
		s.value.WorkBinding = nil
		s.value.ReviewBinding = nil
		s.value.Lane = ResponsibilityLaneUnbound
	case protocol.GoalExecutionBindingStateReserved:
		s.value.ExecutionID = ""
		s.value.ReservedExecutionID = executionID
		s.value.WorkBinding = nil
		s.value.ReviewBinding = nil
		s.value.Lane = ResponsibilityLanePlanning
	case protocol.GoalExecutionBindingStateConfirmed:
		s.value.ExecutionID = executionID
		s.value.ReservedExecutionID = ""
		if previousExecutionID != executionID {
			s.value.WorkBinding = nil
			s.value.ReviewBinding = nil
			s.value.Lane = ResponsibilityLaneExecution
		}
	}
	return true
}

// BindCoordination 消费服务端 coordination receipt。它只允许当前 Execution
// 幂等升级，或把 exact reserved successor 物化为当前 Execution。
func (s *ResponsibilityAuthorityState) BindCoordination(executionID string) bool {
	if s == nil {
		return false
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.releasedExecutionID == executionID &&
		s.value.ExecutionID == "" &&
		s.value.ReservedExecutionID == "" {
		return false
	}
	if s.value.ExecutionID == executionID &&
		s.value.ReservedExecutionID == "" &&
		s.value.WorkBinding == nil &&
		s.value.ReviewBinding == nil &&
		s.value.Lane == ResponsibilityLaneCoordination {
		return true
	}
	if s.value.ExecutionID != "" && s.value.ExecutionID != executionID {
		return false
	}
	if s.value.ExecutionID == "" &&
		s.value.ReservedExecutionID != "" &&
		s.value.ReservedExecutionID != executionID {
		return false
	}
	s.value.ExecutionID = executionID
	s.value.ReservedExecutionID = ""
	s.releasedExecutionID = ""
	s.value.WorkBinding = nil
	s.value.ReviewBinding = nil
	s.value.Lane = ResponsibilityLaneCoordination
	s.value.Generation++
	return true
}

// ConfirmGoalExecution 同步 Execution 侧 durable Goal confirmation，但在
// Execution identity 未变化时保留 exact Work/Review lane。Goal promotion 不是
// responsibility release，不能借此把 worker/reviewer 升格成 coordinator。
func (s *ResponsibilityAuthorityState) ConfirmGoalExecution(
	goalID string,
	objectiveRevision int64,
	executionID string,
) bool {
	if s == nil || s.goalAuthority == nil {
		return false
	}
	goalID = strings.TrimSpace(goalID)
	executionID = strings.TrimSpace(executionID)
	if goalID == "" || objectiveRevision <= 0 || executionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.value.ExecutionID != "" && s.value.ExecutionID != executionID {
		return false
	}
	if s.value.ReservedExecutionID != "" && s.value.ReservedExecutionID != executionID {
		return false
	}
	if !s.goalAuthority.Bind(goalID, objectiveRevision, executionID) {
		return false
	}
	s.value.GoalID = goalID
	s.value.ObjectiveRevision = objectiveRevision
	s.value.ExecutionID = executionID
	s.value.ReservedExecutionID = ""
	s.releasedExecutionID = ""
	if s.value.Lane == ResponsibilityLaneUnbound || s.value.Lane == ResponsibilityLanePlanning {
		s.value.Lane = ResponsibilityLaneExecution
	}
	s.value.Generation++
	return true
}

// BindWork 消费持久化 self Assignment 后的 exact receipt。未释放的 review 或
// sibling work responsibility 不能被覆盖。
func (s *ResponsibilityAuthorityState) BindWork(binding *protocol.ExecutionWorkBinding) bool {
	if s == nil || !completeRuntimeWorkBinding(binding) {
		return false
	}
	next := cloneRuntimeWorkBinding(binding)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.value.ExecutionID == next.ExecutionID &&
		s.value.ReservedExecutionID == "" &&
		s.value.ReviewBinding == nil &&
		s.value.Lane == ResponsibilityLaneWork &&
		sameRuntimeWorkBinding(s.value.WorkBinding, next) {
		return true
	}
	if s.value.ExecutionID != "" && s.value.ExecutionID != next.ExecutionID {
		return false
	}
	if s.value.ReservedExecutionID != "" && s.value.ReservedExecutionID != next.ExecutionID {
		return false
	}
	if s.value.ReviewBinding != nil ||
		(s.value.WorkBinding != nil && !sameRuntimeWorkBinding(s.value.WorkBinding, next)) {
		return false
	}
	s.value.ExecutionID = next.ExecutionID
	s.value.ReservedExecutionID = ""
	s.releasedExecutionID = ""
	s.value.WorkBinding = next
	s.value.ReviewBinding = nil
	s.value.Lane = ResponsibilityLaneWork
	s.value.Generation++
	return true
}

// RevokeExecution 撤销 exact terminal predecessor；迟到的旧 receipt 不能清除
// 已经绑定的 successor。
func (s *ResponsibilityAuthorityState) RevokeExecution(executionID string) bool {
	if s == nil {
		return false
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialBindingsInvalid {
		return false
	}
	if s.value.ExecutionID != executionID {
		return false
	}
	s.releasedExecutionID = executionID
	s.value.ExecutionID = ""
	s.value.WorkBinding = nil
	s.value.ReviewBinding = nil
	if s.value.ReservedExecutionID != "" {
		s.value.Lane = ResponsibilityLanePlanning
	} else {
		s.value.Lane = ResponsibilityLaneUnbound
	}
	s.value.Generation++
	return true
}

func completeRuntimeReviewBinding(binding *protocol.ExecutionReviewBinding) bool {
	return binding != nil &&
		strings.TrimSpace(binding.ExecutionID) != "" &&
		strings.TrimSpace(binding.PlanID) != "" &&
		strings.TrimSpace(binding.WorkItemID) != "" &&
		strings.TrimSpace(binding.SpecID) != "" &&
		strings.TrimSpace(binding.AssignmentID) != "" &&
		strings.TrimSpace(binding.SubmissionID) != "" &&
		strings.TrimSpace(binding.ReviewDispatchID) != "" &&
		strings.TrimSpace(binding.TargetAgentID) != ""
}

func cloneRuntimeReviewBinding(binding *protocol.ExecutionReviewBinding) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	result.ExecutionID = strings.TrimSpace(result.ExecutionID)
	result.PlanID = strings.TrimSpace(result.PlanID)
	result.WorkItemID = strings.TrimSpace(result.WorkItemID)
	result.SpecID = strings.TrimSpace(result.SpecID)
	result.AssignmentID = strings.TrimSpace(result.AssignmentID)
	result.SubmissionID = strings.TrimSpace(result.SubmissionID)
	result.ReviewDispatchID = strings.TrimSpace(result.ReviewDispatchID)
	result.TargetAgentID = strings.TrimSpace(result.TargetAgentID)
	return &result
}

type responsibilityAuthorityContextKey struct{}

// WithResponsibilityAuthorityState 把当前 round 的统一 capability 交给组合 MCP builder。
func WithResponsibilityAuthorityState(
	ctx context.Context,
	state *ResponsibilityAuthorityState,
) context.Context {
	if state == nil {
		return ctx
	}
	return context.WithValue(ctx, responsibilityAuthorityContextKey{}, state)
}

// ResponsibilityAuthorityStateFromContext 读取可信 runtime 注入的统一 capability。
func ResponsibilityAuthorityStateFromContext(ctx context.Context) *ResponsibilityAuthorityState {
	state, _ := ctx.Value(responsibilityAuthorityContextKey{}).(*ResponsibilityAuthorityState)
	return state
}
