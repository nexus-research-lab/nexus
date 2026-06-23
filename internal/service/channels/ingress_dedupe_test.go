package channels

import (
	"context"
	"errors"
	"testing"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

func TestIngressServiceDeduplicatesReqID(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	control := NewControlService(cfg, db, agentService, router)
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(control)

	request := IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "创建每天九点的新闻定时任务",
		RoundID: "evt-1",
		ReqID:   "message-1",
	}
	first, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("第一次 Accept 失败: %v", err)
	}
	second, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("重复 Accept 不应失败: %v", err)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("重复 req_id 不应再次下发 DM，实际请求数: %d", len(handler.requests))
	}
	if second == nil || !second.Duplicate {
		t.Fatalf("重复消息应返回 duplicate=true: %+v", second)
	}
	if second.SessionKey != first.SessionKey || second.RoundID != first.RoundID || second.ReqID != first.ReqID {
		t.Fatalf("重复消息返回的原始结果不一致: first=%+v second=%+v", first, second)
	}
}

func TestIngressServiceRetriesFailedReqID(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{err: errors.New("dm temporarily unavailable")}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	control := NewControlService(cfg, db, agentService, router)
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(control)

	request := IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "停止每日新闻定时任务",
		RoundID: "evt-1",
		ReqID:   "message-1",
	}
	if _, err := service.Accept(context.Background(), request); err == nil {
		t.Fatal("第一次 DM 失败应返回错误")
	}
	handler.err = nil
	result, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("失败后的同 req_id 应允许重试: %v", err)
	}
	if result == nil || result.Duplicate {
		t.Fatalf("失败重试成功不应标记 duplicate: %+v", result)
	}
	if len(handler.requests) != 2 {
		t.Fatalf("失败后重试应再次下发 DM，实际请求数: %d", len(handler.requests))
	}
}
