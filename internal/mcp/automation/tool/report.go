package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func dailyReport(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_scheduled_task_daily_report",
		Description: "按日期聚合定时任务运行和投递状态。默认查询今天；普通 agent 只能查看自己名下任务，主智能体可传 agent_id 或 job_id。当前会话是飞书/IM 群且未指定 job_id/agent_id/query 时，默认聚合当前群相关任务；DM/Room/IM 群里的 query 会优先匹配当前会话任务，显式写“这里/当前会话/这个群/当前频道”会强制限定到当前会话；泛化的“当前会话/这个群定时任务发送情况”会聚合当前会话任务，带具体任务名时仍定位唯一任务。任务明细会返回 signals、suggested_tools、latest_execution_error、latest_delivery_error、recovery_run_id、execution_failed_run_ids、delivery_pending_run_ids、delivery_skipped_run_ids、manual_redelivery_run_ids 和 delivery_dead_letter_run_ids，便于直接解释发送情况、修正执行失败、恢复卡住 run 或补投递失败结果。",
		SearchHint:  searchHintDailyReport,
		InputSchema: dailyReportSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if payload, handled, err := currentConversationDailyReport(ctx, svc, sctx, args); handled {
				if err != nil {
					return render.Error(err), nil
				}
				return render.JSON(render.DecorateTimes(payload, payload.Timezone)), nil
			}
			scopedCtx := scopedToolContext(ctx, sctx)
			jobID := argx.String(args, "job_id")
			agentID := ""
			var err error
			if jobID != "" || argx.String(args, "query") != "" {
				scope, err := requireOwnedTaskHistoryScope(ctx, svc, sctx, args)
				if err != nil {
					return render.Error(err), nil
				}
				scopedCtx = scope.Context
				jobID = scope.JobID
			} else {
				agentID, err = resolveListAgentID(sctx, argx.String(args, "agent_id"))
				if err != nil {
					return render.Error(err), nil
				}
			}
			payload, err := svc.GetDailyReport(scopedCtx, protocol.CronDailyReportInput{
				Date:     argx.String(args, "date"),
				Timezone: argx.FirstNonEmpty(argx.String(args, "timezone"), sctx.DefaultTimezone),
				AgentID:  agentID,
				JobID:    jobID,
			})
			if err != nil {
				return render.Error(err), nil
			}
			if payload == nil {
				return render.Error(protocol.ErrJobNotFound), nil
			}
			return render.JSON(render.DecorateTimes(payload, payload.Timezone)), nil
		},
	}
}
