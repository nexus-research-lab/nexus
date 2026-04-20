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
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/semantic"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

const createDescription = "创建新的定时任务。若用户没有明确提供执行方式、结果回传方式或时区，" +
	"必须先通过 AskUserQuestion 与用户确认，禁止直接套默认参数。" +
	"只有非常简单、短文本、一次一条的提醒/播报类任务，才允许默认按 temporary + none 创建。" +
	"优先使用和页面一致的 execution_mode / reply_mode 语义，不要直接组合底层 session_target / delivery 细节。"

func create(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "create_scheduled_task",
		Description: createDescription,
		InputSchema: createSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			normalized := semantic.ApplySimpleDefaults(args)
			if err := semantic.RequireExplicitCreateFields(normalized); err != nil {
				return render.Error(err), nil
			}
			input, err := buildCreateInput(normalized, sctx)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.CreateTask(ctx, input)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

// buildCreateInput 把工具入参（已经过守卫与默认值处理）翻译成底层 CreateJobInput。
func buildCreateInput(args map[string]any, sctx contract.ServerContext) (automationsvc.CreateJobInput, error) {
	schedule, err := builder.Schedule(args["schedule"])
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	agentID := argx.FirstNonEmpty(argx.String(args, "agent_id"), sctx.CurrentAgentID)
	if agentID == "" {
		return automationsvc.CreateJobInput{}, errors.New("agent_id is required")
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))

	sessionTarget, err := semantic.SessionTarget(args, sctx, executionMode)
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	delivery, err := semantic.Delivery(args, sctx, executionMode, replyMode, sessionTarget)
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	if err := semantic.ValidatePage(sessionTarget, delivery, executionMode, replyMode); err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	return automationsvc.CreateJobInput{
		Name:          argx.String(args, "name"),
		AgentID:       agentID,
		Schedule:      schedule,
		Instruction:   argx.String(args, "instruction"),
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        semantic.Source(args["source"], sctx, agentID),
		Enabled:       argx.Bool(args, "enabled", true),
	}, nil
}
