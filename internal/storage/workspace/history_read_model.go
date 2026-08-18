// INPUT: 稳定 canonical source 快照、规范化 physical round groups 与分页请求。
// OUTPUT: 宿主控制面 SQLite/B-Tree 中原子代际化、可校验且可淘汰的历史读模型。
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
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	_ "modernc.org/sqlite"
)

const (
	historyReadModelSchemaVersion = 3
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
			`DROP TABLE IF EXISTS history_read_groups;
			 DROP TABLE IF EXISTS history_read_scopes;`,
		); err != nil {
			return fmt.Errorf("reset history read model: %w", err)
		}
	}
	statements := []string{
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
		_ = m.deleteScope(ctx, access.Scope)
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
		_ = m.deleteScope(ctx, access.Scope)
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
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_groups WHERE scope = ?`, access.Scope); err != nil {
		return err
	}
	disabledReason := ""
	disabledDigest := ""
	disabledCoverage := ""
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
		if err = insertHistoryReadModelGroups(ctx, tx, access.Scope, generation, built.Groups); err != nil {
			return err
		}
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
	if err = tx.Commit(); err != nil {
		return err
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
) error {
	statement, err := tx.PrepareContext(
		ctx,
		`INSERT INTO history_read_groups(
			scope, generation, sequence, cursor_round_id, cursor_timestamp,
			group_key, first_non_synthetic_timestamp, synthetic_interrupts_json,
			payload, payload_digest
		 ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer statement.Close()
	totalBytes := int64(0)
	for index, group := range groups {
		if err = ctx.Err(); err != nil {
			return err
		}
		payload, marshalErr := marshalHistoryPageJSONBounded(group, historyReadModelMaxGroupBytes)
		if marshalErr != nil {
			return marshalErr
		}
		if int64(len(payload)) > historyReadModelMaxDataBytes-totalBytes {
			return errHistoryPageIndexResourceLimit
		}
		totalBytes += int64(len(payload))
		metadata := historyPageIndexMetadataForGroup(group)
		syntheticJSON, marshalErr := json.Marshal(metadata.SyntheticInterrupts)
		if marshalErr != nil {
			return marshalErr
		}
		var first any
		if metadata.FirstNonSyntheticTimestamp != nil {
			first = *metadata.FirstNonSyntheticTimestamp
		}
		digest := sha256.Sum256(payload)
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
			hex.EncodeToString(digest[:]),
		); err != nil {
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
	db, err := m.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM history_read_groups WHERE scope = ?`, scope); err != nil {
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
		`DELETE FROM history_read_scopes WHERE accessed_at_ms < ?`,
		cutoff.UnixMilli(),
	); err != nil {
		return err
	}
	return tx.Commit()
}
