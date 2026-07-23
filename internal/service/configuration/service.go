// INPUT: Nexus 领域服务、数据库、主机配置与 runtime 管理器。
// OUTPUT: 按 owner 隔离、只允许主智能体写入的统一配置服务。
// POS: configuration 控制面的依赖装配与安全边界。
package configuration

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

var (
	// ErrMainAgentRequired 防止普通 Agent 读取或改写 owner 级全局配置。
	ErrMainAgentRequired = errors.New("只有 Nexus 主智能体可以读取或修改全局配置")
	// ErrRevisionRequired 强制对话写入先读取/预检再应用。
	ErrRevisionRequired = errors.New("expected_revision 不能为空；请先调用 plan_nexus_configuration_change")
)

// Service 聚合现有领域服务，不直接复刻其业务规则。
type Service struct {
	cfg        config.Config
	db         *sql.DB
	dialect    storage.SQLDialect
	agents     *agentsvc.Service
	providers  *providersvc.Service
	prefs      *preferencessvc.Service
	channels   *channels.ControlService
	connectors *connectorsvc.Service
	skills     *skillsvc.Service
	runtime    *runtimectx.Manager
}

// NewService 创建统一配置控制面。
func NewService(
	cfg config.Config,
	db *sql.DB,
	agents *agentsvc.Service,
	providers *providersvc.Service,
	prefs *preferencessvc.Service,
	channelControl *channels.ControlService,
	connectors *connectorsvc.Service,
	skills *skillsvc.Service,
	runtime *runtimectx.Manager,
) *Service {
	return &Service{
		cfg: cfg, db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver),
		agents: agents, providers: providers, prefs: prefs, channels: channelControl,
		connectors: connectors, skills: skills, runtime: runtime,
	}
}

func requireMainActor(actor Actor) error {
	if !actor.IsMainAgent {
		return ErrMainAgentRequired
	}
	if strings.TrimSpace(actor.OwnerUserID) == "" || strings.TrimSpace(actor.AgentID) == "" {
		return errors.New("配置调用缺少可信 owner 或 agent 身份")
	}
	return nil
}

func scopedContext(ctx context.Context, actor Actor) context.Context {
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: actor.OwnerUserID, Username: actor.AgentID,
		Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal,
	})
}
