// INPUT: normalized DM queue items and the current Agent/session boundary.
// OUTPUT: durable direct-user admission records and one-time dispatch claims.
// POS: DM queue-to-configuration capability trust bridge.
package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) recordTrustedQueueAdmission(
	ctx context.Context,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
	trusted bool,
) error {
	if !trusted {
		return nil
	}
	agentID := inputQueueLocationAgentID(location)
	if !trustedDMWebSocketSession(location.SessionKey, agentID) {
		return nil
	}
	item.AgentID = agentID
	item.SessionKey = strings.TrimSpace(location.SessionKey)
	item.OwnerUserID = strings.TrimSpace(location.OwnerUserID)
	return s.RecordTrustedQueueAdmission(ctx, location, item)
}

func (s *Service) claimTrustedQueueAdmission(
	ctx context.Context,
	normalizedSessionKey string,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) (queueadmissionstore.Claim, bool, error) {
	if s.QueueTrust == nil || item.Source != protocol.InputQueueSourceUser {
		return queueadmissionstore.Claim{}, false, nil
	}
	agentID := inputQueueLocationAgentID(location)
	if !trustedDMWebSocketSession(normalizedSessionKey, agentID) ||
		strings.TrimSpace(location.SessionKey) != strings.TrimSpace(normalizedSessionKey) {
		return queueadmissionstore.Claim{}, false, nil
	}
	agentValue, err := s.Agents.GetAgent(ctx, agentID)
	if err != nil {
		return queueadmissionstore.Claim{}, false, err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != strings.TrimSpace(location.OwnerUserID) {
		return queueadmissionstore.Claim{}, false, errors.New("queued DM agent owner changed before dispatch")
	}
	item.AgentID = agentID
	item.SessionKey = strings.TrimSpace(location.SessionKey)
	item.OwnerUserID = strings.TrimSpace(location.OwnerUserID)
	binding, err := queueadmissionstore.NewBinding(location, item)
	if err != nil {
		return queueadmissionstore.Claim{}, false, err
	}
	return s.QueueTrust.Claim(ctx, binding)
}

func (s *Service) revokeQueueAdmission(
	ctx context.Context,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) error {
	if s.QueueTrust == nil ||
		item.Source != protocol.InputQueueSourceUser {
		return nil
	}
	agentID := inputQueueLocationAgentID(location)
	if agentID == "" {
		return nil
	}
	item.AgentID = agentID
	item.SessionKey = strings.TrimSpace(location.SessionKey)
	item.OwnerUserID = strings.TrimSpace(location.OwnerUserID)
	binding, err := queueadmissionstore.NewBinding(location, item)
	if err != nil {
		return err
	}
	return s.QueueTrust.Revoke(ctx, binding)
}

func inputQueueItemByID(
	items []protocol.InputQueueItem,
	itemID string,
) (protocol.InputQueueItem, bool) {
	itemID = strings.TrimSpace(itemID)
	for _, item := range items {
		if strings.TrimSpace(item.ID) == itemID {
			return item, true
		}
	}
	return protocol.InputQueueItem{}, false
}
