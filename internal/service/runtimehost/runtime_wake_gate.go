// INPUT: idle drain 识别到 runtime 自行开启 turn 的会话键，以及准备启动普通 round 的请求。
// OUTPUT: 自唤醒交接期间的会话闸门；普通输入在交接完成前等待，随后按“已有运行 round”路由到队列或引导。
// POS: DM/Room 共用的自唤醒交接互斥边界；只保护 drain 识别 → 自唤醒 round 登记这一窗口，不承载 round 生命周期。
package runtimehost

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// ErrRuntimeWakeHandoffPending 表示普通 round 撞上了正在交接的自唤醒 turn；调用方应等待后从路由阶段重试。
var ErrRuntimeWakeHandoffPending = errors.New("runtime 自唤醒 turn 正在交接")

// RuntimeWakeGates 以会话键登记正在交接的自唤醒 turn；零值可用。
type RuntimeWakeGates struct {
	mu      sync.Mutex
	pending map[string]chan struct{}
}

// Begin 在 drain 识别到自唤醒时同步登记；返回的 release 幂等，交接成功或失败都必须调用。
func (g *RuntimeWakeGates) Begin(key string) func() {
	key = strings.TrimSpace(key)
	if g == nil || key == "" {
		return func() {}
	}
	done := make(chan struct{})
	g.mu.Lock()
	if g.pending == nil {
		g.pending = make(map[string]chan struct{})
	}
	g.pending[key] = done
	g.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			if g.pending[key] == done {
				delete(g.pending, key)
			}
			g.mu.Unlock()
			close(done)
		})
	}
}

// Pending 返回 key 当前交接的完成信号；没有交接时为 nil。
func (g *RuntimeWakeGates) Pending(key string) <-chan struct{} {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if done, ok := g.pending[strings.TrimSpace(key)]; ok {
		return done
	}
	return nil
}

// Wait 阻塞到 key 的交接结束；返回是否真的等待过。
func (g *RuntimeWakeGates) Wait(ctx context.Context, key string) (bool, error) {
	done := g.Pending(key)
	if done == nil {
		return false, nil
	}
	select {
	case <-done:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	}
}
