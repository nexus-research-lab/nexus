package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// GetAuthURL 生成 OAuth 授权地址。
func (s *Service) GetAuthURL(ctx context.Context, ownerUserID string, connectorID string, redirectURI string, extras map[string]string) (*AuthURLResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	if err := s.purgeExpiredStates(ctx); err != nil {
		return nil, err
	}
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	normalizedExtras := normalizeExtras(extras)
	for _, key := range provider.RequiredExtraKeys() {
		if strings.TrimSpace(normalizedExtras[key]) == "" {
			return nil, fmt.Errorf("%s 参数缺失", key)
		}
	}
	clientID, _, configErr := s.oauthCredentials(ctx, ownerUserID, entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	resolvedRedirectURI := strings.TrimSpace(redirectURI)
	if resolvedRedirectURI == "" {
		resolvedRedirectURI = s.config.ConnectorOAuthRedirectURI
	}
	if err := s.validateRedirectURI(resolvedRedirectURI); err != nil {
		return nil, err
	}
	redirectKind := oauthRedirectKind(resolvedRedirectURI)
	var verifier string
	var challenge string
	if provider.RequiresPKCE() {
		verifier, challenge, err = providers.GeneratePKCE()
		if err != nil {
			return nil, err
		}
	}
	state, err := providers.RandomState()
	if err != nil {
		return nil, err
	}
	extraJSON, err := json.Marshal(normalizedExtras)
	if err != nil {
		return nil, err
	}
	if err = s.insertState(ctx, stateRow{
		OwnerUserID:  ownerUserID,
		State:        state,
		ConnectorID:  entry.ConnectorID,
		CodeVerifier: verifier,
		RedirectURI:  resolvedRedirectURI,
		RedirectKind: redirectKind,
		ShopDomain:   normalizedExtras["shop"],
		ExtraJSON:    string(extraJSON),
		ExpiresAt:    time.Now().Add(s.oauthStateTTL()),
	}); err != nil {
		return nil, err
	}
	authURL, err := provider.BuildAuthURL(ctx, providers.AuthRequest{
		ClientID:     clientID,
		RedirectURI:  resolvedRedirectURI,
		Scopes:       entry.Scopes,
		State:        state,
		CodeVerifier: challenge,
		Extra:        normalizedExtras,
	})
	if err != nil {
		return nil, err
	}
	return &AuthURLResult{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// CompleteOAuthCallback 完成 OAuth token 交换。
func (s *Service) CompleteOAuthCallback(ctx context.Context, ownerUserID string, request OAuthCallbackRequest) (*Info, error) {
	requestOwnerUserID := strings.TrimSpace(ownerUserID)
	if requestOwnerUserID == "" {
		if userID, ok := authctx.CurrentUserID(ctx); ok {
			requestOwnerUserID = userID
		}
	}
	stateValue := strings.TrimSpace(request.State)
	state, err := s.consumeState(ctx, requestOwnerUserID, stateValue)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, errors.New("OAuth state 无效或已过期")
	}
	if state.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("OAuth state 无效或已过期")
	}
	ownerUserID = normalizeConnectorOwnerUserID(ctx, state.OwnerUserID)
	entry, ok := getConnector(state.ConnectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	requestRedirectURI := strings.TrimSpace(request.RedirectURI)
	if requestRedirectURI != "" && state.RedirectURI != "" && requestRedirectURI != state.RedirectURI {
		return nil, errors.New("redirect URI 不匹配")
	}
	if err := s.validateRedirectURI(connectorFirstNonEmpty(requestRedirectURI, state.RedirectURI)); err != nil {
		return nil, err
	}
	extra, err := state.extra()
	if err != nil {
		return nil, err
	}
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	clientID, clientSecret, configErr := s.oauthCredentials(ctx, ownerUserID, entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	payload, err := provider.ExchangeToken(ctx, s.httpClient, providers.TokenRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  state.RedirectURI,
		Code:         strings.TrimSpace(request.Code),
		CodeVerifier: state.CodeVerifier,
		Extra:        extra,
	})
	if err != nil {
		return nil, err
	}
	credentials := mergeCredentialExtras(normalizeOAuthPayload(payload), extra)
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &info, nil
}

func (s *Service) insertState(ctx context.Context, row stateRow) error {
	ownerUserID := normalizeConnectorOwnerUserID(ctx, row.OwnerUserID)
	query := fmt.Sprintf(
		"INSERT INTO connector_oauth_states (owner_user_id, state, connector_id, code_verifier, redirect_uri, redirect_kind, shop_domain, extra_json, expires_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)",
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
		s.bind(5),
		s.bind(6),
		s.bind(7),
		s.bind(8),
		s.bind(9),
	)
	_, err := s.db.ExecContext(
		ctx,
		query,
		ownerUserID,
		row.State,
		row.ConnectorID,
		emptyStringAsNil(row.CodeVerifier),
		row.RedirectURI,
		connectorFirstNonEmpty(row.RedirectKind, oauthRedirectKind(row.RedirectURI)),
		emptyStringAsNil(row.ShopDomain),
		emptyStringAsNil(row.ExtraJSON),
		row.ExpiresAt,
	)
	return err
}

func (s *Service) consumeState(ctx context.Context, ownerUserID string, state string) (*stateRow, error) {
	if strings.TrimSpace(state) == "" {
		return nil, nil
	}
	normalizedOwnerUserID := strings.TrimSpace(ownerUserID)
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_states WHERE state = %s RETURNING owner_user_id, state, connector_id, code_verifier, redirect_uri, redirect_kind, shop_domain, extra_json, expires_at",
		s.bind(1),
	)
	args := []any{strings.TrimSpace(state)}
	if normalizedOwnerUserID != "" {
		normalizedOwnerUserID = normalizeConnectorOwnerUserID(ctx, normalizedOwnerUserID)
		query = fmt.Sprintf(
			"DELETE FROM connector_oauth_states WHERE owner_user_id = %s AND state = %s RETURNING owner_user_id, state, connector_id, code_verifier, redirect_uri, redirect_kind, shop_domain, extra_json, expires_at",
			s.bind(1),
			s.bind(2),
		)
		args = []any{normalizedOwnerUserID, strings.TrimSpace(state)}
	}
	var row stateRow
	var codeVerifier sql.NullString
	var redirectKind sql.NullString
	var shopDomain sql.NullString
	var extraJSON sql.NullString
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&row.OwnerUserID,
		&row.State,
		&row.ConnectorID,
		&codeVerifier,
		&row.RedirectURI,
		&redirectKind,
		&shopDomain,
		&extraJSON,
		&row.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.CodeVerifier = codeVerifier.String
	row.RedirectKind = connectorFirstNonEmpty(redirectKind.String, oauthRedirectKind(row.RedirectURI))
	row.ShopDomain = shopDomain.String
	row.ExtraJSON = extraJSON.String
	return &row, nil
}

func (s *Service) purgeExpiredStates(ctx context.Context) error {
	query := fmt.Sprintf("DELETE FROM connector_oauth_states WHERE expires_at < %s", s.bind(1))
	_, err := s.db.ExecContext(ctx, query, time.Now())
	return err
}

func (s *Service) oauthStateTTL() time.Duration {
	if s.config.ConnectorOAuthStateTTLSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(s.config.ConnectorOAuthStateTTLSeconds) * time.Second
}

func (s *Service) validateRedirectURI(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("redirect URI 格式不正确")
	}
	for _, allowed := range s.config.ConnectorOAuthAllowedOrigins {
		allowedURL, err := url.Parse(strings.TrimSpace(allowed))
		if err != nil || allowedURL.Scheme == "" || allowedURL.Host == "" {
			continue
		}
		if parsed.Scheme == allowedURL.Scheme && parsed.Host == allowedURL.Host && strings.HasPrefix(parsed.Path, allowedURL.Path) {
			return nil
		}
	}
	return errors.New("redirect URI 不在允许列表中")
}

func oauthRedirectKind(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return oauthRedirectKindWeb
	}
	if strings.EqualFold(parsed.Scheme, "nexus") {
		return oauthRedirectKindDesktop
	}
	return oauthRedirectKindWeb
}

func (row stateRow) extra() (map[string]string, error) {
	result := map[string]string{}
	if strings.TrimSpace(row.ExtraJSON) != "" {
		if err := json.Unmarshal([]byte(row.ExtraJSON), &result); err != nil {
			return nil, errors.New("OAuth state extra 参数格式不正确")
		}
	}
	if result["shop"] == "" && strings.TrimSpace(row.ShopDomain) != "" {
		result["shop"] = row.ShopDomain
	}
	return normalizeExtras(result), nil
}
