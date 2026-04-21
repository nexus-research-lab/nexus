package tool

import (
	"context"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/semantic"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

const createDescription = "创建定时任务。本工具 == UI「新建任务」对话框的命令版本，字段一对一映射：" +
	"name / instruction / schedule(kind=single|daily|interval) / execution_mode / reply_mode / " +
	"selected_session_key(existing) / named_session_key(dedicated) / selected_reply_session_key(selected)。" +
	"若用户没有明确 execution_mode / reply_mode / schedule.timezone，必须先用 AskUserQuestion 与用户确认，" +
	"禁止默认套值。只有短文本、一次一条的提醒/播报类任务才允许默认按 temporary + none 创建。"

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

// buildCreateInput 把工具入参翻译成底层 CreateJobInput。
// 只接受 UI 对齐字段，不再允许直接传 session_target / delivery / source。
func buildCreateInput(args map[string]any, sctx contract.ServerContext) (automationsvc.CreateJobInput, error) {
	schedule, err := builder.Schedule(args["schedule"])
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	agentID, err := resolveCreateAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))

	if err := semantic.ValidatePage(executionMode, replyMode); err != nil {
		return automationsvc.CreateJobInput{}, err
	}

	sessionTarget, err := semantic.SessionTarget(args, sctx, executionMode)
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	delivery, err := semantic.Delivery(args, sctx, executionMode, replyMode, sessionTarget)
	if err != nil {
		return automationsvc.CreateJobInput{}, err
	}
	return automationsvc.CreateJobInput{
		Name:          argx.String(args, "name"),
		AgentID:       agentID,
		Schedule:      schedule,
		Instruction:   argx.String(args, "instruction"),
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        semantic.Source(sctx, agentID),
		Enabled:       argx.Bool(args, "enabled", true),
	}, nil
}
