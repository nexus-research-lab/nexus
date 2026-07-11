package capability

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestHandleCapabilitySummaryScopesCountsByOwner(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.ConnectorCredentialsKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	skillService := skillspkg.NewService(cfg, agentService, workspaceService)
	connectorService := connectorsvc.NewService(cfg, db)
	automationService := automationsvc.NewService(cfg, db, nil, nil, nil, nil, nil, nil)
	handler := New(handlershared.NewAPI(nil), skillService, connectorService, automationService)

	ctxA := capabilityOwnerContext("owner-a")
	ctxB := capabilityOwnerContext("owner-b")
	if _, err := agentService.CreateAgent(ctxA, protocol.CreateRequest{Name: "Owner A Agent"}); err != nil {
		t.Fatalf("创建 owner-a agent 失败: %v", err)
	}
	if _, err := agentService.CreateAgent(ctxB, protocol.CreateRequest{Name: "Owner B Agent"}); err != nil {
		t.Fatalf("创建 owner-b agent 失败: %v", err)
	}
	sourceDir := filepath.Join(t.TempDir(), "owner-a-skill")
	writeCapabilityTestSkill(t, sourceDir, "owner-a-skill")
	if _, err := skillService.ImportLocalPath(ctxA, sourceDir); err != nil {
		t.Fatalf("导入 owner-a skill 失败: %v", err)
	}
	insertCapabilityConnector(t, db, "owner-a", "github", "connected")

	summaryA := readCapabilitySummary(t, handler, ctxA)
	summaryB := readCapabilitySummary(t, handler, ctxB)
	if summaryA["connected_connectors_count"] != 1 || summaryB["connected_connectors_count"] != 0 {
		t.Fatalf("connector count 未按 owner 隔离: owner-a=%+v owner-b=%+v", summaryA, summaryB)
	}
	if summaryA["skills_count"] != summaryB["skills_count"]+1 {
		t.Fatalf("skill count 未按 owner 私有 registry 统计: owner-a=%+v owner-b=%+v", summaryA, summaryB)
	}
}

func capabilityOwnerContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func readCapabilitySummary(t *testing.T, handler *Handlers, ctx context.Context) map[string]int {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/capability/summary", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.HandleCapabilitySummary(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("summary HTTP 状态不正确: code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析 summary 响应失败: %v", err)
	}
	return payload.Data
}

func insertCapabilityConnector(t *testing.T, db *sql.DB, ownerUserID string, connectorID string, state string) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO connector_connections (owner_user_id, connector_id, state, credentials, auth_type)
VALUES (?, ?, ?, '{}', 'oauth2')`,
		ownerUserID,
		connectorID,
		state,
	)
	if err != nil {
		t.Fatalf("写入 connector 测试数据失败: %v", err)
	}
}

func writeCapabilityTestSkill(t *testing.T, root string, name string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建测试 skill 目录失败: %v", err)
	}
	content := `---
name: ` + name + `
title: Owner A Skill
description: 测试技能
---

# ` + name + `
`
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("写入测试 SKILL.md 失败: %v", err)
	}
}
