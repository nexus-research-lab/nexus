package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	ChannelConfigStatusConfigured = "configured"
	ChannelConfigStatusConnected  = "connected"
	ChannelConfigStatusPending    = "pending"
	ChannelConfigStatusError      = "error"
	ChannelConfigStatusDisabled   = "disabled"

	PairingStatusPending  = "pending"
	PairingStatusActive   = "active"
	PairingStatusDisabled = "disabled"
	PairingStatusRejected = "rejected"

	PairingSourceManual   = "manual"
	PairingSourceIngress  = "ingress"
	PairingSourceWeChatQR = "wechat_qr"
)

var (
	ErrChannelNotFound         = errors.New("channel not found")
	ErrPairingNotFound         = errors.New("pairing not found")
	ErrPairingApprovalRequired = errors.New("im pairing requires approval")
)

type ChannelCredentialField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Placeholder string `json:"placeholder,omitempty"`
}

type ChannelCatalogItem struct {
	ChannelType       string                   `json:"channel_type"`
	Title             string                   `json:"title"`
	BotLabel          string                   `json:"bot_label"`
	Description       string                   `json:"description"`
	DocsURL           string                   `json:"docs_url,omitempty"`
	RuntimeStatus     string                   `json:"runtime_status"`
	RuntimeNote       string                   `json:"runtime_note,omitempty"`
	SupportsGroup     bool                     `json:"supports_group"`
	SupportsQRCode    bool                     `json:"supports_qr_code"`
	SupportsOAuthLink bool                     `json:"supports_oauth_link"`
	CredentialFields  []ChannelCredentialField `json:"credential_fields"`
}

type ChannelStats struct {
	PairedUserCount  int `json:"paired_user_count"`
	PairedGroupCount int `json:"paired_group_count"`
	PendingCount     int `json:"pending_count"`
}

type ChannelConfigView struct {
	ChannelCatalogItem
	Configured      bool              `json:"configured"`
	ConnectionState string            `json:"connection_state"`
	Status          string            `json:"status"`
	AgentID         string            `json:"agent_id,omitempty"`
	AgentName       string            `json:"agent_name,omitempty"`
	PublicConfig    map[string]string `json:"public_config,omitempty"`
	HasCredentials  bool              `json:"has_credentials"`
	LastError       string            `json:"last_error,omitempty"`
	QRPayload       string            `json:"qr_payload,omitempty"`
	UpdatedAt       *time.Time        `json:"updated_at,omitempty"`
	Stats           ChannelStats      `json:"stats"`
}

type UpsertChannelConfigRequest struct {
	AgentID     string            `json:"agent_id"`
	Config      map[string]string `json:"config"`
	Credentials map[string]string `json:"credentials"`
}

type PairingView struct {
	PairingID     string     `json:"pairing_id"`
	ChannelType   string     `json:"channel_type"`
	ChatType      string     `json:"chat_type"`
	ExternalRef   string     `json:"external_ref"`
	ThreadID      string     `json:"thread_id,omitempty"`
	ExternalName  string     `json:"external_name,omitempty"`
	AgentID       string     `json:"agent_id"`
	AgentName     string     `json:"agent_name,omitempty"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type PairingQuery struct {
	ChannelType string
	Status      string
	AgentID     string
}

type CreatePairingRequest struct {
	ChannelType  string `json:"channel_type"`
	ChatType     string `json:"chat_type"`
	ExternalRef  string `json:"external_ref"`
	ThreadID     string `json:"thread_id,omitempty"`
	ExternalName string `json:"external_name,omitempty"`
	AgentID      string `json:"agent_id"`
	Status       string `json:"status,omitempty"`
	Source       string `json:"source,omitempty"`
}

type UpdatePairingRequest struct {
	AgentID      *string `json:"agent_id,omitempty"`
	Status       *string `json:"status,omitempty"`
	ExternalName *string `json:"external_name,omitempty"`
}

type channelConfigRow struct {
	OwnerUserID          string
	ChannelType          string
	AgentID              string
	Status               string
	ConfigJSON           string
	CredentialsEncrypted sql.NullString
	LastError            sql.NullString
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type pairingRow struct {
	PairingID     string
	OwnerUserID   string
	ChannelType   string
	ChatType      string
	ExternalRef   string
	ThreadID      string
	ExternalName  sql.NullString
	AgentID       string
	Status        string
	Source        string
	LastMessageAt sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type pairingApprovalError struct {
	PairingID string
	Message   string
}

func (e *pairingApprovalError) Error() string {
	return e.Message
}

func (e *pairingApprovalError) Unwrap() error {
	return ErrPairingApprovalRequired
}

type ControlService struct {
	config    config.Config
	db        *sql.DB
	driver    string
	key       []byte
	agents    agentWorkspaceResolver
	router    *Router
	idFactory func(string) string
}

func NewControlService(
	cfg config.Config,
	db *sql.DB,
	agents agentWorkspaceResolver,
	router *Router,
) *ControlService {
	key, err := credentials.DecodeKey(cfg.ConnectorCredentialsKey)
	if err != nil && strings.TrimSpace(cfg.ConnectorCredentialsKey) != "" {
		fmt.Fprintln(os.Stderr, "WARNING: CONNECTOR_CREDENTIALS_KEY 解析失败，IM 通道凭据加密将不可用")
	}
	return &ControlService{
		config:    cfg,
		db:        db,
		driver:    storage.NormalizeSQLDriver(cfg.DatabaseDriver),
		key:       key,
		agents:    agents,
		router:    router,
		idFactory: newDeliveryID,
	}
}

func (s *ControlService) ListChannels(ctx context.Context, ownerUserID string) ([]ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	rows, err := s.listChannelConfigRows(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	stats, err := s.channelStats(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	byType := make(map[string]channelConfigRow, len(rows))
	for _, row := range rows {
		byType[row.ChannelType] = row
	}

	result := make([]ChannelConfigView, 0, len(channelCatalog()))
	for _, catalog := range channelCatalog() {
		catalogStats := stats[catalog.ChannelType]
		if isPlannedChannel(catalog.ChannelType) {
			catalogStats = ChannelStats{}
		}
		view := ChannelConfigView{
			ChannelCatalogItem: catalog,
			ConnectionState:    "not_configured",
			Status:             "not_configured",
			Stats:              catalogStats,
		}
		row, ok := byType[catalog.ChannelType]
		if ok {
			publicConfig, _ := decodeStringMap(row.ConfigJSON)
			view.Configured = true
			view.Status = firstNonEmpty(row.Status, ChannelConfigStatusConfigured)
			view.ConnectionState = s.connectionStateFor(ownerUserID, catalog.ChannelType, view.Status)
			view.AgentID = row.AgentID
			view.AgentName = s.agentName(ctx, row.AgentID)
			view.PublicConfig = publicConfig
			view.HasCredentials = row.CredentialsEncrypted.Valid && strings.TrimSpace(row.CredentialsEncrypted.String) != ""
			view.LastError = nullStringValue(row.LastError)
			view.QRPayload = publicConfig["qr_payload"]
			view.UpdatedAt = &row.UpdatedAt
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *ControlService) CountConfiguredChannels(ctx context.Context, ownerUserID string) (int, error) {
	rows, err := s.listChannelConfigRows(ctx, normalizeChannelOwnerUserID(ownerUserID))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled || isPlannedChannel(row.ChannelType) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *ControlService) CountActivePairings(ctx context.Context, ownerUserID string) (int, error) {
	rows, err := s.listPairingRows(ctx, normalizeChannelOwnerUserID(ownerUserID), PairingQuery{Status: PairingStatusActive})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if isPlannedChannel(row.ChannelType) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *ControlService) UpsertChannelConfig(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	request UpsertChannelConfigRequest,
) (*ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	catalog, ok := channelCatalogByType(channelType)
	if !ok {
		return nil, ErrChannelNotFound
	}
	if isPlannedChannel(channelType) {
		return nil, errors.New("消息渠道未上线")
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return nil, errors.New("agent_id is required")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}

	publicConfig := normalizeStringMap(request.Config)
	secrets := normalizeStringMap(request.Credentials)
	existing, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, err
	}
	if err = validateChannelConfigInput(catalog, publicConfig, secrets, existing != nil && existing.CredentialsEncrypted.Valid); err != nil {
		return nil, err
	}
	credentialsEncrypted := sql.NullString{}
	if len(secrets) > 0 {
		encrypted, encryptErr := s.encryptCredentials(secrets)
		if encryptErr != nil {
			return nil, encryptErr
		}
		credentialsEncrypted = sql.NullString{String: encrypted, Valid: true}
	} else if existing != nil {
		credentialsEncrypted = existing.CredentialsEncrypted
	}

	configJSON, err := encodeStringMap(publicConfig)
	if err != nil {
		return nil, err
	}
	if err = s.upsertChannelConfigRow(ctx, channelConfigRow{
		OwnerUserID:          ownerUserID,
		ChannelType:          channelType,
		AgentID:              agentID,
		Status:               ChannelConfigStatusConfigured,
		ConfigJSON:           configJSON,
		CredentialsEncrypted: credentialsEncrypted,
	}); err != nil {
		return nil, err
	}
	runtimeStatus := ChannelConfigStatusConfigured
	runtimeError := ""
	if err = s.configureRouterChannel(ctx, ownerUserID, channelType, configJSON, credentialsEncrypted); err != nil {
		runtimeStatus = ChannelConfigStatusError
		runtimeError = err.Error()
	} else if s.router != nil && s.router.IsReadyForOwner(ownerUserID, channelType) {
		runtimeStatus = ChannelConfigStatusConnected
	}
	if err = s.updateChannelConfigRuntimeState(ctx, ownerUserID, channelType, runtimeStatus, runtimeError); err != nil {
		return nil, err
	}

	items, err := s.ListChannels(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ChannelType == channelType {
			copyItem := item
			return &copyItem, nil
		}
	}
	return nil, ErrChannelNotFound
}

func (s *ControlService) DeleteChannelConfig(ctx context.Context, ownerUserID string, channelType string) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	query := "DELETE FROM im_channel_configs WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, ownerUserID, channelType)
	if err == nil && s.router != nil {
		s.router.UnregisterForOwner(ctx, ownerUserID, channelType)
	}
	return err
}

func (s *ControlService) ListPairings(ctx context.Context, ownerUserID string, query PairingQuery) ([]PairingView, error) {
	rows, err := s.listPairingRows(ctx, normalizeChannelOwnerUserID(ownerUserID), query)
	if err != nil {
		return nil, err
	}
	result := make([]PairingView, 0, len(rows))
	for _, row := range rows {
		result = append(result, s.pairingView(ctx, row))
	}
	return result, nil
}

func (s *ControlService) CreatePairing(ctx context.Context, ownerUserID string, request CreatePairingRequest) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	row, err := s.buildPairingRow(ctx, ownerUserID, request)
	if err != nil {
		return nil, err
	}
	if err = s.upsertPairingRow(ctx, row); err != nil {
		return nil, err
	}
	created, err := s.getPairingRow(ctx, ownerUserID, row.PairingID)
	if err != nil {
		return nil, err
	}
	view := s.pairingView(ctx, *created)
	return &view, nil
}

func (s *ControlService) UpdatePairing(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	request UpdatePairingRequest,
) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	existing, err := s.getPairingRow(ctx, ownerUserID, strings.TrimSpace(pairingID))
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrPairingNotFound
	}
	if request.AgentID != nil {
		agentID := strings.TrimSpace(*request.AgentID)
		if agentID == "" {
			return nil, errors.New("agent_id cannot be empty")
		}
		if err = s.ensureAgent(ctx, agentID); err != nil {
			return nil, err
		}
		existing.AgentID = agentID
	}
	if request.Status != nil {
		status := normalizePairingStatus(*request.Status, existing.Status)
		if status == "" {
			return nil, errors.New("status is invalid")
		}
		existing.Status = status
	}
	if request.ExternalName != nil {
		existing.ExternalName = sql.NullString{
			String: strings.TrimSpace(*request.ExternalName),
			Valid:  strings.TrimSpace(*request.ExternalName) != "",
		}
	}
	if err = s.upsertPairingRow(ctx, *existing); err != nil {
		return nil, err
	}
	updated, err := s.getPairingRow(ctx, ownerUserID, existing.PairingID)
	if err != nil {
		return nil, err
	}
	view := s.pairingView(ctx, *updated)
	return &view, nil
}

func (s *ControlService) DeletePairing(ctx context.Context, ownerUserID string, pairingID string) error {
	query := "DELETE FROM im_pairings WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	result, err := s.db.ExecContext(ctx, query, normalizeChannelOwnerUserID(ownerUserID), strings.TrimSpace(pairingID))
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrPairingNotFound
	}
	return nil
}

func (s *ControlService) ResolveIngressAgent(ctx context.Context, request IngressRequest) (string, error) {
	channelType := normalizeIMChannelType(request.Channel)
	if channelType == "" || channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
		return strings.TrimSpace(request.AgentID), nil
	}
	if _, ok := channelCatalogByType(channelType); !ok {
		return strings.TrimSpace(request.AgentID), nil
	}

	ownerUserID := normalizeChannelOwnerUserID(firstNonEmpty(request.OwnerUserID, authctx.OwnerUserID(ctx)))
	chatType := protocol.NormalizeSessionChatType(request.ChatType)
	externalRef := strings.TrimSpace(request.Ref)
	if externalRef == "" {
		return strings.TrimSpace(request.AgentID), nil
	}
	threadID := strings.TrimSpace(request.ThreadID)

	active, err := s.findPairingByTarget(ctx, ownerUserID, channelType, chatType, externalRef, threadID, PairingStatusActive)
	if err != nil {
		return "", err
	}
	if active != nil {
		if err = s.touchPairing(ctx, ownerUserID, active.PairingID); err != nil {
			return "", err
		}
		return active.AgentID, nil
	}

	candidateAgentID := strings.TrimSpace(request.AgentID)
	if candidateAgentID == "" {
		candidateAgentID, _ = s.defaultAgentForChannel(ctx, ownerUserID, channelType)
	}
	if candidateAgentID == "" && s.agents != nil {
		if defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx); defaultErr == nil && defaultAgent != nil {
			candidateAgentID = defaultAgent.AgentID
		}
	}
	if candidateAgentID == "" {
		return "", errors.New("channel ingress requires an active pairing or agent_id")
	}

	pending := CreatePairingRequest{
		ChannelType:  channelType,
		ChatType:     chatType,
		ExternalRef:  externalRef,
		ThreadID:     threadID,
		ExternalName: strings.TrimSpace(request.ExternalName),
		AgentID:      candidateAgentID,
		Status:       PairingStatusPending,
		Source:       PairingSourceIngress,
	}
	row, err := s.buildPairingRow(ctx, ownerUserID, pending)
	if err != nil {
		return "", err
	}
	if err = s.upsertPairingRow(ctx, row); err != nil {
		return "", err
	}
	return "", &pairingApprovalError{
		PairingID: row.PairingID,
		Message:   "IM 对象尚未配对授权，请先在配对授权页批准",
	}
}

func (s *ControlService) LoadConfiguredChannels(ctx context.Context) error {
	rows, err := s.listAllChannelConfigRows(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled {
			continue
		}
		if err := s.configureRouterChannel(ctx, row.OwnerUserID, row.ChannelType, row.ConfigJSON, row.CredentialsEncrypted); err != nil {
			_ = s.updateChannelConfigRuntimeState(ctx, row.OwnerUserID, row.ChannelType, ChannelConfigStatusError, err.Error())
		}
	}
	return nil
}

func (s *ControlService) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *ControlService) listChannelConfigRows(ctx context.Context, ownerUserID string) ([]channelConfigRow, error) {
	query := `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs
WHERE owner_user_id = ` + s.bind(1)
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannelConfigRows(rows)
}

func (s *ControlService) listAllChannelConfigRows(ctx context.Context) ([]channelConfigRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannelConfigRows(rows)
}

func (s *ControlService) getChannelConfigRow(ctx context.Context, ownerUserID string, channelType string) (*channelConfigRow, error) {
	query := `
SELECT owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs
WHERE owner_user_id = ` + s.bind(1) + " AND channel_type = " + s.bind(2)
	row := s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(channelType))
	item, err := scanChannelConfigRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) upsertChannelConfigRow(ctx context.Context, row channelConfigRow) error {
	if s.driver == "pgx" {
		query := `
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error
) VALUES ($1, $2, $3, $4, $5, $6, NULL)
ON CONFLICT (owner_user_id, channel_type) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    status = EXCLUDED.status,
    config_json = EXCLUDED.config_json,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    last_error = NULL,
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(ctx, query, row.OwnerUserID, row.ChannelType, row.AgentID, row.Status, row.ConfigJSON, nullableString(row.CredentialsEncrypted.String))
		return err
	}
	query := `
INSERT INTO im_channel_configs (
    owner_user_id, channel_type, agent_id, status, config_json, credentials_encrypted, last_error
) VALUES (?, ?, ?, ?, ?, ?, NULL)
ON CONFLICT(owner_user_id, channel_type) DO UPDATE SET
    agent_id = excluded.agent_id,
    status = excluded.status,
    config_json = excluded.config_json,
    credentials_encrypted = excluded.credentials_encrypted,
    last_error = NULL,
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(ctx, query, row.OwnerUserID, row.ChannelType, row.AgentID, row.Status, row.ConfigJSON, nullableString(row.CredentialsEncrypted.String))
	return err
}

func (s *ControlService) updateChannelConfigRuntimeState(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	status string,
	lastError string,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	status = firstNonEmpty(normalizeChannelConfigStatus(status), ChannelConfigStatusConfigured)
	if s.driver == "pgx" {
		query := `
UPDATE im_channel_configs
SET status = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = $1 AND channel_type = $2`
		_, err := s.db.ExecContext(ctx, query, ownerUserID, channelType, status, nullableString(lastError))
		return err
	}
	query := `
UPDATE im_channel_configs
SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = ? AND channel_type = ?`
	_, err := s.db.ExecContext(ctx, query, status, nullableString(lastError), ownerUserID, channelType)
	return err
}

func scanChannelConfigRows(rows *sql.Rows) ([]channelConfigRow, error) {
	result := []channelConfigRow{}
	for rows.Next() {
		item, err := scanChannelConfigScanner(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanChannelConfigRow(row sqlScanner) (*channelConfigRow, error) {
	return scanChannelConfigScanner(row)
}

func scanChannelConfigScanner(row sqlScanner) (*channelConfigRow, error) {
	var item channelConfigRow
	err := row.Scan(
		&item.OwnerUserID,
		&item.ChannelType,
		&item.AgentID,
		&item.Status,
		&item.ConfigJSON,
		&item.CredentialsEncrypted,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.ChannelType = normalizeIMChannelType(item.ChannelType)
	return &item, nil
}

func (s *ControlService) listPairingRows(ctx context.Context, ownerUserID string, query PairingQuery) ([]pairingRow, error) {
	sqlText := `
SELECT pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1)
	args := []any{strings.TrimSpace(ownerUserID)}
	if channelType := normalizeIMChannelType(query.ChannelType); channelType != "" {
		args = append(args, channelType)
		sqlText += " AND channel_type = " + s.bind(len(args))
	}
	if status := normalizePairingStatus(query.Status, ""); status != "" {
		args = append(args, status)
		sqlText += " AND status = " + s.bind(len(args))
	}
	if agentID := strings.TrimSpace(query.AgentID); agentID != "" {
		args = append(args, agentID)
		sqlText += " AND agent_id = " + s.bind(len(args))
	}
	sqlText += " ORDER BY updated_at DESC, created_at DESC, pairing_id DESC"

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []pairingRow{}
	for rows.Next() {
		item, err := scanPairingScanner(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (s *ControlService) getPairingRow(ctx context.Context, ownerUserID string, pairingID string) (*pairingRow, error) {
	query := `
SELECT pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + " AND pairing_id = " + s.bind(2)
	item, err := scanPairingScanner(s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(pairingID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) findPairingByTarget(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	chatType string,
	externalRef string,
	threadID string,
	status string,
) (*pairingRow, error) {
	query := `
SELECT pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + `
  AND channel_type = ` + s.bind(2) + `
  AND chat_type = ` + s.bind(3) + `
  AND external_ref = ` + s.bind(4) + `
  AND thread_id = ` + s.bind(5) + `
  AND status = ` + s.bind(6) + `
LIMIT 1`
	item, err := scanPairingScanner(s.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		normalizeIMChannelType(channelType),
		protocol.NormalizeSessionChatType(chatType),
		strings.TrimSpace(externalRef),
		strings.TrimSpace(threadID),
		normalizePairingStatus(status, PairingStatusActive),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) upsertPairingRow(ctx context.Context, row pairingRow) error {
	if strings.TrimSpace(row.PairingID) == "" {
		row.PairingID = s.idFactory("pair")
	}
	if s.driver == "pgx" {
		query := `
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (owner_user_id, channel_type, chat_type, external_ref, thread_id) DO UPDATE SET
    external_name = EXCLUDED.external_name,
    agent_id = EXCLUDED.agent_id,
    status = EXCLUDED.status,
    source = EXCLUDED.source,
    last_message_at = COALESCE(EXCLUDED.last_message_at, im_pairings.last_message_at),
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(
			ctx,
			query,
			row.PairingID,
			row.OwnerUserID,
			row.ChannelType,
			row.ChatType,
			row.ExternalRef,
			row.ThreadID,
			nullStringValueOrNil(row.ExternalName),
			row.AgentID,
			row.Status,
			row.Source,
			nullTimeValueOrNil(row.LastMessageAt),
		)
		return err
	}
	query := `
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, channel_type, chat_type, external_ref, thread_id) DO UPDATE SET
    external_name = excluded.external_name,
    agent_id = excluded.agent_id,
    status = excluded.status,
    source = excluded.source,
    last_message_at = COALESCE(excluded.last_message_at, im_pairings.last_message_at),
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(
		ctx,
		query,
		row.PairingID,
		row.OwnerUserID,
		row.ChannelType,
		row.ChatType,
		row.ExternalRef,
		row.ThreadID,
		nullStringValueOrNil(row.ExternalName),
		row.AgentID,
		row.Status,
		row.Source,
		nullTimeValueOrNil(row.LastMessageAt),
	)
	return err
}

func scanPairingScanner(row sqlScanner) (*pairingRow, error) {
	var item pairingRow
	err := row.Scan(
		&item.PairingID,
		&item.OwnerUserID,
		&item.ChannelType,
		&item.ChatType,
		&item.ExternalRef,
		&item.ThreadID,
		&item.ExternalName,
		&item.AgentID,
		&item.Status,
		&item.Source,
		&item.LastMessageAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.ChannelType = normalizeIMChannelType(item.ChannelType)
	item.ChatType = protocol.NormalizeSessionChatType(item.ChatType)
	return &item, nil
}

func (s *ControlService) buildPairingRow(ctx context.Context, ownerUserID string, request CreatePairingRequest) (pairingRow, error) {
	channelType := normalizeIMChannelType(request.ChannelType)
	if _, ok := channelCatalogByType(channelType); !ok {
		return pairingRow{}, ErrChannelNotFound
	}
	chatType := protocol.NormalizeSessionChatType(request.ChatType)
	externalRef := strings.TrimSpace(request.ExternalRef)
	if externalRef == "" {
		return pairingRow{}, errors.New("external_ref is required")
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return pairingRow{}, errors.New("agent_id is required")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return pairingRow{}, err
	}
	status := normalizePairingStatus(request.Status, PairingStatusActive)
	if status == "" {
		return pairingRow{}, errors.New("status is invalid")
	}
	source := normalizePairingSource(request.Source, PairingSourceManual)
	if source == "" {
		return pairingRow{}, errors.New("source is invalid")
	}
	return pairingRow{
		PairingID:    s.idFactory("pair"),
		OwnerUserID:  strings.TrimSpace(ownerUserID),
		ChannelType:  channelType,
		ChatType:     chatType,
		ExternalRef:  externalRef,
		ThreadID:     strings.TrimSpace(request.ThreadID),
		ExternalName: sql.NullString{String: strings.TrimSpace(request.ExternalName), Valid: strings.TrimSpace(request.ExternalName) != ""},
		AgentID:      agentID,
		Status:       status,
		Source:       source,
	}, nil
}

func (s *ControlService) pairingView(ctx context.Context, row pairingRow) PairingView {
	var lastMessageAt *time.Time
	if row.LastMessageAt.Valid {
		value := row.LastMessageAt.Time
		lastMessageAt = &value
	}
	return PairingView{
		PairingID:     row.PairingID,
		ChannelType:   row.ChannelType,
		ChatType:      row.ChatType,
		ExternalRef:   row.ExternalRef,
		ThreadID:      row.ThreadID,
		ExternalName:  nullStringValue(row.ExternalName),
		AgentID:       row.AgentID,
		AgentName:     s.agentName(ctx, row.AgentID),
		Status:        row.Status,
		Source:        row.Source,
		LastMessageAt: lastMessageAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func (s *ControlService) channelStats(ctx context.Context, ownerUserID string) (map[string]ChannelStats, error) {
	query := `
SELECT channel_type, chat_type, status, COUNT(1)
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + `
GROUP BY channel_type, chat_type, status`
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]ChannelStats{}
	for rows.Next() {
		var channelType, chatType, status string
		var count int
		if err = rows.Scan(&channelType, &chatType, &status, &count); err != nil {
			return nil, err
		}
		channelType = normalizeIMChannelType(channelType)
		item := result[channelType]
		if status == PairingStatusPending {
			item.PendingCount += count
		}
		if status == PairingStatusActive && protocol.NormalizeSessionChatType(chatType) == "dm" {
			item.PairedUserCount += count
		}
		if status == PairingStatusActive && protocol.NormalizeSessionChatType(chatType) == "group" {
			item.PairedGroupCount += count
		}
		result[channelType] = item
	}
	return result, rows.Err()
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

func (s *ControlService) touchPairing(ctx context.Context, ownerUserID string, pairingID string) error {
	query := "UPDATE im_pairings SET last_message_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(pairingID))
	return err
}

func (s *ControlService) encryptCredentials(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if len(s.key) == 0 {
		return "", errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 IM 通道凭据")
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return credentials.EncryptPayload(s.key, payload)
}

func (s *ControlService) decryptCredentials(encrypted sql.NullString) (map[string]string, error) {
	if !encrypted.Valid || strings.TrimSpace(encrypted.String) == "" {
		return nil, nil
	}
	if len(s.key) == 0 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 IM 通道凭据")
	}
	payload, err := credentials.DecryptPayload(s.key, encrypted.String)
	if err != nil {
		return nil, err
	}
	var result map[string]string
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return normalizeStringMap(result), nil
}

func (s *ControlService) configureRouterChannel(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	configJSON string,
	encrypted sql.NullString,
) error {
	if s.router == nil {
		return nil
	}
	if isPlannedChannel(channelType) {
		return nil
	}
	_ = configJSON
	secrets, err := s.decryptCredentials(encrypted)
	if err != nil {
		return err
	}
	switch normalizeIMChannelType(channelType) {
	case ChannelTypeTelegram:
		token := strings.TrimSpace(secrets["bot_token"])
		if token == "" {
			return nil
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, newTelegramChannel(token, nil).WithOwner(ownerUserID))
	case ChannelTypeDiscord:
		token := strings.TrimSpace(secrets["bot_token"])
		if token == "" {
			return nil
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, newDiscordChannel(token, nil).WithOwner(ownerUserID))
	default:
		return nil
	}
}

func (s *ControlService) connectionStateFor(ownerUserID string, channelType string, status string) string {
	status = firstNonEmpty(status, ChannelConfigStatusConfigured)
	if status == ChannelConfigStatusDisabled {
		return "disabled"
	}
	if status == ChannelConfigStatusError {
		return "error"
	}
	if s.router != nil && s.router.IsReadyForOwner(ownerUserID, channelType) {
		return "connected"
	}
	if status == ChannelConfigStatusConnected {
		return ChannelConfigStatusConfigured
	}
	return status
}

func validateChannelConfigInput(
	catalog ChannelCatalogItem,
	publicConfig map[string]string,
	secrets map[string]string,
	hasExistingCredentials bool,
) error {
	for _, field := range catalog.CredentialFields {
		if !field.Required {
			continue
		}
		if field.Secret {
			if strings.TrimSpace(secrets[field.Key]) == "" && !hasExistingCredentials {
				return fmt.Errorf("%s is required", field.Key)
			}
			continue
		}
		if strings.TrimSpace(publicConfig[field.Key]) == "" {
			return fmt.Errorf("%s is required", field.Key)
		}
	}
	return nil
}

func normalizeIMChannelType(channelType string) string {
	return protocol.NormalizeStoredChannelType(channelType)
}

func normalizeChannelConfigStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ChannelConfigStatusConfigured,
		ChannelConfigStatusConnected,
		ChannelConfigStatusPending,
		ChannelConfigStatusError,
		ChannelConfigStatusDisabled:
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func normalizeChannelOwnerUserID(ownerUserID string) string {
	if strings.TrimSpace(ownerUserID) == "" {
		return authctx.SystemUserID
	}
	return strings.TrimSpace(ownerUserID)
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		result[normalizedKey] = strings.TrimSpace(value)
	}
	return result
}

func encodeStringMap(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringMap(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return normalizeStringMap(result), nil
}

func normalizePairingStatus(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PairingStatusPending, PairingStatusActive, PairingStatusDisabled, PairingStatusRejected:
		return strings.ToLower(strings.TrimSpace(value))
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func normalizePairingSource(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PairingSourceManual, PairingSourceIngress, PairingSourceWeChatQR:
		return strings.ToLower(strings.TrimSpace(value))
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func nullStringValueOrNil(value sql.NullString) any {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return strings.TrimSpace(value.String)
}

func nullTimeValueOrNil(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func channelCatalog() []ChannelCatalogItem {
	return []ChannelCatalogItem{
		{
			ChannelType:   ChannelTypeDingTalk,
			Title:         "钉钉",
			BotLabel:      "钉钉机器人",
			Description:   "未上线",
			DocsURL:       "https://open.dingtalk.com/",
			RuntimeStatus: "planned",
			RuntimeNote:   "未上线：消息渠道接入将在后续版本补充",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "client_id", Label: "Client ID（AppKey）", Kind: "text", Required: true, Placeholder: "填写开发者控制台的 Client ID"},
				{Key: "client_secret", Label: "Client Secret（AppSecret）", Kind: "password", Required: true, Secret: true, Placeholder: "填写开发者控制台的 Client Secret"},
			},
		},
		{
			ChannelType:      ChannelTypeWeChat,
			Title:            "微信",
			BotLabel:         "微信 ClawBot",
			Description:      "未上线",
			RuntimeStatus:    "planned",
			RuntimeNote:      "未上线：消息渠道接入将在后续版本补充",
			SupportsQRCode:   false,
			CredentialFields: []ChannelCredentialField{},
		},
		{
			ChannelType:   ChannelTypeFeishu,
			Title:         "飞书",
			BotLabel:      "飞书机器人",
			Description:   "未上线",
			DocsURL:       "https://open.feishu.cn/",
			RuntimeStatus: "planned",
			RuntimeNote:   "未上线：消息渠道接入将在后续版本补充",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "app_id", Label: "App ID", Kind: "text", Required: true, Placeholder: "例如 cli_xxxxxxxxx"},
				{Key: "app_secret", Label: "App Secret", Kind: "password", Required: true, Secret: true, Placeholder: "填写应用 App Secret"},
			},
		},
		{
			ChannelType:   ChannelTypeTelegram,
			Title:         "Telegram",
			BotLabel:      "Telegram Bot",
			Description:   "未上线",
			DocsURL:       "https://core.telegram.org/bots",
			RuntimeStatus: "planned",
			RuntimeNote:   "未上线：消息渠道接入将在后续版本补充",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "bot_token", Label: "Bot Token", Kind: "password", Required: true, Secret: true, Placeholder: "粘贴来自 @BotFather 的 Token"},
			},
		},
		{
			ChannelType:       ChannelTypeDiscord,
			Title:             "Discord",
			BotLabel:          "Discord Bot",
			Description:       "未上线",
			DocsURL:           "https://discord.com/developers/applications",
			RuntimeStatus:     "planned",
			RuntimeNote:       "未上线：消息渠道接入将在后续版本补充",
			SupportsGroup:     true,
			SupportsOAuthLink: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "application_id", Label: "Application ID", Kind: "text", Required: true, Placeholder: "例如 1470267845714645176"},
				{Key: "bot_token", Label: "Bot Token", Kind: "password", Required: true, Secret: true, Placeholder: "从 Bot 页面复制 Token"},
			},
		},
	}
}

func channelCatalogByType(channelType string) (ChannelCatalogItem, bool) {
	channelType = normalizeIMChannelType(channelType)
	for _, item := range channelCatalog() {
		if item.ChannelType == channelType {
			return item, true
		}
	}
	return ChannelCatalogItem{}, false
}

func isPlannedChannel(channelType string) bool {
	item, ok := channelCatalogByType(channelType)
	return ok && item.RuntimeStatus == "planned"
}

func sortedChannelTypes() []string {
	items := make([]string, 0, len(channelCatalog()))
	for _, item := range channelCatalog() {
		items = append(items, item.ChannelType)
	}
	sort.Strings(items)
	return items
}
