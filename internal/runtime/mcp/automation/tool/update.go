package tool

import (
	"context"
	"errors"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/semantic"
)

const updateDescription = "按 job_id 或 query 局部更新定时任务字段。query 只在当前权限范围内唯一命中当前未删除任务时才会执行，多候选会要求用户确认。字段语义与 UI「编辑任务」对话框一致：" +
	"name / instruction / execution_kind / schedule / execution_mode / reply_mode / selected_session_key / " +
	"instruction_append / named_session_key / selected_reply_session_key / reply_agent_id / reply_session_key / reply_channel / reply_to / reply_account_id / reply_thread_id / overlap_policy / enabled。只有提供的字段会被更新。" +
	"除了 job_id/query 之外必须至少提供一个要修改的字段。" +
	"用户说“再加一条要求/补充任务细节”时优先用 instruction_append；只有明确要重写任务内容时才用 instruction。" +
	"只改投递目标时不需要同时传 execution_mode；传 reply_channel/reply_to/reply_session_key 会默认按 reply_mode=channel 处理，当前会话是结构化外部 IM 群且 reply_channel 与当前通道一致时可省略 reply_to；" +
	"当前内部 DM/Room 会话里传 reply_mode=selected 可省略 selected_reply_session_key；外部 IM 群改发当前群请用 reply_mode=channel；传 reply_agent_id 会默认按 reply_mode=agent 处理；reply_mode=agent 且不传 reply_agent_id 时默认投递到该任务所属 Agent 的定时任务收件箱。"

func update(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "update_scheduled_task",
		Description: updateDescription,
		SearchHint:  searchHintUpdateScheduledTask,
		InputSchema: updateSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			if args == nil {
				args = map[string]any{}
			}
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			semantic.ReassembleFlatSchedule(args)
			semantic.ApplyDefaultTimezone(args, sctx)
			semantic.ApplyDeliveryFieldDefaults(args)
			semantic.ApplySelectedReplyCurrentDefault(args, sctx)
			input, err := buildUpdateInput(args, sctx, scope.Job)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.UpdateTask(scope.Context, scope.JobID, input)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

// buildUpdateInput 把工具入参映射成底层 UpdateJobInput（仅设置出现的字段）。
// 只接受 UI 对齐字段，不再允许直接传 session_target / delivery / source。
func buildUpdateInput(args map[string]any, sctx contract.ServerContext, currentJob protocol.CronJob) (protocol.UpdateJobInput, error) {
	input := protocol.UpdateJobInput{}
	if name, ok := args["name"]; ok {
		s := strings.TrimSpace(argx.StringOf(name))
		input.Name = &s
	}
	instruction, err := updateInstruction(args, currentJob.Instruction)
	if err != nil {
		return protocol.UpdateJobInput{}, err
	}
	if instruction != nil {
		input.Instruction = instruction
	}
	if executionKind, ok := args["execution_kind"]; ok {
		s := protocol.NormalizeExecutionKind(argx.StringOf(executionKind))
		input.ExecutionKind = &s
	}
	if enabled, ok := args["enabled"]; ok {
		b := argx.ParseBool(enabled)
		input.Enabled = &b
	}
	if overlapPolicy, ok := args["overlap_policy"]; ok {
		s := strings.TrimSpace(argx.StringOf(overlapPolicy))
		input.OverlapPolicy = &s
	}
	if raw, ok := args["schedule"]; ok {
		schedule, err := builder.Schedule(raw, sctx.DefaultTimezone)
		if err != nil {
			return protocol.UpdateJobInput{}, err
		}
		input.Schedule = &schedule
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))
	if executionMode != "" {
		if err := semantic.ValidatePage(executionMode, replyMode); err != nil {
			return protocol.UpdateJobInput{}, err
		}
		target, err := semantic.SessionTarget(args, sctx, executionMode)
		if err != nil {
			return protocol.UpdateJobInput{}, err
		}
		input.SessionTarget = &target
		if replyMode != "" {
			delivery, err := semantic.Delivery(args, sctx, currentJob.AgentID, executionMode, replyMode, target)
			if err != nil {
				return protocol.UpdateJobInput{}, err
			}
			input.Delivery = &delivery
		}
	} else if replyMode != "" {
		if replyMode == "execution" {
			return protocol.UpdateJobInput{}, errors.New("reply_mode=execution update requires execution_mode in the same call so the execution session can be resolved safely")
		}
		delivery, err := semantic.Delivery(args, sctx, currentJob.AgentID, executionMode, replyMode, protocol.SessionTarget{})
		if err != nil {
			return protocol.UpdateJobInput{}, err
		}
		input.Delivery = &delivery
	}
	if !hasUpdateFields(input) {
		return protocol.UpdateJobInput{}, errors.New("update_scheduled_task requires at least one field to update besides job_id")
	}
	return input, nil
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

func hasUpdateFields(input protocol.UpdateJobInput) bool {
	return input.Name != nil ||
		input.Schedule != nil ||
		input.Instruction != nil ||
		input.ExecutionKind != nil ||
		input.SessionTarget != nil ||
		input.Delivery != nil ||
		input.Source != nil ||
		input.OverlapPolicy != nil ||
		input.Enabled != nil
}
