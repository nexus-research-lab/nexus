package message

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var workspaceFileArtifactToolPathKeys = []string{"file_path", "path"}
var imagegenArtifactOutputPrefixes = []string{"{", "["}

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
	if artifact := p.imagegenArtifactForToolResult(toolResult, toolUseID, toolName); artifact != nil {
		return []map[string]any{artifact.Map()}
	}
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
	kind, mimeType := workspaceFileArtifactKindAndMIME(relativePath, "")
	block := protocol.WorkspaceFileArtifactBlock{
		ID:               fmt.Sprintf("workspace_file:%s:%s", toolUseID, relativePath),
		Type:             protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path:             relativePath,
		DisplayPath:      relativePath,
		Label:            label,
		Title:            workspaceFileArtifactTitle(relativePath),
		ArtifactKind:     kind,
		MIMEType:         mimeType,
		Operation:        operation,
		Scope:            protocol.WorkspaceFileArtifactScopeAgentWorkspace,
		WorkspaceAgentID: p.ctx.AgentID,
		SourceToolUseID:  toolUseID,
		SourceToolName:   toolName,
	}
	return []map[string]any{block.Map()}
}

func (p *Processor) imagegenArtifactForToolResult(toolResult map[string]any, toolUseID string, toolName string) *protocol.WorkspaceFileArtifactBlock {
	if strings.TrimSpace(toolName) != "Bash" {
		return nil
	}
	payload := firstImagegenPayload(toolResultContentText(toolResult["content"]))
	if len(payload) == 0 || normalizeString(payload["domain"]) != "imagegen" {
		return nil
	}
	item, _ := payload["item"].(map[string]any)
	if len(item) == 0 {
		return nil
	}
	relativePath := p.normalizeWorkspaceArtifactPath(normalizeString(item["path"]))
	if relativePath == "" {
		return nil
	}
	mimeType := normalizeString(item["mime_type"])
	kind, inferredMIME := workspaceFileArtifactKindAndMIME(relativePath, mimeType)
	if mimeType == "" {
		mimeType = inferredMIME
	}
	return &protocol.WorkspaceFileArtifactBlock{
		ID:               fmt.Sprintf("workspace_file:%s:%s", toolUseID, relativePath),
		Type:             protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path:             relativePath,
		DisplayPath:      relativePath,
		Label:            "生成图片",
		Title:            workspaceFileArtifactTitle(relativePath),
		ArtifactKind:     kind,
		MIMEType:         mimeType,
		Operation:        protocol.WorkspaceFileArtifactOperationWrite,
		Scope:            protocol.WorkspaceFileArtifactScopeAgentWorkspace,
		WorkspaceAgentID: p.ctx.AgentID,
		SourceToolUseID:  toolUseID,
		SourceToolName:   toolName,
	}
}

func firstImagegenPayload(content string) map[string]any {
	for _, candidate := range imagegenJSONCandidates(content) {
		var payload map[string]any
		if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
			continue
		}
		if normalizeString(payload["domain"]) == "imagegen" {
			return payload
		}
	}
	return nil
}

func imagegenJSONCandidates(content string) []string {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return nil
	}
	var candidates []string
	for _, prefix := range imagegenArtifactOutputPrefixes {
		if strings.HasPrefix(trimmedContent, prefix) {
			candidates = append(candidates, trimmedContent)
			break
		}
	}
	for _, line := range strings.Split(trimmedContent, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		for _, prefix := range imagegenArtifactOutputPrefixes {
			if strings.HasPrefix(trimmedLine, prefix) {
				candidates = append(candidates, trimmedLine)
				break
			}
		}
	}
	return candidates
}

func toolResultContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var builder strings.Builder
		for _, item := range typed {
			if text := toolResultContentText(item); strings.TrimSpace(text) != "" {
				if builder.Len() > 0 {
					builder.WriteByte('\n')
				}
				builder.WriteString(text)
			}
		}
		return builder.String()
	case map[string]any:
		return firstNonEmpty(
			rawString(typed["text"]),
			rawString(typed["content"]),
			rawString(typed["data"]),
		)
	default:
		return ""
	}
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

func workspaceFileArtifactKindAndMIME(path string, mimeType string) (string, string) {
	normalizedMIME := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(normalizedMIME, "image/") {
		if normalizedMIME == "image/svg+xml" {
			return protocol.WorkspaceFileArtifactKindSVG, normalizedMIME
		}
		return protocol.WorkspaceFileArtifactKindImage, normalizedMIME
	}
	if normalizedMIME == "application/pdf" {
		return protocol.WorkspaceFileArtifactKindPDF, normalizedMIME
	}
	if normalizedMIME == "text/html" {
		return protocol.WorkspaceFileArtifactKindHTML, normalizedMIME
	}
	if normalizedMIME == "text/markdown" {
		return protocol.WorkspaceFileArtifactKindMarkdown, normalizedMIME
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return protocol.WorkspaceFileArtifactKindImage, "image/png"
	case ".jpg", ".jpeg":
		return protocol.WorkspaceFileArtifactKindImage, "image/jpeg"
	case ".webp":
		return protocol.WorkspaceFileArtifactKindImage, "image/webp"
	case ".gif":
		return protocol.WorkspaceFileArtifactKindImage, "image/gif"
	case ".avif":
		return protocol.WorkspaceFileArtifactKindImage, "image/avif"
	case ".svg":
		return protocol.WorkspaceFileArtifactKindSVG, "image/svg+xml"
	case ".pdf":
		return protocol.WorkspaceFileArtifactKindPDF, "application/pdf"
	case ".md", ".markdown":
		return protocol.WorkspaceFileArtifactKindMarkdown, "text/markdown"
	case ".html", ".htm":
		return protocol.WorkspaceFileArtifactKindHTML, "text/html"
	case ".mmd", ".mermaid":
		return protocol.WorkspaceFileArtifactKindMermaid, "text/plain"
	default:
		return protocol.WorkspaceFileArtifactKindFile, normalizedMIME
	}
}

func workspaceFileArtifactTitle(path string) string {
	normalized := strings.TrimSpace(filepath.Base(filepath.ToSlash(path)))
	if normalized == "." || normalized == string(filepath.Separator) {
		return ""
	}
	return normalized
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
