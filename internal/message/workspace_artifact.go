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
	if !isImagegenArtifactTool(toolName) {
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
		Label:            imagegenArtifactLabel(payload),
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

func imagegenArtifactLabel(payload map[string]any) string {
	switch strings.TrimSpace(normalizeString(payload["action"])) {
	case "edit", "edit_image":
		return "编辑图片"
	default:
		return "生成图片"
	}
}

func isImagegenArtifactTool(toolName string) bool {
	normalized := strings.TrimSpace(toolName)
	if normalized == "" {
		return false
	}
	switch normalized {
	case "Bash", "generate_image", "edit_image", "nexus_imagegen":
		return true
	}
	return strings.HasPrefix(normalized, "mcp__nexus_imagegen__") ||
		strings.HasPrefix(normalized, "nexus_imagegen__") ||
		strings.HasPrefix(normalized, "nexus_imagegen.")
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

type workspaceArtifactFormat struct {
	kind     string
	mimeType string
}

var workspaceArtifactFormatsByMIME = map[string]workspaceArtifactFormat{
	"image/svg+xml":   {kind: protocol.WorkspaceFileArtifactKindSVG, mimeType: "image/svg+xml"},
	"application/pdf": {kind: protocol.WorkspaceFileArtifactKindPDF, mimeType: "application/pdf"},
	"text/html":       {kind: protocol.WorkspaceFileArtifactKindHTML, mimeType: "text/html"},
	"text/markdown":   {kind: protocol.WorkspaceFileArtifactKindMarkdown, mimeType: "text/markdown"},
}

var workspaceArtifactFormatsByExtension = map[string]workspaceArtifactFormat{
	".png":      {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/png"},
	".jpg":      {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/jpeg"},
	".jpeg":     {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/jpeg"},
	".webp":     {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/webp"},
	".gif":      {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/gif"},
	".avif":     {kind: protocol.WorkspaceFileArtifactKindImage, mimeType: "image/avif"},
	".svg":      {kind: protocol.WorkspaceFileArtifactKindSVG, mimeType: "image/svg+xml"},
	".pdf":      {kind: protocol.WorkspaceFileArtifactKindPDF, mimeType: "application/pdf"},
	".md":       {kind: protocol.WorkspaceFileArtifactKindMarkdown, mimeType: "text/markdown"},
	".markdown": {kind: protocol.WorkspaceFileArtifactKindMarkdown, mimeType: "text/markdown"},
	".html":     {kind: protocol.WorkspaceFileArtifactKindHTML, mimeType: "text/html"},
	".htm":      {kind: protocol.WorkspaceFileArtifactKindHTML, mimeType: "text/html"},
	".mmd":      {kind: protocol.WorkspaceFileArtifactKindMermaid, mimeType: "text/plain"},
	".mermaid":  {kind: protocol.WorkspaceFileArtifactKindMermaid, mimeType: "text/plain"},
}

func workspaceFileArtifactKindAndMIME(path string, mimeType string) (string, string) {
	normalizedMIME := strings.ToLower(strings.TrimSpace(mimeType))
	if format, exists := workspaceArtifactFormatsByMIME[normalizedMIME]; exists {
		return format.kind, format.mimeType
	}
	if strings.HasPrefix(normalizedMIME, "image/") {
		return protocol.WorkspaceFileArtifactKindImage, normalizedMIME
	}
	if format, exists := workspaceArtifactFormatsByExtension[strings.ToLower(filepath.Ext(path))]; exists {
		return format.kind, format.mimeType
	}
	return protocol.WorkspaceFileArtifactKindFile, normalizedMIME
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
