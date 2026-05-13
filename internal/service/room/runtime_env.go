package room

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const (
	nexusRoomIDEnvName              = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName  = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName         = "NEXUS_ROOM_AGENT_ID"
	nexusProjectRootEnvName         = "NEXUS_PROJECT_ROOT"
	nexusRoomInternalAPIBaseEnvName = "NEXUS_ROOM_INTERNAL_API_BASE"
	nexusRoomInternalTokenEnvName   = "NEXUS_ROOM_INTERNAL_TOKEN"
	nexusctlUserIDEnvName           = "NEXUSCTL_USER_ID"
)

func (s *RealtimeService) roomRuntimeEnv(roundValue *activeRoomRound, slot *activeRoomSlot) map[string]string {
	if roundValue == nil || slot == nil {
		return nil
	}
	env := map[string]string{
		nexusRoomIDEnvName:             strings.TrimSpace(roundValue.RoomID),
		nexusRoomConversationIDEnvName: strings.TrimSpace(roundValue.ConversationID),
		nexusRoomAgentIDEnvName:        strings.TrimSpace(slot.AgentID),
		nexusProjectRootEnvName:        strings.TrimSpace(appfs.Root()),
		nexusctlUserIDEnvName:          strings.TrimSpace(roundValue.OwnerUserID),
	}
	if s.internalAPI.BaseURL != "" && s.internalAPI.Token != "" {
		env[nexusRoomInternalAPIBaseEnvName] = s.internalAPI.BaseURL
		env[nexusRoomInternalTokenEnvName] = s.internalAPI.Token
	}
	return env
}
