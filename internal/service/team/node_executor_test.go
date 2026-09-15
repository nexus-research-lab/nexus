package team

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func TestNodeExecutionDurableOutputAndRevocationFence(t *testing.T) {
	ctx := t.Context()
	db := newNodeTestDB(t)
	var received []relaycontract.DeliveryOutput
	var ids []string
	loseReceipt := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "Bearer machine" {
			t.Error("machine request mixed browser credentials")
		}
		if r.URL.Path == "/auth/v1/nodes/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nodeToken{Token: "machine", ExpiresAt: time.Now().Add(time.Minute)}})
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/delivery/renew" {
			_, _ = w.Write([]byte(`{"code":"0000","data":{}}`))
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/claim" {
			if r.Header.Get("Idempotency-Key") != "first" {
				t.Error("claim changed durable identity")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "0000", "data": map[string]any{"delivery": relaycontract.Delivery{ID: "delivery", NodeID: "node", AgentID: "online", LeaseID: "lease", State: "leased", RoomID: "online-room", MessageID: "message", Messages: []relaycontract.Message{{ID: "message"}}}}})
			return
		}
		if r.URL.Path != "/api/relay/v1/node/deliveries/delivery/outputs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var output relaycontract.DeliveryOutput
		if err := json.NewDecoder(r.Body).Decode(&output); err != nil {
			t.Error(err)
		}
		received = append(received, output)
		ids = append(ids, r.Header.Get("Idempotency-Key"))
		if loseReceipt {
			loseReceipt = false
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"code":"0000","data":{}}`))
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", AppMode: "desktop", RemoteURL: server.URL, AuthSessionCookieName: "nexus_session", ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	repo := teamstore.NewRepository(cfg, db)
	nodes, err := NewNodeService(cfg, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := nodes.keys.EncryptEnvelope([]byte("machine"))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := repo.PrepareNodeGrant(ctx, teamstore.NodeGrant{Scope: "scope", OwnerUserID: "local-owner", NodeID: "node", RemoteURL: server.URL, CredentialEncrypted: secret, ExecutionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SetNodeState(ctx, *grant, "pending", "authorized"); err != nil {
		t.Fatal(err)
	}
	grant.State = "authorized"
	client, err := relaysvc.NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	executor := &NodeExecutor{nodes: nodes, relay: client, logger: slog.Default(), active: map[string]string{}}
	executor.prepare = func(_ context.Context, binding, agentID string) (*protocol.ConversationContextAggregate, error) {
		if binding != "scope:online-room" || agentID != "local" {
			t.Errorf("wrong local binding: %s %s", binding, agentID)
		}
		room := &protocol.ConversationContextAggregate{}
		room.Room.ID, room.Conversation.ID = "room", "conversation"
		return room, nil
	}
	if _, err = executor.machineToken(ctx, *grant); err != nil {
		t.Fatal(err)
	}
	wrongURL := *grant
	wrongURL.RemoteURL = "https://other.invalid"
	if _, err = executor.machineToken(ctx, wrongURL); !errors.Is(err, ErrNodeUnavailable) {
		t.Fatalf("credential origin fence: %v", err)
	}
	prepare := func(id, agent string) teamstore.NodeJob {
		t.Helper()
		job, err := repo.PrepareNodeJob(ctx, teamstore.NodeJob{ID: id, NodeID: "node", OwnerUserID: "local-owner", LocalAgentID: agent, AgentID: "online", Scope: "scope"})
		if err != nil {
			t.Fatal(err)
		}
		job.State = "ready"
		job.RoomID, job.ConversationID, job.RoundID = "room", "conversation", "relay_"+id
		job.Delivery = &relaycontract.Delivery{ID: "delivery", LeaseID: "lease", Messages: []relaycontract.Message{{ID: "message"}}}
		if err = repo.SaveNodeJob(ctx, *job, "claiming", nil); err != nil {
			t.Fatal(err)
		}
		return *job
	}
	job := prepare("first", "local")
	starts, stops := 0, 0
	executor.start = func(ctx context.Context, request roomrealtime.ChatRequest, admission func(context.Context) error) error {
		if err := admission(ctx); err != nil {
			return err
		}
		starts++
		if request.PermissionMode != sdkpermission.ModeDefault || request.ExecutionOrigin != "relay" || len(request.TargetAgentIDs) != 1 || request.TargetAgentIDs[0] != "local" {
			t.Errorf("unsafe admission: %+v", request)
		}
		emit := func(id, text, stop string, complete bool, mode string) {
			request.EventObserver(ctx, protocol.EventMessage{EventType: protocol.EventTypeMessage, RoundID: job.RoundID, AgentID: "local", MessageID: id, DeliveryMode: mode, Data: map[string]any{"role": "assistant", "message_id": id, "is_complete": complete, "stop_reason": stop, "content": []any{map[string]any{"type": "text", "text": text}, map[string]any{"type": "thinking", "thinking": "private"}}}})
		}
		emit("stream", "partial", "", false, protocol.DeliveryModeDurable)
		emit("transient", "private", "end_turn", true, protocol.DeliveryModeEphemeral)
		emit("intermediate", "工作进展", "tool_use", true, protocol.DeliveryModeDurable)
		emit("intermediate", "工作进展", "tool_use", true, protocol.DeliveryModeDurable)
		// 最终消息走真实 SDK→Nexus mapper，避免只用手写事件验证消费者。
		mapper := message.NewEventMapper(message.EventMapperOptions{Context: message.MessageContext{RoundID: job.RoundID, AgentID: "local", RoomID: job.RoomID, ConversationID: job.ConversationID}})
		mapped, err := mapper.Map(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult, UUID: "answer", Result: &sdkprotocol.ResultMessage{Subtype: "success", Result: "完成结果"}})
		if err != nil {
			return err
		}
		for _, event := range mapped.Events {
			request.EventObserver(ctx, event)
		}
		request.EventObserver(ctx, protocol.EventMessage{EventType: protocol.EventTypeRoundStatus, RoundID: job.RoundID, Data: map[string]any{"is_terminal": true, "status": "finished"}})
		return nil
	}
	executor.stop = func(_ context.Context, request roomrealtime.InterruptRequest) error {
		stops++
		if request.RoundID != job.RoundID || request.SessionKey != protocol.BuildRoomSharedSessionKey(job.ConversationID) {
			t.Error("stop escaped exact round")
		}
		return roomrealtime.ErrTargetRoomRoundNotRunning
	}
	if err = executor.consume(ctx, *grant, job, nodeToken{Token: "machine", ExpiresAt: time.Now().Add(time.Minute)}); err == nil {
		t.Fatal("lost receipt should remain pending")
	}
	current, err := repo.NodeJob(ctx, job.OwnerUserID, job.ID)
	if err != nil || current.State != "draining" || starts != 1 || stops != 1 {
		t.Fatalf("job=%+v starts=%d stops=%d: %v", current, starts, stops, err)
	}
	if err = executor.drain(ctx, *grant, *current, "machine"); err != nil {
		t.Fatal(err)
	}
	if len(received) != 3 || ids[0] == "" || ids[0] != ids[1] || ids[1] == ids[2] || received[1].Kind != "assistant" || received[2].Kind != "final" || received[1].Content.Blocks[0].Text != "工作进展" || received[2].Content.Blocks[0].Text != "完成结果" {
		t.Fatalf("outputs=%+v ids=%v", received, ids)
	}
	if out, err := repo.NextNodeOutput(ctx, job.ID); err != nil || out != nil {
		t.Fatalf("outbox=%+v %v", out, err)
	}
	if err = executor.execute(ctx, *grant, job, nodeToken{}); !errors.Is(err, teamstore.ErrNodeConflict) || starts != 1 {
		t.Fatalf("replayed tools: %d %v", starts, err)
	}
	// 两个进程式调用争抢同一条 inbox，只有一个能跨越副作用边界。
	concurrent := prepare("concurrent", "other-local")
	concurrent.State = "running"
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() { defer wait.Done(); results <- repo.SaveNodeJob(ctx, concurrent, "ready", nil) }()
	}
	wait.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, teamstore.ErrNodeConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("admitted %d executions", success)
	}
	unstarted := prepare("cancel", "cancel-local")
	if err = repo.SetNodeState(ctx, *grant, "authorized", "revoking"); err != nil {
		t.Fatal(err)
	}
	unstarted.State = "running"
	if err = repo.SaveNodeJob(ctx, unstarted, "ready", nil); !errors.Is(err, teamstore.ErrNodeConflict) {
		t.Fatalf("revoked start: %v", err)
	}
	if active, err := repo.ActiveNodeJob(ctx, "local-owner", "cancel-local"); err != nil || active != nil {
		t.Fatalf("unstarted cancellation stuck: %+v %v", active, err)
	}
	if active, err := repo.ActiveNodeJob(ctx, "local-owner", "other-local"); err != nil || active == nil || active.State != "running" {
		t.Fatalf("unknown run must remain blocked: %+v %v", active, err)
	}
}
