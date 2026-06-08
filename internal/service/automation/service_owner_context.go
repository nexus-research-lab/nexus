package automation

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func contextForJobOwner(ctx context.Context, job protocol.CronJob) context.Context {
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "automation",
	})
}

func backgroundContextForJobOwner(job protocol.CronJob) context.Context {
	return contextForJobOwner(context.Background(), job)
}
