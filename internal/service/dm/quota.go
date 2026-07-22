// INPUT: 账号订阅额度、内部 Goal continuation 与当前 Goal 状态。
// OUTPUT: runtime 启动门禁，以及额度耗尽时的 Goal usage_limited 投影。
// POS: DM 账号额度和 Goal 状态之间的适配边界。
package dm

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type quotaChecker interface {
	EnsureQuotaAvailable(context.Context, string) error
}

// SetQuotaChecker 注入订阅额度检查器。
func (s *Service) SetQuotaChecker(checker quotaChecker) {
	s.quota = checker
}

func (s *Service) ensureQuotaAvailable(ctx context.Context) error {
	if s.quota == nil {
		return nil
	}
	return s.quota.EnsureQuotaAvailable(ctx, authctx.OwnerUserID(ctx))
}

func (s *Service) recordGoalQuotaLimit(ctx context.Context, sessionKey string, roundID string, quotaErr error) {
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
		s.loggerFor(ctx).Warn("标记 Goal 账号额度限制失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", err,
		)
	}
}
