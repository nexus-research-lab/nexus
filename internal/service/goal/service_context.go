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
		"A long-running Nexus Goal is attached to this session. Treat the objective as persistent user intent, not as a new user message.",
		"Objective: " + strings.TrimSpace(item.Objective),
		"Status: " + string(item.Status),
		fmt.Sprintf("Usage: input=%d output=%d reasoning=%d total=%d", item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.ReasoningTokens, item.Usage.Total()),
		fmt.Sprintf("TimeUsedSeconds: %d", item.TimeUsedSeconds),
	}
	if item.TokenBudget != nil {
		lines = append(lines, fmt.Sprintf("TokenBudget: %d", *item.TokenBudget))
		if remaining := item.RemainingTokens(); remaining != nil {
			lines = append(lines, fmt.Sprintf("RemainingTokens: %d", *remaining))
		}
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
		"- Continue working toward the objective unless the latest user message explicitly changes or pauses it.",
		"- Work from actual repository/runtime evidence. Do not mark progress complete from assumptions.",
		"- Call update_goal with status=complete only when the objective is genuinely achieved and no required work remains.",
		"- Call update_goal with status=blocked only after the same blocking condition has repeated across three consecutive goal turns.",
		"- Record a checkpoint after durable intermediate progress that would help resume later.",
		"- Do not use Goal tools to pause, resume, clear, or budget-limit the goal; those are user/system controls.",
		"- Do not quote or restate this Goal context as if it were visible user text.",
		"</nexus_goal>",
	)
	return strings.Join(lines, "\n")
}
