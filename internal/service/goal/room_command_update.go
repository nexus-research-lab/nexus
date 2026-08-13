// INPUT: 服务端已验证的 Room lead 与同一次 Goal replace mutation。
// OUTPUT: 与 objective/budget 同版本提交的 Room ownership。
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
		mutation.changed = true
		mutation.payload["room_lead_agent_id"] = leadAgentID
	}
}
