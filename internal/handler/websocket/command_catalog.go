// INPUT: 已绑定 WebSocket session、当前 Agent 身份与 runtime 命令快照。
// OUTPUT: 仅含安全元数据的 session-scoped command_catalog 事件。
// POS: runtime 控制面到浏览器命令补全协议的投影边界。
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

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	commandNameMaxRunes         = 128
	commandDescriptionMaxRunes  = 512
	commandArgumentHintMaxRunes = 256
)

func (h *Handler) handleGetCommandCatalog(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok || parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	h.ensureSessionBinding(ctx, sender, sessionKey)
	initialize, _ := handlershared.BoolValue(inbound["initialize_runtime"])
	if err := h.sendCommandCatalog(ctx, sender, sessionKey, parsed, inbound, initialize); err != nil {
		h.sendGatewayError(ctx, sender, sessionKey, "command_catalog_error", err, map[string]any{
			"type": handlershared.StringValue(inbound["type"]),
		})
	}
}

func (h *Handler) sendCommandCatalog(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
	parsed protocol.SessionKey,
	inbound map[string]any,
	initialize bool,
) error {
	runtimeSessionKey, agentID, err := h.resolveCommandCatalogTarget(ctx, parsed, inbound)
	if err != nil {
		return err
	}
	snapshot, err := h.commandCatalogSnapshot(
		ctx,
		runtimeSessionKey,
	)
	if err != nil {
		return err
	}
	if initialize && snapshot.Status == runtimectx.CommandCatalogStatusLoading &&
		parsed.Kind == protocol.SessionKeyKindAgent && h.dm != nil {
		if err = h.dm.EnsureCommandCatalogRuntime(ctx, runtimeSessionKey, agentID); err != nil {
			return err
		}
		snapshot, err = h.commandCatalogSnapshot(ctx, runtimeSessionKey)
		if err != nil {
			return err
		}
	}
	data := projectCommandCatalog(snapshot, agentID)
	return sender.SendEvent(ctx, protocol.NewCommandCatalogEvent(sessionKey, data))
}

func (h *Handler) commandCatalogSnapshot(
	ctx context.Context,
	runtimeSessionKey string,
) (runtimectx.CommandCatalogSnapshot, error) {
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

func (h *Handler) resolveCommandCatalogTarget(
	ctx context.Context,
	parsed protocol.SessionKey,
	inbound map[string]any,
) (string, string, error) {
	if parsed.Kind == protocol.SessionKeyKindAgent {
		return parsed.Raw, parsed.AgentID, nil
	}
	if parsed.Kind != protocol.SessionKeyKindRoom || !parsed.IsShared {
		return "", "", errors.New("command catalog requires an agent or shared Room session")
	}

	conversationID := parsed.ConversationID
	if requested := handlershared.StringValue(inbound["conversation_id"]); requested != "" &&
		requested != conversationID {
		return "", "", errors.New("conversation_id does not match session_key")
	}
	agentID := handlershared.StringValue(inbound["agent_id"])
	if agentID == "" {
		return "", "", errors.New("agent_id is required for a Room command catalog")
	}
	if h.roomService == nil {
		return "", "", errors.New("Room service is unavailable")
	}
	contextValue, err := h.roomService.GetConversationContext(ctx, conversationID)
	if err != nil {
		return "", "", err
	}
	if roomID := handlershared.StringValue(inbound["room_id"]); roomID != "" &&
		roomID != contextValue.Room.ID {
		return "", "", errors.New("room_id does not match conversation")
	}
	if !roomHasAgent(contextValue.Members, agentID) {
		return "", "", errors.New("agent_id is not a Room member")
	}
	return protocol.BuildRoomAgentSessionKey(
		conversationID,
		agentID,
		contextValue.Room.RoomType,
	), agentID, nil
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

func projectCommandCatalog(
	snapshot runtimectx.CommandCatalogSnapshot,
	agentID string,
) protocol.CommandCatalogData {
	status := protocol.CommandCatalogStatus(snapshot.Status)
	commands := make([]protocol.CommandDescriptor, 0, len(snapshot.Commands))
	if snapshot.Status == runtimectx.CommandCatalogStatusReady {
		for _, command := range snapshot.Commands {
			if descriptor, ok := projectRuntimeCommand(command); ok {
				commands = append(commands, descriptor)
			}
		}
		sort.Slice(commands, func(left int, right int) bool {
			return commands[left].Name < commands[right].Name
		})
	}
	data := protocol.CommandCatalogData{
		RuntimeKind: string(snapshot.RuntimeKind),
		Status:      status,
		AgentID:     strings.TrimSpace(agentID),
		Commands:    commands,
	}
	if status == protocol.CommandCatalogStatusReady {
		data.Revision = commandCatalogRevision(data)
	}
	return data
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
		Execution:    protocol.CommandExecutionRuntimePrompt,
		Enabled:      true,
	}, true
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
		RuntimeKind string                       `json:"runtime_kind"`
		AgentID     string                       `json:"agent_id"`
		Commands    []protocol.CommandDescriptor `json:"commands"`
	}{
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
