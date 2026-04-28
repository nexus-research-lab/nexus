package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestServiceImportsAndInstallsSkill(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "技能测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	items, err := service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 agent 技能失败: %v", err)
	}
	if !containsSkill(items, "memory-manager") {
		t.Fatalf("系统托管 skill 未暴露: %+v", items)
	}
	if !containsSkill(items, "room-collaboration") {
		t.Fatalf("Room 协作系统 skill 未暴露: %+v", items)
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "room-collaboration"); err == nil {
		t.Fatal("系统托管 room-collaboration skill 不应允许手动安装")
	}

	localSkillRoot := filepath.Join(t.TempDir(), "demo-skill")
	if err = os.MkdirAll(localSkillRoot, 0o755); err != nil {
		t.Fatalf("创建本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(localSkillRoot, "SKILL.md"), []byte(`---
name: demo-skill
title: Demo Skill
description: 这是一个测试技能
tags: [demo, test]
---

# demo-skill

skill body
`), 0o644); err != nil {
		t.Fatalf("写入本地 skill 失败: %v", err)
	}

	imported, err := service.ImportLocalPath(localSkillRoot)
	if err != nil {
		t.Fatalf("导入本地 skill 失败: %v", err)
	}
	if imported.Name != "demo-skill" {
		t.Fatalf("导入的 skill 名称不正确: %+v", imported)
	}

	installed, err := service.InstallSkill(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("安装 skill 失败: %v", err)
	}
	if !installed.Installed {
		t.Fatalf("安装后状态不正确: %+v", installed)
	}

	if err = service.UninstallSkill(ctx, agentValue.AgentID, "demo-skill"); err != nil {
		t.Fatalf("卸载 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("再次读取 agent 技能失败: %v", err)
	}
	for _, item := range items {
		if item.Name == "demo-skill" && item.Installed {
			t.Fatalf("卸载后仍显示 installed: %+v", item)
		}
	}
}

func TestSkillResponseSlicesMarshalAsEmptyArray(t *testing.T) {
	info := Info{
		Name: "demo-skill",
		Tags: firstNonEmptySlice(nil),
	}
	payload, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("序列化技能信息失败: %v", err)
	}
	if string(payload) == "" || !strings.Contains(string(payload), `"tags":[]`) {
		t.Fatalf("tags 未按协议序列化为空数组: %s", string(payload))
	}
}

func TestToStringSliceReturnsEmptySlice(t *testing.T) {
	if result := toStringSlice(""); result == nil || len(result) != 0 {
		t.Fatalf("空字符串应返回空切片，实际: %#v", result)
	}
	if result := toStringSlice(nil); result == nil || len(result) != 0 {
		t.Fatalf("nil 应返回空切片，实际: %#v", result)
	}
}

func TestParseSkillFrontmatterWithoutTagsReturnsEmptySlice(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: demo-skill
title: Demo Skill
description: no tags here
---

# Demo Skill
`, "demo-skill")
	if parsed.Tags == nil {
		t.Fatal("未声明 tags 时也必须返回空切片，不能是 nil")
	}
	if len(parsed.Tags) != 0 {
		t.Fatalf("未声明 tags 时应为空切片，实际: %#v", parsed.Tags)
	}
}

func containsSkill(items []Info, target string) bool {
	for _, item := range items {
		if item.Name == target {
			return true
		}
	}
	return false
}

func newSkillsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	return config.Config{
		Host:                      "127.0.0.1",
		Port:                      18012,
		ProjectName:               "nexus-skills-test",
		APIPrefix:                 "/nexus/v1",
		WebSocketPath:             "/nexus/v1/chat/ws",
		DefaultAgentID:            "nexus",
		WorkspacePath:             filepath.Join(root, "workspace"),
		CacheFileDir:              filepath.Join(root, "cache"),
		DatabaseDriver:            "sqlite",
		DatabaseURL:               filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI: "http://localhost:3000/capability/connectors",
	}
}

func migrateSkillsSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, skillsTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func skillsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
