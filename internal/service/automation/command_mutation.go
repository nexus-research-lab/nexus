// INPUT: round-scoped Actor、Automation mutation operation 与 CLI intent。
// OUTPUT: 不写入的确定性 plan，或经 digest/revision/人工确认后的领域写入结果。
// POS: Nexus Automation command 的唯一变更入口；tool input 和模型文本都不能绕过本层。
package automation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

var runtimeAutomationRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

type RuntimeCommandApplyOptions struct {
	HumanConfirmed         bool
	HumanApprovalRequestID string
}

// PlanRuntimeCommand 解析并验证一次 Automation 变更，但不写入。
func (s *Service) PlanRuntimeCommand(
	ctx context.Context,
	actor command.Actor,
	operation string,
	input automationdomain.AutomationCommandInput,
) (*automationdomain.AutomationCommandPlan, error) {
	if s == nil || !actor.Valid() {
		return nil, errors.New("Automation runtime command Actor 无效")
	}
	if !actor.MutationAllowed() {
		return nil, errors.New("当前 runtime round 只有 Automation 查询权限")
	}
	ctx = runtimeAutomationCommandContext(ctx, actor)
	if err := s.validateRuntimeCommandActor(ctx, actor); err != nil {
		return nil, err
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	plan := &automationdomain.AutomationCommandPlan{
		Operation: operation, RequiresConfirmation: true, Risk: "write",
	}
	var err error
	switch operation {
	case automationdomain.AutomationCommandOperationCreate:
		var createInput automationdomain.CreateJobInput
		createInput, input, err = s.runtimeCommandCreateInput(ctx, actor, input, "plan-placeholder")
		if err == nil {
			plan.Target = strings.TrimSpace(createInput.AgentID)
			plan.Summary = fmt.Sprintf("创建定时任务「%s」，执行 Agent=%s", createInput.Name, createInput.AgentID)
			plan.CurrentRevision = "new:" + createInput.AgentID
		}
	case automationdomain.AutomationCommandOperationUpdate:
		var scope runtimeCommandTaskScope
		scope, err = s.runtimeCommandTaskScope(ctx, actor, input, true)
		if err == nil {
			input.JobID = scope.JobID
			_, input, err = s.runtimeCommandUpdateInput(ctx, actor, scope.Job, input)
			plan.Target = scope.JobID
			plan.Summary = fmt.Sprintf("修改定时任务「%s」", scope.Job.Name)
			plan.CurrentRevision = runtimeTaskRevision(scope.Job)
			plan.ObservedConfigurationVersion = scope.Job.ConfigurationVersion
		}
	case automationdomain.AutomationCommandOperationDelete,
		automationdomain.AutomationCommandOperationRun:
		var scope runtimeCommandTaskScope
		scope, err = s.runtimeCommandTaskScope(ctx, actor, input, true)
		if err == nil {
			if err = rejectAgentScriptControl(ctx, scope.Job); err != nil {
				break
			}
			input.JobID = scope.JobID
			plan.Target = scope.JobID
			plan.CurrentRevision = runtimeTaskRevision(scope.Job)
			plan.ObservedConfigurationVersion = scope.Job.ConfigurationVersion
			if operation == automationdomain.AutomationCommandOperationDelete {
				plan.Risk = "destructive"
				plan.Summary = fmt.Sprintf("删除定时任务「%s」", scope.Job.Name)
			} else {
				plan.Risk = "external_effect"
				plan.Summary = fmt.Sprintf("立即运行定时任务「%s」", scope.Job.Name)
			}
		}
	case automationdomain.AutomationCommandOperationRetryDelivery:
		var scope runtimeCommandTaskScope
		scope, err = s.runtimeCommandTaskScope(ctx, actor, input, true)
		if err == nil {
			input.JobID = scope.JobID
			if strings.TrimSpace(input.RunID) == "" {
				err = errors.New("retry_delivery requires run_id")
				break
			}
			var runs []automationdomain.ScheduledTaskRun
			runs, err = s.ListTaskRuns(ctx, scope.JobID)
			if err != nil {
				break
			}
			var selected *automationdomain.ScheduledTaskRun
			for index := range runs {
				if strings.TrimSpace(runs[index].RunID) == strings.TrimSpace(input.RunID) {
					selected = &runs[index]
					break
				}
			}
			if selected == nil {
				err = automationdomain.ErrRunNotFound
				break
			}
			if err = validateDeliveryRetry(*selected); err != nil {
				break
			}
			plan.Target = scope.JobID + ":" + selected.RunID
			plan.Summary = fmt.Sprintf("只重投递任务「%s」的运行结果，不重新执行任务", scope.Job.Name)
			plan.Risk = "external_effect"
			plan.CurrentRevision = runtimeDeliveryRevision(scope.Job, *selected)
			plan.ObservedConfigurationVersion = scope.Job.ConfigurationVersion
		}
	case automationdomain.AutomationCommandOperationSetHeartbeat,
		automationdomain.AutomationCommandOperationWake:
		var agentID string
		agentID, err = runtimeCommandAgentID(actor, input.AgentID)
		if err == nil {
			input.AgentID = agentID
			var status *automationdomain.HeartbeatStatus
			status, err = s.GetHeartbeatStatus(ctx, agentID)
			if err != nil {
				break
			}
			plan.Target = agentID
			plan.CurrentRevision = runtimeHeartbeatRevision(*status)
			plan.ObservedConfigurationVersion = status.ConfigurationVersion
			if operation == automationdomain.AutomationCommandOperationSetHeartbeat {
				err = validateRuntimeHeartbeatInput(input, *status)
				plan.Summary = fmt.Sprintf("修改 Agent %s 的 heartbeat 配置", agentID)
			} else {
				err = validateRuntimeWakeInput(input)
				plan.Summary = fmt.Sprintf("唤醒 Agent %s", agentID)
				plan.Risk = "external_effect"
			}
		}
	default:
		err = fmt.Errorf("未知 Automation mutation operation %q", operation)
	}
	if err != nil {
		return nil, err
	}
	plan.Input = input
	plan.PlanDigest, err = runtimeAutomationPlanDigest(actor, *plan)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// ApplyRuntimeCommand 在同一 service 中重新 plan，并执行 revision、digest 和确认栅栏。
func (s *Service) ApplyRuntimeCommand(
	ctx context.Context,
	actor command.Actor,
	request automationdomain.AutomationCommandRequest,
	options RuntimeCommandApplyOptions,
) (*automationdomain.AutomationCommandApplyResult, error) {
	request.RequestID = strings.TrimSpace(request.RequestID)
	if !runtimeAutomationRequestIDPattern.MatchString(request.RequestID) {
		return nil, errors.New("request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
	}
	if replayed, found, replayErr := s.ReplayRuntimeCommand(ctx, actor, request); replayErr != nil {
		return nil, replayErr
	} else if found {
		return replayed, nil
	}
	plan, err := s.PlanRuntimeCommand(ctx, actor, request.Operation, request.Input)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.ExpectedRevision) == "" || request.ExpectedRevision != plan.CurrentRevision {
		return nil, fmt.Errorf("Automation 状态已变化：expected_revision=%s current_revision=%s；请重新 plan", request.ExpectedRevision, plan.CurrentRevision)
	}
	if strings.TrimSpace(request.PlanDigest) == "" || request.PlanDigest != plan.PlanDigest {
		return nil, errors.New("plan_digest 与当前 Actor、输入或 revision 不匹配；请重新 plan")
	}
	if plan.RequiresConfirmation && !options.HumanConfirmed {
		return nil, errors.New("该 Automation 变更缺少当前会话的真人确认")
	}
	if plan.RequiresConfirmation && strings.TrimSpace(options.HumanApprovalRequestID) == "" {
		return nil, errors.New("该 Automation 变更缺少真人确认 request_id")
	}
	ctx = runtimeAutomationCommandContext(ctx, actor)
	intentDigest, err := runtimeAutomationIntentDigest(actor, request.Operation, request.Input)
	if err != nil {
		return nil, err
	}
	claimed, isNew, err := s.repository.ClaimRuntimeCommand(ctx, automationstore.RuntimeCommandRecord{
		OwnerUserID: actor.OwnerUserID, RequestID: request.RequestID,
		ActorAgentID: actor.AgentID, Operation: plan.Operation, IntentDigest: intentDigest,
		ApprovalRequestID: strings.TrimSpace(options.HumanApprovalRequestID),
	})
	if err != nil {
		return nil, err
	}
	if !isNew {
		data, decodeErr := decodeRuntimeAutomationCommandResult(plan.Operation, claimed.ResultJSON)
		if decodeErr != nil {
			return nil, decodeErr
		}
		return &automationdomain.AutomationCommandApplyResult{
			Operation: plan.Operation, Outcome: "replayed", Data: data,
		}, nil
	}
	result := &automationdomain.AutomationCommandApplyResult{
		Operation: plan.Operation, Outcome: "applied",
	}
	runID := strings.TrimSpace(plan.Input.RunID)
	switch plan.Operation {
	case automationdomain.AutomationCommandOperationCreate:
		createInput, _, buildErr := s.runtimeCommandCreateInput(ctx, actor, plan.Input, request.RequestID)
		if buildErr != nil {
			return nil, buildErr
		}
		result.Data, err = s.CreateTask(ctx, createInput)
	case automationdomain.AutomationCommandOperationUpdate:
		scope, scopeErr := s.runtimeCommandTaskScope(ctx, actor, plan.Input, true)
		if scopeErr != nil {
			return nil, scopeErr
		}
		updateInput, _, buildErr := s.runtimeCommandUpdateInput(ctx, actor, scope.Job, plan.Input)
		if buildErr != nil {
			return nil, buildErr
		}
		var updated *automationdomain.ScheduledTask
		if plan.ObservedConfigurationVersion < 1 {
			return nil, automationdomain.ErrConfigurationVersionConflict
		}
		if plan.Input.CancelActiveRun {
			updated, err = s.UpdateTaskAtVersionAndRunningRun(
				ctx,
				scope.JobID,
				plan.ObservedConfigurationVersion,
				runID,
				updateInput,
			)
		} else {
			updated, err = s.UpdateTaskAtVersion(
				ctx, scope.JobID, plan.ObservedConfigurationVersion, updateInput,
			)
		}
		if err == nil && plan.Input.CancelActiveRun {
			updated, err = s.RecoverTaskRunningRun(
				ctx, scope.JobID, runID,
			)
		}
		result.Data = updated
	case automationdomain.AutomationCommandOperationDelete:
		scope, scopeErr := s.runtimeCommandTaskScope(ctx, actor, plan.Input, true)
		if scopeErr != nil {
			return nil, scopeErr
		}
		result.Data, err = s.DeleteTaskAtVersion(ctx, scope.JobID, plan.ObservedConfigurationVersion)
	case automationdomain.AutomationCommandOperationRun:
		scope, scopeErr := s.runtimeCommandTaskScope(ctx, actor, plan.Input, true)
		if scopeErr != nil {
			return nil, scopeErr
		}
		result.Data, err = s.runTaskNow(
			ctx,
			scope.JobID,
			&plan.ObservedConfigurationVersion,
			manualRunIdentity{RequestID: request.RequestID, IntentDigest: intentDigest},
		)
	case automationdomain.AutomationCommandOperationRetryDelivery:
		scope, scopeErr := s.runtimeCommandTaskScope(ctx, actor, plan.Input, true)
		if scopeErr != nil {
			return nil, scopeErr
		}
		result.Data, err = s.RetryRunDeliveryAtVersion(
			ctx, scope.JobID, runID, plan.ObservedConfigurationVersion,
		)
	case automationdomain.AutomationCommandOperationSetHeartbeat:
		status, statusErr := s.GetHeartbeatStatus(ctx, plan.Target)
		if statusErr != nil {
			return nil, statusErr
		}
		update := runtimeHeartbeatUpdate(plan.Input, *status)
		result.Data, err = s.UpdateHeartbeatAtVersion(
			ctx, plan.Target, plan.ObservedConfigurationVersion, update,
		)
	case automationdomain.AutomationCommandOperationWake:
		_, statusErr := s.GetHeartbeatStatus(ctx, plan.Target)
		if statusErr != nil {
			return nil, statusErr
		}
		result.Data, err = s.wakeHeartbeat(
			ctx,
			plan.Target,
			automationdomain.HeartbeatWakeInput{Mode: strings.TrimSpace(plan.Input.Mode), Text: commandOptionalString(plan.Input.Text)},
			&plan.ObservedConfigurationVersion,
			heartbeatWakeIdentity{
				ownerUserID: actor.OwnerUserID, requestID: request.RequestID, intentDigest: intentDigest,
			},
		)
	default:
		return nil, fmt.Errorf("未知 Automation apply operation %q", plan.Operation)
	}
	if err != nil {
		_ = s.repository.MarkRuntimeCommandUncertain(
			ctx, actor.OwnerUserID, request.RequestID, intentDigest, err,
		)
		return nil, err
	}
	encodedResult, err := json.Marshal(result.Data)
	if err != nil {
		_ = s.repository.MarkRuntimeCommandUncertain(
			ctx, actor.OwnerUserID, request.RequestID, intentDigest, err,
		)
		return nil, err
	}
	if err = s.repository.CompleteRuntimeCommand(
		ctx, actor.OwnerUserID, request.RequestID, intentDigest, string(encodedResult),
	); err != nil {
		return nil, err
	}
	return result, nil
}

// ReplayRuntimeCommand 在重新 plan 或再次请求确认前，按稳定 intent 查找已完成结果。
func (s *Service) ReplayRuntimeCommand(
	ctx context.Context,
	actor command.Actor,
	request automationdomain.AutomationCommandRequest,
) (*automationdomain.AutomationCommandApplyResult, bool, error) {
	if s == nil || !actor.Valid() {
		return nil, false, errors.New("Automation runtime command Actor 无效")
	}
	ctx = runtimeAutomationCommandContext(ctx, actor)
	if err := s.validateRuntimeCommandActor(ctx, actor); err != nil {
		return nil, false, err
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if !runtimeAutomationRequestIDPattern.MatchString(request.RequestID) {
		return nil, false, errors.New("request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符")
	}
	intentDigest, err := runtimeAutomationIntentDigest(actor, request.Operation, request.Input)
	if err != nil {
		return nil, false, err
	}
	record, err := s.repository.GetRuntimeCommand(ctx, actor.OwnerUserID, request.RequestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if record.ActorAgentID != strings.TrimSpace(actor.AgentID) ||
		record.Operation != strings.TrimSpace(request.Operation) ||
		record.IntentDigest != intentDigest {
		return nil, false, automationdomain.ErrRuntimeCommandConflict
	}
	if record.Status != "applied" {
		if record.Operation == automationdomain.AutomationCommandOperationRun {
			run, found, runErr := s.repository.GetRunByClientRequest(
				ctx, actor.OwnerUserID, "", request.RequestID, intentDigest,
			)
			if runErr != nil {
				return nil, false, runErr
			}
			if !found {
				return nil, false, automationdomain.ErrRuntimeCommandUncertain
			}
			runResult := executionResultFromRun(*run, false)
			encoded, encodeErr := json.Marshal(runResult)
			if encodeErr != nil {
				return nil, false, encodeErr
			}
			if err = s.repository.CompleteRuntimeCommandFromRun(
				ctx, actor.OwnerUserID, request.RequestID, intentDigest, string(encoded),
			); err != nil {
				return nil, false, err
			}
			return &automationdomain.AutomationCommandApplyResult{
				Operation: record.Operation, Outcome: "replayed", Data: runResult,
			}, true, nil
		}
		if record.Operation != automationdomain.AutomationCommandOperationWake {
			return nil, false, automationdomain.ErrRuntimeCommandUncertain
		}
		wake, wakeErr := s.repository.GetHeartbeatWakeByRequest(ctx, actor.OwnerUserID, request.RequestID)
		if errors.Is(wakeErr, sql.ErrNoRows) {
			return nil, false, automationdomain.ErrRuntimeCommandUncertain
		}
		if wakeErr != nil {
			return nil, false, wakeErr
		}
		expectedAgentID, agentErr := runtimeCommandAgentID(actor, request.Input.AgentID)
		if agentErr != nil {
			return nil, false, agentErr
		}
		if wake.IntentDigest != intentDigest || wake.SourceID != expectedAgentID {
			return nil, false, automationdomain.ErrRuntimeCommandConflict
		}
		var wakePayload struct {
			Mode string `json:"wake_mode"`
		}
		if err = json.Unmarshal([]byte(wake.Payload), &wakePayload); err != nil {
			return nil, false, automationdomain.ErrRuntimeCommandUncertain
		}
		wakeResult := &automationdomain.HeartbeatWakeResult{
			AgentID:   strings.TrimSpace(wake.SourceID),
			Mode:      strings.TrimSpace(wakePayload.Mode),
			Scheduled: strings.TrimSpace(wakePayload.Mode) == automationdomain.WakeModeNow,
		}
		encoded, encodeErr := json.Marshal(wakeResult)
		if encodeErr != nil {
			return nil, false, encodeErr
		}
		if err = s.repository.CompleteRuntimeCommandFromHeartbeatWake(
			ctx, actor.OwnerUserID, request.RequestID, intentDigest, string(encoded),
		); err != nil {
			return nil, false, err
		}
		return &automationdomain.AutomationCommandApplyResult{
			Operation: record.Operation, Outcome: "replayed", Data: wakeResult,
		}, true, nil
	}
	data, err := decodeRuntimeAutomationCommandResult(record.Operation, record.ResultJSON)
	if err != nil {
		return nil, false, err
	}
	return &automationdomain.AutomationCommandApplyResult{
		Operation: record.Operation, Outcome: "replayed", Data: data,
	}, true, nil
}

func decodeRuntimeAutomationCommandResult(operation string, raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, automationdomain.ErrRuntimeCommandUncertain
	}
	var target any
	switch strings.TrimSpace(operation) {
	case automationdomain.AutomationCommandOperationCreate,
		automationdomain.AutomationCommandOperationUpdate:
		target = &automationdomain.ScheduledTask{}
	case automationdomain.AutomationCommandOperationDelete:
		target = &automationdomain.DeleteJobResult{}
	case automationdomain.AutomationCommandOperationRun:
		target = &automationdomain.ExecutionResult{}
	case automationdomain.AutomationCommandOperationRetryDelivery:
		target = &automationdomain.ScheduledTaskRun{}
	case automationdomain.AutomationCommandOperationSetHeartbeat:
		target = &automationdomain.HeartbeatStatus{}
	case automationdomain.AutomationCommandOperationWake:
		target = &automationdomain.HeartbeatWakeResult{}
	default:
		return nil, fmt.Errorf("未知 Automation command replay operation %q", operation)
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return nil, fmt.Errorf("解析 Automation command replay: %w", err)
	}
	return target, nil
}

func (s *Service) runtimeCommandCreateInput(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
	requestID string,
) (automationdomain.CreateJobInput, automationdomain.AutomationCommandInput, error) {
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	input.AgentID = agentID
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Instruction) == "" {
		return automationdomain.CreateJobInput{}, input, errors.New("create requires name and instruction")
	}
	if strings.TrimSpace(input.InstructionAdd) != "" {
		return automationdomain.CreateJobInput{}, input, errors.New("create 不接受 instruction_append")
	}
	schedule, err := automationCommandSchedule(input.Schedule, actor.DefaultTimezone)
	if err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	target, delivery, err := runtimeCommandTargets(actor, input)
	if err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	expiresAt, err := runtimeCommandExpiration(input.ExpiresAt)
	if err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	createInput := automationdomain.CreateJobInput{
		RequestID: requestID, Name: strings.TrimSpace(input.Name), AgentID: agentID,
		Schedule: schedule, Instruction: strings.TrimSpace(input.Instruction),
		ExecutionKind: strings.TrimSpace(input.ExecutionKind), PermissionMode: strings.TrimSpace(input.PermissionMode),
		SessionTarget: target, Delivery: delivery, Source: runtimeCommandSource(actor),
		OverlapPolicy: strings.TrimSpace(input.OverlapPolicy), ExpiresAt: expiresAt, Enabled: enabled,
	}.Normalized()
	if automationdomain.NormalizeExecutionKind(createInput.ExecutionKind) == automationdomain.ExecutionKindScript {
		return automationdomain.CreateJobInput{}, input, errors.New("script scheduled tasks 只能由人类控制面创建")
	}
	if err = createInput.Validate(); err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	if err = s.validateAgentAndTarget(ctx, createInput.AgentID, createInput.SessionTarget); err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	ownerUserID, err := s.resolveTaskOwnerUserID(ctx, createInput.AgentID)
	if err != nil || strings.TrimSpace(ownerUserID) != strings.TrimSpace(actor.OwnerUserID) {
		if err == nil {
			err = errors.New("目标 Agent 不属于当前 owner")
		}
		return automationdomain.CreateJobInput{}, input, err
	}
	if err = s.validateTaskExpiration(createInput.ExpiresAt); err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	candidate := automationdomain.ScheduledTask{
		OwnerUserID: ownerUserID, AgentID: createInput.AgentID,
		Delivery: createInput.Delivery, Source: createInput.Source,
	}
	if err = s.prepareTaskDeliveryMutation(ctx, &candidate, &createInput.Source); err != nil {
		return automationdomain.CreateJobInput{}, input, err
	}
	return createInput, input, nil
}

func (s *Service) runtimeCommandUpdateInput(
	ctx context.Context,
	actor command.Actor,
	current automationdomain.ScheduledTask,
	input automationdomain.AutomationCommandInput,
) (automationdomain.UpdateJobInput, automationdomain.AutomationCommandInput, error) {
	input.RunID = strings.TrimSpace(input.RunID)
	input.Name = strings.TrimSpace(input.Name)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.Instruction = strings.TrimSpace(input.Instruction)
	input.InstructionAdd = strings.TrimSpace(input.InstructionAdd)
	if input.RunID != "" && !input.CancelActiveRun {
		return automationdomain.UpdateJobInput{}, input, errors.New("run_id 只能与 cancel_active_run=true 同时使用")
	}
	if input.CancelActiveRun {
		if input.Enabled != nil && *input.Enabled {
			return automationdomain.UpdateJobInput{}, input, errors.New("cancel_active_run=true 不能同时设置 enabled=true")
		}
		currentRunID := strings.TrimSpace(current.RunningRunID)
		if currentRunID == "" {
			return automationdomain.UpdateJobInput{}, input, errors.New("当前任务没有可取消的 active run")
		}
		if input.RunID != "" && input.RunID != currentRunID {
			return automationdomain.UpdateJobInput{}, input, errors.New("run_id 与当前 active run 不一致；请重新 inspect/plan")
		}
		input.RunID = currentRunID
	}
	if input.Instruction != "" && input.InstructionAdd != "" {
		return automationdomain.UpdateJobInput{}, input, errors.New("instruction 和 instruction_append 不能同时使用")
	}
	update := automationdomain.UpdateJobInput{}
	if input.Name != "" {
		update.Name = commandOptionalString(input.Name)
	}
	if input.AgentID != "" {
		agentID, err := runtimeCommandAgentID(actor, input.AgentID)
		if err != nil {
			return automationdomain.UpdateJobInput{}, input, err
		}
		input.AgentID = agentID
		if agentID != strings.TrimSpace(current.AgentID) {
			update.AgentID = &agentID
		}
	}
	if input.Instruction != "" {
		update.Instruction = commandOptionalString(input.Instruction)
	} else if input.InstructionAdd != "" {
		value := strings.TrimSpace(current.Instruction) + "\n\n" + input.InstructionAdd
		update.Instruction = &value
	}
	if input.Schedule != nil {
		schedule, err := automationCommandSchedule(input.Schedule, actor.DefaultTimezone)
		if err != nil {
			return automationdomain.UpdateJobInput{}, input, err
		}
		update.Schedule = &schedule
	}
	if strings.TrimSpace(input.ExecutionKind) != "" {
		if automationdomain.NormalizeExecutionKind(input.ExecutionKind) == automationdomain.ExecutionKindScript {
			return automationdomain.UpdateJobInput{}, input, errors.New("Agent 对话不能把任务切换为 script")
		}
		update.ExecutionKind = commandOptionalString(input.ExecutionKind)
	}
	if strings.TrimSpace(input.PermissionMode) != "" {
		update.PermissionMode = commandOptionalString(input.PermissionMode)
	}
	if strings.TrimSpace(input.OverlapPolicy) != "" {
		update.OverlapPolicy = commandOptionalString(input.OverlapPolicy)
	}
	update.Enabled = input.Enabled
	update.ClearExpiresAt = input.ClearExpiresAt
	if strings.TrimSpace(input.ExpiresAt) != "" {
		expiresAt, err := runtimeCommandExpiration(input.ExpiresAt)
		if err != nil {
			return automationdomain.UpdateJobInput{}, input, err
		}
		update.ExpiresAt = expiresAt
	}
	targetChanged := strings.TrimSpace(input.ContextMode) != "" || strings.TrimSpace(input.ExecutionMode) != "" ||
		strings.TrimSpace(input.SelectedSessionKey) != "" || strings.TrimSpace(input.NamedSessionKey) != "" || update.AgentID != nil
	deliveryChanged := input.DeliverResult != nil || strings.TrimSpace(input.ReplyMode) != "" ||
		strings.TrimSpace(input.SelectedReplySessionKey) != "" || strings.TrimSpace(input.ReplySessionKey) != "" || update.AgentID != nil
	if targetChanged || deliveryChanged {
		target, delivery, err := runtimeCommandTargets(actor, input)
		if err != nil {
			return automationdomain.UpdateJobInput{}, input, err
		}
		if targetChanged {
			update.SessionTarget = &target
		}
		if deliveryChanged {
			update.Delivery = &delivery
			source := runtimeCommandSource(actor)
			update.Source = &source
		}
	}
	if input.CancelActiveRun {
		value := false
		update.Enabled = &value
	}
	if len(changedTaskFields(update)) == 0 {
		return automationdomain.UpdateJobInput{}, input, errors.New("update 至少需要一个实际变更字段")
	}
	next, err := s.applyTaskUpdate(current, update)
	if err != nil {
		return automationdomain.UpdateJobInput{}, input, err
	}
	if err = rejectAgentScriptControl(ctx, current, next); err != nil {
		return automationdomain.UpdateJobInput{}, input, err
	}
	agentChanged := strings.TrimSpace(next.AgentID) != strings.TrimSpace(current.AgentID)
	if agentChanged && update.SessionTarget == nil {
		return automationdomain.UpdateJobInput{}, input, errors.New("changing agent_id requires session target in the same update")
	}
	if err = s.validateAgentAndTarget(ctx, next.AgentID, next.SessionTarget); err != nil {
		return automationdomain.UpdateJobInput{}, input, err
	}
	if update.Delivery != nil {
		if err = s.prepareTaskDeliveryMutation(ctx, &next, update.Source); err != nil {
			return automationdomain.UpdateJobInput{}, input, err
		}
	}
	if err = s.validateTaskUpdate(ctx, current, next); err != nil {
		return automationdomain.UpdateJobInput{}, input, err
	}
	return update, input, nil
}

func runtimeCommandExpiration(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("expires_at 必须是 RFC3339: %w", err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func runtimeTaskRevision(task automationdomain.ScheduledTask) string {
	return fmt.Sprintf("task:%s:%d", strings.TrimSpace(task.JobID), task.ConfigurationVersion)
}

func runtimeHeartbeatRevision(status automationdomain.HeartbeatStatus) string {
	return fmt.Sprintf("heartbeat:%s:%d", strings.TrimSpace(status.AgentID), status.ConfigurationVersion)
}

func runtimeDeliveryRevision(task automationdomain.ScheduledTask, run automationdomain.ScheduledTaskRun) string {
	return fmt.Sprintf("delivery:%s:%d:%s:%d", task.JobID, task.ConfigurationVersion, run.RunID, run.DeliveryAttempts)
}

func runtimeAutomationPlanDigest(actor command.Actor, plan automationdomain.AutomationCommandPlan) (string, error) {
	payload := struct {
		OwnerID    string                                  `json:"owner_id"`
		AgentID    string                                  `json:"agent_id"`
		SessionKey string                                  `json:"session_key"`
		SourceType string                                  `json:"source_type"`
		Operation  string                                  `json:"operation"`
		Target     string                                  `json:"target"`
		Revision   string                                  `json:"revision"`
		Input      automationdomain.AutomationCommandInput `json:"input"`
	}{
		OwnerID: actor.OwnerUserID, AgentID: actor.AgentID, SessionKey: actor.SessionKey,
		SourceType: actor.SourceContextType, Operation: plan.Operation, Target: plan.Target,
		Revision: plan.CurrentRevision, Input: plan.Input,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func runtimeAutomationIntentDigest(
	actor command.Actor,
	operation string,
	input automationdomain.AutomationCommandInput,
) (string, error) {
	payload := struct {
		OwnerID    string                                  `json:"owner_id"`
		ActorID    string                                  `json:"actor_id"`
		SessionKey string                                  `json:"session_key"`
		SourceType string                                  `json:"source_type"`
		Operation  string                                  `json:"operation"`
		Input      automationdomain.AutomationCommandInput `json:"input"`
	}{
		OwnerID: strings.TrimSpace(actor.OwnerUserID), ActorID: strings.TrimSpace(actor.AgentID),
		SessionKey: strings.TrimSpace(actor.SessionKey), SourceType: strings.TrimSpace(actor.SourceContextType),
		Operation: strings.TrimSpace(operation), Input: input,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateRuntimeHeartbeatInput(input automationdomain.AutomationCommandInput, current automationdomain.HeartbeatStatus) error {
	update := runtimeHeartbeatUpdate(input, current)
	config := automationdomain.HeartbeatConfig{
		AgentID: current.AgentID, Enabled: update.Enabled, EverySeconds: update.EverySeconds,
		TargetMode: update.TargetMode, AckMaxChars: update.AckMaxChars,
	}
	return config.Normalized().Validate()
}

func runtimeHeartbeatUpdate(input automationdomain.AutomationCommandInput, current automationdomain.HeartbeatStatus) automationdomain.HeartbeatUpdateInput {
	result := automationdomain.HeartbeatUpdateInput{
		Enabled: current.Enabled, EverySeconds: current.EverySeconds,
		TargetMode: current.TargetMode, AckMaxChars: current.AckMaxChars,
	}
	if input.Enabled != nil {
		result.Enabled = *input.Enabled
	}
	if input.EverySeconds > 0 {
		result.EverySeconds = input.EverySeconds
	}
	if strings.TrimSpace(input.TargetMode) != "" {
		result.TargetMode = strings.TrimSpace(input.TargetMode)
	}
	if input.AckMaxChars != nil {
		result.AckMaxChars = *input.AckMaxChars
	}
	return result
}

func validateRuntimeWakeInput(input automationdomain.AutomationCommandInput) error {
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = automationdomain.WakeModeNow
	}
	if mode != automationdomain.WakeModeNow && mode != automationdomain.WakeModeNextHeartbeat {
		return errors.New("wake mode 必须是 now 或 next-heartbeat")
	}
	return nil
}

func commandOptionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
