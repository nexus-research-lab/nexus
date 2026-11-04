// INPUT: 宿主生命周期、Room 目录版本和全局 WebSocket 订阅。
// OUTPUT: 跨进程目录变更观察器的启动与同步退出。
// POS: 目录观察服务的应用装配，不承载目录业务规则。
package server

import (
	"context"

	rooms "github.com/nexus-research-lab/nexus/internal/service/room"
)

// startDirectoryWatch 补齐独立 CLI 写入后的全局目录失效通知；宿主自身 mutation 的即时通知已覆盖的版本不重复发送。
func (s *Server) startDirectoryWatch(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.Core == nil || s.services.Core.Room == nil || s.handlers.websocket == nil {
		return nil, nil
	}
	websocketHandler := s.handlers.websocket
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		rooms.WatchDirectory(runCtx, s.services.Core.Room, websocketHandler.DirectoryCoverage(), func(ctx context.Context, change rooms.DirectoryChange) {
			// 只发送无资源身份的失效信号，客户端继续按自身权限读取目录。
			websocketHandler.BroadcastObservedDirectoryChange(ctx, change)
		}, func(err error) {
			s.api.BaseLogger().Warn("读取目录变更版本失败", "err", err)
		})
	}()
	return func() { cancel(); <-done }, nil
}
