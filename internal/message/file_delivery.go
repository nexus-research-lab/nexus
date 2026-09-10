// INPUT: Exact nexus.deliver_files result and current Agent round identity.
// OUTPUT: Durable deliverable blocks attached to the producing assistant message.
// POS: Explicit file delivery receipt projection; ordinary Bash/MCP text and Markdown are not evidence.
package message

import (
	"encoding/json"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func isFileDeliveryTool(name string) bool {
	switch name {
	case "mcp__nexus__deliver_files", "nexus__deliver_files", "nexus.deliver_files", "nexus/deliver_files":
		return true
	}
	return false
}

func (p *Processor) fileDeliveryArtifacts(result map[string]any, toolID, toolName string) []map[string]any {
	var receipt struct {
		Kind         string   `json:"kind"`
		Version      int      `json:"version"`
		AgentID      string   `json:"agent_id"`
		AgentRoundID string   `json:"agent_round_id"`
		Paths        []string `json:"paths"`
	}
	if json.Unmarshal([]byte(toolResultContentText(result["content"])), &receipt) != nil ||
		receipt.Kind != "file_delivery" || receipt.Version != 1 ||
		p.ctx.AgentID == "" || receipt.AgentID != p.ctx.AgentID ||
		p.ctx.AgentRoundID == "" || receipt.AgentRoundID != p.ctx.AgentRoundID ||
		len(receipt.Paths) == 0 || len(receipt.Paths) > 32 {
		return nil
	}
	blocks := make([]map[string]any, 0, len(receipt.Paths))
	seen := make(map[string]bool)
	for _, path := range receipt.Paths {
		if path == "" || path != p.normalizeWorkspaceArtifactPath(path) {
			return nil
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		kind, mime := workspaceFileArtifactKindAndMIME(path, "")
		block := protocol.WorkspaceFileArtifactBlock{
			ID:   fmt.Sprintf("workspace_file:%s:%s", toolID, path),
			Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
			Role: "deliverable", ProducerAgentID: p.ctx.AgentID, SourceAgentRoundID: p.ctx.AgentRoundID,
			Path: path, DisplayPath: path, Title: workspaceFileArtifactTitle(path),
			ArtifactKind: kind, MIMEType: mime,
			Scope:            protocol.WorkspaceFileArtifactScopeAgentWorkspace,
			WorkspaceAgentID: p.ctx.AgentID, SourceToolUseID: toolID, SourceToolName: toolName,
		}
		blocks = append(blocks, block.Map())
	}
	return blocks
}
