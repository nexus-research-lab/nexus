package room

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func resolveRoomHostDefaultTarget(
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) (string, bool) {
	if contextValue == nil || !contextValue.Room.HostAutoReplyEnabled {
		return "", false
	}
	hostAgentID := strings.TrimSpace(contextValue.Room.HostAgentID)
	if hostAgentID == "" {
		return "", false
	}
	if _, ok := agentNameByID[hostAgentID]; !ok {
		return "", false
	}
	return hostAgentID, true
}

func initialRoomTriggerType(request ChatRequest, targetResolution string) string {
	if request.Internal && strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation" {
		return "goal_continuation"
	}
	if targetResolution == "room_host_default" {
		return "room_host_default"
	}
	return "public_chat"
}

func shouldBroadcastRoomChatAck(request ChatRequest) bool {
	if !request.Internal {
		return true
	}
	return strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation"
}

func (s *RealtimeService) validateChatRequest(request ChatRequest) (string, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", "", err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", "", errors.New("session_key must be room shared key")
	}
	if strings.TrimSpace(request.RoundID) == "" {
		return "", "", errors.New("round_id is required")
	}
	if !protocol.HasChatInput(request.Content, request.Attachments) &&
		!(request.Internal && strings.TrimSpace(request.GoalContext) != "") {
		return "", "", errors.New("content is required")
	}
	conversationID := cmp.Or(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", "", errors.New("conversation_id is required")
	}
	return sessionKey, conversationID, nil
}

func resolveChatTargetAgentIDs(
	request ChatRequest,
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) ([]string, string, error) {
	if len(request.TargetAgentIDs) > 0 {
		targetAgentIDs := normalizeExplicitTargetAgentIDs(request.TargetAgentIDs)
		if len(targetAgentIDs) == 0 {
			return nil, "", errors.New("target_agent_ids must not be empty")
		}
		for _, agentID := range targetAgentIDs {
			if !roomdomain.IsMemberAgent(contextValue.Members, agentID) {
				return nil, "", fmt.Errorf("target_agent_id is not a room member: %s", agentID)
			}
		}
		return targetAgentIDs, "explicit_target", nil
	}
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(request.Content, reverseAgentNames(agentNameByID))
	return targetAgentIDs, roomTargetResolution(targetAgentIDs), nil
}

func normalizeExplicitTargetAgentIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		agentID := strings.TrimSpace(value)
		if agentID == "" {
			continue
		}
		if _, ok := seen[agentID]; ok {
			continue
		}
		seen[agentID] = struct{}{}
		result = append(result, agentID)
	}
	return result
}
