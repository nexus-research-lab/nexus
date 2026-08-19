// INPUT: 与 shell tool_use 匹配的 Nexus CLI JSON tool_result，含可选 Room 首次进入动作。
// OUTPUT: 仅针对真实 Agent/Room 创建成功结果的资源卡片内容块。
// POS: Nexus 管理 Skill 的执行结果到消息资源产物之间的投影层。
package message

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (p *Processor) nexusResourceArtifactForToolResult(
	toolResult map[string]any,
	toolUseID string,
	toolName string,
) *protocol.NexusResourceArtifactBlock {
	if !isNexusResourceArtifactTool(toolName) {
		return nil
	}
	payload := nexusResourcePayloadForToolResult(toolResult)
	if len(payload) == 0 || !payloadSucceeded(payload) {
		return nil
	}
	domain := normalizeString(payload["domain"])
	if normalizeString(payload["action"]) != "create" {
		return nil
	}
	item, _ := payload["item"].(map[string]any)
	if len(item) == 0 {
		return nil
	}

	switch domain {
	case protocol.NexusResourceKindAgent:
		return nexusAgentResourceArtifact(item, toolUseID, toolName)
	case protocol.NexusResourceKindRoom:
		return nexusRoomResourceArtifact(item, payload, toolUseID, toolName)
	default:
		return nil
	}
}

func isNexusResourceArtifactTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "bash", "powershell", "shell", "shell_command":
		return true
	default:
		return false
	}
}

func nexusResourcePayloadForToolResult(toolResult map[string]any) map[string]any {
	for _, value := range []any{
		toolResult["content"],
		toolResult["structured_output"],
		toolResult["tool_use_result"],
		toolResult["metadata"],
	} {
		if payload := firstNexusResourcePayload(nexusResourceResultText(value)); len(payload) > 0 {
			return payload
		}
	}
	return nil
}

func nexusResourceResultText(value any) string {
	if payload, ok := value.(map[string]any); ok {
		parts := make([]string, 0, 6)
		for _, key := range []string{"stdout", "output", "result", "text", "content", "data"} {
			if text := toolResultContentText(payload[key]); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return toolResultContentText(value)
}

func firstNexusResourcePayload(content string) map[string]any {
	for _, candidate := range imagegenJSONCandidates(content) {
		var payload map[string]any
		if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
			continue
		}
		domain := normalizeString(payload["domain"])
		if domain == protocol.NexusResourceKindAgent || domain == protocol.NexusResourceKindRoom {
			return payload
		}
	}
	return nil
}

func payloadSucceeded(payload map[string]any) bool {
	value, exists := payload["success"]
	return !exists || boolValue(value)
}

func nexusAgentResourceArtifact(
	item map[string]any,
	toolUseID string,
	toolName string,
) *protocol.NexusResourceArtifactBlock {
	agentID := normalizeString(item["agent_id"])
	if agentID == "" {
		return nil
	}
	name := firstNonEmpty(normalizeString(item["display_name"]), normalizeString(item["name"]), agentID)
	return &protocol.NexusResourceArtifactBlock{
		ID:              fmt.Sprintf("nexus_resource:agent:%s:%s", toolUseID, agentID),
		Type:            protocol.ContentBlockTypeNexusResourceArtifact,
		ResourceKind:    protocol.NexusResourceKindAgent,
		ResourceID:      agentID,
		Name:            name,
		Description:     normalizeString(item["description"]),
		Avatar:          normalizeString(item["avatar"]),
		VibeTags:        normalizedStringSlice(item["vibe_tags"]),
		SourceToolUseID: toolUseID,
		SourceToolName:  toolName,
	}
}

func nexusRoomResourceArtifact(
	item map[string]any,
	payload map[string]any,
	toolUseID string,
	toolName string,
) *protocol.NexusResourceArtifactBlock {
	room, _ := item["room"].(map[string]any)
	conversation, _ := item["conversation"].(map[string]any)
	roomID := normalizeString(room["id"])
	if roomID == "" {
		return nil
	}
	name := firstNonEmpty(normalizeString(room["name"]), normalizeString(conversation["title"]), roomID)
	return &protocol.NexusResourceArtifactBlock{
		ID:                    fmt.Sprintf("nexus_resource:room:%s:%s", toolUseID, roomID),
		Type:                  protocol.ContentBlockTypeNexusResourceArtifact,
		ResourceKind:          protocol.NexusResourceKindRoom,
		ResourceID:            roomID,
		Name:                  name,
		Description:           normalizeString(room["description"]),
		Avatar:                normalizeString(room["avatar"]),
		ConversationID:        normalizeString(conversation["id"]),
		Members:               nexusRoomResourceArtifactMembers(item["member_agents"]),
		InitialMessage:        normalizeString(payload["initial_message"]),
		InitialTargetAgentIDs: normalizedStringSlice(payload["initial_target_agent_ids"]),
		SourceToolUseID:       toolUseID,
		SourceToolName:        toolName,
	}
}

func nexusRoomResourceArtifactMembers(value any) []protocol.NexusResourceArtifactMember {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	members := make([]protocol.NexusResourceArtifactMember, 0, len(items))
	for _, item := range items {
		member, ok := item.(map[string]any)
		if !ok {
			continue
		}
		memberID := firstNonEmpty(normalizeString(member["agent_id"]), normalizeString(member["id"]))
		if memberID == "" {
			continue
		}
		members = append(members, protocol.NexusResourceArtifactMember{
			ID:     memberID,
			Name:   firstNonEmpty(normalizeString(member["display_name"]), normalizeString(member["name"]), memberID),
			Avatar: normalizeString(member["avatar"]),
		})
	}
	return members
}

func normalizedStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if normalized := strings.TrimSpace(normalizeString(item)); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}
