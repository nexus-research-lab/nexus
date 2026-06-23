package memory

import (
	"strings"
	"time"
)

func scoreItem(query string, scope MemoryScope, item MemoryItem) float64 {
	queryTokens := tokenizeText(strings.ToLower(query))
	if len(queryTokens) == 0 {
		return 0
	}
	textTokens := tokenizeText(strings.ToLower(strings.Join([]string{
		item.Title,
		item.Content,
		item.Category,
		item.Source,
		item.Scope,
	}, " ")))
	common := 0
	for token := range queryTokens {
		if _, ok := textTokens[token]; ok {
			common++
		}
	}
	score := float64(common) / float64(len(queryTokens))
	score += scopeBoost(scope, item)
	score += statusBoost(item.Status)
	score += priorityBoost(item.Priority)
	score += recencyBoost(item.CreatedAt)
	if item.AccessCount > 1 {
		score += min(float64(item.AccessCount-1)*0.02, 0.12)
	}
	return score
}

func scopeBoost(scope MemoryScope, item MemoryItem) float64 {
	scopeKey := scope.Key()
	agentID := strings.TrimSpace(scope.AgentID)
	userID := strings.TrimSpace(scope.UserID)
	switch {
	case item.Scope == "":
		return 0
	case scopeKey != "" && item.Scope == scopeKey:
		return 0.35
	case itemScopeAgentID(item.Scope) == agentID && agentID != "":
		return 0.16
	case itemScopeUserID(item.Scope) == userID && userID != "":
		return 0.12
	case sameRoomScope(item.Scope, scope) && scope.Kind == ScopeKindRoomAgentSession:
		return 0.10
	default:
		return 0
	}
}

func scopeCanAccessItem(scope MemoryScope, item MemoryItem) bool {
	itemScope := strings.TrimSpace(item.Scope)
	if itemScope == "" {
		return false
	}
	scopeKey := scope.Key()
	if scopeKey != "" && itemScope == scopeKey {
		return true
	}
	agentID := strings.TrimSpace(scope.AgentID)
	userID := strings.TrimSpace(scope.UserID)
	switch scopeKeyKind(itemScope) {
	case ScopeKindUser:
		return itemScopeUserID(itemScope) == userID && userID != ""
	case ScopeKindAgent:
		return itemScopeAgentID(itemScope) == agentID && agentID != ""
	case ScopeKindDMSession:
		return scope.Kind == ScopeKindAgent &&
			itemScopeAgentID(itemScope) == agentID &&
			agentID != ""
	case ScopeKindRoomAgentSession:
		return scope.Kind == ScopeKindAgent &&
			itemScopeAgentID(itemScope) == agentID &&
			agentID != ""
	case ScopeKindRoomShared:
		return sameRoomScope(itemScope, scope) &&
			(scope.Kind == ScopeKindRoomShared || scope.Kind == ScopeKindRoomAgentSession)
	default:
		return false
	}
}

func scopeKeyKind(scope string) ScopeKind {
	kind, _, _ := strings.Cut(strings.TrimSpace(scope), ":")
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return ""
	}
	return ScopeKind(kind)
}

func itemScopeAgentID(scope string) string {
	parts := strings.Split(strings.TrimSpace(scope), ":")
	if len(parts) == 0 {
		return ""
	}
	switch ScopeKind(strings.TrimSpace(parts[0])) {
	case ScopeKindAgent, ScopeKindDMSession:
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[1])
		}
	case ScopeKindRoomAgentSession:
		if len(parts) >= 4 {
			return strings.TrimSpace(parts[3])
		}
	}
	return ""
}

func itemScopeUserID(scope string) string {
	parts := strings.Split(strings.TrimSpace(scope), ":")
	if len(parts) >= 2 && ScopeKind(strings.TrimSpace(parts[0])) == ScopeKindUser {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func sameRoomScope(itemScope string, scope MemoryScope) bool {
	roomID, conversationID, ok := itemScopeRoomPair(itemScope)
	if !ok {
		return false
	}
	scopeRoomID := strings.TrimSpace(scope.RoomID)
	scopeConversationID := strings.TrimSpace(scope.ConversationID)
	return roomID == scopeRoomID &&
		conversationID == scopeConversationID &&
		roomID != "" &&
		conversationID != ""
}

func itemScopeRoomPair(scope string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(scope), ":")
	if len(parts) == 0 {
		return "", "", false
	}
	switch ScopeKind(strings.TrimSpace(parts[0])) {
	case ScopeKindRoomShared, ScopeKindRoomAgentSession:
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), true
		}
	}
	return "", "", false
}

func statusBoost(status string) float64 {
	switch strings.TrimSpace(status) {
	case "promoted", "active":
		return 0.12
	case "candidate", "needs_confirmation":
		return 0.05
	case "auto", "pending":
		return 0.03
	default:
		return 0
	}
}

func priorityBoost(priority string) float64 {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 0.12
	case "medium":
		return 0.06
	case "low":
		return 0.02
	default:
		return 0
	}
}

func recencyBoost(createdAt time.Time) float64 {
	if createdAt.IsZero() {
		return 0
	}
	age := time.Since(createdAt)
	switch {
	case age <= 24*time.Hour:
		return 0.10
	case age <= 7*24*time.Hour:
		return 0.07
	case age <= 30*24*time.Hour:
		return 0.04
	case age <= 180*24*time.Hour:
		return 0.02
	default:
		return 0
	}
}
