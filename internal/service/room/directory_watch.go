// INPUT: 已提交目录版本、可取消生命周期和失效通知回调。
// OUTPUT: 合并跨进程目录变更通知，读取失败保留上次版本供下次核对。
// POS: Room/Agent 目录变化观察器，不重新执行资源 mutation。
package room

import (
	"context"
	"time"
)

// DirectoryRevisionSource 只提供目录事务提交后的标量版本。
type DirectoryRevisionSource interface {
	DirectoryRevision(context.Context) (int64, error)
}

// WatchDirectory 在启动和版本变化时发出核对信号，读取失败后继续核对。
// 一秒只读取单行版本，连续事务合并为一次通知。
func WatchDirectory(ctx context.Context, source DirectoryRevisionSource, notify func(context.Context), report func(error)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var previous int64
	initialized := false
	for {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		revision, err := source.DirectoryRevision(readCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			report(err)
		} else if !initialized || revision != previous {
			notify(ctx)
			previous, initialized = revision, true
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
