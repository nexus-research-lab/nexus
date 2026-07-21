package dm

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
)

type fakeDMQuotaChecker struct {
	err error
}

func TestServiceHandleChatMarksInternalGoalUsageLimitedWhenQuotaExceeded(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(factory), permissionctx.NewContext())
	goalProvider := &fakeGoalContextProvider{}
	service.SetGoalContextProvider(goalProvider)
	quotaErr := subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}
	service.SetQuotaChecker(fakeDMQuotaChecker{err: quotaErr})

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "owner-internal-goal-quota",
		Role:   authsvc.RoleOwner,
	})
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Internal Goal Quota Agent"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(agentValue.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "internal-goal-quota", "")
	err = service.HandleChat(ctx, Request{
		SessionKey: sessionKey,
		Content:    "internal goal continuation",
		GoalID:     "goal-internal-quota",
		RoundID:    "round-internal-goal-quota",
		Internal:   true,
	})
	if !errors.Is(err, subscriptionsvc.ErrQuotaExceeded) {
		t.Fatalf("账号额度耗尽应阻止内部 Goal runtime，实际: %v", err)
	}
	if got := goalProvider.recordedUsageLimitReasons(); len(got) != 1 || got[0] != quotaErr.ClientMessage() {
		t.Fatalf("Goal usage limit reasons = %#v, want actionable subscription quota message", got)
	}
	if got := factory.LastOptions(); got.Model != "" || client.connectCalls != 0 {
		t.Fatalf("账号额度耗尽时不应创建或连接内部 Goal runtime: options=%+v connect_calls=%d", got, client.connectCalls)
	}
}

func (f fakeDMQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestServiceHandleChatBlocksRuntimeWhenQuotaExceeded(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(factory), permission)
	errQuota := errors.New("quota exceeded")
	service.SetQuotaChecker(fakeDMQuotaChecker{err: errQuota})

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "owner-quota",
		Role:   authsvc.RoleOwner,
	})
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Quota Agent"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(agentValue.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "quota", "")
	err = service.HandleChat(ctx, Request{
		SessionKey: sessionKey,
		Content:    "should not start runtime",
		RoundID:    "round-quota",
	})
	if !errors.Is(err, errQuota) {
		t.Fatalf("quota exceeded 应阻止 DM runtime，实际: %v", err)
	}
	if got := factory.LastOptions(); got.Model != "" || client.connectCalls != 0 {
		t.Fatalf("quota exceeded 时不应创建或连接 runtime: options=%+v connect_calls=%d", got, client.connectCalls)
	}
}
