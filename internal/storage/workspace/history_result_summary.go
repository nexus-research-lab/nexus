package workspace

import (
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func mergeRoundResultSummaries(rows []protocol.Message) []protocol.Message {
	if len(rows) == 0 {
		return rows
	}
	merger := newRoundResultSummaryMerger(rows)
	merger.attachMatchingResults()
	return merger.buildResultRows()
}

type roundResultSummaryMerger struct {
	rows                        []protocol.Message
	lastAssistantIndexByRoundID map[string]int
	assistantTextByRoundID      map[string]string
	mergedResultMessageIDs      map[string]struct{}
}

func newRoundResultSummaryMerger(rows []protocol.Message) *roundResultSummaryMerger {
	merger := &roundResultSummaryMerger{
		rows:                        cloneHistoryRows(rows),
		lastAssistantIndexByRoundID: make(map[string]int),
		assistantTextByRoundID:      make(map[string]string),
		mergedResultMessageIDs:      make(map[string]struct{}),
	}
	merger.indexAssistants()
	return merger
}

func cloneHistoryRows(rows []protocol.Message) []protocol.Message {
	cloned := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		cloned = append(cloned, protocol.Clone(row))
	}
	return cloned
}

func (m *roundResultSummaryMerger) indexAssistants() {
	for index, row := range m.rows {
		if protocol.MessageRole(row) != "assistant" {
			continue
		}
		roundID := stringFromAny(row["round_id"])
		if roundID == "" {
			continue
		}
		m.lastAssistantIndexByRoundID[roundID] = index
		if assistantText := message.ExtractAssistantDisplayText(row); assistantText != "" {
			m.assistantTextByRoundID[roundID] = assistantText
		}
	}
}

func (m *roundResultSummaryMerger) attachMatchingResults() {
	for _, row := range m.rows {
		if protocol.MessageRole(row) != "result" {
			continue
		}
		roundID := stringFromAny(row["round_id"])
		assistantIndex, hasAssistant := m.lastAssistantIndexByRoundID[roundID]
		if !hasAssistant {
			continue
		}

		assistant := protocol.Clone(m.rows[assistantIndex])
		summary := message.BuildAssistantResultSummary(row, m.assistantTextByRoundID[roundID])
		if len(summary) == 0 {
			continue
		}
		assistant["result_summary"] = summary
		m.rows[assistantIndex] = assistant
		if messageID := stringFromAny(row["message_id"]); messageID != "" {
			m.mergedResultMessageIDs[messageID] = struct{}{}
		}
	}
}

func (m *roundResultSummaryMerger) buildResultRows() []protocol.Message {
	result := make([]protocol.Message, 0, len(m.rows))
	for _, row := range m.rows {
		if protocol.MessageRole(row) == "result" {
			if _, merged := m.mergedResultMessageIDs[stringFromAny(row["message_id"])]; merged {
				continue
			}
			result = append(result, message.BuildSyntheticAssistantFromResult(row))
			continue
		}
		result = append(result, row)
	}
	return result
}
