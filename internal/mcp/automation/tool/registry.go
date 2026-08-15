// INPUT: automation 服务与服务端签发的 runtime 来源上下文。
// OUTPUT: 交互 Session 的查询/变更双工具面，或独立后台 profile 的只读查询工具。
// POS: nexus_automation 的 capability 注册边界。
package tool

import (
	"context"
	"errors"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

const (
	automationQueryToolName  = "automation_query"
	automationUpdateToolName = "automation_update"
)

// BuildAll 汇集模型可见工具，供 mcp.NewServer 注册。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	queryTool := buildAutomationQueryTool(svc, sctx)
	if !sctx.StableInteractiveSurface && !isTrustedInteractiveSource(sctx) {
		return []sdktool.Tool{queryTool}
	}
	return []sdktool.Tool{
		queryTool,
		buildAutomationUpdateTool(svc, sctx),
	}
}

func buildAutomationQueryTool(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	findTool := find(svc, sctx)
	inspectTool := inspectTask(svc, sctx)
	reportTool := report(svc, sctx)
	heartbeatTool := getHeartbeat(svc, sctx)
	return sdktool.Tool{
		Name:        automationQueryToolName,
		Description: "查询 Nexus 定时任务、运行记录、审计、日报或 heartbeat 状态。operation 必须是 list|get|runs|events|report|heartbeat。",
		SearchHint:  "automation scheduled task heartbeat query list status runs events report 查询 定时任务 心跳",
		InputSchema: automationQuerySchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			routed := automationOperationArguments(args)
			switch automationOperation(args) {
			case "list":
				return findTool.Handler(ctx, routed)
			case "get":
				routed["view"] = "status"
				return inspectTool.Handler(ctx, routed)
			case "runs":
				routed["view"] = "runs"
				return inspectTool.Handler(ctx, routed)
			case "events":
				routed["view"] = "events"
				return inspectTool.Handler(ctx, routed)
			case "report":
				return reportTool.Handler(ctx, routed)
			case "heartbeat":
				return heartbeatTool.Handler(ctx, routed)
			default:
				return render.Error(errors.New("automation_query operation must be one of list, get, runs, events, report, heartbeat")), nil
			}
		},
	}
}

func buildAutomationUpdateTool(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	createTool := create(svc, sctx)
	updateTool := update(svc, sctx)
	deleteTool := del(svc, sctx)
	runTool := runNow(svc, sctx)
	repairTool := repair(svc, sctx)
	heartbeatUpdateTool := updateHeartbeat(svc, sctx)
	heartbeatWakeTool := wakeHeartbeat(svc, sctx)
	description := "创建、修改、删除、立即运行或补投递 Nexus 定时任务，也可配置或唤醒 heartbeat。operation 必须是 create|update|delete|run|retry_delivery|set_heartbeat|wake。"
	if channel, chatType, ok := currentExternalIMSummary(sctx); ok {
		description = "当前可信调用来自 " + channel + " " + chatType + "；结果是否回到这里仅由 deliver_result 控制，Nexus 自动绑定真实路由。" + description
	}
	return sdktool.Tool{
		Name:        automationUpdateToolName,
		Description: description,
		SearchHint:  "automation scheduled task heartbeat create update delete run retry delivery wake 创建 修改 删除 执行 补发 心跳 唤醒",
		InputSchema: automationUpdateSchema(sctx),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			routed := automationOperationArguments(args)
			switch automationOperation(args) {
			case "create":
				return createTool.Handler(ctx, routed)
			case "update":
				return updateTool.Handler(ctx, routed)
			case "delete":
				return deleteTool.Handler(ctx, routed)
			case "run":
				return runTool.Handler(ctx, routed)
			case "retry_delivery":
				routed["action"] = "retry_delivery"
				return repairTool.Handler(ctx, routed)
			case "set_heartbeat":
				return heartbeatUpdateTool.Handler(ctx, routed)
			case "wake":
				return heartbeatWakeTool.Handler(ctx, routed)
			default:
				return render.Error(errors.New("automation_update operation must be one of create, update, delete, run, retry_delivery, set_heartbeat, wake")), nil
			}
		},
	}
}

func automationOperation(args map[string]any) string {
	value, _ := args["operation"].(string)
	return strings.ToLower(strings.TrimSpace(value))
}

func automationOperationArguments(args map[string]any) map[string]any {
	routed := make(map[string]any, len(args))
	for key, value := range args {
		if key != "operation" {
			routed[key] = value
		}
	}
	return routed
}

func isTrustedInteractiveSource(sctx contract.ServerContext) bool {
	switch strings.TrimSpace(sctx.SourceContextType) {
	case "agent", "agent_paired", "room":
		return true
	default:
		return false
	}
}
