// INPUT: WebSocket session 请求、Nexus host command 与 runtime 初始化能力快照。
// OUTPUT: 合并且仅含安全元数据的 session-scoped command_catalog 权威事件。
// POS: Nexus/bridge runtime 命令目录到浏览器补全协议的唯一投影边界。
package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	commandNameMaxRunes         = 128
	commandDescriptionMaxRunes  = 512
	commandArgumentHintMaxRunes = 256
)

func (h *Handler) commandCatalogEvent(
	ctx context.Context,
	sessionKey string,
	parsed protocol.SessionKey,
	inbound map[string]any,
) (protocol.EventMessage, error) {
	agentID, err := h.resolveCommandCatalogAgent(ctx, parsed, inbound)
	if err != nil {
		return protocol.EventMessage{}, err
	}
	data, err := h.commandCatalogData(ctx, parsed, agentID)
	if err != nil {
		return protocol.EventMessage{}, err
	}
	return protocol.NewCommandCatalogEvent(sessionKey, data), nil
}

func (h *Handler) broadcastCommandCatalog(
	ctx context.Context,
	sessionKey string,
	parsed protocol.SessionKey,
) error {
	if h == nil || h.permission == nil {
		return nil
	}
	data, err := h.commandCatalogData(ctx, parsed, parsed.AgentID)
	if err != nil {
		return err
	}
	errs := h.permission.BroadcastEvent(
		ctx,
		sessionKey,
		protocol.NewCommandCatalogEvent(sessionKey, data),
	)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (h *Handler) initializeBoundCommandCatalog(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
	parsed protocol.SessionKey,
) {
	if parsed.Kind != protocol.SessionKeyKindAgent || h.dm == nil {
		return
	}
	snapshot, err := h.commandCatalogSnapshot(ctx, sessionKey)
	if err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	if h.runtime != nil &&
		h.runtime.HasSession(sessionKey) &&
		(snapshot.Status == runtimectx.CommandCatalogStatusReady ||
			snapshot.Status == runtimectx.CommandCatalogStatusUnavailable) {
		return
	}
	starting := snapshot
	starting.Status = runtimectx.CommandCatalogStatusStarting
	h.broadcastProjectedCommandCatalog(ctx, sessionKey, parsed.AgentID, starting)

	if err = h.dm.EnsureRuntimeSession(ctx, sessionKey, parsed.AgentID); err != nil {
		failed, snapshotErr := h.commandCatalogSnapshot(ctx, sessionKey)
		if snapshotErr != nil {
			failed = starting
		}
		failed = projectCommandCatalogStartupFailure(failed)
		h.broadcastProjectedCommandCatalog(ctx, sessionKey, parsed.AgentID, failed)
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": "bind_session",
		})
		return
	}
	if err = h.broadcastCommandCatalog(ctx, sessionKey, parsed); err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": "bind_session",
		})
	}
}

func projectCommandCatalogStartupFailure(
	snapshot runtimectx.CommandCatalogSnapshot,
) runtimectx.CommandCatalogSnapshot {
	// cold 只表示尚未尝试；启动失败必须投影为终态，否则 Composer 会
	// 永久显示“正在加载”。并发的新代际若已经 ready，则保留其权威结果。
	if snapshot.Status != runtimectx.CommandCatalogStatusReady {
		snapshot.Status = runtimectx.CommandCatalogStatusUnavailable
	}
	return snapshot
}

func (h *Handler) broadcastProjectedCommandCatalog(
	ctx context.Context,
	sessionKey string,
	agentID string,
	snapshot runtimectx.CommandCatalogSnapshot,
) {
	if h == nil || h.permission == nil {
		return
	}
	data := projectCommandCatalog(
		snapshot,
		agentID,
		h.hostCommandDescriptors(slashcommandsvc.ScopeDM),
	)
	_ = h.permission.BroadcastEvent(
		ctx,
		sessionKey,
		protocol.NewCommandCatalogEvent(sessionKey, data),
	)
}

func (h *Handler) commandCatalogData(
	ctx context.Context,
	parsed protocol.SessionKey,
	agentID string,
) (protocol.CommandCatalogData, error) {
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		snapshot, err := h.commandCatalogSnapshot(ctx, parsed.Raw)
		if err != nil {
			return protocol.CommandCatalogData{}, err
		}
		return projectCommandCatalog(
			snapshot,
			agentID,
			h.hostCommandDescriptors(slashcommandsvc.ScopeDM),
		), nil
	case protocol.SessionKeyKindRoom:
		return projectCommandCatalog(
			runtimectx.CommandCatalogSnapshot{
				Status: runtimectx.CommandCatalogStatusUnavailable,
			},
			agentID,
			h.hostCommandDescriptors(slashcommandsvc.ScopeRoom),
		), nil
	default:
		return protocol.CommandCatalogData{}, errors.New("command catalog requires an agent or Room session")
	}
}

func (h *Handler) resolveCommandCatalogAgent(
	ctx context.Context,
	parsed protocol.SessionKey,
	inbound map[string]any,
) (string, error) {
	if parsed.Kind == protocol.SessionKeyKindAgent {
		if h == nil || h.dm == nil {
			return "", errors.New("DM service is unavailable")
		}
		if requestedAgentID := handlershared.StringValue(inbound["agent_id"]); requestedAgentID != "" &&
			requestedAgentID != parsed.AgentID {
			return "", errors.New("agent_id does not match session_key")
		}
		if err := h.dm.AuthorizeHostCommand(ctx, parsed.Raw, parsed.AgentID); err != nil {
			return "", err
		}
		return parsed.AgentID, nil
	}
	if parsed.Kind != protocol.SessionKeyKindRoom || !parsed.IsShared {
		return "", errors.New("command catalog requires an agent or shared Room session")
	}

	conversationID := parsed.ConversationID
	if requested := handlershared.StringValue(inbound["conversation_id"]); requested != "" &&
		requested != conversationID {
		return "", errors.New("conversation_id does not match session_key")
	}
	agentID := handlershared.StringValue(inbound["agent_id"])
	if agentID == "" {
		return "", errors.New("agent_id is required for a Room command catalog")
	}
	if h.roomService == nil {
		return "", errors.New("Room service is unavailable")
	}
	contextValue, err := h.roomService.GetConversationContext(ctx, conversationID)
	if err != nil {
		return "", err
	}
	if contextValue == nil || contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return "", errors.New("Room command catalog requires a group Room")
	}
	if roomID := handlershared.StringValue(inbound["room_id"]); roomID != "" &&
		roomID != contextValue.Room.ID {
		return "", errors.New("room_id does not match conversation")
	}
	if !roomHasAgent(contextValue.Members, agentID) {
		return "", errors.New("agent_id is not a Room member")
	}
	return agentID, nil
}

func roomHasAgent(members []protocol.MemberRecord, agentID string) bool {
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent &&
			strings.TrimSpace(member.MemberAgentID) == agentID {
			return true
		}
	}
	return false
}

func (h *Handler) hostCommandDescriptors(scope slashcommandsvc.Scope) []protocol.CommandDescriptor {
	if h == nil || h.hostCommands == nil {
		return nil
	}
	return h.hostCommands.Descriptors(scope)
}

func (h *Handler) commandCatalogSnapshot(
	ctx context.Context,
	runtimeSessionKey string,
) (runtimectx.CommandCatalogSnapshot, error) {
	if h == nil || h.runtime == nil {
		return runtimectx.CommandCatalogSnapshot{
			Status: runtimectx.CommandCatalogStatusUnavailable,
		}, nil
	}
	snapshot, err := h.runtime.CommandCatalog(
		ctx,
		runtimeSessionKey,
		authctx.OwnerUserID(ctx),
	)
	if errors.Is(err, runtimectx.ErrCommandCatalogOwnerMismatch) {
		return runtimectx.CommandCatalogSnapshot{}, errors.New("command catalog is not available for this session")
	}
	return snapshot, err
}

func projectCommandCatalog(
	snapshot runtimectx.CommandCatalogSnapshot,
	agentID string,
	hostCommands []protocol.CommandDescriptor,
) protocol.CommandCatalogData {
	commands := projectHostCommands(hostCommands)
	if snapshot.Status == runtimectx.CommandCatalogStatusReady {
		for _, command := range snapshot.Commands {
			if descriptor, ok := projectRuntimeCommand(command); ok {
				commands = append(commands, descriptor)
			}
		}
	}
	commands = mergeCommandDescriptors(commands)
	data := protocol.CommandCatalogData{
		Generation:  snapshot.Generation,
		RuntimeKind: string(snapshot.RuntimeKind),
		Status:      protocol.CommandCatalogStatus(snapshot.Status),
		AgentID:     strings.TrimSpace(agentID),
		Commands:    commands,
	}
	data.Revision = commandCatalogRevision(data)
	return data
}

func projectHostCommands(commands []protocol.CommandDescriptor) []protocol.CommandDescriptor {
	result := make([]protocol.CommandDescriptor, 0, len(commands))
	for _, command := range commands {
		name := strings.TrimSpace(strings.TrimPrefix(command.Name, "/"))
		if name == "" ||
			len([]rune(name)) > commandNameMaxRunes ||
			!isPublicRuntimeCommandName(name) {
			continue
		}
		result = append(result, protocol.CommandDescriptor{
			Name:           name,
			Description:    limitCommandText(command.Description, commandDescriptionMaxRunes),
			ArgumentHint:   limitCommandText(command.ArgumentHint, commandArgumentHintMaxRunes),
			Execution:      protocol.CommandExecutionHost,
			Enabled:        command.Enabled,
			DisabledReason: limitCommandText(command.DisabledReason, commandDescriptionMaxRunes),
		})
	}
	return result
}

func projectRuntimeCommand(
	command agentclient.SlashCommand,
) (protocol.CommandDescriptor, bool) {
	name := strings.TrimSpace(strings.TrimPrefix(command.Name, "/"))
	if name == "" ||
		len([]rune(name)) > commandNameMaxRunes ||
		!isPublicRuntimeCommandName(name) {
		return protocol.CommandDescriptor{}, false
	}
	return protocol.CommandDescriptor{
		Name:         name,
		Description:  limitCommandText(command.Description, commandDescriptionMaxRunes),
		ArgumentHint: limitCommandText(command.ArgumentHint, commandArgumentHintMaxRunes),
		Execution:    protocol.CommandExecutionRuntime,
		Enabled:      true,
	}, true
}

func mergeCommandDescriptors(commands []protocol.CommandDescriptor) []protocol.CommandDescriptor {
	result := make([]protocol.CommandDescriptor, 0, len(commands))
	seen := map[string]struct{}{}
	// Host command 总是在 runtime command 之前传入，因此名称冲突时由 Nexus 保留。
	for _, command := range commands {
		key := strings.ToLower(strings.TrimSpace(command.Name))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, command)
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Name < result[right].Name
	})
	return result
}

func isPublicRuntimeCommandName(name string) bool {
	fields := strings.Fields(name)
	if len(fields) == 1 {
		return true
	}
	return len(fields) == 2 &&
		fields[1] == "(MCP)" &&
		strings.HasSuffix(name, " (MCP)")
}

func limitCommandText(value string, maxRunes int) string {
	normalized := strings.TrimSpace(value)
	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}
	return string(runes[:maxRunes])
}

func commandCatalogRevision(data protocol.CommandCatalogData) string {
	payload, err := json.Marshal(struct {
		Status      protocol.CommandCatalogStatus `json:"status"`
		Generation  uint64                        `json:"generation"`
		RuntimeKind string                        `json:"runtime_kind"`
		AgentID     string                        `json:"agent_id"`
		Commands    []protocol.CommandDescriptor  `json:"commands"`
	}{
		Status:      data.Status,
		Generation:  data.Generation,
		RuntimeKind: data.RuntimeKind,
		AgentID:     data.AgentID,
		Commands:    data.Commands,
	})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("commands-%s", hex.EncodeToString(digest[:8]))
}
