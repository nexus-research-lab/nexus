package auth

import (
	"context"
	"testing"

	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

type fakeTokenUsageStore struct {
	ownerUserID string
	summary     usagesvc.Summary
}

func (f *fakeTokenUsageStore) Summary(_ context.Context, ownerUserID string) (usagesvc.Summary, error) {
	f.ownerUserID = ownerUserID
	return f.summary, nil
}

func TestTokenUsageSummaryReadsLedgerOnly(t *testing.T) {
	store := &fakeTokenUsageStore{
		summary: usagesvc.Summary{
			InputTokens:  11,
			OutputTokens: 7,
			TotalTokens:  18,
			SessionCount: 1,
			MessageCount: 1,
		},
	}
	handler := &Handlers{
		usage: store,
	}
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user_1",
		Username: "admin",
		Role:     authsvc.RoleOwner,
	})

	result, err := handler.buildTokenUsageSummary(ctx)
	if err != nil {
		t.Fatalf("读取 token 用量失败: %v", err)
	}
	if store.ownerUserID != "user_1" {
		t.Fatalf("应按当前用户读取 ledger: %q", store.ownerUserID)
	}
	if result.TotalTokens != 18 || result.InputTokens != 11 || result.OutputTokens != 7 {
		t.Fatalf("应直接返回 ledger 汇总: %+v", result)
	}
}
