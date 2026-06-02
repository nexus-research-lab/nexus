package agent_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestServiceBootstrapsMainAgentAndCreatesAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()

	items, err := service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("列出主智能体失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("主智能体初始化数量不正确: got=%d", len(items))
	}
	if items[0].AgentID != cfg.DefaultAgentID {
		t.Fatalf("主智能体 ID 不匹配: got=%s want=%s", items[0].AgentID, cfg.DefaultAgentID)
	}
	if items[0].Options.Provider != "" {
		t.Fatalf("主智能体应跟随默认 provider，不应写死显式 provider: %+v", items[0].Options)
	}
	if items[0].Options.PermissionMode != "default" {
		t.Fatalf("主智能体默认权限应为询问模式: %+v", items[0].Options)
	}
	if len(items[0].Options.AllowedTools) != 0 {
		t.Fatalf("主智能体默认不应预授权工具: %+v", items[0].Options.AllowedTools)
	}
	assertRuntimeEmotionStateFile(t, items[0].WorkspacePath)

	validation, err := service.ValidateName(ctx, "测试助手", "")
	if err != nil {
		t.Fatalf("校验名称失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("名称应该可用: %+v", validation)
	}

	created, err := service.CreateAgent(ctx, protocol.CreateRequest{
		Name:        "测试助手",
		Description: "首个集成测试 agent",
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.AgentID == "" {
		t.Fatal("创建后的 agent_id 不能为空")
	}
	if _, err = os.Stat(created.WorkspacePath); err != nil {
		t.Fatalf("workspace 目录未创建: %v", err)
	}
	assertRuntimeEmotionStateFile(t, created.WorkspacePath)
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-a"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-a 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-a", "SKILL.md"), []byte("# skill-a\n"), 0o644); err != nil {
		t.Fatalf("写入测试 skill-a 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".claude", "skills", "skill-b"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-b 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(created.WorkspacePath, ".claude", "skills", "skill-b", "SKILL.md"), []byte("# skill-b\n"), 0o644); err != nil {
		t.Fatalf("写入测试 skill-b 失败: %v", err)
	}

	loaded, err := service.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatalf("读取 agent 失败: %v", err)
	}
	if loaded.SkillsCount != 2 {
		t.Fatalf("skills_count 不正确: got=%d want=2", loaded.SkillsCount)
	}

	items, err = service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("再次列出 agent 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("agent 数量不正确: got=%d want=2", len(items))
	}
	for _, item := range items {
		if item.AgentID == created.AgentID && item.SkillsCount != 2 {
			t.Fatalf("list_agents skills_count 不正确: got=%d want=2", item.SkillsCount)
		}
	}

	validation, err = service.ValidateName(ctx, "测试助手", "")
	if err != nil {
		t.Fatalf("重复名称校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("重复名称应只作为展示名并允许复用: %+v", validation)
	}
}

func TestServicePersistsAgentRuntimeProviderModel(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	maxTurns := 6
	maxThinkingTokens := 2048
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{
		Name: "runtime-agent",
		Options: &protocol.Options{
			Provider:          "glm",
			Model:             "glm-5.1",
			PermissionMode:    "default",
			AllowedTools:      []string{"Read"},
			DisallowedTools:   []string{"Write"},
			MaxTurns:          &maxTurns,
			MaxThinkingTokens: &maxThinkingTokens,
			MCPServers:        map[string]any{"local": map[string]any{"command": "nexus-mcp"}},
			SettingSources:    []string{"project"},
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.Options.Provider != "glm" || created.Options.Model != "glm-5.1" {
		t.Fatalf("runtime provider/model 未持久化: %+v", created.Options)
	}

	nextName := "runtime-agent"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Name: &nextName,
		Options: &protocol.Options{
			Provider:        "kimi-code",
			Model:           "kimi-for-coding",
			PermissionMode:  "bypassPermissions",
			AllowedTools:    []string{"Read", "Edit"},
			DisallowedTools: []string{},
			SettingSources:  []string{"project"},
		},
	})
	if err != nil {
		t.Fatalf("更新 agent runtime 失败: %v", err)
	}
	if updated.Options.Provider != "kimi-code" || updated.Options.Model != "kimi-for-coding" {
		t.Fatalf("runtime provider/model 更新后未持久化: %+v", updated.Options)
	}
	if updated.Options.MaxTurns == nil || *updated.Options.MaxTurns != maxTurns {
		t.Fatalf("未提交 max_turns 时应保留原值: %+v", updated.Options)
	}
	if updated.Options.MaxThinkingTokens == nil || *updated.Options.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("未提交 max_thinking_tokens 时应保留原值: %+v", updated.Options)
	}
	if _, ok := updated.Options.MCPServers["local"]; !ok {
		t.Fatalf("未提交 mcp_servers 时应保留原值: %+v", updated.Options.MCPServers)
	}
}

func TestServiceAllowsSelfNameValidationAndCaseOnlyRename(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "sam"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "Sam", created.AgentID)
	if err != nil {
		t.Fatalf("大小写改名校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("同一 agent 只改大小写时名称应该可用: %+v", validation)
	}

	nextName := "Sam"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("大小写改名失败: %v", err)
	}
	if updated.Name != "Sam" {
		t.Fatalf("大小写改名未生效: %+v", updated)
	}
}

func TestServiceAllowsDuplicateAndSlugCollidingAgentNames(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	first, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a b"})
	if err != nil {
		t.Fatalf("创建基准 agent 失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "a_b", "")
	if err != nil {
		t.Fatalf("校验 slug 冲突名称失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("名称派生 slug 冲突不应阻断创建: %+v", validation)
	}

	second, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a_b"})
	if err != nil {
		t.Fatalf("创建名称派生 slug 冲突 agent 不应失败: %v", err)
	}
	third, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a b"})
	if err != nil {
		t.Fatalf("重复展示名不应阻断创建: %v", err)
	}
	if first.AgentID == second.AgentID || first.AgentID == third.AgentID || second.AgentID == third.AgentID {
		t.Fatalf("重复展示名应创建独立 agent_id: first=%s second=%s third=%s", first.AgentID, second.AgentID, third.AgentID)
	}
	if slug := agentSlug(t, db, first.AgentID); slug != first.AgentID {
		t.Fatalf("新建 agent slug 应绑定 agent_id: got=%s want=%s", slug, first.AgentID)
	}
	if slug := agentSlug(t, db, second.AgentID); slug != second.AgentID {
		t.Fatalf("新建 agent slug 应绑定 agent_id: got=%s want=%s", slug, second.AgentID)
	}

	nextName := "a_b"
	updated, err := service.UpdateAgent(ctx, first.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("改成其他 agent 的展示名不应失败: %v", err)
	}
	if updated.Name != "a_b" {
		t.Fatalf("展示名改名未生效: %+v", updated)
	}
	if slug := agentSlug(t, db, first.AgentID); slug != first.AgentID {
		t.Fatalf("改名不应改变 agent slug: got=%s want=%s", slug, first.AgentID)
	}
}

func TestServiceHardDeletesAgentAndAllowsNameReuse(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "可重建助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("删除 agent 失败: %v", err)
	}

	assertNoRowsForAgent(t, db, "agents", "id", created.AgentID)
	assertNoRowsForAgent(t, db, "profiles", "agent_id", created.AgentID)
	assertNoRowsForAgent(t, db, "runtimes", "agent_id", created.AgentID)

	if _, err = service.GetAgent(ctx, created.AgentID); !errors.Is(err, agentpkg.ErrAgentNotFound) {
		t.Fatalf("硬删除后读取 agent 应返回不存在: %v", err)
	}

	recreated, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "可重建助手"})
	if err != nil {
		t.Fatalf("删除后应允许复用名称: %v", err)
	}
	if recreated.AgentID == created.AgentID {
		t.Fatalf("复用名称应创建新的 agent_id: old=%s new=%s", created.AgentID, recreated.AgentID)
	}
}

func TestServiceUsesAgentIDWorkspacePathAndRenameKeepsWorkspace(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "chatbuddy"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if filepath.Base(created.WorkspacePath) != agentpkg.BuildWorkspaceDirName(created.AgentID) {
		t.Fatalf("workspace 目录应使用 agent_id: got=%s agent_id=%s", created.WorkspacePath, created.AgentID)
	}
	workspaceMarker := filepath.Join(created.WorkspacePath, "marker.txt")
	if err = os.WriteFile(workspaceMarker, []byte("ok"), 0o644); err != nil {
		t.Fatalf("写入 workspace 标记失败: %v", err)
	}
	agentsFile := filepath.Join(created.WorkspacePath, "AGENTS.md")
	customAgentsContent := "# AGENTS.md\n\n用户自定义规则\n"
	if err = os.WriteFile(agentsFile, []byte(customAgentsContent), 0o644); err != nil {
		t.Fatalf("写入 AGENTS.md 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(cfg.WorkspacePath, "chat"), 0o755); err != nil {
		t.Fatalf("创建冲突候选目录失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "chat", created.AgentID)
	if err != nil {
		t.Fatalf("编辑态名称校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("agent_id 目录模式不应被同名目录阻断: %+v", validation)
	}

	nextName := "chat"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	if updated.Name != "chat" {
		t.Fatalf("改名未生效: %+v", updated)
	}
	if updated.WorkspacePath != created.WorkspacePath {
		t.Fatalf("改名不应移动 workspace_path: got=%s want=%s", updated.WorkspacePath, created.WorkspacePath)
	}
	if _, err = os.Stat(filepath.Join(updated.WorkspacePath, "marker.txt")); err != nil {
		t.Fatalf("workspace 内容应保留在原目录: %v", err)
	}
	agentsContent, err := os.ReadFile(agentsFile)
	if err != nil {
		t.Fatalf("读取 AGENTS.md 失败: %v", err)
	}
	if string(agentsContent) != customAgentsContent {
		t.Fatalf("改名不应重写 AGENTS.md 系统身份字段: %s", agentsContent)
	}
}

func TestDeleteAgentRemovesTranscriptProject(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "删除助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	projectDir := agentTranscriptProjectDir(created.WorkspacePath)
	if err = os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 项目目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, "delete-session.jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript 文件失败: %v", err)
	}
	if err = json.NewEncoder(file).Encode(map[string]any{
		"type":      "user",
		"uuid":      "delete-user-1",
		"sessionId": "delete-session",
		"message": map[string]any{
			"role":    "user",
			"content": "你好",
		},
	}); err != nil {
		_ = file.Close()
		t.Fatalf("写入 transcript 文件失败: %v", err)
	}
	_ = file.Close()

	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("删除 agent 失败: %v", err)
	}
	if _, err = os.Stat(projectDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 agent 后 transcript 项目目录仍残留: %v", err)
	}
}

func newTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18010,
		ProjectName:    "nexus-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

var agentTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func agentTranscriptProjectDir(workspacePath string) string {
	return filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeAgentTranscriptPath(canonicalizeAgentTranscriptPath(workspacePath)),
	)
}

func assertRuntimeEmotionStateFile(t *testing.T, workspacePath string) {
	t.Helper()
	statePath := filepath.Join(workspacePath, ".agents", "emotion.json")
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("emotion state 未初始化: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("emotion state 应为文件: %s", statePath)
	}
	if info.Size() != 0 {
		t.Fatalf("emotion state 初始文件应为空: size=%d", info.Size())
	}
}

func assertNoRowsForAgent(t *testing.T, db *sql.DB, table string, column string, value string) {
	t.Helper()

	var count int
	query := "SELECT COUNT(1) FROM " + table + " WHERE " + column + " = ?"
	if err := db.QueryRow(query, value).Scan(&count); err != nil {
		t.Fatalf("查询 %s.%s 失败: %v", table, column, err)
	}
	if count != 0 {
		t.Fatalf("删除 agent 后 %s 仍有残留: %d", table, count)
	}
}

func agentSlug(t *testing.T, db *sql.DB, agentID string) string {
	t.Helper()

	var slug string
	if err := db.QueryRow(`SELECT slug FROM agents WHERE id = ?`, agentID).Scan(&slug); err != nil {
		t.Fatalf("查询 agent slug 失败: %v", err)
	}
	return slug
}

func canonicalizeAgentTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if absolutePath, err := filepath.Abs(path); err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeAgentTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := agentTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + agentTranscriptHash(path)
}

func agentTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func migrateSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, testMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func testMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
