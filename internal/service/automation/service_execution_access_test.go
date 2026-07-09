package automation

import (
	"context"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	_ "modernc.org/sqlite"
)

func TestScheduledTaskPermissionHandlerApprovesAgentAllowedTools(t *testing.T) {
	handler := scheduledTaskPermissionHandlerForOptions(protocol.Options{
		AllowedTools:    []string{"WebSearch", "nexus_automation"},
		DisallowedTools: []string{"Write"},
	}, false)

	searchDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "WebSearch"})
	if err != nil {
		t.Fatalf("WebSearch 权限处理失败: %v", err)
	}
	if searchDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Agent 已授权的 WebSearch 应允许后台执行: %+v", searchDecision)
	}
	wrappedSearchDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "mcp__brave_search__brave_web_search"})
	if err != nil {
		t.Fatalf("包装后的 WebSearch 权限处理失败: %v", err)
	}
	if wrappedSearchDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("WebSearch 授权应匹配常见搜索 MCP 工具名: %+v", wrappedSearchDecision)
	}

	wrappedDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "mcp__nexus_automation__get_scheduled_task_daily_report"})
	if err != nil {
		t.Fatalf("包装后的 nexus_automation 工具权限处理失败: %v", err)
	}
	if wrappedDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("nexus_automation 授权应匹配包装工具名: %+v", wrappedDecision)
	}

	writeDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("Write 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("Agent 禁用的 Write 不应被后台授权: %+v", writeDecision)
	}

	questionDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "AskUserQuestion"})
	if err != nil {
		t.Fatalf("AskUserQuestion 权限处理失败: %v", err)
	}
	if questionDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("后台定时任务不应允许 AskUserQuestion: %+v", questionDecision)
	}
}

func TestScheduledTaskPermissionHandlerUsesImagegenDefaultTools(t *testing.T) {
	enabledHandler := scheduledTaskPermissionHandlerForOptions(protocol.Options{
		AllowedTools: []string{"Read"},
	}, true)
	enabledDecision, err := enabledHandler(context.Background(), sdkpermission.Request{ToolName: "mcp__nexus_imagegen__generate_image"})
	if err != nil {
		t.Fatalf("图片生成权限处理失败: %v", err)
	}
	if enabledDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("配置生图 provider 后定时任务应默认允许图片生成工具: %+v", enabledDecision)
	}

	disabledHandler := scheduledTaskPermissionHandlerForOptions(protocol.Options{
		AllowedTools: []string{"Read", "nexus_imagegen"},
	}, false)
	disabledDecision, err := disabledHandler(context.Background(), sdkpermission.Request{ToolName: "mcp__nexus_imagegen__generate_image"})
	if err != nil {
		t.Fatalf("图片生成权限处理失败: %v", err)
	}
	if disabledDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("未配置生图 provider 时定时任务不应允许图片生成工具: %+v", disabledDecision)
	}
}

func TestServiceListTasksScopesByOwnerUserID(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	ctxUser1 := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-1", Username: "user-1"})
	ctxUser2 := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-2", Username: "user-2"})

	taskUser1, err := service.CreateTask(ctxUser1, automationdomain.CreateJobInput{
		Name:        "用户 1 任务",
		AgentID:     "agent-1",
		Instruction: "user 1",
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
		t.Fatalf("创建 user-1 任务失败: %v", err)
	}
	if _, err = service.CreateTask(ctxUser2, automationdomain.CreateJobInput{
		Name:        "用户 2 任务",
		AgentID:     "agent-1",
		Instruction: "user 2",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	}); err != nil {
		t.Fatalf("创建 user-2 任务失败: %v", err)
	}

	user1Tasks, err := service.ListTasks(ctxUser1, "")
	if err != nil {
		t.Fatalf("ListTasks user-1 失败: %v", err)
	}
	if len(user1Tasks) != 1 || user1Tasks[0].JobID != taskUser1.JobID {
		t.Fatalf("user-1 scope 不正确: %+v", user1Tasks)
	}
	user2View, err := service.GetTask(ctxUser2, taskUser1.JobID)
	if err != nil {
		t.Fatalf("GetTask user-2 失败: %v", err)
	}
	if user2View != nil {
		t.Fatalf("user-2 不应读取 user-1 任务: %+v", user2View)
	}
	globalTasks, err := service.ListTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTasks global 失败: %v", err)
	}
	if len(globalTasks) != 2 {
		t.Fatalf("global scope 应看到 2 个任务，实际 %d", len(globalTasks))
	}
}
