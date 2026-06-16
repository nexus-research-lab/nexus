package automation

import (
	"context"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

func (s *Service) runtimeImagegenDefaultEnabled(ctx context.Context) bool {
	if s == nil || s.providers == nil {
		return false
	}
	_, err := s.providers.ResolveImageConfig(ctx, "")
	return err == nil
}
