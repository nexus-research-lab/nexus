package dm

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
