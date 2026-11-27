// ingress_test.go 覆盖渠道入站消息、命令、租约、权限与重试测试。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

type fakeIngressDMHandler struct {
	requests     []dmsvc.Request
	ownerUserIDs []string
	err          error
}

func (f *fakeIngressDMHandler) HandleChat(ctx context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
	if f.err != nil {
		return f.err
	}
	return nil
}

func newIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18040,
		ProjectName:    "nexus-channel-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func ingressTestOwnerContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
}

func migrateIngressSQLite(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	handlertest.MigrateSQLiteFromDir(t, databaseURL, ingressMigrationDir(t))
	// 与生产保持相同的单连接、事务和外键语义，避免并发审批使用裸连接竞争写锁。
	db, err := storage.OpenDB(config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	return db
}

func ingressMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}

func TestIngressDoesNotRepeatUncertainControlCommand(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	service := NewIngressService(cfg, agents, &fakeIngressDMHandler{}, router)
	service.SetControlService(NewControlService(cfg, db, agents, router))
	commands := &recordingIngressCommandHandler{err: errors.New("命令执行结果未知")}
	service.SetCommandHandler(commands)
	request := IngressRequest{Channel: "internal", Ref: "chat", Content: "/stop", ReqID: "same-command"}
	if _, err := service.Accept(t.Context(), request); err == nil {
		t.Fatal("预期命令失败")
	}
	_, _ = service.Accept(t.Context(), request)
	if len(commands.requests) != 1 {
		t.Fatalf("未知控制命令被重复执行: %d", len(commands.requests))
	}
}

func TestIngressRecoveryScansPastUnknownWithoutPlatformRedelivery(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	control := NewControlService(cfg, db, agents, router)
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(control)
	request, err := service.normalizeRequest(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= ingressRecoveryBatchSize; i++ {
		request.reqID, request.roundID = fmt.Sprintf("message-%03d", i), fmt.Sprintf("round-%03d", i)
		if _, _, err := service.claimIngress(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		if err := control.beginIngressDispatch(t.Context(), request); err != nil {
			t.Fatal(err)
		}
	}
	service.SetRoundIndexReader(func(_ context.Context, session string) (*protocol.SessionRoundIndex, error) {
		if session != request.sessionKey {
			t.Fatalf("wrong session: %s", session)
		}
		return &protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{
			{RoundID: "round-100", HasUserMessage: true}, {RoundID: "round-099", HasUserMessage: false},
		}}, nil
	})
	cursor := ingressMessageRow{}
	first, err := service.recoverIngressBatch(t.Context(), &cursor)
	if err != nil || !first.HasMore {
		t.Fatalf("first batch: %+v %v", first, err)
	}
	second, err := service.recoverIngressBatch(t.Context(), &cursor)
	if err != nil || second.HasMore || cursor.ReqID != "" {
		t.Fatalf("second batch: %+v %v", second, err)
	}
	var accepted int
	if err := db.QueryRow(`SELECT COUNT(*) FROM im_ingress_messages WHERE status='accepted'`).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 || len(handler.requests) != 0 {
		t.Fatalf("accepted=%d reruns=%d", accepted, len(handler.requests))
	}
	// 无证据的旧消息留在待核验状态，不能伪造受理或重跑。
	unknown, err := control.getIngressMessage(t.Context(), request.ownerUserID, request.channelStored, request.accountID, "message-099")
	if err != nil || unknown.Status != "processing" {
		t.Fatalf("unknown: %+v %v", unknown, err)
	}
}

type recordingIngressCommandHandler struct {
	requests     []IngressCommandRequest
	result       IngressCommandResult
	err          error
	pendingCount int
	pendingErr   error
}

func (h *recordingIngressCommandHandler) HandleIngressCommand(
	_ context.Context,
	request IngressCommandRequest,
) (IngressCommandResult, error) {
	h.requests = append(h.requests, request)
	return h.result, h.err
}

func (h *recordingIngressCommandHandler) CountPendingIngressPermissionRequests(
	_ context.Context,
	_ IngressCommandRequest,
) (int, error) {
	return h.pendingCount, h.pendingErr
}

func TestShortPermissionCommandRejectsRuntimeAndAutomationAmbiguity(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	defaultAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("初始化默认 Agent 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	dm := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permission)
	delivery := &recordingDeliveryChannel{channelType: ChannelTypeFeishu}
	router.RegisterForOwner("", delivery)
	if err = router.Start(context.Background()); err != nil {
		t.Fatalf("启动 channel router 失败: %v", err)
	}
	defer router.Stop(context.Background())
	control := NewControlService(cfg, db, agentService, router)
	const target = "runtime-automation-ambiguity"
	if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: target,
		AgentID:     defaultAgent.AgentID,
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatalf("创建 pairing 失败: %v", err)
	}
	commands := &recordingIngressCommandHandler{
		pendingCount: 1,
		result: IngressCommandResult{
			Handled: true,
			Reply:   "不应执行",
		},
	}
	service := NewIngressService(cfg, agentService, dm, router)
	service.SetControlService(control)
	service.SetRuntimePermissionContext(permission)

	initial, err := service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeFeishu,
		ChatType: protocol.RoomTypeDM,
		Ref:      target,
		Content:  "触发普通工具权限",
		ReqID:    "ambiguity-trigger",
		Delivery: &DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: target},
	})
	if err != nil || initial == nil || len(dm.requests) != 1 {
		t.Fatalf("初始 ingress 失败: requests=%+v err=%v", dm.requests, err)
	}
	service.SetCommandHandler(commands)
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	decisions := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := dm.requests[0].PermissionHandler(requestCtx, sdkpermission.Request{ToolName: "Write"})
		decisions <- decision
	}()
	_ = waitForRuntimePermissionNotice(t, delivery)

	if _, err = service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeFeishu,
		ChatType: protocol.RoomTypeDM,
		Ref:      target,
		Content:  "/y",
		ReqID:    "ambiguity-decision",
		Delivery: &DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: target},
	}); err != nil {
		t.Fatalf("歧义 /y 处理失败: %v", err)
	}
	if got := permission.CountSessionPermissionRequests(initial.SessionKey, ""); got != 1 {
		t.Fatalf("歧义 /y 不应消费 runtime 请求: pending=%d", got)
	}
	if len(commands.requests) != 0 {
		t.Fatalf("歧义 /y 不应消费 Automation 请求: %+v", commands.requests)
	}
	select {
	case decision := <-decisions:
		t.Fatalf("歧义 /y 不应释放 runtime: %+v", decision)
	case <-time.After(100 * time.Millisecond):
	}
	delivery.mu.Lock()
	texts := append([]string(nil), delivery.texts...)
	delivery.mu.Unlock()
	if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, "多个待确认请求") }) {
		t.Fatalf("歧义 /y 未回投安全提示: %+v", texts)
	}
}

func waitForRuntimePermissionNotice(t *testing.T, channel *recordingDeliveryChannel) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		channel.mu.Lock()
		texts := append([]string(nil), channel.texts...)
		channel.mu.Unlock()
		for _, text := range texts {
			if strings.Contains(text, "【Nexus 权限确认】") && strings.Contains(text, "/y：允许本次") {
				return text
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待 IM runtime 权限通知超时")
	return ""
}

func TestIngressServiceFeishuAllowsManagedSemanticSkillsWithRestrictiveAgentTools(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	if _, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{AllowedTools: []string{"Skill"}},
	}); err != nil {
		t.Fatalf("收紧默认 agent 工具权限失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	if _, err := service.Accept(context.Background(), IngressRequest{
		Channel:  "feishu",
		ChatType: "group",
		Ref:      "oc_group_123",
		Content:  "检查今天的定时任务发送情况",
	}); err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}
	if len(handler.requests) != 1 || handler.requests[0].PermissionHandler == nil {
		t.Fatalf("未下发带权限处理器的请求: %+v", handler.requests)
	}

	for _, skillName := range []string{"goal-manager", "execution-orchestrator"} {
		decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
			ToolName: "Skill",
			Input:    map[string]any{"name": skillName},
		})
		if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("限制 allowlist 时仍应允许加载托管 Skill %q: decision=%+v err=%v", skillName, decision, err)
		}
	}

	readDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Read 权限处理失败: %v", err)
	}
	if readDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("限制 allowlist 时不应顺带放开普通只读工具: %+v", readDecision)
	}
}

func TestIngressRetryMarkerStopsBeforeRuntimeDispatch(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{err: errors.New("runtime admission uncertain")}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(NewControlService(cfg, db, agents, router))
	var retryable *channelcontract.RetryableIngressError
	if _, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat"}); !errors.As(err, &retryable) {
		t.Fatalf("准备失败应可安全重试: %v", err)
	}
	_, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello", ReqID: "message", RoundID: "round"})
	if err == nil || errors.As(err, &retryable) || len(handler.requests) != 1 {
		t.Fatalf("运行时错误不能标为安全重试: %v calls=%d", err, len(handler.requests))
	}
}
