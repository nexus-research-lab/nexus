// INPUT: 同一历史页内的 assistant 快照、result 行与 Agent 执行身份。
// OUTPUT: result 只挂到同 root round 的对应 agent round，未匹配结果转合成 assistant。
// POS: compact 后历史行到前端 assistant 终态的唯一 result 配对入口。
package message

import (
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// MergeHistoryResultSummaries 按物理 Agent round 配对结果，返回独立副本；不修改输入历史。
func MergeHistoryResultSummaries(rows []protocol.Message) []protocol.Message {
	if len(rows) == 0 {
		return rows
	}
	merger := newRoundResultSummaryMerger(rows)
	merger.attachMatchingResults()
	return merger.buildResultRows()
}

type roundResultSummaryMerger struct {
	rows                   []protocol.Message
	assistantByAgentRound  map[agentRoundSummaryKey]resultSummaryTarget
	assistantByAgent       map[agentSummaryKey]resultSummaryTarget
	legacyAssistantByAgent map[agentSummaryKey]resultSummaryTarget
	lastAssistantByRound   map[string]resultSummaryTarget
	mergedResultMessageIDs map[string]struct{}
}

type agentRoundSummaryKey struct {
	roundID      string
	agentRoundID string
}

type agentSummaryKey struct {
	roundID string
	agentID string
}

type resultSummaryTarget struct {
	index         int
	assistantText string
}

func newRoundResultSummaryMerger(rows []protocol.Message) *roundResultSummaryMerger {
	merger := &roundResultSummaryMerger{
		rows:                   cloneResultHistoryRows(rows),
		assistantByAgentRound:  make(map[agentRoundSummaryKey]resultSummaryTarget),
		assistantByAgent:       make(map[agentSummaryKey]resultSummaryTarget),
		legacyAssistantByAgent: make(map[agentSummaryKey]resultSummaryTarget),
		lastAssistantByRound:   make(map[string]resultSummaryTarget),
		mergedResultMessageIDs: make(map[string]struct{}),
	}
	merger.indexAssistants()
	return merger
}

func (m *roundResultSummaryMerger) indexAssistants() {
	for index, row := range m.rows {
		if protocol.MessageRole(row) != "assistant" {
			continue
		}
		roundID := historyIdentity(row["round_id"])
		if roundID == "" {
			continue
		}
		target := resultSummaryTarget{
			index:         index,
			assistantText: ExtractAssistantDisplayText(row),
		}
		m.lastAssistantByRound[roundID] = target
		agentID := historyIdentity(row["agent_id"])
		agentRoundID := historyIdentity(row["agent_round_id"])
		if agentRoundID != "" {
			m.assistantByAgentRound[agentRoundSummaryKey{
				roundID:      roundID,
				agentRoundID: agentRoundID,
			}] = target
		}
		if agentID != "" {
			key := agentSummaryKey{roundID: roundID, agentID: agentID}
			m.assistantByAgent[key] = target
			if agentRoundID == "" {
				m.legacyAssistantByAgent[key] = target
			}
		}
	}
}

func (m *roundResultSummaryMerger) attachMatchingResults() {
	for _, row := range m.rows {
		if protocol.MessageRole(row) != "result" {
			continue
		}
		target, hasAssistant := m.matchingAssistant(row)
		if !hasAssistant {
			continue
		}

		assistant := protocol.Clone(m.rows[target.index])
		summary := BuildAssistantResultSummary(row, target.assistantText)
		if len(summary) == 0 {
			continue
		}
		assistant["result_summary"] = summary
		m.rows[target.index] = assistant
		if messageID := historyIdentity(row["message_id"]); messageID != "" {
			m.mergedResultMessageIDs[messageID] = struct{}{}
		}
	}
}

func (m *roundResultSummaryMerger) matchingAssistant(result protocol.Message) (resultSummaryTarget, bool) {
	roundID := historyIdentity(result["round_id"])
	if roundID == "" {
		return resultSummaryTarget{}, false
	}
	agentID := historyIdentity(result["agent_id"])
	if agentRoundID := historyIdentity(result["agent_round_id"]); agentRoundID != "" {
		if target, ok := m.assistantByAgentRound[agentRoundSummaryKey{
			roundID:      roundID,
			agentRoundID: agentRoundID,
		}]; ok {
			return target, true
		}
		// 旧 transcript_ref 没有 agent_round_id；只允许同 Agent 的 legacy
		// assistant 兜底，不能退化到 root round 后误挂给另一个 Agent。
		if agentID != "" {
			target, ok := m.legacyAssistantByAgent[agentSummaryKey{
				roundID: roundID,
				agentID: agentID,
			}]
			return target, ok
		}
		return resultSummaryTarget{}, false
	}
	if agentID != "" {
		target, ok := m.assistantByAgent[agentSummaryKey{
			roundID: roundID,
			agentID: agentID,
		}]
		return target, ok
	}
	target, ok := m.lastAssistantByRound[roundID]
	return target, ok
}

func (m *roundResultSummaryMerger) buildResultRows() []protocol.Message {
	result := make([]protocol.Message, 0, len(m.rows))
	for _, row := range m.rows {
		if protocol.MessageRole(row) == "result" {
			if _, merged := m.mergedResultMessageIDs[historyIdentity(row["message_id"])]; merged {
				continue
			}
			result = append(result, BuildSyntheticAssistantFromResult(row))
			continue
		}
		result = append(result, row)
	}
	return result
}
func cloneResultHistoryRows(rows []protocol.Message) []protocol.Message {
	cloned := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		cloned = append(cloned, protocol.Clone(row))
	}
	return cloned
}

func historyIdentity(value any) string {
	if number, ok := value.(json.Number); ok {
		return number.String()
	}
	return normalizeString(value)
}
