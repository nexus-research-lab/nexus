// INPUT: runtime 拥有的隐藏上下文、Automation binding、输入选项与客户端能力。
// OUTPUT: next-turn context 设置、内部来源投影与发送给 provider 的干净选项。
// POS: 应用层上下文到 runtime 下一轮模型输入的统一注入边界。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"html"
	"maps"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

// RuntimeInputOptionsForPurpose 剥离只属于本地队列/历史层的控制字段。
func RuntimeInputOptionsForPurpose(options sdkprotocol.OutboundMessageOptions, purpose string) sdkprotocol.OutboundMessageOptions {
	if strings.TrimSpace(options.Purpose) != strings.TrimSpace(purpose) {
		return options
	}
	options.Meta = false
	options.Synthetic = false
	options.HiddenFromUser = false
	options.RecallQuery = ""
	options.Priority = ""
	options.Purpose = ""
	options.Metadata = nil
	return options
}

// AutomationRunContextualInputs 把可信 run binding 投影为隐藏上下文。
// 工具权限不从这里解析；该文本只帮助模型理解本轮运行语义。
func AutomationRunContextualInputs(binding *protocol.AutomationRunContext) []ContextualInputBlock {
	if binding == nil {
		return nil
	}
	normalized := binding.Normalized()
	if !normalized.Valid() {
		return nil
	}
	attributes := []string{
		fmt.Sprintf(`job_id="%s"`, html.EscapeString(normalized.JobID)),
		fmt.Sprintf(`run_id="%s"`, html.EscapeString(normalized.RunID)),
	}
	if normalized.JobName != "" {
		attributes = append(attributes, fmt.Sprintf(`task_name="%s"`, html.EscapeString(normalized.JobName)))
	}
	if normalized.PermissionPolicyRevision > 0 {
		attributes = append(attributes, fmt.Sprintf(`permission_revision="%d"`, normalized.PermissionPolicyRevision))
	}
	body := "This turn is a scheduled-task run. The scheduler owns result delivery; return only the requested result and do not address or route the destination yourself. nexus.command is read-only and already scoped to this task."
	if normalized.ResumeToolName != "" {
		body += fmt.Sprintf(
			" The user approved a previous permission request. Call tool %q again with the task's original arguments",
			normalized.ResumeToolName,
		)
		if normalized.ResumeResourceScope != "" {
			body += fmt.Sprintf(" for resource %q", normalized.ResumeResourceScope)
		}
		body += "; wait for its actual result before summarizing."
	}
	content := fmt.Sprintf(
		"<nexus_automation_context %s>\n%s\n</nexus_automation_context>",
		strings.Join(attributes, " "),
		body,
	)
	return []ContextualInputBlock{
		NewContextualInputBlock(ContextualInputNameAutomation, content, 0, map[string]string{
			"job_id": normalized.JobID,
			"run_id": normalized.RunID,
		}),
	}
}
