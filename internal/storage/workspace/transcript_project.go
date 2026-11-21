// INPUT: SDK transcript chain、Nexus round marker 与会话标识。
// OUTPUT: 带稳定 root/source round 身份、保留空白 Goal 续跑边界与 runtime 自唤醒触发行的 Nexus 历史消息，以及可在用户轮次边界保存与恢复的投影状态。
// POS: DM transcript 到产品历史的唯一投影入口；只负责把 transcript 条目识别为轮次边界事件并交给 message.Reducer，消息解释规则与实时链路共用。
package workspace

import (
	"iter"
	"slices"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func projectTranscriptChain(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []protocol.Message {
	return projectTranscriptChainWithFilter(
		workspacePath,
		sessionKey,
		agentID,
		chain,
		roundMarkers,
		shouldSkipTranscriptEntry,
	)
}

func projectExplicitTranscriptChain(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
) []protocol.Message {
	return projectTranscriptChainWithFilter(
		workspacePath,
		sessionKey,
		agentID,
		chain,
		nil,
		shouldSkipExplicitTranscriptEntry,
	)
}

func projectTranscriptChainWithFilter(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
	shouldSkip func(map[string]any) bool,
) []protocol.Message {
	projected := make([]protocol.Message, 0, len(chain))
	aligned := alignTranscriptRoundMarkersWithFilter(chain, roundMarkers, shouldSkip)
	visitProjectedTranscript(workspacePath, sessionKey, agentID, slices.Values(chain), aligned, shouldSkip, func(rows ...protocol.Message) {
		projected = append(projected, rows...)
	})
	return projected
}

func visitProjectedTranscript(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain iter.Seq[transcriptEntry],
	alignedMarkers []transcriptRoundMarker,
	shouldSkip func(map[string]any) bool,
	emit func(...protocol.Message),
) {
	visitProjectedTranscriptFrom(workspacePath, sessionKey, agentID, chain, alignedMarkers, shouldSkip, &transcriptProjectionState{}, nil, emit)
}

// transcriptProjectionState 是投影循环跨条目的全部状态。归约器只在轮次边界事件处重建，
// 重建后尚未处理任何条目时，它完全由 Processor* 字段记录的边界决定，
// 因而此刻的状态可以保存为检查点并在之后恢复。JSON 形状随读模型持久化，改动需提升读模型版本。
type transcriptProjectionState struct {
	CurrentRoundID           string `json:"current_round_id,omitempty"`
	CurrentRoundPurpose      string `json:"current_round_purpose,omitempty"`
	ProcessorActive          bool   `json:"processor_active,omitempty"`
	ProcessorRoundID         string `json:"processor_round_id,omitempty"`
	ProcessorAgentRoundID    string `json:"processor_agent_round_id,omitempty"`
	ProcessorParentID        string `json:"processor_parent_id,omitempty"`
	ProcessorSessionID       string `json:"processor_session_id,omitempty"`
	ProcessorBoundaryKind    string `json:"processor_boundary_kind,omitempty"`
	LastTimestamp            int64  `json:"last_timestamp,omitempty"`
	MarkerIndex              int    `json:"marker_index,omitempty"`
	LastVisibleUserContent   string `json:"last_visible_user_content,omitempty"`
	LastVisibleUserTimestamp int64  `json:"last_visible_user_timestamp,omitempty"`

	reducer *message.Reducer
}

// transcriptRoundStart 是一个用户轮次的起点：处理第 Position 个条目时重建了 processor。
// State 是处理该条目之前的状态，Rows 是此前已产出的行数。从该条目重新投影只需要这两者，
// 因为重放它会再次重建 processor，不依赖此前 processor 的内部状态。
type transcriptRoundStart struct {
	Position int
	Rows     int
	State    transcriptProjectionState
}

// beginRound 把一个轮次边界事件交给归约器，并记录恢复它所需的全部字段。
func (state *transcriptProjectionState) beginRound(scope message.MessageContext, boundary message.RoundBoundary) {
	state.CurrentRoundID = boundary.RoundID
	state.ProcessorActive = true
	state.ProcessorRoundID = boundary.RoundID
	state.ProcessorAgentRoundID = boundary.AgentRoundID
	state.ProcessorParentID = boundary.ParentID
	state.ProcessorSessionID = boundary.SessionID
	state.ProcessorBoundaryKind = string(boundary.Kind)
	state.reducer = message.NewReducer(scope)
	state.reducer.BeginRound(boundary)
}

// restoreReducer 按检查点记录的边界重建一个尚未处理任何条目的归约器。
func (state *transcriptProjectionState) restoreReducer(scope message.MessageContext) {
	state.reducer = nil
	if state.ProcessorActive {
		state.reducer = message.NewReducer(scope)
		state.reducer.BeginRound(message.RoundBoundary{
			Kind:         message.RoundBoundaryKind(state.ProcessorBoundaryKind),
			RoundID:      state.ProcessorRoundID,
			AgentRoundID: state.ProcessorAgentRoundID,
			ParentID:     state.ProcessorParentID,
			SessionID:    state.ProcessorSessionID,
		})
	}
}

// transcriptReducerScope 是历史回放交给归约器的会话级身份。
func transcriptReducerScope(workspacePath string, sessionKey string, agentID string) message.MessageContext {
	return message.MessageContext{
		SessionKey:    sessionKey,
		AgentID:       agentID,
		WorkspacePath: strings.TrimSpace(workspacePath),
	}
}

// visitProjectedTranscriptFrom 从 state 继续投影。roundStart 非 nil 时，每当一个条目重建了
// processor 且处理完毕，就以该轮次起点和处理完该条目后的状态回调一次，供调用方保存可恢复的检查点。
func visitProjectedTranscriptFrom(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain iter.Seq[transcriptEntry],
	alignedMarkers []transcriptRoundMarker,
	shouldSkip func(map[string]any) bool,
	state *transcriptProjectionState,
	roundStart func(start transcriptRoundStart, next transcriptProjectionState),
	emit func(...protocol.Message),
) {
	scope := transcriptReducerScope(workspacePath, sessionKey, agentID)
	position, emitted := -1, 0
	var pending *transcriptRoundStart
	output := emit
	emit = func(rows ...protocol.Message) {
		emitted += len(rows)
		output(rows...)
	}
	for entry := range chain {
		if pending != nil {
			roundStart(*pending, *state)
			pending = nil
		}
		position++
		before := *state
		before.reducer = nil
		if shouldSkip(entry.Data) {
			continue
		}

		entryTimestamp := transcriptEntryTimestamp(entry.Data, entry.Index, state.LastTimestamp)
		state.LastTimestamp = entryTimestamp
		entryPosition := message.TranscriptPosition{Line: entry.Index, Timestamp: entryTimestamp}

		if guidanceRows := buildTranscriptGuidanceMessages(
			sessionKey,
			agentID,
			state.CurrentRoundID,
			entry.Data,
			entryTimestamp,
		); len(guidanceRows) > 0 {
			emit(guidanceRows...)
			continue
		}

		decoded, err := sdkprotocol.DecodeMessage(entry.Data)
		if err != nil {
			continue
		}

		switch decoded.Type {
		case sdkprotocol.MessageTypeUser:
			if isTranscriptToolResult(decoded) {
				if state.reducer == nil {
					state.beginRound(scope, message.ImplicitRoundBoundary(
						textutil.FirstNonEmpty(stringFromAny(entry.Data["parentUuid"]), strings.TrimSpace(decoded.UUID)),
						decoded.SessionID,
					))
				}
				emit(state.reducer.ReduceAt(decoded, entryPosition).DurableMessages...)
				continue
			}
			if !shouldAlignTranscriptUserTurn(entry.Data) {
				continue
			}
			marker := consumeTranscriptRoundMarker(alignedMarkers, &state.MarkerIndex)
			// 普通空输入不建立轮次；匹配隐藏 Goal marker 的空输入必须先切换
			// processor，避免成果继承上一次真实用户输入的 round。
			if !shouldMaterializeTranscriptUserTurn(entry.Data) && !transcriptRoundMarkerPresent(marker) {
				continue
			}
			state.CurrentRoundPurpose = textutil.FirstNonEmpty(
				strings.TrimSpace(marker.Purpose),
				stringFromAny(entry.Data["purpose"]),
			)
			userContent := transcriptUserContent(entry.Data)
			if !transcriptRoundMarkerPresent(marker) &&
				shouldSuppressUnmatchedTranscriptUserTurn(
					entry.Data,
					userContent,
					entryTimestamp,
					state.LastVisibleUserContent,
					state.LastVisibleUserTimestamp,
				) {
				continue
			}
			trigger, runtimeWake := transcriptRuntimeWakeTrigger(entry.Data, marker)
			boundary := message.RoundBoundary{
				Kind:         message.RoundBoundaryUserInput,
				RoundID:      textutil.FirstNonEmpty(marker.RoundID, buildTranscriptRoundID(decoded.UUID)),
				AgentRoundID: strings.TrimSpace(marker.AgentRoundID),
				SessionID:    decoded.SessionID,
			}
			if runtimeWake {
				boundary.Kind = message.RoundBoundaryRuntimeWake
			}
			boundary.ParentID = textutil.FirstNonEmpty(strings.TrimSpace(marker.UserMessageID), message.DerivedUserMessageID(boundary.RoundID))
			state.beginRound(scope, boundary)
			if roundStart != nil {
				pending = &transcriptRoundStart{Position: position, Rows: emitted, State: before}
			}
			if marker.HiddenFromUser ||
				boolValueAny(entry.Data["hidden_from_user"]) ||
				isTranscriptGoalContextOnlyUserTurn(entry.Data) {
				continue
			}
			userMessage := buildTranscriptUserMessage(
				sessionKey,
				agentID,
				state.CurrentRoundID,
				marker.SourceRoundID,
				marker.UserMessageID,
				decoded.SessionID,
				entry.Data,
				marker.Content,
				marker.Attachments,
				marker.DeliveryPolicy,
				marker.ControlOnly,
				entryTimestamp,
			)
			if userMessage == nil {
				continue
			}
			if runtimeWake {
				message.DecorateRuntimeWakeUserMessage(*userMessage, trigger)
			}
			emit(*userMessage)
			state.LastVisibleUserContent = stringFromAny((*userMessage)["content"])
			state.LastVisibleUserTimestamp = entryTimestamp
		case sdkprotocol.MessageTypeAssistant,
			sdkprotocol.MessageTypeAttachment,
			sdkprotocol.MessageTypeSystem,
			sdkprotocol.MessageTypeTaskProgress:
			// Echo 输出必须通过宿主准入后的 overlay 才能进入用户历史。
			if state.CurrentRoundPurpose == "echo_followup" {
				continue
			}
			if state.reducer == nil {
				state.beginRound(scope, message.ImplicitRoundBoundary(buildTranscriptRoundID(decoded.UUID), decoded.SessionID))
			}
			emit(state.reducer.ReduceAt(decoded, entryPosition).DurableMessages...)
		case sdkprotocol.MessageTypeResult:
			// result 统一以 Nexus overlay 为真相源。
			// transcript 即使带了 result，也不再直接投影进历史，
			// 避免 assistant/usage 与 runtime result 语义重新混在一起。
			continue
		default:
			continue
		}
	}
	if pending != nil {
		roundStart(*pending, *state)
	}
}

func buildTranscriptUserMessage(
	sessionKey string,
	agentID string,
	roundID string,
	sourceRoundID string,
	userMessageID string,
	sessionID string,
	entry map[string]any,
	contentOverride string,
	attachments []protocol.ChatAttachment,
	deliveryPolicy string,
	controlOnly bool,
	timestamp int64,
) *protocol.Message {
	content := textutil.FirstNonEmpty(contentOverride, transcriptUserContent(entry))
	if content == "" {
		return nil
	}
	// 与 overlay marker 物化保持同一派生规则，保证按 message_id 去重仍然生效。
	if userMessageID = strings.TrimSpace(userMessageID); userMessageID == "" {
		userMessageID = message.DerivedUserMessageID(roundID)
	}
	payload := protocol.Message{
		"message_id":  userMessageID,
		"session_key": sessionKey,
		"agent_id":    agentID,
		"round_id":    roundID,
		"role":        "user",
		"content":     content,
		"timestamp":   timestamp,
	}
	if sourceRoundID = strings.TrimSpace(sourceRoundID); sourceRoundID != "" {
		payload["source_round_id"] = sourceRoundID
	}
	if strings.TrimSpace(sessionID) != "" {
		payload["session_id"] = strings.TrimSpace(sessionID)
	}
	if strings.TrimSpace(deliveryPolicy) != "" {
		payload["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(deliveryPolicy))
	}
	if controlOnly {
		payload["control_only"] = true
	}
	if normalizedAttachments := protocol.NormalizeChatAttachments(attachments, agentID); len(normalizedAttachments) > 0 {
		payload["attachments"] = normalizedAttachments
	}
	return &payload
}

func transcriptUserContent(entry map[string]any) string {
	if command := transcriptSlashCommandContent(entry); command != "" {
		return command
	}
	if trigger, ok := transcriptRuntimeWakeTrigger(entry, transcriptRoundMarker{}); ok {
		return trigger.Content()
	}
	return sanitizeTranscriptUserContent(transcriptRawUserContent(entry))
}

// transcriptRuntimeWakeTrigger 识别 runtime 自唤醒注入的用户条目（Claude Code 的 <task-notification>）。
// 对齐到的 marker 带有 live 侧的触发元数据时以其为准，保证刷新前后展示一致；原始标签永不展示。
func transcriptRuntimeWakeTrigger(entry map[string]any, marker transcriptRoundMarker) (message.RuntimeWakeTrigger, bool) {
	trigger, parsed := message.ParseTaskNotificationPrompt(transcriptRawUserContent(entry))
	origin, _ := entry["origin"].(map[string]any)
	if !parsed && stringFromAny(origin["kind"]) != "task-notification" {
		return message.RuntimeWakeTrigger{}, false
	}
	if message.IsRuntimeWakeMetadata(marker.Metadata) {
		return message.RuntimeWakeTriggerFromMetadata(marker.Content, marker.Metadata), true
	}
	return trigger, true
}

func transcriptRawUserContent(entry map[string]any) string {
	messageValue, _ := entry["message"].(map[string]any)
	contentValue := messageValue["content"]
	if text := stringFromAny(contentValue); text != "" {
		return text
	}
	items, _ := contentValue.([]any)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := stringFromAny(payload["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func sanitizeTranscriptUserContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if message.IsInternalTranscriptInterruptPrompt(trimmed) ||
		message.IsInternalTranscriptContinuationPrompt(trimmed) ||
		message.IsInternalExplicitSkillPrompt(trimmed) {
		return ""
	}
	trimmed, _, _ = stripLeadingTranscriptInternalContexts(trimmed)
	return stripTranscriptRuntimeContext(trimmed)
}

func stripLeadingTranscriptInternalContexts(content string) (string, bool, bool) {
	original := strings.TrimSpace(content)
	remaining := original
	stripped := false
	hasGoal := false
	for {
		remaining = strings.TrimSpace(remaining)
		var closeTag string
		switch {
		case strings.HasPrefix(remaining, "<internal_context "):
			closeTag = "</internal_context>"
		case strings.HasPrefix(remaining, "<codex_internal_context "):
			closeTag = "</codex_internal_context>"
		default:
			return remaining, stripped, hasGoal
		}

		openEnd := strings.IndexByte(remaining, '>')
		if openEnd < 0 {
			return original, false, false
		}
		closeOffset := strings.Index(remaining[openEnd+1:], closeTag)
		if closeOffset < 0 {
			return original, false, false
		}
		if strings.Contains(remaining[:openEnd+1], `source="goal"`) {
			hasGoal = true
		}
		remaining = remaining[openEnd+1+closeOffset+len(closeTag):]
		stripped = true
	}
}

func isTranscriptInternalContextOnly(content string) bool {
	remaining, stripped, _ := stripLeadingTranscriptInternalContexts(content)
	return stripped && strings.TrimSpace(remaining) == ""
}

func stripTranscriptRuntimeContext(content string) string {
	const (
		runtimeContextOpen  = "<nexus_runtime_context>"
		runtimeContextClose = "</nexus_runtime_context>"
	)
	trimmed := strings.TrimSpace(content)
	if !strings.HasSuffix(trimmed, runtimeContextClose) {
		return trimmed
	}
	openIndex := strings.LastIndex(trimmed, runtimeContextOpen)
	if openIndex < 0 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:openIndex])
}

func transcriptSlashCommandContent(entry map[string]any) string {
	content := strings.TrimSpace(transcriptRawUserContent(entry))
	name := transcriptTaggedValue(content, "command-name")
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	args := transcriptTaggedValue(content, "command-args")
	return strings.TrimSpace(name + " " + args)
}

func transcriptTaggedValue(content string, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(content, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(content[start:], closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[start : start+end])
}

func shouldSuppressUnmatchedTranscriptUserTurn(
	entry map[string]any,
	content string,
	timestamp int64,
	lastVisibleContent string,
	lastVisibleTimestamp int64,
) bool {
	command := transcriptSlashCommandContent(entry)
	if command == "" ||
		strings.TrimSpace(content) != strings.TrimSpace(lastVisibleContent) {
		return false
	}
	if timestamp <= 0 || lastVisibleTimestamp <= 0 {
		return true
	}
	const duplicateCommandToleranceMS = 5 * 1000
	distance := timestamp - lastVisibleTimestamp
	return distance >= 0 && distance <= duplicateCommandToleranceMS
}

func isTranscriptLocalCommandResultUserTurn(entry map[string]any) bool {
	content := strings.TrimSpace(transcriptRawUserContent(entry))
	return strings.HasPrefix(content, "Unknown skill:") ||
		strings.HasPrefix(content, "This skill can only be invoked by Nexus,")
}

func isTranscriptToolResult(message sdkprotocol.ReceivedMessage) bool {
	if message.User == nil {
		return false
	}
	if message.User.ToolUseResult != nil {
		return true
	}
	for _, block := range message.User.Message.Content {
		blockType := strings.TrimSpace(string(block.Type()))
		if blockType == "tool_result" || blockType == "server_tool_result" {
			return true
		}
	}
	return false
}

func buildTranscriptRoundID(uuid string) string {
	trimmed := strings.TrimSpace(uuid)
	if trimmed == "" {
		return "transcript_round"
	}
	return trimmed
}

func transcriptEntryTimestamp(entry map[string]any, index int, lastTimestamp int64) int64 {
	value := stringFromAny(entry["timestamp"])
	if value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed.UnixMilli()
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed.UnixMilli()
		}
	}
	if lastTimestamp > 0 {
		return lastTimestamp + 1
	}
	return int64(index + 1)
}
