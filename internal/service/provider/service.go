// INPUT: 数据库、运行配置、HTTP client 与日志依赖。
// OUTPUT: Provider 管理服务及稳定导出的聚合 CAS 错误协议。
// POS: Provider 服务装配入口。
package provider

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

var (
	// ErrConfigurationVersionConflict 表示 Provider 聚合已被并发写入推进。
	ErrConfigurationVersionConflict = providerstore.ErrConfigurationVersionConflict
	// ErrProviderNotFound 表示 Provider 在条件写入前已不存在。
	ErrProviderNotFound = providerstore.ErrProviderNotFound
	// ErrModelNotFound 表示条件模型写入没有命中持久化模型卡。
	ErrModelNotFound = providerstore.ErrModelNotFound
	// ErrMutationNotApplied 表示 Provider 事务已确认回滚或尚未开始。
	ErrMutationNotApplied = providerstore.ErrMutationNotApplied
)

// Service 提供 Provider 配置管理与运行时解析。
type Service struct {
	repository                    *providerstore.Repository
	now                           func() time.Time
	idFactory                     func(string) string
	client                        *http.Client
	logger                        *slog.Logger
	defaultAgentSelectionResolver DefaultAgentSelectionResolver
	desktopMode                   bool
}

// DefaultAgentSelection 表示用户为 Agent runtime 选择的全局默认模型。
type DefaultAgentSelection struct {
	Provider    string
	Model       string
	RuntimeKind string
}

// DefaultAgentSelectionResolver 让 Provider 生命周期在不依赖偏好服务实现的前提下校验回退目标。
type DefaultAgentSelectionResolver func(context.Context, string) (DefaultAgentSelection, error)

type providerModelTarget struct {
	provider providerstore.Entity
	model    providerstore.ModelEntity
}

// NewServiceWithDB 使用共享 DB 创建 Provider 配置服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository:  providerstore.NewRepository(cfg, db),
		now:         func() time.Time { return time.Now().UTC() },
		idFactory:   newProviderID,
		client:      &http.Client{Timeout: 30 * time.Second},
		logger:      logx.NewDiscardLogger(),
		desktopMode: strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop"),
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

// SetDefaultAgentSelectionResolver 注入用户全局默认模型读取器。
func (s *Service) SetDefaultAgentSelectionResolver(resolver DefaultAgentSelectionResolver) {
	s.defaultAgentSelectionResolver = resolver
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func newProviderID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UTC().UnixNano(), providerIDCounter.Add(1))
}
