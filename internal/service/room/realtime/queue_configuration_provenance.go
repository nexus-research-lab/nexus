// INPUT: normalized Room queue items and current Room/Agent/session authority.
// OUTPUT: durable direct-user admissions and one-time dispatch claims.
// POS: Room queue-to-configuration capability trust bridge.
package realtime

import (
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) recordTrustedRoomQueueAdmission(
	ctx context.Context,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
	trusted bool,
) error {
	if !trusted || s == nil || s.queueTrust == nil {
		return nil
	}
	item, ok := authoritativeRoomQueueItem(location, item)
	if !ok || item.Source != protocol.InputQueueSourceUser {
		return nil
	}
	binding, err := queueadmissionstore.NewBinding(location, item)
	if err != nil {
		return err
	}
	principal, ok := authctx.DirectHumanPrincipalBindingFromContext(ctx, binding.OwnerUserID)
	if !ok {
		return errors.New("trusted Room queue admission requires the authenticated owner principal")
	}
	return s.queueTrust.Record(ctx, queueadmissionstore.Admission{
		Binding: binding,
		Principal: queueadmissionstore.PrincipalBinding{
			UserID:     principal.UserID,
			AuthMethod: principal.AuthMethod,
			SessionID:  principal.SessionID,
		},
	})
}

func (s *Service) recordTrustedRoomQueueAdmissions(
	ctx context.Context,
	entries []workspacestore.InputQueueEnqueue,
	items []protocol.InputQueueItem,
	trusted bool,
) error {
	if !trusted || s == nil || s.queueTrust == nil {
		return nil
	}
	if len(entries) != len(items) {
		return errors.New("Room queue admission batch does not match committed items")
	}
	for index := range entries {
		if err := s.recordTrustedRoomQueueAdmission(
			ctx,
			entries[index].Location,
			items[index],
			true,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) rollbackRoomQueueAdmissions(
	ctx context.Context,
	entries []workspacestore.InputQueueEnqueue,
	items []protocol.InputQueueItem,
) {
	for index := range entries {
		if index >= len(items) {
			break
		}
		item, ok := authoritativeRoomQueueItem(entries[index].Location, items[index])
		if ok && s.queueTrust != nil {
			if binding, err := queueadmissionstore.NewBinding(entries[index].Location, item); err == nil {
				_ = s.queueTrust.Revoke(ctx, binding)
			}
		}
		_, _ = s.inputQueue.Delete(entries[index].Location, items[index].ID)
	}
}

func (s *Service) revokeRoomQueueAdmission(
	ctx context.Context,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) error {
	if s == nil || s.queueTrust == nil {
		return nil
	}
	item, ok := authoritativeRoomQueueItem(location, item)
	if !ok || item.Source != protocol.InputQueueSourceUser {
		return nil
	}
	binding, err := queueadmissionstore.NewBinding(location, item)
	if err != nil {
		return err
	}
	return s.queueTrust.Revoke(ctx, binding)
}

func (s *Service) claimTrustedRoomQueueAdmission(
	ctx context.Context,
	sharedSessionKey string,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) (queueadmissionstore.Claim, bool, error) {
	if s == nil || s.queueTrust == nil || item.Source != protocol.InputQueueSourceUser {
		return queueadmissionstore.Claim{}, false, nil
	}
	item, ok := authoritativeRoomQueueItem(location, item)
	if !ok {
		return queueadmissionstore.Claim{}, false, nil
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, location.ConversationID)
	if err != nil {
		return queueadmissionstore.Claim{}, false, err
	}
	if err = requireGroupRoomContext(contextValue); err != nil {
		return queueadmissionstore.Claim{}, false, err
	}
	if strings.TrimSpace(sharedSessionKey) != protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID) ||
		strings.TrimSpace(location.RoomID) != strings.TrimSpace(contextValue.Room.ID) ||
		strings.TrimSpace(location.ConversationID) != strings.TrimSpace(contextValue.Conversation.ID) ||
		strings.TrimSpace(location.OwnerUserID) != strings.TrimSpace(contextValue.Room.OwnerUserID) {
		return queueadmissionstore.Claim{}, false, errors.New("queued Room identity changed before dispatch")
	}
	for _, targetAgentID := range inputQueueTargetAgentIDs(item) {
		if !roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			return queueadmissionstore.Claim{}, false, errors.New("queued Room target is no longer a member")
		}
		agentValue, targetErr := s.agents.GetAgent(ctx, targetAgentID)
		if targetErr != nil {
			return queueadmissionstore.Claim{}, false, targetErr
		}
		if strings.TrimSpace(agentValue.OwnerUserID) != strings.TrimSpace(contextValue.Room.OwnerUserID) {
			return queueadmissionstore.Claim{}, false, errors.New("queued Room target owner changed before dispatch")
		}
	}
	binding, err := queueadmissionstore.NewBinding(location, item)
	if err != nil {
		return queueadmissionstore.Claim{}, false, err
	}
	return s.queueTrust.Claim(ctx, binding)
}

func authoritativeRoomQueueItem(
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) (protocol.InputQueueItem, bool) {
	parsed := protocol.ParseSessionKey(location.SessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.Channel != protocol.SessionChannelWebSocketSegment ||
		parsed.ChatType != "group" ||
		strings.TrimSpace(parsed.Ref) != strings.TrimSpace(location.ConversationID) ||
		strings.TrimSpace(parsed.AgentID) == "" ||
		strings.TrimSpace(location.OwnerUserID) == "" ||
		strings.TrimSpace(location.RoomID) == "" ||
		strings.TrimSpace(location.ConversationID) == "" {
		return protocol.InputQueueItem{}, false
	}
	item.Scope = protocol.InputQueueScopeRoom
	item.SessionKey = strings.TrimSpace(location.SessionKey)
	item.RoomID = strings.TrimSpace(location.RoomID)
	item.ConversationID = strings.TrimSpace(location.ConversationID)
	item.AgentID = strings.TrimSpace(parsed.AgentID)
	item.OwnerUserID = strings.TrimSpace(location.OwnerUserID)
	return item, true
}
