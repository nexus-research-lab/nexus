package room

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

// goalCancellationProvider 是用户取消当前 Room Goal 所需的最小 Goal 能力。
type goalCancellationProvider interface {
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	Clear(context.Context, string) (bool, error)
}

// cancelActiveRoomGoalForUser 定义用户取消的边界：只清除 active Goal，
// 不把暂停或已完成的历史 Goal 重新解释为取消，也不触发新的续跑。
func (s *RealtimeService) cancelActiveRoomGoalForUser(
	ctx context.Context,
	sessionKey string,
	content string,
) error {
	if s == nil || !isGoalCancellationRequest(content) {
		return nil
	}
	provider, ok := s.goals.(goalCancellationProvider)
	if !ok {
		return nil
	}
	goal, err := provider.CurrentOptional(ctx, strings.TrimSpace(sessionKey))
	if errors.Is(err, goalsvc.ErrGoalNotFound) || goal == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if protocol.NormalizeGoalStatus(goal.Status) != protocol.GoalStatusActive {
		return nil
	}
	_, err = provider.Clear(ctx, goal.ID)
	if errors.Is(err, goalsvc.ErrGoalNotFound) {
		return nil
	}
	if err == nil {
		s.loggerFor(ctx).Info("用户取消 Room active Goal",
			"session_key", strings.TrimSpace(sessionKey),
			"goal_id", strings.TrimSpace(goal.ID),
			"content", strings.TrimSpace(content),
		)
	}
	return err
}

// isGoalCancellationRequest 只识别短、明确的停止意图，避免把普通讨论中的“停止”误判为取消。
func isGoalCancellationRequest(content string) bool {
	content = normalizeGoalCancellationText(content)
	if content == "" {
		return false
	}
	if content == "算了" || content == "不用了" || content == "取消" || content == "停止" || content == "停下" {
		return true
	}
	for _, phrase := range []string{
		"算了不用了",
		"不用继续",
		"取消这个任务",
		"取消任务",
		"停止这个任务",
		"停止任务",
	} {
		if strings.Contains(content, phrase) {
			return true
		}
	}
	return false
}

func normalizeGoalCancellationText(content string) string {
	content = strings.TrimSpace(strings.ToLower(content))
	var builder strings.Builder
	for _, runeValue := range content {
		if unicode.IsSpace(runeValue) || unicode.IsPunct(runeValue) {
			continue
		}
		builder.WriteRune(runeValue)
	}
	return builder.String()
}
