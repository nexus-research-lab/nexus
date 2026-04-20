package tool

import (
	"context"
	"errors"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

func update(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "update_scheduled_task",
		Description: "按 job_id 局部更新定时任务字段。",
		InputSchema: updateSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
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
		schedule, err := builder.Schedule(raw)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.Schedule = &schedule
	}
	if raw, ok := args["session_target"]; ok {
		target, err := builder.SessionTarget(raw, sctx.CurrentSessionKey)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.SessionTarget = &target
	}
	if raw, ok := args["delivery"]; ok {
		delivery, err := builder.Delivery(raw, sctx.CurrentSessionKey)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.Delivery = &delivery
	}
	if raw, ok := args["source"]; ok {
		source, err := builder.Source(raw)
		if err != nil {
			return automationsvc.UpdateJobInput{}, err
		}
		input.Source = &source
	}
	return input, nil
}
