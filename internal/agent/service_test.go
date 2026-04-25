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

	agentsvc "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestServiceBootstrapsMainAgentAndCreatesAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := bootstrap.NewAgentService(cfg)
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

	validation, err := service.ValidateName(ctx, "测试助手", "")
	if err != nil {
		t.Fatalf("校验名称失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("名称应该可用: %+v", validation)
	}

	created, err := service.CreateAgent(ctx, agentsvc.CreateRequest{
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
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-a"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-a 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-b"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-b 失败: %v", err)
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
	if validation.IsAvailable {
		t.Fatalf("重复名称不应可用: %+v", validation)
	}
}

func TestServiceAllowsSelfNameValidationAndCaseOnlyRename(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, agentsvc.CreateRequest{Name: "sam"})
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
	updated, err := service.UpdateAgent(ctx, created.AgentID, agentsvc.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("大小写改名失败: %v", err)
	}
	if updated.Name != "Sam" {
		t.Fatalf("大小写改名未生效: %+v", updated)
	}
}

func TestDeleteAgentRemovesTranscriptProject(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, agentsvc.CreateRequest{Name: "删除助手"})
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
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
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

	db, err := sql.Open("sqlite3", databaseURL)
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
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
