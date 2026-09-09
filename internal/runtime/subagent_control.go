// INPUT: 固定 runtime session/round 的宿主 MCP 回连操作。
// OUTPUT: 仅向当前存活的父 runtime 转交 capability 协商后的子智能体控制。
// POS: MCP 与 bridge 的窄适配，复用原生准入 hook 与 task 生命周期。
package runtime

import (
	"context"
	"errors"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// SubagentControl 的 toolUseID 只能来自 SDK MCP metadata，不能来自模型 input。
type SubagentControl func(context.Context, string, string, map[string]any) (map[string]any, error)

// BindSubagentControl 固定当前物理 round；旧回调不能借用后续 round 的 client。
func (m *Manager) BindSubagentControl(sessionKey, roundID string) SubagentControl {
	return func(ctx context.Context, toolUseID, operation string, input map[string]any) (map[string]any, error) {
		m.mu.Lock()
		state := m.sessions[sessionKey]
		var client Client
		if state != nil {
			if _, ok := state.SubagentHooks[roundID]; ok {
				client = state.Client
			}
		}
		m.mu.Unlock()
		control, ok := client.(interface {
			ControlSubagent(context.Context, string, string, map[string]any) (map[string]any, error)
		})
		if !ok {
			return nil, errors.New("当前物理 round 没有可用的 Subagent control；不能通过重试或 CLI 绕过")
		}
		return control.ControlSubagent(ctx, toolUseID, operation, input)
	}
}

func (c *agentClient) ControlSubagent(ctx context.Context, toolUseID, operation string, input map[string]any) (map[string]any, error) {
	session, err := c.currentSession()
	if err != nil {
		return nil, err
	}
	if !session.Supports(bridge.CapabilitySubagentControl) {
		return nil, errors.New("当前 runtime 未协商 subagent_control_v1，请使用支持该能力的 nxs runtime")
	}
	return session.Control().ControlSubagent(ctx, toolUseID, operation, input)
}
