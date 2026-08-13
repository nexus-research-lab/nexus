package session_test

import (
	"context"
	"errors"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type fakeExternalSessionIdentityResolver struct {
	current bool
}

func TestDeletingUnpairedExternalSessionPausesBoundTaskForRebind(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	core, db := newSessionTestCoreServices(t, cfg)
	core.Session.SetRuntimeManager(runtimectx.NewManager())
	automationService := automationsvc.NewService(
		cfg,
		db,
		core.Agent,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	core.Deletion.SetTaskCleaner(automationService)
	core.Session.SetTaskReferenceResolver(automationService)
	core.Session.SetExternalSessionIdentityResolver(&fakeExternalSessionIdentityResolver{current: false})

	ctx := context.Background()
	agentValue, err := core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "IM Task 助手"})
	if err != nil {
		t.Fatalf("创建 Agent: %v", err)
	}
	sessionKey := protocol.BuildAgentAccountSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"account-old",
		"contact-old",
		"",
	)
	if _, err = core.Session.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: sessionKey,
		Title:      "旧微信联系人",
	}); err != nil {
		t.Fatalf("创建 IM Session: %v", err)
	}
	interval := 300
	task, err := automationService.CreateTask(ctx, automationdomain.CreateJobInput{
		Name:        "绑定旧微信会话的任务",
		AgentID:     agentValue.AgentID,
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &interval, Timezone: "UTC"},
		Instruction: "发送提醒",
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
			WakeMode:        automationdomain.WakeModeNow,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:   automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("创建绑定任务: %v", err)
	}

	if err = core.Session.DeleteSession(ctx, sessionKey); err != nil {
		t.Fatalf("删除已解绑 IM Session: %v", err)
	}
	kept, err := automationService.GetTask(ctx, task.JobID)
	if err != nil || kept == nil {
		t.Fatalf("删除 Session 后任务应保留: task=%+v err=%v", kept, err)
	}
	if kept.Enabled || kept.SessionBindingState != automationdomain.TaskSessionBindingStateRebindRequired ||
		len(kept.SessionBindingIssues) != 1 || kept.SessionBindingIssues[0] != automationdomain.TaskSessionBindingIssueExecution {
		t.Fatalf("任务未停用并标记执行会话待重绑: %+v", kept)
	}
}

func (f *fakeExternalSessionIdentityResolver) ResolveExternalSessionIdentity(
	context.Context,
	string,
	string,
) (*protocol.ExternalSessionIdentity, error) {
	return &protocol.ExternalSessionIdentity{
		ChannelType:    protocol.SessionChannelWeixinPersonal,
		AccountHint:    "A1B2C3",
		AccountStatus:  "connected",
		PairingStatus:  map[bool]string{true: "active", false: "disabled"}[f.current],
		CurrentPairing: f.current,
		CanDelete:      !f.current,
	}, nil
}

type fakeSessionTaskReferenceResolver struct {
	count int
}

func (f *fakeSessionTaskReferenceResolver) CountTasksReferencingSessions(
	_ context.Context,
	_ string,
	sessionKeys []string,
) (map[string]int, error) {
	result := make(map[string]int, len(sessionKeys))
	for _, sessionKey := range sessionKeys {
		result[sessionKey] = f.count
	}
	return result, nil
}

func TestExternalSessionIdentityAndDeletionUsePairingAndTaskFacts(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())

	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "IM Session 助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentAccountSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"account-current",
		"contact-a",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: sessionKey,
		Title:      "微信联系人甲",
	}); err != nil {
		t.Fatalf("创建 IM Session 失败: %v", err)
	}

	identityResolver := &fakeExternalSessionIdentityResolver{current: true}
	taskResolver := &fakeSessionTaskReferenceResolver{}
	sessionService.SetExternalSessionIdentityResolver(identityResolver)
	sessionService.SetTaskReferenceResolver(taskResolver)

	items, err := sessionService.ListAgentSessions(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出带身份 IM Session 失败: %v", err)
	}
	if len(items) != 1 || items[0].ExternalIdentity == nil ||
		items[0].ExternalIdentity.AccountHint != "A1B2C3" ||
		!items[0].ExternalIdentity.CurrentPairing ||
		items[0].ExternalIdentity.CanDelete {
		t.Fatalf("当前 IM Session 身份投影不正确: %+v", items)
	}
	if err = sessionService.DeleteSession(ctx, sessionKey); !errors.Is(
		err,
		sessionsvc.ErrExternalSessionPairingActive,
	) {
		t.Fatalf("当前配对 IM Session 应拒绝删除: %v", err)
	}

	identityResolver.current = false
	taskResolver.count = 1
	items, err = sessionService.ListAgentSessions(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出任务引用 IM Session 失败: %v", err)
	}
	if len(items) != 1 || items[0].ExternalIdentity == nil ||
		items[0].ExternalIdentity.CurrentPairing ||
		!items[0].ExternalIdentity.CanDelete ||
		items[0].ExternalIdentity.TaskReferenceCount != 1 {
		t.Fatalf("任务引用 IM Session 身份投影不正确: %+v", items)
	}
	if err = sessionService.DeleteSession(ctx, sessionKey); err != nil {
		t.Fatalf("解绑后的 IM Session 应允许删除并由任务域暂停引用任务: %v", err)
	}
	if _, err = sessionService.GetMutableSession(ctx, sessionKey); !errors.Is(
		err,
		sessionsvc.ErrSessionNotFound,
	) {
		t.Fatalf("删除后 IM Session 仍可读取: %v", err)
	}
}
