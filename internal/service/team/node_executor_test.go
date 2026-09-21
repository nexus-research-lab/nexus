package team

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func TestDeliveryAttachmentUsesNativeRoomUploadAndVerifiedBytes(t *testing.T) {
	data := []byte("shared report")
	hash := sha256.Sum256(data)
	file := relaycontract.MessageAttachment{ID: "file", Name: "report.txt", Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/renew") {
			_, _ = w.Write([]byte(`{"code":"0000","data":{}}`))
			return
		}
		if r.URL.Path != "/api/relay/v1/node/deliveries/delivery/files/file" || r.Header.Get("X-Delivery-Lease") != "lease" || r.Header.Get("Authorization") != "Bearer machine" {
			t.Errorf("附件凭据/路径错误: %s", r.URL.Path)
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client, err := relaysvc.NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var uploads int
	executor := &NodeExecutor{relay: client, upload: func(_ context.Context, room, conversation, name, destination string, reader io.Reader) (*workspacesvc.UploadResult, error) {
		uploads++
		got, _ := io.ReadAll(reader)
		if room != "local-room" || conversation != "local-conversation" || name != file.Name || destination != "attachments/file" || !bytes.Equal(got, data) {
			t.Fatal("未使用精确原生 Room 附件作用域")
		}
		return &workspacesvc.UploadResult{Path: destination + "/" + name}, nil
	}}
	delivery := &relaycontract.Delivery{ID: "delivery", LeaseID: "lease", MessageID: "message", Messages: []relaycontract.Message{{ID: "message", Content: relaycontract.MessageContent{Version: 1, Attachments: []relaycontract.MessageAttachment{file}}}}}
	job := teamstore.NodeJob{RoomID: "local-room", ConversationID: "local-conversation", Delivery: delivery}
	content, _, err := deliveryRoomContext(delivery, "agent")
	if err != nil || content != "" {
		t.Fatalf("附件单独发送失败: %q %v", content, err)
	}
	attachments, err := executor.prepareDeliveryAttachments(t.Context(), job, "machine")
	if err != nil || len(attachments) != 1 || attachments[0].Scope != protocol.ChatAttachmentScopeRoomConversation || attachments[0].Kind != protocol.ChatAttachmentKindText {
		t.Fatalf("原生附件: %+v %v", attachments, err)
	}
	delivery.Messages[0].Content.Attachments[0].SHA256 = strings.Repeat("0", 64)
	if _, err = executor.prepareDeliveryAttachments(t.Context(), job, "machine"); err == nil || uploads != 1 {
		t.Fatal("校验失败的内容进入本机工作区")
	}
	delivery.Messages[0].Content.Blocks = []relaycontract.ContentBlock{{Type: "markdown", Text: "/browser @Lucy inspect report"}}
	content, _, err = deliveryRoomContext(delivery, "agent")
	if err != nil || content != "/browser @Lucy inspect report" {
		t.Fatalf("Slash 被改写: %q %v", content, err)
	}
}

func TestNodeExecutorWakesFromWebSocketWithoutTaskPolling(t *testing.T) {
	var pending atomic.Bool
	var reads, claims atomic.Int32
	hints := make(chan struct{}, 8)
	claimed := make(chan struct{}, 8)
	connected := make(chan struct{}, 8)
	disconnect := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/nodes/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nodeToken{Token: "machine", ExpiresAt: time.Now().Add(time.Minute), Agents: []nodeAgent{{AgentID: "online", SourceAgentID: "local"}}}})
		case "/ws/relay/node":
			if r.Header.Get("Authorization") != "Bearer machine" || r.Header.Get("Cookie") != "" {
				t.Error("node websocket mixed credentials")
			}
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow()
			ctx := conn.CloseRead(r.Context())
			connected <- struct{}{}
			for {
				if wsjson.Write(ctx, conn, relaycontract.StreamUpdated{Type: "deliveries.updated"}) != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-disconnect:
					return
				case <-hints:
				}
			}
		case "/api/relay/v1/node/deliveries/pending":
			reads.Add(1)
			ids := []string{}
			if pending.Load() {
				ids = append(ids, "online")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "0000", "data": map[string]any{"agent_ids": ids}})
		case "/api/relay/v1/node/deliveries/claim":
			claims.Add(1)
			pending.Store(false)
			_, _ = w.Write([]byte(`{"code":"0000","data":{"delivery":null}}`))
			claimed <- struct{}{}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", AppMode: "desktop", RemoteURL: server.URL, ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	repo := teamstore.NewRepository(cfg, newNodeTestDB(t))
	nodes, err := NewNodeService(cfg, repo, func(context.Context) ([]protocol.Agent, error) {
		return []protocol.Agent{{AgentID: "local", Status: "active"}}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := nodes.keys.EncryptEnvelope([]byte("machine"))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := repo.PrepareNodeGrant(t.Context(), teamstore.NodeGrant{Scope: "scope", OwnerUserID: "local-owner", NodeID: "node", RemoteURL: server.URL, CredentialEncrypted: credential, AgentIDs: []string{"online"}, ExecutionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	client, err := relaysvc.NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	executor := &NodeExecutor{nodes: nodes, relay: client, logger: slog.Default(), active: map[string]string{}, loop: duework.New(duework.Options{})}
	nodes.executor = executor
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { executor.Run(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("executor did not stop")
		}
	}()
	// 进程已启动后登记，必须由提交唤醒发现，不能等下一次扫描。
	if err = nodes.setNodeState(t.Context(), *grant, "pending", "authorized"); err != nil {
		t.Fatal(err)
	}
	wait := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(3 * time.Second):
			t.Fatal("missing websocket wake")
		}
	}
	wait(connected)
	time.Sleep(100 * time.Millisecond)
	before := reads.Load()
	time.Sleep(5200 * time.Millisecond)
	if before == 0 || reads.Load() != before || claims.Load() != 0 {
		t.Fatalf("idle task polling: before=%d after=%d claims=%d", before, reads.Load(), claims.Load())
	}
	pending.Store(true)
	hints <- struct{}{}
	wait(claimed)
	// 无新消息提示的积压在重连初始通知后仍会被发现。
	pending.Store(true)
	disconnect <- struct{}{}
	wait(connected)
	wait(claimed)
	if claims.Load() != 2 {
		t.Fatalf("claim count = %d", claims.Load())
	}
	if err = nodes.setNodeState(t.Context(), *grant, "authorized", "revoked"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	before = reads.Load()
	pending.Store(true)
	hints <- struct{}{}
	time.Sleep(100 * time.Millisecond)
	if reads.Load() != before || claims.Load() != 2 {
		t.Fatal("revoked node continued reading or claiming")
	}
}

func TestNodeFailureLoggingIsCorrelatedThrottledAndRedacted(t *testing.T) {
	var output bytes.Buffer
	executor := &NodeExecutor{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	grant := teamstore.NodeGrant{NodeID: "node", OwnerUserID: "owner"}
	job := teamstore.NodeJob{ID: "job", AgentID: "agent", Delivery: &relaycontract.Delivery{ID: "delivery", MessageID: "message"}}
	err := &relaycontract.RemoteError{StatusCode: 403, Code: "forbidden", RequestID: "request", Message: "SECRET_REMOTE_BODY"}
	executor.logFailure(t.Context(), "publish_output", grant, job, err)
	executor.logFailure(t.Context(), "publish_output", grant, job, err)
	if strings.Count(output.String(), "\n") != 1 || strings.Contains(output.String(), "SECRET_REMOTE_BODY") {
		t.Fatalf("日志重复或泄露正文: %s", &output)
	}
	var record map[string]any
	if decodeErr := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	for key, want := range map[string]any{"stage": "publish_output", "job_id": "job", "delivery_id": "delivery", "source_message_id": "message", "http_status": float64(403), "remote_request_id": "request"} {
		if record[key] != want {
			t.Fatalf("%s = %v, want %v", key, record[key], want)
		}
	}
	executor.logTimes["node:agent:publish_output"] = time.Now().Add(-2 * time.Minute)
	executor.logFailure(t.Context(), "publish_output", grant, job, err)
	executor.logFailure(t.Context(), "claim_delivery", grant, job, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	executor.logFailure(ctx, "cancelled", grant, job, err)
	if strings.Count(output.String(), "\n") != 3 {
		t.Fatalf("限频窗口或取消过滤不正确: %s", &output)
	}
	for reason, failure := range map[string]error{"node_grant_inactive": ErrNodeInactive, "node_credential_rejected": ErrNodeCredentialRejected} {
		output.Reset()
		executor.logFailure(t.Context(), reason, grant, teamstore.NodeJob{}, failure)
		if decodeErr := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if record["reason"] != reason || !errors.Is(failure, ErrNodeLogin) {
			t.Fatalf("授权失败原因丢失: %s", &output)
		}
	}
}

func TestDeliveryRoomContextUsesExactTriggerAndLocalAgent(t *testing.T) {
	delivery := &relaycontract.Delivery{AgentID: "remote", MessageID: "trigger", Messages: []relaycontract.Message{
		{ID: "prior", AuthorType: "agent", AuthorAgentID: "remote", AuthorUserID: "agent-owner"},
		{ID: "trigger", AuthorType: "user", AuthorUserID: "speaker", AuthorUsername: "test", AuthorDisplayName: "测试用户", Content: relaycontract.MessageContent{Blocks: []relaycontract.ContentBlock{{Type: "markdown", Text: "执行任务"}}}},
		{ID: "later", AuthorType: "user"},
	}}
	content, history, err := deliveryRoomContext(delivery, "local")
	if err != nil || content != "执行任务" || len(history) != 2 || history[0]["agent_id"] != "local" {
		t.Fatalf("unexpected Room context: %q %#v %v", content, history, err)
	}
	delivery.MessageID = "missing"
	if history[1]["author_user_id"] != "speaker" || history[1]["author_username"] != "test" || history[1]["author_display_name"] != "测试用户" || history[0]["author_user_id"] != nil {
		t.Fatalf("真人身份丢失或混入 Agent 所有者: %#v", history)
	}
	if _, _, err := deliveryRoomContext(delivery, "local"); err == nil {
		t.Fatal("missing trigger must not fall back to the latest message")
	}
}

func TestNodeMessageHistorySurvivesRecentWindowAndIsScoped(t *testing.T) {
	db := newNodeTestDB(t)
	repo := teamstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	for index := 0; index < 105; index++ {
		id := fmt.Sprintf("job-%03d", index)
		job := teamstore.NodeJob{ID: id, OwnerUserID: "local-owner", Scope: "scope", NodeID: "node", LocalAgentID: "local", State: "completed", Delivery: &relaycontract.Delivery{ID: "delivery-" + id, RoomID: "room", MessageID: id}}
		encoded, err := json.Marshal(job)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO team_node_jobs(id,node_id,owner_user_id,local_agent_id,state,scope,data_json,source_room_id,source_message_id,delivery_id) VALUES (?,?,?,?,?,?,?,?,?,?)`, id, "node", "local-owner", "local", "completed", "scope", string(encoded), "room", id, "delivery-"+id); err != nil {
			t.Fatal(err)
		}
	}
	recent, err := repo.NodeJobs(t.Context(), "local-owner", "scope")
	if err != nil || len(recent) != 100 {
		t.Fatalf("recent window: %d %v", len(recent), err)
	}
	for _, reference := range []string{"job-000", "delivery-job-000"} {
		jobs, err := repo.NodeMessageJobs(t.Context(), "local-owner", "scope", "room", []string{reference}, "")
		if err != nil || len(jobs) != 1 || jobs[0].ID != "job-000" {
			t.Fatalf("history lookup: %#v %v", jobs, err)
		}
	}
	for _, scope := range [][3]string{{"other-owner", "scope", "room"}, {"local-owner", "other-scope", "room"}, {"local-owner", "scope", "other-room"}} {
		jobs, err := repo.NodeMessageJobs(t.Context(), scope[0], scope[1], scope[2], nil, "job-000")
		if err != nil || len(jobs) != 0 {
			t.Fatalf("history crossed scope: %#v %v", jobs, err)
		}
	}
}

func TestNodeExecutionDurableOutputAndRevocationFence(t *testing.T) {
	ctx := t.Context()
	db := newNodeTestDB(t)
	var received []relaycontract.DeliveryOutput
	var ids []string
	loseReceipt := true
	uploads := 0
	renewStatus := http.StatusOK
	tokenRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "Bearer machine" {
			t.Error("machine request mixed browser credentials")
		}
		if r.URL.Path == "/auth/v1/nodes/token" {
			tokenRequests++
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nodeToken{Token: "machine", ExpiresAt: time.Now().Add(time.Minute)}})
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/delivery/files" {
			data, _ := io.ReadAll(r.Body)
			if string(data) != "original" || r.Header.Get("X-Delivery-Lease") != "lease" {
				t.Error("产物不是冻结内容或缺少租约")
			}
			uploads++
			sum := sha256.Sum256(data)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": relaycontract.MessageAttachment{ID: "shared-file", Name: "result.txt", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}})
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/delivery/renew" {
			if renewStatus != http.StatusOK {
				w.WriteHeader(renewStatus)
				_, _ = w.Write([]byte(`{"code":"lease_rejected","message":"lease unavailable"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":"0000","data":{}}`))
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/delivery/fail" {
			_, _ = w.Write([]byte(`{"code":"0000","data":{}}`))
			return
		}
		if r.URL.Path == "/api/relay/v1/node/deliveries/claim" {
			if r.Header.Get("Idempotency-Key") != "first" {
				t.Error("claim changed durable identity")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "0000", "data": map[string]any{"delivery": relaycontract.Delivery{ID: "delivery", NodeID: "node", AgentID: "online", LeaseID: "lease", State: "leased", RoomID: "online-room", MessageID: "message", Messages: []relaycontract.Message{{ID: "message", AuthorType: "user", Content: relaycontract.MessageContent{Version: 1, Blocks: []relaycontract.ContentBlock{{Type: "markdown", Text: "处理任务"}}}}}}}})
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
	nodes, err := NewNodeService(cfg, repo, nil, nil)
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
	if _, err = executor.cachedToken(ctx, *grant); err != nil || tokenRequests != 1 {
		t.Fatalf("valid machine token not reused: %d %v", tokenRequests, err)
	}
	executor.tokens[grant.NodeID] = cachedNodeToken{credential: secret, value: nodeToken{ExpiresAt: time.Now().Add(time.Second)}}
	if _, err = executor.cachedToken(ctx, *grant); err != nil || tokenRequests != 2 {
		t.Fatalf("expiring machine token not refreshed: %d %v", tokenRequests, err)
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
		job.Delivery = &relaycontract.Delivery{ID: "delivery", LeaseID: "lease", MessageID: "message", Messages: []relaycontract.Message{{ID: "message", AuthorType: "user", Content: relaycontract.MessageContent{Version: 1, Blocks: []relaycontract.ContentBlock{{Type: "markdown", Text: "处理任务"}}}}}}
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
		if request.Content != "处理任务" || len(request.PublicContext) != 1 || request.UserMessageID != "message" || !request.Internal {
			t.Errorf("在线消息未通过 Room 公区上下文适配: %+v", request)
		}
		if request.PermissionMode != "" || request.ExecutionOrigin != "relay" || len(request.TargetAgentIDs) != 1 || request.TargetAgentIDs[0] != "local" {
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
		mapped, err := mapper.Map(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult, UUID: "answer", Result: &sdkprotocol.ResultMessage{Subtype: "success", Result: "完成结果", DurationMS: 53400, TotalCostUSD: 0.0785, Usage: map[string]any{"input_tokens": 3500, "output_tokens": 1600, "cache_read_input_tokens": 40000}}})
		if err != nil {
			return err
		}
		for _, event := range mapped.Events {
			if event.Data["role"] == "assistant" {
				event.Data["model"] = "glm-5.3-flash"
				event.Data["recalled_memories"] = []any{"PRIVATE_MEMORY"}
			}
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
	stats := received[2].Content.Execution
	if stats == nil || stats.Model != "glm-5.3-flash" || stats.ResultSummary == nil || stats.ResultSummary.DurationMS != 53400 || stats.ResultSummary.Usage.CacheReadInputTokens != 40000 || *stats.ResultSummary.TotalCostUSD != 0.0785 {
		t.Fatalf("最终输出统计丢失: %+v", stats)
	}
	wire, _ := json.Marshal(received)
	if strings.Contains(string(wire), "PRIVATE_MEMORY") || strings.Contains(string(wire), "thinking") {
		t.Fatal("私人执行信息进入共享输出")
	}
	if err = executor.execute(ctx, *grant, job, nodeToken{}); !errors.Is(err, teamstore.ErrNodeConflict) || starts != 1 {
		t.Fatalf("replayed tools: %d %v", starts, err)
	}
	// 已结束执行的恢复区分暂时故障与明确拒绝，不重跑 runtime。
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusUnauthorized, http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusConflict} {
		pending := prepare(fmt.Sprintf("renew-%d", status), fmt.Sprintf("local-%d", status))
		pending.State = "running"
		if err := repo.SaveNodeJob(ctx, pending, "ready", nil); err != nil {
			t.Fatal(err)
		}
		pending.State = "draining"
		if err := repo.SaveNodeJob(ctx, pending, "running", outputText("lease", "assistant", "待交付结果", nil)); err != nil {
			t.Fatal(err)
		}
		renewStatus = status
		if err := executor.drain(ctx, *grant, pending, "machine"); err == nil {
			t.Fatal("续租拒绝不能当作成功")
		}
		current, err := repo.NodeJob(ctx, pending.OwnerUserID, pending.ID)
		want := "draining"
		if status == http.StatusConflict {
			want = "failed"
		}
		if err != nil || current == nil || current.State != want {
			t.Fatalf("续租 HTTP %d: job=%+v err=%v", status, current, err)
		}
		if starts != 1 || len(received) != 3 {
			t.Fatal("续租失败后不应重新执行或发布结果")
		}
	}
	renewStatus = http.StatusOK
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
	// 明确交付文件冻结到原 outbox，重复快照不再交付，普通工作文件不外发。
	filesJob := prepare("files", "file-agent")
	filesJob.State = "running"
	if err = repo.SaveNodeJob(ctx, filesJob, "ready", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "result.txt")
	if err = os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	executor.openFile = func(context.Context, string, string) (*os.File, string, error) {
		f, err := os.Open(path)
		return f, "result.txt", err
	}
	observer := nodeObserver{executor: executor}
	block := protocol.WorkspaceFileArtifactBlock{ID: "artifact", Type: protocol.ContentBlockTypeWorkspaceFileArtifact, Role: "deliverable", ProducerAgentID: "file-agent", WorkspaceAgentID: "file-agent", Scope: protocol.WorkspaceFileArtifactScopeAgentWorkspace, SourceAgentRoundID: "agent-round", Path: "result.txt"}
	event := protocol.EventMessage{AgentRoundID: "agent-round", Data: map[string]any{"content": []any{block.Map()}}}
	if err = observer.captureFiles(ctx, &filesJob, event); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = observer.captureFiles(ctx, &filesJob, event); err != nil {
		t.Fatal(err)
	}
	output, err := repo.NextNodeOutput(ctx, filesJob.ID)
	if err != nil || output == nil || len(output.Files) != 1 || string(output.Files[0].Data) != "original" || filesJob.Sequence != 1 {
		t.Fatal("交付内容未冻结或重复", output, err)
	}
	block.ID = "working"
	block.Role = "working_file"
	event.Data["content"] = []any{block.Map()}
	if err = observer.captureFiles(ctx, &filesJob, event); err != nil || filesJob.Sequence != 1 {
		t.Fatal("普通工作文件外发", err)
	}
	block.Role = "deliverable"
	block.SourceAgentRoundID = "other-round"
	event.Data["content"] = []any{block.Map()}
	if err = observer.captureFiles(ctx, &filesJob, event); err == nil {
		t.Fatal("接受其他轮次产物")
	}
	filesJob.State = "draining"
	if err = repo.SaveNodeJob(ctx, filesJob, "running", outputText("lease", "final", "完成", nil)); err != nil {
		t.Fatal(err)
	}
	if err = executor.drain(ctx, *grant, filesJob, "machine"); err != nil {
		t.Fatal(err)
	}
	if uploads != 1 || len(received) != 5 || len(received[3].Content.Attachments) != 1 || received[3].Content.Attachments[0].ID != "shared-file" {
		t.Fatal("未复用输出完成文件交付", uploads, received)
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

func TestNodeTerminalStatusDistinguishesInterruption(t *testing.T) {
	for _, status := range []string{"interrupted", "cancelled", "error"} {
		t.Run(status, func(t *testing.T) {
			repo := teamstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, newNodeTestDB(t))
			job, err := repo.PrepareNodeJob(t.Context(), teamstore.NodeJob{ID: "terminal", NodeID: "node", OwnerUserID: "local-owner", LocalAgentID: "local", Scope: "scope"})
			if err != nil {
				t.Fatal(err)
			}
			job.State = "running"
			if err := repo.SaveNodeJob(t.Context(), *job, "claiming", nil); err != nil {
				t.Fatal(err)
			}
			observer := nodeObserver{executor: &NodeExecutor{nodes: &NodeService{store: repo}, logger: slog.Default()}, done: make(chan struct{})}
			if err := observer.apply(t.Context(), job, protocol.EventMessage{EventType: protocol.EventTypeRoundStatus, Data: map[string]any{"is_terminal": true, "status": status}}); err != nil {
				t.Fatal(err)
			}
			saved, err := repo.NodeJob(t.Context(), job.OwnerUserID, job.ID)
			want := "cancelled"
			if status == "error" {
				want = "failed"
			}
			if err != nil || saved == nil || saved.State != want {
				t.Fatalf("终态=%+v, err=%v", saved, err)
			}
			if active, err := repo.ActiveNodeJob(t.Context(), job.OwnerUserID, job.LocalAgentID); err != nil || active != nil {
				t.Fatalf("终态仍占用执行槽: %+v %v", active, err)
			}
		})
	}
}
