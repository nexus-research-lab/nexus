// INPUT: 稳定 canonical source 快照、规范化 physical round groups 与分页请求。
// OUTPUT: 宿主控制面 SQLite/B-Tree 中原子代际化、随历史投影版本重建的分页与大内容 detail 读模型；
// 重建沿用上一代际，只重写签名变化的分组。
// POS: workspace canonical 历史之上的唯一当前派生查询层；数据库可整体删除重建。
package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	_ "modernc.org/sqlite"
)

const (
	historyReadModelSchemaVersion = 11 // runtime 自唤醒的 <task-notification> 投影为触发行，不再显示原始标签。
	historyReadModelFileName      = "history-read-model.v1.sqlite"
	historyReadModelBusyTimeoutMS = 5000
	historyReadModelMaxGroups     = 1_000_000
	historyReadModelMaxGroupBytes = 64 * 1024 * 1024
	historyReadModelMaxPageBytes  = 128 * 1024 * 1024
	historyReadModelMaxDataBytes  = 4 * 1024 * 1024 * 1024
	historyReadModelRetention     = 30 * 24 * time.Hour
)

type historyReadModel struct {
	path string
	mu   sync.Mutex
	db   *sql.DB
	// memo 记录本进程最近一次提交的分组摘要，让未变化的分组跳过投影与序列化。
	memo historyReadModelMemoStore
}

type historyReadModelScope struct {
	Generation       string
	GroupCount       int
	Sources          []historyPageSourceSnapshot
	DisabledReason   string
	DisabledDigest   string
	DisabledCoverage string
}

type historyReadModelQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var historyReadModels sync.Map

func sharedHistoryReadModel(root string) *historyReadModel {
	path := historyReadModelPath(root)
	value, _ := historyReadModels.LoadOrStore(path, &historyReadModel{path: path})
	return value.(*historyReadModel)
}

// historyReadModelPath 只接受宿主配置的 users root。生产布局把派生库放入
// app/cache；显式测试根则留在该临时根内，避免触碰真实用户状态。
func historyReadModelPath(root string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if strings.TrimSpace(root) == "" {
		return filepath.Join(appfs.AppDir(), "cache", historyReadModelFileName)
	}
	if filepath.Base(cleaned) == "users" {
		return filepath.Join(filepath.Dir(cleaned), "app", "cache", historyReadModelFileName)
	}
	return filepath.Join(cleaned, ".nexus-cache", historyReadModelFileName)
}

func (m *historyReadModel) database(ctx context.Context) (*sql.DB, error) {
	if m == nil {
		return nil, errors.New("history read model is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return m.db, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		db, err := openHistoryReadModelDatabase(ctx, m.path)
		if err == nil {
			m.db = db
			return db, nil
		}
		if !historyReadModelDatabaseCorrupt(err) || attempt > 0 {
			return nil, err
		}
		removeHistoryReadModelFiles(m.path)
	}
	return nil, errHistoryPageIndexInvalid
}

func openHistoryReadModelDatabase(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path + fmt.Sprintf(
		"?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate",
		historyReadModelBusyTimeoutMS,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = initializeHistoryReadModel(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func historyReadModelDatabaseCorrupt(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"database disk image is malformed",
		"file is not a database",
		"sqlite_corrupt",
		"sqlite_notadb",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func removeHistoryReadModelFiles(path string) {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Remove(candidate)
	}
}

func (m *historyReadModel) resetCorruptDatabase(current *sql.DB, cause error) {
	if m == nil || current == nil || !historyReadModelDatabaseCorrupt(cause) {
		return
	}
	m.mu.Lock()
	if m.db != current {
		m.mu.Unlock()
		return
	}
	m.db = nil
	_ = current.Close()
	removeHistoryReadModelFiles(m.path)
	m.mu.Unlock()
}

func initializeHistoryReadModel(ctx context.Context, db *sql.DB) error {
	var currentVersion int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read history read model version: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history read model initialization: %w", err)
	}
	defer tx.Rollback()
	if currentVersion != historyReadModelSchemaVersion {
		// 该数据库只有派生数据。版本变化直接丢弃代际，避免长期保留迁移链。
		// version=0 也会清理，使上次初始化中断留下的半张表自愈。
		if _, err = tx.ExecContext(
			ctx,
			`DROP TABLE IF EXISTS history_room_refs;
			 DROP TABLE IF EXISTS history_read_message_keys;
			 DROP TABLE IF EXISTS history_read_tail;
			 DROP TABLE IF EXISTS history_read_details;
			 DROP TABLE IF EXISTS history_read_groups;
			 DROP TABLE IF EXISTS history_read_scopes;`,
		); err != nil {
			return fmt.Errorf("reset history read model: %w", err)
		}
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS history_room_refs (
			scope TEXT NOT NULL, member TEXT NOT NULL, stamp TEXT NOT NULL, message_id TEXT NOT NULL,
			payload BLOB NOT NULL, digest BLOB NOT NULL, updated_at_ms INTEGER NOT NULL,
			PRIMARY KEY(scope,member,message_id)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS history_room_refs_age ON history_room_refs(updated_at_ms)`,
		`CREATE TABLE IF NOT EXISTS history_read_message_keys (scope TEXT NOT NULL, generation TEXT NOT NULL, message_id TEXT NOT NULL, sequence INTEGER NOT NULL, PRIMARY KEY(scope,generation,message_id)) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS history_read_tail (scope TEXT PRIMARY KEY, generation TEXT NOT NULL, rows_json BLOB NOT NULL, frozen_refs_json BLOB NOT NULL, rows_digest TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS history_read_scopes (
			scope TEXT PRIMARY KEY,
			schema_version INTEGER NOT NULL,
			generation TEXT NOT NULL,
			group_count INTEGER NOT NULL,
			sources_json BLOB NOT NULL,
			round_index_json BLOB NOT NULL,
			disabled_reason TEXT NOT NULL DEFAULT '',
			disabled_digest TEXT NOT NULL DEFAULT '',
			disabled_coverage TEXT NOT NULL DEFAULT '',
			accessed_at_ms INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS history_read_groups (
			scope TEXT NOT NULL,
			generation TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			cursor_round_id TEXT NOT NULL,
			cursor_timestamp INTEGER NOT NULL,
			group_key TEXT NOT NULL,
			first_non_synthetic_timestamp INTEGER,
			synthetic_interrupts_json BLOB NOT NULL,
			payload BLOB NOT NULL,
			payload_digest TEXT NOT NULL,
			PRIMARY KEY (scope, generation, sequence)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS history_read_details (
			scope TEXT NOT NULL,
			generation TEXT NOT NULL,
			detail_ref TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			kind TEXT NOT NULL,
			media_type TEXT NOT NULL,
			byte_size INTEGER NOT NULL,
			payload BLOB NOT NULL,
			payload_digest TEXT NOT NULL,
			PRIMARY KEY (scope, generation, detail_ref)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS history_read_message_sequence ON history_read_message_keys(scope,generation,sequence)`,
		`CREATE INDEX IF NOT EXISTS history_read_detail_sequence ON history_read_details(scope,generation,sequence)`,
		`CREATE INDEX IF NOT EXISTS history_read_group_key ON history_read_groups(scope,generation,group_key,sequence)`,
		`CREATE INDEX IF NOT EXISTS history_read_groups_cursor
			ON history_read_groups(scope, generation, cursor_round_id, sequence)`,
		`CREATE INDEX IF NOT EXISTS history_read_groups_time
			ON history_read_groups(scope, generation, cursor_timestamp, sequence)`,
	}
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize history read model: %w", err)
		}
	}
	if _, err = tx.ExecContext(
		ctx,
		fmt.Sprintf(`PRAGMA user_version = %d`, historyReadModelSchemaVersion),
	); err != nil {
		return fmt.Errorf("write history read model version: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit history read model initialization: %w", err)
	}
	return nil
}

func (m *historyReadModel) load(
	ctx context.Context,
	access historyPageIndexAccess,
	request historyPageIndexRequest,
) (protocol.MessagePage, bool, error) {
	db, err := m.database(ctx)
	if err != nil {
		return protocol.MessagePage{}, false, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		m.resetCorruptDatabase(db, err)
		return protocol.MessagePage{}, false, err
	}
	defer tx.Rollback()
	scope, ok, err := readHistoryReadModelScope(ctx, tx, access.Scope)
	if err != nil || !ok {
		_ = tx.Rollback()
		m.resetCorruptDatabase(db, err)
		return protocol.MessagePage{}, false, err
	}
	if scope.DisabledReason != "" {
		_ = tx.Rollback()
		return protocol.MessagePage{}, false, &historyPageIndexDisabledError{
			sourceDigest: scope.DisabledDigest,
			coverage:     scope.DisabledCoverage,
		}
	}
	valid, err := access.ValidateSources(ctx, scope.Sources)
	if err != nil {
		_ = tx.Rollback()
		m.resetCorruptDatabase(db, err)
		return protocol.MessagePage{}, false, err
	}
	if !valid {
		_ = tx.Rollback()
		// 失效代际不会被读取（每次读取先校验 source），保留它供下一次重建复用未变化的分组前缀。
		return protocol.MessagePage{}, false, errHistoryPageIndexInvalid
	}
	metadata, metadataStart, metadataEnd, err := readHistoryReadModelMetadataWindow(
		ctx,
		tx,
		access.Scope,
		scope,
		request,
	)
	if err != nil {
		_ = tx.Rollback()
		if historyReadModelDatabaseCorrupt(err) {
			m.resetCorruptDatabase(db, err)
		} else {
			_ = m.deleteScope(ctx, access.Scope)
		}
		return protocol.MessagePage{}, false, err
	}
	selection := selectHistoryPageIndexGroups(metadata, request)
	adjustHistoryReadModelSelection(
		metadata,
		metadataStart,
		metadataEnd,
		scope.GroupCount,
		request,
		&selection,
	)
	groups, err := readHistoryReadModelGroups(
		ctx,
		tx,
		access.Scope,
		scope.Generation,
		metadata,
		selection.Indexes,
		metadataStart,
	)
	if err != nil {
		_ = tx.Rollback()
		if historyReadModelDatabaseCorrupt(err) {
			m.resetCorruptDatabase(db, err)
		} else {
			_ = m.deleteScope(ctx, access.Scope)
		}
		return protocol.MessagePage{}, false, err
	}
	if err = tx.Commit(); err != nil {
		m.resetCorruptDatabase(db, err)
		return protocol.MessagePage{}, false, err
	}
	_, _ = db.ExecContext(
		ctx,
		`UPDATE history_read_scopes SET accessed_at_ms = ? WHERE scope = ?`,
		time.Now().UnixMilli(),
		access.Scope,
	)
	return materializeHistoryPageSelection(groups, selection, request), true, nil
}

func (m *historyReadModel) loadRoundIndex(
	ctx context.Context,
	access historyPageIndexAccess,
	activeRoundIDs []string,
	collapseRoomRounds bool,
) (protocol.SessionRoundIndex, bool, error) {
	db, err := m.database(ctx)
	if err != nil {
		return protocol.SessionRoundIndex{}, false, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		m.resetCorruptDatabase(db, err)
		return protocol.SessionRoundIndex{}, false, err
	}
	defer tx.Rollback()
	scope, ok, err := readHistoryReadModelScope(ctx, tx, access.Scope)
	if err != nil || !ok {
		_ = tx.Rollback()
		m.resetCorruptDatabase(db, err)
		return protocol.SessionRoundIndex{}, false, err
	}
	if scope.DisabledReason != "" {
		_ = tx.Rollback()
		return protocol.SessionRoundIndex{}, false, &historyPageIndexDisabledError{
			sourceDigest: scope.DisabledDigest,
			coverage:     scope.DisabledCoverage,
		}
	}
	valid, err := access.ValidateSources(ctx, scope.Sources)
	if err != nil {
		return protocol.SessionRoundIndex{}, false, err
	}
	if !valid {
		_ = tx.Rollback()
		// 失效代际不会被读取（每次读取先校验 source），保留它供下一次重建复用未变化的分组前缀。
		return protocol.SessionRoundIndex{}, false, errHistoryPageIndexInvalid
	}
	items, err := readHistoryReadModelRoundIndex(ctx, tx, access.Scope)
	if err != nil {
		_ = tx.Rollback()
		if historyReadModelDatabaseCorrupt(err) {
			m.resetCorruptDatabase(db, err)
		} else {
			_ = m.deleteScope(ctx, access.Scope)
		}
		return protocol.SessionRoundIndex{}, false, err
	}
	if err = tx.Commit(); err != nil {
		m.resetCorruptDatabase(db, err)
		return protocol.SessionRoundIndex{}, false, err
	}
	_, _ = db.ExecContext(
		ctx,
		`UPDATE history_read_scopes SET accessed_at_ms = ? WHERE scope = ?`,
		time.Now().UnixMilli(),
		access.Scope,
	)
	return materializeSessionRoundIndex(
		items,
		activeRoundIDs,
		collapseRoomRounds,
	), true, nil
}

func readHistoryReadModelScope(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
) (historyReadModelScope, bool, error) {
	var value historyReadModelScope
	var version int
	var sourcesJSON []byte
	err := db.QueryRowContext(
		ctx,
		`SELECT schema_version, generation, group_count, sources_json,
			disabled_reason, disabled_digest, disabled_coverage
		 FROM history_read_scopes WHERE scope = ?`,
		scope,
	).Scan(
		&version,
		&value.Generation,
		&value.GroupCount,
		&sourcesJSON,
		&value.DisabledReason,
		&value.DisabledDigest,
		&value.DisabledCoverage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return historyReadModelScope{}, false, nil
	}
	if err != nil {
		return historyReadModelScope{}, false, err
	}
	if version != historyReadModelSchemaVersion || strings.TrimSpace(value.Generation) == "" ||
		value.GroupCount < 0 || value.GroupCount > historyReadModelMaxGroups {
		return historyReadModelScope{}, false, errHistoryPageIndexInvalid
	}
	if err = json.Unmarshal(sourcesJSON, &value.Sources); err != nil {
		return historyReadModelScope{}, false, errHistoryPageIndexInvalid
	}
	if value.Sources == nil {
		value.Sources = []historyPageSourceSnapshot{}
	}
	return value, true, nil
}

func readHistoryReadModelRoundIndex(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
) ([]protocol.SessionRoundIndexItem, error) {
	var payload []byte
	if err := db.QueryRowContext(
		ctx,
		`SELECT round_index_json FROM history_read_scopes WHERE scope = ?`,
		scope,
	).Scan(&payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 || int64(len(payload)) > historyReadModelMaxPageBytes {
		return nil, errHistoryPageIndexResourceLimit
	}
	var items []protocol.SessionRoundIndexItem
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, errHistoryPageIndexInvalid
	}
	if items == nil {
		items = []protocol.SessionRoundIndexItem{}
	}
	return items, nil
}

func readHistoryReadModelMetadataWindow(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
	modelScope historyReadModelScope,
	request historyPageIndexRequest,
) ([]historyPageIndexGroup, int, int, error) {
	total := modelScope.GroupCount
	if total == 0 {
		return []historyPageIndexGroup{}, 0, 0, nil
	}
	// active 会动态隐藏 synthetic interrupt 并可能重新合并 physical groups。
	// 带 cursor 的 active 请求先走完整 metadata oracle；payload 仍只读取选中页。
	// 普通最新页是绝大多数热路径，始终保持 bounded B-Tree window。
	if len(request.ActiveRoundIDs) > 0 && (strings.TrimSpace(request.BeforeRoundID) != "" ||
		request.BeforeRoundTimestamp > 0 ||
		strings.TrimSpace(request.AroundRoundID) != "") {
		items, err := readHistoryReadModelMetadataRange(
			ctx, db, scope, modelScope.Generation, 0, total,
		)
		return items, 0, total, err
	}

	windowSize := max(64, (normalizeRoundPageLimit(request.Limit)+2)*8)
	anchor, found, err := locateHistoryReadModelAnchor(
		ctx, db, scope, modelScope.Generation, total, request,
	)
	if err != nil {
		return nil, 0, 0, err
	}
	if !found {
		items, rangeErr := readHistoryReadModelMetadataRange(
			ctx, db, scope, modelScope.Generation, 0, min(total, 1),
		)
		return items, 0, min(total, 1), rangeErr
	}

	around := strings.TrimSpace(request.AroundRoundID) != ""
	start := max(0, anchor-windowSize)
	end := total
	if around || strings.TrimSpace(request.BeforeRoundID) != "" || request.BeforeRoundTimestamp > 0 {
		end = min(total, anchor+windowSize+1)
	}
	for {
		items, rangeErr := readHistoryReadModelMetadataRange(
			ctx, db, scope, modelScope.Generation, start, end,
		)
		if rangeErr != nil {
			return nil, 0, 0, rangeErr
		}
		selection := selectHistoryPageIndexGroups(items, request)
		selectedLogical := historyReadModelSelectedLogicalCount(items, selection, request)
		if around {
			radius := normalizeRoundAroundLimit(request.AroundLimit)
			targetVisible, leftCount, rightCount := historyReadModelCursorMargins(
				items, request.AroundRoundID, request,
			)
			leftReady := start == 0 || leftCount >= radius
			rightReady := end == total || rightCount >= radius
			if targetVisible && leftReady && rightReady {
				return items, start, end, nil
			}
		} else if selectedLogical >= normalizeRoundPageLimit(request.Limit) || start == 0 {
			return items, start, end, nil
		}
		if start == 0 && end == total {
			return items, start, end, nil
		}
		windowSize *= 2
		start = max(0, anchor-windowSize)
		if around || strings.TrimSpace(request.BeforeRoundID) != "" || request.BeforeRoundTimestamp > 0 {
			end = min(total, anchor+windowSize+1)
		}
	}
}

func locateHistoryReadModelAnchor(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
	generation string,
	total int,
	request historyPageIndexRequest,
) (int, bool, error) {
	aroundRoundID := strings.TrimSpace(request.AroundRoundID)
	beforeRoundID := strings.TrimSpace(request.BeforeRoundID)
	if aroundRoundID != "" || (beforeRoundID != "" && request.BeforeRoundTimestamp <= 0) {
		cursor := aroundRoundID
		if cursor == "" {
			cursor = beforeRoundID
		}
		var sequence sql.NullInt64
		err := db.QueryRowContext(
			ctx,
			`SELECT MIN(sequence) FROM history_read_groups
			 WHERE scope = ? AND generation = ? AND cursor_round_id = ?`,
			scope,
			generation,
			cursor,
		).Scan(&sequence)
		if err != nil {
			return 0, false, err
		}
		if !sequence.Valid {
			return 0, false, nil
		}
		return int(sequence.Int64), true, nil
	}
	if request.BeforeRoundTimestamp > 0 {
		query := `SELECT MIN(sequence) FROM history_read_groups
			WHERE scope = ? AND generation = ? AND cursor_timestamp >= ?`
		arguments := []any{scope, generation, request.BeforeRoundTimestamp}
		if beforeRoundID != "" {
			query = `SELECT MIN(sequence) FROM history_read_groups
				WHERE scope = ? AND generation = ? AND
				(cursor_timestamp > ? OR (cursor_timestamp = ? AND cursor_round_id >= ?))`
			arguments = []any{
				scope, generation,
				request.BeforeRoundTimestamp, request.BeforeRoundTimestamp, beforeRoundID,
			}
		}
		var sequence sql.NullInt64
		if err := db.QueryRowContext(ctx, query, arguments...).Scan(&sequence); err != nil {
			return 0, false, err
		}
		if !sequence.Valid {
			return total, true, nil
		}
		return int(sequence.Int64), true, nil
	}
	return total, true, nil
}

func readHistoryReadModelMetadataRange(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
	generation string,
	start int,
	end int,
) ([]historyPageIndexGroup, error) {
	if start < 0 || end < start {
		return nil, errHistoryPageIndexInvalid
	}
	rows, err := db.QueryContext(
		ctx,
		`SELECT sequence, cursor_round_id, cursor_timestamp, group_key,
			first_non_synthetic_timestamp, synthetic_interrupts_json
		 FROM history_read_groups
		 WHERE scope = ? AND generation = ? AND sequence >= ? AND sequence < ?
		 ORDER BY sequence`,
		scope,
		generation,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]historyPageIndexGroup, 0, end-start)
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var item historyPageIndexGroup
		var sequence int
		var first sql.NullInt64
		var syntheticJSON []byte
		if err = rows.Scan(
			&sequence,
			&item.CursorRoundID,
			&item.CursorRoundTimestamp,
			&item.GroupKey,
			&first,
			&syntheticJSON,
		); err != nil {
			return nil, err
		}
		if sequence != start+len(result) {
			return nil, errHistoryPageIndexInvalid
		}
		if first.Valid {
			value := first.Int64
			item.FirstNonSyntheticTimestamp = &value
		}
		if err = json.Unmarshal(syntheticJSON, &item.SyntheticInterrupts); err != nil {
			return nil, errHistoryPageIndexInvalid
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(result) != end-start {
		return nil, errHistoryPageIndexInvalid
	}
	return result, nil
}

func historyReadModelSelectedLogicalCount(
	metadata []historyPageIndexGroup,
	selection historyPageIndexSelection,
	request historyPageIndexRequest,
) int {
	selected := make([]historyPageIndexGroup, 0, len(selection.Indexes))
	for _, index := range selection.Indexes {
		if index >= 0 && index < len(metadata) {
			selected = append(selected, metadata[index])
		}
	}
	return len(buildVisibleHistoryPageLogicalGroups(
		selected,
		normalizeActiveRoundIDs(request.ActiveRoundIDs),
	))
}

func historyReadModelCursorMargins(
	metadata []historyPageIndexGroup,
	cursor string,
	request historyPageIndexRequest,
) (bool, int, int) {
	logical := buildVisibleHistoryPageLogicalGroups(
		metadata,
		normalizeActiveRoundIDs(request.ActiveRoundIDs),
	)
	for index, group := range logical {
		if group.CursorRoundID == strings.TrimSpace(cursor) {
			return true, index, len(logical) - index - 1
		}
	}
	return false, 0, 0
}

func adjustHistoryReadModelSelection(
	metadata []historyPageIndexGroup,
	windowStart int,
	windowEnd int,
	total int,
	request historyPageIndexRequest,
	selection *historyPageIndexSelection,
) {
	if selection == nil || len(selection.Indexes) == 0 {
		return
	}
	around := strings.TrimSpace(request.AroundRoundID) != ""
	if windowStart > 0 || (around && windowEnd < total) {
		selection.HasMore = true
	}
	if windowStart == 0 || selection.NextBeforeRoundID != nil {
		return
	}
	oldestPhysical := selection.Indexes[0]
	if oldestPhysical < 0 || oldestPhysical >= len(metadata) {
		return
	}
	visible := buildVisibleHistoryPageLogicalGroups(
		metadata[:oldestPhysical+1],
		normalizeActiveRoundIDs(request.ActiveRoundIDs),
	)
	if len(visible) == 0 {
		return
	}
	oldest := visible[len(visible)-1]
	selection.NextBeforeRoundID = stringPointer(oldest.CursorRoundID)
	timestamp := oldest.CursorRoundTimestamp
	selection.NextBeforeRoundTimestamp = &timestamp
}

func readHistoryReadModelGroups(
	ctx context.Context,
	db historyReadModelQuerier,
	scope string,
	generation string,
	metadata []historyPageIndexGroup,
	indexes []int,
	sequenceStart int,
) ([]historyPageIndexedGroup, error) {
	result := make([]historyPageIndexedGroup, 0, len(indexes))
	selectedBytes := int64(0)
	for _, index := range indexes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if index < 0 || index >= len(metadata) {
			return nil, errHistoryPageIndexInvalid
		}
		var payload []byte
		var digest string
		err := db.QueryRowContext(
			ctx,
			`SELECT payload, payload_digest FROM history_read_groups
			 WHERE scope = ? AND generation = ? AND sequence = ?`,
			scope,
			generation,
			sequenceStart+index,
		).Scan(&payload, &digest)
		if err != nil {
			return nil, err
		}
		if len(payload) == 0 || int64(len(payload)) > historyReadModelMaxGroupBytes ||
			int64(len(payload)) > historyReadModelMaxPageBytes-selectedBytes {
			return nil, errHistoryPageIndexResourceLimit
		}
		selectedBytes += int64(len(payload))
		actual := sha256.Sum256(payload)
		if hex.EncodeToString(actual[:]) != digest {
			return nil, errHistoryPageIndexInvalid
		}
		var group historyPageIndexedGroup
		if err = json.Unmarshal(payload, &group); err != nil ||
			!historyPageGroupVisibilityMatches(metadata[index], group) {
			return nil, errHistoryPageIndexInvalid
		}
		for rowIndex, row := range group.Items {
			normalized, ok := normalizeDecodedJSONValue(map[string]any(row)).(map[string]any)
			if !ok {
				return nil, errHistoryPageIndexInvalid
			}
			group.Items[rowIndex] = protocol.Message(normalized)
		}
		result = append(result, group)
	}
	return result, nil
}

func (m *historyReadModel) persist(
	ctx context.Context,
	access historyPageIndexAccess,
	built historyPageIndexBuild,
) error {
	// 派生库位于宿主根，但代际发布仍必须通过 canonical 容器的删除栅栏；
	// session/conversation 已删除时只能放弃缓存，不能留下可被误认的新代际。
	if access.OpenRoot != nil {
		root, openErr := access.OpenRoot(false)
		if openErr != nil {
			return openErr
		}
		root.Close()
	}
	db, err := m.database(ctx)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	sourcesJSON, err := json.Marshal(built.Sources)
	if err != nil {
		return err
	}
	roundIndexJSON, err := marshalHistoryPageJSONBounded(
		built.RoundIndex,
		historyReadModelMaxPageBytes,
	)
	if err != nil {
		return err
	}
	generation := historyReadModelGeneration(access.Scope, built.Sources)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 长会话每轮只改变尾部分组；沿用上一代际并保留签名相同的前缀，
	// 避免每次重建都把全部分组重新写入 B-Tree 和 WAL。
	var reusable []string
	var memo []historyReadModelGroupMemo
	if !built.Disabled {
		previous, ok, readErr := readHistoryReadModelScope(ctx, tx, access.Scope)
		if readErr == nil && ok && previous.DisabledReason == "" && previous.GroupCount > 0 {
			signatures, signatureErr := readHistoryReadModelGroupSignatures(ctx, tx, access.Scope, previous.Generation, previous.GroupCount)
			if signatureErr == nil {
				generation = previous.Generation
				reusable = signatures
				memo = m.memo.load(access.Scope, generation)
			}
		}
	}
	if reusable == nil {
		if err = deleteHistoryReadModelRows(ctx, tx, access.Scope, "", 0); err != nil {
			return err
		}
	} else if err = deleteHistoryReadModelOtherGenerations(ctx, tx, access.Scope, generation); err != nil {
		return err
	}
	disabledReason := ""
	disabledDigest := ""
	disabledCoverage := ""
	var written []historyReadModelGroupMemo
	if built.Disabled {
		disabledReason = historyPageIndexDisabledResourceLimit
		disabledCoverage = built.DisabledCoverage
		if disabledCoverage == "" {
			disabledCoverage = historyPageDisabledCoverageAll
		}
		disabledDigest = historyPageSourceDigestForCoverage(built.Sources, disabledCoverage)
	} else {
		if len(built.Groups) > historyReadModelMaxGroups {
			return errHistoryPageIndexResourceLimit
		}
		if written, err = writeHistoryReadModelGroupsMemo(ctx, tx, access.Scope, generation, built.Groups, 0, reusable, memo, true); err != nil {
			return err
		}
	}
	if err = persistHistoryReadTail(ctx, tx, access.Scope, generation, built.TailRows, built.FrozenRefs); err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO history_read_scopes(
			scope, schema_version, generation, group_count, sources_json, round_index_json,
			disabled_reason, disabled_digest, disabled_coverage, accessed_at_ms
		 ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(scope) DO UPDATE SET
			schema_version = excluded.schema_version,
			generation = excluded.generation,
			group_count = excluded.group_count,
			sources_json = excluded.sources_json,
			round_index_json = excluded.round_index_json,
			disabled_reason = excluded.disabled_reason,
			disabled_digest = excluded.disabled_digest,
			disabled_coverage = excluded.disabled_coverage,
			accessed_at_ms = excluded.accessed_at_ms`,
		access.Scope,
		historyReadModelSchemaVersion,
		generation,
		len(built.Groups),
		sourcesJSON,
		roundIndexJSON,
		disabledReason,
		disabledDigest,
		disabledCoverage,
		time.Now().UnixMilli(),
	)
	if err != nil {
		return err
	}
	// 已拿到 SQLite 写锁；目录若在首次检查后删除，与删除方的 scope 清理串行收口。
	if access.OpenRoot != nil {
		root, openErr := access.OpenRoot(false)
		if openErr != nil {
			return openErr
		}
		root.Close()
	}
	// 提交前先撤掉旧备忘：提交结果不确定时宁可下一次走慢路径。
	m.memo.forget(access.Scope)
	if err = tx.Commit(); err != nil {
		return err
	}
	if written != nil {
		m.memo.store(access.Scope, generation, written)
	}
	_ = m.evictColdScopes(ctx, time.Now().Add(-historyReadModelRetention))
	return nil
}

func insertHistoryReadModelGroups(
	ctx context.Context,
	tx *sql.Tx,
	scope string,
	generation string,
	groups []historyPageIndexedGroup,
	startSequence ...int,
) error {
	start := 0
	if len(startSequence) > 0 {
		start = startSequence[0]
	}
	_, err := writeHistoryReadModelGroupsMemo(ctx, tx, scope, generation, groups, start, nil, nil, false)
	return err
}

// writeHistoryReadModelGroupsMemo 从 start 开始写入分组。reusable 非 nil 时它是同一代际中
// 已有分组的签名（下标即 sequence）：签名相同的前缀原样保留，第一个不同之处及其后全部重写。
// 资源预算仍按全部分组累计，与完整写入保持同一上限语义。
//
// memo 是本进程上次写入同一代际时的分组摘要：结构哈希与备忘一致、且库内签名仍等于
// 备忘签名的分组，直接沿用而不再投影和序列化。summarize 为真（或 reusable 非 nil）时
// 返回本次每个分组的摘要，供提交成功后更新备忘。
func writeHistoryReadModelGroupsMemo(
	ctx context.Context,
	tx *sql.Tx,
	scope string,
	generation string,
	groups []historyPageIndexedGroup,
	start int,
	reusable []string,
	memo []historyReadModelGroupMemo,
	summarize bool,
) ([]historyReadModelGroupMemo, error) {
	statement, err := tx.PrepareContext(
		ctx,
		`INSERT INTO history_read_groups(
			scope, generation, sequence, cursor_round_id, cursor_timestamp,
			group_key, first_non_synthetic_timestamp, synthetic_interrupts_json,
			payload, payload_digest
		 ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return nil, err
	}
	defer statement.Close()
	detailStatement, err := tx.PrepareContext(
		ctx,
		`INSERT OR REPLACE INTO history_read_details(
			scope, generation, detail_ref, sequence, kind, media_type,
			byte_size, payload, payload_digest
		 ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return nil, err
	}
	defer detailStatement.Close()
	keyStatement, err := tx.PrepareContext(
		ctx,
		`INSERT OR IGNORE INTO history_read_message_keys(scope,generation,message_id,sequence) VALUES(?,?,?,?)`,
	)
	if err != nil {
		return nil, err
	}
	defer keyStatement.Close()
	preserving := reusable != nil
	summarize = summarize || reusable != nil
	var summaries []historyReadModelGroupMemo
	if summarize {
		summaries = make([]historyReadModelGroupMemo, 0, len(groups))
	}
	totalBytes := int64(0)
	for offset, group := range groups {
		index := start + offset
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var hash uint64
		if summarize {
			hash = historyReadModelGroupHash(scope, generation, index, group)
		}
		if preserving && index < len(memo) && index < len(reusable) &&
			memo[index].Hash == hash && memo[index].Signature == reusable[index] {
			// 同一内容在同一代际、同一 sequence 上必然得到同一签名，而库内仍是该签名。
			if memo[index].Bytes > historyReadModelMaxDataBytes-totalBytes {
				return nil, errHistoryPageIndexResourceLimit
			}
			totalBytes += memo[index].Bytes
			summaries = append(summaries, memo[index])
			continue
		}
		projected, details, projectionErr := projectHistoryReadModelGroup(
			scope,
			generation,
			index,
			group,
		)
		if projectionErr != nil {
			return nil, projectionErr
		}
		payload, marshalErr := marshalHistoryPageJSONBounded(projected, historyReadModelMaxGroupBytes)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if int64(len(payload)) > historyReadModelMaxDataBytes-totalBytes {
			return nil, errHistoryPageIndexResourceLimit
		}
		totalBytes += int64(len(payload))
		for _, detail := range details {
			if int64(len(detail.Payload)) > historyReadModelMaxDataBytes-totalBytes {
				return nil, errHistoryPageIndexResourceLimit
			}
			totalBytes += int64(len(detail.Payload))
		}
		metadata := historyPageIndexMetadataForGroup(group)
		syntheticJSON, marshalErr := json.Marshal(metadata.SyntheticInterrupts)
		if marshalErr != nil {
			return nil, marshalErr
		}
		var first any
		if metadata.FirstNonSyntheticTimestamp != nil {
			first = *metadata.FirstNonSyntheticTimestamp
		}
		digest := sha256.Sum256(payload)
		payloadDigest := hex.EncodeToString(digest[:])
		if summarize {
			groupBytes := int64(len(payload))
			detailDigests := make([]historyReadModelDetailDigest, 0, len(details))
			for _, detail := range details {
				groupBytes += int64(len(detail.Payload))
				detailDigests = append(detailDigests, historyReadModelDetailDigest{Ref: detail.Ref, Digest: detail.Digest})
			}
			signature := historyReadModelGroupSignature(
				metadata.CursorRoundID,
				metadata.CursorRoundTimestamp,
				metadata.GroupKey,
				metadata.FirstNonSyntheticTimestamp,
				syntheticJSON,
				payloadDigest,
				detailDigests,
			)
			summaries = append(summaries, historyReadModelGroupMemo{Hash: hash, Signature: signature, Bytes: groupBytes})
			if preserving && index < len(reusable) && reusable[index] == signature {
				continue
			}
		}
		if preserving {
			preserving = false
			if err = deleteHistoryReadModelRows(ctx, tx, scope, generation, index); err != nil {
				return nil, err
			}
		}
		if _, err = statement.ExecContext(
			ctx,
			scope,
			generation,
			index,
			metadata.CursorRoundID,
			metadata.CursorRoundTimestamp,
			metadata.GroupKey,
			first,
			syntheticJSON,
			payload,
			payloadDigest,
		); err != nil {
			return nil, err
		}
		for _, row := range group.Items {
			if id := stringFromAny(row["message_id"]); id != "" {
				if _, err = keyStatement.ExecContext(ctx, scope, generation, id, index); err != nil {
					return nil, err
				}
			}
		}
		for _, detail := range details {
			if _, err = detailStatement.ExecContext(
				ctx,
				scope,
				generation,
				detail.Ref,
				index,
				detail.Kind,
				detail.MediaType,
				len(detail.Payload),
				detail.Payload,
				detail.Digest,
			); err != nil {
				return nil, err
			}
		}
	}
	// 新结果是旧前缀时，删掉多出的旧尾部分组。
	if preserving && start+len(groups) < len(reusable) {
		if err = deleteHistoryReadModelRows(ctx, tx, scope, generation, start+len(groups)); err != nil {
			return nil, err
		}
	}
	return summaries, nil
}

type historyReadModelDetailDigest struct {
	Ref    string
	Digest string
}

// historyReadModelGroupSignature 覆盖一行分组被读取时可见的全部字段：分页元数据、
// 正文摘要以及按引用排序的 detail 摘要。正文只保存 detail 引用，必须单独比较 detail 内容。
func historyReadModelGroupSignature(
	cursorRoundID string,
	cursorTimestamp int64,
	groupKey string,
	firstNonSynthetic *int64,
	syntheticJSON []byte,
	payloadDigest string,
	details []historyReadModelDetailDigest,
) string {
	sort.Slice(details, func(left, right int) bool { return details[left].Ref < details[right].Ref })
	hasher := sha256.New()
	first := "null"
	if firstNonSynthetic != nil {
		first = strconv.FormatInt(*firstNonSynthetic, 10)
	}
	_, _ = fmt.Fprintf(hasher, "%d:%s|%d|%d:%s|%s|%d:%s|%s|", len(cursorRoundID), cursorRoundID, cursorTimestamp, len(groupKey), groupKey, first, len(syntheticJSON), syntheticJSON, payloadDigest)
	for _, detail := range details {
		_, _ = fmt.Fprintf(hasher, "%d:%s=%s;", len(detail.Ref), detail.Ref, detail.Digest)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// readHistoryReadModelGroupSignatures 只读取摘要列，不加载正文与 detail 字节。
// 行数与 group_count 不一致时返回错误，调用方退回完整重写。
func readHistoryReadModelGroupSignatures(
	ctx context.Context,
	tx *sql.Tx,
	scope string,
	generation string,
	count int,
) ([]string, error) {
	details := make(map[int][]historyReadModelDetailDigest)
	detailRows, err := tx.QueryContext(ctx, `SELECT sequence, detail_ref, payload_digest FROM history_read_details WHERE scope=? AND generation=?`, scope, generation)
	if err != nil {
		return nil, err
	}
	for detailRows.Next() {
		var sequence int
		var detail historyReadModelDetailDigest
		if err = detailRows.Scan(&sequence, &detail.Ref, &detail.Digest); err != nil {
			detailRows.Close()
			return nil, err
		}
		details[sequence] = append(details[sequence], detail)
	}
	if err = errors.Join(detailRows.Err(), detailRows.Close()); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT sequence, cursor_round_id, cursor_timestamp, group_key, first_non_synthetic_timestamp, synthetic_interrupts_json, payload_digest
		 FROM history_read_groups WHERE scope=? AND generation=? ORDER BY sequence`, scope, generation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	signatures := make([]string, 0, count)
	for rows.Next() {
		var sequence int
		var cursorRoundID, groupKey, payloadDigest string
		var cursorTimestamp int64
		var first sql.NullInt64
		var syntheticJSON []byte
		if err = rows.Scan(&sequence, &cursorRoundID, &cursorTimestamp, &groupKey, &first, &syntheticJSON, &payloadDigest); err != nil {
			return nil, err
		}
		if sequence != len(signatures) {
			return nil, errHistoryPageIndexInvalid
		}
		var firstValue *int64
		if first.Valid {
			firstValue = &first.Int64
		}
		signatures = append(signatures, historyReadModelGroupSignature(cursorRoundID, cursorTimestamp, groupKey, firstValue, syntheticJSON, payloadDigest, details[sequence]))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(signatures) != count {
		return nil, errHistoryPageIndexInvalid
	}
	return signatures, nil
}

// deleteHistoryReadModelRows 删除 scope 的分组行；generation 为空时删除所有代际。
func deleteHistoryReadModelRows(ctx context.Context, tx *sql.Tx, scope string, generation string, fromSequence int) error {
	for _, table := range []string{"history_read_message_keys", "history_read_groups", "history_read_details"} {
		var err error
		if generation == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE scope = ?`, scope)
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE scope = ? AND generation = ? AND sequence >= ?`, scope, generation, fromSequence)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteHistoryReadModelOtherGenerations(ctx context.Context, tx *sql.Tx, scope string, generation string) error {
	for _, table := range []string{"history_read_message_keys", "history_read_groups", "history_read_details"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE scope = ? AND generation <> ?`, scope, generation); err != nil {
			return err
		}
	}
	return nil
}

func historyReadModelGeneration(scope string, sources []historyPageSourceSnapshot) string {
	digest := sha256.Sum256([]byte(scope + "\x00" + historyPageSourceDigest(sources)))
	return hex.EncodeToString(digest[:16])
}

func (m *historyReadModel) deleteScope(ctx context.Context, scope string) error {
	m.memo.forget(scope)
	db, err := m.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_room_refs WHERE scope=?`, scope); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_details WHERE scope = ?`, scope); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_groups WHERE scope = ?`, scope); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_message_keys WHERE scope = ?`, scope); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_tail WHERE scope = ?`, scope); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_scopes WHERE scope = ?`, scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *historyReadModel) evictColdScopes(ctx context.Context, cutoff time.Time) error {
	db, err := m.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_room_refs WHERE updated_at_ms < ?`, cutoff.UnixMilli()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(
		ctx,
		`DELETE FROM history_read_details
		 WHERE scope IN (SELECT scope FROM history_read_scopes WHERE accessed_at_ms < ?)`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(
		ctx,
		`DELETE FROM history_read_groups
		 WHERE scope IN (SELECT scope FROM history_read_scopes WHERE accessed_at_ms < ?)`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(
		ctx,
		`DELETE FROM history_read_message_keys WHERE scope IN (SELECT scope FROM history_read_scopes WHERE accessed_at_ms < ?)`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`DELETE FROM history_read_tail WHERE scope IN (SELECT scope FROM history_read_scopes WHERE accessed_at_ms < ?)`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`DELETE FROM history_read_scopes WHERE accessed_at_ms < ?`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	return tx.Commit()
}
