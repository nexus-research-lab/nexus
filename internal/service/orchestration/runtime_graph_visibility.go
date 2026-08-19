// INPUT: provider-neutral Runtime Tool NodeRun 名称与既有领域投影。
// OUTPUT: Tool 是否构成用户可理解、值得进入 WorkGraph 画布的动作分类。
// POS: Runtime Graph 展示层的 Tool 语义分类；不改变执行、权限、路由或持久化事实。
package orchestration

import (
	"strings"
	"unicode"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
)

var runtimeGraphVisibleToolFamilies = []string{
	"WebSearch",
	"WebFetch",
	"Write",
	"Edit",
	"MultiEdit",
	"NotebookEdit",
}

// These names are the complete Goal/Execution MCP surface that existed before
// the control plane moved to the round-scoped nexus CLI. They are read-model
// compatibility facts only: recognizing one never restores an MCP route or grants
// authority. Historical rows do not carry the newer command-transport metadata,
// so the projection must classify their exact semantic identity here or they are
// mistaken for ordinary external MCP work and leak back onto the canvas.
var runtimeGraphLegacyManagedTransportOperations = map[string]struct{}{
	"getgoal":                 {},
	"creategoal":              {},
	"retargetgoal":            {},
	"auditobjectivealignment": {},
	"updategoal":              {},
	"getexecution":            {},
	"prepareplanexecution":    {},
	"planexecution":           {},
	"abandonexecution":        {},
	"assignwork":              {},
	"submitwork":              {},
	"reviewwork":              {},
	"blockwork":               {},
	"resumework":              {},
	"takeoverwork":            {},
	"auditexecutionalignment": {},
	"promoteexecutiontogoal":  {},
}

// 未包装的 Provider 工具只有在名称表达可观察动作时才进入画布；外部 MCP
// capability 调用默认可见，而本地 filesystem/workspace/tool discovery 服务
// 留在详情。Goal/Execution CLI 控制 transport 始终留在 direct owner 详情；
// 其他运行失败、控制边、Artifact 与显式 hint 仍由上层结构事实提升。
var runtimeGraphVisibleActionPrefixes = []string{
	"append",
	"approve",
	"archive",
	"cancel",
	"click",
	"copy",
	"create",
	"delete",
	"deploy",
	"download",
	"edit",
	"execute",
	"export",
	"fill",
	"generate",
	"install",
	"kill",
	"move",
	"navigate",
	"open",
	"patch",
	"post",
	"publish",
	"reject",
	"remove",
	"rename",
	"render",
	"restore",
	"run",
	"send",
	"screenshot",
	"start",
	"stop",
	"submit",
	"type",
	"uninstall",
	"update",
	"upload",
	"write",
}

var runtimeGraphSupportingMCPServerMarkers = []string{
	"filesystem",
	"localfs",
	"nexusexecution",
	"nexusgoal",
	"skill",
	"toolsearch",
	"workspace",
}

func runtimeGraphToolActionVisible(item protocol.ExecutionRuntimeNodeRun) bool {
	if item.Kind != protocol.ExecutionRuntimeNodeTool {
		return false
	}
	if runtimeGraphIsCommandTransport(item) {
		return false
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return false
	}
	for _, family := range runtimeGraphVisibleToolFamilies {
		if toolpolicy.MatchesItem(name, family) {
			return true
		}
	}
	if runtimeGraphExternalCapabilityVisible(name) {
		return true
	}
	leaf := runtimeGraphCanonicalToolLeaf(name)
	for _, prefix := range runtimeGraphVisibleActionPrefixes {
		if strings.HasPrefix(leaf, prefix) {
			return true
		}
	}
	return strings.HasSuffix(leaf, "codeexecution")
}

func runtimeGraphIsCommandTransport(item protocol.ExecutionRuntimeNodeRun) bool {
	switch value := item.Metadata[runtimeGraphCommandTransportMetadataKey].(type) {
	case bool:
		if value {
			return true
		}
	case string:
		if strings.EqualFold(strings.TrimSpace(value), "true") {
			return true
		}
	}
	return runtimeGraphIsLegacyManagedTransport(item.Name)
}

func runtimeGraphIsLegacyManagedTransport(name string) bool {
	trimmed := strings.TrimSpace(name)
	leaf := runtimeGraphCanonicalToolLeaf(trimmed)
	if _, exists := runtimeGraphLegacyManagedTransportOperations[leaf]; !exists {
		return false
	}

	separatorIndex := -1
	for _, separator := range []string{"__", ".", "/"} {
		if index := strings.LastIndex(trimmed, separator); index > separatorIndex {
			separatorIndex = index
		}
	}
	// Old SDK projections sometimes persisted only the MCP operation name.
	if separatorIndex < 0 {
		return true
	}

	prefix := trimmed[:separatorIndex]
	server := runtimeGraphCanonicalToolLeaf(prefix)
	return strings.Contains(server, "nexusexecution") ||
		strings.Contains(server, "nexusgoal")
}

func runtimeGraphIsSubmissionTool(name string) bool {
	return runtimeGraphCanonicalToolLeaf(name) == "submitwork"
}

func runtimeGraphExternalCapabilityVisible(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if !strings.HasPrefix(normalized, "mcp__") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(normalized, "mcp__"), "__", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return false
	}
	server := runtimeGraphCanonicalToolLeaf(parts[0])
	for _, marker := range runtimeGraphSupportingMCPServerMarkers {
		if strings.Contains(server, marker) {
			return false
		}
	}
	return true
}

func runtimeGraphCanonicalToolLeaf(name string) string {
	leaf := strings.TrimSpace(name)
	for _, separator := range []string{"__", ".", "/"} {
		if index := strings.LastIndex(leaf, separator); index >= 0 {
			leaf = leaf[index+len(separator):]
		}
	}
	var result strings.Builder
	for _, value := range strings.ToLower(leaf) {
		if unicode.IsLetter(value) || unicode.IsDigit(value) {
			result.WriteRune(value)
		}
	}
	return result.String()
}
