package server

import (
	"context"
	"net/http"
	"strings"
	"time"
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
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	return stopAll, nil
}
