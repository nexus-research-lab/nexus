package message

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var workspaceFileArtifactToolPathKeys = []string{"file_path", "path"}

func (p *Processor) workspaceFileArtifactsForToolResult(toolResult map[string]any) []map[string]any {
	if boolValue(toolResult["is_error"]) {
		return nil
	}
	toolUseID := normalizeString(toolResult["tool_use_id"])
	if toolUseID == "" {
		return nil
	}
	toolUse := p.segment.FindToolUse(toolUseID)
	if len(toolUse) == 0 {
		return nil
	}
	toolName := normalizeString(toolUse["name"])
	operation, label, ok := workspaceFileArtifactOperation(toolName)
	if !ok {
		return nil
	}
	input, _ := toolUse["input"].(map[string]any)
	if len(input) == 0 {
		return nil
	}
	path := firstWorkspaceFileArtifactPath(input)
	relativePath := p.normalizeWorkspaceArtifactPath(path)
	if relativePath == "" {
		return nil
	}
	block := protocol.WorkspaceFileArtifactBlock{
		ID:               fmt.Sprintf("workspace_file:%s:%s", toolUseID, relativePath),
		Type:             protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path:             relativePath,
		DisplayPath:      relativePath,
		Label:            label,
		Operation:        operation,
		Scope:            protocol.WorkspaceFileArtifactScopeAgentWorkspace,
		WorkspaceAgentID: p.ctx.AgentID,
		SourceToolUseID:  toolUseID,
		SourceToolName:   toolName,
	}
	return []map[string]any{block.Map()}
}

func workspaceFileArtifactOperation(toolName string) (string, string, bool) {
	switch strings.TrimSpace(toolName) {
	case "Write":
		return protocol.WorkspaceFileArtifactOperationWrite, "生成或更新文件", true
	case "Edit", "MultiEdit", "NotebookEdit":
		return protocol.WorkspaceFileArtifactOperationUpdate, "更新文件", true
	default:
		return "", "", false
	}
}

func firstWorkspaceFileArtifactPath(input map[string]any) string {
	for _, key := range workspaceFileArtifactToolPathKeys {
		value, ok := input[key].(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (p *Processor) normalizeWorkspaceArtifactPath(rawPath string) string {
	normalized := strings.TrimSpace(rawPath)
	if normalized == "" {
		return ""
	}
	normalized = filepath.Clean(normalized)
	if filepath.IsAbs(normalized) {
		return p.relativeWorkspaceArtifactPath(normalized)
	}
	if strings.HasPrefix(normalized, "~") {
		return ""
	}
	relativePath := filepath.ToSlash(strings.TrimPrefix(normalized, "./"))
	if !isSafeRelativeWorkspacePath(relativePath) {
		return ""
	}
	return relativePath
}

func (p *Processor) relativeWorkspaceArtifactPath(absolutePath string) string {
	workspacePath := strings.TrimSpace(p.ctx.WorkspacePath)
	if workspacePath != "" {
		relativePath, err := filepath.Rel(filepath.Clean(workspacePath), absolutePath)
		if err == nil && isSafeRelativeWorkspacePath(relativePath) {
			return filepath.ToSlash(relativePath)
		}
	}
	if relativePath := relativePathFromNexusWorkspacePath(absolutePath); relativePath != "" {
		return relativePath
	}
	return ""
}

func isSafeRelativeWorkspacePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(path))
	return normalized != "." &&
		normalized != ".." &&
		!strings.HasPrefix(normalized, "../") &&
		!strings.HasPrefix(normalized, "/")
}

func relativePathFromNexusWorkspacePath(path string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for index := 0; index < len(parts)-2; index += 1 {
		if parts[index] != ".nexus" || parts[index+1] != "workspace" {
			continue
		}
		relativeParts := parts[index+3:]
		if len(relativeParts) == 0 {
			return ""
		}
		return strings.Join(relativeParts, "/")
	}
	return ""
}
