package channels

import (
	"errors"
	"testing"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestIngressRetryMarkerStopsBeforeRuntimeDispatch(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{err: errors.New("runtime admission uncertain")}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(NewControlService(cfg, db, agents, router))
	var retryable *channelcontract.RetryableIngressError
	if _, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat"}); !errors.As(err, &retryable) {
		t.Fatalf("准备失败应可安全重试: %v", err)
	}
	_, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello", ReqID: "message", RoundID: "round"})
	if err == nil || errors.As(err, &retryable) || len(handler.requests) != 1 {
		t.Fatalf("运行时错误不能标为安全重试: %v calls=%d", err, len(handler.requests))
	}
}
