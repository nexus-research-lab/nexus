// INPUT: Room ledger 追加字节、已发布尾轮原始行和 SQLite 分页代际。
// OUTPUT: 可验证的追加只重投影尾轮及新增轮次，并原子推进读模型进度。
// POS: Room 历史的保守增量路径；已有引用变化、跨轮修正和文件替换回退 canonical 重建。
package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 保留原始尾轮，不能把带 synthetic interrupt 或裁剪 detail 的展示结果作为新输入。
func roomHistoryTailRows(rows []map[string]any, groups []historyPageIndexedGroup, resolved []protocol.Message) []protocol.Message {
	if len(groups) == 0 {
		return nil
	}
	key := groups[len(groups)-1].GroupKey
	start := len(rows)
	for start > 0 && historyPageGroupKey(protocol.Message(rows[start-1]), true) == key {
		start--
	}
	if start == len(rows) {
		return nil
	}
	for _, row := range rows[:start] {
		if historyPageGroupKey(protocol.Message(row), true) == key {
			return nil
		}
	}
	tail := make([]protocol.Message, 0, len(rows)-start)
	for _, row := range rows[start:] {
		if !roomHistoryIncrementalRow(protocol.Message(row)) {
			return nil
		}
		tail = append(tail, protocol.Message(row))
	}
	ids := make(map[string]bool, len(tail))
	for _, row := range tail {
		ids[stringFromAny(row["message_id"])] = true
	}
	resolvedTail := make([]protocol.Message, 0, len(tail))
	for _, row := range resolved {
		if ids[stringFromAny(row["message_id"])] {
			resolvedTail = append(resolvedTail, row)
		}
	}
	projected, err := buildHistoryPageIndexedGroups(context.Background(), resolvedTail, true)
	if err != nil || !reflect.DeepEqual(projected, groups[len(groups)-1:]) {
		return nil
	}
	return tail
}

func roomHistoryIncrementalRow(row protocol.Message) bool {
	role := protocol.MessageRole(row)
	return (role == "user" || role == "assistant" || stringFromAny(row[overlayKindField]) == overlayKindTranscriptRef) && (stringFromAny(row[overlayKindField]) == "" || stringFromAny(row[overlayKindField]) == overlayKindTranscriptRef) &&
		stringFromAny(row["source_round_id"]) == "" && stringFromAny(row["message_id"]) != "" &&
		stringFromAny(row["round_id"]) != "" && messageTimestamp(row) > 0
}

func persistHistoryReadTail(ctx context.Context, tx *sql.Tx, scope, generation string, rows []protocol.Message) error {
	payload, err := marshalHistoryPageJSONBounded(rows, historyReadModelMaxGroupBytes)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	_, err = tx.ExecContext(ctx, `INSERT INTO history_read_tail(scope,generation,rows_json,rows_digest) VALUES(?,?,?,?)
 ON CONFLICT(scope) DO UPDATE SET generation=excluded.generation, rows_json=excluded.rows_json, rows_digest=excluded.rows_digest`, scope, generation, payload, hex.EncodeToString(digest[:]))
	return err
}

func (s *RoomHistoryStore) refreshRoomHistoryTail(ctx context.Context, owner, conversation, scopeKey string) (historyPageIndexBuild, bool) {
	if err := s.tryRefreshRoomHistoryTail(ctx, owner, conversation, scopeKey); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logx.FromContext(ctx).Info("Room 历史增量读取回退重建", "conversation_id", conversation, "error", err)
		}
		return historyPageIndexBuild{}, false
	}
	db, err := s.readModel.database(ctx)
	if err != nil {
		return historyPageIndexBuild{}, false
	}
	scope, ok, err := readHistoryReadModelScope(ctx, db, scopeKey)
	if err != nil || !ok || scope.DisabledReason != "" {
		return historyPageIndexBuild{}, false
	}
	valid, err := s.validateRoomHistoryPageSources(ctx, owner, conversation, scope.Sources)
	if err != nil || !valid {
		return historyPageIndexBuild{}, false
	}
	index, err := readHistoryReadModelRoundIndex(ctx, db, scopeKey)
	return historyPageIndexBuild{Sources: scope.Sources, RoundIndex: index, Cacheable: true}, err == nil
}

func (s *RoomHistoryStore) tryRefreshRoomHistoryTail(ctx context.Context, owner, conversation, scopeKey string) error {
	started := time.Now()
	db, err := s.readModel.database(ctx)
	if err != nil {
		return err
	}
	previous, ok, err := readHistoryReadModelScope(ctx, db, scopeKey)
	if err != nil || !ok || previous.DisabledReason != "" || previous.GroupCount == 0 || len(previous.Sources) == 0 {
		return err
	}
	old := previous.Sources[0]
	current, err := s.snapshotRoomHistoryLedger(ctx, owner, conversation)
	if err != nil || current == old {
		return err
	}
	if !old.Exists || !current.Exists || old.FileIdentity != current.FileIdentity || current.Size <= old.Size || current.Size-old.Size > historyPageSourceMaxBytes {
		return fmt.Errorf("source replaced, truncated or append budget exceeded: %w", errHistoryPageIndexInvalid)
	}
	expected := append([]historyPageSourceSnapshot{current}, previous.Sources[1:]...)
	if valid, checkErr := s.validateRoomHistoryPageSources(ctx, owner, conversation, expected); checkErr != nil || !valid {
		return fmt.Errorf("referenced source changed: %w", errHistoryPageIndexInvalid)
	}
	var payload []byte
	var digest string
	if err = db.QueryRowContext(ctx, `SELECT rows_json,rows_digest FROM history_read_tail WHERE scope=? AND generation=?`, scopeKey, previous.Generation).Scan(&payload, &digest); err != nil {
		return err
	}
	actualDigest := sha256.Sum256(payload)
	if hex.EncodeToString(actualDigest[:]) != digest {
		return errHistoryPageIndexInvalid
	}
	var boundaryTimestamp int64
	if err = db.QueryRowContext(ctx, `SELECT cursor_timestamp FROM history_read_groups WHERE scope=? AND generation=? AND sequence=?`, scopeKey, previous.Generation, previous.GroupCount-1).Scan(&boundaryTimestamp); err != nil {
		return err
	}
	var tail []protocol.Message
	if err = json.Unmarshal(payload, &tail); err != nil || len(tail) == 0 {
		return fmt.Errorf("tail checkpoint unavailable: %w", errHistoryPageIndexInvalid)
	}
	parent, name, err := s.files.openRoomFileParent(owner, s.paths.RoomConversationOverlayPath(owner, conversation), false)
	if err != nil {
		return err
	}
	defer parent.Close()
	file, err := parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, old.Size)
	if err != nil || fingerprint != old.EdgeFingerprint {
		return errHistoryPageIndexInvalid
	}
	// 只接受完整 JSONL 边界；半行或被替换的文件由原有重建路径处理。
	var boundary [1]byte
	if _, err = file.ReadAt(boundary[:], old.Size-1); err != nil || boundary[0] != '\n' {
		return errHistoryPageIndexInvalid
	}
	if _, err = file.ReadAt(boundary[:], current.Size-1); err != nil || boundary[0] != '\n' {
		return errHistoryPageIndexInvalid
	}
	decoder := json.NewDecoder(io.NewSectionReader(file, old.Size, current.Size-old.Size))
	rows := append([]protocol.Message(nil), tail...)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		var row protocol.Message
		err = decoder.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if !roomHistoryIncrementalRow(row) || messageTimestamp(row) < boundaryTimestamp {
			return fmt.Errorf("append requires full normalization: %w", errHistoryPageIndexInvalid)
		}
		rows = append(rows, row)
	}
	raw := make([]map[string]any, len(rows))
	for i, row := range rows {
		raw[i] = map[string]any(row)
	}
	addedSources, err := s.collectRoomHistoryPageDependencySources(ctx, owner, raw)
	if err != nil {
		return err
	}
	byKey := make(map[string]historyPageSourceSnapshot)
	for _, source := range append(append([]historyPageSourceSnapshot(nil), previous.Sources[1:]...), addedSources...) {
		byKey[historyPageSourceKey(source)] = source
	}
	dependencies := make([]historyPageSourceSnapshot, 0, len(byKey))
	for _, source := range byKey {
		dependencies = append(dependencies, source)
	}
	sortHistoryPageSources(dependencies)
	expected = append([]historyPageSourceSnapshot{current}, dependencies...)
	if historyPageSourcesExceedLimit(expected, historyPageSourceMaxBytes) {
		return errHistoryPageIndexResourceLimit
	}
	resolved, err := s.resolveRoomHistoryRowsContext(ctx, owner, conversation, raw)
	if err != nil {
		return err
	}
	groups, err := buildHistoryPageIndexedGroups(ctx, resolved, true)
	if err != nil || len(groups) == 0 {
		return errHistoryPageIndexInvalid
	}
	start := previous.GroupCount - 1
	for _, row := range rows {
		var count int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM history_read_message_keys WHERE scope=? AND generation=? AND message_id=? AND sequence<?`, scopeKey, previous.Generation, stringFromAny(row["message_id"]), start).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errHistoryPageIndexInvalid
		}
	}
	if groups[0].GroupKey != historyPageGroupKey(tail[0], true) {
		return errHistoryPageIndexInvalid
	}
	// 跨到旧轮次的修正不能局部归并，否则会漏掉去重、引导抑制等全局语义。
	for _, group := range groups {
		var count int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM history_read_groups WHERE scope=? AND generation=? AND sequence<? AND (group_key=? OR cursor_timestamp>=?)`, scopeKey, previous.Generation, start, group.GroupKey, group.CursorRoundTimestamp).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errHistoryPageIndexInvalid
		}
	}
	nextTail := roomHistoryTailRows(raw, groups, resolved)
	if len(nextTail) == 0 {
		return errHistoryPageIndexInvalid
	}
	roundItems, err := readHistoryReadModelRoundIndex(ctx, db, scopeKey)
	if err != nil {
		return err
	}
	entries := make(map[string]*sessionRoundIndexAccumulator)
	latest := make(map[string]int)
	for i, row := range rows {
		latest[stringFromAny(row["message_id"])] = i
	}
	for i, row := range rows {
		if latest[stringFromAny(row["message_id"])] != i {
			continue
		}
		data, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return marshalErr
		}
		var item roundIndexOverlayJSONRow
		if err = json.Unmarshal(data, &item); err != nil {
			return err
		}
		item.applyToIndex(entries, nil, true, "")
	}
	mergedIndex := make([]protocol.SessionRoundIndexItem, 0, len(roundItems)+len(entries))
	for _, item := range roundItems {
		if _, replaced := entries[item.RoundID]; !replaced {
			mergedIndex = append(mergedIndex, item)
		}
	}
	mergedIndex = append(mergedIndex, buildSessionRoundIndex(entries).Items...)
	after, err := s.snapshotRoomHistoryLedger(ctx, owner, conversation)
	if err != nil || after != current {
		return errHistoryPageIndexInvalid
	}
	if valid, checkErr := s.validateRoomHistoryPageSources(ctx, owner, conversation, expected); checkErr != nil || !valid {
		return errHistoryPageIndexInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	actual, ok, err := readHistoryReadModelScope(ctx, tx, scopeKey)
	if err != nil || !ok || actual.Generation != previous.Generation || !snapshotsEqual(actual.Sources, previous.Sources) {
		return errHistoryPageIndexInvalid
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_message_keys WHERE scope=? AND generation=? AND sequence>=?`, scopeKey, previous.Generation, start); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_groups WHERE scope=? AND generation=? AND sequence>=?`, scopeKey, previous.Generation, start); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_details WHERE scope=? AND generation=? AND sequence>=?`, scopeKey, previous.Generation, start); err != nil {
		return err
	}
	if start+len(groups) > historyReadModelMaxGroups {
		return errHistoryPageIndexResourceLimit
	}
	if err = insertHistoryReadModelGroups(ctx, tx, scopeKey, previous.Generation, groups, start); err != nil {
		return err
	}
	sources, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	index, err := marshalHistoryPageJSONBounded(mergedIndex, historyReadModelMaxPageBytes)
	if err != nil {
		return err
	}
	if err = persistHistoryReadTail(ctx, tx, scopeKey, previous.Generation, nextTail); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE history_read_scopes SET sources_json=?,round_index_json=?,group_count=? WHERE scope=?`, sources, index, start+len(groups), scopeKey)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	logx.FromContext(ctx).Info("Room 历史增量更新完成", "conversation_id", conversation, "source_bytes", current.Size, "appended_bytes", current.Size-old.Size, "replaced_groups", len(groups), "unchanged_groups", start, "duration_ms", time.Since(started).Milliseconds())
	return nil
}
