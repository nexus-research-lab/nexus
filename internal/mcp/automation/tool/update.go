package tool

import (
	"context"
	"errors"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/semantic"
)

const updateDescription = "按 job_id 或 query 局部更新定时任务字段。query 只在当前权限范围内唯一命中当前未删除任务时才会执行，多候选会要求用户确认。字段语义与 UI「编辑任务」对话框一致：" +
	"name / instruction / execution_kind / permission_mode / schedule / context_mode / deliver_result / " +
	"instruction_append / overlap_policy / expires_at / clear_expires_at / enabled。只有提供的字段会被更新。" +
	"启用或停用直接设置 enabled；cancel_active_run=true 会隐含 enabled=false，并中断当前 active run。" +
	"除了 job_id/query 之外必须至少提供一个要修改的字段。" +
	"用户说“再加一条要求/补充任务细节”时优先用 instruction_append；只有明确要重写任务内容时才用 instruction。" +
	"投递权限：普通 Agent 只能改为自身真实会话，Room 成员只能额外改回当前 Room，外部通道只能使用当前明确授权的同一会话（含账号与 thread）；只有主智能体自己的可信 Nexus 私有 DM 可在 owner scope 内指定其他真实会话或任意已配置通道目标。新的可信会话 grant 会随修改持久化，实际投递前还会重读最新任务并复核当前权限。" +
	"context_mode 只决定是否复用当前上下文，deliver_result 只决定是否把结果送回当前可信会话。外部 IM 的 channel/account/target/thread/session 由 Nexus 自动绑定，模型不得填写或猜测路由字段。"

func updateDescriptionForContext(sctx contract.ServerContext) string {
	channel, chatType, ok := currentExternalIMSummary(sctx)
	if !ok {
		return updateDescription
	}
	return "当前可信调用来自 " + channel + " " + chatType +
		"。把任务结果改回这里时只传 deliver_result=true；Nexus 会自动注入真实路由，绝不要填写或猜测 channel/account/chat/thread/session。" +
		updateDescription
}

func update(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "update_scheduled_task",
		Description: updateDescriptionForContext(sctx),
		SearchHint:  searchHintUpdateScheduledTask,
		InputSchema: updateSchema(sctx),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if err := requireTrustedInteractiveMutation(sctx); err != nil {
				return render.Error(err), nil
			}
			if args == nil {
				args = map[string]any{}
			}
			if err := normalizeUpdateCancellation(args); err != nil {
				return render.Error(err), nil
			}
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			if err = requireAgentExecutionTaskMutation(scope.Job); err != nil {
				return render.Error(err), nil
			}
			semantic.ReassembleFlatSchedule(args)
			semantic.ApplyDefaultTimezone(args, sctx)
			semantic.ApplyIntentDefaults(args, sctx)
			semantic.ApplyDeliveryFieldDefaults(args)
			semantic.ApplySelectedReplyCurrentDefault(args, sctx)
			semantic.BindCurrentExternalChannelReply(args, sctx)
			input, err := buildUpdateInput(args, sctx, scope.Job)
			if err != nil {
				return render.Error(err), nil
			}
			if input.Delivery != nil {
				if err = requireConversationDeliveryScope(sctx, scope.Job.AgentID, *input.Delivery); err != nil {
					return render.Error(err), nil
				}
				// Delivery grant 必须随最近一次可信对话修改持久化。后台投递据此
				// 重新验证 owner-main 或 self/current-context 边界，不能只信创建时快照。
				source := semantic.Source(sctx, scope.Job.AgentID)
				input.Source = &source
			}
			job, err := svc.UpdateTaskAtVersion(
				scope.Context,
				scope.JobID,
				scope.Job.ConfigurationVersion,
				input,
			)
			if err != nil {
				return render.Error(err), nil
			}
			if argx.Bool(args, "cancel_active_run", false) {
				runID := firstNonEmptyString(
					argx.String(args, "run_id"),
					strings.TrimSpace(job.RunningRunID),
					strings.TrimSpace(scope.Job.RunningRunID),
				)
				if runID != "" {
					job, err = svc.RecoverTaskRunningRun(scope.Context, scope.JobID, runID)
					if err != nil {
						return render.Error(err), nil
					}
				}
			}
			job, err = verifyScheduledTaskVersionedWrite(
				scope.Context,
				svc,
				*job,
				scope.Job.ConfigurationVersion,
			)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

func normalizeUpdateCancellation(args map[string]any) error {
	cancelActiveRun := argx.Bool(args, "cancel_active_run", false)
	if !cancelActiveRun {
		if strings.TrimSpace(argx.String(args, "run_id")) != "" {
			return errors.New("run_id requires cancel_active_run=true")
		}
		return nil
	}
	if raw, ok := args["enabled"]; ok && argx.ParseBool(raw) {
		return errors.New("cancel_active_run cannot be combined with enabled=true")
	}
	args["enabled"] = false
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// buildUpdateInput 把工具入参映射成底层 UpdateJobInput（仅设置出现的字段）。
// 只接受 UI 对齐字段，不再允许直接传 session_target / delivery / source。
func buildUpdateInput(args map[string]any, sctx contract.ServerContext, currentJob automationdomain.ScheduledTask) (automationdomain.UpdateJobInput, error) {
	if err := requireAgentExecutionTaskMutation(currentJob); err != nil {
		return automationdomain.UpdateJobInput{}, err
	}
	builder := scheduledTaskUpdateInputBuilder{args: args, server: sctx, currentJob: currentJob}
	return builder.build()
}

type scheduledTaskUpdateInputBuilder struct {
	args       map[string]any
	server     contract.ServerContext
	currentJob automationdomain.ScheduledTask
	input      automationdomain.UpdateJobInput
}

func (b *scheduledTaskUpdateInputBuilder) build() (automationdomain.UpdateJobInput, error) {
	stages := []func() error{
		b.applyBasicFields,
		b.applyInstruction,
		b.applyExpiration,
		b.applySchedule,
		b.applyRouting,
	}
	for _, stage := range stages {
		if err := stage(); err != nil {
			return automationdomain.UpdateJobInput{}, err
		}
	}
	if !hasUpdateFields(b.input) {
		return automationdomain.UpdateJobInput{}, errors.New("update_scheduled_task requires at least one field to update besides job_id")
	}
	return b.input, nil
}

func (b *scheduledTaskUpdateInputBuilder) applyBasicFields() error {
	if name, ok := b.args["name"]; ok {
		s := strings.TrimSpace(argx.StringOf(name))
		b.input.Name = &s
	}
	if executionKind, ok := b.args["execution_kind"]; ok {
		s := automationdomain.NormalizeExecutionKind(argx.StringOf(executionKind))
		if s == automationdomain.ExecutionKindScript {
			return errors.New("execution_kind=script is human-control-plane only and cannot be selected through an Agent conversation")
		}
		b.input.ExecutionKind = &s
	}
	if permissionMode, ok := b.args["permission_mode"]; ok {
		s := automationdomain.NormalizePermissionMode(argx.StringOf(permissionMode))
		b.input.PermissionMode = &s
	}
	if enabled, ok := b.args["enabled"]; ok {
		value := argx.ParseBool(enabled)
		b.input.Enabled = &value
	}
	if overlapPolicy, ok := b.args["overlap_policy"]; ok {
		s := strings.TrimSpace(argx.StringOf(overlapPolicy))
		b.input.OverlapPolicy = &s
	}
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applyInstruction() error {
	instruction, err := updateInstruction(b.args, b.currentJob.Instruction)
	if err != nil {
		return err
	}
	b.input.Instruction = instruction
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applyExpiration() error {
	expiresAt, err := parseExpiresAt(b.args)
	if err != nil {
		return err
	}
	b.input.ExpiresAt = expiresAt
	if clearExpiresAt, ok := b.args["clear_expires_at"]; ok {
		b.input.ClearExpiresAt = argx.ParseBool(clearExpiresAt)
	}
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applySchedule() error {
	raw, ok := b.args["schedule"]
	if !ok {
		return nil
	}
	schedule, err := builder.Schedule(raw, b.server.DefaultTimezone)
	if err != nil {
		return err
	}
	b.input.Schedule = &schedule
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applyRouting() error {
	executionMode := strings.TrimSpace(argx.String(b.args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(b.args, "reply_mode"))
	if executionMode != "" {
		return b.applyExecutionRoute(executionMode, replyMode)
	}
	if replyMode != "" {
		return b.applyDeliveryOnly(replyMode)
	}
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applyExecutionRoute(executionMode string, replyMode string) error {
	if err := semantic.ValidatePage(executionMode, replyMode); err != nil {
		return err
	}
	target, err := semantic.SessionTarget(b.args, b.server, executionMode)
	if err != nil {
		return err
	}
	b.input.SessionTarget = &target
	if replyMode == "" {
		return nil
	}
	delivery, err := semantic.Delivery(b.args, b.server, executionMode, replyMode, target)
	if err != nil {
		return err
	}
	b.input.Delivery = &delivery
	return nil
}

func (b *scheduledTaskUpdateInputBuilder) applyDeliveryOnly(replyMode string) error {
	if replyMode == "execution" {
		return errors.New("reply_mode=execution update requires execution_mode in the same call so the execution session can be resolved safely")
	}
	delivery, err := semantic.Delivery(
		b.args,
		b.server,
		"",
		replyMode,
		automationdomain.SessionTarget{},
	)
	if err != nil {
		return err
	}
	b.input.Delivery = &delivery
	return nil
}

func updateInstruction(args map[string]any, currentInstruction string) (*string, error) {
	rawInstruction, hasInstruction := args["instruction"]
	rawAppend, hasAppend := args["instruction_append"]
	if hasInstruction && hasAppend {
		return nil, errors.New("instruction and instruction_append cannot be used together")
	}
	if hasInstruction {
		s := strings.TrimSpace(argx.StringOf(rawInstruction))
		return &s, nil
	}
	if !hasAppend {
		return nil, nil
	}
	appendix := strings.TrimSpace(argx.StringOf(rawAppend))
	if appendix == "" {
		return nil, errors.New("instruction_append cannot be empty")
	}
	updated := appendInstruction(currentInstruction, appendix)
	return &updated, nil
}

func appendInstruction(currentInstruction, appendix string) string {
	current := strings.TrimSpace(currentInstruction)
	if current == "" {
		return appendix
	}
	return current + "\n\n" + appendix
}

func hasUpdateFields(input automationdomain.UpdateJobInput) bool {
	return input.Name != nil ||
		input.Schedule != nil ||
		input.Instruction != nil ||
		input.ExecutionKind != nil ||
		input.PermissionMode != nil ||
		input.SessionTarget != nil ||
		input.Delivery != nil ||
		input.Source != nil ||
		input.OverlapPolicy != nil ||
		input.ExpiresAt != nil ||
		input.ClearExpiresAt ||
		input.Enabled != nil
}
