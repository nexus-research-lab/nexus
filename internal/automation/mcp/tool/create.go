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

const createDescription = "创建定时任务（== UI「新建任务」对话框的命令版本）。" +
	"必填：name / instruction / schedule。schedule.kind 支持 single|daily|interval|cron 四种：" +
	"single+run_at / daily+daily_time(+weekdays) / interval+interval_value+interval_unit / cron+expr(标准 5 段 cron 表达式，会被翻译回 daily 形态以保证 UI 可编辑；只支持 minute/hour 为单整数 + dom/month=* 的表达式)。" +
	"schedule.timezone 缺省按服务器默认时区（通常 Asia/Shanghai）。" +
	"可选：execution_mode(main|existing|temporary|dedicated) + reply_mode(none|execution|selected)，缺省走 temporary+none——" +
	"短文本提醒类任务直接发即可；execution_mode=existing 时若不传 selected_session_key 默认使用当前会话。" +
	"想让结果回到当前会话：显式 execution_mode=existing + reply_mode=execution。"

func create(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "create_scheduled_task",
		Description: createDescription,
		InputSchema: createSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			if args == nil {
				args = map[string]any{}
			}
			semantic.ReassembleFlatSchedule(args)
			semantic.ApplyDefaultTimezone(args, sctx)
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
	schedule, err := builder.Schedule(args["schedule"], sctx.DefaultTimezone)
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
