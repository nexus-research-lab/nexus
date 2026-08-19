// INPUT: physical round 装配阶段登记的临时资源清理函数。
// OUTPUT: 可并发登记、幂等关闭且关闭后立即回收迟到资源的 round 资源容器。
// POS: runtime command capability、0600 输入槽与 physical round 生命周期的统一所有权边界。
package runtimecommand

import "sync"

// RoundResources owns host resources that must remain valid for one physical
// round and must not follow a shorter runtime-preparation context.
type RoundResources struct {
	mu       sync.Mutex
	closed   bool
	cleanups []func()
}

func NewRoundResources() *RoundResources {
	return &RoundResources{}
}

// Add transfers ownership of cleanup to the physical round. A resource that
// arrives after terminal close is reclaimed immediately instead of leaking.
func (r *RoundResources) Add(cleanup func()) {
	if cleanup == nil {
		return
	}
	if r == nil {
		cleanup()
		return
	}
	r.mu.Lock()
	if !r.closed {
		r.cleanups = append(r.cleanups, cleanup)
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	cleanup()
}

// Close releases the round's resources in reverse construction order.
func (r *RoundResources) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	cleanups := r.cleanups
	r.cleanups = nil
	r.mu.Unlock()
	for index := len(cleanups) - 1; index >= 0; index-- {
		cleanups[index]()
	}
}
