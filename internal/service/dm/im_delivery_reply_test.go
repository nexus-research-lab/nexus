package dm

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

func TestIMFeedbackQueuesAtOriginalSessionAndDispatchesOnlyOnce(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agents := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	prompts := make(chan string, 8)
	client.onQuery = func(_ context.Context, p string) { prompts <- p }
	manager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	permission := permissionctx.NewContext()
	service := NewService(cfg, agents, manager, permission)
	session := "agent:nexus:ws:dm:im-feedback-origin"
	sender := newDMTestSender("im-feedback")
	permission.BindSession(session, sender)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: authctx.SystemUserID, Role: authctx.RoleOwner})
	if err := service.HandleChat(ctx, Request{SessionKey: session, Content: "正在处理的原任务", RoundID: "original-round", BroadcastUserMessage: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-prompts:
	case <-time.After(2 * time.Second):
		t.Fatal("original task did not start")
	}
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = manager.CloseSession(closeCtx, session)
	})
	store := imdelivery.NewRepository(cfg, db)
	service.SetIMDeliveryStore(store, func(context.Context, string, string) error { return nil })
	d := imdelivery.Delivery{ID: "delivery", OwnerUserID: authctx.SystemUserID, Source: imdelivery.Source{Kind: "tool", AgentID: "nexus", SessionKey: session}, TargetSessionKey: "im", Content: "待确认草案"}
	if _, err = store.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	reply := imdelivery.Reply{ID: "feedback", DeliveryID: d.ID, OwnerUserID: d.OwnerUserID, Input: imdelivery.Input{ID: "human"}, Content: "确认收到", ForwardedContent: "来自微信联系人的反馈：确认收到"}
	reply, err = store.SaveReply(ctx, reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	_, location, err := service.resolveInputQueueLocation(ctx, session, "nexus")
	if err != nil {
		t.Fatal(err)
	}
	queued, err := service.inputQueue.Snapshot(location)
	if err != nil || len(queued) != 1 || queued[0].Source != protocol.InputQueueSourceIMDeliveryReply {
		t.Fatalf("feedback not durably queued: %+v %v", queued, err)
	}
	altered := queued[0]
	altered.Content = "批准执行"
	if err = service.dispatchIMDeliveryReply(ctx, session, altered); err == nil {
		t.Fatal("tampered feedback accepted")
	}
	if err = service.guideInputQueueItem(ctx, session, location, reply.ID); err == nil {
		t.Fatal("feedback injected into busy turn")
	}
	select {
	case p := <-prompts:
		t.Fatalf("busy task was interrupted: %s", p)
	default:
	}
	finishDMQueueRound(client, "original-finished")
	select {
	case p := <-prompts:
		if !strings.Contains(p, "确认收到") {
			t.Fatal(p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original session did not continue")
	}
	saved, err := store.GetReply(ctx, reply.OwnerUserID, reply.ID)
	if err != nil || saved.State != "started" {
		t.Fatalf("missing dispatch claim %+v %v", saved, err)
	}
	if err = service.AcceptIMDeliveryReply(ctx, d, reply); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-prompts:
		t.Fatalf("duplicate feedback executed: %s", p)
	default:
	}
	finishDMQueueRound(client, "feedback-finished")
	collectEventsUntil(t, sender.events, func(e protocol.EventMessage) bool {
		return e.EventType == protocol.EventTypeRoundStatus && e.Data["round_id"] == reply.ID && e.Data["status"] == "finished"
	})
}
