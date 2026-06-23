package provider

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const (
	// ProviderKindLLM 表示对话运行时 Provider。
	ProviderKindLLM = "llm"
	// ProviderKindImageGeneration 表示图片生成 Provider。
	ProviderKindImageGeneration = "image_generation"
)

var providerIDCounter atomic.Uint64

// Service 提供 Provider 配置管理与运行时解析。
type Service struct {
	repository *providerstore.Repository
	now        func() time.Time
	idFactory  func(string) string
	client     *http.Client
	logger     *slog.Logger
}

type providerModelTarget struct {
	provider providerstore.Entity
	model    providerstore.ModelEntity
}

// NewServiceWithDB 使用共享 DB 创建 Provider 配置服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: providerstore.NewRepository(cfg, db),
		now:        func() time.Time { return time.Now().UTC() },
		idFactory:  newProviderID,
		client:     &http.Client{Timeout: 30 * time.Second},
		logger:     logx.NewDiscardLogger(),
	}
}

// SetLogger 注入 Provider 服务日志器。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetHTTPClient 覆盖 Provider 服务使用的 HTTP client，主要用于测试。
func (s *Service) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.client = client
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func newProviderID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UTC().UnixNano(), providerIDCounter.Add(1))
}
