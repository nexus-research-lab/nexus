// INPUT: 已提交目录版本、宿主即时通知覆盖的版本、可取消生命周期和失效通知回调。
// OUTPUT: 只补发即时通知未覆盖的跨进程目录变更，读取失败保留上次版本供下次核对。
// POS: Room/Agent 目录变化观察器，不重新执行资源 mutation。
package room

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// DirectoryVersionSource 只提供目录事务提交后的标量版本。
type DirectoryVersionSource interface {
	DirectoryVersion(context.Context) (roomrepo.DirectoryVersion, error)
}

// DirectoryChange 是观察器需要补发的失效范围；Agents 为 true 时 Agent 列表也需重读。
type DirectoryChange struct {
	Version roomrepo.DirectoryVersion
	Agents  bool
}

// DirectoryCoverage 记录宿主即时通知发出前已提交的版本。
// 客户端在收到即时通知后才重读，因此读到的数据不早于该版本，观察器无需重复通知。
type DirectoryCoverage struct {
	mu      sync.Mutex
	covered roomrepo.DirectoryVersion
}

// Cover 记录一次即时通知覆盖的版本；agents 表示该通知会令客户端同时重读 Agent 列表。
func (c *DirectoryCoverage) Cover(version roomrepo.DirectoryVersion, agents bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.covered.Directory = max(c.covered.Directory, version.Directory)
	if agents {
		c.covered.Agent = max(c.covered.Agent, version.Agent)
	}
}

func (c *DirectoryCoverage) uncovered(previous, current roomrepo.DirectoryVersion) (DirectoryChange, bool) {
	if c == nil {
		c = &DirectoryCoverage{}
	}
	c.mu.Lock()
	covered := c.covered
	c.mu.Unlock()
	agents := current.Agent != previous.Agent && current.Agent > covered.Agent
	directory := current.Directory != previous.Directory && current.Directory > covered.Directory
	return DirectoryChange{Version: current, Agents: agents}, agents || directory
}

// DirectoryVersion 读取已提交目录版本；仓储不支持时返回 errors.ErrUnsupported。
func (s *Service) DirectoryVersion(ctx context.Context) (roomrepo.DirectoryVersion, error) {
	source, ok := s.repository.(DirectoryVersionSource)
	if !ok {
		return roomrepo.DirectoryVersion{}, errors.ErrUnsupported
	}
	return source.DirectoryVersion(ctx)
}

// WatchDirectory 以启动时版本为基线，只在版本变化且未被即时通知覆盖时发出核对信号。
// 一秒只读取单行版本，连续事务合并为一次通知；启动前的变化由客户端订阅核对补齐。
func WatchDirectory(
	ctx context.Context,
	source DirectoryVersionSource,
	coverage *DirectoryCoverage,
	notify func(context.Context, DirectoryChange),
	report func(error),
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var previous roomrepo.DirectoryVersion
	initialized := false
	for {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		version, err := source.DirectoryVersion(readCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errors.ErrUnsupported) {
			return
		}
		if err != nil {
			report(err)
		} else {
			if initialized {
				if change, ok := coverage.uncovered(previous, version); ok {
					notify(ctx, change)
				}
			}
			previous, initialized = version, true
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
