package team

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRoomMembershipProvisionsAndRecoversNode(t *testing.T) {
	var registered nodeStatus
	posts, deletes, tokenStatus := 0, 0, http.StatusOK
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/auth/v1/status":
			data = nodeIdentity{Authenticated: true, UserID: "remote", OrganizationID: "org"}
		case "/auth/v1/agents":
			data = []nodeAgent{{AgentID: "online", SourceAgentID: "local"}, {AgentID: "foreign", SourceAgentID: "other-host"}}
		case "/auth/v1/nodes/token":
			if tokenStatus != http.StatusOK {
				w.WriteHeader(tokenStatus)
				return
			}
			data = nodeToken{Token: "token", ExpiresAt: time.Now().Add(time.Minute)}
		case "/auth/v1/nodes":
			posts++
			var input struct {
				NodeID   string   `json:"node_id"`
				Name     string   `json:"name"`
				AgentIDs []string `json:"agent_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if len(input.AgentIDs) != 1 || input.AgentIDs[0] != "online" {
				t.Error("不能授权其他宿主 Agent")
			}
			registered = nodeStatus{NodeID: input.NodeID, Name: input.Name, AgentIDs: input.AgentIDs}
			data = registered
		default:
			if r.Method == http.MethodDelete {
				deletes++
				tokenStatus = http.StatusOK
			} else {
				w.WriteHeader(404)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer remote.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", AppMode: "desktop", RemoteURL: remote.URL, AuthSessionCookieName: "session", ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	repo := teamstore.NewRepository(cfg, newNodeTestDB(t))
	members := []relaycontract.RoomMember{{Type: "agent", ID: "online", State: "active"}, {Type: "agent", ID: "foreign", State: "active"}}
	service, err := NewNodeService(cfg, repo, func(context.Context) ([]protocol.Agent, error) {
		return []protocol.Agent{{AgentID: "local", Name: "Lucy", Status: "active"}}, nil
	}, func(context.Context, string, string) (relaycontract.RoomDetails, error) {
		return relaycontract.RoomDetails{Members: members}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service.executor = &NodeExecutor{nodes: service, prepare: func(context.Context, string, string) (*protocol.ConversationContextAggregate, error) {
		return &protocol.ConversationContextAggregate{Room: protocol.RoomRecord{ID: "room"}, Conversation: protocol.ConversationRecord{ID: "conversation"}}, nil
	}}
	service.executor.ready.Store(true)
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: "local-owner"})
	prepare := func() error {
		_, err := service.PrepareRooms(ctx, "cookie", []string{"online-room", "another-room", "online-room"})
		return err
	}
	if _, err := service.PrepareRoom(ctx, "", "online-room"); !errors.Is(err, ErrNodeLogin) {
		t.Fatal("本地免登录不可登记", err)
	}
	if err := prepare(); err != nil {
		t.Fatal(err)
	}
	session, _ := service.session(ctx, "cookie")
	scope := session.scope
	grant, err := repo.NodeGrant(ctx, scope, "local-owner")
	if err != nil || grant == nil || !grant.ExecutionEnabled || grant.State != "authorized" {
		t.Fatalf("入群未启用: %+v %v", grant, err)
	}
	if err := prepare(); err != nil || posts != 1 {
		t.Fatalf("重复准备重新登记: %v posts=%d", err, posts)
	}
	tokenStatus = http.StatusServiceUnavailable
	if err := prepare(); err == nil || posts != 1 || deletes != 0 {
		t.Fatal("网络故障不能重建授权")
	}
	tokenStatus = http.StatusUnauthorized
	job, err := repo.PrepareNodeJob(ctx, teamstore.NodeJob{ID: "inflight", NodeID: grant.NodeID, OwnerUserID: "local-owner", LocalAgentID: "local", AgentID: "online", Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	if err := prepare(); err == nil || deletes != 0 {
		t.Fatal("不能在已有任务收尾前替换节点")
	}
	job.State = "completed"
	if err := repo.SaveNodeJob(ctx, job, "claiming", nil); err != nil {
		t.Fatal(err)
	}
	if err := prepare(); err != nil || posts != 2 || deletes != 1 {
		t.Fatalf("未恢复已失效授权: %v posts=%d deletes=%d", err, posts, deletes)
	}
	members[0].State = "removed"
	if err := prepare(); err != nil || posts != 2 {
		t.Fatal("被移除成员不能重新登记")
	}
}

func TestRecoverJobRequiresExactStoppedRoundAndRemoteReceipt(t *testing.T) {
	remoteState := "leased"
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/auth/v1/status":
			data = nodeIdentity{Authenticated: true, UserID: "remote", OrganizationID: "org"}
		case "/auth/v1/nodes/node":
			data = nodeStatus{NodeID: "node", Name: "Node", AgentIDs: []string{"agent"}}
		case "/auth/v1/nodes/token":
			data = nodeToken{Token: "token", ExpiresAt: time.Now().Add(time.Hour)}
		case "/api/relay/v1/node/deliveries/claim":
			if r.Header.Get("Idempotency-Key") != "job" {
				t.Error("恢复更换了 claim")
			}
			data = map[string]any{"delivery": relaycontract.Delivery{ID: "delivery", LeaseID: "lease", State: remoteState}}
		case "/api/relay/v1/node/deliveries/delivery/fail":
			remoteState = "failed"
			data = map[string]any{}
		default:
			t.Errorf("意外调用 %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "0000", "data": data})
	}))
	defer remote.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", AppMode: "desktop", RemoteURL: remote.URL, ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	repo := teamstore.NewRepository(cfg, newNodeTestDB(t))
	svc, err := NewNodeService(cfg, repo, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: "local-owner"})
	session, err := svc.session(ctx, "cookie")
	scope := session.scope
	if err != nil {
		t.Fatal(err)
	}
	secret, err := svc.keys.EncryptEnvelope([]byte("credential"))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := repo.PrepareNodeGrant(ctx, teamstore.NodeGrant{Scope: scope, OwnerUserID: "local-owner", NodeID: "node", Name: "Node", AgentIDs: []string{"agent"}, RemoteURL: remote.URL, CredentialEncrypted: secret, ExecutionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SetNodeState(ctx, *grant, "pending", "authorized"); err != nil {
		t.Fatal(err)
	}
	job, err := repo.PrepareNodeJob(ctx, teamstore.NodeJob{ID: "job", NodeID: "node", OwnerUserID: "local-owner", LocalAgentID: "local", AgentID: "agent", Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	job.State = "running"
	job.RoundID = "round"
	job.ConversationID = "conversation"
	job.Delivery = &relaycontract.Delivery{ID: "delivery", LeaseID: "lease"}
	if err = repo.SaveNodeJob(ctx, job, "claiming", nil); err != nil {
		t.Fatal(err)
	}
	client, _ := relaysvc.NewClient(remote.URL, time.Second)
	stopped := false
	svc.executor = &NodeExecutor{nodes: svc, relay: client, loop: duework.New(duework.Options{}), stop: func(_ context.Context, request roomrealtime.InterruptRequest) error {
		if request.RoundID != "round" || request.SessionKey != protocol.BuildRoomSharedSessionKey("conversation") {
			t.Fatal("停止越界")
		}
		if stopped {
			return roomrealtime.ErrTargetRoomRoundNotRunning
		}
		return nil
	}}
	if err = svc.RecoverJob(ctx, "cookie", "job"); !errors.Is(err, teamstore.ErrNodeConflict) {
		t.Fatal("仅发送停止就解锁", err)
	}
	if current, _ := repo.NodeJob(ctx, "local-owner", "job"); current.State != "running" {
		t.Fatal("提前解锁")
	}
	stopped = true
	if err = svc.RecoverJob(ctx, "cookie", "job"); err != nil {
		t.Fatal(err)
	}
	if active, err := repo.ActiveNodeJob(ctx, "local-owner", "local"); err != nil || active != nil || remoteState != "failed" {
		t.Fatal("未释放执行槽", active, err)
	}
	if err = svc.RecoverJob(ctx, "cookie", "job"); err != nil {
		t.Fatal("重复恢复失败", err)
	}
}

func newNodeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "node.db")+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO owner_profiles(owner_user_id, username, display_name, role, status, created_at, updated_at) VALUES ('local-owner','owner','Owner','member','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	return db
}

func (s *NodeService) PrepareRoom(ctx context.Context, cookie, roomID string) ([]NodeRoomBinding, error) {
	return s.PrepareRooms(ctx, cookie, []string{roomID})
}
