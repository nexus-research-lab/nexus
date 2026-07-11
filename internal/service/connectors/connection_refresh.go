package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
)

func (s *Service) refreshActiveConnectionIfNeeded(ctx context.Context, ownerUserID string, record connectionRecord) (connectionRecord, error) {
	if record.ConnectorID != "feishu-docx" {
		return record, nil
	}
	payload, err := s.connectionCredentialsPayload(record)
	if err != nil {
		return record, err
	}
	current, err := credentialMapFromPayload(payload)
	if err != nil {
		return record, err
	}
	if !credentialNeedsRefresh(current) {
		return record, nil
	}
	refreshToken := strings.TrimSpace(current["refresh_token"])
	if refreshToken == "" {
		return record, nil
	}
	provider, err := providers.Get(record.ConnectorID)
	if err != nil {
		return record, err
	}
	refreshProvider, ok := provider.(providers.RefreshTokenProvider)
	if !ok {
		return record, nil
	}
	clientID, clientSecret, err := s.oauthCredentials(ctx, ownerUserID, record.ConnectorID)
	if err != nil {
		return record, err
	}
	payload, err = refreshProvider.RefreshToken(ctx, s.httpClient, providers.TokenRefreshRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	})
	if err != nil {
		return record, err
	}
	updated, err := credentialMapFromPayload([]byte(normalizeOAuthPayload(payload)))
	if err != nil {
		return record, err
	}
	for key, value := range current {
		if _, exists := updated[key]; !exists {
			updated[key] = value
		}
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return record, err
	}
	record.Credentials = string(encoded)
	record.CredentialsEncrypted = sql.NullString{}
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: record.ConnectorID,
		State:       "connected",
		Credentials: record.Credentials,
		AuthType:    record.AuthType,
	}); err != nil {
		return record, err
	}
	return record, nil
}
