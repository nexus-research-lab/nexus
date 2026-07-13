package workspace

import (
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 本文件是历史 legacy 形状的唯一归一化收口：
// - 旧 Room 行 `round_id = "root:agent"` 在这里拆成 root round + agent_round_id；
// - 旧 DM marker 的 message_id==round_id 已在 materializeRoundMarkerMessages 归一；
// 运行时其他代码不再解析这些旧形状。

type turnAccumulator struct {
	turn      protocol.ConversationTurn
	slotOrder []string
	slots     map[string]*protocol.AgentTurnSlot
}

// ProjectConversationTurns 把归一化历史行投影成升序 ConversationTurn 列表。
func ProjectConversationTurns(
	rows []protocol.Message,
	collapseRoomAgentRounds bool,
	activeRoundIDs []string,
) []protocol.ConversationTurn {
	projector := newConversationTurnProjector(collapseRoomAgentRounds, activeRoundIDs)
	for _, row := range rows {
		projector.add(row)
	}
	return projector.turns()
}

type conversationTurnProjector struct {
	collapseRoomAgentRounds bool
	active                  map[string]struct{}
	order                   []string
	accumulators            map[string]*turnAccumulator
}

type projectedTurnRow struct {
	source       protocol.Message
	rootRoundID  string
	agentRoundID string
	agentID      string
	message      protocol.ConversationMessage
}

func newConversationTurnProjector(
	collapseRoomAgentRounds bool,
	activeRoundIDs []string,
) *conversationTurnProjector {
	return &conversationTurnProjector{
		collapseRoomAgentRounds: collapseRoomAgentRounds,
		active:                  normalizeActiveRoundIDs(activeRoundIDs),
		order:                   make([]string, 0),
		accumulators:            make(map[string]*turnAccumulator),
	}
}

func (p *conversationTurnProjector) add(source protocol.Message) {
	row, ok := p.projectRow(source)
	if !ok {
		return
	}
	acc := p.ensureAccumulator(row.rootRoundID)
	acc.observeTimestamp(row.message.Timestamp)
	acc.addRow(row)
}

func (p *conversationTurnProjector) projectRow(source protocol.Message) (projectedTurnRow, bool) {
	rawRoundID := strings.TrimSpace(stringFromAny(source["round_id"]))
	agentID := strings.TrimSpace(stringFromAny(source["agent_id"]))
	rootRoundID := rawRoundID
	if p.collapseRoomAgentRounds {
		rootRoundID = normalizeRoomHistoryRoundID(rawRoundID, agentID)
	}
	rootRoundID = firstNonEmpty(rootRoundID, stringFromAny(source["message_id"]))
	if rootRoundID == "" {
		return projectedTurnRow{}, false
	}
	agentRoundID := strings.TrimSpace(stringFromAny(source["agent_round_id"]))
	if agentRoundID == "" && rawRoundID != rootRoundID {
		// 旧后缀值直接作为稳定 agent_round_id，避免重放时生成新标识。
		agentRoundID = rawRoundID
	}
	return projectedTurnRow{
		source:       source,
		rootRoundID:  rootRoundID,
		agentRoundID: agentRoundID,
		agentID:      agentID,
		message:      convertTurnMessage(source, rootRoundID, agentRoundID, agentID),
	}, true
}

func (p *conversationTurnProjector) ensureAccumulator(roundID string) *turnAccumulator {
	if acc := p.accumulators[roundID]; acc != nil {
		return acc
	}
	acc := &turnAccumulator{
		turn: protocol.ConversationTurn{
			RoundID:      roundID,
			Status:       "finished",
			AgentSlots:   []protocol.AgentTurnSlot{},
			SystemEvents: []protocol.ConversationMessage{},
			IsLoaded:     true,
		},
		slots: make(map[string]*protocol.AgentTurnSlot),
	}
	p.accumulators[roundID] = acc
	p.order = append(p.order, roundID)
	return acc
}

func (p *conversationTurnProjector) turns() []protocol.ConversationTurn {
	turns := make([]protocol.ConversationTurn, 0, len(p.order))
	for _, roundID := range p.order {
		acc := p.accumulators[roundID]
		acc.finalizeSlots()
		acc.turn.Status = resolveTurnStatus(acc.turn, p.active)
		markActiveTurnSlots(acc.turn.AgentSlots, roundID, p.active)
		turns = append(turns, acc.turn)
	}
	sort.SliceStable(turns, func(left, right int) bool {
		if turns[left].CreatedAt != turns[right].CreatedAt {
			return turns[left].CreatedAt < turns[right].CreatedAt
		}
		return turns[left].RoundID < turns[right].RoundID
	})
	return turns
}

func (acc *turnAccumulator) observeTimestamp(timestamp int64) {
	if acc.turn.CreatedAt == 0 || (timestamp > 0 && timestamp < acc.turn.CreatedAt) {
		acc.turn.CreatedAt = timestamp
	}
	if timestamp > acc.turn.UpdatedAt {
		acc.turn.UpdatedAt = timestamp
	}
}

func (acc *turnAccumulator) addRow(row projectedTurnRow) {
	switch strings.TrimSpace(stringFromAny(row.source["role"])) {
	case "user":
		if acc.turn.UserMessage == nil {
			message := row.message
			acc.turn.UserMessage = &message
			return
		}
		acc.turn.SystemEvents = append(acc.turn.SystemEvents, row.message)
	case "assistant":
		acc.addAssistantRow(row)
	case "result":
		applySlotResultSummary(
			acc.ensureSlot(row.agentID, row.agentRoundID),
			buildResultSummaryFromRow(row.source),
			row.message.Timestamp,
		)
	default:
		acc.turn.SystemEvents = append(acc.turn.SystemEvents, row.message)
	}
}

func (acc *turnAccumulator) addAssistantRow(row projectedTurnRow) {
	slot := acc.ensureSlot(row.agentID, row.agentRoundID)
	slot.AssistantMessages = append(slot.AssistantMessages, row.message)
	if slot.StartedAt == nil && row.message.Timestamp > 0 {
		startedAt := row.message.Timestamp
		slot.StartedAt = &startedAt
	}
	if summary, ok := row.source["result_summary"].(map[string]any); ok && len(summary) > 0 {
		applySlotResultSummary(slot, summary, row.message.Timestamp)
	}
}

func (acc *turnAccumulator) finalizeSlots() {
	for _, key := range acc.slotOrder {
		acc.turn.AgentSlots = append(acc.turn.AgentSlots, *acc.slots[key])
	}
}

func markActiveTurnSlots(
	slots []protocol.AgentTurnSlot,
	roundID string,
	active map[string]struct{},
) {
	if _, isActive := active[roundID]; !isActive {
		return
	}
	for index := range slots {
		if !protocol.IsTerminalRoundStatus(slots[index].Status) {
			slots[index].Status = "running"
		}
	}
}

func (acc *turnAccumulator) ensureSlot(agentID string, agentRoundID string) *protocol.AgentTurnSlot {
	key := agentRoundID
	if key == "" {
		key = "agent:" + agentID
	}
	if slot := acc.slots[key]; slot != nil {
		return slot
	}
	slot := &protocol.AgentTurnSlot{
		AgentID:            agentID,
		AgentRoundID:       agentRoundID,
		Status:             "finished",
		AssistantMessages:  []protocol.ConversationMessage{},
		PendingPermissions: []protocol.TurnPendingPermission{},
	}
	acc.slots[key] = slot
	acc.slotOrder = append(acc.slotOrder, key)
	return slot
}

func convertTurnMessage(
	row protocol.Message,
	rootRoundID string,
	agentRoundID string,
	agentID string,
) protocol.ConversationMessage {
	converted := protocol.ConversationMessage{
		MessageID:    strings.TrimSpace(stringFromAny(row["message_id"])),
		SessionKey:   strings.TrimSpace(stringFromAny(row["session_key"])),
		Role:         strings.TrimSpace(stringFromAny(row["role"])),
		RoundID:      rootRoundID,
		AgentRoundID: agentRoundID,
		AgentID:      agentID,
		ParentID:     strings.TrimSpace(stringFromAny(row["parent_id"])),
		Content:      row["content"],
		Timestamp:    messageTimestamp(row),
		StreamStatus: strings.TrimSpace(stringFromAny(row["stream_status"])),
	}
	if converted.Role == "result" {
		converted.Content = row["result"]
	}
	if summary, ok := row["result_summary"].(map[string]any); ok && len(summary) > 0 {
		converted.ResultSummary = summary
	}
	return converted
}

func buildResultSummaryFromRow(row protocol.Message) map[string]any {
	summary := map[string]any{
		"subtype": stringFromAny(row["subtype"]),
	}
	if duration := protocol.Int64FromAny(row["duration_ms"]); duration > 0 {
		summary["duration_ms"] = duration
	}
	if result := strings.TrimSpace(stringFromAny(row["result"])); result != "" {
		summary["result"] = result
	}
	if isError, ok := row["is_error"].(bool); ok {
		summary["is_error"] = isError
	}
	return summary
}

func applySlotResultSummary(slot *protocol.AgentTurnSlot, summary map[string]any, timestamp int64) {
	slot.ResultSummary = summary
	slot.Status = turnStatusFromSubtype(stringFromAny(summary["subtype"]))
	if timestamp > 0 {
		finishedAt := timestamp
		slot.FinishedAt = &finishedAt
	}
}

// turnStatusFromSubtype 把 result subtype 归一成 spec 状态枚举。
func turnStatusFromSubtype(subtype string) string {
	switch normalizeRoundStatusValue(subtype) {
	case roundStatusInterrupted:
		return "interrupted"
	case roundStatusError:
		return "error"
	case roundStatusRunning:
		return "running"
	default:
		return "finished"
	}
}

func resolveTurnStatus(turn protocol.ConversationTurn, active map[string]struct{}) string {
	if _, isActive := active[turn.RoundID]; isActive {
		return "running"
	}
	status := "finished"
	for _, slot := range turn.AgentSlots {
		switch slot.Status {
		case "error":
			return "error"
		case "interrupted":
			status = "interrupted"
		case "running", "pending":
			// 非 active 的 running slot 属于历史残留，按 interrupted 收口。
			if status == "finished" {
				status = "interrupted"
			}
		}
	}
	return status
}

// BuildConversationTurnIndex 由投影结果派生 turn 导航索引。
func BuildConversationTurnIndex(turns []protocol.ConversationTurn) []protocol.ConversationTurnIndexItem {
	items := make([]protocol.ConversationTurnIndexItem, 0, len(turns))
	for _, turn := range turns {
		item := protocol.ConversationTurnIndexItem{
			RoundID:   turn.RoundID,
			CreatedAt: turn.CreatedAt,
			UpdatedAt: turn.UpdatedAt,
			Status:    turn.Status,
			AgentIDs:  make([]string, 0, len(turn.AgentSlots)),
			Loaded:    turn.IsLoaded,
		}
		if turn.UserMessage != nil {
			item.UserPreview = turnPreviewText(turn.UserMessage.Content)
		}
		for _, slot := range turn.AgentSlots {
			if strings.TrimSpace(slot.AgentID) != "" {
				item.AgentIDs = append(item.AgentIDs, slot.AgentID)
			}
		}
		items = append(items, item)
	}
	return items
}

func turnPreviewText(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok || stringFromAny(block["type"]) != "text" {
				continue
			}
			if text := strings.TrimSpace(stringFromAny(block["text"])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

// PaginateConversationTurns 按 round 对投影结果分页。turns 必须升序。
func PaginateConversationTurns(
	turns []protocol.ConversationTurn,
	limit int,
	beforeRoundID string,
	aroundRoundID string,
	descending bool,
) protocol.TurnPage {
	pageLimit := normalizeRoundPageLimit(limit)
	page := protocol.TurnPage{Turns: []protocol.ConversationTurn{}}
	if len(turns) == 0 {
		return page
	}

	if aroundRoundID = strings.TrimSpace(aroundRoundID); aroundRoundID != "" {
		target := -1
		for index, turn := range turns {
			if turn.RoundID == aroundRoundID {
				target = index
				break
			}
		}
		if target < 0 {
			return page
		}
		start := max(target-pageLimit, 0)
		end := min(target+pageLimit+1, len(turns))
		page.Turns = append(page.Turns, turns[start:end]...)
		if start > 0 {
			page.NextBeforeRoundID = turns[start].RoundID
		}
		if end < len(turns) {
			page.BackwardsAfterRoundID = turns[end-1].RoundID
		}
	} else {
		end := len(turns)
		if beforeRoundID = strings.TrimSpace(beforeRoundID); beforeRoundID != "" {
			end = 0
			for index, turn := range turns {
				if turn.RoundID == beforeRoundID {
					end = index
					break
				}
			}
		}
		start := max(end-pageLimit, 0)
		page.Turns = append(page.Turns, turns[start:end]...)
		if start > 0 {
			page.NextBeforeRoundID = turns[start].RoundID
		}
	}

	if descending {
		for left, right := 0, len(page.Turns)-1; left < right; left, right = left+1, right-1 {
			page.Turns[left], page.Turns[right] = page.Turns[right], page.Turns[left]
		}
	}
	return page
}
