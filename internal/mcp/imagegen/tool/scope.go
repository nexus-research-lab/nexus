package tool

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
)

func scopedToolContext(ctx context.Context, sctx contract.ServerContext) context.Context {
	ownerUserID := strings.TrimSpace(sctx.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "imagegen_mcp_runtime",
	})
}

func requireWorkspacePath(sctx contract.ServerContext) (string, error) {
	workspacePath := strings.TrimSpace(sctx.WorkspacePath)
	if workspacePath == "" {
		return "", errors.New("nexus_imagegen 缺少当前 Agent workspace")
	}
	return workspacePath, nil
}
