package protocol

import "strings"

const (
	// ContentBlockTypeWorkspaceFileArtifact 表示 Agent 在工作区生成或更新的文件产物。
	ContentBlockTypeWorkspaceFileArtifact = "workspace_file_artifact"
)

const (
	WorkspaceFileArtifactScopeAgentWorkspace = "agent_workspace"
)

const (
	WorkspaceFileArtifactOperationWrite  = "write"
	WorkspaceFileArtifactOperationUpdate = "update"
)

// WorkspaceFileArtifactBlock 描述对话中可直接打开的工作区文件产物。
type WorkspaceFileArtifactBlock struct {
	ID               string `json:"id,omitempty"`
	Type             string `json:"type"`
	Path             string `json:"path"`
	DisplayPath      string `json:"display_path,omitempty"`
	Label            string `json:"label,omitempty"`
	Operation        string `json:"operation,omitempty"`
	Scope            string `json:"scope,omitempty"`
	WorkspaceAgentID string `json:"workspace_agent_id,omitempty"`
	SourceToolUseID  string `json:"source_tool_use_id,omitempty"`
	SourceToolName   string `json:"source_tool_name,omitempty"`
}

// Map 转成动态消息块，供现有 transcript map 写入链路复用。
func (b WorkspaceFileArtifactBlock) Map() map[string]any {
	result := map[string]any{
		"type": ContentBlockTypeWorkspaceFileArtifact,
		"path": strings.TrimSpace(b.Path),
	}
	if value := strings.TrimSpace(b.ID); value != "" {
		result["id"] = value
	}
	if value := strings.TrimSpace(b.DisplayPath); value != "" {
		result["display_path"] = value
	}
	if value := strings.TrimSpace(b.Label); value != "" {
		result["label"] = value
	}
	if value := strings.TrimSpace(b.Operation); value != "" {
		result["operation"] = value
	}
	if value := strings.TrimSpace(b.Scope); value != "" {
		result["scope"] = value
	}
	if value := strings.TrimSpace(b.WorkspaceAgentID); value != "" {
		result["workspace_agent_id"] = value
	}
	if value := strings.TrimSpace(b.SourceToolUseID); value != "" {
		result["source_tool_use_id"] = value
	}
	if value := strings.TrimSpace(b.SourceToolName); value != "" {
		result["source_tool_name"] = value
	}
	return result
}
