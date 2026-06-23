package cli

import (
	"strings"

	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	"github.com/spf13/cobra"
)

func parseMemoryFields(values []string) ([]memorysvc.Field, error) {
	items := make([]memorysvc.Field, 0, len(values))
	for _, value := range values {
		key, fieldValue, found := strings.Cut(value, "=")
		if !found {
			return nil, usageErrorf("field 格式错误: %s", value)
		}
		items = append(items, memorysvc.Field{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(fieldValue),
		})
	}
	return items, nil
}

type memoryScopeFlags struct {
	Kind           string
	UserID         string
	AgentID        string
	SessionKey     string
	SessionID      string
	RoomID         string
	ConversationID string
}

func addMemoryScopeFlags(command *cobra.Command, scope *memoryScopeFlags) {
	command.Flags().StringVar(&scope.Kind, "scope-kind", string(memorysvc.ScopeKindAgent), "user|agent|dm_session|room_shared|room_agent_session")
	command.Flags().StringVar(&scope.UserID, "user-id", "", "owner user id")
	command.Flags().StringVar(&scope.AgentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&scope.SessionKey, "session-key", "", "session key")
	command.Flags().StringVar(&scope.SessionID, "session-id", "", "runtime session id")
	command.Flags().StringVar(&scope.RoomID, "room-id", "", "room id")
	command.Flags().StringVar(&scope.ConversationID, "conversation-id", "", "conversation id")
}

func (s memoryScopeFlags) toMemoryScope() memorysvc.MemoryScope {
	return memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKind(strings.TrimSpace(s.Kind)),
		UserID:         strings.TrimSpace(s.UserID),
		AgentID:        strings.TrimSpace(s.AgentID),
		SessionKey:     strings.TrimSpace(s.SessionKey),
		SessionID:      strings.TrimSpace(s.SessionID),
		RoomID:         strings.TrimSpace(s.RoomID),
		ConversationID: strings.TrimSpace(s.ConversationID),
	}
}
