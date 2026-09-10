// INPUT: exact nexus.command tool_use、直接/包装的 MCP structured tool_result 与历史 CLI tool_use。
// OUTPUT: 带完整 Draft/命名图快照的 workgraph_artifact assistant 内容块。
// POS: 受管 WorkGraph authoring 结果进入普通 DM/Room 最终回复的唯一消息投影。
package message

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var workGraphArtifactOperations = map[string]struct{}{
	"extract_workgraph_preview":         {},
	"get_workgraph_preview":             {},
	"revise_workgraph_preview":          {},
	"select_workgraph_preview_revision": {},
	"save_workgraph_preview":            {},
}

func (p *Processor) workGraphArtifactForToolResult(
	toolResult map[string]any,
	structuredOutput map[string]any,
) map[string]any {
	if boolValue(toolResult["is_error"]) {
		return nil
	}
	toolUseID := normalizeString(toolResult["tool_use_id"])
	toolUse := p.segment.FindToolUse(toolUseID)
	commandOperation, native := managedExecutionCommandOperation(toolUse)
	if toolUseID == "" || len(toolUse) == 0 || commandOperation == "" {
		return nil
	}
	operation := commandOperation
	data := nativeWorkGraphArtifactData(operation, toolResult, structuredOutput)
	if !native {
		payload := firstWorkGraphArtifactPayload(toolResultContentText(toolResult["content"]))
		operation = normalizeString(payload["operation"])
		if normalizeString(payload["domain"]) != "execution" ||
			normalizeString(payload["action"]) != "invoke" || boolValue(payload["is_error"]) {
			return nil
		}
		data = mapValue(payload["data"])
	}
	if _, ok := workGraphArtifactOperations[operation]; !ok {
		return nil
	}
	if commandOperation != operation {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	artifact := protocol.WorkGraphArtifactBlock{
		ID:              fmt.Sprintf("workgraph:%s", toolUseID),
		Type:            protocol.ContentBlockTypeWorkGraphArtifact,
		State:           protocol.WorkGraphArtifactStateDraft,
		Operation:       operation,
		SourceToolUseID: toolUseID,
	}
	switch operation {
	case "extract_workgraph_preview":
		artifact.Preview = decodeWorkGraphPreview(data["preview"])
		artifact.HeadRevision = 1
		artifact.SelectedRevision = 1
	case "get_workgraph_preview":
		populateWorkGraphDraftArtifact(&artifact, data)
	case "revise_workgraph_preview", "select_workgraph_preview_revision":
		populateWorkGraphDraftArtifact(&artifact, mapValue(data["draft"]))
	case "save_workgraph_preview":
		artifact.State = protocol.WorkGraphArtifactStateSaved
		artifact.Workflow = decodeWorkGraphWorkflow(data["workflow"])
		if artifact.Workflow != nil {
			artifact.HeadRevision = artifact.Workflow.Version
			artifact.SelectedRevision = artifact.Workflow.Version
			artifact.VersionCount = int(artifact.Workflow.Version)
		}
	}
	if artifact.Preview == nil && artifact.Workflow == nil {
		return nil
	}
	return artifact.Map()
}

func nativeWorkGraphArtifactData(
	operation string,
	toolResult map[string]any,
	structuredOutput map[string]any,
) map[string]any {
	candidates := []any{
		toolResult["structured_output"],
		structuredOutput["structuredContent"],
		structuredOutput["structured_content"],
		structuredOutput["structured_output"],
		structuredOutput["content"],
		structuredOutput,
	}
	for _, candidate := range candidates {
		data := mapValue(candidate)
		if workGraphArtifactDataMatchesOperation(operation, data) {
			return data
		}
	}
	return nil
}

func workGraphArtifactDataMatchesOperation(operation string, data map[string]any) bool {
	if len(data) == 0 {
		return false
	}
	switch operation {
	case "extract_workgraph_preview", "get_workgraph_preview":
		return decodeWorkGraphPreview(data["preview"]) != nil
	case "revise_workgraph_preview", "select_workgraph_preview_revision":
		return decodeWorkGraphPreview(mapValue(data["draft"])["preview"]) != nil
	case "save_workgraph_preview":
		return decodeWorkGraphWorkflow(data["workflow"]) != nil
	default:
		return false
	}
}

func managedExecutionCommandOperation(toolUse map[string]any) (string, bool) {
	name := normalizeString(toolUse["name"])
	input := mapValue(toolUse["input"])
	if name == "mcp__nexus__command" || name == "nexus__command" ||
		name == "nexus.command" || name == "nexus/command" {
		operation := normalizeString(input["operation"])
		if normalizeString(input["domain"]) != "execution" ||
			normalizeString(input["action"]) != "invoke" ||
			normalizeString(input["request_id"]) == "" {
			return "", false
		}
		if _, ok := workGraphArtifactOperations[operation]; !ok {
			return "", false
		}
		return operation, true
	}
	if name != "Bash" && name != "PowerShell" {
		return "", false
	}
	command := strings.TrimSpace(normalizeString(input["command"]))
	if strings.ContainsAny(command, "\n\r|;<>`") || strings.Contains(command, "$(") {
		return "", false
	}
	commandToken := `"${NEXUS_COMMAND_PATH}"`
	if name == "PowerShell" {
		commandToken = `& "${env:NEXUS_COMMAND_PATH}"`
	}
	if !strings.HasPrefix(command, commandToken) ||
		len(command) == len(commandToken) || !isWorkGraphCommandWhitespace(command[len(commandToken)]) {
		return "", false
	}
	arguments := strings.Fields(command[len(commandToken):])
	if len(arguments) != 7 || arguments[0] != "--json" ||
		arguments[1] != "execution" || arguments[2] != "invoke" {
		return "", false
	}
	values := make(map[string]string, 2)
	for index := 3; index < len(arguments); index += 2 {
		flag := arguments[index]
		if index+1 >= len(arguments) || (flag != "--operation" && flag != "--request-id") || values[flag] != "" {
			return "", false
		}
		value, ok := unquoteWorkGraphCommandArgument(arguments[index+1])
		if !ok {
			return "", false
		}
		values[flag] = value
	}
	operation := values["--operation"]
	if operation == "" || values["--request-id"] == "" {
		return "", false
	}
	if _, ok := workGraphArtifactOperations[operation]; !ok {
		return "", false
	}
	return operation, false
}

func isWorkGraphCommandWhitespace(value byte) bool {
	return value == ' ' || value == '\t'
}

func unquoteWorkGraphCommandArgument(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	if value[0] == '\'' || value[0] == '"' {
		if len(value) < 2 || value[len(value)-1] != value[0] {
			return "", false
		}
		value = value[1 : len(value)-1]
	}
	if value == "" || strings.ContainsAny(value, "'\"") {
		return "", false
	}
	return value, true
}

func firstWorkGraphArtifactPayload(content string) map[string]any {
	for _, candidate := range imagegenJSONCandidates(content) {
		var payload map[string]any
		if json.Unmarshal([]byte(candidate), &payload) == nil &&
			normalizeString(payload["domain"]) == "execution" {
			return payload
		}
	}
	return nil
}

func populateWorkGraphDraftArtifact(artifact *protocol.WorkGraphArtifactBlock, data map[string]any) {
	if artifact == nil || len(data) == 0 {
		return
	}
	artifact.Preview = decodeWorkGraphPreview(data["preview"])
	artifact.HeadRevision = int64Value(data["head_revision"])
	artifact.SelectedRevision = int64Value(data["selected_revision"])
	if versions, ok := data["versions"].([]any); ok {
		artifact.VersionCount = len(versions)
	}
}

func decodeWorkGraphPreview(value any) *protocol.WorkGraphWorkflowPreview {
	var preview protocol.WorkGraphWorkflowPreview
	if !decodeWorkGraphArtifactValue(value, &preview) || strings.TrimSpace(preview.PreviewID) == "" || len(preview.Nodes) == 0 {
		return nil
	}
	return &preview
}

func decodeWorkGraphWorkflow(value any) *protocol.WorkGraphWorkflow {
	var workflow protocol.WorkGraphWorkflow
	if !decodeWorkGraphArtifactValue(value, &workflow) || strings.TrimSpace(workflow.ID) == "" || len(workflow.Nodes) == 0 {
		return nil
	}
	return &workflow
}

func decodeWorkGraphArtifactValue(value any, target any) bool {
	encoded, err := json.Marshal(value)
	return err == nil && json.Unmarshal(encoded, target) == nil
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}
