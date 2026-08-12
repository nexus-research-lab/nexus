// INPUT: 服务端已验证的 Room lead/成员规模与同一次 Goal replace mutation。
// OUTPUT: 与 objective/budget 同版本提交的 Room ownership 和协作完成门槛。
// POS: UI `/goal` 与 set_goal 替换共享 Goal 时 server-only metadata 的合并边界。
package goal

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func applyServerRoomGoalUpdate(
	item *protocol.Goal,
	request protocol.UpdateGoalRequest,
	mutation *goalUpdateMutation,
) {
	if item == nil || mutation == nil || !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return
	}
	leadAgentID := strings.TrimSpace(request.RoomLeadAgentID)
	leadAgentName := strings.TrimSpace(request.RoomLeadAgentName)
	if leadAgentID != "" &&
		(RoomLeadAgentID(*item) != leadAgentID || RoomLeadAgentName(*item) != leadAgentName) {
		previousLeadAgentID := RoomLeadAgentID(*item)
		item.Metadata = cloneMap(item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata[protocol.GoalMetadataRoomGoalScope] = "room"
		item.Metadata[protocol.GoalMetadataRoomGoalLeadAgentID] = leadAgentID
		if leadAgentName == "" {
			delete(item.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
		} else {
			item.Metadata[protocol.GoalMetadataRoomGoalLeadAgentName] = leadAgentName
		}
		if previousLeadAgentID != leadAgentID {
			clearRoomGoalCollaborationEvidence(item.Metadata)
		}
		mutation.changed = true
		mutation.payload["room_lead_agent_id"] = leadAgentID
	}
	if request.RoomCollaborationRequired == nil {
		return
	}
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	required := *request.RoomCollaborationRequired
	roundID := strings.TrimSpace(request.RoomCollaborationRoundID)
	changed := RoomCollaborationRequired(*item) != required
	if required {
		item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired] = true
		if roundID != "" && protocol.GoalMetadataString(
			item.Metadata,
			protocol.GoalMetadataRoomGoalCollaborationRequirementRound,
		) != roundID {
			item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequirementRound] = roundID
			changed = true
		}
	} else {
		for _, key := range []string{
			protocol.GoalMetadataRoomGoalCollaborationRequired,
			protocol.GoalMetadataRoomGoalCollaborationRequirementRound,
			protocol.GoalMetadataRoomGoalCollaborationObserved,
			protocol.GoalMetadataRoomGoalCollaborationAgentID,
			protocol.GoalMetadataRoomGoalCollaborationRoundID,
			protocol.GoalMetadataRoomGoalCollaborationObservedAt,
		} {
			if _, exists := item.Metadata[key]; exists {
				delete(item.Metadata, key)
				changed = true
			}
		}
	}
	if changed {
		mutation.changed = true
		mutation.payload["room_collaboration_required"] = required
	}
}

func clearRoomGoalCollaborationEvidence(metadata map[string]any) {
	for _, key := range []string{
		protocol.GoalMetadataRoomGoalCollaborationObserved,
		protocol.GoalMetadataRoomGoalCollaborationAgentID,
		protocol.GoalMetadataRoomGoalCollaborationRoundID,
		protocol.GoalMetadataRoomGoalCollaborationObservedAt,
	} {
		delete(metadata, key)
	}
}
