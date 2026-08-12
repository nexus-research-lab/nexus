// INPUT: runtime 拥有的隐藏上下文块、当前用户输入与客户端能力。
// OUTPUT: next-turn context 设置，或带明确内部来源标签的兼容输入。
// POS: 应用层上下文到 runtime 下一轮模型输入的统一注入边界。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// ContextualInputBlock 表示运行时拥有、应注入到下一轮模型输入的隐藏上下文。
type ContextualInputBlock struct {
	Name     string
	Content  string
	Priority int
	Metadata map[string]string
}

func NewContextualInputBlock(name string, content string, priority int, metadata map[string]string) ContextualInputBlock {
	return ContextualInputBlock{
		Name:     name,
		Content:  content,
		Priority: priority,
		Metadata: cloneStringMap(metadata),
	}
}

type nextTurnContextClient interface {
	SetNextTurnContext(context.Context, []ContextualInputBlock) error
}

type nextTurnContextClearer interface {
	ClearNextTurnContext(context.Context) error
}

const (
	contextOnlyTurnTrigger                = "Continue."
	ContextualInputNameRoundRecovery      = "round_recovery"
	ContextualInputNameExecution          = "execution"
	ContextualInputNameTransport          = "transport"
	ContextualInputNameAutomation         = "automation"
	ContextualInputNameAutomationDelivery = "automation_delivery"
)

func PrepareRoundContentWithContext(
	ctx context.Context,
	client Client,
	content any,
	blocks []ContextualInputBlock,
) (any, error) {
	blocks = normalizeContextualInputBlocks(blocks)
	if len(blocks) == 0 {
		return content, nil
	}
	if isContextOnlyContent(content) {
		return prependContextualInputBlocks(content, blocks), nil
	}
	buffered, err := prepareBufferedContext(ctx, client, blocks)
	if err != nil {
		return nil, err
	}
	if buffered {
		return contentWithContextTrigger(content), nil
	}
	return prependContextualInputBlocks(content, blocks), nil
}

// prepareBufferedContext 只在 client 同时提供设置与逻辑清理时使用下一轮 buffer。
// 缺少清理能力的自定义 client 直接内联，避免失败发送留下跨轮残留。
func prepareBufferedContext(
	ctx context.Context,
	client Client,
	blocks []ContextualInputBlock,
) (bool, error) {
	setter, canSet := client.(nextTurnContextClient)
	clearer, canClear := client.(nextTurnContextClearer)
	if !canSet || !canClear {
		return false, nil
	}
	if err := clearer.ClearNextTurnContext(ctx); err != nil {
		if errors.Is(err, agentclient.ErrUnsupportedCapability) {
			return false, nil
		}
		return false, err
	}
	if err := setter.SetNextTurnContext(ctx, blocks); err != nil {
		if errors.Is(err, agentclient.ErrUnsupportedCapability) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// PrepareAtomicRoundContent 清空 bridge 的一次性上下文后原样返回命令输入。
func PrepareAtomicRoundContent(
	ctx context.Context,
	client Client,
	content any,
) (any, error) {
	clearer, ok := client.(nextTurnContextClearer)
	if !ok {
		return content, nil
	}
	if err := clearer.ClearNextTurnContext(ctx); err != nil &&
		!errors.Is(err, agentclient.ErrUnsupportedCapability) {
		return nil, err
	}
	return content, nil
}

func isContextOnlyContent(content any) bool {
	value, ok := content.(string)
	return ok && strings.TrimSpace(value) == ""
}

func contentWithContextTrigger(content any) any {
	switch value := content.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return contextOnlyTurnTrigger
		}
	}
	return content
}

func normalizeContextualInputBlocks(blocks []ContextualInputBlock) []ContextualInputBlock {
	result := make([]ContextualInputBlock, 0, len(blocks))
	for _, block := range blocks {
		block.Name = strings.TrimSpace(block.Name)
		block.Content = strings.TrimSpace(block.Content)
		if block.Content == "" {
			continue
		}
		if len(block.Metadata) > 0 {
			metadata := make(map[string]string, len(block.Metadata))
			for key, value := range block.Metadata {
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key == "" || value == "" {
					continue
				}
				metadata[key] = value
			}
			block.Metadata = metadata
		}
		result = append(result, block)
	}
	return result
}

func renderContextualInputBlocks(blocks []ContextualInputBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if content := renderContextualInputBlock(block); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func renderContextualInputBlock(block ContextualInputBlock) string {
	content := strings.TrimSpace(block.Content)
	if content == "" {
		return ""
	}
	if source := internalContextSourceName(block.Name); source != "" {
		return renderInternalContext(source, content)
	}
	return content
}

func internalContextSourceName(name string) string {
	switch strings.TrimSpace(name) {
	case "goal", "goal_context":
		return "goal"
	case ContextualInputNameExecution:
		return ContextualInputNameExecution
	case ContextualInputNameTransport:
		return ContextualInputNameTransport
	case ContextualInputNameRoundRecovery:
		return ContextualInputNameRoundRecovery
	case ContextualInputNameAutomation:
		return ContextualInputNameAutomation
	case ContextualInputNameAutomationDelivery:
		return ContextualInputNameAutomationDelivery
	default:
		return ""
	}
}

func renderInternalContext(source string, content string) string {
	content = strings.TrimSpace(content)
	if isInternalContext(content) {
		return content
	}
	content = unwrapLegacyGoalContext(content)
	return fmt.Sprintf("<internal_context source=\"%s\">\n%s\n</internal_context>", source, content)
}

func isInternalContext(content string) bool {
	content = strings.TrimSpace(content)
	return (strings.HasPrefix(content, "<internal_context ") &&
		strings.HasSuffix(content, "</internal_context>")) ||
		(strings.HasPrefix(content, "<codex_internal_context ") &&
			strings.HasSuffix(content, "</codex_internal_context>"))
}

func unwrapLegacyGoalContext(content string) string {
	content = strings.TrimSpace(content)
	const openTag = "<goal_context>"
	const closeTag = "</goal_context>"
	if strings.HasPrefix(content, openTag) && strings.HasSuffix(content, closeTag) {
		content = strings.TrimPrefix(content, openTag)
		content = strings.TrimSuffix(content, closeTag)
		return strings.TrimSpace(content)
	}
	return content
}

func prependContextualInputBlocks(content any, blocks []ContextualInputBlock) any {
	prefix := renderContextualInputBlocks(blocks)
	if prefix == "" {
		return content
	}
	switch value := content.(type) {
	case string:
		return prependText(prefix, value)
	case []map[string]any:
		return prependTextBlock(prefix, value)
	case []any:
		return prependAnyTextBlock(prefix, value)
	default:
		return content
	}
}

func prependText(prefix string, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return strings.TrimSpace(prefix)
	}
	return strings.TrimSpace(prefix) + "\n\n" + text
}

func prependTextBlock(prefix string, blocks []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(blocks)+1)
	result = append(result, map[string]any{
		"type": "text",
		"text": strings.TrimSpace(prefix),
	})
	for _, block := range blocks {
		result = append(result, maps.Clone(block))
	}
	return result
}

func prependAnyTextBlock(prefix string, blocks []any) []any {
	result := make([]any, 0, len(blocks)+1)
	result = append(result, map[string]any{
		"type": "text",
		"text": strings.TrimSpace(prefix),
	})
	result = append(result, blocks...)
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	return maps.Clone(input)
}
