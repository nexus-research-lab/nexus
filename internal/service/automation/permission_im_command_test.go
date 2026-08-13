package automation

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type mutableAutomationDeliveryGrant struct {
	mu       sync.Mutex
	allowed  bool
	agentIDs []string
	calls    []string
}

func (g *mutableAutomationDeliveryGrant) ValidateAutomationDeliveryGrant(
	_ context.Context,
	_ string,
	agentID string,
	sessionKey string,
) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agentIDs = append(g.agentIDs, strings.TrimSpace(agentID))
	g.calls = append(g.calls, sessionKey)
	if !g.allowed {
		return fmt.Errorf("%w: pairing revoked", channels.ErrExternalSessionGrantUnavailable)
	}
	return nil
}

func (g *mutableAutomationDeliveryGrant) agentIDsSnapshot() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.agentIDs)
}

func (g *mutableAutomationDeliveryGrant) setAllowed(allowed bool) {
	g.mu.Lock()
	g.allowed = allowed
	g.mu.Unlock()
}

type permissionIMFixture struct {
	service    *Service
	dm         *fakeDMRunner
	delivery   *fakeDeliveryRouter
	grant      *mutableAutomationDeliveryGrant
	ownerCtx   context.Context
	task       automationdomain.ScheduledTask
	request    automationdomain.AutomationPermissionRequest
	sessionKey string
}

func newPermissionIMFixture(t *testing.T) permissionIMFixture {
	t.Helper()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:   permission,
		requiredTool: "WebSearch",
		resultText:   "审批后的执行结果",
	}
	delivery := &fakeDeliveryRouter{}
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	service.SetDeliveryGrantResolver(grant)
	ownerCtx := contextForOwner(context.Background(), "user-1")
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"weixin-account",
		"weixin-user",
		"",
	)
	task, err := service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
		Name:        "微信审批任务",
		AgentID:     "agent-1",
		Instruction: "搜索信息并回传",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-im", ""),
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeLast,
			SessionKey: sessionKey,
		},
		Source: automationdomain.Source{
			Kind:       automationdomain.SourceKindCLI,
			SessionKey: sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.RunTaskNow(ownerCtx, task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	var pending []automationdomain.AutomationPermissionRequest
	waitFor(t, 2*time.Second, func() bool {
		var listErr error
		pending, listErr = service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(pending) == 1
	})
	return permissionIMFixture{
		service:    service,
		dm:         dm,
		delivery:   delivery,
		grant:      grant,
		ownerCtx:   ownerCtx,
		task:       *task,
		request:    pending[0],
		sessionKey: sessionKey,
	}
}

func TestPermissionIMSlashApprovesAndDoesNotEnterAgentRuntime(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	if fixture.request.DeliverySessionKey != fixture.sessionKey {
		t.Fatalf("审批请求没有保留原 IM 会话: %+v", fixture.request)
	}
	messages := fixture.delivery.Messages()
	if len(messages) != 1 ||
		!strings.Contains(messages[0], "【Nexus 定时任务 · 微信审批任务】") ||
		!strings.Contains(messages[0], "/y：允许本次") ||
		!strings.Contains(messages[0], "/a：此任务始终允许") ||
		!strings.Contains(messages[0], "/d：拒绝") ||
		strings.Contains(messages[0], "/approve") ||
		strings.Contains(messages[0], fixture.request.RequestID) {
		t.Fatalf("IM 权限通知不完整: %+v", messages)
	}
	requestsBefore := len(fixture.dm.Requests())
	result, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/y",
	})
	if err != nil || !result.Handled || !strings.Contains(result.Reply, "已批准") {
		t.Fatalf("IM /y 失败: result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := fixture.service.ListTaskRuns(fixture.ownerCtx, fixture.task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	if got := len(fixture.dm.Requests()); got != requestsBefore+1 {
		t.Fatalf("命令应只触发权限续跑，不应作为聊天再进入 runtime: before=%d after=%d", requestsBefore, got)
	}
	messages = fixture.delivery.Messages()
	if len(messages) < 2 || messages[len(messages)-1] != "审批后的执行结果" {
		t.Fatalf("续跑结果应原样回投且不能带固定任务前缀: %+v", messages)
	}

	requestsAfter := len(fixture.dm.Requests())
	duplicate, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/approve " + fixture.request.RequestID,
	})
	if err != nil || !duplicate.Handled || !strings.Contains(duplicate.Reply, "已处理") {
		t.Fatalf("重复 IM 审批未幂等返回: result=%+v err=%v", duplicate, err)
	}
	if got := len(fixture.dm.Requests()); got != requestsAfter {
		t.Fatalf("重复审批不应再次续跑: before=%d after=%d", requestsAfter, got)
	}
}

func TestPermissionIMBareConfirmationRequiresCurrentPendingSession(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	notCurrent, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelWeixinPersonal, "dm", "another-user", ""),
		Content:     "是",
	})
	if err != nil || notCurrent.Handled {
		t.Fatalf("非当前审批会话的普通“是”不应被控制面消费: result=%+v err=%v", notCurrent, err)
	}
	current, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "是",
	})
	if err != nil || !current.Handled || !strings.Contains(current.Reply, "已批准") {
		t.Fatalf("当前会话唯一待确认请求应接受明确“是”: result=%+v err=%v", current, err)
	}
}

func TestPermissionIMShortAlwaysPersistsTaskGrant(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	result, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/a",
	})
	if err != nil || !result.Handled || !strings.Contains(result.Reply, "始终允许") {
		t.Fatalf("IM /a 失败: result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		current, loadErr := fixture.service.GetTask(fixture.ownerCtx, fixture.task.JobID)
		return loadErr == nil && current != nil && slices.ContainsFunc(
			current.PermissionPolicy.Grants,
			func(grant automationdomain.TaskPermissionGrant) bool {
				return grant.Source == automationdomain.PermissionGrantSourceUserApproval &&
					grant.Capability.ToolName == fixture.request.Capability.ToolName
			},
		)
	})
}

func TestPermissionIMShortDenyStopsRun(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	requestsBefore := len(fixture.dm.Requests())
	result, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/d",
	})
	if err != nil || !result.Handled || !strings.Contains(result.Reply, "已拒绝") {
		t.Fatalf("IM /d 失败: result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		current, loadErr := fixture.service.GetTask(fixture.ownerCtx, fixture.task.JobID)
		return loadErr == nil && current != nil &&
			current.PermissionState == automationdomain.TaskPermissionStateDenied
	})
	if got := len(fixture.dm.Requests()); got != requestsBefore {
		t.Fatalf("/d 不应触发权限续跑: before=%d after=%d", requestsBefore, got)
	}
}

func TestPermissionIMLegacyLongCommandsRemainCompatibilityAliases(t *testing.T) {
	tests := map[string]protocol.IMPermissionCommand{
		"/approve": protocol.IMPermissionCommandAllowOnce,
		"/always":  protocol.IMPermissionCommandAllowAlways,
		"/deny":    protocol.IMPermissionCommandDeny,
	}
	for input, want := range tests {
		command, ok := parsePermissionIMCommand(input)
		if !ok || command.name != want || command.bare || command.malformed {
			t.Fatalf("legacy %q = %+v, %t; want %q", input, command, ok, want)
		}
	}
}

func TestPermissionIMSlashWithoutIDRejectsMultiplePendingRequests(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	secondTask, err := fixture.service.CreateTask(fixture.ownerCtx, automationdomain.CreateJobInput{
		Name:        "第二个微信审批任务",
		AgentID:     fixture.task.AgentID,
		Instruction: "再次搜索信息并回传",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "second-permission-im", ""),
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeLast,
			SessionKey: fixture.sessionKey,
		},
		Source: automationdomain.Source{
			Kind:       automationdomain.SourceKindCLI,
			SessionKey: fixture.sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("创建第二个 IM 审批任务失败: %v", err)
	}
	if _, err = fixture.service.RunTaskNow(fixture.ownerCtx, secondTask.JobID); err != nil {
		t.Fatalf("运行第二个 IM 审批任务失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		pending, listErr := fixture.service.ListPermissionRequests(
			fixture.ownerCtx,
			automationdomain.PermissionRequestStatusPending,
			"",
		)
		return listErr == nil && len(pending) == 2
	})

	result, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/y",
	})
	if err != nil || !result.Handled || !strings.Contains(result.Reply, "多个待确认请求") {
		t.Fatalf("无 ID 命令必须拒绝猜测多个请求: result=%+v err=%v", result, err)
	}
	pending, err := fixture.service.ListPermissionRequests(
		fixture.ownerCtx,
		automationdomain.PermissionRequestStatusPending,
		"",
	)
	if err != nil || len(pending) != 2 {
		t.Fatalf("歧义命令不应消费任何审批请求: pending=%+v err=%v", pending, err)
	}
}

func TestPermissionIMApprovalFailsClosedAfterPairingRevocation(t *testing.T) {
	fixture := newPermissionIMFixture(t)
	fixture.grant.setAllowed(false)
	result, err := fixture.service.HandleIngressCommand(fixture.ownerCtx, channels.IngressCommandRequest{
		OwnerUserID: "user-1",
		AgentID:     fixture.task.AgentID,
		SessionKey:  fixture.sessionKey,
		Content:     "/a",
	})
	if err != nil || !result.Handled || !strings.Contains(result.Reply, "配对授权已失效") {
		t.Fatalf("撤销 pairing 后审批必须 fail closed: result=%+v err=%v", result, err)
	}
	pending, listErr := fixture.service.ListPermissionRequests(
		fixture.ownerCtx,
		automationdomain.PermissionRequestStatusPending,
		fixture.task.JobID,
	)
	if listErr != nil || len(pending) != 1 || pending[0].RequestID != fixture.request.RequestID {
		t.Fatalf("撤销 pairing 后不应消费审批请求: requests=%+v err=%v", pending, listErr)
	}
}

func TestAutomationIMControlNotificationsCoverEveryExternalChannel(t *testing.T) {
	for _, channelType := range []string{
		protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu,
	} {
		if !externalIMChannel(channelType) {
			t.Fatalf("外部 IM 通道 %s 未进入 Automation 控制通知链路", channelType)
		}
		sessionKey := protocol.BuildAgentSessionKey("agent-1", channelType, protocol.RoomTypeDM, "target", "")
		if !externalIMSessionKey(sessionKey) {
			t.Fatalf("外部 IM session %s 未被识别", sessionKey)
		}
	}
}

func TestPermissionIMCommandsRequireExactPairedExternalDMIdentity(t *testing.T) {
	for _, channelType := range []string{
		protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu,
	} {
		sessionKey := protocol.BuildAgentAccountSessionKey(
			"agent-1",
			channelType,
			protocol.RoomTypeDM,
			"account-1",
			"target-1",
			"",
		)
		if !trustedPermissionIMCommandSession(sessionKey, "agent-1") {
			t.Fatalf("%s active-paired DM 应允许进入权限命令复核", channelType)
		}
		if trustedPermissionIMCommandSession(sessionKey, "agent-2") {
			t.Fatalf("%s DM 不得被另一 Agent 使用", channelType)
		}
		groupKey := protocol.BuildAgentAccountSessionKey(
			"agent-1",
			channelType,
			protocol.RoomTypeGroup,
			"account-1",
			"target-1",
			"",
		)
		if trustedPermissionIMCommandSession(groupKey, "agent-1") {
			t.Fatalf("%s 群聊不得执行持久权限命令", channelType)
		}
	}
	if trustedPermissionIMCommandSession(
		protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelWebSocket, protocol.RoomTypeDM, "internal", ""),
		"agent-1",
	) {
		t.Fatal("WebSocket DM 不得冒充外部 IM 权限命令入口")
	}
}
