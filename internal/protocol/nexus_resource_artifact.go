// INPUT: Nexus CLI 成功创建 Agent/Room 后返回的结构化资源数据。
// OUTPUT: 可持久化到消息内容中的 Nexus 资源卡片及可选首次进入动作协议块。
// POS: 真实系统资源操作到对话可点击产物之间的协议真相源。
package protocol

const (
	ContentBlockTypeNexusResourceArtifact = "nexus_resource_artifact"
	NexusResourceKindAgent                = "agent"
	NexusResourceKindRoom                 = "room"
)

// NexusResourceArtifactBlock 描述一次真实创建成功后可从对话打开的 Nexus 资源。
type NexusResourceArtifactBlock struct {
	ID                    string                        `json:"id"`
	Type                  string                        `json:"type"`
	ResourceKind          string                        `json:"resource_kind"`
	ResourceID            string                        `json:"resource_id"`
	Name                  string                        `json:"name"`
	Description           string                        `json:"description,omitempty"`
	Avatar                string                        `json:"avatar,omitempty"`
	ConversationID        string                        `json:"conversation_id,omitempty"`
	Members               []NexusResourceArtifactMember `json:"members,omitempty"`
	VibeTags              []string                      `json:"vibe_tags,omitempty"`
	InitialMessage        string                        `json:"initial_message,omitempty"`
	InitialTargetAgentIDs []string                      `json:"initial_target_agent_ids,omitempty"`
	SourceToolUseID       string                        `json:"source_tool_use_id,omitempty"`
	SourceToolName        string                        `json:"source_tool_name,omitempty"`
}

// NexusResourceArtifactMember 是 Room 卡片展示所需的最小成员快照。
type NexusResourceArtifactMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

func (b NexusResourceArtifactBlock) Map() map[string]any {
	result := map[string]any{
		"id":            b.ID,
		"type":          ContentBlockTypeNexusResourceArtifact,
		"resource_kind": b.ResourceKind,
		"resource_id":   b.ResourceID,
		"name":          b.Name,
	}
	if b.Description != "" {
		result["description"] = b.Description
	}
	if b.Avatar != "" {
		result["avatar"] = b.Avatar
	}
	if b.ConversationID != "" {
		result["conversation_id"] = b.ConversationID
	}
	if len(b.Members) > 0 {
		result["members"] = b.Members
	}
	if len(b.VibeTags) > 0 {
		result["vibe_tags"] = b.VibeTags
	}
	if b.InitialMessage != "" {
		result["initial_message"] = b.InitialMessage
	}
	if len(b.InitialTargetAgentIDs) > 0 {
		result["initial_target_agent_ids"] = b.InitialTargetAgentIDs
	}
	if b.SourceToolUseID != "" {
		result["source_tool_use_id"] = b.SourceToolUseID
	}
	if b.SourceToolName != "" {
		result["source_tool_name"] = b.SourceToolName
	}
	return result
}
