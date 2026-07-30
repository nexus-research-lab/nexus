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
	CommandCatalogStatusCold        CommandCatalogStatus = "cold"
	CommandCatalogStatusStarting    CommandCatalogStatus = "starting"
	CommandCatalogStatusReady       CommandCatalogStatus = "ready"
	CommandCatalogStatusUnavailable CommandCatalogStatus = "unavailable"
)

// CommandCatalogSnapshot 是 Manager 对上层暴露的只读命令目录快照。
type CommandCatalogSnapshot struct {
	Status      CommandCatalogStatus
	Generation  uint64
	RuntimeKind agentclient.RuntimeKind
	Commands    []agentclient.SlashCommand
}

// CommandCatalog 返回 runtime 初始化时缓存的命令目录，不读取子进程或隐式创建 runtime。
func (m *Manager) CommandCatalog(
	_ context.Context,
	sessionKey string,
	ownerUserID string,
) (CommandCatalogSnapshot, error) {
	if m == nil {
		return CommandCatalogSnapshot{Status: CommandCatalogStatusUnavailable}, nil
	}

	m.mu.RLock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	var status CommandCatalogStatus
	var generation uint64
	var runtimeKind agentclient.RuntimeKind
	var commands []agentclient.SlashCommand
	var runtimeOwnerUserID string
	var hasSession bool
	if state != nil && !state.Closing {
		status = state.CommandCatalogStatus
		generation = state.RuntimeGeneration
		runtimeKind = state.RuntimeKind
		commands = cloneSlashCommands(state.Commands)
		runtimeOwnerUserID = state.OwnerUserID
		hasSession = sessionStateHasConnectedClient(state)
	}
	m.mu.RUnlock()

	if runtimeOwnerMismatch(runtimeOwnerUserID, strings.TrimSpace(ownerUserID)) {
		return CommandCatalogSnapshot{}, ErrCommandCatalogOwnerMismatch
	}
	if state == nil {
		return CommandCatalogSnapshot{
			Status:      CommandCatalogStatusCold,
			Generation:  generation,
			RuntimeKind: runtimeKind,
			Commands:    []agentclient.SlashCommand{},
		}, nil
	}
	if status == "" {
		status = CommandCatalogStatusCold
	}
	if !hasSession &&
		(status == CommandCatalogStatusReady ||
			status == CommandCatalogStatusUnavailable) {
		status = CommandCatalogStatusCold
		commands = nil
	}
	if commands == nil {
		commands = []agentclient.SlashCommand{}
	}
	return CommandCatalogSnapshot{
		Status:      status,
		Generation:  generation,
		RuntimeKind: runtimeKind,
		Commands:    commands,
	}, nil
}

// SyncCommandCatalog 在 runtime 完成初始化后读取一次能力快照，并绑定到当前 generation。
func (m *Manager) SyncCommandCatalog(
	ctx context.Context,
	sessionKey string,
	client Client,
) error {
	if m == nil || client == nil {
		return agentclient.ErrNotConnected
	}
	sessionKey = strings.TrimSpace(sessionKey)
	generation, ok := m.beginCommandCatalogSync(sessionKey, client)
	if !ok {
		return nil
	}
	provider, ok := client.(SlashCommandProvider)
	if !ok {
		m.storeCommandCatalog(
			sessionKey,
			client,
			generation,
			CommandCatalogStatusUnavailable,
			nil,
		)
		return nil
	}
	commands, err := provider.SupportedCommands(ctx)
	if err != nil {
		status := CommandCatalogStatusUnavailable
		if errors.Is(err, agentclient.ErrUnsupportedCapability) {
			m.storeCommandCatalog(sessionKey, client, generation, status, nil)
			return nil
		}
		if IsRuntimeTransportClosedError(err) {
			status = CommandCatalogStatusCold
		}
		if !m.storeCommandCatalog(sessionKey, client, generation, status, nil) {
			// provider 返回时 session 可能已经切到下一代；旧错误不应
			// 让上层误判当前连接也失败。
			return nil
		}
		return err
	}
	m.storeCommandCatalog(
		sessionKey,
		client,
		generation,
		CommandCatalogStatusReady,
		commands,
	)
	return nil
}

// BeginRuntimeConnection 在一个已有 session 重新建立底层连接前刷新目录生命周期。
//
// adapter 可以在 transport 断开后被 Manager 复用；此时它代表的是新 runtime
// 进程/连接，不能继续沿用上一代 ready 快照。首次创建或已经热连接的 client
// 不增加 generation，只有从非热状态重新连接才会切换到新 generation。
func (m *Manager) BeginRuntimeConnection(
	sessionKey string,
	client Client,
	wasConnected bool,
) uint64 {
	if m == nil || client == nil {
		return 0
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil || state.Closing || state.Client != client {
		return 0
	}
	if wasConnected {
		state.RuntimeConnectionStarted = true
		return state.RuntimeGeneration
	}
	// 新 client 在 GetOrCreate/replaceRuntimeClient 时已经分配了 generation；
	// 后续同一 client 的重连（包括同步进行中的替换）才切到下一代。
	if state.RuntimeConnectionStarted ||
		state.CommandCatalogStatus == CommandCatalogStatusCold ||
		state.CommandCatalogStatus == CommandCatalogStatusReady ||
		state.CommandCatalogStatus == CommandCatalogStatusUnavailable {
		state.RuntimeGeneration = m.nextGeneration.Add(1)
	}
	state.RuntimeConnectionStarted = true
	state.CommandCatalogStatus = CommandCatalogStatusStarting
	state.CommandCatalogSyncing = false
	state.Commands = nil
	m.touchStateLocked(state)
	return state.RuntimeGeneration
}

// MarkCommandCatalogCold 允许同一 generation 在连接失败后再次完成初始化。
func (m *Manager) MarkCommandCatalogCold(sessionKey string, client Client) {
	if m == nil {
		return
	}
	m.mu.RLock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	var generation uint64
	if state != nil && !state.Closing && state.Client == client {
		generation = state.RuntimeGeneration
	}
	m.mu.RUnlock()
	m.MarkCommandCatalogColdForGeneration(sessionKey, client, generation)
}

// MarkCommandCatalogColdForGeneration 只把指定 generation 标记为 cold，避免旧连接
// 的失败回调覆盖已经切换到新 generation 的 runtime。
func (m *Manager) MarkCommandCatalogColdForGeneration(
	sessionKey string,
	client Client,
	generation uint64,
) {
	m.storeCommandCatalog(
		strings.TrimSpace(sessionKey),
		client,
		generation,
		CommandCatalogStatusCold,
		nil,
	)
}

func (m *Manager) beginCommandCatalogSync(sessionKey string, client Client) (uint64, bool) {
	if m == nil || sessionKey == "" || client == nil {
		return 0, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil || state.Closing || state.Client != client || state.CommandCatalogSyncing {
		return 0, false
	}
	switch state.CommandCatalogStatus {
	case "", CommandCatalogStatusCold, CommandCatalogStatusStarting:
		state.CommandCatalogStatus = CommandCatalogStatusStarting
		state.CommandCatalogSyncing = true
		m.touchStateLocked(state)
		return state.RuntimeGeneration, true
	default:
		return 0, false
	}
}

func (m *Manager) storeCommandCatalog(
	sessionKey string,
	client Client,
	generation uint64,
	status CommandCatalogStatus,
	commands []agentclient.SlashCommand,
) bool {
	if m == nil || sessionKey == "" || client == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.sessions[sessionKey]
	if state == nil ||
		state.Closing ||
		state.Client != client ||
		state.RuntimeGeneration != generation {
		return false
	}
	state.CommandCatalogStatus = status
	state.CommandCatalogSyncing = false
	state.Commands = cloneSlashCommands(commands)
	m.touchStateLocked(state)
	return true
}

func cloneSlashCommands(commands []agentclient.SlashCommand) []agentclient.SlashCommand {
	if len(commands) == 0 {
		return nil
	}
	result := make([]agentclient.SlashCommand, 0, len(commands))
	for _, command := range commands {
		copyCommand := command
		if command.Raw != nil {
			copyCommand.Raw = make(map[string]any, len(command.Raw))
			for key, value := range command.Raw {
				copyCommand.Raw[key] = value
			}
		}
		result = append(result, copyCommand)
	}
	return result
}
