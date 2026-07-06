package room

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

type quotaChecker interface {
	EnsureQuotaAvailable(context.Context, string) error
}

// SetQuotaChecker 注入订阅额度检查器。
func (s *RealtimeService) SetQuotaChecker(checker quotaChecker) {
	s.quota = checker
}

func (s *RealtimeService) ensureQuotaAvailable(ctx context.Context) error {
	if s.quota == nil {
		return nil
	}
	return s.quota.EnsureQuotaAvailable(ctx, authctx.OwnerUserID(ctx))
}
