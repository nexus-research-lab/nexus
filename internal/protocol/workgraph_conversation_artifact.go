// INPUT: 受管 execution CLI 返回的完整 WorkGraph Draft 或已保存命名图。
// OUTPUT: 可持久化在 assistant 消息中的只读工作图产物块。
// POS: 普通 DM/Room 最终回复直接展示草图、并按 exact source Execution 对照原图的消息协议。
package protocol

import "strings"

const ContentBlockTypeWorkGraphArtifact = "workgraph_artifact"

const (
	WorkGraphArtifactStateDraft = "draft"
	WorkGraphArtifactStateSaved = "saved"
)

// WorkGraphArtifactBlock 保存生成该消息时的完整图快照；历史消息不依赖当前 Draft head。
type WorkGraphArtifactBlock struct {
	ID               string                    `json:"id,omitempty"`
	Type             string                    `json:"type"`
	State            string                    `json:"state"`
	Operation        string                    `json:"operation"`
	HeadRevision     int64                     `json:"head_revision,omitempty"`
	SelectedRevision int64                     `json:"selected_revision,omitempty"`
	VersionCount     int                       `json:"version_count,omitempty"`
	Preview          *WorkGraphWorkflowPreview `json:"preview,omitempty"`
	Workflow         *WorkGraphWorkflow        `json:"workflow,omitempty"`
	SourceToolUseID  string                    `json:"source_tool_use_id,omitempty"`
}

// Map 转成现有 transcript 使用的动态内容块。
func (b WorkGraphArtifactBlock) Map() map[string]any {
	result := map[string]any{
		"type":      ContentBlockTypeWorkGraphArtifact,
		"state":     strings.TrimSpace(b.State),
		"operation": strings.TrimSpace(b.Operation),
	}
	if value := strings.TrimSpace(b.ID); value != "" {
		result["id"] = value
	}
	if b.HeadRevision > 0 {
		result["head_revision"] = b.HeadRevision
	}
	if b.SelectedRevision > 0 {
		result["selected_revision"] = b.SelectedRevision
	}
	if b.VersionCount > 0 {
		result["version_count"] = b.VersionCount
	}
	if b.Preview != nil {
		result["preview"] = b.Preview
	}
	if b.Workflow != nil {
		result["workflow"] = b.Workflow
	}
	if value := strings.TrimSpace(b.SourceToolUseID); value != "" {
		result["source_tool_use_id"] = value
	}
	return result
}
