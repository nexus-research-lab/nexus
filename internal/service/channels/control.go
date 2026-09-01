// INPUT: Nexus 配置、数据库、Agent 解析器与通道路由器。
// OUTPUT: owner 隔离、active/legacy keyring 凭据兼容的消息渠道控制服务及共享写入锁。
// POS: Channels 业务服务的装配根，统一持有配置、持久化和进程内并发边界。
package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type ControlService struct {
	config                    config.Config
	db                        *sql.DB
	driver                    string
	key                       []byte
	keyring                   *credentials.Keyring
	agents                    agentWorkspaceResolver
	router                    *Router
	httpClient                *http.Client
	idFactory                 func(string) string
	loginStore                *channelLoginStore
	loginTimeout              time.Duration
	weixinLoginClientFactory  func(string, map[string]string) personalWeixinLoginClient
	registrationClientFactory func(string) appregistration.Client
	registrationPollInterval  time.Duration
	authorizationCommitGuard  ChannelLoginAuthorizationCommitGuard
	keyErr                    error
}

func NewControlService(
	cfg config.Config,
	db *sql.DB,
	agents agentWorkspaceResolver,
	router *Router,
) *ControlService {
	key, err := credentials.DecodeKey(cfg.ConnectorCredentialsKey)
	keyring, keyringErr := credentials.NewKeyring(
		cfg.ConnectorCredentialsKey,
		cfg.ConnectorCredentialsLegacyKeys,
	)
	if err == nil {
		err = keyringErr
	}
	return &ControlService{
		config:       cfg,
		db:           db,
		driver:       storage.NormalizeSQLDriver(cfg.DatabaseDriver),
		key:          key,
		keyring:      keyring,
		agents:       agents,
		router:       router,
		idFactory:    channelcontract.NewID,
		loginStore:   newChannelLoginStore(),
		loginTimeout: 8 * time.Minute,
		keyErr:       err,
	}
}

// SetHTTPClient 注入 IM 通道主动投递使用的 HTTP client，主要用于测试或统一出站链路配置。
func (s *ControlService) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

// SetChannelLoginAuthorizationCommitGuard wires the fail-closed lease used by
// conversationally bound QR logins immediately before credential persistence.
func (s *ControlService) SetChannelLoginAuthorizationCommitGuard(
	guard ChannelLoginAuthorizationCommitGuard,
) {
	if s == nil {
		return
	}
	s.authorizationCommitGuard = guard
}

func (s *ControlService) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *ControlService) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, s.bind(index))
	}
	return strings.Join(items, ",")
}

func (s *ControlService) ensureAgent(ctx context.Context, agentID string) error {
	if s.agents == nil {
		return nil
	}
	_, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	return err
}

func (s *ControlService) agentName(ctx context.Context, agentID string) string {
	if s.agents == nil || strings.TrimSpace(agentID) == "" {
		return ""
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil || agentValue == nil {
		return ""
	}
	return strings.TrimSpace(agentValue.Name)
}

func (s *ControlService) defaultAgentForChannel(ctx context.Context, ownerUserID string, channelType string) (string, error) {
	row, err := s.getChannelConfigRow(ctx, ownerUserID, normalizeIMChannelType(channelType))
	if err != nil || row == nil {
		return "", err
	}
	return row.AgentID, nil
}

func (s *ControlService) encryptCredentials(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if s.keyErr != nil && strings.TrimSpace(s.config.ConnectorCredentialsKey) != "" {
		return "", classifyChannelControlError(
			ErrChannelCredentialStoreUnavailable,
			fmt.Errorf("CONNECTOR_CREDENTIALS_KEY 解析失败: %w", s.keyErr),
		)
	}
	if len(s.key) == 0 {
		return "", classifyChannelControlError(
			ErrChannelCredentialStoreUnavailable,
			errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 IM 通道凭据"),
		)
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return s.keyring.EncryptEnvelope(payload)
}

func (s *ControlService) decryptCredentials(encrypted sql.NullString) (map[string]string, error) {
	if !encrypted.Valid || strings.TrimSpace(encrypted.String) == "" {
		return nil, nil
	}
	if s.keyErr != nil && strings.TrimSpace(s.config.ConnectorCredentialsKey) != "" {
		return nil, classifyChannelControlError(
			ErrChannelCredentialStoreUnavailable,
			fmt.Errorf("CONNECTOR_CREDENTIALS_KEY 解析失败: %w", s.keyErr),
		)
	}
	if len(s.key) == 0 {
		return nil, classifyChannelControlError(
			ErrChannelCredentialStoreUnavailable,
			errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 IM 通道凭据"),
		)
	}
	payload, err := s.keyring.DecryptEnvelope(encrypted.String)
	if err != nil {
		return nil, err
	}
	var result map[string]string
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return normalizeStringMap(result), nil
}
