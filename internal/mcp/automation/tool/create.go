package tool

import (
	"context"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/builder"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/semantic"
)

const createDescription = "创建定时任务（== UI「新建任务」对话框的命令版本）。" +
	"必填：name / instruction / schedule。schedule.kind 支持 single|daily|interval|cron 四种：" +
	"single+run_at / daily+daily_time(+weekdays) / interval+interval_value+interval_unit / cron+expr(标准 5 段 cron 表达式，会被翻译回 daily 形态以保证 UI 可编辑；只支持 minute/hour 为单整数 + dom/month=* 的表达式)。" +
	"schedule.timezone 缺省按服务器默认时区（通常 Asia/Shanghai）。" +
	"execution_kind=script 时 instruction 会作为 workspace 脚本直接执行，不占用 Agent 会话，输出会进入运行产物。" +
	"可选：execution_mode(main|existing|temporary|dedicated) + reply_mode(none|execution|selected|agent|channel)。" +
	"需要投递到某个智能体时，用 reply_mode=agent + reply_agent_id；缺省投递到任务目标智能体的定时任务收件箱。" +
	"需要投递到 IM/外部通道时，用 reply_mode=channel + reply_channel/reply_to，或 reply_session_key（例如 agent:<agent_id>:fs:group:<chat_id> 或 agent:<agent_id>:weixin-personal:dm:<user_id>）；如果当前会话就是结构化外部 IM 群，可只传 reply_mode=channel，或只传匹配当前通道的 reply_channel，工具会默认投递回当前群；用户在当前群说“发到这个群/每天推送/每天发送/每天播报”时，工具也会默认 temporary+channel；但“不要推送/静默运行”不会触发该默认。" +
	"当前 DM/Room 里，用户说“每天搜索新闻发给我/告诉我/通知我”这类独立重任务且不依赖当前聊天历史时，可省略 execution_mode/reply_mode，工具会默认 temporary+selected 回投当前会话；如果任务要总结当前对话/聊天记录，必须显式选择执行会话。" +
	"短文本提醒类任务在当前会话中可缺省，工具会默认 existing+execution，让用户能看到提醒；execution_mode=existing 时若不传 selected_session_key 默认使用当前会话。" +
	"独立/临时执行但仍要让用户看到结果时，用 execution_mode=temporary + reply_mode=selected + selected_reply_session_key；只有用户明确要求后台静默时才用 reply_mode=none。" +
	"overlap_policy 可选 skip|allow，缺省 skip。" +
	"想让结果回到当前会话：显式 execution_mode=existing + reply_mode=execution。"

func create(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "create_scheduled_task",
		Description: createDescription,
		SearchHint:  searchHintCreateScheduledTask,
		InputSchema: createSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if args == nil {
				args = map[string]any{}
			}
			semantic.ReassembleFlatSchedule(args)
			semantic.ApplyDefaultTimezone(args, sctx)
			normalized := semantic.ApplyConversationDefaults(args, sctx)
			if err := semantic.RequireExplicitCreateFields(normalized, sctx); err != nil {
				return render.Error(err), nil
			}
			input, err := buildCreateInput(normalized, sctx)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.CreateTask(scopedToolContext(ctx, sctx), input)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

// buildCreateInput 把工具入参翻译成底层 CreateJobInput。
// 只接受 UI 对齐字段，不再允许直接传 session_target / delivery / source。
func buildCreateInput(args map[string]any, sctx contract.ServerContext) (automationdomain.CreateJobInput, error) {
	schedule, err := builder.Schedule(args["schedule"], sctx.DefaultTimezone)
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	agentID, err := resolveCreateAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	executionKind := automationdomain.NormalizeExecutionKind(argx.String(args, "execution_kind"))
	if executionKind == automationdomain.ExecutionKindScript {
		return automationdomain.CreateJobInput{
			Name:          argx.String(args, "name"),
			AgentID:       agentID,
			Schedule:      schedule,
			Instruction:   argx.String(args, "instruction"),
			ExecutionKind: automationdomain.ExecutionKindScript,
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
			Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
			Source:        semantic.Source(sctx, agentID),
			OverlapPolicy: argx.String(args, "overlap_policy"),
			Enabled:       argx.Bool(args, "enabled", true),
		}, nil
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))

	if err := semantic.ValidatePage(executionMode, replyMode); err != nil {
		return automationdomain.CreateJobInput{}, err
	}

	sessionTarget, err := semantic.SessionTarget(args, sctx, executionMode)
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	delivery, err := semantic.Delivery(args, sctx, agentID, executionMode, replyMode, sessionTarget)
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	return automationdomain.CreateJobInput{
		Name:          argx.String(args, "name"),
		AgentID:       agentID,
		Schedule:      schedule,
		Instruction:   argx.String(args, "instruction"),
		ExecutionKind: executionKind,
		SessionTarget: sessionTarget,
		Delivery:      delivery,
		Source:        semantic.Source(sctx, agentID),
		OverlapPolicy: argx.String(args, "overlap_policy"),
		Enabled:       argx.Bool(args, "enabled", true),
	}, nil
}
