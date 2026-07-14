package privateview

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	defaultThreadLimit = 50
	defaultEventLimit  = 80
	defaultRoomLimit   = 120
	maxThreadLimit     = 200
	maxEventLimit      = 300
	maxRoomLimit       = 300
	previewRunes       = 120
)

// Query 描述 Agent 私域投影的过滤条件。
type Query struct {
	RoomID         string
	ConversationID string
	Limit          int
	RoomLimit      int
}

type ThreadBuilder struct {
	Thread protocol.AgentPrivateThread
	Events []protocol.AgentPrivateEvent
}

func Project(
	agentID string,
	contexts []protocol.ConversationContextAggregate,
	workspacePath string,
) (map[string]*ThreadBuilder, error) {
	messageStore := workspacestore.NewRoomDirectedMessageStore(workspacePath)
	builders := make(map[string]*ThreadBuilder)
	for _, contextValue := range contexts {
		messages, err := messageStore.ReadMessages(contextValue.Conversation.ID)
		if err != nil {
			return nil, err
		}
		if len(messages) == 0 {
			continue
		}
		participantsByID := participantsByID(contextValue)
		for _, message := range messages {
			event, ok := buildEvent(agentID, contextValue, participantsByID, message)
			if !ok {
				continue
			}
			builder := builders[event.ThreadID]
			if builder == nil {
				builder = &ThreadBuilder{Thread: threadFromEvent(agentID, event)}
				builders[event.ThreadID] = builder
			}
			builder.Events = append(builder.Events, event)
			builder.Thread.MessageCount++
			if event.Timestamp >= builder.Thread.LastTimestamp {
				updateThreadFromEvent(&builder.Thread, event)
			}
		}
	}
	return builders, nil
}

func ThreadLimit(value int) int {
	return normalizeLimit(value, defaultThreadLimit, maxThreadLimit)
}

func EventLimit(value int) int {
	return normalizeLimit(value, defaultEventLimit, maxEventLimit)
}

func RoomLimit(value int) int {
	return normalizeLimit(value, defaultRoomLimit, maxRoomLimit)
}

func buildEvent(
	agentID string,
	contextValue protocol.ConversationContextAggregate,
	participantsByID map[string]protocol.AgentPrivateParticipant,
	message protocol.RoomDirectedMessageRecord,
) (protocol.AgentPrivateEvent, bool) {
	participantIDs := participantIDs(message)
	if !slices.Contains(participantIDs, agentID) {
		return protocol.AgentPrivateEvent{}, false
	}
	sourceAgentID := strings.TrimSpace(message.SourceAgentID)
	messageID := strings.TrimSpace(message.MessageID)
	if sourceAgentID == "" || messageID == "" {
		return protocol.AgentPrivateEvent{}, false
	}

	scope, _ := scope(agentID, participantIDs)
	threadID := threadID(scope, participantIDs)
	return protocol.AgentPrivateEvent{
		MessageID:         messageID,
		ThreadID:          threadID,
		Direction:         direction(agentID, message),
		SourceAgentID:     sourceAgentID,
		Recipients:        normalizedAgents(message.Recipients),
		WakeTargets:       normalizedAgents(message.WakeTargets),
		Content:           strings.TrimSpace(message.Content),
		ReplyRoute:        message.ReplyRoute,
		WakePolicy:        message.WakePolicy,
		DelaySeconds:      message.DelaySeconds,
		CorrelationID:     strings.TrimSpace(message.CorrelationID),
		RootRoundID:       strings.TrimSpace(message.RootRoundID),
		CausedByRoundID:   strings.TrimSpace(message.CausedByRoundID),
		HopIndex:          message.HopIndex,
		RoomID:            contextValue.Room.ID,
		RoomName:          contextValue.Room.Name,
		RoomType:          contextValue.Room.RoomType,
		ConversationID:    contextValue.Conversation.ID,
		ConversationTitle: contextValue.Conversation.Title,
		Participants:      buildParticipants(participantIDs, participantsByID),
		Timestamp:         message.Timestamp,
	}, true
}

func threadFromEvent(agentID string, event protocol.AgentPrivateEvent) protocol.AgentPrivateThread {
	thread := protocol.AgentPrivateThread{
		ThreadID: event.ThreadID,
		AgentID:  agentID,
	}
	updateThreadFromEvent(&thread, event)
	return thread
}

func updateThreadFromEvent(thread *protocol.AgentPrivateThread, event protocol.AgentPrivateEvent) {
	participantIDs := make([]string, 0, len(event.Participants))
	for _, participant := range event.Participants {
		if strings.TrimSpace(participant.AgentID) != "" {
			participantIDs = append(participantIDs, strings.TrimSpace(participant.AgentID))
		}
	}
	scope, peers := scope(thread.AgentID, participantIDs)
	thread.Scope = scope
	thread.ParticipantAgentIDs = participantIDs
	thread.PeerAgentIDs = peers
	thread.Participants = slices.Clone(event.Participants)
	thread.RoomID = event.RoomID
	thread.RoomName = event.RoomName
	thread.RoomType = event.RoomType
	thread.ConversationID = event.ConversationID
	thread.ConversationTitle = event.ConversationTitle
	thread.LastMessageID = event.MessageID
	thread.LastContentPreview = contentPreview(event.Content)
	thread.LastTimestamp = event.Timestamp
}

func participantIDs(message protocol.RoomDirectedMessageRecord) []string {
	ids := make([]string, 0, 1+len(message.Recipients)+len(message.ReplyRoute.Recipients))
	ids = append(ids, strings.TrimSpace(message.SourceAgentID))
	ids = append(ids, message.Recipients...)
	ids = append(ids, message.ReplyRoute.Recipients...)
	return normalizedAgents(ids)
}

func scope(agentID string, participantIDs []string) (string, []string) {
	peers := make([]string, 0, len(participantIDs))
	for _, participantID := range participantIDs {
		if participantID != agentID {
			peers = append(peers, participantID)
		}
	}
	slices.Sort(peers)
	switch len(peers) {
	case 0:
		return "self", peers
	case 1:
		return "direct", peers
	default:
		return "audience", peers
	}
}

func threadID(scope string, participantIDs []string) string {
	hash := sha256.Sum256([]byte(scope + ":" + strings.Join(participantIDs, ",")))
	return "pd_" + hex.EncodeToString(hash[:])[:16]
}

func direction(agentID string, message protocol.RoomDirectedMessageRecord) string {
	if strings.TrimSpace(message.SourceAgentID) == agentID &&
		len(message.Recipients) == 1 &&
		strings.TrimSpace(message.Recipients[0]) == agentID {
		return "self"
	}
	if strings.TrimSpace(message.SourceAgentID) == agentID {
		return "outgoing"
	}
	return "incoming"
}

func participantsByID(
	contextValue protocol.ConversationContextAggregate,
) map[string]protocol.AgentPrivateParticipant {
	participants := make(map[string]protocol.AgentPrivateParticipant, len(contextValue.MemberAgents))
	for _, agent := range contextValue.MemberAgents {
		participants[agent.AgentID] = protocol.AgentPrivateParticipant{
			AgentID: agent.AgentID,
			Name:    agent.Name,
			Avatar:  agent.Avatar,
		}
	}
	return participants
}

func buildParticipants(
	participantIDs []string,
	participantsByID map[string]protocol.AgentPrivateParticipant,
) []protocol.AgentPrivateParticipant {
	participants := make([]protocol.AgentPrivateParticipant, 0, len(participantIDs))
	for _, participantID := range participantIDs {
		participant := participantsByID[participantID]
		if strings.TrimSpace(participant.AgentID) == "" {
			participant = protocol.AgentPrivateParticipant{
				AgentID: participantID,
				Name:    participantID,
			}
		}
		participants = append(participants, participant)
	}
	return participants
}

func normalizedAgents(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	slices.Sort(result)
	return result
}

func contentPreview(content string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	runes := []rune(normalized)
	if len(runes) <= previewRunes {
		return normalized
	}
	return string(runes[:previewRunes]) + "..."
}

func normalizeLimit(value int, defaultValue int, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return min(value, maxValue)
}
