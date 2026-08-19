package host

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

// cliServiceProvider 按命令域延迟创建服务，避免 nexusctl help 等命令启动全量后端依赖。
type cliServiceProvider struct {
	cfg config.Config

	mu     sync.Mutex
	logger *slog.Logger

	app     *serverapp.AppServices
	appErr  error
	appDone bool

	auth     *authsvc.Service
	authDB   *sql.DB
	authErr  error
	authDone bool
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

func (p *cliServiceProvider) AppServices() (*serverapp.AppServices, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.appDone {
		return p.app, p.appErr
	}
	p.appDone = true
	p.app, p.appErr = serverapp.NewAppServices(p.cfg, p.logger)
	return p.app, p.appErr
}

func (p *cliServiceProvider) AuthService() (*authsvc.Service, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.app != nil {
		return p.app.Auth, nil
	}
	if p.authDone {
		return p.auth, p.authErr
	}
	p.authDone = true
	p.authDB, p.authErr = serverapp.OpenDB(p.cfg)
	if p.authErr != nil {
		return nil, p.authErr
	}
	p.auth = authsvc.NewServiceWithDB(p.cfg, p.authDB)
	return p.auth, nil
}

// Close 释放 provider 在本次 CLI 执行中延迟创建的全部资源。
func (p *cliServiceProvider) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	app := p.app
	authDB := p.authDB
	p.app = nil
	p.auth = nil
	p.authDB = nil
	p.mu.Unlock()

	var closeErrors []error
	if app != nil {
		closeErrors = append(closeErrors, app.Close(ctx))
	}
	if authDB != nil {
		closeErrors = append(closeErrors, authDB.Close())
	}
	return errors.Join(closeErrors...)
}
