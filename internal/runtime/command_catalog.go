// INPUT: Manager 中的 session runtime 与 bridge 可选命令能力。
// OUTPUT: 当前 session 的命令目录状态、runtime 类型和安全描述原值。
// POS: WebSocket command catalog 与具体 SDK client 之间的只读能力门面。
package runtime

import (
	"context"
	"errors"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// ErrCommandCatalogOwnerMismatch 阻止跨 owner 读取运行时命令目录。
var ErrCommandCatalogOwnerMismatch = errors.New("runtime command catalog owner mismatch")

// CommandCatalogStatus 表示命令目录在运行时生命周期中的可用状态。
type CommandCatalogStatus string

const (
	CommandCatalogStatusLoading     CommandCatalogStatus = "loading"
	CommandCatalogStatusReady       CommandCatalogStatus = "ready"
	CommandCatalogStatusUnavailable CommandCatalogStatus = "unavailable"
)

// CommandCatalogSnapshot 是 Manager 对上层暴露的只读命令目录快照。
type CommandCatalogSnapshot struct {
	Status      CommandCatalogStatus
	RuntimeKind agentclient.RuntimeKind
	Commands    []agentclient.SlashCommand
}

// CommandCatalog 返回当前 session 的命令目录，不为读取目录隐式创建 runtime。
func (m *Manager) CommandCatalog(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
) (CommandCatalogSnapshot, error) {
	if m == nil {
		return CommandCatalogSnapshot{Status: CommandCatalogStatusUnavailable}, nil
	}

	m.mu.RLock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	var client Client
	var runtimeKind agentclient.RuntimeKind
	var runtimeOwnerUserID string
	if state != nil && !state.Closing {
		client = state.Client
		runtimeKind = state.RuntimeKind
		runtimeOwnerUserID = state.OwnerUserID
	}
	m.mu.RUnlock()

	if runtimeOwnerMismatch(runtimeOwnerUserID, strings.TrimSpace(ownerUserID)) {
		return CommandCatalogSnapshot{}, ErrCommandCatalogOwnerMismatch
	}
	if client == nil {
		return CommandCatalogSnapshot{
			Status:      CommandCatalogStatusLoading,
			RuntimeKind: runtimeKind,
			Commands:    []agentclient.SlashCommand{},
		}, nil
	}
	provider, ok := client.(SlashCommandProvider)
	if !ok {
		return CommandCatalogSnapshot{
			Status:      CommandCatalogStatusUnavailable,
			RuntimeKind: runtimeKind,
			Commands:    []agentclient.SlashCommand{},
		}, nil
	}
	commands, err := provider.SupportedCommands(ctx)
	if err != nil {
		if IsRuntimeTransportClosedError(err) {
			return CommandCatalogSnapshot{
				Status:      CommandCatalogStatusLoading,
				RuntimeKind: runtimeKind,
				Commands:    []agentclient.SlashCommand{},
			}, nil
		}
		return CommandCatalogSnapshot{}, err
	}
	if commands == nil {
		commands = []agentclient.SlashCommand{}
	}
	return CommandCatalogSnapshot{
		Status:      CommandCatalogStatusReady,
		RuntimeKind: runtimeKind,
		Commands:    commands,
	}, nil
}
