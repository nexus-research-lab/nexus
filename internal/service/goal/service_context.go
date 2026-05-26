package goal

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RuntimeContext 返回当前运行时应计入的 Goal。
// 对齐 Codex：普通 active turn 只做用量/耗时记账，不额外注入 Goal context；
// `<goal_context>` 只用于隐藏续跑、objective update 和 budget limit steering。
func (s *Service) RuntimeContext(ctx context.Context, sessionKey string) (string, *protocol.Goal, error) {
	item, err := s.Current(ctx, sessionKey)
	if err != nil {
		return "", nil, err
	}
	if !protocol.IsRuntimeAccountingGoalStatus(item.Status) {
		return "", nil, nil
	}
	if !protocol.IsRuntimeGoalStatus(item.Status) {
		return "", item, nil
	}
	item, err = s.accountActiveWallClockUsage(ctx, *item)
	if err != nil {
		return "", nil, err
	}
	return "", item, nil
}
