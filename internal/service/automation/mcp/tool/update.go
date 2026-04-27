package tool

import (
	"context"
	"errors"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/render"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/semantic"
)

const updateDescription = "按 job_id 局部更新定时任务字段。字段语义与 UI「编辑任务」对话框一致：" +
	"name / instruction / schedule / execution_mode / reply_mode / selected_session_key / " +
	"named_session_key / selected_reply_session_key / enabled。只有提供的字段会被更新。"

func update(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "update_scheduled_task",
		Description: updateDescription,
		InputSchema: updateSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			if args == nil {
				args = map[string]any{}
			}
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			semantic.ReassembleFlatSchedule(args)
			semantic.ApplyDefaultTimezone(args, sctx)
			input, err := buildUpdateInput(args, sctx)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.UpdateTask(ctx, jobID, input)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

// buildUpdateInput 把工具入参映射成底层 UpdateJobInput（仅设置出现的字段）。
// 只接受 UI 对齐字段，不再允许直接传 session_target / delivery / source。
func buildUpdateInput(args map[string]any, sctx contract.ServerContext) (automationsvc.UpdateJobInput, error) {
	input := automationsvc.UpdateJobInput{}
	if name, ok := args["name"]; ok {
		s := strings.TrimSpace(argx.StringOf(name))
		input.Name = &s
	}
	if instr, ok := args["instruction"]; ok {
		s := strings.TrimSpace(argx.StringOf(instr))
		input.Instruction = &s
	}
	if enabled, ok := args["enabled"]; ok {
		b := argx.ParseBool(enabled)
		input.Enabled = &b
	}
	if raw, ok := args["schedule"]; ok {
		schedule, err := builder.Schedule(raw, sctx.DefaultTimezone)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.Schedule = &schedule
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))
	if executionMode != "" {
		if err := semantic.ValidatePage(executionMode, replyMode); err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		target, err := semantic.SessionTarget(args, sctx, executionMode)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.SessionTarget = &target
		if replyMode != "" {
			delivery, err := semantic.Delivery(args, sctx, executionMode, replyMode, target)
			if err != nil {
				return automationsvc.UpdateJobInput{}, err
			}
			input.Delivery = &delivery
		}
	} else if replyMode != "" {
		return automationsvc.UpdateJobInput{}, errors.New("reply_mode update requires execution_mode in the same call so the routing context stays consistent")
	}
	return input, nil
}
