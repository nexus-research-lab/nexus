package echo

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func TestSettingsFinalizeContextOutlivesRequestCancellation(t *testing.T) {
	ownerCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-a",
		Role:   authctx.RoleOwner,
	})
	requestCtx, cancelRequest := context.WithCancel(ownerCtx)
	cancelRequest()

	finalizeCtx, cancelFinalize := newSettingsFinalizeContext(requestCtx)
	defer cancelFinalize()

	if err := finalizeCtx.Err(); err != nil {
		t.Fatalf("finalize context inherited request cancellation: %v", err)
	}
	if _, ok := finalizeCtx.Deadline(); !ok {
		t.Fatal("finalize context must remain bounded")
	}
	if ownerID := authctx.OwnerUserID(finalizeCtx); ownerID != "owner-a" {
		t.Fatalf("finalize context owner=%q, want owner-a", ownerID)
	}
}
