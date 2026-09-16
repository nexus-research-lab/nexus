package team

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

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

func TestNodeGrantRecoversExactIntentWithoutExposingCredentials(t *testing.T) {
	db := newNodeTestDB(t)
	var registered nodeStatus
	var originalCredential string
	posts, deletes := 0, 0
	remoteOwner := "remote-owner"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || len(r.Cookies()) != 1 || r.Cookies()[0].Name != "nexus_session" || r.Header.Get("Origin") != "http://"+r.Host {
			t.Error("不能转发宿主凭据或丢失固定 Origin")
		}
		var data any
		switch r.URL.Path {
		case "/auth/v1/status":
			data = nodeIdentity{Authenticated: true, UserID: remoteOwner, OrganizationID: "org"}
		case "/auth/v1/agents":
			data = []nodeAgent{{AgentID: "online", SourceAgentID: "local"}, {AgentID: "other-host", SourceAgentID: "elsewhere"}}
		case "/auth/v1/nodes":
			posts++
			var input struct {
				NodeID     string   `json:"node_id"`
				Name       string   `json:"name"`
				Credential string   `json:"credential"`
				AgentIDs   []string `json:"agent_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if originalCredential != "" && (input.Credential != originalCredential || input.NodeID != registered.NodeID) {
				t.Error("重试改变了已持久化的授权身份")
			}
			originalCredential = input.Credential
			registered = nodeStatus{NodeID: input.NodeID, Name: input.Name, AgentIDs: input.AgentIDs}
			// 模拟响应未知；第二次提交必须复用已落盘的同一个凭据。
			if posts == 1 {
				w.WriteHeader(503)
				return
			}
			data = registered
		default:
			if registered.NodeID == "" || r.URL.Path != "/auth/v1/nodes/"+registered.NodeID {
				w.WriteHeader(404)
				return
			}
			if r.Method == http.MethodDelete {
				deletes++
				now := time.Now()
				registered.RevokedAt = &now
				w.WriteHeader(503)
				return
			}
			data = registered
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", AppMode: "desktop", RemoteURL: server.URL, AuthSessionCookieName: "nexus_session", ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	repo := teamstore.NewRepository(cfg, db)
	local := func(context.Context) ([]protocol.Agent, error) {
		return []protocol.Agent{{AgentID: "local", Name: "Amy"}}, nil
	}
	service, err := NewNodeService(cfg, repo, local)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: "local-owner"})
	input := NodeConnectInput{Name: "Laptop", AgentIDs: []string{"online"}}
	if err = service.Connect(ctx, "", input); !errors.Is(err, ErrNodeLogin) {
		t.Fatalf("local-only login: %v", err)
	}
	if err = service.Connect(ctx, "session", NodeConnectInput{Name: "Laptop", AgentIDs: []string{"other-host"}}); !errors.Is(err, ErrNodeInput) {
		t.Fatalf("foreign host agent: %v", err)
	}
	service.keys = nil
	if err = service.Connect(ctx, "session", input); !errors.Is(err, credentials.ErrKeyUnavailable) || posts != 0 {
		t.Fatalf("missing key sent grant: %v", err)
	}
	service, _ = NewNodeService(cfg, repo, local)
	if err = service.Connect(ctx, "session", input); err == nil {
		t.Fatal("模拟的未知注册不能返回成功")
	}
	var payload string
	if err = db.QueryRow(`SELECT data_json FROM team_node_grants`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if originalCredential == "" || strings.Contains(payload, originalCredential) || strings.Contains(payload, `"session"`) {
		t.Fatal("凭据或 Cookie 明文进入数据库")
	}
	// 重启服务后仍重放原意图；更改输入不能覆盖待确认授权。
	service, _ = NewNodeService(cfg, repo, local)
	if err = service.Connect(ctx, "session", NodeConnectInput{Name: "Changed", AgentIDs: input.AgentIDs}); !errors.Is(err, teamstore.ErrNodeConflict) {
		t.Fatalf("changed intent: %v", err)
	}
	if err = service.Connect(ctx, "session", input); err != nil || posts != 2 {
		t.Fatalf("exact retry posts=%d: %v", posts, err)
	}
	view, err := service.View(ctx, "session")
	if err != nil || view.State != "authorized" || view.ExecutionAvailable || len(view.Candidates) != 1 {
		t.Fatalf("view=%+v: %v", view, err)
	}
	encoded, _ := json.Marshal(view)
	if strings.Contains(string(encoded), originalCredential) || strings.Contains(string(encoded), "CookieHash") {
		t.Fatal("浏览器视图泄漏凭据")
	}
	remoteOwner = "another-owner"
	view, err = service.View(ctx, "another-session")
	if err != nil || view.State != "disconnected" {
		t.Fatalf("remote owner isolation=%+v: %v", view, err)
	}
	remoteOwner = "remote-owner"
	if err = service.Revoke(ctx, "session"); err == nil {
		t.Fatal("未知撤销不能返回成功")
	}
	var state string
	if err = db.QueryRow(`SELECT state FROM team_node_grants`).Scan(&state); err != nil || state != "revoking" {
		t.Fatalf("state=%s: %v", state, err)
	}
	view, err = service.View(ctx, "session")
	if err != nil || view.State != "revoked" {
		t.Fatalf("revocation reconciliation=%+v: %v", view, err)
	}
	if err = db.QueryRow(`SELECT data_json FROM team_node_grants`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var grant teamstore.NodeGrant
	if err = json.Unmarshal([]byte(payload), &grant); err != nil || grant.CredentialEncrypted != "" {
		t.Fatal("撤销终态必须清掉加密凭据")
	}
	if err = service.Revoke(ctx, "session"); err != nil || deletes != 1 {
		t.Fatalf("duplicate revoke=%d: %v", deletes, err)
	}
	// 旧 Control 的 404 不能被当作终止记录；保留冻结状态，不能创建新意图。
	scope, owner, err := service.scope(ctx, "session")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.PrepareNodeGrant(ctx, teamstore.NodeGrant{Scope: scope, OwnerUserID: owner, NodeID: "not-yet-registered", Name: "pending", AgentIDs: input.AgentIDs})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Revoke(ctx, "session"); err == nil {
		t.Fatalf("unknown registration 404: %v", err)
	}
	view, err = service.View(ctx, "session")
	if err != nil || view.State != "revoking" {
		t.Fatalf("missing cancellation receipt: %+v %v", view, err)
	}
	// 新 Control 的精确终止记录可以安全解除冻结，且不要求保留原名称或 Agent 列表。
	now := time.Now()
	registered = nodeStatus{NodeID: "not-yet-registered", Name: "Revoked", AgentIDs: []string{}, RevokedAt: &now}
	view, err = service.View(ctx, "new-session")
	if err != nil || view.State != "revoked" {
		t.Fatalf("pending cancellation receipt: %+v %v", view, err)
	}
}
