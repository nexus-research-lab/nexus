// INPUT: 账号订阅额度、内部 Room Goal continuation 与当前 Goal 状态。
// OUTPUT: 额度耗尽时的 Goal usage_limited 投影。
// POS: Room 实时编排与 Goal 账号额度状态之间的适配边界。
package realtime

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *Service) recordGoalQuotaLimit(
	ctx context.Context,
	sessionKey string,
	roundID string,
	quotaErr error,
) {
	if s.goals == nil || quotaErr == nil {
		return
	}
	reason, ok := protocol.ClientErrorMessage(quotaErr)
	if !ok {
		return
	}
	if _, err := s.goals.UsageLimitForSession(ctx, sessionKey, roundID, reason); err != nil &&
		!errors.Is(err, goalsvc.ErrGoalDisabled) &&
		!errors.Is(err, goalsvc.ErrGoalNotFound) &&
		!errors.Is(err, goalsvc.ErrGoalInvalidState) {
		s.loggerFor(ctx).Warn("标记 Room Goal 账号额度限制失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", err,
		)
	}
}
