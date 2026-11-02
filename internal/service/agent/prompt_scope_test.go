package agent

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// TestRuntimeScopeSectionStableAcrossEntryPoints 防止同一会话因驱动入口不同而翻转 system prompt。
func TestRuntimeScopeSectionStableAcrossEntryPoints(t *testing.T) {
	background := buildRuntimeScopeSection(context.Background(), "user_a")
	interactive := buildRuntimeScopeSection(authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:   "user_a",
		Username: "alice",
	}), "user_a")
	if background != interactive {
		t.Fatalf("scope section differs by entry point:\n%s\n---\n%s", background, interactive)
	}
}
