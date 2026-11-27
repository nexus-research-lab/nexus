// INPUT: Room 公区游标、目标 Agent 与同源分页读模型。
// OUTPUT: 包含游标之后消息和目标 Agent 最近终态的完整正文历史。
// POS: runtime 公区输入读取；不使用 UI 页大小或截断的 detail 预览。
package workspace

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ReadRuntimeHistoryContext 读取成员唤醒时的公区上下文：游标所在轮次之后的消息，以及目标成员最近一次终态。
// 游标在表里定位不到、或含无法还原的大块内容（图片）时，按 canonical 读取整段公区历史。
func (s *RoomHistoryStore) ReadRuntimeHistoryContext(ctx context.Context, owner, conversation, cursorID, agentID, throughID string, activeRoundIDs []string) (result []protocol.Message, resultErr error) {
	active := normalizeActiveRoundIDs(activeRoundIDs)
	defer func() { result = runtimeHistoryWindow(result, throughID, active) }()
	started := time.Now()
	mode := "canonical"
	defer func() {
		args := []any{"conversation_id", conversation, "agent_id", agentID, "mode", mode, "message_count", len(result), "duration_ms", time.Since(started).Milliseconds(), "err", resultErr}
		if resultErr != nil || time.Since(started) >= 500*time.Millisecond {
			logx.FromContext(ctx).Warn("Room runtime 历史读取", args...)
		} else {
			logx.FromContext(ctx).Debug("Room runtime 历史读取", args...)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cursorID != "" {
		scope, err := s.readyConversationHistory(ctx, owner, conversation)
		if err != nil {
			return nil, err
		}
		rows, ok, err := s.history.readRuntimeWindow(ctx, scope, "", cursorID, agentID, throughID)
		if err != nil {
			return nil, err
		}
		if ok {
			mode = "projection"
			return rows, nil
		}
	}
	rows, err := s.readResolvedRowsContext(ctx, owner, conversation)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, nil), nil
}

// 只修改刚解码的消息。图片解码后不能还原原始 data URI 或 source 形状，交给 canonical。
func restoreRuntimeToolDetails(value any, load func(string) (HistoryMessageDetail, error)) error {
	switch block := value.(type) {
	case []any:
		for _, item := range block {
			if err := restoreRuntimeToolDetails(item, load); err != nil {
				return err
			}
		}
	case map[string]any:
		if block["detail_truncated"] == true {
			if block["type"] != "tool_result" || block["detail_kind"] != historyMessageDetailKindToolResult {
				return ErrHistoryMessageDetailUnavailable
			}
			detail, err := load(stringFromAny(block["detail_ref"]))
			if err != nil {
				return err
			}
			if detail.Kind != historyMessageDetailKindToolResult || detail.ByteSize != protocol.Int64FromAny(block["detail_size"]) {
				return ErrHistoryMessageDetailUnavailable
			}
			var content any
			if err = json.Unmarshal(detail.Payload, &content); err != nil {
				return err
			}
			block["content"] = normalizeDecodedJSONValue(content)
			for _, key := range []string{"detail_ref", "detail_truncated", "detail_kind", "detail_size"} {
				delete(block, key)
			}
			return nil
		}
		for _, key := range []string{"content", "source"} {
			if err := restoreRuntimeToolDetails(block[key], load); err != nil {
				return err
			}
		}
	}
	return nil
}

func runtimeHistoryHasTerminal(groups []historyPageIndexedGroup, agentID string) bool {
	for _, group := range groups {
		for _, row := range group.Items {
			if stringFromAny(row["agent_id"]) != agentID {
				continue
			}
			role := protocol.MessageRole(row)
			complete, _ := row["is_complete"].(bool)
			summary, _ := row["result_summary"].(map[string]any)
			if role == "result" || (role == "assistant" && (complete || len(summary) > 0)) {
				return true
			}
		}
	}
	return false
}

// 当前输入已落盘，但尚未执行，不能把它投影为上次中断；同时隔离稍后排队的输入。
func runtimeHistoryWindow(rows []protocol.Message, throughID string, active map[string]struct{}) []protocol.Message {
	end := len(rows)
	if throughID != "" {
		for i, row := range rows {
			if stringFromAny(row["message_id"]) == throughID {
				end = i + 1
				break
			}
		}
	}
	result := make([]protocol.Message, 0, end)
	for _, row := range rows[:end] {
		if !indexedSyntheticInterruptIsActive(row, active) {
			result = append(result, row)
		}
	}
	return result
}
