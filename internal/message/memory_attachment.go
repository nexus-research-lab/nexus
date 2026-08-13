// INPUT: runtime 的 relevant_memories attachment。
// OUTPUT: 可随 Assistant 持久化的记忆引用摘要，不复制正文和绝对路径。
// POS: 模型记忆附件到 Nexus Assistant 展示元数据的投影边界。
package message

import (
	"path/filepath"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// captureRelevantMemoryAttachment 暂存本轮已实际注入模型的记忆摘要。
func (p *Processor) captureRelevantMemoryAttachment(attachment sdkprotocol.AttachmentMessage) bool {
	if !strings.EqualFold(strings.TrimSpace(attachment.Type), "relevant_memories") {
		return false
	}
	p.recalledMemories = mergeRecalledMemoryReferences(
		p.recalledMemories,
		recalledMemoryReferences(attachment),
	)
	return true
}

func recalledMemoryReferences(attachment sdkprotocol.AttachmentMessage) []map[string]any {
	items := sliceValue(attachment.Additional["memories"])
	if len(items) == 0 {
		items = sliceValue(attachment.Content)
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := mapValue(item)
		name := recalledMemoryName(payload)
		description := firstNonEmpty(
			normalizeString(payload["description"]),
			memoryFrontmatterDescription(normalizeString(payload["content"])),
			name,
		)
		if description == "" {
			continue
		}
		result = append(result, map[string]any{
			"name":        name,
			"description": description,
		})
	}
	return result
}

func recalledMemoryName(payload map[string]any) string {
	name := firstNonEmpty(
		normalizeString(payload["name"]),
		normalizeString(payload["filename"]),
		filepath.Base(normalizeString(payload["path"])),
	)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.TrimSpace(strings.NewReplacer("_", " ", "-", " ").Replace(name))
}

func memoryFrontmatterDescription(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	frontmatter, _, found := strings.Cut(strings.TrimPrefix(content, "---\n"), "\n---")
	if !found {
		return ""
	}
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "description") {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

func mergeRecalledMemoryReferences(current []map[string]any, incoming []map[string]any) []map[string]any {
	result := cloneBlockSlice(current)
	seen := make(map[string]struct{}, len(result)+len(incoming))
	for _, reference := range result {
		seen[normalizeString(reference["name"])+"\x00"+normalizeString(reference["description"])] = struct{}{}
	}
	for _, reference := range incoming {
		key := normalizeString(reference["name"]) + "\x00" + normalizeString(reference["description"])
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cloneMap(reference))
	}
	return result
}

func sliceValue(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}
