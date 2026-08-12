// INPUT: 已由 message 投影生成的 workspace_file_artifact 内容块与 exact runtime identity。
// OUTPUT: 回挂到对应 Tool NodeRun 的有界结构化 Artifact 引用与 durable 写后的 session 失效事实。
// POS: durable 消息 Artifact 与 Runtime Graph 的事实关联层；不读取文件、不推断交付，也不触发后续路线。
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	runtimeGraphArtifactsMetadataKey = "workspace_artifacts"
	runtimeGraphArtifactLimit        = protocol.ExecutionRuntimeGraphArtifactProjectionLimit
)

// ObserveRuntimeArtifacts 把 durable message 已确认的结构化 Artifact 先按 exact
// ToolUse 独立持久化。Tool NodeRun 可以先到或后到；读取时才回挂，不凭路径或
// 工具名称造节点。所有 Runtime Graph 仓储共享同一到达顺序无关的写契约。
func (s *Service) ObserveRuntimeArtifacts(
	ctx context.Context,
	actor ActorContext,
	message protocol.Message,
) error {
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil
	}
	artifactsByToolUseID := runtimeGraphArtifactsFromMessage(message)
	if len(artifactsByToolUseID) == 0 {
		return nil
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return err
	}
	// Preserve visibility of any artifact prefix committed before a later
	// artifact write fails.
	defer s.invalidateActor(ctx, actor)
	now := s.now().UTC()
	for toolUseID, artifacts := range artifactsByToolUseID {
		for _, artifact := range artifacts {
			ref := protocol.ExecutionRuntimeArtifactRef{
				GraphID:      identity.GraphID,
				OwnerUserID:  identity.OwnerUserID,
				SessionKey:   identity.SessionKey,
				ExecutionID:  identity.ExecutionID,
				RootRoundID:  identity.RootRoundID,
				AgentRoundID: identity.AgentRoundID,
				ToolUseID:    toolUseID,
				Artifact:     artifact,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			ref.ID = stableRuntimeGraphID(
				"runtime_artifact",
				identity.OwnerUserID,
				identity.SessionKey,
				identity.AgentRoundID,
				toolUseID,
				artifact.ID,
				artifact.Path,
			)
			if err = repository.UpsertRuntimeGraphArtifact(ctx, ref); err != nil {
				return err
			}
		}
	}
	return nil
}

func runtimeGraphArtifactsFromMessage(
	message protocol.Message,
) map[string][]protocol.WorkspaceFileArtifactBlock {
	result := make(map[string][]protocol.WorkspaceFileArtifactBlock)
	for _, block := range runtimeGraphMessageContentBlocks(message["content"]) {
		artifact, ok := runtimeGraphWorkspaceArtifact(block)
		if !ok {
			continue
		}
		toolUseID := strings.TrimSpace(artifact.SourceToolUseID)
		result[toolUseID] = mergeRuntimeGraphArtifacts(result[toolUseID], []protocol.WorkspaceFileArtifactBlock{artifact})
	}
	return result
}

func runtimeGraphMessageContentBlocks(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if block, ok := item.(map[string]any); ok {
				result = append(result, block)
			}
		}
		return result
	default:
		return nil
	}
}

func runtimeGraphWorkspaceArtifact(
	values map[string]any,
) (protocol.WorkspaceFileArtifactBlock, bool) {
	if strings.TrimSpace(runtimeGraphAnyString(values["type"])) != protocol.ContentBlockTypeWorkspaceFileArtifact {
		return protocol.WorkspaceFileArtifactBlock{}, false
	}
	path := normalizeRuntimeGraphArtifactPath(runtimeGraphAnyString(values["path"]))
	toolUseID := strings.TrimSpace(runtimeGraphAnyString(values["source_tool_use_id"]))
	if path == "" || toolUseID == "" {
		return protocol.WorkspaceFileArtifactBlock{}, false
	}
	artifact := protocol.WorkspaceFileArtifactBlock{
		ID:               strings.TrimSpace(runtimeGraphAnyString(values["id"])),
		Type:             protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path:             path,
		DisplayPath:      strings.TrimSpace(runtimeGraphAnyString(values["display_path"])),
		Label:            strings.TrimSpace(runtimeGraphAnyString(values["label"])),
		Title:            strings.TrimSpace(runtimeGraphAnyString(values["title"])),
		ArtifactKind:     strings.TrimSpace(runtimeGraphAnyString(values["artifact_kind"])),
		MIMEType:         strings.TrimSpace(runtimeGraphAnyString(values["mime_type"])),
		Operation:        strings.TrimSpace(runtimeGraphAnyString(values["operation"])),
		Scope:            strings.TrimSpace(runtimeGraphAnyString(values["scope"])),
		WorkspaceAgentID: strings.TrimSpace(runtimeGraphAnyString(values["workspace_agent_id"])),
		SourceToolUseID:  toolUseID,
		SourceToolName:   strings.TrimSpace(runtimeGraphAnyString(values["source_tool_name"])),
	}
	if artifact.ID == "" {
		artifact.ID = fmt.Sprintf("workspace_file:%s:%s", toolUseID, path)
	}
	if artifact.DisplayPath == "" {
		artifact.DisplayPath = path
	}
	return artifact, true
}

func runtimeGraphNodeArtifacts(
	item protocol.ExecutionRuntimeNodeRun,
) []protocol.WorkspaceFileArtifactBlock {
	result := mergeRuntimeGraphArtifacts(nil, item.Artifacts)
	// 只读兼容独立 Artifact 表落地前已写入 Node metadata 的本地历史数据。
	raw, exists := item.Metadata[runtimeGraphArtifactsMetadataKey]
	if !exists || raw == nil {
		return result
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return result
	}
	var stored []protocol.WorkspaceFileArtifactBlock
	if json.Unmarshal(encoded, &stored) != nil {
		return result
	}
	return mergeRuntimeGraphArtifacts(result, stored)
}

func mergeRuntimeGraphArtifacts(
	current []protocol.WorkspaceFileArtifactBlock,
	incoming []protocol.WorkspaceFileArtifactBlock,
) []protocol.WorkspaceFileArtifactBlock {
	result := make([]protocol.WorkspaceFileArtifactBlock, 0, min(
		len(current)+len(incoming),
		runtimeGraphArtifactLimit,
	))
	seen := make(map[string]struct{})
	for _, collection := range [][]protocol.WorkspaceFileArtifactBlock{current, incoming} {
		for _, artifact := range collection {
			path := normalizeRuntimeGraphArtifactPath(artifact.Path)
			toolUseID := strings.TrimSpace(artifact.SourceToolUseID)
			if path == "" || toolUseID == "" {
				continue
			}
			artifact.Path = path
			artifact.Type = protocol.ContentBlockTypeWorkspaceFileArtifact
			key := firstNonEmpty(strings.TrimSpace(artifact.ID), toolUseID+"\x00"+path)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, artifact)
			if len(result) == runtimeGraphArtifactLimit {
				return result
			}
		}
	}
	return result
}

func normalizeRuntimeGraphArtifactPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) || strings.HasPrefix(value, "~") {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.HasPrefix(value, "/") {
		return ""
	}
	return value
}

func runtimeGraphAnyString(value any) string {
	result, _ := value.(string)
	return result
}
