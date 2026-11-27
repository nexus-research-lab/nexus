// service_test.go 覆盖 Agent 服务与创建请求集成测试。
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
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	_ "modernc.org/sqlite"
)

func TestServiceKeepsBusinessTagsOutOfRuntimeProfile(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()

	agentValue, err := service.CreateAgent(ctx, protocol.CreateRequest{
		Name:         "business-tag-agent",
		BusinessTags: []string{"catalog-enterprise-x9"},
		VibeTags:     []string{"style-deliberate-y7"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(agentValue.BusinessTags, []string{"catalog-enterprise-x9"}) {
		t.Fatalf("业务标签未独立持久化: %+v", agentValue.BusinessTags)
	}

	agentValue, err = service.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{
		BusinessTags: []string{"Catalog-Retail-Z8", " catalog-retail-z8 ", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(agentValue.BusinessTags, []string{"Catalog-Retail-Z8"}) {
		t.Fatalf("业务标签应去空并忽略大小写重复: %+v", agentValue.BusinessTags)
	}
	if !slices.Equal(agentValue.VibeTags, []string{"style-deliberate-y7"}) {
		t.Fatalf("更新业务标签不应改写风格标签: %+v", agentValue.VibeTags)
	}

	prompt, err := service.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "style-deliberate-y7") {
		t.Fatal("运行时画像应保留风格标签")
	}
	if strings.Contains(prompt, "Catalog-Retail-Z8") {
		t.Fatal("业务标签不应进入运行时提示词")
	}
}

func TestServiceManagesSymmetricAgentContacts(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "contact-owner", Role: authctx.RoleOwner,
	})
	amy, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatal(err)
	}
	devin, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "Devin"})
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.AddAgentContact(ctx, amy.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: devin.AgentID, Alias: "开发搭档",
	})
	if err != nil {
		t.Fatalf("AddAgentContact() error = %v", err)
	}
	if created.ContactAgentID != devin.AgentID || created.Alias != "开发搭档" {
		t.Fatalf("created contact = %+v", created)
	}
	reverse, err := service.ListAgentContacts(ctx, devin.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reverse) != 1 || reverse[0].ContactAgentID != amy.AgentID || reverse[0].Alias != "" {
		t.Fatalf("reverse contacts = %+v", reverse)
	}
	if _, err = service.AddAgentContact(ctx, devin.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: amy.AgentID, Alias: "产品搭档",
	}); err != nil {
		t.Fatal(err)
	}
	forward, err := service.ListAgentContacts(ctx, amy.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(forward) != 1 || forward[0].Alias != "开发搭档" {
		t.Fatalf("peer alias update changed owner alias: %+v", forward)
	}
	mainAgent, err := service.GetDefaultAgent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddAgentContact(ctx, mainAgent.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: amy.AgentID,
	}); !errors.Is(err, agentpkg.ErrAgentContactUnsupported) {
		t.Fatalf("main Agent contact error = %v", err)
	}
	if err = service.DeleteAgentContact(ctx, devin.AgentID, amy.AgentID); err != nil {
		t.Fatal(err)
	}
	for _, agentID := range []string{amy.AgentID, devin.AgentID} {
		items, listErr := service.ListAgentContacts(ctx, agentID)
		if listErr != nil || len(items) != 0 {
			t.Fatalf("contacts after delete for %s = %+v, err = %v", agentID, items, listErr)
		}
	}
}

func TestServiceUsesRuntimeVersionForCompareAndSwap(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "versioned-agent"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.RuntimeVersion != 1 {
		t.Fatalf("初始 runtime_version = %d, want 1", created.RuntimeVersion)
	}

	expectedVersion := created.RuntimeVersion
	options := created.Options
	options.PermissionMode = "plan"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Options:                &options,
		ExpectedRuntimeVersion: &expectedVersion,
	})
	if err != nil {
		t.Fatalf("使用当前版本更新 agent 失败: %v", err)
	}
	if updated.RuntimeVersion != 2 {
		t.Fatalf("更新后 runtime_version = %d, want 2", updated.RuntimeVersion)
	}

	staleName := "stale-name"
	staleOptions := updated.Options
	staleOptions.PermissionMode = "bypassPermissions"
	if _, err = service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Name:                   &staleName,
		Options:                &staleOptions,
		ExpectedRuntimeVersion: &expectedVersion,
	}); !errors.Is(err, agentpkg.ErrRuntimeVersionConflict) {
		t.Fatalf("使用过期版本更新 error = %v, want ErrRuntimeVersionConflict", err)
	}

	current, err := service.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatalf("重新读取 agent 失败: %v", err)
	}
	if current.RuntimeVersion != 2 || current.Name != created.Name || current.Options.PermissionMode != "plan" {
		t.Fatalf("过期更新未整体回滚: %+v", current)
	}

	unconditionalOptions := current.Options
	unconditionalOptions.PermissionMode = "default"
	unconditional, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Options: &unconditionalOptions,
	})
	if err != nil {
		t.Fatalf("无 expected version 更新失败: %v", err)
	}
	if unconditional.RuntimeVersion != 3 {
		t.Fatalf("无条件更新后 runtime_version = %d, want 3", unconditional.RuntimeVersion)
	}
}

func TestDeleteAgentRemovesTranscriptProject(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)
	goalCleaner := &fakeAgentGoalCleaner{}
	service.SetGoalCleaner(goalCleaner)

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
	if len(goalCleaner.agentIDs) != 1 || goalCleaner.agentIDs[0] != created.AgentID {
		t.Fatalf("goal cleaner agent IDs = %#v, want deleted agent", goalCleaner.agentIDs)
	}
}

type fakeAgentGoalCleaner struct {
	agentIDs []string
}

func (f *fakeAgentGoalCleaner) DeleteGoalsForAgent(_ context.Context, agentID string) (int, error) {
	f.agentIDs = append(f.agentIDs, agentID)
	return 1, nil
}

func newTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
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
		appfs.UserRuntimeRoot(authctx.SystemUserID),
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
	handlertest.MigrateSQLiteFromDir(t, databaseURL, testMigrationDir(t))
}

func newAgentTestService(t *testing.T, cfg config.Config) (*agentpkg.Service, *sql.DB) {
	t.Helper()
	service, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭 Agent 测试数据库失败: %v", err)
		}
	})
	return service, db
}

func testMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}

func TestAgentCreationRequestReplaysExactResultAndSurvivesDeletion(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	request := protocol.CreateRequest{
		Name:              "Recovery Agent",
		CreationRequestID: "web-create:recovery-agent",
	}

	created, err := service.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("first CreateAgent() error = %v", err)
	}
	replayed, err := service.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("replayed CreateAgent() error = %v", err)
	}
	if replayed.AgentID != created.AgentID {
		t.Fatalf("replayed agent_id = %q, want %q", replayed.AgentID, created.AgentID)
	}
	result, err := service.GetAgentCreationRequestResult(ctx, request.CreationRequestID)
	if err != nil || result.Status != protocol.AgentCreationRequestCommitted ||
		result.Agent == nil || result.Agent.AgentID != created.AgentID {
		t.Fatalf("creation result = %#v, err=%v", result, err)
	}

	conflict := request
	conflict.Name = "Different Agent"
	if _, err = service.CreateAgent(ctx, conflict); !errors.Is(err, agentpkg.ErrAgentCreationRequestConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}
	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	deletedResult, err := service.GetAgentCreationRequestResult(ctx, request.CreationRequestID)
	if err != nil || deletedResult.Status != protocol.AgentCreationRequestDeleted {
		t.Fatalf("deleted creation result = %#v, err=%v", deletedResult, err)
	}
	if _, err = service.CreateAgent(ctx, request); !errors.Is(err, agentpkg.ErrAgentCreationResultDeleted) {
		t.Fatalf("late replay error = %v, want deleted tombstone", err)
	}
}

func TestAgentCreationWorkspaceFailureBecomesTerminalWithoutAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	blockedRoot := filepath.Join(t.TempDir(), "workspace-is-a-file")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.WorkspacePath = blockedRoot
	service, db := newAgentTestService(t, cfg)
	request := protocol.CreateRequest{
		Name:              "Blocked Workspace Agent",
		CreationRequestID: "web-create:blocked-workspace",
	}
	if _, err := service.CreateAgent(context.Background(), request); !errors.Is(err, agentpkg.ErrAgentCreationFailed) {
		t.Fatalf("CreateAgent() error = %v, want terminal creation failure", err)
	}
	result, err := service.GetAgentCreationRequestResult(context.Background(), request.CreationRequestID)
	if err != nil || result.Status != protocol.AgentCreationRequestFailed {
		t.Fatalf("creation result = %#v, err=%v", result, err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = ?`, request.Name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed creation wrote %d Agent rows", count)
	}
}
