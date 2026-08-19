// INPUT: Room host 签发的 exact WorkBinding 与同一物理 round 内的显式责任转场。
// OUTPUT: Execution command、runtime graph 与 subagent hooks 共用的并发安全动态 WorkBinding。
// POS: runtime identity 边界；DM 不注入该状态，Room 只能消费宿主 mutation receipt，模型输入不能写入。
package runtime

import (
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// WorkBindingState 保存当前物理 round 的 exact 工作责任。一个尚未释放的
// binding 不允许被另一个 Assignment 覆盖；宿主必须先完成显式责任转场。
type WorkBindingState struct {
	mu             sync.RWMutex
	binding        *protocol.ExecutionWorkBinding
	responsibility *ResponsibilityAuthorityState
}

// NewWorkBindingState 从宿主已有的 structured Room slot binding 创建状态。
func NewWorkBindingState(binding *protocol.ExecutionWorkBinding) *WorkBindingState {
	state := &WorkBindingState{}
	normalized := binding.Normalized()
	if normalized.Complete() {
		state.binding = &normalized
	}
	return state
}

// NewWorkBindingStateFromResponsibility 为 runtime graph/hook 消费面提供
// WorkBinding 投影视图；读写都落到统一 Responsibility authority，不复制状态。
func NewWorkBindingStateFromResponsibility(
	state *ResponsibilityAuthorityState,
) *WorkBindingState {
	if state == nil {
		return NewWorkBindingState(nil)
	}
	return &WorkBindingState{responsibility: state}
}

// Load 返回隔离副本；不完整的 binding 永远不构成 runtime capability。
func (s *WorkBindingState) Load() (*protocol.ExecutionWorkBinding, bool) {
	if s == nil {
		return nil, false
	}
	if s.responsibility != nil {
		authority, _ := s.responsibility.Load()
		if !authority.WorkBinding.Complete() {
			return nil, false
		}
		return cloneRuntimeWorkBinding(authority.WorkBinding), true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.binding.Complete() {
		return nil, false
	}
	return cloneRuntimeWorkBinding(s.binding), true
}

// Bind 消费宿主签发的 exact receipt。重复绑定幂等；未释放的责任不能切换。
func (s *WorkBindingState) Bind(binding *protocol.ExecutionWorkBinding) bool {
	if s == nil {
		return false
	}
	if s.responsibility != nil {
		return s.responsibility.BindWork(binding)
	}
	normalized := binding.Normalized()
	if !normalized.Complete() {
		return false
	}
	next := &normalized
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binding != nil && !sameRuntimeWorkBinding(s.binding, next) {
		return false
	}
	s.binding = next
	return true
}

// Clear 撤销当前 round 的工作 capability；已构造的 command context 会立即观察到。
func (s *WorkBindingState) Clear() {
	if s == nil {
		return
	}
	if s.responsibility != nil {
		authority, _ := s.responsibility.Load()
		if authority.ExecutionID != "" {
			s.responsibility.BindCoordination(authority.ExecutionID)
		}
		return
	}
	s.mu.Lock()
	s.binding = nil
	s.mu.Unlock()
}

// sameRuntimeWorkBinding 比较两个已清洗的 binding 是否同一身份。
func sameRuntimeWorkBinding(left, right *protocol.ExecutionWorkBinding) bool {
	return left != nil && right != nil && *left == *right
}

// cloneRuntimeWorkBinding 浅拷贝快照所有权；字段清洗已在 ingress 完成。
func cloneRuntimeWorkBinding(binding *protocol.ExecutionWorkBinding) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}
