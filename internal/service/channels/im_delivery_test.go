package channels

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

type imTestSessions map[string]protocol.Session

func (s imTestSessions) ResolveDeliverySession(_ context.Context, key string) (*protocol.Session, error) {
	v, ok := s[key]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

func TestTrackedIMDeliveryKeepsExactOriginAndNeverReplaysUnknown(t *testing.T) {
	root, workspace := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-a")
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", WorkspacePath: root}
	agents := &stubAgentResolver{agentByID: map[string]*protocol.Agent{"agent-a": {AgentID: "agent-a", OwnerUserID: authctx.SystemUserID, WorkspacePath: workspace}}}
	router := NewRouter(cfg, db, agents, nil)
	external := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	router.RegisterForOwner(authctx.SystemUserID, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	control := NewControlService(cfg, db, agents, router)
	paired, err := control.CreatePairing(context.Background(), authctx.SystemUserID, CreatePairingRequest{ChannelType: ChannelTypeWeixinPersonal, AccountID: "account", ChatType: "dm", ExternalRef: "person", AgentID: "agent-a", Status: PairingStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	sourceA := "agent:agent-a:ws:dm:task-a"
	sourceB := "agent:agent-a:ws:dm:task-b"
	target := paired.SessionKey
	now := time.Now().UTC()
	sessions := imTestSessions{}
	for _, key := range []string{sourceA, sourceB, target} {
		sessions[key] = protocol.Session{SessionKey: key, AgentID: "agent-a", CreatedAt: now, ChatType: "dm", Title: key}
	}
	router.SetSessionProjectionResolver(sessions)
	store := imdelivery.NewRepository(cfg, db)
	router.SetIMDeliverySupport(store, control)
	ctx := ingressTestOwnerContext(authctx.SystemUserID)
	if _, err = router.RememberSessionRoute(ctx, "agent-a", target, DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, AccountID: "account", To: "person", SessionKey: target}); err != nil {
		t.Fatal(err)
	}
	send := func(source, call string) (DeliveryResult, error) {
		return control.SendAgentExternalSessionMessage(WithIMDeliverySource(ctx, imdelivery.Source{Kind: "tool", AgentID: "agent-a", SessionKey: source, RoundID: "round", CallID: call}), authctx.SystemUserID, "agent-a", target, "相同的草案")
	}
	first, err := send(sourceA, "call")
	if err != nil {
		t.Fatal(err)
	}
	retry, err := send(sourceA, "call")
	if err != nil || retry.DeliveryID != first.DeliveryID || external.sentCount() != 1 {
		t.Fatalf("duplicate physical send %+v %v count=%d", retry, err, external.sentCount())
	}
	second, err := send(sourceB, "call")
	if err != nil || second.DeliveryID == first.DeliveryID {
		t.Fatalf("different sessions collapsed %v", err)
	}
	rows, err := store.List(ctx, authctx.SystemUserID, target, "草案", 0, 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("query %v %v", rows, err)
	}
	pending, err := router.stageIMDelivery(ctx, imdelivery.Source{Kind: "tool", AgentID: "agent-a", SessionKey: sourceA, RoundID: "round", CallID: "uncertain"}, "agent-a", target, "相同的草案")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSend(ctx, authctx.SystemUserID, pending.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = send(sourceA, "uncertain"); err == nil || external.sentCount() != 2 {
		t.Fatal("unknown delivery replayed")
	}

	// The evidence comes from ingress, not transcript text or an arbitrary round.
	if _, err = db.Exec(`INSERT INTO im_ingress_messages(owner_user_id,channel_type,account_id,req_id,agent_id,session_key,round_id,status) VALUES (?,?,?,?,?,?,?,?)`, authctx.SystemUserID, ChannelTypeWeixinPersonal, "account", "human-message", "agent-a", target, "human-round", "processing"); err != nil {
		t.Fatal(err)
	}
	request := normalizedIngressRequest{ownerUserID: authctx.SystemUserID, agentID: "agent-a", sessionKey: target, parsed: protocol.ParseSessionKey(target), roundID: "human-round", reqID: "human-message", content: "确认第一份"}
	if err = control.recordDeliveryInput(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "human-round", "确认第一份"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ round, content string }{{"other-round", "确认第一份"}, {"human-round", "伪造批准"}} {
		if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, bad.round, bad.content); err == nil {
			t.Fatal("forged human input accepted")
		}
	}
	if _, err = db.Exec(`UPDATE im_ingress_messages SET status='failed' WHERE req_id='human-message'`); err != nil {
		t.Fatal(err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "human-round", "确认第一份"); err == nil {
		t.Fatal("failed input accepted")
	}

	// 历史 failed 也没有证明副作用未发生，不能重领并换掉人类输入证据。
	claimed, duplicate, err := control.claimIngressMessage(ctx, ingressMessageClaimInput{OwnerUserID: authctx.SystemUserID, Channel: ChannelTypeWeixinPersonal, AccountID: "account", ReqID: "human-message", AgentID: "agent-a", SessionKey: target, RoundID: "retry-round"})
	if claimed || !errors.Is(err, ErrIngressOutcomeUnknown) || duplicate == nil || duplicate.RoundID != "human-round" {
		t.Fatalf("legacy failed claim changed evidence: %v %+v %v", claimed, duplicate, err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "retry-round", "确认第一份"); err == nil {
		t.Fatal("unknown input became a new approval")
	}
	status := PairingStatusDisabled
	if _, err = control.UpdatePairing(ctx, authctx.SystemUserID, paired.PairingID, UpdatePairingRequest{Status: &status}); err != nil {
		t.Fatal(err)
	}
	status = PairingStatusActive
	if _, err = control.UpdatePairing(ctx, authctx.SystemUserID, paired.PairingID, UpdatePairingRequest{Status: &status}); err != nil {
		t.Fatal(err)
	}
	old, err := store.GetDelivery(ctx, authctx.SystemUserID, first.DeliveryID)
	if err != nil || !old.ReturnRevoked || old.State != "sent" {
		t.Fatalf("pairing revival restored old return address or changed send fact %+v %v", old, err)
	}
	if _, err = send(sourceA, "call"); !errors.Is(err, imdelivery.ErrConflict) && err == nil {
		t.Fatal("revoked invocation unexpectedly succeeded")
	}
}
