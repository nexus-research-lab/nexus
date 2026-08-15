// INPUT: 当前可信来源、目标 Agent 与 heartbeat 配置/唤醒参数。
// OUTPUT: owner/Agent 收窄后的 heartbeat 状态、CAS 更新结果或 wake 动作结果。
// POS: nexus_automation 的 heartbeat 对话控制入口。
package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func getHeartbeat(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_heartbeat",
		Description: "读取当前 Agent 的 heartbeat 持久配置、configuration_version 与运行态。只有主智能体自己的可信 Nexus 私有 DM 可指定 owner scope 内其他 Agent；Room、外部、automation、queue 与 internal 来源保持当前 Agent 只读。",
		SearchHint:  searchHintGetHeartbeat,
		InputSchema: heartbeatGetSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			status, err := svc.GetHeartbeatStatus(scopedToolContext(ctx, sctx), agentID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(status), nil
		},
	}
}

func updateHeartbeat(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "update_heartbeat",
		Description: "局部修改 heartbeat 配置。工具先读取 configuration_version，再用 CAS 写入并重读核验；并发旧版本会失败，不会覆盖较新的设置。普通 Agent 和 Room 内主智能体只能修改自身；主智能体仅在自己的可信 Nexus 私有 DM 可指定 owner scope 内 Agent。",
		SearchHint:  searchHintUpdateHeartbeat,
		InputSchema: heartbeatUpdateSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if err := requireTrustedInteractiveMutation(sctx); err != nil {
				return render.Error(err), nil
			}
			agentID, err := resolveCreateAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			scopedCtx := scopedToolContext(ctx, sctx)
			current, err := svc.GetHeartbeatStatus(scopedCtx, agentID)
			if err != nil {
				return render.Error(err), nil
			}
			input, err := heartbeatUpdateFromArgs(args, *current)
			if err != nil {
				return render.Error(err), nil
			}
			updated, err := svc.UpdateHeartbeatAtVersion(
				scopedCtx,
				agentID,
				current.ConfigurationVersion,
				input,
			)
			if err != nil {
				return render.Error(err), nil
			}
			verified, err := verifyHeartbeatUpdate(scopedCtx, svc, *current, *updated)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(verified), nil
		},
	}
}

func wakeHeartbeat(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "wake_heartbeat",
		Description: "登记一次 heartbeat 唤醒动作；它不会修改 heartbeat 配置或推进 configuration_version。普通 Agent 和 Room 内主智能体只能唤醒自身；主智能体仅在自己的可信 Nexus 私有 DM 可指定 owner scope 内 Agent。",
		SearchHint:  searchHintWakeHeartbeat,
		InputSchema: heartbeatWakeSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			if err := requireTrustedInteractiveMutation(sctx); err != nil {
				return render.Error(err), nil
			}
			agentID, err := resolveCreateAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			var text *string
			if _, ok := args["text"]; ok {
				value := strings.TrimSpace(argx.String(args, "text"))
				text = &value
			}
			result, err := svc.WakeHeartbeat(
				scopedToolContext(ctx, sctx),
				agentID,
				automationdomain.HeartbeatWakeInput{
					Mode: argx.String(args, "mode"),
					Text: text,
				},
			)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(result), nil
		},
	}
}

func heartbeatUpdateFromArgs(
	args map[string]any,
	current automationdomain.HeartbeatStatus,
) (automationdomain.HeartbeatUpdateInput, error) {
	input := automationdomain.HeartbeatUpdateInput{
		Enabled:      current.Enabled,
		EverySeconds: current.EverySeconds,
		TargetMode:   current.TargetMode,
		AckMaxChars:  current.AckMaxChars,
	}
	changed := false
	if value, ok := args["enabled"]; ok {
		input.Enabled = argx.ParseBool(value)
		changed = true
	}
	if value, ok := args["every_seconds"]; ok {
		input.EverySeconds = argx.Int(value)
		changed = true
	}
	if value, ok := args["target_mode"]; ok {
		input.TargetMode = strings.TrimSpace(argx.StringOf(value))
		changed = true
	}
	if value, ok := args["ack_max_chars"]; ok {
		input.AckMaxChars = argx.Int(value)
		changed = true
	}
	if !changed {
		return automationdomain.HeartbeatUpdateInput{}, errors.New("automation_update operation=set_heartbeat requires at least one configuration field")
	}
	return input, nil
}

func verifyHeartbeatUpdate(
	ctx context.Context,
	svc contract.Service,
	previous automationdomain.HeartbeatStatus,
	expected automationdomain.HeartbeatStatus,
) (*automationdomain.HeartbeatStatus, error) {
	if expected.ConfigurationVersion != previous.ConfigurationVersion+1 {
		return nil, fmt.Errorf(
			"heartbeat version did not advance exactly once: previous=%d current=%d",
			previous.ConfigurationVersion,
			expected.ConfigurationVersion,
		)
	}
	persisted, err := svc.GetHeartbeatStatus(ctx, strings.TrimSpace(expected.AgentID))
	if err != nil {
		return nil, err
	}
	if persisted.ConfigurationVersion != expected.ConfigurationVersion ||
		persisted.Enabled != expected.Enabled ||
		persisted.EverySeconds != expected.EverySeconds ||
		strings.TrimSpace(persisted.TargetMode) != strings.TrimSpace(expected.TargetMode) ||
		persisted.AckMaxChars != expected.AckMaxChars {
		return nil, errors.New("heartbeat update verification failed: persisted configuration differs")
	}
	return persisted, nil
}
