package automation

import (
	"context"
	"slices"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestCreateTaskCopiesSessionAndAgentPermissionSnapshot(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-source", "")
	now := time.Now().UTC()
	store := workspacestore.NewSessionFileStore(workspacePath).ForOwner(authctx.SystemUserID)
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  protocol.SessionChannelWebSocket,
		ChatType:     protocol.RoomTypeDM,
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Options: protocol.WithSessionRuntimeSettings(nil, protocol.SessionRuntimeSettings{
			PermissionMode: automationdomain.PermissionModeDontAsk,
		}),
		IsActive: true,
	}); err != nil {
		t.Fatalf("准备来源 Session 失败: %v", err)
	}

	authority := &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-1": {
			AgentID:       "agent-1",
			OwnerUserID:   authctx.SystemUserID,
			WorkspacePath: workspacePath,
			Options: protocol.Options{
				PermissionMode:  automationdomain.PermissionModePlan,
				AllowedTools:    []string{"WebSearch"},
				DisallowedTools: []string{"Write"},
			},
		},
	}}
	dm := &fakeDMRunner{permission: permissionctx.NewContext()}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		newAutomationTestDB(t),
		nil,
		dm,
		nil,
		dm.permission,
		&fakeWorkspaceReader{},
		nil,
	)
	service.agents = authority

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限快照",
		AgentID:     "agent-1",
		Instruction: "搜索并保存结果",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind:       automationdomain.SourceKindAgent,
			SessionKey: sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if task.PermissionMode != automationdomain.PermissionModeDontAsk {
		t.Fatalf("应优先复制 Session permission mode: %+v", task)
	}
	if !slices.Equal(task.PermissionPolicy.DeniedTools, []string{"Write"}) {
		t.Fatalf("Agent deny 未写入任务快照: %+v", task.PermissionPolicy)
	}
	policy := taskRuntimeToolPolicy(*task)
	if policy == nil || !slices.Contains(policy.AllowedTools, "WebSearch") ||
		!slices.Equal(policy.DisallowedTools, []string{"Write"}) {
		t.Fatalf("runtime 工具快照不完整: %+v", policy)
	}

	authority.mu.Lock()
	changed := authority.agents["agent-1"]
	changed.Options.PermissionMode = automationdomain.PermissionModeAcceptEdits
	changed.Options.AllowedTools = []string{"Read"}
	changed.Options.DisallowedTools = []string{"Bash"}
	authority.agents["agent-1"] = changed
	authority.mu.Unlock()

	allowed, hardDenied, err := service.taskPolicyAllowsCapability(context.Background(), *task, automationdomain.PermissionCapability{
		ToolName: "WebSearch",
		Effect:   automationdomain.PermissionEffectRead,
	})
	if err != nil || !allowed || hardDenied {
		t.Fatalf("Agent 修改后任务原 allow 快照应继续生效: allowed=%v hardDenied=%v err=%v", allowed, hardDenied, err)
	}
	allowed, hardDenied, err = service.taskPolicyAllowsCapability(context.Background(), *task, automationdomain.PermissionCapability{
		ToolName: "Write",
		Effect:   automationdomain.PermissionEffectWrite,
	})
	if err != nil || allowed || !hardDenied {
		t.Fatalf("Agent 修改后任务原 deny 快照应继续生效: allowed=%v hardDenied=%v err=%v", allowed, hardDenied, err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return len(dm.Requests()) == 1 })
	runtimePolicy := dm.Requests()[0].RuntimeToolPolicy
	if runtimePolicy == nil || !slices.Contains(runtimePolicy.AllowedTools, "WebSearch") ||
		slices.Contains(runtimePolicy.AllowedTools, "Read") ||
		!slices.Equal(runtimePolicy.DisallowedTools, []string{"Write"}) {
		t.Fatalf("执行未使用任务创建快照: %+v", runtimePolicy)
	}
}

func TestCreateTaskExplicitPermissionModeOverridesCopiedSessionMode(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:           "显式权限",
		AgentID:        "agent-1",
		Instruction:    "仅规划",
		PermissionMode: automationdomain.PermissionModePlan,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if task.PermissionMode != automationdomain.PermissionModePlan {
		t.Fatalf("显式 permission mode 未保留: %+v", task)
	}
}
