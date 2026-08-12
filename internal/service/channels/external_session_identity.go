// INPUT: owner 作用域与结构化外部 IM session_key。
// OUTPUT: 不泄露完整账号标识的账号指纹、配对状态与会话对象名称。
// POS: Channel pairing/account 真相到 Session 目录身份投影的唯一适配入口。
package channels

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ResolveExternalSessionIdentity 解析外部 IM Session 当前绑定状态。
func (s *ControlService) ResolveExternalSessionIdentity(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.ExternalSessionIdentity, error) {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, nil
	}
	channelType := normalizeIMChannelType(parsed.Channel)
	if channelType == "" || channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
		return nil, nil
	}
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	pairing, err := s.externalSessionPairing(ctx, ownerUserID, channelType, parsed)
	if err != nil {
		return nil, err
	}
	accountStatus, err := s.externalSessionAccountStatus(
		ctx,
		ownerUserID,
		channelType,
		parsed.AccountID,
	)
	if err != nil {
		return nil, err
	}

	identity := &protocol.ExternalSessionIdentity{
		ChannelType:       channelType,
		AccountHint:       externalAccountHint(parsed.AccountID),
		LegacySessionHint: externalLegacySessionHint(parsed),
		AccountStatus:     accountStatus,
		PairingStatus:     "unpaired",
		CurrentPairing:    false,
		CanDelete:         true,
	}
	if pairing != nil {
		identity.PeerName = nullStringValue(pairing.ExternalName)
		identity.PairingStatus = pairing.Status
		identity.CurrentPairing = pairing.Status == PairingStatusActive &&
			externalSessionAccountAvailable(accountStatus)
		identity.CanDelete = !identity.CurrentPairing
	}
	return identity, nil
}

func externalSessionAccountAvailable(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status != "" &&
		status != "removed" &&
		normalizeChannelConfigStatus(status) != ChannelConfigStatusDisabled
}

func (s *ControlService) externalSessionPairing(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	parsed protocol.SessionKey,
) (*pairingRow, error) {
	rows, err := s.listPairingRows(ctx, ownerUserID, PairingQuery{
		ChannelType: channelType,
		AgentID:     parsed.AgentID,
	})
	if err != nil {
		return nil, err
	}
	wantThreadID := ingressPairingThreadID(parsed.ChatType, parsed.ThreadID)
	var fallback *pairingRow
	for index := range rows {
		row := rows[index]
		if row.ChannelType != channelType ||
			row.AgentID != strings.TrimSpace(parsed.AgentID) ||
			row.ChatType != protocol.NormalizeSessionChatType(parsed.ChatType) ||
			row.ExternalRef != strings.TrimSpace(parsed.Ref) ||
			row.ThreadID != wantThreadID {
			continue
		}
		if row.AccountID == strings.TrimSpace(parsed.AccountID) {
			return &row, nil
		}
		if row.AccountID == "" && usesAccountlessPairingFallback(channelType, parsed.AccountID) {
			copyRow := row
			fallback = &copyRow
		}
	}
	return fallback, nil
}

func (s *ControlService) externalSessionAccountStatus(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID != "" {
		accounts, err := s.listChannelAccountRows(ctx, ownerUserID, channelType)
		if err != nil {
			return "", err
		}
		for _, account := range accounts {
			if account.AccountID == accountID {
				return firstNonEmpty(account.Status, ChannelConfigStatusConnected), nil
			}
		}
		if channelType == ChannelTypeWeixinPersonal {
			return "removed", nil
		}
	}
	config, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return "", err
	}
	if config == nil {
		return "removed", nil
	}
	return firstNonEmpty(config.Status, ChannelConfigStatusConfigured), nil
}

func externalAccountHint(accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(accountID))
	return fmt.Sprintf("%X", digest[:3])
}

func externalLegacySessionHint(parsed protocol.SessionKey) string {
	if strings.TrimSpace(parsed.AccountID) != "" {
		return ""
	}
	return externalAccountHint("legacy:" + strings.TrimSpace(parsed.Raw))
}
