// permission_test.go 覆盖自动化任务权限管道、IM 命令审批、可靠性、会话与快照测试。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
)

type mutableAutomationDeliveryGrant struct {
	mu       sync.Mutex
	allowed  bool
	agentIDs []string
	calls    []string
}

func (g *mutableAutomationDeliveryGrant) ListAgentExternalSessions(
	context.Context,
	string,
	string,
	string,
) ([]channels.AgentExternalSession, error) {
	return nil, nil
}

func (g *mutableAutomationDeliveryGrant) ValidateExternalSessionGrant(
	ctx context.Context,
	_ string,
	agentID string,
	sessionKey string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agentIDs = append(g.agentIDs, strings.TrimSpace(agentID))
	g.calls = append(g.calls, sessionKey)
	if !g.allowed {
		return fmt.Errorf("%w: pairing revoked", channels.ErrExternalSessionGrantUnavailable)
	}
	return nil
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
	var messages []string
	// 权限请求可见与 IM 通知送达是两个异步阶段（#298）：先等通知送达，再做数量与内容断言。
	waitFor(t, 2*time.Second, func() bool {
		messages = fixture.delivery.Messages()
		return len(messages) >= 1
	})
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
		// 执行完成和投递是两个独立持久阶段；只在投递已确认后断言回传正文。
		return listErr == nil && len(runs) == 1 &&
			runs[0].Status == automationdomain.RunStatusSucceeded &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})
	if got := len(fixture.dm.Requests()); got != requestsBefore+1 {
		t.Fatalf("命令应只触发权限续跑，不应作为聊天再进入 runtime: before=%d after=%d", requestsBefore, got)
	}
	// 执行终态先于异步结果投递，等待投递完成后再核对正文。
	waitFor(t, 2*time.Second, func() bool {
		messages = fixture.delivery.Messages()
		return len(messages) >= 2
	})
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

type mutableConnectorResolver struct {
	mu        sync.Mutex
	connected bool
	owners    []string
	ids       []string
}

func (r *mutableConnectorResolver) LoadActiveConnection(
	_ context.Context,
	ownerUserID string,
	connectorID string,
) (*connectordomain.ConnectionSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.owners = append(r.owners, ownerUserID)
	r.ids = append(r.ids, connectorID)
	if !r.connected {
		return nil, errors.New("connector is disconnected")
	}
	return &connectordomain.ConnectionSnapshot{ConnectorID: connectorID}, nil
}

func (r *mutableConnectorResolver) setConnected(connected bool) {
	r.mu.Lock()
	r.connected = connected
	r.mu.Unlock()
}

type permissionDrainDMRunner struct {
	noMainWindow
	permission       *permissionctx.Context
	requiredTool     string
	interruptStarted chan struct{}
	releaseInterrupt chan struct{}

	mu                sync.Mutex
	requests          []dmsvc.Request
	interrupts        []dmsvc.InterruptRequest
	firstRequest      *dmsvc.Request
	firstStopped      bool
	resumedBeforeStop bool
	interruptOnce     sync.Once
	terminalOnce      sync.Once
}

func (r *permissionDrainDMRunner) HandleChat(_ context.Context, request dmsvc.Request) error {
	r.mu.Lock()
	if len(r.requests) > 0 && !r.firstStopped {
		r.resumedBeforeStop = true
		// 模拟真实 DM durable queue：函数 callback 无法持久化，重建时会丢失。
		request.PermissionHandler = nil
	}
	r.requests = append(r.requests, request)
	if r.firstRequest == nil {
		first := request
		r.firstRequest = &first
	}
	r.mu.Unlock()

	go func() {
		if request.PermissionHandler == nil {
			r.emitTerminal(request, "missing scheduled permission handler", "error")
			return
		}
		decision, err := request.PermissionHandler(context.Background(), sdkpermission.Request{
			ToolName: r.requiredTool,
			Input: map[string]any{
				"url": "https://example.feishu.cn/wiki/permission-drain",
			},
		})
		if err != nil {
			r.emitTerminal(request, err.Error(), "error")
			return
		}
		if decision.Behavior == sdkpermission.BehaviorAllow {
			r.emitTerminal(request, "ok", "success")
		}
		// 首次拒绝由精确 interrupt 负责结束物理 round，模拟真实 runtime。
	}()
	return nil
}

func (r *permissionDrainDMRunner) HandleInterrupt(ctx context.Context, request dmsvc.InterruptRequest) error {
	r.interruptOnce.Do(func() { close(r.interruptStarted) })
	select {
	case <-r.releaseInterrupt:
	case <-ctx.Done():
		return ctx.Err()
	}
	r.mu.Lock()
	r.interrupts = append(r.interrupts, request)
	r.firstStopped = true
	first := r.firstRequest
	r.mu.Unlock()
	if first != nil {
		r.terminalOnce.Do(func() {
			r.emitTerminal(*first, "permission blocked", "cancelled")
		})
	}
	return nil
}

func (r *permissionDrainDMRunner) emitTerminal(request dmsvc.Request, result string, subtype string) {
	r.permission.BroadcastEvent(context.Background(), request.SessionKey, protocol.EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    "durable",
		EventType:       protocol.EventTypeMessage,
		SessionKey:      request.SessionKey,
		Data: map[string]any{
			"message_id": "result_" + request.RoundID,
			"round_id":   request.RoundID,
			"role":       "result",
			"subtype":    subtype,
			"result":     result,
			"session_id": "sdk_" + request.RoundID,
		},
		Timestamp: time.Now().UnixMilli(),
	})
	r.permission.BroadcastEvent(
		context.Background(),
		request.SessionKey,
		protocol.NewRoundStatusEvent(request.SessionKey, request.RoundID, "finished", subtype),
	)
}

func (r *permissionDrainDMRunner) snapshot() ([]dmsvc.Request, []dmsvc.InterruptRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := append([]dmsvc.Request(nil), r.requests...)
	interrupts := append([]dmsvc.InterruptRequest(nil), r.interrupts...)
	return requests, interrupts, r.resumedBeforeStop
}

func TestPermissionApprovalDrainsPhysicalAttemptBeforeResume(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	runner := &permissionDrainDMRunner{
		permission:       permission,
		requiredTool:     "mcp__nexus_feishu_docx__read",
		interruptStarted: make(chan struct{}),
		releaseInterrupt: make(chan struct{}),
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		runner,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "审批续跑收尾屏障",
		AgentID:     "agent-1",
		Instruction: "读取飞书文档",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-drain", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil {
		t.Fatalf("RunTaskNow 失败: result=%+v err=%v", result, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1
	})
	select {
	case <-runner.interruptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("权限阻塞后没有精确停止旧物理 attempt")
	}
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取审批请求失败: requests=%+v err=%v", requests, err)
	}
	type decisionResult struct {
		value *automationdomain.PermissionDecisionResult
		err   error
	}
	decisionCh := make(chan decisionResult, 1)
	go func() {
		value, resolveErr := service.ResolvePermissionRequest(
			ownerCtx,
			requests[0].RequestID,
			permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowTask),
		)
		decisionCh <- decisionResult{value: value, err: resolveErr}
	}()
	select {
	case early := <-decisionCh:
		t.Fatalf("旧 attempt 未收尾时审批不应启动续跑: result=%+v err=%v", early.value, early.err)
	case <-time.After(50 * time.Millisecond):
	}
	if currentRequests, _, _ := runner.snapshot(); len(currentRequests) != 1 {
		t.Fatalf("旧 attempt 未收尾时不应派发下一轮: %+v", currentRequests)
	}
	close(runner.releaseInterrupt)
	resolved := <-decisionCh
	if resolved.err != nil || resolved.value == nil || !resolved.value.ResumeStarted {
		t.Fatalf("收尾完成后自动续跑失败: result=%+v err=%v", resolved.value, resolved.err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].Attempts != 2 {
		t.Fatalf("续跑没有保持同一 logical run: runs=%+v err=%v", runs, err)
	}
	dispatched, interrupts, resumedBeforeStop := runner.snapshot()
	if resumedBeforeStop || len(dispatched) != 2 || dispatched[1].PermissionHandler == nil {
		t.Fatalf("续跑在旧 attempt 前启动或丢失权限上下文: requests=%+v resumed_before_stop=%v", dispatched, resumedBeforeStop)
	}
	if dispatched[1].AutomationRun == nil ||
		dispatched[1].AutomationRun.ResumeToolName != "mcp__nexus_feishu_docx__read" {
		t.Fatalf("续跑未通过可信上下文要求重试触发审批的工具: %+v", dispatched[1].AutomationRun)
	}
	if len(interrupts) != 1 || interrupts[0].RoundID != dispatched[0].RoundID {
		t.Fatalf("没有精确停止触发审批的旧 round: interrupts=%+v requests=%+v", interrupts, dispatched)
	}
}

func TestPermissionBlockRejectsLaterGrantedTools(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission, requiredTool: "WebSearch"}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限阻塞后的工具闸门",
		AgentID:     "agent-1",
		Instruction: "先搜索，再读取本地文件",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "blocked-followup-tool", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	policy := appendTaskPermissionGrant(task.PermissionPolicy, automationdomain.TaskPermissionGrant{
		GrantID: "grant-read",
		Capability: automationdomain.PermissionCapability{
			ToolName: "Read",
			Effect:   automationdomain.PermissionEffectRead,
		},
		Source: automationdomain.PermissionGrantSourceUserApproval,
	})
	updated, err := service.repository.UpdateTaskPermissionPolicyIfRevision(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		task.PermissionPolicy.Revision,
		policy,
		automationdomain.TaskPermissionStateReady,
	)
	if err != nil || !updated {
		t.Fatalf("预置 Read 授权失败: updated=%v err=%v", updated, err)
	}
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1
	})
	dispatched := dm.Requests()
	if len(dispatched) != 1 || dispatched[0].PermissionHandler == nil {
		t.Fatalf("任务没有携带权限处理器: %+v", dispatched)
	}
	decision, err := dispatched[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "/tmp/should-not-run"},
	})
	if err != nil {
		t.Fatalf("阻塞后的工具检查失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny || decision.ErrorCode != automationPermissionRequiredCode {
		t.Fatalf("权限阻塞后不应继续放行已经授权的其他工具: %+v", decision)
	}
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 || requests[0].Capability.ToolName != "WebSearch" {
		t.Fatalf("后续工具不得替换最先阻塞任务的权限请求: requests=%+v err=%v", requests, err)
	}
}

func TestPermissionPipelineSeparatesFeishuGrantFromOAuthReadiness(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:   permission,
		requiredTool: "mcp__nexus_feishu_docx__read",
	}
	connectors := &mutableConnectorResolver{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	service.SetConnectorResolver(connectors)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "读取飞书文档",
		AgentID:     "agent-1",
		Instruction: "读取飞书文档并总结",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "feishu-oauth", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil {
		t.Fatalf("RunTaskNow 失败: result=%+v err=%v", result, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1 && requests[0].Kind == automationdomain.PermissionRequestKindTool
	})
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取工具审批请求失败: requests=%+v err=%v", requests, err)
	}
	otherOwnerCtx := contextForOwner(context.Background(), "another-owner")
	otherRequests, err := service.ListPermissionRequests(otherOwnerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(otherRequests) != 0 {
		t.Fatalf("其他 owner 不应看到审批请求: requests=%+v err=%v", otherRequests, err)
	}
	if _, err = service.ResolvePermissionRequest(
		otherOwnerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowOnce),
	); !errors.Is(err, automationdomain.ErrPermissionRequestNotFound) {
		t.Fatalf("其他 owner 不应能审批请求: %v", err)
	}
	if _, err = service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowOnce),
	); err != nil {
		t.Fatalf("allow_once 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(items) == 1 && items[0].Kind == automationdomain.PermissionRequestKindConnectorReauth
	})
	requests, err = service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 || requests[0].Capability.ConnectorID != "feishu-docx" {
		t.Fatalf("工具授权与 connector readiness 未分层: requests=%+v err=%v", requests, err)
	}
	if _, err = service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionRetry),
	); !errors.Is(err, automationdomain.ErrPermissionConnectorNotReady) {
		t.Fatalf("连接未恢复时不应消耗 reauth 请求: %v", err)
	}
	connectors.setConnected(true)
	resolved, err := service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionRetry),
	)
	if err != nil || !resolved.ResumeStarted {
		t.Fatalf("连接恢复后继续 run 失败: result=%+v err=%v", resolved, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].RunID != *result.RunID || runs[0].Attempts != 3 {
		t.Fatalf("Feishu 重连应继续同一 logical run: runs=%+v err=%v", runs, err)
	}
}

func TestLegacyScriptTaskPermissionBackfillKeepsExactScriptGrant(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "旧版脚本任务",
		AgentID:       "agent-legacy-script",
		Instruction:   "printf legacy-script",
		ExecutionKind: automationdomain.ExecutionKindScript,
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
		t.Fatalf("创建旧脚本兼容测试任务失败: %v", err)
	}
	resetTaskPermissionPolicyToLegacy(t, db, task.JobID)
	if err = service.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("旧脚本权限策略回填失败: %v", err)
	}
	persisted, err := service.repository.GetScheduledTask(
		context.Background(), task.OwnerUserID, task.JobID,
	)
	if err != nil || persisted == nil {
		t.Fatalf("读取回填后的旧脚本失败: task=%+v err=%v", persisted, err)
	}
	if len(persisted.PermissionPolicy.Grants) != 1 {
		t.Fatalf("旧脚本应只有一个精确兼容授权: %+v", persisted.PermissionPolicy.Grants)
	}
	grant := persisted.PermissionPolicy.Grants[0]
	if grant.Source != automationdomain.PermissionGrantSourceLegacyCompat ||
		!permissionGrantMatches(grant, buildScriptPermissionCapability(*persisted)) {
		t.Fatalf("旧脚本未获得 hash-bound legacy grant: %+v", grant)
	}
}

func resetTaskPermissionPolicyToLegacy(t *testing.T, db *sql.DB, jobID string) {
	t.Helper()
	if _, err := db.Exec(`
UPDATE automation_scheduled_tasks
SET permission_policy_json = '{}',
    permission_policy_revision = 0,
    permission_state = 'uninitialized',
    pending_permission_request_id = NULL
WHERE job_id = ?
`, jobID); err != nil {
		t.Fatal(err)
	}
}

func newMainWindowTestTask(t *testing.T, service *Service, name string) *automationdomain.ScheduledTask {
	t.Helper()
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          name,
		AgentID:       "agent-1",
		Instruction:   "处理" + name,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetMain},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	return task
}

func TestMainTargetRunFailsWhenMainWindowRejects(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission, mainErr: errors.New("agent agent-1 has no main window")}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, dm, nil, permission, &fakeWorkspaceReader{}, nil)
	task := newMainWindowTestTask(t, service, "无主窗口")

	if _, err := service.RunTaskNow(context.Background(), task.JobID); err == nil {
		t.Fatal("主窗口拒收时运行应失败")
	}
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].Status != automationdomain.RunStatusFailed || runs[0].FinishedAt == nil {
		t.Fatalf("拒收应把 run 收口为 failed: runs=%+v err=%v", runs, err)
	}
}

func TestTaskRevisionSupersedesPendingPermissionRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, requiredTool: "WebSearch"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "修订失效测试",
		AgentID:     "agent-1",
		Instruction: "旧任务定义",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-revision", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1
	})
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取待审批请求失败: requests=%+v err=%v", requests, err)
	}
	newInstruction := "新任务定义"
	updated, err := service.UpdateTask(ownerCtx, task.JobID, automationdomain.UpdateJobInput{Instruction: &newInstruction})
	if err != nil {
		t.Fatalf("UpdateTask 失败: %v", err)
	}
	if updated.PermissionPolicy.Revision <= task.PermissionPolicy.Revision ||
		updated.PermissionState != automationdomain.TaskPermissionStateReady || updated.PendingPermissionRequestID != "" {
		t.Fatalf("任务修订未切换到新权限边界: %+v", updated)
	}
	request, err := service.repository.GetPermissionRequest(ownerCtx, task.OwnerUserID, requests[0].RequestID)
	if err != nil || request.Status != automationdomain.PermissionRequestStatusSuperseded {
		t.Fatalf("旧审批请求未失效: request=%+v err=%v", request, err)
	}
	runs, err := service.ListTaskRuns(ownerCtx, task.JobID)
	if err != nil || len(runs) != 1 || runs[0].Status != automationdomain.RunStatusCancelled {
		t.Fatalf("旧权限边界下的阻塞 run 未取消: runs=%+v err=%v", runs, err)
	}
	if _, err = service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowOnce),
	); !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("旧审批请求不应在任务修订后生效: %v", err)
	}
}

func TestPermissionInputSummaryRedactsSecretsAndSignedURLQueries(t *testing.T) {
	summary := summarizePermissionInput(map[string]any{
		"access_token": "secret-token",
		"url":          "https://example.com/doc?token=signed-secret#fragment",
		"nested": map[string]any{
			"password": "secret-password",
		},
	})
	if summary["access_token"] != "[redacted]" {
		t.Fatalf("access token 未脱敏: %+v", summary)
	}
	urlValue, _ := summary["url"].(string)
	if strings.Contains(urlValue, "signed-secret") || strings.Contains(urlValue, "?") || strings.Contains(urlValue, "#") {
		t.Fatalf("审批摘要不应保留签名 URL 查询或 fragment: %q", urlValue)
	}
	nested, _ := summary["nested"].(map[string]any)
	if nested["password"] != "[redacted]" {
		t.Fatalf("嵌套凭据未脱敏: %+v", summary)
	}
}

type permissionDispatchFailureDMRunner struct {
	noMainWindow
	err error
}

func (r *permissionDispatchFailureDMRunner) HandleChat(context.Context, dmsvc.Request) error {
	return r.err
}

func TestDeniedPermissionRunClosesDeliveryAndObservabilityPendingState(t *testing.T) {
	db := newPermissionReliabilityMigratedDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限拒绝投递收口",
		AgentID:     "agent-1",
		Instruction: "等待用户批准后执行",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      fakeStructuredDelivery("agent-1", "permission-denied"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-permission-denied-delivery"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, Status: automationdomain.RunStatusPending,
		DeliveryStatus:           automationdomain.DeliveryStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID: "permission-denied-delivery", OwnerUserID: task.OwnerUserID,
		JobID: task.JobID, RunID: runID, PolicyRevision: task.PermissionPolicy.Revision,
		Kind: automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName: "Write", Effect: automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:permission-denied-delivery",
		},
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request: request, TaskState: automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_task_runs
SET delivery_attempts = 2, delivery_attempt_id = 'stale-attempt',
    delivery_attempt_started_at = CURRENT_TIMESTAMP,
    delivery_next_attempt_at = CURRENT_TIMESTAMP,
    delivery_dead_letter_at = CURRENT_TIMESTAMP
WHERE run_id = ?`, runID); err != nil {
		t.Fatalf("预置过期投递状态失败: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		storedRequest.RequestID,
		permissionDecisionInputForRequest(*storedRequest, automationdomain.PermissionDecisionDeny),
	)
	if err != nil {
		t.Fatalf("拒绝权限请求失败: %v", err)
	}
	if decision.Run == nil || decision.Run.Status != automationdomain.RunStatusFailed ||
		decision.Run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		decision.Run.DeliveryAttempts != 0 || decision.Run.DeliveryNextAttemptAt != nil ||
		decision.Run.DeliveryDeadLetterAt != nil {
		t.Fatalf("拒绝后的 run 未完整收口: %+v", decision.Run)
	}
	if decision.Task.LastRunStatus != automationdomain.RunStatusFailed ||
		decision.Task.LastDeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("拒绝后的任务摘要不是权威终态: %+v", decision.Task)
	}
	status, err := service.GetTaskStatus(ownerCtx, task.JobID, 10, 10)
	if err != nil {
		t.Fatalf("GetTaskStatus 失败: %v", err)
	}
	if status.Health.DeliveryPendingRunCount != 0 || len(status.Health.DeliveryPendingRunIDs) != 0 ||
		status.Health.DeliveryFailedRunCount != 0 || status.Health.ManualRedeliveryAvailable {
		t.Fatalf("无可投递结果不应暴露等待或补投动作: %+v", status.Health)
	}
	if status.Health.FailedRunCount != 1 || len(status.Health.ExecutionFailedRunIDs) != 1 ||
		status.Health.ExecutionFailedRunIDs[0] != runID {
		t.Fatalf("权限拒绝仍应保留执行失败事实: %+v", status.Health)
	}
}

func newPermissionReliabilityMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission-reliability.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agents (
id, slug, name, description, definition, status, workspace_path
) VALUES ('agent-1', 'agent-1', 'Agent 1', '', '', 'active', '/tmp/agent-1')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestResumePermissionRunFailureKeepsExactActionableRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "重试补偿保留审批身份",
		AgentID:     "agent-1",
		Instruction: "执行已批准操作",
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

	runID := "run-resume-compensation"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    task.JobID,
		OwnerUserID:              task.OwnerUserID,
		TriggerKind:              automationdomain.TriggerKindManual,
		Status:                   automationdomain.RunStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID:      "permission-resume-compensation",
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		PolicyRevision: task.PermissionPolicy.Revision,
		Kind:           automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName:         "Write",
			Effect:           automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:resume-compensation",
		},
		ResumeSafe: false,
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	request = *storedRequest
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry {
		t.Fatalf("权限批准未进入显式重试状态: result=%+v err=%v", decision, err)
	}
	historicalRequest := request
	if err = service.repository.SetTaskPermissionState(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		automationdomain.TaskPermissionStateReady,
		"",
	); err != nil {
		t.Fatalf("模拟下一次权限阻塞前重置任务失败: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_task_runs
SET block_state = '', blocked_request_id = NULL
WHERE run_id = ? AND owner_user_id = ?`,
		runID,
		task.OwnerUserID,
	); err != nil {
		t.Fatalf("模拟下一次权限阻塞前重置 run 失败: %v", err)
	}
	request.RequestID = "permission-resume-current"
	request.Capability.ToolName = "WebFetch"
	request.Capability.InputFingerprint = "sha256:resume-current"
	storedRequest, _, err = service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建同 run/revision 的后继权限请求失败: %v", err)
	}
	request = *storedRequest
	decision, err = service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Run == nil || decision.Run.BlockedRequestID != request.RequestID {
		t.Fatalf("后继审批没有成为 run 的 exact retry identity: result=%+v err=%v", decision, err)
	}

	if _, err = db.Exec(
		`UPDATE automation_task_runs SET blocked_request_id = NULL WHERE run_id = ?`,
		runID,
	); err != nil {
		t.Fatalf("制造旧版 run 请求投影失败: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks SET running_run_id = ? WHERE job_id = ?`,
		"competing-run",
		task.JobID,
	); err != nil {
		t.Fatalf("制造运行占用冲突失败: %v", err)
	}
	actionable, err := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("旧版 task exact request 应能定位同一 run: requests=%+v err=%v", actionable, err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(historicalRequest),
	); !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("历史 approved request 不得恢复当前阻塞: %v", err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(request),
	); err == nil {
		t.Fatal("运行占用冲突时 ResumePermissionRun 应失败")
	}
	var boundRequestID sql.NullString
	if err = db.QueryRow(
		`SELECT blocked_request_id FROM automation_task_runs WHERE run_id = ?`,
		runID,
	).Scan(&boundRequestID); err != nil {
		t.Fatalf("读取旧版 run 修复结果失败: %v", err)
	}
	if !boundRequestID.Valid || boundRequestID.String != request.RequestID {
		t.Fatalf("旧版 run 未绑定 exact request_id: %+v", boundRequestID)
	}

	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks
SET permission_state = ?, pending_permission_request_id = NULL
WHERE job_id = ?`,
		automationdomain.TaskPermissionStateReady,
		task.JobID,
	); err != nil {
		t.Fatalf("制造 task 补偿投影丢失失败: %v", err)
	}
	actionable, err = service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("exact run/request 应能修复丢失的任务投影: requests=%+v err=%v", actionable, err)
	}
	if _, err = service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		runID,
		permissionResumeInputForRequest(request),
	); err == nil {
		t.Fatal("投影修复后仍有运行占用冲突时 ResumePermissionRun 应失败")
	}

	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil {
		t.Fatalf("读取补偿后的任务失败: task=%+v err=%v", persisted, err)
	}
	if persisted.PermissionState != automationdomain.TaskPermissionStateReadyToRetry ||
		persisted.PendingPermissionRequestID != request.RequestID {
		t.Fatalf("失败补偿丢失 exact request_id: %+v", persisted)
	}
	actionable, err = service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
		t.Fatalf("失败补偿后的请求不可再次操作: requests=%+v err=%v", actionable, err)
	}
}

func TestResumePermissionRunDispatchFailureRestoresExactRunRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	dispatchErr := errors.New("injected resume dispatch failure")
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&permissionDispatchFailureDMRunner{err: dispatchErr},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "恢复派发失败仍可重试",
		AgentID:     "agent-1",
		Instruction: "执行已批准操作",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-dispatch-failure", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-resume-dispatch-failure"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    task.JobID,
		OwnerUserID:              task.OwnerUserID,
		TriggerKind:              automationdomain.TriggerKindManual,
		Status:                   automationdomain.RunStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("创建阻塞 run 失败: %v", err)
	}
	request := automationdomain.AutomationPermissionRequest{
		RequestID:      "permission-resume-dispatch-failure",
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		PolicyRevision: task.PermissionPolicy.Revision,
		Kind:           automationdomain.PermissionRequestKindTool,
		Capability: automationdomain.PermissionCapability{
			ToolName:         "Write",
			Effect:           automationdomain.PermissionEffectWrite,
			InputFingerprint: "sha256:resume-dispatch-failure",
		},
		ResumeSafe: false,
	}
	storedRequest, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		automationstore.PermissionRequestCreateInput{
			Request:    request,
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("创建权限请求失败: %v", err)
	}
	request = *storedRequest
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil || decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry {
		t.Fatalf("权限批准未进入显式重试状态: result=%+v err=%v", decision, err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = service.ResumePermissionRun(
			ownerCtx,
			task.JobID,
			runID,
			permissionResumeInputForRequest(request),
		); !errors.Is(err, dispatchErr) {
			t.Fatalf("第 %d 次恢复没有返回派发错误: %v", attempt, err)
		}
		persisted, taskErr := service.repository.GetScheduledTask(
			context.Background(),
			task.OwnerUserID,
			task.JobID,
		)
		persistedRun, runErr := service.repository.GetRun(
			context.Background(),
			task.OwnerUserID,
			task.JobID,
			runID,
		)
		if taskErr != nil || persisted == nil ||
			persisted.PermissionState != automationdomain.TaskPermissionStateReadyToRetry ||
			persisted.PendingPermissionRequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后 task 丢失 exact request: task=%+v err=%v", attempt, persisted, taskErr)
		}
		if runErr != nil || persistedRun == nil ||
			persistedRun.Status != automationdomain.RunStatusPending ||
			persistedRun.BlockState != automationdomain.RunBlockStateReadyToRetry ||
			persistedRun.BlockedRequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后 run 不可再次恢复: run=%+v err=%v", attempt, persistedRun, runErr)
		}
		actionable, listErr := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
		if listErr != nil || len(actionable) != 1 || actionable[0].RequestID != request.RequestID {
			t.Fatalf("第 %d 次失败后审批卡不可操作: requests=%+v err=%v", attempt, actionable, listErr)
		}
	}
}

type automationPermissionEventRecorder struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (r *automationPermissionEventRecorder) NotifyAutomationPermissionEvent(
	_ context.Context,
	event protocol.EventMessage,
) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *automationPermissionEventRecorder) snapshot() []protocol.EventMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]protocol.EventMessage(nil), r.events...)
}

func TestAutomationPermissionProjectsToRecipientDMAndResolvesFromComposer(t *testing.T) {
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:               permission,
		requiredTool:             "WebSearch",
		skipPermissionAfterFirst: true,
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	recipientSession := protocol.BuildRoomAgentSessionKey(
		"dm-recipient-conversation",
		"agent-recipient",
		protocol.RoomTypeDM,
	)
	roomID := "dm-recipient-room"
	conversationID := "dm-recipient-conversation"
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey:     recipientSession,
			AgentID:        "agent-recipient",
			ChannelType:    protocol.SessionChannelWebSocket,
			RoomID:         &roomID,
			ConversationID: &conversationID,
		},
	}})
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	task, err := service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
		Name:        "接收会话审批",
		AgentID:     "agent-executor",
		Instruction: "搜索最新资料",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeExplicit,
			Channel:    protocol.SessionChannelWebSocket,
			To:         recipientSession,
			SessionKey: recipientSession,
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err = service.RunTaskNow(ownerCtx, task.JobID); err != nil {
		t.Fatalf("RunTaskNow() error = %v", err)
	}

	var request automationdomain.AutomationPermissionRequest
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(
			ownerCtx,
			automationdomain.PermissionRequestStatusPending,
			task.JobID,
		)
		if listErr != nil || len(requests) != 1 {
			return false
		}
		request = requests[0]
		return len(recorder.snapshot()) == 1
	})
	events := recorder.snapshot()
	if request.DeliverySessionKey != recipientSession ||
		len(events) != 1 ||
		events[0].EventType != protocol.EventTypePermissionRequest ||
		events[0].SessionKey != recipientSession ||
		events[0].RoomID != roomID ||
		events[0].ConversationID != conversationID ||
		events[0].AgentID != "agent-recipient" ||
		events[0].Data["request_source"] != "automation" ||
		events[0].Data["automation_task_name"] != task.Name ||
		events[0].Data["automation_allow_task"] != true {
		t.Fatalf("recipient permission event = %+v, request = %+v", events, request)
	}

	replayed, err := service.ListSessionPermissionEvents(ownerCtx, recipientSession)
	if err != nil || len(replayed) != 1 ||
		replayed[0].Data["request_id"] != request.RequestID {
		t.Fatalf("ListSessionPermissionEvents() = %+v, %v", replayed, err)
	}
	wrongSession := protocol.BuildAgentSessionKey(
		"agent-recipient",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"wrong-conversation",
		"",
	)
	handled, err := service.ResolveSessionPermissionResponse(ownerCtx, wrongSession, map[string]any{
		"request_id": request.RequestID,
		"decision":   "allow",
	})
	if err != nil || handled {
		t.Fatalf("wrong recipient must not handle request: handled=%v err=%v", handled, err)
	}

	handled, err = service.ResolveSessionPermissionResponse(ownerCtx, recipientSession, map[string]any{
		"request_id":       request.RequestID,
		"decision":         "allow",
		"automation_scope": "task",
	})
	if err != nil || !handled {
		t.Fatalf("recipient decision failed: handled=%v err=%v", handled, err)
	}
	stored, err := service.repository.GetPermissionRequest(
		ownerCtx,
		task.OwnerUserID,
		request.RequestID,
	)
	if err != nil || stored.Status != automationdomain.PermissionRequestStatusApproved ||
		stored.Decision != automationdomain.PermissionDecisionAllowTask {
		t.Fatalf("stored decision = %+v, %v", stored, err)
	}
	events = recorder.snapshot()
	if len(events) != 2 ||
		events[1].EventType != protocol.EventTypePermissionRequestResolved ||
		events[1].SessionKey != recipientSession ||
		events[1].Data["request_id"] != request.RequestID {
		t.Fatalf("resolved recipient event = %+v", events)
	}
	if replayed, err = service.ListSessionPermissionEvents(ownerCtx, recipientSession); err != nil || len(replayed) != 0 {
		t.Fatalf("resolved request must not replay: %+v, %v", replayed, err)
	}
	requestIDs, err := service.PendingPermissionRequestIDsForRoom(ownerCtx, roomID, conversationID)
	if err != nil || len(requestIDs) != 0 {
		t.Fatalf("resolved request must leave room snapshot: %+v, %v", requestIDs, err)
	}
}

func TestAutomationPermissionProjectsToConfiguredRoom(t *testing.T) {
	permission := permissionctx.NewContext()
	conversationID := "room-recipient-conversation"
	roomID := "room-recipient"
	room := &fakeRoomRunner{
		permission: permission,
		contexts: map[string]*protocol.ConversationContextAggregate{
			conversationID: {
				Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
				Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
			},
		},
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		room,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	roomSession := protocol.BuildRoomSharedSessionKey(conversationID)
	job := automationdomain.ScheduledTask{
		JobID:       "task-room-recipient",
		OwnerUserID: "user-1",
		Name:        "Room 收件任务",
		AgentID:     "agent-executor",
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeExplicit,
			SessionKey: roomSession,
		},
		PermissionPolicy: automationdomain.TaskPermissionPolicy{
			Version:  taskPermissionPolicyVersion,
			Revision: 1,
		},
		PermissionState: automationdomain.TaskPermissionStateReady,
	}
	job, request := persistAutomationPermissionSessionFixture(t, service, ownerCtx, job, "permission-room-recipient")
	cancelledCtx, cancel := context.WithCancel(ownerCtx)
	cancel()
	service.publishScheduledPermissionRequest(cancelledCtx, scheduledPermissionScope{
		Job: job, RunID: request.RunID, SessionKey: request.SessionKey, RoundID: request.RoundID,
	}, request)

	events := recorder.snapshot()
	if len(events) != 1 || events[0].SessionKey != roomSession ||
		events[0].RoomID != roomID || events[0].ConversationID != conversationID ||
		events[0].AgentID != job.AgentID {
		t.Fatalf("Room recipient event = %+v", events)
	}
	requestIDs, err := service.PendingPermissionRequestIDsForRoom(ownerCtx, roomID, conversationID)
	if err != nil || len(requestIDs) != 1 || requestIDs[0] != request.RequestID {
		t.Fatalf("Room pending snapshot = %+v, %v", requestIDs, err)
	}
}

func persistAutomationPermissionSessionFixture(
	t *testing.T,
	service *Service,
	ctx context.Context,
	job automationdomain.ScheduledTask,
	requestID string,
) (automationdomain.ScheduledTask, automationdomain.AutomationPermissionRequest) {
	t.Helper()
	persisted, err := service.repository.UpsertScheduledTask(ctx, job)
	if err != nil {
		t.Fatalf("UpsertScheduledTask() error = %v", err)
	}
	runID := "run-" + requestID
	if err = service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    persisted.JobID,
		OwnerUserID:              persisted.OwnerUserID,
		PermissionPolicyRevision: persisted.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("InsertRunPending() error = %v", err)
	}
	request, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		ctx,
		automationstore.PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID:          requestID,
				OwnerUserID:        persisted.OwnerUserID,
				JobID:              persisted.JobID,
				RunID:              runID,
				PolicyRevision:     persisted.PermissionPolicy.Revision,
				Kind:               automationdomain.PermissionRequestKindTool,
				Capability:         automationdomain.PermissionCapability{ToolName: "WebSearch", Effect: automationdomain.PermissionEffectRead, InputFingerprint: requestID},
				Description:        "需要搜索资料",
				DeliverySessionKey: automationPermissionApprovalSessionKey(persisted.Delivery, persisted.Source),
				ResumeSafe:         true,
			},
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("CreatePermissionRequestAndBlockRun() error = %v", err)
	}
	updated, err := service.repository.GetScheduledTask(ctx, persisted.OwnerUserID, persisted.JobID)
	if err != nil || updated == nil {
		t.Fatalf("GetScheduledTask() = %+v, %v", updated, err)
	}
	return *updated, *request
}

func TestUpdateTaskRebindsAgentAndPermissionSnapshotAtomically(t *testing.T) {
	ownerUserID := authctx.SystemUserID
	agentAWorkspace := newAutomationOwnerWorkspace(t, ownerUserID, "agent-a")
	agentBWorkspace := newAutomationOwnerWorkspace(t, ownerUserID, "agent-b")
	agentBSession := protocol.BuildAgentSessionKey("agent-b", "ws", "dm", "daily-ops", "")
	now := time.Now().UTC()
	if _, err := workspacestore.NewSessionFileStore(agentBWorkspace).
		ForOwner(ownerUserID).
		UpsertSession(agentBWorkspace, protocol.Session{
			SessionKey:   agentBSession,
			AgentID:      "agent-b",
			ChannelType:  protocol.SessionChannelWebSocket,
			ChatType:     protocol.RoomTypeDM,
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options: protocol.WithSessionRuntimeSettings(nil, protocol.SessionRuntimeSettings{
				PermissionMode: automationdomain.PermissionModeDontAsk,
			}),
		}); err != nil {
		t.Fatalf("准备新执行 Session: %v", err)
	}

	authority := &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-a": {
			AgentID: "agent-a", OwnerUserID: ownerUserID, WorkspacePath: agentAWorkspace,
			Options: protocol.Options{PermissionMode: automationdomain.PermissionModePlan, AllowedTools: []string{"WebSearch"}},
		},
		"agent-b": {
			AgentID: "agent-b", OwnerUserID: ownerUserID, WorkspacePath: agentBWorkspace,
			Options: protocol.Options{PermissionMode: automationdomain.PermissionModeAcceptEdits, AllowedTools: []string{"Read"}, DisallowedTools: []string{"Write"}},
		},
	}}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: agentBWorkspace},
		newAutomationTestDB(t), nil, nil, nil, nil, &fakeWorkspaceReader{}, nil,
	)
	service.agents = authority
	created, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "切换执行智能体", AgentID: "agent-a", Instruction: "生成日报",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:        automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	agentB := "agent-b"
	bound := automationdomain.SessionTarget{Kind: automationdomain.SessionTargetBound, BoundSessionKey: agentBSession}
	copyMode := ""
	updated, err := service.UpdateTask(context.Background(), created.JobID, automationdomain.UpdateJobInput{
		AgentID: &agentB, SessionTarget: &bound, PermissionMode: &copyMode,
	})
	if err != nil {
		t.Fatalf("UpdateTask agent rebind: %v", err)
	}
	if updated.AgentID != "agent-b" || updated.SessionTarget.BoundSessionKey != agentBSession {
		t.Fatalf("执行身份与 Session 未原子重绑: %+v", updated)
	}
	if updated.PermissionMode != automationdomain.PermissionModeDontAsk ||
		!slices.Equal(updated.PermissionPolicy.DeniedTools, []string{"Write"}) ||
		updated.PermissionPolicy.Revision != created.PermissionPolicy.Revision+1 {
		t.Fatalf("新执行方权限快照不正确: %+v", updated)
	}
	runtimePolicy := taskRuntimeToolPolicy(*updated)
	if runtimePolicy == nil || !slices.Contains(runtimePolicy.AllowedTools, "Read") ||
		slices.Contains(runtimePolicy.AllowedTools, "WebSearch") {
		t.Fatalf("仍在使用旧执行智能体工具快照: %+v", runtimePolicy)
	}
}
