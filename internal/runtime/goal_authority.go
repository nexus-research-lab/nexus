// INPUT: runtime mint 的 exact Goal identity、objective revision 与可选 Execution identity。
// OUTPUT: 同一物理 round 内 Goal/Execution MCP 共用的并发安全动态 authority 与 context 传递 helper。
// POS: runtime identity 边界；共享状态普通 round 保持空，只有可信 continuation/WorkBinding 或成功 create_goal 才能写入；持久负责人新 round 使用 nexus_goal 私有副本。
package runtime

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
)

// GoalAuthority 是当前物理 round 可操作的精确 Goal 身份。
// ExecutionID 仅在 runtime 已持有对应 identity 时携带；Goal-only authority 允许为空。
type GoalAuthority struct {
	GoalID            string
	ObjectiveRevision int64
	ExecutionID       string
}

// GoalAuthorityState 让同一 round 的 Goal MCP 与 Execution MCP 共享动态 authority。
// Goal identity 不允许在一个物理 round 内切换；同一 Goal 的 revision 只允许单调推进。
type GoalAuthorityState struct {
	mu                sync.RWMutex
	goalID            string
	executionID       string
	objectiveRevision *atomic.Int64
}

// NewGoalAuthorityState 创建自持 objective revision 的 authority 状态。
func NewGoalAuthorityState(goalID string, objectiveRevision int64, executionID string) *GoalAuthorityState {
	revision := &atomic.Int64{}
	revision.Store(objectiveRevision)
	return NewGoalAuthorityStateWithRevision(goalID, executionID, revision)
}

// NewGoalAuthorityStateWithRevision 复用 runtime 已有的 revision 指针。
// 这让 steering adoption、Goal MCP 与 Execution MCP 观察同一份单调 revision。
func NewGoalAuthorityStateWithRevision(
	goalID string,
	executionID string,
	objectiveRevision *atomic.Int64,
) *GoalAuthorityState {
	if objectiveRevision == nil {
		objectiveRevision = &atomic.Int64{}
	}
	state := &GoalAuthorityState{objectiveRevision: objectiveRevision}
	if strings.TrimSpace(goalID) != "" && objectiveRevision.Load() > 0 {
		state.goalID = strings.TrimSpace(goalID)
		state.executionID = strings.TrimSpace(executionID)
	}
	return state
}

// Load 返回当前 exact Goal authority。GoalID 与 positive revision 缺一时视为未授权；
// ExecutionID 是可选 fence，不参与 Goal-only authority 的有效性判断。
func (s *GoalAuthorityState) Load() (GoalAuthority, bool) {
	if s == nil {
		return GoalAuthority{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	authority := GoalAuthority{
		GoalID:            s.goalID,
		ObjectiveRevision: s.objectiveRevision.Load(),
		ExecutionID:       s.executionID,
	}
	return authority, authority.GoalID != "" && authority.ObjectiveRevision > 0
}

// Bind 授予或推进同一 Goal 的 authority。它拒绝跨 Goal 切换与 revision 回退。
func (s *GoalAuthorityState) Bind(goalID string, objectiveRevision int64, executionID string) bool {
	if s == nil {
		return false
	}
	goalID = strings.TrimSpace(goalID)
	executionID = strings.TrimSpace(executionID)
	if goalID == "" || objectiveRevision <= 0 {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	currentRevision := s.objectiveRevision.Load()
	if s.goalID != "" && s.goalID != goalID {
		return false
	}
	if s.goalID == goalID && currentRevision > objectiveRevision {
		return false
	}
	if s.goalID == goalID && currentRevision == objectiveRevision &&
		s.executionID != "" && s.executionID != executionID {
		return false
	}
	s.goalID = goalID
	s.executionID = executionID
	s.objectiveRevision.Store(objectiveRevision)
	return true
}

// Clear 撤销当前 round 的 Goal capability；保留 state 指针以便已构造的 MCP server 立即失效。
func (s *GoalAuthorityState) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.goalID = ""
	s.executionID = ""
	s.objectiveRevision.Store(0)
	s.mu.Unlock()
}

// ObjectiveRevisionState 返回兼容 runtime steering adoption 的共享 revision 指针。
// 单独推进这个值不会 mint authority，因为 Load 仍要求精确 GoalID。
func (s *GoalAuthorityState) ObjectiveRevisionState() *atomic.Int64 {
	if s == nil {
		return nil
	}
	return s.objectiveRevision
}

type goalAuthorityContextKey struct{}

// WithGoalAuthorityState 把当前 round 的共享 authority 交给组合 MCP builder。
func WithGoalAuthorityState(ctx context.Context, state *GoalAuthorityState) context.Context {
	if state == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, goalAuthorityContextKey{}, state)
}

// GoalAuthorityStateFromContext 读取由可信 runtime 注入的共享 authority。
func GoalAuthorityStateFromContext(ctx context.Context) *GoalAuthorityState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(goalAuthorityContextKey{}).(*GoalAuthorityState)
	return state
}
