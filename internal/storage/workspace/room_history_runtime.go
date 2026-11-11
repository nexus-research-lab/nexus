// INPUT: Room 公区游标、目标 Agent 与同源分页读模型。
// OUTPUT: 包含游标之后消息和目标 Agent 最近终态的完整正文历史。
// POS: runtime 公区输入读取；不使用 UI 页大小或截断的 detail 预览。
package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ReadRuntimeHistoryContext 在游标可定位时复用读模型，只读取未消费区间与恢复所需的终态。
// 普通工具详情在同一快照中恢复原文；游标失效、图片格式或详情无法还原时从 canonical 读取。
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
		access := s.historyPageAccess(owner, conversation)
		rows, err := s.readRuntimeHistoryIndex(ctx, access, cursorID, agentID, throughID, active)
		if err == nil {
			mode = "cursor"
			return rows, nil
		}
		if errors.Is(err, errHistoryPageIndexInvalid) {
			built, _, buildErr := awaitHistoryPageIndexBuild(ctx, access, false)
			if buildErr == nil && !built.Disabled {
				if built.Groups != nil {
					// 首次重建已持有完整正文，直接复用，避免刚建完又扫描一次文件。
					mode = "rebuilt"
					rows = make([]protocol.Message, 0)
					for _, group := range built.Groups {
						rows = append(rows, group.Items...)
					}
					return rows, nil
				}
				if rows, err = s.readRuntimeHistoryIndex(ctx, access, cursorID, agentID, throughID, active); err == nil {
					mode = "cursor"
					return rows, nil
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := s.readResolvedRowsContext(ctx, owner, conversation)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, nil), nil
}

func (s *RoomHistoryStore) readRuntimeHistoryIndex(ctx context.Context, access historyPageIndexAccess, cursorID, agentID, throughID string, active map[string]struct{}) ([]protocol.Message, error) {
	db, err := s.readModel.database(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	scope, ok, err := readHistoryReadModelScope(ctx, tx, access.Scope)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errHistoryPageIndexInvalid
	}
	if scope.DisabledReason != "" {
		return nil, errHistoryPageIndexResourceLimit
	}
	valid, err := access.ValidateSources(ctx, scope.Sources)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errHistoryPageIndexInvalid
	}
	var start int
	err = tx.QueryRowContext(ctx, `SELECT sequence FROM history_read_message_keys WHERE scope=? AND generation=? AND message_id=?`, access.Scope, scope.Generation, cursorID).Scan(&start)
	if err != nil {
		return nil, err
	}
	end := scope.GroupCount
	if throughID != "" {
		var sequence int
		if err := tx.QueryRowContext(ctx, `SELECT sequence FROM history_read_message_keys WHERE scope=? AND generation=? AND message_id=?`, access.Scope, scope.Generation, throughID).Scan(&sequence); err != nil {
			return nil, err
		}
		end = sequence + 1
	}
	groups, err := readRuntimeHistoryGroups(ctx, tx, access.Scope, scope.Generation, start, end)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		groups[i].Items = runtimeHistoryWindow(groups[i].Items, throughID, active)
	}
	// 最近终态可能已消费，但下一轮恢复说明仍需要它；向前查到该 Agent 的终态即停止。
	prefix := make([]historyPageIndexedGroup, 0)
	needsTerminal := agentID != "" && !runtimeHistoryHasTerminal(groups, agentID)
	for before := start - 1; needsTerminal && before >= 0; before-- {
		previous, readErr := readRuntimeHistoryGroups(ctx, tx, access.Scope, scope.Generation, before, before+1)
		if readErr != nil {
			return nil, readErr
		}
		previous[0].Items = runtimeHistoryWindow(previous[0].Items, "", active)
		prefix = append(prefix, previous[0])
		if runtimeHistoryHasTerminal(previous, agentID) {
			break
		}
	}
	rows := make([]protocol.Message, 0)
	for index := len(prefix) - 1; index >= 0; index-- {
		rows = append(rows, prefix[index].Items...)
	}
	for _, group := range groups {
		rows = append(rows, group.Items...)
	}
	valid, err = access.ValidateSources(ctx, scope.Sources)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errHistoryPageIndexInvalid
	}
	return rows, tx.Commit()
}

func readRuntimeHistoryGroups(ctx context.Context, tx *sql.Tx, scope, generation string, start, end int) ([]historyPageIndexedGroup, error) {
	metadata, err := readHistoryReadModelMetadataRange(ctx, tx, scope, generation, start, end)
	if err != nil {
		return nil, err
	}
	indexes := make([]int, len(metadata))
	for index := range indexes {
		indexes[index] = index
	}
	groups, err := readHistoryReadModelGroups(ctx, tx, scope, generation, metadata, indexes, start)
	if err != nil {
		return nil, err
	}
	remaining := int64(historyReadModelMaxPageBytes)
	for _, group := range groups {
		for _, row := range group.Items {
			if err = restoreRuntimeToolDetails(row["content"], func(ref string) (HistoryMessageDetail, error) {
				detail, err := readHistoryDetail(ctx, tx, scope, generation, ref, remaining)
				remaining -= detail.ByteSize
				return detail, err
			}); err != nil {
				return nil, err
			}
		}
	}
	return groups, nil
}

// 只修改本事务刚解码的消息。图片解码后不能还原原始 data URI 或 source 形状，交给 canonical。
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
