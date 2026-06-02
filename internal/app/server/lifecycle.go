package server

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	// Git 导入和外部 skill 更新会同步等待网络传输与重试，写超时需要覆盖完整操作窗口。
	httpWriteTimeout = 6 * time.Minute
	httpIdleTimeout  = 60 * time.Second
)

// ListenAndServe 启动后台服务与 HTTP 服务。
func (s *Server) ListenAndServe(ctx context.Context) error {
	stopBackground, err := s.startBackgroundServices(ctx)
	if err != nil {
		return err
	}
	defer stopBackground()

	httpServer := &http.Server{
		Addr:              s.config.Address(),
		Handler:           s.router,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	go func() {
		<-ctx.Done()
		s.api.BaseLogger().Info("收到停止信号，开始关闭 HTTP 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.api.BaseLogger().Info("HTTP 服务开始监听",
		"addr", s.config.Address(),
		"api_prefix", s.config.APIPrefix,
		"websocket_path", s.config.WebSocketPath,
	)
	return httpServer.ListenAndServe()
}

func (s *Server) startBackgroundServices(ctx context.Context) (func(), error) {
	var stops []func()
	stopAll := func() {
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}

	if s.services != nil && s.services.Channels != nil {
		if s.services.ChannelControl != nil {
			if err := s.services.ChannelControl.LoadConfiguredChannels(ctx); err != nil {
				s.api.BaseLogger().Warn("加载 IM 通道配置失败，跳过数据库通道注册", "err", err)
			}
		}
		s.api.BaseLogger().Info("启动通道适配器",
			"discord_enabled", s.config.DiscordEnabled,
			"discord_configured", strings.TrimSpace(s.config.DiscordBotToken) != "",
			"telegram_enabled", s.config.TelegramEnabled,
			"telegram_configured", strings.TrimSpace(s.config.TelegramBotToken) != "",
			"registered_channels", s.services.Channels.RegisteredChannelTypes(),
		)
		if err := s.services.Channels.Start(ctx); err != nil {
			s.api.BaseLogger().Error("启动通道适配器失败", "err", err)
			return nil, err
		}
		stops = append(stops, func() {
			s.services.Channels.Stop(context.Background())
		})
	}

	if s.services != nil && s.services.Automation != nil {
		s.api.BaseLogger().Info("启动自动化调度器")
		if err := s.services.Automation.Start(ctx); err != nil {
			s.api.BaseLogger().Error("启动自动化调度器失败", "err", err)
			stopAll()
			return nil, err
		}
		stops = append(stops, s.services.Automation.Stop)
	}

	if stopRuntimeIdleReclaimer := s.startRuntimeIdleSessionReclaimer(ctx); stopRuntimeIdleReclaimer != nil {
		stops = append(stops, stopRuntimeIdleReclaimer)
	}

	return stopAll, nil
}

func (s *Server) startRuntimeIdleSessionReclaimer(ctx context.Context) func() {
	if s.services == nil || s.services.Runtime == nil {
		return nil
	}
	idleFor := s.config.RuntimeIdleSessionTTL()
	sweepInterval := s.config.RuntimeIdleSessionSweepInterval()
	if idleFor <= 0 || sweepInterval <= 0 {
		return nil
	}

	runCtx, stop := context.WithCancel(ctx)
	s.api.BaseLogger().Info("启动 runtime 空闲 session 回收器",
		"idle_ttl_seconds", int64(idleFor.Seconds()),
		"sweep_interval_seconds", int64(sweepInterval.Seconds()),
	)
	go s.runRuntimeIdleSessionReclaimer(runCtx, sweepInterval, idleFor)
	return stop
}

func (s *Server) runRuntimeIdleSessionReclaimer(ctx context.Context, sweepInterval time.Duration, idleFor time.Duration) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			closed, err := s.services.Runtime.CloseIdleSessions(ctx, idleFor)
			if err != nil {
				s.api.BaseLogger().Warn("runtime 空闲 session 回收失败", "closed", closed, "err", err)
				continue
			}
			if closed > 0 {
				s.api.BaseLogger().Info("runtime 空闲 session 已回收", "closed", closed)
			}
		}
	}
}
