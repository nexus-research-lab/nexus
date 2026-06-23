package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

const (
	nexusRoomIDEnvName             = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName        = "NEXUS_ROOM_AGENT_ID"
	nexusctlUserIDEnvName          = "NEXUSCTL_USER_ID"
)

func (s *RealtimeService) roomRuntimeEnv(roundValue *activeRoomRound, slot *activeRoomSlot) map[string]string {
	if roundValue == nil || slot == nil {
		return nil
	}
	env := map[string]string{
		nexusRoomIDEnvName:             strings.TrimSpace(roundValue.RoomID),
		nexusRoomConversationIDEnvName: strings.TrimSpace(roundValue.ConversationID),
		nexusRoomAgentIDEnvName:        strings.TrimSpace(slot.AgentID),
		nexusctlUserIDEnvName:          strings.TrimSpace(roundValue.OwnerUserID),
	}
	return env
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

func (s *RealtimeService) runtimeImagegenDefaultEnabled(ctx context.Context) bool {
	resolver, ok := s.providers.(imagegenDefaultResolver)
	if !ok || resolver == nil {
		return false
	}
	_, err := resolver.ResolveImageConfig(ctx, "")
	return err == nil
}

func (s *RealtimeService) resolveAgentRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (runtimeselectionsvc.Selection, error) {
	ownerUserIDs := []string(nil)
	if roundValue != nil {
		ownerUserIDs = append(ownerUserIDs, roundValue.OwnerUserID)
	}
	return runtimeselectionsvc.NewService(s.prefs).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:        agentValue,
		OwnerUserIDs: ownerUserIDs,
	})
}
