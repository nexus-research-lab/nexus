// INPUT: 宿主生命周期、目录数据库和全局 WebSocket 订阅。
// OUTPUT: 跨进程目录变更观察器的启动与同步退出。
// POS: 目录观察服务的应用装配，不承载目录业务规则。
package server

import (
	"context"
	rooms "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// startDirectoryWatch 补齐独立 CLI 写入后的全局目录失效通知。
func (s *Server) startDirectoryWatch(ctx context.Context) (func(), error) {
	if s.services == nil || s.services.DB == nil || s.handlers.websocket == nil {
		return nil, nil
	}
	source := roomrepo.NewSQLRepository(s.config.DatabaseDriver, s.services.DB)
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		rooms.WatchDirectory(runCtx, source, func(ctx context.Context) {
			// 仅发送无资源身份的失效信号，客户端继续按自身权限读取目录。
			s.handlers.websocket.BroadcastDirectoryChanged(ctx, "agent_room_directory_changed", nil)
		}, func(err error) {
			s.api.BaseLogger().Warn("读取目录变更版本失败", "err", err)
		})
	}()
	return func() { cancel(); <-done }, nil
}
