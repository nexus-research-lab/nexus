package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RuntimeContext 返回注入给模型的当前 Goal 上下文。
func (s *Service) RuntimeContext(ctx context.Context, sessionKey string) (string, *protocol.Goal, error) {
	item, err := s.Current(ctx, sessionKey)
	if err != nil {
		return "", nil, err
	}
	checkpoint, err := s.repo.LatestCheckpoint(ctx, item.ID)
	if err != nil {
		return "", nil, err
	}
	return BuildRuntimeContextWithCheckpoint(*item, checkpoint), item, nil
}

// BuildRuntimeContext 构造稳定的 Goal runtime prompt 段落。
func BuildRuntimeContext(item protocol.Goal) string {
	return BuildRuntimeContextWithCheckpoint(item, nil)
}

// BuildRuntimeContextWithCheckpoint 构造包含最新 checkpoint 的 Goal runtime prompt 段落。
func BuildRuntimeContextWithCheckpoint(item protocol.Goal, checkpoint *protocol.GoalCheckpoint) string {
	if !protocol.IsCurrentGoalStatus(item.Status) {
		return ""
	}
	lines := []string{
		"<nexus_goal>",
		"当前会话存在一个需要持续推进的 Goal。你必须在后续回答和工具调用中优先围绕它推进，除非用户明确改变目标。",
		"Objective: " + strings.TrimSpace(item.Objective),
		"Status: " + string(item.Status),
		fmt.Sprintf("Usage: input=%d output=%d total=%d", item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.TotalTokens),
	}
	if item.TokenBudget != nil {
		lines = append(lines, fmt.Sprintf("TokenBudget: %d", *item.TokenBudget))
	}
	if checkpoint != nil && strings.TrimSpace(checkpoint.Summary) != "" {
		lines = append(lines,
			"LatestCheckpoint:",
			fmt.Sprintf("- ContinuationCount: %d", checkpoint.ContinuationCount),
			"- Summary: "+strings.TrimSpace(checkpoint.Summary),
		)
	}
	lines = append(lines,
		"Rules:",
		"- 如果 Goal 已经真正完成，调用 Goal 工具标记完成。",
		"- 如果没有用户输入或外部状态就无法继续，调用 Goal 工具标记阻塞并说明需要什么。",
		"- 当完成一段可恢复的中间成果时，调用 Goal 工具记录 checkpoint。",
		"- 不要把 Goal 上下文当作用户新消息复述。",
		"</nexus_goal>",
	)
	return strings.Join(lines, "\n")
}
