package server

import (
	"log/slog"
	"net/http"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"

	"github.com/go-chi/chi/v5"
)

// Server 表示完整 HTTP 进程入口。
type Server struct {
	config   config.Config
	api      *handlershared.API
	router   chi.Router
	services *AppServices
	handlers handlerSet
}

// New 创建 HTTP server。
func New(cfg config.Config) (*Server, error) {
	return NewWithLogger(cfg, nil)
}

// NewWithLogger 创建带显式 logger 的 HTTP server。
func NewWithLogger(cfg config.Config, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = newLogger(cfg)
	}

	appServices, err := NewAppServices(cfg, logger)
	if err != nil {
		return nil, err
	}

	api := handlershared.NewAPI(logger)
	websocketHandler := newWebSocketHandler(api, appServices)

	server := &Server{
		config:   cfg,
		api:      api,
		router:   chi.NewRouter(),
		services: appServices,
		handlers: newHandlerSet(api, appServices, websocketHandler),
	}

	server.mountMiddleware(logger)
	server.mountRoutes()
	return server, nil
}

// Router 返回已初始化路由。
func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) mountMiddleware(logger *slog.Logger) {
	s.router.Use(handlershared.RequestContextMiddleware(logger))
	s.router.Use(handlershared.AccessLogMiddleware())
	s.router.Use(handlershared.RecoverMiddleware(s.api))
	s.router.Use(handlershared.AuthMiddleware(s.api, s.services.Auth))
}

func newLogger(cfg config.Config) *slog.Logger {
	return logx.New(logx.Options{
		Service: cfg.ProjectName,
		Level:   cfg.LogLevel,
		Format:  cfg.LogFormat,
		Stdout:  cfg.LogStdout,
		NoColor: cfg.LogNoColor,
		File: logx.FileOptions{
			Enabled:     cfg.LogFileEnabled,
			Path:        cfg.LogPath,
			RotateDaily: cfg.LogRotateDaily,
			MaxSizeMB:   cfg.LogMaxSizeMB,
			MaxAgeDays:  cfg.LogMaxAgeDays,
			MaxBackups:  cfg.LogMaxBackups,
			Compress:    cfg.LogCompress,
		},
	})
}
