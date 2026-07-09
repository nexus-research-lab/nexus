package goal

import "github.com/nexus-research-lab/nexus/internal/protocol"

// 这些函数把 Room Goal 存在 metadata 里的键解释成业务判定（谁是负责人、是否要求/已有协作证据）。
// protocol 只负责定义键的词汇（常量）和通用的 typed-map 取值；这层"读键 → 业务语义"的解释属于 goal 域。

// RoomLeadAgentID 返回 Room Goal 的负责人 Agent。
func RoomLeadAgentID(goal protocol.Goal) string {
	return protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataRoomGoalLeadAgentID)
}

// RoomLeadAgentName 返回 Room Goal 的负责人展示名。
func RoomLeadAgentName(goal protocol.Goal) string {
	return protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
}

// RoomCollaborationRequired 判断 Room Goal 是否要求非负责人可见协作。
func RoomCollaborationRequired(goal protocol.Goal) bool {
	return protocol.GoalMetadataBool(goal.Metadata, protocol.GoalMetadataRoomGoalCollaborationRequired)
}

// RoomCollaborationObserved 判断 Room Goal 是否已有非负责人可见协作证据。
func RoomCollaborationObserved(goal protocol.Goal) bool {
	return protocol.GoalMetadataBool(goal.Metadata, protocol.GoalMetadataRoomGoalCollaborationObserved)
}
