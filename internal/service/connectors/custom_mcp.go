// INPUT: owner 作用域、自定义 MCP 表单与 Connector 加密存储。
// OUTPUT: 默认开启的脱敏目录、逐条历史密文恢复投影、加密 CRUD、owner 启停与 runtime 可消费的完整 MCP 配置。
// POS: 自定义 MCP 复用 Connector 选择语义及 owner 可用性门禁的唯一持久化边界。
package connectors

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

const (
	customMCPAuthType        = "custom_mcp"
	customMCPAuthNone        = "none"
	customMCPAuthBearer      = "bearer"
	customMCPAuthHeaders     = "headers"
	customMCPConnectorPrefix = "custom-mcp:"
	customMCPCatalogLockID   = "__custom_mcp_catalog__"
	customMCPConfigReady     = "ready"
	customMCPConfigRecovery  = "recovery_required"
)

var (
	// ErrCustomMCPServerInvalid 表示请求中的 MCP 配置无法安全进入 runtime。
	ErrCustomMCPServerInvalid = errors.New("自定义 MCP 配置无效")
	// ErrCustomMCPServerNotFound 表示目标自定义 MCP 已不存在。
	ErrCustomMCPServerNotFound = errors.New("自定义 MCP 不存在")
	// ErrCustomMCPServerNameConflict 表示 owner 下已有同名 MCP server。
	ErrCustomMCPServerNameConflict = errors.New("自定义 MCP 名称已存在")
	// ErrCustomMCPServerRecoveryRequired 表示历史密文必须先被完整配置替换。
	ErrCustomMCPServerRecoveryRequired = errors.New("历史自定义 MCP 配置需要恢复")
)

// CustomMCPServerInput 表示创建或更新自定义 MCP 的完整配置。
// env、bearer_token 与 headers 的 nil value 表示更新时保留已有秘密。
type CustomMCPServerInput struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Command     string             `json:"command,omitempty"`
	Args        []string           `json:"args,omitempty"`
	Env         map[string]*string `json:"env,omitempty"`
	URL         string             `json:"url,omitempty"`
	AuthType    string             `json:"auth_type,omitempty"`
	BearerToken *string            `json:"bearer_token,omitempty"`
	Headers     map[string]*string `json:"headers,omitempty"`
}

// CustomMCPServer 是返回给用户界面的脱敏 MCP 配置。
type CustomMCPServer struct {
	ConnectorID        string `json:"connector_id"`
	Enabled            bool   `json:"enabled"`
	ConfigurationState string `json:"configuration_state"`
	CustomMCPServerInput
}

type customMCPServerRecord struct {
	connection connectionRecord
	server     *storedCustomMCPServer
}

type storedCustomMCPServer struct {
	Enabled     bool              `json:"-"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	AuthType    string            `json:"auth_type,omitempty"`
	BearerToken string            `json:"bearer_token,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// IsCustomMCPConnectorID 判断 Connector ID 是否属于自定义 MCP。
func IsCustomMCPConnectorID(connectorID string) bool {
	return strings.HasPrefix(strings.TrimSpace(connectorID), customMCPConnectorPrefix)
}

// ListCustomMCPServers 返回 owner 下全部脱敏自定义 MCP。
func (s *Service) ListCustomMCPServers(
	ctx context.Context,
	ownerUserID string,
) ([]CustomMCPServer, error) {
	records, err := s.listCustomMCPServerRecords(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	items := make([]CustomMCPServer, 0, len(records))
	for _, record := range records {
		if record.server == nil {
			items = append(items, recoveryCustomMCPServer(record.connection))
			continue
		}
		items = append(items, redactCustomMCPServer(
			record.connection.ConnectorID,
			*record.server,
		))
	}
	sort.Slice(items, func(left, right int) bool {
		return strings.ToLower(items[left].Name) < strings.ToLower(items[right].Name)
	})
	return items, nil
}

// CreateCustomMCPServer 创建一条 owner 级自定义 MCP 配置。
func (s *Service) CreateCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	input CustomMCPServerInput,
) (*CustomMCPServer, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	unlock := s.lockConnectorMutation(ownerUserID, customMCPCatalogLockID)
	defer unlock()

	server, err := materializeCustomMCPServer(input, nil)
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrCustomMCPServerInvalid, err)
	}
	if err = s.ensureCustomMCPServerNameAvailable(ctx, ownerUserID, "", server.Name); err != nil {
		return nil, err
	}
	connectorID, err := newCustomMCPConnectorID()
	if err != nil {
		return nil, err
	}
	if err = s.storeCustomMCPServer(ctx, ownerUserID, connectorID, server); err != nil {
		return nil, err
	}
	item := redactCustomMCPServer(connectorID, server)
	return &item, nil
}

// GetCustomMCPServer 返回一条 owner 级脱敏自定义 MCP 配置。
func (s *Service) GetCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*CustomMCPServer, error) {
	record, err := s.loadCustomMCPServerRecord(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	if record.server == nil {
		item := recoveryCustomMCPServer(record.connection)
		return &item, nil
	}
	item := redactCustomMCPServer(strings.TrimSpace(connectorID), *record.server)
	return &item, nil
}

// UpdateCustomMCPServer 更新一条 owner 级自定义 MCP 配置。
func (s *Service) UpdateCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	input CustomMCPServerInput,
) (*CustomMCPServer, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	unlock := s.lockConnectorMutation(ownerUserID, customMCPCatalogLockID)
	defer unlock()

	record, err := s.loadCustomMCPServerRecord(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	previous := record.server
	server, err := materializeCustomMCPServer(input, previous)
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrCustomMCPServerInvalid, err)
	}
	if err = s.ensureCustomMCPServerNameAvailable(
		ctx,
		ownerUserID,
		strings.TrimSpace(connectorID),
		server.Name,
	); err != nil {
		return nil, err
	}
	server.Enabled = record.connection.AvailabilityEnabled.Bool
	if err = s.storeCustomMCPServer(ctx, ownerUserID, connectorID, server); err != nil {
		return nil, err
	}
	item := redactCustomMCPServer(connectorID, server)
	return &item, nil
}

// SetCustomMCPServerEnabled 控制自定义 MCP 是否进入 Connector 选择面与 runtime。
func (s *Service) SetCustomMCPServerEnabled(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	enabled bool,
) (*CustomMCPServer, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	unlock := s.lockConnectorMutation(ownerUserID, customMCPCatalogLockID)
	defer unlock()

	record, err := s.loadCustomMCPServerRecord(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	if record.server == nil {
		return nil, ErrCustomMCPServerRecoveryRequired
	}
	record.server.Enabled = enabled
	if err = s.storeCustomMCPServer(ctx, ownerUserID, connectorID, *record.server); err != nil {
		return nil, err
	}
	item := redactCustomMCPServer(strings.TrimSpace(connectorID), *record.server)
	return &item, nil
}

// DeleteCustomMCPServer 删除自定义 MCP 配置。
func (s *Service) DeleteCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) error {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	connectorID = strings.TrimSpace(connectorID)
	unlock := s.lockConnectorMutation(ownerUserID, customMCPCatalogLockID)
	defer unlock()
	if _, err := s.loadCustomMCPConnectionRecord(ctx, ownerUserID, connectorID); err != nil {
		return err
	}
	_, err := s.mutateConnector(ctx, ownerUserID, connectorID, nil, func(tx *sql.Tx) error {
		query := fmt.Sprintf(
			"DELETE FROM connector_connections WHERE owner_user_id = %s AND connector_id = %s",
			s.bind(1),
			s.bind(2),
		)
		_, deleteErr := tx.ExecContext(ctx, query, ownerUserID, connectorID)
		return deleteErr
	})
	return err
}

// LoadActiveCustomMCPServer 返回 runtime 需要的完整配置；非自定义 ID 返回空结果。
func (s *Service) LoadActiveCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (string, map[string]any, error) {
	if !IsCustomMCPConnectorID(connectorID) {
		return "", nil, nil
	}
	server, err := s.loadStoredCustomMCPServer(ctx, ownerUserID, connectorID)
	if errors.Is(err, ErrCustomMCPServerNotFound) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if !server.Enabled {
		return "", nil, nil
	}
	return server.Name, server.runtimeConfig(), nil
}

func (s *Service) storeCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
	server storedCustomMCPServer,
) error {
	payload, err := json.Marshal(server)
	if err != nil {
		return err
	}
	return s.upsertConnection(ctx, connectionRecord{
		OwnerUserID:         ownerUserID,
		ConnectorID:         strings.TrimSpace(connectorID),
		State:               customMCPConnectionState(server.Enabled),
		AvailabilityEnabled: sql.NullBool{Bool: server.Enabled, Valid: true},
		Credentials:         string(payload),
		AuthType:            customMCPAuthType,
	})
}

func customMCPConnectionState(enabled bool) string {
	if enabled {
		return "connected"
	}
	return "disconnected"
}

func (s *Service) listCustomMCPServerRecords(
	ctx context.Context,
	ownerUserID string,
) ([]customMCPServerRecord, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		`SELECT owner_user_id, connector_id, state, enabled, credentials, credentials_encrypted,
		        credentials_key_id, auth_type
		   FROM connector_connections
		  WHERE owner_user_id = %s AND auth_type = %s`,
		s.bind(1),
		s.bind(2),
	)
	rows, err := s.db.QueryContext(ctx, query, ownerUserID, customMCPAuthType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]customMCPServerRecord, 0)
	for rows.Next() {
		var record connectionRecord
		if err = rows.Scan(
			&record.OwnerUserID,
			&record.ConnectorID,
			&record.State,
			&record.AvailabilityEnabled,
			&record.Credentials,
			&record.CredentialsEncrypted,
			&record.CredentialsKeyID,
			&record.AuthType,
		); err != nil {
			return nil, err
		}
		item := customMCPServerRecord{connection: record}
		if server, decodeErr := s.decodeStoredCustomMCPServer(record); decodeErr == nil {
			item.server = &server
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadCustomMCPConnectionRecord(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*connectionRecord, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	connectorID = strings.TrimSpace(connectorID)
	if !IsCustomMCPConnectorID(connectorID) {
		return nil, ErrCustomMCPServerNotFound
	}
	query := fmt.Sprintf(
		`SELECT owner_user_id, connector_id, state, enabled, credentials, credentials_encrypted,
		        credentials_key_id, auth_type
		   FROM connector_connections
		  WHERE owner_user_id = %s AND connector_id = %s AND auth_type = %s`,
		s.bind(1),
		s.bind(2),
		s.bind(3),
	)
	var record connectionRecord
	err := s.db.QueryRowContext(
		ctx,
		query,
		ownerUserID,
		connectorID,
		customMCPAuthType,
	).Scan(
		&record.OwnerUserID,
		&record.ConnectorID,
		&record.State,
		&record.AvailabilityEnabled,
		&record.Credentials,
		&record.CredentialsEncrypted,
		&record.CredentialsKeyID,
		&record.AuthType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCustomMCPServerNotFound
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Service) loadCustomMCPServerRecord(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*customMCPServerRecord, error) {
	connection, err := s.loadCustomMCPConnectionRecord(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	record := &customMCPServerRecord{connection: *connection}
	if server, decodeErr := s.decodeStoredCustomMCPServer(*connection); decodeErr == nil {
		record.server = &server
	}
	return record, nil
}

func (s *Service) loadStoredCustomMCPServer(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*storedCustomMCPServer, error) {
	record, err := s.loadCustomMCPServerRecord(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	if record.server == nil {
		return nil, ErrCustomMCPServerRecoveryRequired
	}
	return record.server, nil
}

func (s *Service) decodeStoredCustomMCPServer(
	record connectionRecord,
) (storedCustomMCPServer, error) {
	payload, err := s.connectionCredentialsPayload(record)
	if err != nil {
		return storedCustomMCPServer{}, err
	}
	var server storedCustomMCPServer
	if err = json.Unmarshal(payload, &server); err != nil {
		return storedCustomMCPServer{}, fmt.Errorf("解析自定义 MCP 配置: %w", err)
	}
	server.Enabled = record.AvailabilityEnabled.Bool
	if server.AuthType == "" {
		server.AuthType = customMCPAuthNone
		if len(server.Headers) > 0 {
			server.AuthType = customMCPAuthHeaders
		}
	}
	return server, nil
}

func (s *Service) ensureCustomMCPServerNameAvailable(
	ctx context.Context,
	ownerUserID string,
	excludedConnectorID string,
	name string,
) error {
	records, err := s.listCustomMCPServerRecords(ctx, ownerUserID)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.server == nil {
			continue
		}
		if record.connection.ConnectorID != excludedConnectorID && strings.EqualFold(record.server.Name, name) {
			return fmt.Errorf("%w：%s", ErrCustomMCPServerNameConflict, name)
		}
	}
	return nil
}

func materializeCustomMCPServer(
	input CustomMCPServerInput,
	previous *storedCustomMCPServer,
) (storedCustomMCPServer, error) {
	serverType := strings.ToLower(strings.TrimSpace(input.Type))
	server := storedCustomMCPServer{
		Enabled: true,
		Name:    strings.TrimSpace(input.Name),
		Type:    serverType,
	}
	if previous != nil {
		server.Enabled = previous.Enabled
	}
	switch serverType {
	case "stdio":
		env, err := materializeCustomMCPSecrets(
			input.Env,
			previousSecretMap(previous, func(server *storedCustomMCPServer) map[string]string {
				return server.Env
			}),
			previous != nil && previous.Type == serverType,
		)
		if err != nil {
			return storedCustomMCPServer{}, fmt.Errorf("env: %w", err)
		}
		server.Command = strings.TrimSpace(input.Command)
		server.Args = append([]string(nil), input.Args...)
		server.Env = env
	case "http", "sse":
		authType := strings.ToLower(strings.TrimSpace(input.AuthType))
		if authType == "" {
			authType = customMCPAuthNone
			if len(input.Headers) > 0 {
				authType = customMCPAuthHeaders
			}
		}
		preserveAuth := previous != nil && previous.Type == serverType && previous.AuthType == authType
		switch authType {
		case customMCPAuthNone:
		case customMCPAuthBearer:
			token, err := materializeCustomMCPBearerToken(input.BearerToken, previous, preserveAuth)
			if err != nil {
				return storedCustomMCPServer{}, err
			}
			server.BearerToken = token
		case customMCPAuthHeaders:
			headers, err := materializeCustomMCPSecrets(
				input.Headers,
				previousSecretMap(previous, func(server *storedCustomMCPServer) map[string]string {
					return server.Headers
				}),
				preserveAuth,
			)
			if err != nil {
				return storedCustomMCPServer{}, fmt.Errorf("headers: %w", err)
			}
			if len(headers) == 0 {
				return storedCustomMCPServer{}, errors.New("headers: 至少需要一个请求头")
			}
			server.Headers = headers
		default:
			return storedCustomMCPServer{}, fmt.Errorf("auth_type 不支持 %q", authType)
		}
		server.URL = strings.TrimSpace(input.URL)
		server.AuthType = authType
	}
	if _, err := clientopts.MergeAgentMCPServers(nil, map[string]any{
		server.Name: server.runtimeConfig(),
	}); err != nil {
		return storedCustomMCPServer{}, err
	}
	return server, nil
}

func materializeCustomMCPBearerToken(
	input *string,
	previous *storedCustomMCPServer,
	preserve bool,
) (string, error) {
	if input == nil {
		if preserve && previous != nil && previous.BearerToken != "" {
			return previous.BearerToken, nil
		}
		return "", errors.New("bearer_token 不能为空")
	}
	token := strings.TrimSpace(*input)
	if token == "" {
		return "", errors.New("bearer_token 不能为空")
	}
	return token, nil
}

func materializeCustomMCPSecrets(
	input map[string]*string,
	previous map[string]string,
	preserve bool,
) (map[string]string, error) {
	if input == nil {
		if !preserve {
			return nil, nil
		}
		result := make(map[string]string, len(previous))
		for key, value := range previous {
			result[key] = value
		}
		return result, nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		if value == nil {
			previousValue, ok := previous[key]
			if !preserve || !ok {
				return nil, fmt.Errorf("%q 缺少新的值", key)
			}
			result[key] = previousValue
			continue
		}
		result[key] = *value
	}
	return result, nil
}

func previousSecretMap(
	server *storedCustomMCPServer,
	get func(*storedCustomMCPServer) map[string]string,
) map[string]string {
	if server == nil {
		return nil
	}
	return get(server)
}

func (server storedCustomMCPServer) runtimeConfig() map[string]any {
	if server.Type == "stdio" {
		return map[string]any{
			"type": server.Type, "command": server.Command,
			"args": server.Args, "env": server.Env,
		}
	}
	headers := server.Headers
	if server.AuthType == customMCPAuthBearer {
		headers = map[string]string{"Authorization": "Bearer " + server.BearerToken}
	}
	return map[string]any{
		"type": server.Type, "url": server.URL, "headers": headers,
	}
}

func redactCustomMCPServer(
	connectorID string,
	server storedCustomMCPServer,
) CustomMCPServer {
	return CustomMCPServer{
		ConnectorID:        connectorID,
		Enabled:            server.Enabled,
		ConfigurationState: customMCPConfigReady,
		CustomMCPServerInput: CustomMCPServerInput{
			Name: server.Name, Type: server.Type, Command: server.Command,
			Args: append([]string(nil), server.Args...), Env: redactCustomMCPSecrets(server.Env),
			URL: server.URL, AuthType: server.AuthType,
			Headers: redactCustomMCPSecrets(server.Headers),
		},
	}
}

func recoveryCustomMCPServer(record connectionRecord) CustomMCPServer {
	connectorID := strings.TrimSpace(record.ConnectorID)
	suffix := strings.TrimPrefix(connectorID, customMCPConnectorPrefix)
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return CustomMCPServer{
		ConnectorID:        connectorID,
		Enabled:            record.AvailabilityEnabled.Bool,
		ConfigurationState: customMCPConfigRecovery,
		CustomMCPServerInput: CustomMCPServerInput{
			Name: "legacy_mcp_" + suffix,
			Type: "stdio",
		},
	}
}

func redactCustomMCPSecrets(values map[string]string) map[string]*string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]*string, len(values))
	for key := range values {
		result[key] = nil
	}
	return result
}

func customMCPServerConnectorInfo(server CustomMCPServer) Info {
	description := strings.ToUpper(server.Type)
	if server.Type == "stdio" {
		description += " · " + server.Command
	} else {
		description += " · " + server.URL
	}
	return Info{
		ConnectorID:     server.ConnectorID,
		Kind:            ConnectorKindCustomMCP,
		Name:            server.Name,
		Title:           server.Name,
		Description:     description,
		Icon:            "custom-mcp",
		Category:        ConnectorKindCustomMCP,
		AuthType:        customMCPAuthType,
		Status:          "available",
		ConnectionState: "connected",
		IsConfigured:    true,
	}
}

func customMCPServerMatches(server CustomMCPServer, query string) bool {
	fields := []string{
		strings.ToLower(server.Name),
		strings.ToLower(server.Type),
		strings.ToLower(server.Command),
		strings.ToLower(server.URL),
	}
	for _, field := range fields {
		if strings.Contains(field, query) {
			return true
		}
	}
	return false
}

func newCustomMCPConnectorID() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("生成自定义 MCP ID: %w", err)
	}
	return customMCPConnectorPrefix + hex.EncodeToString(buffer), nil
}
