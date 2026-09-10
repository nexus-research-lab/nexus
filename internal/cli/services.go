package cli

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
)

// cliServiceProvider 按命令域延迟创建服务，避免 nexusctl help 等命令启动全量后端依赖。
type cliServiceProvider struct {
	cfg config.Config

	mu     sync.Mutex
	logger *slog.Logger

	app     *app.AppServices
	appErr  error
	appDone bool
}

func newCLIServiceProvider(cfg config.Config) *cliServiceProvider {
	return &cliServiceProvider{cfg: cfg}
}

func (p *cliServiceProvider) SetLogger(logger *slog.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger = logger
	if p.app != nil {
		bindServiceLogger(p.app, logger)
	}
}

func (p *cliServiceProvider) AppServices() (*app.AppServices, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.appDone {
		return p.app, p.appErr
	}
	p.appDone = true
	p.app, p.appErr = app.NewAppServices(p.cfg, p.logger)
	return p.app, p.appErr
}

// Close 释放 provider 在本次 CLI 执行中延迟创建的全部资源。
func (p *cliServiceProvider) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	app := p.app
	p.app = nil
	p.mu.Unlock()

	var closeErrors []error
	if app != nil {
		closeErrors = append(closeErrors, app.Close(ctx))
	}
	return errors.Join(closeErrors...)
}
