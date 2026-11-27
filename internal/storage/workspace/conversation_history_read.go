// INPUT: 写时投影表里按根轮次落表的消息骨架、轮次导航条目与 detail。
// OUTPUT: 与旧读模型同语义的消息分页、轮次导航、detail 展开和成员唤醒时的公区上下文窗口。
// POS: 写时投影读模型的读取层：只查表、按页规范化，不读 ledger/transcript，也不触发任何构建。
package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type conversationHistoryRoundMeta struct {
	key    string
	cursor string
	first  int64
}

// beginConversationHistoryRead 打开只读事务并确认会话已就绪；identity 为空时不比对来源身份。
// 读事务只负责取行，解码与规范化都在提交之后做，避免长读快照挡住 WAL checkpoint。
func (h *conversationHistoryStore) beginConversationHistoryRead(
	ctx context.Context,
	scope string,
	identity string,
) (*sql.Tx, bool, error) {
	db, err := h.database(ctx)
	if err != nil {
		return nil, false, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, false, err
	}
	state, ok, err := readConversationHistoryState(ctx, tx, scope)
	if err != nil || !ok || !state.ready() || (identity != "" && state.Identity != identity) {
		_ = tx.Rollback()
		return nil, false, err
	}
	return tx, true, nil
}

func readConversationHistoryRounds(ctx context.Context, q conversationHistoryQuerier, scope string) ([]conversationHistoryRoundMeta, error) {
	rows, err := q.QueryContext(
		ctx,
		`SELECT round_key, cursor_round_id, first_timestamp FROM conversation_history_rounds
		 WHERE scope = ? ORDER BY first_timestamp, first_seq`,
		scope,
	)
	if err != nil {
		return nil, err
	}
	result := make([]conversationHistoryRoundMeta, 0)
	for rows.Next() {
		var item conversationHistoryRoundMeta
		if err = rows.Scan(&item.key, &item.cursor, &item.first); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result = append(result, item)
	}
	return result, errors.Join(rows.Err(), rows.Close())
}

// loadConversationHistoryRoundRows 取出若干轮次的全部行（含投递回执），并去掉已被 durable 引导用户行
// 覆盖的 guided_input 系统行：这是规范化里唯一跨轮次的规则，靠一次 guided_source 查询完成。
func loadConversationHistoryRoundRows(
	ctx context.Context,
	q conversationHistoryQuerier,
	scope string,
	roundKeys []string,
) ([]protocol.Message, error) {
	// 按首次出现顺序取行：规范化是稳定排序，同毫秒的行要靠这个顺序与 canonical 一致。
	encodedKeys, err := json.Marshal(roundKeys)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(
		ctx,
		`SELECT payload FROM conversation_history_rows
		 WHERE scope = ? AND round_key IN (SELECT value FROM json_each(?)) ORDER BY seq`,
		scope, string(encodedKeys),
	)
	if err != nil {
		return nil, err
	}
	payloads := make([][]byte, 0)
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			_ = rows.Close()
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	result := make([]protocol.Message, 0, len(payloads))
	covered := make(map[string]bool)
	for _, payload := range payloads {
		row, err := decodeConversationHistoryPayload(payload)
		if err != nil {
			return nil, err
		}
		if source := conversationHistoryGuidedInputSource(row); source != "" {
			hidden, known := covered[source]
			if !known {
				var exists int
				err = q.QueryRowContext(
					ctx,
					`SELECT 1 FROM conversation_history_rows WHERE scope = ? AND guided_source = ? LIMIT 1`,
					scope, source,
				).Scan(&exists)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
				hidden = err == nil
				covered[source] = hidden
			}
			if hidden {
				continue
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func conversationHistoryGuidedInputSource(row protocol.Message) string {
	if stringFromAny(row["role"]) != "system" {
		return ""
	}
	metadata, _ := row["metadata"].(map[string]any)
	if stringFromAny(metadata["subtype"]) != message.SystemMessageSubtypeGuidedInput {
		return ""
	}
	return strings.TrimSpace(stringFromAny(metadata["source_round_id"]))
}

// readPage 返回一页完整轮次。分页语义（limit、before 游标、around 跳转、has_more）直接复用旧读模型的
// selectHistoryPageIndexGroups：一个根轮次就是一个分组，游标是 (first_timestamp, cursor_round_id)。
// ok=false 表示会话没有就绪，调用方回退旧路径。
func (h *conversationHistoryStore) readPage(
	ctx context.Context,
	scope conversationHistoryScope,
	identity string,
	request historyPageIndexRequest,
) (protocol.MessagePage, bool, error) {
	tx, ok, err := h.beginConversationHistoryRead(ctx, scope.key, identity)
	if err != nil || !ok {
		return protocol.MessagePage{}, false, err
	}
	defer tx.Rollback()
	// 第一版读出会话全部轮次的游标再选页，以便原样复用旧读模型的分页语义；1600 轮时不到 1ms。
	// 单会话到上万轮时再改成按游标 LIMIT 查询。
	rounds, err := readConversationHistoryRounds(ctx, tx, scope.key)
	if err != nil {
		return protocol.MessagePage{}, false, err
	}
	metadata := make([]historyPageIndexGroup, len(rounds))
	for index, round := range rounds {
		first := round.first
		metadata[index] = historyPageIndexGroup{
			CursorRoundID:              round.cursor,
			CursorRoundTimestamp:       round.first,
			GroupKey:                   round.key,
			FirstNonSyntheticTimestamp: &first,
		}
	}
	selection := selectHistoryPageIndexGroups(metadata, request)
	keys := make([]string, 0, len(selection.Indexes))
	for _, index := range selection.Indexes {
		if index >= 0 && index < len(rounds) {
			keys = append(keys, rounds[index].key)
		}
	}
	rows, err := loadConversationHistoryRoundRows(ctx, tx, scope.key, keys)
	if err != nil {
		return protocol.MessagePage{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return protocol.MessagePage{}, false, err
	}
	normalized := normalizeHistoryRows(rows, normalizeActiveRoundIDs(request.ActiveRoundIDs))
	items := make([]protocol.Message, 0, len(normalized))
	for _, row := range normalized {
		items = append(items, normalizeHistoryPageRow(row, request.CollapseRoomRounds))
	}
	return protocol.MessagePage{
		Items:                    items,
		HasMore:                  selection.HasMore,
		NextBeforeRoundID:        selection.NextBeforeRoundID,
		NextBeforeRoundTimestamp: selection.NextBeforeRoundTimestamp,
	}, true, nil
}

// readRoundIndex 返回轮次导航；条目在写入时按轮重算，这里只排序并标记正在运行的轮次。
func (h *conversationHistoryStore) readRoundIndex(
	ctx context.Context,
	scope conversationHistoryScope,
	identity string,
	activeRoundIDs []string,
) (protocol.SessionRoundIndex, bool, error) {
	tx, ok, err := h.beginConversationHistoryRead(ctx, scope.key, identity)
	if err != nil || !ok {
		return protocol.SessionRoundIndex{}, false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(
		ctx,
		`SELECT index_item FROM conversation_history_rounds WHERE scope = ? AND length(index_item) > 0`,
		scope.key,
	)
	if err != nil {
		return protocol.SessionRoundIndex{}, false, err
	}
	payloads := make([][]byte, 0)
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			_ = rows.Close()
			return protocol.SessionRoundIndex{}, false, err
		}
		payloads = append(payloads, payload)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return protocol.SessionRoundIndex{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return protocol.SessionRoundIndex{}, false, err
	}
	items := make([]protocol.SessionRoundIndexItem, 0, len(payloads))
	for _, payload := range payloads {
		var item protocol.SessionRoundIndexItem
		if err = json.Unmarshal(payload, &item); err != nil {
			return protocol.SessionRoundIndex{}, false, err
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(left, right int) bool { return roundIndexItemBefore(items[left], items[right]) })
	return materializeSessionRoundIndex(items, activeRoundIDs, scope.collapse), true, nil
}

// readDetail 展开大块内容；ref 不在表里时 ok=false，调用方回退旧读模型（页面可能来自切换前的旧路径）。
func (h *conversationHistoryStore) readDetail(
	ctx context.Context,
	scope conversationHistoryScope,
	identity string,
	ref string,
) (HistoryMessageDetail, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return HistoryMessageDetail{}, false, nil
	}
	tx, ok, err := h.beginConversationHistoryRead(ctx, scope.key, identity)
	if err != nil || !ok {
		return HistoryMessageDetail{}, false, err
	}
	defer tx.Rollback()
	stored, err := fetchConversationHistoryDetail(ctx, tx, scope.key, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return HistoryMessageDetail{}, false, nil
	}
	if err != nil {
		return HistoryMessageDetail{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return HistoryMessageDetail{}, false, err
	}
	// 解压与摘要校验在事务外做：读快照只持有到取完压缩数据为止。
	detail, err := stored.decode(historyMessageDetailMaxBytes)
	if err != nil {
		return HistoryMessageDetail{}, false, err
	}
	return detail, true, nil
}

// conversationHistoryStoredDetail 是事务内取出的压缩 detail，decode 在事务外解压并校验。
type conversationHistoryStoredDetail struct {
	detail     HistoryMessageDetail
	digest     string
	compressed []byte
}

func fetchConversationHistoryDetail(
	ctx context.Context,
	q conversationHistoryQuerier,
	scope string,
	ref string,
) (conversationHistoryStoredDetail, error) {
	stored := conversationHistoryStoredDetail{detail: HistoryMessageDetail{Ref: ref}}
	err := q.QueryRowContext(
		ctx,
		`SELECT kind, media_type, byte_size, payload_digest, payload FROM conversation_history_details
		 WHERE scope = ? AND detail_ref = ?`,
		scope, ref,
	).Scan(&stored.detail.Kind, &stored.detail.MediaType, &stored.detail.ByteSize, &stored.digest, &stored.compressed)
	return stored, err
}

func (stored conversationHistoryStoredDetail) decode(budget int64) (HistoryMessageDetail, error) {
	detail := stored.detail
	if detail.ByteSize <= 0 || detail.ByteSize > budget || detail.ByteSize > historyMessageDetailMaxBytes {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	payload, err := gunzipConversationHistoryPayload(stored.compressed, detail.ByteSize)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	actual := sha256.Sum256(payload)
	if int64(len(payload)) != detail.ByteSize || hex.EncodeToString(actual[:]) != stored.digest {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	detail.Payload = payload
	return detail, nil
}

// readRuntimeWindow 是成员唤醒时的公区上下文：cursorID 所在轮次到 throughID 所在轮次的全部消息，
// 缺目标成员最近终态时向前逐轮补到找到为止；大块工具输出恢复成原文。调用方再按 throughID 与
// active 集合裁剪（runtimeHistoryWindow）。找不到游标时 ok=false，调用方回退 canonical。
func (h *conversationHistoryStore) readRuntimeWindow(
	ctx context.Context,
	scope conversationHistoryScope,
	identity string,
	cursorID string,
	agentID string,
	throughID string,
) ([]protocol.Message, bool, error) {
	tx, ok, err := h.beginConversationHistoryRead(ctx, scope.key, identity)
	if err != nil || !ok {
		return nil, false, err
	}
	defer tx.Rollback()
	rounds, err := readConversationHistoryRounds(ctx, tx, scope.key)
	if err != nil {
		return nil, false, err
	}
	positions := make(map[string]int, len(rounds))
	for index, round := range rounds {
		positions[round.key] = index
	}
	locate := func(messageID string) (int, bool, error) {
		var key string
		err := tx.QueryRowContext(
			ctx,
			`SELECT round_key FROM conversation_history_rows WHERE scope = ? AND message_id = ? AND kind = ?`,
			scope.key, strings.TrimSpace(messageID), conversationHistoryKindMessage,
		).Scan(&key)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		position, found := positions[key]
		return position, found, nil
	}
	start, found, err := locate(cursorID)
	if err != nil || !found {
		return nil, false, err
	}
	end := len(rounds) - 1
	if strings.TrimSpace(throughID) != "" {
		if end, found, err = locate(throughID); err != nil || !found {
			return nil, false, err
		}
	}
	if end < start {
		return nil, false, nil
	}
	keys := make([]string, 0, end-start+1)
	for _, round := range rounds[start : end+1] {
		keys = append(keys, round.key)
	}
	collected, err := loadConversationHistoryRoundRows(ctx, tx, scope.key, keys)
	if err != nil {
		return nil, false, err
	}
	// 补目标成员最近终态时逐轮判断；行是骨架（大块内容已拆出），这里的规范化代价很小。
	hasTerminal := func(rows []protocol.Message) bool {
		normalized := normalizeHistoryRows(cloneHistoryRows(rows), nil)
		return runtimeHistoryHasTerminal([]historyPageIndexedGroup{{Items: normalized}}, agentID)
	}
	if agentID != "" && !hasTerminal(collected) {
		for before := start - 1; before >= 0; before-- {
			previous, err := loadConversationHistoryRoundRows(ctx, tx, scope.key, []string{rounds[before].key})
			if err != nil {
				return nil, false, err
			}
			collected = append(previous, collected...)
			if hasTerminal(previous) {
				break
			}
		}
	}
	// 读快照内只取压缩的 detail；解压、摘要校验与最终规范化都在提交之后做，避免长读快照挡住 checkpoint。
	refs := make(map[string]struct{})
	for _, row := range collected {
		collectConversationHistoryDetailRefs(row["content"], refs)
	}
	stored := make(map[string]conversationHistoryStoredDetail, len(refs))
	total := int64(0)
	for ref := range refs {
		item, fetchErr := fetchConversationHistoryDetail(ctx, tx, scope.key, ref)
		if errors.Is(fetchErr, sql.ErrNoRows) {
			return nil, false, nil
		}
		if fetchErr != nil {
			return nil, false, fetchErr
		}
		if total += item.detail.ByteSize; total > conversationHistoryRuntimeMaxBytes {
			// 与旧路径相同的上限：超出时交给 canonical 读取。
			return nil, false, nil
		}
		stored[ref] = item
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	normalized := normalizeHistoryRows(collected, nil)
	for _, row := range normalized {
		if err = restoreRuntimeToolDetails(row["content"], func(ref string) (HistoryMessageDetail, error) {
			item, found := stored[ref]
			if !found {
				return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
			}
			return item.decode(item.detail.ByteSize)
		}); err != nil {
			if errors.Is(err, ErrHistoryMessageDetailUnavailable) {
				// 图片等无法还原原始形状的内容交给 canonical。
				return nil, false, nil
			}
			return nil, false, err
		}
	}
	return normalized, true, nil
}
