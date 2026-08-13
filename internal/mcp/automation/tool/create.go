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

const createDescription = "创建持久化定时任务（== UI「新建任务」对话框的命令版本）。用户可见的提醒、延迟动作和重复任务必须使用本工具，不要用临时 wakeup 或会话状态代替。" +
	"必填：name / instruction / schedule。schedule.kind 支持 single|daily|interval|cron 四种：" +
	"single+run_at / daily+daily_time(+weekdays) / interval+interval_value+interval_unit / cron+expr(标准 5 段 cron 表达式，会被翻译回 daily 形态以保证 UI 可编辑；只支持 minute/hour 为单整数 + dom/month=* 的表达式)。" +
	"schedule.timezone 缺省按服务器默认时区（通常 Asia/Shanghai）。" +
	"对话入口只允许 execution_kind=agent；宿主脚本任务只能由人类控制面创建。" +
	"可选：context_mode(isolated|current) + deliver_result(boolean) + permission_mode(default|plan|acceptEdits|bypassPermissions|dontAsk)。" +
	"context_mode 默认 isolated；只有任务明确需要读取当前聊天记录时才用 current。deliver_result 在当前会话中默认 true，无会话时默认 false。" +
	"省略 permission_mode 时，会在创建瞬间复制当前 Session 的有效模式（无覆盖则复制 Agent 模式）以及 Agent 工具 allow/deny，之后任务与 Agent 配置相互独立；显式 permission_mode 会覆盖复制出的模式。bypassPermissions 会跳过 SDK 权限检查，只有用户明确要求时才选择。SDK 发出的额外任务权限请求仍由 Nexus 持久审批。" +
	"投递权限：普通 Agent 只能回到自身真实会话，Room 成员只能额外回到当前 Room，外部通道只能回到当前明确授权的同一会话（含账号与 thread）；只有主智能体自己的可信 Nexus 私有 DM 可在 owner scope 内指定其他真实会话或任意已配置通道目标。任务实际投递前会按最新配置和当前主智能体/Room 成员身份再次校验。" +
	"外部 IM 中只需表达 deliver_result；Nexus 根据可信入站上下文自动绑定 channel/account/target/thread/session，模型不得猜测或填写这些路由字段。" +
	"任务到点后无人值守执行，instruction 必须自包含，所需工具必须预先授权，不能依赖 AskUserQuestion 补充信息。" +
	"overlap_policy 可选 skip|allow，缺省 skip。" +
	"expires_at 可选 RFC3339 时间；到期后只停止后续触发，不中断正在执行的任务。"

func createDescriptionForContext(sctx contract.ServerContext) string {
	channel, chatType, ok := currentExternalIMSummary(sctx)
	if !ok {
		return createDescription
	}
	return "当前可信调用来自 " + channel + " " + chatType +
		"。结果是否回到这里仅由 deliver_result 控制；Nexus 会自动注入真实路由，绝不要填写或猜测 channel/account/chat/thread/session。" +
		createDescription
}

func create(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "create_scheduled_task",
		Description: createDescriptionForContext(sctx),
		SearchHint:  searchHintCreateScheduledTask,
		InputSchema: createSchema(sctx),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if err := requireTrustedInteractiveMutation(sctx); err != nil {
				return render.Error(err), nil
			}
			if args == nil {
				args = map[string]any{}
			}
			requestID := strings.TrimSpace(argx.String(args, "request_id"))
			if requestID == "" {
				return render.Error(errors.New("request_id is required for idempotent scheduled task creation")), nil
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
			if err = requireConversationDeliveryScope(sctx, input.AgentID, input.Delivery); err != nil {
				return render.Error(err), nil
			}
			input.RequestID = requestID
			job, err := svc.CreateTask(scopedToolContext(ctx, sctx), input)
			if err != nil {
				return render.Error(err), nil
			}
			verified, err := verifyScheduledTaskPresent(ctx, svc, sctx, *job)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(verified, verified.Schedule.Timezone)), nil
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
	expiresAt, err := parseExpiresAt(args)
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	executionKind := automationdomain.NormalizeExecutionKind(argx.String(args, "execution_kind"))
	if executionKind == automationdomain.ExecutionKindScript {
		return automationdomain.CreateJobInput{}, errors.New("execution_kind=script is human-control-plane only and cannot be created through an Agent conversation")
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
	delivery, err := semantic.Delivery(args, sctx, executionMode, replyMode, sessionTarget)
	if err != nil {
		return automationdomain.CreateJobInput{}, err
	}
	return automationdomain.CreateJobInput{
		RequestID:      strings.TrimSpace(argx.String(args, "request_id")),
		Name:           argx.String(args, "name"),
		AgentID:        agentID,
		Schedule:       schedule,
		Instruction:    argx.String(args, "instruction"),
		ExecutionKind:  executionKind,
		PermissionMode: strings.TrimSpace(argx.String(args, "permission_mode")),
		SessionTarget:  sessionTarget,
		Delivery:       delivery,
		Source:         semantic.Source(sctx, agentID),
		OverlapPolicy:  argx.String(args, "overlap_policy"),
		ExpiresAt:      expiresAt,
		Enabled:        argx.Bool(args, "enabled", true),
	}, nil
}
