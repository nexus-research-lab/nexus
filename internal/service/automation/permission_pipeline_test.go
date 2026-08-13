package automation

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

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
		requiredTool:     "mcp__nexus_connectors__feishu_docx_read",
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
		dispatched[1].AutomationRun.ResumeToolName != "mcp__nexus_connectors__feishu_docx_read" {
		t.Fatalf("续跑未通过可信上下文要求重试触发审批的工具: %+v", dispatched[1].AutomationRun)
	}
	if len(interrupts) != 1 || interrupts[0].RoundID != dispatched[0].RoundID {
		t.Fatalf("没有精确停止触发审批的旧 round: interrupts=%+v requests=%+v", interrupts, dispatched)
	}
}

func TestPermissionDecisionRejectsStaleCardSnapshot(t *testing.T) {
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
		Name:        "过期审批卡",
		AgentID:     "agent-1",
		Instruction: "搜索资料",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "stale-permission-card", ""),
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
		t.Fatalf("读取审批请求失败: requests=%+v err=%v", requests, err)
	}
	request := requests[0]
	staleInput := permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce)
	staleInput.RunID = "run_from_stale_card"
	if _, err = service.ResolvePermissionRequest(ownerCtx, request.RequestID, staleInput); !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("页面看到的 run 与请求不一致时必须拒绝审批: %v", err)
	}
	stored, err := service.repository.GetPermissionRequest(ownerCtx, task.OwnerUserID, request.RequestID)
	if err != nil || stored.Status != automationdomain.PermissionRequestStatusPending {
		t.Fatalf("错误目标快照不应消耗审批请求: request=%+v err=%v", stored, err)
	}
	if err = service.repository.SetTaskPermissionState(
		ownerCtx,
		task.OwnerUserID,
		task.JobID,
		automationdomain.TaskPermissionStateAwaitingApproval,
		"permission_newer_request",
	); err != nil {
		t.Fatalf("切换任务当前请求失败: %v", err)
	}
	if _, err = service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowOnce),
	); !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("非任务当前请求不得继续审批: %v", err)
	}
	actionable, err := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 0 {
		t.Fatalf("非当前请求不应继续出现在可操作列表: requests=%+v err=%v", actionable, err)
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

func TestPermissionPipelineRequiresExplicitRetryAfterEffectStarted(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		requiredTools: []string{
			"mcp__nexus_connectors__feishu_docx_write",
			"WebSearch",
		},
	}
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
		Name:        "先写入再搜索",
		AgentID:     "agent-1",
		Instruction: "先写入飞书，再搜索补充资料",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-effect", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	policy := appendTaskPermissionGrant(task.PermissionPolicy, automationdomain.TaskPermissionGrant{
		GrantID: "grant-write",
		Capability: automationdomain.PermissionCapability{
			ToolName:    "mcp__nexus_connectors__feishu_docx_write",
			ConnectorID: "feishu-docx",
			Effect:      automationdomain.PermissionEffectWrite,
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
		t.Fatalf("预置写能力授权失败: updated=%v err=%v", updated, err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil {
		t.Fatalf("RunTaskNow 失败: result=%+v err=%v", result, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1 && !requests[0].ResumeSafe
	})
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取副作用后的权限请求失败: requests=%+v err=%v", requests, err)
	}
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowOnce),
	)
	if err != nil {
		t.Fatalf("allow_once 失败: %v", err)
	}
	if decision.ResumeStarted || decision.Task.PermissionState != automationdomain.TaskPermissionStateReadyToRetry ||
		decision.Task.PendingPermissionRequestID != requests[0].RequestID {
		t.Fatalf("越过副作用边界后不得自动重放: %+v", decision)
	}
	if decision.Run == nil || decision.Run.BlockState != automationdomain.RunBlockStateReadyToRetry || !decision.Run.EffectStarted {
		t.Fatalf("run 未保留显式重试边界: %+v", decision.Run)
	}
	actionable, err := service.ListPermissionRequests(ownerCtx, "actionable", task.JobID)
	if err != nil || len(actionable) != 1 || actionable[0].Status != automationdomain.PermissionRequestStatusApproved {
		t.Fatalf("已审批但待确认重试的卡片不可恢复: requests=%+v err=%v", actionable, err)
	}
	resumed, err := service.ResumePermissionRun(
		ownerCtx,
		task.JobID,
		*result.RunID,
		permissionResumeInputForRequest(requests[0]),
	)
	if err != nil || !resumed.ResumeStarted {
		t.Fatalf("显式确认重试失败: result=%+v err=%v", resumed, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].RunID != *result.RunID || runs[0].Attempts != 2 {
		t.Fatalf("显式重试未复用 logical run: runs=%+v err=%v", runs, err)
	}
}

func TestPermissionPipelineSeparatesFeishuGrantFromOAuthReadiness(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:   permission,
		requiredTool: "mcp__nexus_connectors__feishu_docx_read",
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

func TestLegacyScheduledTaskPermissionBackfillPreservesTaskAndCreatesRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	createdBy := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := createdBy.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "旧版飞书任务",
		AgentID:     "agent-legacy",
		Instruction: "读取飞书文档并总结",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("创建兼容测试任务失败: %v", err)
	}
	resetTaskPermissionPolicyToLegacy(t, db, task.JobID)

	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:   permission,
		requiredTool: "mcp__nexus_connectors__feishu_docx_read",
	}
	recovered := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	if err = recovered.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("旧任务权限策略回填失败: %v", err)
	}
	persisted, err := recovered.repository.GetScheduledTask(
		context.Background(), task.OwnerUserID, task.JobID,
	)
	if err != nil || persisted == nil {
		t.Fatalf("读取回填后的旧任务失败: task=%+v err=%v", persisted, err)
	}
	if persisted.JobID != task.JobID || persisted.Name != task.Name ||
		persisted.AgentID != task.AgentID || persisted.Instruction != task.Instruction ||
		!reflect.DeepEqual(persisted.Schedule.Normalized(), task.Schedule.Normalized()) {
		t.Fatalf("权限回填改变了旧任务定义: before=%+v after=%+v", task, persisted)
	}
	if persisted.PermissionPolicy.Revision != 1 ||
		persisted.PermissionState != automationdomain.TaskPermissionStateReady {
		t.Fatalf("旧任务权限策略未初始化: %+v", persisted)
	}

	result, err := recovered.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil {
		t.Fatalf("回填后的旧任务无法运行: result=%+v err=%v", result, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := recovered.ListPermissionRequests(
			ownerCtx,
			automationdomain.PermissionRequestStatusPending,
			task.JobID,
		)
		return listErr == nil && len(requests) == 1
	})
	requests, err := recovered.ListPermissionRequests(
		ownerCtx,
		automationdomain.PermissionRequestStatusPending,
		task.JobID,
	)
	if err != nil || len(requests) != 1 ||
		requests[0].Capability.ToolName != "mcp__nexus_connectors__feishu_docx_read" ||
		requests[0].PolicyRevision != 1 {
		t.Fatalf("旧任务未进入持久权限确认链路: requests=%+v err=%v", requests, err)
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

func TestScriptPermissionGrantIsBoundToExactScriptContent(t *testing.T) {
	service := &Service{idFactory: func(prefix string) string { return prefix + "-1" }}
	job := automationdomain.ScheduledTask{
		OwnerUserID:   "owner-1",
		AgentID:       "agent-1",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Instruction:   "printf first",
	}
	grant := service.scriptPermissionGrant(job, automationdomain.PermissionGrantSourceUserApproval)
	if !permissionGrantMatches(grant, buildScriptPermissionCapability(job)) {
		t.Fatalf("原脚本内容应命中 hash-bound grant: %+v", grant)
	}
	job.Instruction = "printf second"
	if permissionGrantMatches(grant, buildScriptPermissionCapability(job)) {
		t.Fatalf("脚本内容改变后不得复用旧 grant: %+v", grant)
	}
}

func TestFeishuPermissionEffectsMatchConnectorToolSemantics(t *testing.T) {
	readOnlyTools := []string{
		"feishu_docx_read",
		"feishu_docx_search",
		"feishu_docx_sheet_sheets",
		"feishu_docx_sheet_values",
		"feishu_docx_sheet_find",
		"feishu_docx_bitable_tables",
		"feishu_docx_bitable_fields",
		"feishu_docx_bitable_records",
		"feishu_docx_drive_list",
		"feishu_docx_wiki_spaces",
		"feishu_docx_wiki_space",
		"feishu_docx_wiki_nodes",
		"feishu_docx_wiki_node",
	}
	for _, toolName := range readOnlyTools {
		qualified := "mcp__nexus_connectors__" + toolName
		if effect := classifyPermissionEffect(qualified); effect != automationdomain.PermissionEffectRead {
			t.Errorf("%s effect = %q, want read", qualified, effect)
		}
	}
	writeTools := []string{
		"feishu_docx_create",
		"feishu_docx_append_markdown",
		"feishu_docx_update_block",
	}
	for _, toolName := range writeTools {
		qualified := "mcp__nexus_connectors__" + toolName
		if effect := classifyPermissionEffect(qualified); effect != automationdomain.PermissionEffectWrite {
			t.Errorf("%s effect = %q, want write", qualified, effect)
		}
	}
}

func TestMainSessionPermissionRequestKeepsOriginalTaskRunContext(t *testing.T) {
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
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 3600,
		TargetMode:   automationdomain.HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "主会话搜索",
		AgentID:     "agent-1",
		Instruction: "在主会话搜索资料",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:     automationdomain.SessionTargetMain,
			WakeMode: automationdomain.WakeModeNow,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil || result.Status != automationdomain.RunStatusQueuedToMain {
		t.Fatalf("Main Session run 入队失败: result=%+v err=%v", result, err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1
	})
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取 Main Session 权限请求失败: requests=%+v err=%v", requests, err)
	}
	request := requests[0]
	if request.RunID != *result.RunID || request.JobID != task.JobID ||
		request.PolicyRevision != task.PermissionPolicy.Revision ||
		request.SessionKey != result.SessionKey || request.RoundID == "" {
		t.Fatalf("Main Session 权限请求丢失 task/run/session/revision 上下文: %+v", request)
	}
	resolved, err := service.ResolvePermissionRequest(
		ownerCtx,
		request.RequestID,
		permissionDecisionInputForRequest(request, automationdomain.PermissionDecisionAllowTask),
	)
	if err != nil || !resolved.ResumeStarted {
		t.Fatalf("Main Session 权限批准后重新入队失败: result=%+v err=%v", resolved, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].RunID != *result.RunID || runs[0].Attempts != 2 {
		t.Fatalf("Main Session 审批后未继续同一 logical run: runs=%+v err=%v", runs, err)
	}
	dmRequests := dm.Requests()
	if len(dmRequests) != 2 || dmRequests[0].PermissionHandler == nil || dmRequests[1].PermissionHandler == nil ||
		dmRequests[0].RoundID == dmRequests[1].RoundID {
		t.Fatalf("Main Session 每个 attempt 必须绑定独立 round 和 task permission handler: %+v", dmRequests)
	}
	if dmRequests[1].AutomationRun == nil || dmRequests[1].AutomationRun.ResumeToolName != "WebSearch" {
		t.Fatalf("Main Session 权限续跑未携带原审批工具: %+v", dmRequests[1].AutomationRun)
	}
}

func TestMainSessionTaskArrivingDuringHeartbeatRunsImmediatelyAfterCurrentRound(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission, delay: 200 * time.Millisecond}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{files: map[string]string{
			"HEARTBEAT.md": "检查一次当前状态。",
		}},
		nil,
	)
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 3600,
		TargetMode:   automationdomain.HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "并发到达的主会话任务",
		AgentID:     "agent-1",
		Instruction: "处理刚刚到达的定时任务",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:     automationdomain.SessionTargetMain,
			WakeMode: automationdomain.WakeModeNow,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{
		Mode: automationdomain.WakeModeNow,
	}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		return len(dm.Requests()) == 1
	})
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result.RunID == nil || result.Status != automationdomain.RunStatusQueuedToMain {
		t.Fatalf("运行中到达的 Main Session task 入队失败: result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	requests := dm.Requests()
	if len(requests) != 2 || requests[1].PermissionHandler == nil {
		t.Fatalf("普通 heartbeat 完成后应立即以 task 权限上下文执行下一轮: %+v", requests)
	}
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].RunID != *result.RunID || runs[0].Attempts != 1 {
		t.Fatalf("并发到达任务应完成原 logical run: runs=%+v err=%v", runs, err)
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
