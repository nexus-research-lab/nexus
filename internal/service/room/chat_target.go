// INPUT: Room 请求里的显式目标、@mention、默认投递策略与当前活跃 round。
// OUTPUT: 保持显式路由优先，并为无目标 follow-up 选择最近活跃 root round 的稳定 slot 列表。
// POS: Room 用户输入目标解析的唯一真相源；活跃 round 默认优先于 room host。
package room

import (
	"cmp"
	"errors"
	"fmt"
	"sort"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const activeRoundDefaultTargetResolution = "active_round_default"

func (s *RealtimeService) resolveActiveRoomTargets(
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
	targetResolution string,
) ([]string, string) {
	if len(targetAgentIDs) > 0 {
		return targetAgentIDs, targetResolution
	}
	activeAgentIDs := s.latestActiveRootRoundAgentIDs(sessionKey, conversationID)
	if len(activeAgentIDs) == 0 {
		return targetAgentIDs, targetResolution
	}
	return activeAgentIDs, activeRoundDefaultTargetResolution
}

type activeRoomTargetSlot struct {
	agentID      string
	agentRoundID string
	timestampMS  int64
	index        int
}

func (s *RealtimeService) latestActiveRootRoundAgentIDs(sessionKey string, conversationID string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)

	s.mu.Lock()
	defer s.mu.Unlock()

	slotsByRoot := make(map[string][]activeRoomTargetSlot)
	latestTimestampByRoot := make(map[string]int64)
	latestSequenceByRoot := make(map[string]uint64)
	for _, roundValue := range s.activeRounds {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		rootRoundID := roomRootRoundID(roundValue)
		if rootRoundID == "" {
			continue
		}
		if roundValue.registrationSequence > latestSequenceByRoot[rootRoundID] {
			latestSequenceByRoot[rootRoundID] = roundValue.registrationSequence
		}
		for _, slot := range roundValue.Slots {
			if !isActiveDeliverySlot(slot) {
				continue
			}
			slotsByRoot[rootRoundID] = append(slotsByRoot[rootRoundID], activeRoomTargetSlot{
				agentID:      strings.TrimSpace(slot.AgentID),
				agentRoundID: strings.TrimSpace(slot.AgentRoundID),
				timestampMS:  slot.TimestampMS,
				index:        slot.Index,
			})
			if slot.TimestampMS > latestTimestampByRoot[rootRoundID] {
				latestTimestampByRoot[rootRoundID] = slot.TimestampMS
			}
		}
	}

	selectedRoot := ""
	var selectedTimestamp int64
	var selectedSequence uint64
	for rootRoundID, slots := range slotsByRoot {
		if len(slots) == 0 {
			continue
		}
		sequence := latestSequenceByRoot[rootRoundID]
		timestamp := latestTimestampByRoot[rootRoundID]
		if selectedRoot == "" || sequence > selectedSequence ||
			(sequence == selectedSequence && timestamp > selectedTimestamp) ||
			(sequence == selectedSequence && timestamp == selectedTimestamp && rootRoundID < selectedRoot) {
			selectedRoot = rootRoundID
			selectedSequence = sequence
			selectedTimestamp = timestamp
		}
	}
	selected := slotsByRoot[selectedRoot]
	sort.Slice(selected, func(i int, j int) bool {
		if selected[i].timestampMS != selected[j].timestampMS {
			return selected[i].timestampMS < selected[j].timestampMS
		}
		if selected[i].index != selected[j].index {
			return selected[i].index < selected[j].index
		}
		if selected[i].agentID != selected[j].agentID {
			return selected[i].agentID < selected[j].agentID
		}
		return selected[i].agentRoundID < selected[j].agentRoundID
	})
	result := make([]string, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))
	for _, slot := range selected {
		if slot.agentID == "" {
			continue
		}
		if _, ok := seen[slot.agentID]; ok {
			continue
		}
		seen[slot.agentID] = struct{}{}
		result = append(result, slot.agentID)
	}
	return result
}

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
