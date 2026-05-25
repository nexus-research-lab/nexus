package runtime

import (
	"context"
	"errors"
	"fmt"
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

func prepareRoundContentWithContext(
	ctx context.Context,
	client Client,
	content any,
	blocks []ContextualInputBlock,
) (any, error) {
	blocks = normalizeContextualInputBlocks(blocks)
	if len(blocks) == 0 {
		return content, nil
	}
	if setter, ok := client.(nextTurnContextClient); ok {
		if err := setter.SetNextTurnContext(ctx, blocks); err == nil {
			return content, nil
		} else if !errors.Is(err, agentclient.ErrUnsupportedCapability) {
			return nil, err
		}
	}
	return prependContextualInputBlocks(content, blocks), nil
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
		if content := strings.TrimSpace(block.Content); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
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
		return prependText(prefix, strings.TrimSpace(stringValueForContextFallback(value)))
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
		copied := make(map[string]any, len(block))
		for key, value := range block {
			copied[key] = value
		}
		result = append(result, copied)
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

func stringValueForContextFallback(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(interface{ String() string }); ok {
		return text.String()
	}
	return fmt.Sprint(value)
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
