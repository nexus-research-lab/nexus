// INPUT: 会话键与可取消的 ctx。
// OUTPUT: 按会话串行化的派发边界锁；不同会话互不阻塞，等待可随 ctx 取消。
// POS: dm 派发边界的唯一互斥原语。锁不可重入，持锁期间不得再取任何会话的派发锁。
package dm

import (
	"context"
	"strings"
	"sync"
)

// dispatchLocks 串行化同一会话的「检查是否在跑 → 起轮或投递」边界。
// 锁内常含 runtime 准备（拉起子进程、装配 MCP），若全局共用一把，一个会话起轮就会挡住所有会话的派发与引导 hook。
type dispatchLocks struct {
	mu      sync.Mutex
	entries map[string]*dispatchLockEntry
}

// dispatchLockEntry 的 token 容量为 1，有值即空闲；refs 计入持有者与等待者，归零即回收。
type dispatchLockEntry struct {
	token chan struct{}
	refs  int
}

// lock 取得会话的派发锁，返回幂等的释放函数；ctx 取消时放弃等待。
func (l *dispatchLocks) lock(ctx context.Context, sessionKey string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(sessionKey)
	entry := l.join(key)
	select {
	case <-ctx.Done():
		l.leave(key)
		return nil, ctx.Err()
	case <-entry.token:
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.token <- struct{}{}
			l.leave(key)
		})
	}, nil
}

func (l *dispatchLocks) join(key string) *dispatchLockEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]*dispatchLockEntry)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &dispatchLockEntry{token: make(chan struct{}, 1)}
		entry.token <- struct{}{}
		l.entries[key] = entry
	}
	entry.refs++
	return entry
}

func (l *dispatchLocks) leave(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.refs--; entry.refs == 0 {
		delete(l.entries, key)
	}
}
