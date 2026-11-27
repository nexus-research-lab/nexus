// INPUT: managed Execution identity、首个读取方的投影函数与 invalidateExecution 失效事实。
// OUTPUT: 同一 Execution 在两次失效之间只投影一次的完整 WorkGraph；并发读取共享同一次计算。
// POS: 公共 WorkGraph 骨架与节点详情的共享读模型；失效契约与前端 execution_invalidated 相同。
package orchestration

import (
	"context"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// executionViewCacheLimit 只需覆盖同时打开的会话；超出时淘汰最久未读的已完成项。
const executionViewCacheLimit = 32

// executionViewCache 以 invalidateExecution 为唯一失效来源：所有改变 WorkGraph 的
// mutation 与 runtime 事实都必须发出失效，前端也只靠同一事件刷新，因此缓存不会比
// UI 契约更陈旧。缓存值只读，调用方必须先复制再裁剪。
type executionViewCache struct {
	mu      sync.Mutex
	entries map[string]*executionViewCacheEntry
	clock   uint64
}

type executionViewCacheEntry struct {
	ownerUserID string
	sessionKey  string
	rootRoundID string
	done        chan struct{}
	view        *protocol.ExecutionView
	err         error
	lastUsed    uint64
}

// invalidate 丢弃该 Execution 的缓存；正在计算的结果仍交给已经在等待的读取方，
// 但不会再被后来的读取复用。
func (c *executionViewCache) invalidate(executionID string) {
	if executionID == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, executionID)
	c.mu.Unlock()
}

// invalidateRootRound 覆盖 runtime 读取的 root_round_id 分支：Plan 绑定前开始的
// root round 在同一轮内继续写入时没有 Execution identity，但仍属于该图。
func (c *executionViewCache) invalidateRootRound(ownerUserID, sessionKey, rootRoundID string) {
	if rootRoundID == "" {
		return
	}
	c.mu.Lock()
	for id, entry := range c.entries {
		if entry.ownerUserID == ownerUserID && entry.sessionKey == sessionKey && entry.rootRoundID == rootRoundID {
			delete(c.entries, id)
		}
	}
	c.mu.Unlock()
}

// load 返回缓存的完整投影，缺失时由第一个读取方计算，其余读取方等待同一结果。
// 计算不随单个请求取消，避免一个关闭的页面让共享读取全部失败。
func (c *executionViewCache) load(
	ctx context.Context,
	execution *protocol.Execution,
	compute func(context.Context) (*protocol.ExecutionView, error),
) (*protocol.ExecutionView, error) {
	executionID := execution.ID
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*executionViewCacheEntry)
	}
	c.clock++
	entry := c.entries[executionID]
	if entry != nil && (entry.ownerUserID != execution.OwnerUserID ||
		entry.sessionKey != execution.SessionKey ||
		entry.rootRoundID != execution.RootRoundID) {
		// identity 已由调用方按 owner/session 校验；不同作用域绝不共享缓存值。
		c.mu.Unlock()
		return compute(ctx)
	}
	if entry == nil {
		entry = &executionViewCacheEntry{
			ownerUserID: execution.OwnerUserID,
			sessionKey:  execution.SessionKey,
			rootRoundID: execution.RootRoundID,
			done:        make(chan struct{}),
		}
		c.entries[executionID] = entry
		c.evictLocked(executionID)
		entry.lastUsed = c.clock
		c.mu.Unlock()
		c.fill(ctx, executionID, entry, compute)
	} else {
		entry.lastUsed = c.clock
		c.mu.Unlock()
	}
	select {
	case <-entry.done:
		return entry.view, entry.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *executionViewCache) fill(
	ctx context.Context,
	executionID string,
	entry *executionViewCacheEntry,
	compute func(context.Context) (*protocol.ExecutionView, error),
) {
	entry.view, entry.err = compute(context.WithoutCancel(ctx))
	// 失败不缓存；history_pending 由后台回填补齐且不发失效，也不缓存。
	keep := entry.err == nil && (entry.view == nil || !entry.view.Graph.HistoryPending)
	close(entry.done)
	if keep {
		return
	}
	c.mu.Lock()
	if c.entries[executionID] == entry {
		delete(c.entries, executionID)
	}
	c.mu.Unlock()
}

func (c *executionViewCache) evictLocked(keepID string) {
	for len(c.entries) > executionViewCacheLimit {
		oldestID := ""
		var oldest *executionViewCacheEntry
		for id, entry := range c.entries {
			if id == keepID || !executionViewCacheEntryDone(entry) {
				continue
			}
			if oldest == nil || entry.lastUsed < oldest.lastUsed {
				oldestID, oldest = id, entry
			}
		}
		if oldest == nil {
			return
		}
		delete(c.entries, oldestID)
	}
}

func executionViewCacheEntryDone(entry *executionViewCacheEntry) bool {
	select {
	case <-entry.done:
		return true
	default:
		return false
	}
}
