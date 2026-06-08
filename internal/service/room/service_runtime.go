package room

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const conversationRuntimeCloseTimeout = 3 * time.Second

// CloseConversationRuntime 关闭指定 conversation 对应的后台 runtime client。
func (s *Service) CloseConversationRuntime(ctx context.Context, roomID string, conversationID string) error {
	contextValue, err := s.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue.Room.ID != strings.TrimSpace(roomID) {
		return ErrConversationNotFound
	}
	return s.closeConversationRuntimeSessions(ctx, []protocol.ConversationContextAggregate{*contextValue}, true, nil)
}

func (s *Service) closeConversationRuntimeSessions(
	_ context.Context,
	contexts []protocol.ConversationContextAggregate,
	closeSharedSession bool,
	agentFilter map[string]struct{},
) error {
	if s.runtime == nil {
		return nil
	}

	errs := make([]error, 0)
	for _, sessionKey := range roomRuntimeSessionKeys(contexts, closeSharedSession, agentFilter) {
		closeCtx, cancel := context.WithTimeout(context.Background(), conversationRuntimeCloseTimeout)
		err := s.runtime.CloseSession(closeCtx, sessionKey)
		cancel()
		if err != nil && !runtimectx.IsRuntimeTransportClosedError(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func roomRuntimeSessionKeys(
	contexts []protocol.ConversationContextAggregate,
	includeSharedSession bool,
	agentFilter map[string]struct{},
) []string {
	keys := make([]string, 0)
	seen := make(map[string]struct{})
	addKey := func(sessionKey string) {
		sessionKey = strings.TrimSpace(sessionKey)
		if sessionKey == "" {
			return
		}
		if _, exists := seen[sessionKey]; exists {
			return
		}
		seen[sessionKey] = struct{}{}
		keys = append(keys, sessionKey)
	}

	for _, contextValue := range contexts {
		if includeSharedSession {
			addKey(protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID))
		}
		for _, sessionValue := range contextValue.Sessions {
			if len(agentFilter) > 0 {
				if _, ok := agentFilter[sessionValue.AgentID]; !ok {
					continue
				}
			}
			addKey(protocol.BuildRoomAgentSessionKey(
				contextValue.Conversation.ID,
				sessionValue.AgentID,
				contextValue.Room.RoomType,
			))
		}
	}
	return keys
}
