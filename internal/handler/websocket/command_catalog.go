// INPUT: WebSocket session 请求、Nexus host/fixed product command、owner 命名工作图与内置 runtime 指令快照。
// OUTPUT: 合并且仅含安全元数据的 session-scoped 动静态 command_catalog 权威事件。
// POS: Nexus 版本化命令目录到浏览器补全协议的唯一投影边界。
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

func (h *Handler) commandCatalogData(
	ctx context.Context,
	parsed protocol.SessionKey,
	agentID string,
) (protocol.CommandCatalogData, error) {
	workflowCommands, err := h.workGraphWorkflowCommands(
		ctx,
		authctx.OwnerUserID(ctx),
	)
	if err != nil {
		return protocol.CommandCatalogData{}, err
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		snapshot, err := h.commandCatalogSnapshot(ctx, agentID)
		if err != nil {
			return protocol.CommandCatalogData{}, err
		}
		hostCommands := []protocol.CommandDescriptor(nil)
		if protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) == protocol.SessionChannelWebSocketSegment {
			hostCommands = h.hostCommandDescriptors(slashcommandsvc.ScopeDM)
		}
		return projectCommandCatalog(
			snapshot,
			agentID,
			hostCommands,
			workflowCommands,
		), nil
	case protocol.SessionKeyKindRoom:
		return projectCommandCatalog(
			slashcommandsvc.RuntimeCatalogSnapshot{
				Status: protocol.CommandCatalogStatusUnavailable,
			},
			agentID,
			h.hostCommandDescriptors(slashcommandsvc.ScopeRoom),
			workflowCommands,
		), nil
	default:
		return protocol.CommandCatalogData{}, errors.New("command catalog requires an agent or Room session")
	}
}

func (h *Handler) workGraphWorkflowCommands(
	ctx context.Context,
	ownerUserID string,
) ([]protocol.CommandDescriptor, error) {
	if h == nil || h.workGraphWorkflows == nil {
		return nil, nil
	}
	return h.workGraphWorkflows.CommandDescriptors(ctx, ownerUserID)
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
		if err := h.dm.AuthorizeDMSessionAccess(ctx, parsed.Raw, parsed.AgentID); err != nil {
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
	agentID string,
) (slashcommandsvc.RuntimeCatalogSnapshot, error) {
	if h == nil || h.commandCatalog == nil {
		return slashcommandsvc.RuntimeCatalogSnapshot{
			Status: protocol.CommandCatalogStatusUnavailable,
		}, nil
	}
	kind := agentclient.RuntimeNXS
	if h.runtimeKindResolver != nil {
		resolvedKind, err := h.runtimeKindResolver(ctx, agentID)
		if err != nil {
			return slashcommandsvc.RuntimeCatalogSnapshot{}, err
		}
		kind = resolvedKind
	}
	return h.commandCatalog.Snapshot(kind), nil
}

func projectCommandCatalog(
	snapshot slashcommandsvc.RuntimeCatalogSnapshot,
	agentID string,
	hostCommands []protocol.CommandDescriptor,
	workflowCommands ...[]protocol.CommandDescriptor,
) protocol.CommandCatalogData {
	commands := projectHostCommands(hostCommands)
	for _, productCommand := range []protocol.CommandDescriptor{
		slashcommandsvc.BrowserCommandDescriptor(),
		slashcommandsvc.VisualizeCommandDescriptor(),
		slashcommandsvc.WorkGraphCommandDescriptor(),
	} {
		if command, ok := projectRuntimeCommand(productCommand); ok {
			commands = append(commands, command)
		}
	}
	for _, workflowSet := range workflowCommands {
		for _, workflowCommand := range workflowSet {
			if command, ok := projectRuntimeCommand(workflowCommand); ok {
				commands = append(commands, command)
			}
		}
	}
	if snapshot.Status == protocol.CommandCatalogStatusReady {
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
	command protocol.CommandDescriptor,
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
		Enabled:      command.Enabled,
		DisabledReason: limitCommandText(
			command.DisabledReason,
			commandDescriptionMaxRunes,
		),
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
