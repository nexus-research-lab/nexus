// INPUT: Nexus 领域服务、数据库、主机配置与 runtime 管理器。
// OUTPUT: 按 owner/Agent/Room 隔离并在每次调用动态鉴权的统一配置服务。
// POS: configuration 控制面的依赖装配与安全边界。
package configuration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

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
	ErrRevisionRequired = errors.New("expected_revision 不能为空；请先运行 nexuscfg plan")
)

// Service 聚合现有领域服务，不直接复刻其业务规则。
type Service struct {
	cfg                        config.Config
	db                         *sql.DB
	dialect                    storage.SQLDialect
	agents                     *agentsvc.Service
	providers                  *providersvc.Service
	prefs                      *preferencessvc.Service
	channels                   *channels.ControlService
	connectors                 *connectorsvc.Service
	skills                     *skillsvc.Service
	runtime                    *runtimectx.Manager
	sessions                   sessionConfigurationService
	rooms                      roomConfigurationService
	roomRuntime                roomRuntimeController
	notifier                   configurationNotifier
	mutationLocks              [256]sync.Mutex
	integrityMu                sync.Mutex
	integrityKey               []byte
	approvalMu                 sync.Mutex
	humanApprovals             map[string]humanApprovalRecord
	approvalNow                func() time.Time
	runtimeCapabilityMu        sync.Mutex
	runtimeCapabilities        map[string]*runtimeCapabilityRecord
	runtimeCapabilityBySession map[string]string
	runtimeCapabilityNow       func() time.Time
	humanVerifier              interactiveHumanVerifier
	roleResolver               activePrincipalRoleResolver
}

type interactiveHumanVerifier interface {
	VerifyInteractiveHuman(context.Context, *authctx.Principal) (*authctx.Principal, error)
	AcquireBoundInteractiveHumanLease(
		context.Context,
		string,
		string,
		string,
	) (*authctx.Principal, func(), error)
}

type activePrincipalRoleResolver interface {
	ResolveActivePrincipalRole(context.Context, string) (string, error)
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
		humanApprovals:             make(map[string]humanApprovalRecord),
		approvalNow:                func() time.Time { return time.Now().UTC() },
		runtimeCapabilities:        make(map[string]*runtimeCapabilityRecord),
		runtimeCapabilityBySession: make(map[string]string),
		runtimeCapabilityNow:       func() time.Time { return time.Now().UTC() },
	}
}

// SetRoomControl 注入 Room 持久化和实时撤权能力。
func (s *Service) SetRoomControl(rooms roomConfigurationService, runtime roomRuntimeController) {
	s.rooms = rooms
	s.roomRuntime = runtime
}

// SetSessionControl 注入 owner-confined、版本化且可安全关闭热态的 Session 服务。
func (s *Service) SetSessionControl(sessions sessionConfigurationService) {
	s.sessions = sessions
}

// SetNotifier 注入目录和 Room resync 事件，不让领域服务反向依赖 websocket。
func (s *Service) SetNotifier(notifier configurationNotifier) {
	s.notifier = notifier
}

// SetPrincipalVerifiers 注入人工在场与当前用户角色的认证真相源。
func (s *Service) SetPrincipalVerifiers(
	humanVerifier interactiveHumanVerifier,
	roleResolver activePrincipalRoleResolver,
) {
	s.humanVerifier = humanVerifier
	s.roleResolver = roleResolver
}

func scopedContext(ctx context.Context, actor Actor) context.Context {
	role := strings.TrimSpace(actor.PrincipalRole)
	authMethod := strings.TrimSpace(actor.AuthMethod)
	if actor.LocalSingleUser &&
		strings.TrimSpace(actor.OwnerUserID) == authctx.SystemUserID {
		role = authctx.RoleOwner
		authMethod = authctx.AuthMethodLocal
	}
	switch role {
	case authctx.RoleOwner, authctx.RoleAdmin, authctx.RoleMember:
	default:
		// 缺少可信管理角色时仍允许访问 owner 自己的私有资源，但按 member
		// fail-closed，绝不能借配置 transport 获得宿主或公共 Provider 权限。
		role = authctx.RoleMember
	}
	if authMethod == "" {
		authMethod = "mcp_runtime"
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: actor.OwnerUserID, Username: actor.AgentID,
		Role: role, AuthMethod: authMethod,
	})
}

// privateProviderMutationContext 把 Provider 写入能力固定为 owner 私有资源。
//
// 即使调用配置服务的真实用户是 owner/admin，也不能把该角色转交给
// Agent 去修改公共订阅 Provider；Provider 服务会据此拒绝任何竞态下回退到的
// public 记录。
func privateProviderMutationContext(ctx context.Context, actor Actor) context.Context {
	authMethod := strings.TrimSpace(actor.AuthMethod)
	if authMethod == "" {
		authMethod = "mcp_runtime"
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: actor.OwnerUserID, Username: actor.AgentID,
		Role: authctx.RoleMember, AuthMethod: authMethod,
	})
}

func actorWithTrustedRequestPrincipal(ctx context.Context, actor Actor) Actor {
	actor.OwnerUserID = strings.TrimSpace(actor.OwnerUserID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if principal := authctx.PrincipalFromContext(ctx); principal != nil &&
		strings.TrimSpace(principal.UserID) == actor.OwnerUserID {
		actor.PrincipalRole = strings.TrimSpace(principal.Role)
		actor.AuthMethod = strings.TrimSpace(principal.AuthMethod)
		actor.AuthSessionID = ""
		if principal.SessionID != nil {
			actor.AuthSessionID = strings.TrimSpace(*principal.SessionID)
		}
		actor.LocalSingleUser = false
	}
	if queued, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok &&
		queued.UserID == actor.OwnerUserID {
		// Background queue workers carry a synthetic RoleOwner solely to scope
		// host reads. Never treat it as configuration authority: restore only
		// the DB-backed auth identity and let roleResolver reload the role.
		actor.PrincipalRole = authctx.RoleMember
		actor.AuthMethod = queued.AuthMethod
		actor.AuthSessionID = queued.SessionID
		actor.LocalSingleUser = false
	}
	state, hasState := authctx.StateFromContext(ctx)
	if hasState &&
		!state.AuthRequired &&
		actor.OwnerUserID == authctx.SystemUserID &&
		strings.TrimSpace(actor.PrincipalRole) == authctx.RoleOwner &&
		strings.TrimSpace(actor.AuthMethod) == authctx.AuthMethodLocal {
		actor.LocalSingleUser = true
	}
	return actor
}

// integrityKeyBytes 返回当前进程的不可导出摘要密钥。进程重启会使旧 plan
// 自动失效，避免把低熵凭据编码成可离线猜测的公开 revision。
func (s *Service) integrityKeyBytes() ([]byte, error) {
	s.integrityMu.Lock()
	defer s.integrityMu.Unlock()
	if len(s.integrityKey) == 0 {
		s.integrityKey = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, s.integrityKey); err != nil {
			s.integrityKey = nil
			return nil, err
		}
	}
	key := make([]byte, len(s.integrityKey))
	copy(key, s.integrityKey)
	return key, nil
}
