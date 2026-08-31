package skills

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"

	_ "modernc.org/sqlite"
)

func TestVersionedURLImportRollsBackFilesAndVersionOnDatabaseFailure(t *testing.T) {
	var revision atomic.Int64
	revision.Store(1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := revision.Load()
		_, _ = writer.Write([]byte(`---
name: atomic-skill
title: Atomic Skill
description: revision ` + string(rune('0'+current)) + `
---

# revision ` + string(rune('0'+current)) + `
`))
	}))
	t.Cleanup(server.Close)

	cfg := newSkillsTestConfig(t)
	cfg.SkillsSourceURLs = "Atomic|" + server.URL
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewServiceWithDB(cfg, db, agentService, workspaceService)
	ctx := context.Background()

	state, err := service.GetCatalogState(ctx)
	if err != nil || state.Version != 1 {
		t.Fatalf("初始 catalog state=%+v err=%v", state, err)
	}
	if _, err = service.ImportSkillURLAtVersion(ctx, server.URL+"/SKILL.md", state.Version); err != nil {
		t.Fatalf("首次 URL 导入失败: %v", err)
	}
	state, err = service.GetCatalogState(ctx)
	if err != nil || state.Version != 2 {
		t.Fatalf("首次导入后 state=%+v err=%v", state, err)
	}
	snapshot, err := service.GetCatalogSkillState(ctx, "atomic-skill")
	if err != nil ||
		!snapshot.Exists ||
		snapshot.CatalogVersion != state.Version ||
		snapshot.SourceIdentity == "" {
		t.Fatalf("稳定 Skill snapshot=%+v err=%v", snapshot, err)
	}
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Skill 删除原子性"})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "atomic-skill"); err != nil {
		t.Fatalf("绑定导入 Skill 失败: %v", err)
	}

	if _, err = db.Exec(`
CREATE TRIGGER fail_atomic_skill_update
BEFORE UPDATE ON imported_skills
BEGIN
    SELECT RAISE(ABORT, 'forced imported skill update failure');
END`); err != nil {
		t.Fatalf("创建 update failure trigger 失败: %v", err)
	}
	revision.Store(2)
	if _, err = service.UpdateSingleSkillAtVersion(
		ctx,
		"atomic-skill",
		state.Version,
	); err == nil || SkillMutationNeedsReconcile(err) {
		t.Fatalf("DB 失败应完整回滚且返回普通失败，err=%v", err)
	}
	afterFailure, err := service.GetCatalogState(ctx)
	if err != nil || afterFailure.Version != state.Version {
		t.Fatalf("失败导入改变 catalog version: state=%+v err=%v", afterFailure, err)
	}
	payload, err := os.ReadFile(filepath.Join(service.registryRoot(ctx), "atomic-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("读取回滚后的 Skill 失败: %v", err)
	}
	if string(payload) == "" || !strings.Contains(string(payload), "# revision 1") || strings.Contains(string(payload), "# revision 2") {
		t.Fatalf("DB 失败后未恢复旧 Skill 内容: %s", payload)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil || !skillEnabledForAgent(reloaded, "atomic-skill") {
		t.Fatalf("失败导入不应影响 Agent 引用: agent=%+v err=%v", reloaded, err)
	}
	if _, err = db.Exec("DROP TRIGGER fail_atomic_skill_update"); err != nil {
		t.Fatalf("删除 update failure trigger 失败: %v", err)
	}

	if _, err = service.UpdateSingleSkillAtVersion(
		ctx,
		"atomic-skill",
		state.Version,
	); err != nil {
		t.Fatalf("使用仍然有效的 expected version 重试失败: %v", err)
	}
	state, err = service.GetCatalogState(ctx)
	if err != nil || state.Version != 3 {
		t.Fatalf("成功更新后 state=%+v err=%v", state, err)
	}

	if _, err = db.Exec(`
CREATE TRIGGER fail_atomic_skill_delete
BEFORE DELETE ON imported_skills
BEGIN
    SELECT RAISE(ABORT, 'forced imported skill delete failure');
END`); err != nil {
		t.Fatalf("创建 delete failure trigger 失败: %v", err)
	}
	if err = service.DeleteSkillAtVersion(ctx, "atomic-skill", state.Version); err == nil ||
		SkillMutationNeedsReconcile(err) {
		t.Fatalf("DB 删除失败应完整回滚，err=%v", err)
	}
	afterFailure, err = service.GetCatalogState(ctx)
	if err != nil || afterFailure.Version != state.Version {
		t.Fatalf("失败删除改变 catalog version: state=%+v err=%v", afterFailure, err)
	}
	if _, err = service.GetSkillDetail(ctx, "atomic-skill", ""); err != nil {
		t.Fatalf("失败删除后 Skill 不应消失: %v", err)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil || !skillEnabledForAgent(reloaded, "atomic-skill") {
		t.Fatalf("失败删除不应提前移除 Agent 引用: agent=%+v err=%v", reloaded, err)
	}
	if _, err = db.Exec("DROP TRIGGER fail_atomic_skill_delete"); err != nil {
		t.Fatalf("删除 delete failure trigger 失败: %v", err)
	}

	if err = service.DeleteSkillAtVersion(ctx, "atomic-skill", state.Version); err != nil {
		t.Fatalf("版本化删除 Skill 失败: %v", err)
	}
	state, err = service.GetCatalogState(ctx)
	if err != nil || state.Version != 4 {
		t.Fatalf("成功删除后 state=%+v err=%v", state, err)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil || skillEnabledForAgent(reloaded, "atomic-skill") {
		t.Fatalf("成功删除后 Agent 引用未清理: agent=%+v err=%v", reloaded, err)
	}
}

func TestCatalogVersionRejectsStaleRemoteImportAndSourceUpdate(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())
	t.Cleanup(server.Close)
	server.Config.Handler.(*http.ServeMux).HandleFunc("/first/SKILL.md", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("---\nname: first-skill\ntitle: First\n---\n\n# First\n"))
	})
	server.Config.Handler.(*http.ServeMux).HandleFunc("/second/SKILL.md", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("---\nname: second-skill\ntitle: Second\n---\n\n# Second\n"))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsSourceURLs = "Test|" + server.URL
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()
	state, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取 catalog state 失败: %v", err)
	}
	if _, err = service.ImportSkillURLAtVersion(
		ctx,
		server.URL+"/first/SKILL.md",
		state.Version,
	); err != nil {
		t.Fatalf("导入第一个 Skill 失败: %v", err)
	}
	if _, err = service.ImportSkillURLAtVersion(
		ctx,
		server.URL+"/second/SKILL.md",
		state.Version,
	); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("过期远端导入 err=%v, want catalog conflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(service.registryRoot(ctx), "second-skill")); !os.IsNotExist(statErr) {
		t.Fatalf("过期远端导入发布了目录: %v", statErr)
	}

	sources, err := service.ListExternalSkillSources(ctx)
	if err != nil || len(sources) == 0 {
		t.Fatalf("读取来源失败: sources=%+v err=%v", sources, err)
	}
	current, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取当前 catalog state 失败: %v", err)
	}
	disabled := false
	if _, err = service.UpdateExternalSkillSourceAtVersion(
		ctx,
		sources[0].SourceID,
		ExternalSkillSourceRequest{Enabled: &disabled},
		current.Version,
	); err != nil {
		t.Fatalf("版本化更新来源失败: %v", err)
	}
	if _, err = service.UpdateExternalSkillSourceAtVersion(
		ctx,
		sources[0].SourceID,
		ExternalSkillSourceRequest{Enabled: &disabled},
		current.Version,
	); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("过期来源更新 err=%v, want catalog conflict", err)
	}
}

func TestConcurrentVersionedRemoteImportsHaveSingleWinner(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	mux.HandleFunc("/alpha/SKILL.md", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("---\nname: alpha-skill\ntitle: Alpha\n---\n\n# Alpha\n"))
	})
	mux.HandleFunc("/beta/SKILL.md", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("---\nname: beta-skill\ntitle: Beta\n---\n\n# Beta\n"))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsSourceURLs = "Race|" + server.URL
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
	service := NewServiceWithDB(cfg, db, nil, nil)
	state, err := service.GetCatalogState(context.Background())
	if err != nil {
		t.Fatalf("读取初始 catalog state 失败: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, skillPath := range []string{"/alpha/SKILL.md", "/beta/SKILL.md"} {
		wait.Add(1)
		go func(path string) {
			defer wait.Done()
			<-start
			_, importErr := service.ImportSkillURLAtVersion(
				context.Background(),
				server.URL+path,
				state.Version,
			)
			results <- importErr
		}(skillPath)
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrCatalogVersionConflict):
			conflicts++
		default:
			t.Fatalf("并发远端导入返回未知错误: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("并发导入 successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
	current, err := service.GetCatalogState(context.Background())
	if err != nil || current.Version != state.Version+1 {
		t.Fatalf("并发导入后 state=%+v err=%v", current, err)
	}
}
