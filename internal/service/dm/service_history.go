package dm

import (
	"context"
	"fmt"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
			merged := dmdomain.MergeRoomBackedSession(*item, *roomSession)
			if !dmdomain.SessionsEqual(*item, merged) {
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
	return s.roomStore.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
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
	current.SessionID = dmdomain.PreferSessionID(current.SessionID, dmdomain.NormalizeString(message["session_id"]))
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

func (s *Service) syncSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	sessionID string,
	runtimeProvider string,
	runtimeModel string,
) (protocol.Session, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	currentSessionID := strings.TrimSpace(dmdomain.StringPointerValue(current.SessionID))
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
