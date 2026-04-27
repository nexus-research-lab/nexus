package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func (s *Service) ensureSession(
	ctx context.Context,
	agentValue *protocol.Agent,
	parsed protocol.SessionKey,
	sessionKey string,
) (protocol.Session, error) {
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return protocol.Session{}, err
	}
	roomSession, err := s.lookupRoomSession(ctx, parsed)
	if err != nil {
		return protocol.Session{}, err
	}

	if item != nil {
		if roomSession != nil {
			merged := mergeRoomBackedSession(*item, *roomSession)
			if !sessionItemsEqual(*item, merged) {
				updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, merged)
				if updateErr != nil {
					return protocol.Session{}, updateErr
				}
				if updated != nil {
					item = updated
				} else {
					item = &merged
				}
			}
		}
		return *item, nil
	}

	if roomSession != nil {
		updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, *roomSession)
		if updateErr != nil {
			return protocol.Session{}, updateErr
		}
		if updated == nil {
			return protocol.Session{}, fmt.Errorf("创建 room 成员会话失败: %s", sessionKey)
		}
		return *updated, nil
	}

	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "New Chat",
		Options:      map[string]any{},
		IsActive:     true,
	})
	if err != nil {
		return protocol.Session{}, err
	}
	if created == nil {
		return protocol.Session{}, fmt.Errorf("创建 session 失败: %s", sessionKey)
	}
	return *created, nil
}

func (s *Service) lookupRoomSession(
	ctx context.Context,
	parsed protocol.SessionKey,
) (*protocol.Session, error) {
	if s.roomStore == nil {
		return nil, nil
	}
	return s.roomStore.GetRoomSessionByKey(ctx, ownerUserIDFromContext(ctx), parsed)
}

func ownerUserIDFromContext(ctx context.Context) string {
	if userID, ok := authsvc.CurrentUserID(ctx); ok {
		return userID
	}
	return authsvc.SystemUserID
}

func (s *Service) appendRuntimeHistoryMessage(
	workspacePath string,
	sessionValue protocol.Session,
	message protocol.Message,
) error {
	if protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return nil
	}
	return s.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
}

func (s *Service) appendSyntheticHistoryMessage(
	workspacePath string,
	sessionValue protocol.Session,
	message protocol.Message,
) error {
	return s.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
}

func (s *Service) refreshSessionMetaAfterRoundMarker(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) refreshSessionMetaAfterMessage(
	workspacePath string,
	current protocol.Session,
	message protocol.Message,
) (*protocol.Session, error) {
	current.SessionID = preferSessionID(current.SessionID, normalizeString(message["session_id"]))
	current.Status = "active"
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) recordRoundMarker(
	workspacePath string,
	sessionValue protocol.Session,
	roundID string,
	content string,
	deliveryPolicies ...protocol.ChatDeliveryPolicy,
) error {
	var deliveryPolicy string
	if len(deliveryPolicies) > 0 {
		deliveryPolicy = string(deliveryPolicies[0])
	}
	return s.history.AppendRoundMarker(
		workspacePath,
		sessionValue.SessionKey,
		roundID,
		content,
		time.Now().UnixMilli(),
		deliveryPolicy,
	)
}

func buildGuidanceMessage(
	sessionValue protocol.Session,
	targetRoundID string,
	sourceRoundID string,
	content string,
	timestamp int64,
) protocol.Message {
	return message.NewGuidedInputMessage(message.GuidedInputMessageInput{
		SessionKey:    sessionValue.SessionKey,
		AgentID:       sessionValue.AgentID,
		RoundID:       targetRoundID,
		SourceRoundID: sourceRoundID,
		Content:       content,
		SessionID:     stringPointerValue(sessionValue.SessionID),
		Timestamp:     timestamp,
	})
}

func (s *Service) syncSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	sessionID string,
	runtimeProvider string,
	runtimeModel string,
) (protocol.Session, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	currentSessionID := strings.TrimSpace(stringPointerValue(current.SessionID))
	if trimmedSessionID == "" {
		return current, nil
	}
	nextProvider := strings.TrimSpace(runtimeProvider)
	nextModel := strings.TrimSpace(runtimeModel)
	currentProvider, _ := current.Options[protocol.OptionRuntimeProvider].(string)
	currentModel, _ := current.Options[protocol.OptionRuntimeModel].(string)
	sessionIDChanged := currentSessionID != trimmedSessionID
	fingerprintChanged := strings.TrimSpace(currentProvider) != nextProvider ||
		strings.TrimSpace(currentModel) != nextModel
	if !sessionIDChanged && !fingerprintChanged {
		return current, nil
	}
	current.SessionID = &trimmedSessionID
	if current.Options == nil {
		current.Options = map[string]any{}
	}
	current.Options[protocol.OptionRuntimeProvider] = nextProvider
	current.Options[protocol.OptionRuntimeModel] = nextModel
	updated, err := s.files.UpsertSession(workspacePath, current)
	if err != nil {
		return protocol.Session{}, err
	}
	if updated == nil {
		return current, nil
	}
	if sessionIDChanged && s.roomStore != nil && updated.RoomSessionID != nil && strings.TrimSpace(*updated.RoomSessionID) != "" {
		if err := s.roomStore.UpdateRoomSessionSDKSessionID(ctx, strings.TrimSpace(*updated.RoomSessionID), trimmedSessionID); err != nil {
			return protocol.Session{}, err
		}
	}
	return *updated, nil
}

func mergeRoomBackedSession(current protocol.Session, roomSession protocol.Session) protocol.Session {
	merged := roomSession
	if strings.TrimSpace(stringPointerValue(merged.SessionID)) == "" && current.SessionID != nil {
		merged.SessionID = current.SessionID
	}
	return merged
}

func sessionItemsEqual(left protocol.Session, right protocol.Session) bool {
	return left.SessionKey == right.SessionKey &&
		left.AgentID == right.AgentID &&
		stringPointerValue(left.SessionID) == stringPointerValue(right.SessionID) &&
		stringPointerValue(left.RoomSessionID) == stringPointerValue(right.RoomSessionID) &&
		stringPointerValue(left.RoomID) == stringPointerValue(right.RoomID) &&
		stringPointerValue(left.ConversationID) == stringPointerValue(right.ConversationID) &&
		left.ChannelType == right.ChannelType &&
		left.ChatType == right.ChatType &&
		left.Status == right.Status &&
		left.Title == right.Title
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func preferSessionID(current *string, next string) *string {
	if strings.TrimSpace(next) != "" {
		return &next
	}
	return current
}

func normalizeString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
