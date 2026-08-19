// INPUT: Agent runtime 启动上下文，以及认证等安全边界转场的 revoke/commit 回调。
// OUTPUT: 可撤销的 admission lease；转场期间阻断新 admission，并在提交前排空旧 admission。
// POS: 不依赖认证或 runtime 实现的安全转场并发原语。
package runtimeadmission

import (
	"context"
	"fmt"
	"sync"
)

// Gate 把 runtime admission 与安全边界转场串行化。
//
// Transition 会先阻断新 admission、取消并排空全部在途 lease，再执行 revoke
// 和 commit。只有 commit 返回后才重新开放 admission。
type Gate struct {
	mu         sync.Mutex
	nextID     uint64
	active     map[uint64]context.CancelFunc
	transition *transitionState
}

type transitionState struct {
	done    chan struct{}
	drained chan struct{}
}

// Lease 表示一次 runtime 启动 admission。
//
// 调用方必须持有到 runtime session 与 round 已共同登记到宿主生命周期管理器，
// 或启动失败为止。Release 与安全转场都会取消 Context。
type Lease struct {
	gate   *Gate
	id     uint64
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

// NewGate 创建 runtime admission gate。
func NewGate() *Gate {
	return &Gate{active: make(map[uint64]context.CancelFunc)}
}

// NewDetachedLease 创建不参与转场的 lease，供没有动态安全边界的调用方使用。
func NewDetachedLease(ctx context.Context) *Lease {
	leaseContext, cancel := context.WithCancel(ctx)
	return &Lease{ctx: leaseContext, cancel: cancel}
}

// Context 返回可在安全转场时被取消的启动上下文。
func (l *Lease) Context() context.Context {
	return l.ctx
}

// Release 结束 admission。该操作幂等。
func (l *Lease) Release() {
	l.once.Do(func() {
		if l.gate == nil {
			l.cancel()
			return
		}
		l.gate.release(l.id)
	})
}

// Admit 获取一次 runtime admission；安全转场期间等待转场完成。
func (g *Gate) Admit(ctx context.Context) (*Lease, error) {
	for {
		g.mu.Lock()
		if g.transition == nil {
			g.nextID++
			id := g.nextID
			leaseContext, cancel := context.WithCancel(ctx)
			g.active[id] = cancel
			g.mu.Unlock()
			return &Lease{gate: g, id: id, ctx: leaseContext}, nil
		}
		done := g.transition.done
		g.mu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Transition 执行一次安全边界转场。
//
// 顺序固定为：阻断新 admission -> 取消并排空在途 admission -> revoke
// 既有 runtime -> commit 新边界 -> 重新开放 admission。revoke 或 commit
// 失败时仍会重新开放，但不会越过失败阶段。
func (g *Gate) Transition(
	ctx context.Context,
	revoke func(context.Context) error,
	commit func(context.Context) error,
) error {
	state, cancels, err := g.beginTransition(ctx)
	if err != nil {
		return err
	}
	defer g.finishTransition(state)

	for _, cancel := range cancels {
		cancel()
	}
	select {
	case <-state.drained:
	case <-ctx.Done():
		return fmt.Errorf("drain runtime admissions: %w", ctx.Err())
	}
	if err = ctx.Err(); err != nil {
		return fmt.Errorf("continue runtime security transition: %w", err)
	}
	if err = revoke(ctx); err != nil {
		return fmt.Errorf("revoke runtime before security transition: %w", err)
	}
	if err = commit(ctx); err != nil {
		return fmt.Errorf("commit runtime security transition: %w", err)
	}
	return nil
}

func (g *Gate) beginTransition(ctx context.Context) (*transitionState, []context.CancelFunc, error) {
	for {
		g.mu.Lock()
		if g.transition == nil {
			state := &transitionState{
				done:    make(chan struct{}),
				drained: make(chan struct{}),
			}
			g.transition = state
			cancels := make([]context.CancelFunc, 0, len(g.active))
			for _, cancel := range g.active {
				cancels = append(cancels, cancel)
			}
			if len(g.active) == 0 {
				close(state.drained)
			}
			g.mu.Unlock()
			return state, cancels, nil
		}
		done := g.transition.done
		g.mu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("wait for runtime security transition: %w", ctx.Err())
		}
	}
}

func (g *Gate) finishTransition(state *transitionState) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.transition = nil
	close(state.done)
}

func (g *Gate) release(id uint64) {
	g.mu.Lock()
	cancel := g.active[id]
	delete(g.active, id)
	if g.transition != nil && len(g.active) == 0 {
		close(g.transition.drained)
	}
	g.mu.Unlock()
	cancel()
}
