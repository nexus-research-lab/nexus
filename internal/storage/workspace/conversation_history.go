// INPUT: Room 公区与 DM 会话写入时已解析的消息行、回填时的 canonical 行、改写与删除指令。
// OUTPUT: 按根轮次落表的消息骨架、轮次导航条目与 gzip 压缩的大块 detail。
// POS: 历史读模型的唯一存储层（research/room-history-rebuild/design.md）：写入时落表，读取只查表。
// 表是派生数据，Room ledger、DM overlay 与 transcript 仍是事实来源；会话未就绪或失配时由读取方同步整会话回填。
package workspace

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	conversationHistoryFileName       = "conversation-history.v1.sqlite"
	conversationHistoryLegacyFileName = "history-read-model.v1.sqlite"
	conversationHistorySchemaVersion  = 2
	// conversationHistoryLiveSeq 传给 applyRows 表示实时写入；负值是回填批次首行的顺序号。
	conversationHistoryLiveSeq       int64 = 0
	conversationHistoryBatchRows           = 500
	conversationHistoryJobTimeout          = 30 * time.Minute
	conversationHistoryRetryMin            = 5 * time.Second
	conversationHistoryRetryMax            = 5 * time.Minute
	conversationHistoryBusyTimeoutMS       = 5000
	// conversationHistoryRuntimeMaxBytes 是成员唤醒上下文里还原的大块内容总量上限，超出时交给 canonical 读取。
	conversationHistoryRuntimeMaxBytes = 128 * 1024 * 1024
	// conversationHistoryVerifyRounds 是进程内第一次读取会话时检查的最近轮次数：其中有未收口的轮次、且与上次
	// 回填记下的指纹不同，说明上次进程可能在 runtime 写完 transcript、表还没提交时退出，整会话回填一次。
	conversationHistoryVerifyRounds = 3

	conversationHistoryStatusBackfilling = "backfilling"
	conversationHistoryStatusReady       = "ready"
	conversationHistoryStatusBroken      = "broken"

	// 行类型：普通消息参与轮次分组与导航；投递回执只随目标轮次一起读出，交给规范化合并。
	conversationHistoryKindMessage = ""
	conversationHistoryKindReceipt = "receipt"
)

var (
	errConversationHistoryLedgerChanged   = errors.New("room ledger was replaced or truncated after projection")
	errConversationHistoryBackfillAborted = errors.New("conversation history backfill was superseded")
	errConversationHistoryNotReady        = errors.New("conversation history is not ready after backfill")
)

// retryableConversationHistoryErr 是回填被并发写入或来源变化打断：不计入失败退避，读取方立刻重来。
func retryableConversationHistoryErr(err error) bool {
	return errors.Is(err, errConversationHistoryBackfillAborted) || errors.Is(err, errConversationHistoryLedgerChanged)
}

// conversationHistoryScope 是一个会话在表里的身份与投影规则。
type conversationHistoryScope struct {
	key string
	// collapse 为 Room 公区：成员的 agent round 合并到根轮次。
	collapse bool
	// defaultAgentID 是导航条目里 assistant/result 缺 agent_id 时的默认值（DM 用会话的 agent）。
	defaultAgentID string
}

type conversationHistoryState struct {
	Status string
	// Identity 是回填时的来源身份：Room 为 ledger 文件身份，DM 为 agent、分段与 transcript 会话 ID。
	Identity string
	// LedgerOffset 只用于 Room：已经落表的 ledger 字节位置。
	LedgerOffset      int64
	ProjectionVersion int
	// VerifiedUnfinished 是上次整会话回填时最近几轮里未收口轮次的指纹（轮次 + 最新行顺序号）。
	// 崩溃留下、永远不会收口的轮次指纹不变，重启后不再触发回填；轮次之后又有落表写入时指纹变化，重新校验。
	VerifiedUnfinished string
}

// accepting 表示实时写入应当落表：回填进行中也要写，回填按「表里已有的行更新」合并。
func (state conversationHistoryState) accepting() bool {
	return state.Status == conversationHistoryStatusReady || state.Status == conversationHistoryStatusBackfilling
}

func (state conversationHistoryState) ready() bool {
	return state.Status == conversationHistoryStatusReady && state.ProjectionVersion == message.ProjectionVersion
}

type conversationHistoryQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type conversationHistoryAttempt struct {
	at       time.Time
	failures int
	err      error
}

type conversationHistoryFlight struct {
	done chan struct{}
	err  error
}

type conversationHistoryStore struct {
	path  string
	mu    sync.Mutex
	db    *sql.DB
	locks sync.Map
	// verified 记录本进程已检查过「最近轮次是否未收口」的会话，见 conversationHistoryVerifyRounds。
	verified sync.Map

	jobsMu   sync.Mutex
	inflight map[string]*conversationHistoryFlight
	attempts map[string]conversationHistoryAttempt
}

var conversationHistoryStores sync.Map

// conversationHistoryForRoot 返回 users root 对应的历史库（进程内共享）。生产布局放在 app/cache；
// 显式测试根留在该临时根内，避免触碰真实用户状态。
func conversationHistoryForRoot(root string) *conversationHistoryStore {
	path := conversationHistoryPath(root)
	value, _ := conversationHistoryStores.LoadOrStore(path, &conversationHistoryStore{
		path:     path,
		inflight: make(map[string]*conversationHistoryFlight),
		attempts: make(map[string]conversationHistoryAttempt),
	})
	return value.(*conversationHistoryStore)
}

func conversationHistoryPath(root string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if strings.TrimSpace(root) == "" {
		return filepath.Join(appfs.AppDir(), "cache", conversationHistoryFileName)
	}
	if filepath.Base(cleaned) == "users" {
		return filepath.Join(filepath.Dir(cleaned), "app", "cache", conversationHistoryFileName)
	}
	return filepath.Join(cleaned, ".nexus-cache", conversationHistoryFileName)
}

func conversationHistoryDatabaseCorrupt(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"database disk image is malformed", "file is not a database", "sqlite_corrupt", "sqlite_notadb"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func removeConversationHistoryFiles(path string) {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Remove(candidate)
	}
}

// lock 串行化同一会话的落表、回填阶段切换与改写；不同会话互不阻塞。
func (h *conversationHistoryStore) lock(scope string) func() {
	value, _ := h.locks.LoadOrStore(scope, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (h *conversationHistoryStore) database(ctx context.Context) (*sql.DB, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db != nil {
		return h.db, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return nil, err
	}
	// 旧版按读取现算的派生读模型已经不用，首次打开时顺手删掉。
	removeConversationHistoryFiles(filepath.Join(filepath.Dir(h.path), conversationHistoryLegacyFileName))
	for attempt := 0; attempt < 2; attempt++ {
		db, err := openConversationHistoryDatabase(ctx, h.path)
		if err == nil {
			h.db = db
			return db, nil
		}
		if !conversationHistoryDatabaseCorrupt(err) || attempt > 0 {
			return nil, err
		}
		removeConversationHistoryFiles(h.path)
	}
	return nil, errConversationHistoryNotReady
}

func openConversationHistoryDatabase(ctx context.Context, path string) (*sql.DB, error) {
	// auto_vacuum 只在建表前生效，所以放在新库里设置；删除会话后空闲时按页归还空间。
	dsn := path + fmt.Sprintf(
		"?_pragma=busy_timeout(%d)&_pragma=auto_vacuum(INCREMENTAL)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate",
		conversationHistoryBusyTimeoutMS,
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
	if err = initializeConversationHistory(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func initializeConversationHistory(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read conversation history version: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if version != conversationHistorySchemaVersion {
		// 表只有派生数据；结构变化直接丢弃，各会话按需重新回填。
		if _, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS conversation_history_rows;
			DROP TABLE IF EXISTS conversation_history_rounds;
			DROP TABLE IF EXISTS conversation_history_details;
			DROP TABLE IF EXISTS conversation_history_state;`); err != nil {
			return fmt.Errorf("reset conversation history: %w", err)
		}
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS conversation_history_state (
			scope TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			identity TEXT NOT NULL,
			ledger_offset INTEGER NOT NULL,
			projection_version INTEGER NOT NULL,
			verified_unfinished TEXT NOT NULL,
			updated_at_ms INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_history_rows (
			scope TEXT NOT NULL,
			message_id TEXT NOT NULL,
			round_key TEXT NOT NULL,
			kind TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			guided_source TEXT NOT NULL,
			seq INTEGER NOT NULL,
			payload BLOB NOT NULL,
			PRIMARY KEY (scope, message_id)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS conversation_history_rows_round ON conversation_history_rows(scope, round_key, seq)`,
		`CREATE INDEX IF NOT EXISTS conversation_history_rows_seq ON conversation_history_rows(scope, seq)`,
		`CREATE INDEX IF NOT EXISTS conversation_history_rows_guided ON conversation_history_rows(scope, guided_source) WHERE guided_source <> ''`,
		`CREATE TABLE IF NOT EXISTS conversation_history_rounds (
			scope TEXT NOT NULL,
			round_key TEXT NOT NULL,
			cursor_round_id TEXT NOT NULL,
			first_timestamp INTEGER NOT NULL,
			first_seq INTEGER NOT NULL,
			index_item BLOB NOT NULL,
			PRIMARY KEY (scope, round_key)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS conversation_history_rounds_page ON conversation_history_rounds(scope, first_timestamp, first_seq)`,
		`CREATE TABLE IF NOT EXISTS conversation_history_details (
			scope TEXT NOT NULL,
			detail_ref TEXT NOT NULL,
			message_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			media_type TEXT NOT NULL,
			byte_size INTEGER NOT NULL,
			payload_digest TEXT NOT NULL,
			payload BLOB NOT NULL,
			PRIMARY KEY (scope, detail_ref)
		) WITHOUT ROWID`,
		`CREATE INDEX IF NOT EXISTS conversation_history_details_message ON conversation_history_details(scope, message_id)`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize conversation history: %w", err)
		}
	}
	if version != conversationHistorySchemaVersion {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, conversationHistorySchemaVersion)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func readConversationHistoryState(ctx context.Context, q conversationHistoryQuerier, scope string) (conversationHistoryState, bool, error) {
	var state conversationHistoryState
	err := q.QueryRowContext(
		ctx,
		`SELECT status, identity, ledger_offset, projection_version, verified_unfinished
		 FROM conversation_history_state WHERE scope = ?`,
		scope,
	).Scan(&state.Status, &state.Identity, &state.LedgerOffset, &state.ProjectionVersion, &state.VerifiedUnfinished)
	if errors.Is(err, sql.ErrNoRows) {
		return conversationHistoryState{}, false, nil
	}
	if err != nil {
		return conversationHistoryState{}, false, err
	}
	return state, true, nil
}

func writeConversationHistoryState(ctx context.Context, q conversationHistoryQuerier, scope string, state conversationHistoryState) error {
	_, err := q.ExecContext(
		ctx,
		`INSERT INTO conversation_history_state (scope, status, identity, ledger_offset, projection_version, verified_unfinished, updated_at_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(scope) DO UPDATE SET status = excluded.status, identity = excluded.identity,
		   ledger_offset = excluded.ledger_offset, projection_version = excluded.projection_version,
		   verified_unfinished = excluded.verified_unfinished, updated_at_ms = excluded.updated_at_ms`,
		scope, state.Status, state.Identity, state.LedgerOffset, state.ProjectionVersion, state.VerifiedUnfinished,
		time.Now().UnixMilli(),
	)
	return err
}

func (h *conversationHistoryStore) state(ctx context.Context, scope string) (conversationHistoryState, bool, error) {
	db, err := h.database(ctx)
	if err != nil {
		return conversationHistoryState{}, false, err
	}
	return readConversationHistoryState(ctx, db, scope)
}

// markBroken 让读取回退旧路径并等待整会话回填；失败只记日志，下一次读取会重试。
func (h *conversationHistoryStore) markBroken(ctx context.Context, scope string) {
	db, err := h.database(ctx)
	if err == nil {
		_, err = db.ExecContext(
			ctx,
			`UPDATE conversation_history_state SET status = ?, updated_at_ms = ? WHERE scope = ?`,
			conversationHistoryStatusBroken, time.Now().UnixMilli(), scope,
		)
	}
	if err != nil {
		logx.FromContext(ctx).Warn("会话历史标记失效失败", "scope", conversationHistoryScopeLabel(scope), "err", err)
	}
}

// resetScope 清空会话的表数据并进入回填状态；调用方持有会话锁。
func (h *conversationHistoryStore) resetScope(ctx context.Context, scope, identity string, offset int64) error {
	db, err := h.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = clearConversationHistoryScopeTx(ctx, tx, scope, false); err != nil {
		return err
	}
	if err = writeConversationHistoryState(ctx, tx, scope, conversationHistoryState{
		Status:            conversationHistoryStatusBackfilling,
		Identity:          identity,
		LedgerOffset:      offset,
		ProjectionVersion: message.ProjectionVersion,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// finishBackfill 把回填中的会话切到 ready；回填期间被标记失效或来源变化时放弃。调用方持有会话锁。
func (h *conversationHistoryStore) finishBackfill(
	ctx context.Context,
	scope string,
	validate func(conversationHistoryState) error,
) error {
	db, err := h.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	state, ok, err := readConversationHistoryState(ctx, tx, scope)
	if err != nil {
		return err
	}
	if !ok || state.Status != conversationHistoryStatusBackfilling {
		return errConversationHistoryBackfillAborted
	}
	if validate != nil {
		if err = validate(state); err != nil {
			return err
		}
	}
	state.Status = conversationHistoryStatusReady
	if state.VerifiedUnfinished, err = conversationHistoryUnfinishedFingerprint(ctx, tx, scope); err != nil {
		return err
	}
	if err = writeConversationHistoryState(ctx, tx, scope, state); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// 整会话回填直接来自 canonical，本身就是本进程对该会话的校验；不记下的话，下一次读取会因为
	// 最近轮次仍未收口（例如运行中的轮次）再次判定失效，形成「回填 → 校验失效 → 回填」的循环。
	h.verified.Store(scope, struct{}{})
	h.checkpoint(ctx)
	return nil
}

func clearConversationHistoryScopeTx(ctx context.Context, tx *sql.Tx, scope string, includeState bool) error {
	tables := []string{"conversation_history_rows", "conversation_history_rounds", "conversation_history_details"}
	if includeState {
		tables = append(tables, "conversation_history_state")
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE scope = ?`, scope); err != nil {
			return err
		}
	}
	return nil
}

// deleteScope 在会话删除时一次删掉四张表里的数据，避免孤儿行；随后按页归还空间。
func (h *conversationHistoryStore) deleteScope(ctx context.Context, scope string) error {
	db, err := h.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = clearConversationHistoryScopeTx(ctx, tx, scope, true); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `PRAGMA incremental_vacuum(256)`)
	return nil
}

// removeRounds 删除被改写裁掉的轮次。回填进行中无法确认快照是否已包含这次改写，直接标记失效重来。
// 调用方持有会话锁。
func (h *conversationHistoryStore) removeRounds(ctx context.Context, scope conversationHistoryScope, roundKeys []string) error {
	if len(roundKeys) == 0 {
		return nil
	}
	db, err := h.database(ctx)
	if err != nil {
		return err
	}
	state, ok, err := readConversationHistoryState(ctx, db, scope.key)
	if err != nil || !ok {
		return err
	}
	if state.Status == conversationHistoryStatusBackfilling {
		h.markBroken(ctx, scope.key)
		return nil
	}
	if state.Status != conversationHistoryStatusReady {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, key := range roundKeys {
		if _, err = tx.ExecContext(
			ctx,
			`DELETE FROM conversation_history_details WHERE scope = ? AND message_id IN
			   (SELECT message_id FROM conversation_history_rows WHERE scope = ? AND round_key = ?)`,
			scope.key, scope.key, key,
		); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM conversation_history_rows WHERE scope = ? AND round_key = ?`, scope.key, key); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM conversation_history_rounds WHERE scope = ? AND round_key = ?`, scope.key, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// applyRows 在一个事务里落表一批行，after 在提交前执行（推进 ledger 偏移或校验回填状态）。
// existingIsNewer 用于回填：表里已有的行是回填开始后实时写入的，比回填读到的快照新。
func (h *conversationHistoryStore) applyRows(
	ctx context.Context,
	scope conversationHistoryScope,
	rows []protocol.Message,
	backfillSeq int64,
	after func(*sql.Tx) error,
) error {
	db, err := h.database(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = applyConversationHistoryRowsTx(ctx, tx, scope, rows, backfillSeq); err != nil {
		return err
	}
	if after != nil {
		if err = after(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// applyLiveRows 落表实时写入；会话还没回填或已失效时跳过，事实来源里已有这些行，回填时一并处理。
// 调用方持有会话锁。
func (h *conversationHistoryStore) applyLiveRows(ctx context.Context, scope conversationHistoryScope, rows []protocol.Message) error {
	state, ok, err := h.state(ctx, scope.key)
	if err != nil || !ok || !state.accepting() {
		return err
	}
	return h.applyRows(ctx, scope, rows, conversationHistoryLiveSeq, nil)
}

// applyBackfillRows 分批落表回填行，每批一个短事务，避免长时间占住写锁挡住实时写入。
// rows 必须已按 compactMessages 合并（每个 message_id 只出现一次）。
func (h *conversationHistoryStore) applyBackfillRows(ctx context.Context, scope conversationHistoryScope, rows []protocol.Message) error {
	for start := 0; start < len(rows); start += conversationHistoryBatchRows {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+conversationHistoryBatchRows, len(rows))
		// 回填行按 canonical 顺序取负顺序号：回填期间先落表的实时行顺序号为正，排在整段历史之后。
		err := h.applyRows(ctx, scope, rows[start:end], int64(start-len(rows)), func(tx *sql.Tx) error {
			state, ok, err := readConversationHistoryState(ctx, tx, scope.key)
			if err != nil {
				return err
			}
			if !ok || state.Status != conversationHistoryStatusBackfilling {
				return errConversationHistoryBackfillAborted
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// compactConversationHistoryBackfillRows 按 canonical 读取的顺序预合并回填行：先去掉读路径会过滤的内部行，
// 再按 message_id 合并，与 normalizeHistoryRows 里的 filterInternalHistoryRows + compactMessages 同序。
func compactConversationHistoryBackfillRows(rows []protocol.Message) []protocol.Message {
	kept := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if _, store := conversationHistoryRowKind(row); store {
			kept = append(kept, row)
		}
	}
	return compactMessages(kept)
}

func applyConversationHistoryRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	scope conversationHistoryScope,
	rows []protocol.Message,
	backfillSeq int64,
) error {
	backfill := backfillSeq < 0
	var liveSeq int64
	if !backfill {
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COALESCE(MAX(seq), 0) FROM conversation_history_rows WHERE scope = ?`,
			scope.key,
		).Scan(&liveSeq); err != nil {
			return err
		}
		liveSeq = max(liveSeq, 0)
	}
	touched := make(map[string]struct{})
	for index, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		kind, store := conversationHistoryRowKind(row)
		if !store {
			continue
		}
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		next := protocol.Clone(row)
		var previousKey string
		var previousPayload []byte
		var seq int64
		err := tx.QueryRowContext(
			ctx,
			`SELECT round_key, payload, seq FROM conversation_history_rows WHERE scope = ? AND message_id = ?`,
			scope.key, messageID,
		).Scan(&previousKey, &previousPayload, &seq)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		merged := next
		if err == nil {
			previous, decodeErr := decodeConversationHistoryPayload(previousPayload)
			if decodeErr != nil {
				return decodeErr
			}
			// 与 compactMessages 一致：先到的快照作为 current，后到的作为 next。
			if backfill {
				merged = mergeCompactedMessage(next, previous)
			} else {
				merged = mergeCompactedMessage(previous, next)
			}
		}
		// 顺序号复现 compactMessages 的「首次出现位置」：回填行取 canonical 位置，实时新行追加到末尾，
		// 实时快照合并保留原位置。读取按它喂给规范化，同毫秒的行与轮次才和 canonical 同序。
		switch {
		case backfill:
			seq = backfillSeq + int64(index)
		case errors.Is(err, sql.ErrNoRows):
			liveSeq++
			seq = liveSeq
		}
		roundKey := historyPageGroupKey(merged, scope.collapse)
		if kind == conversationHistoryKindReceipt {
			if roundKey, err = conversationHistoryReceiptRoundKey(ctx, tx, scope, merged); err != nil {
				return err
			}
		}
		if roundKey == "" {
			continue
		}
		skeleton, details := projectConversationHistoryDetails(scope.key, messageID, merged)
		if err = replaceConversationHistoryDetails(ctx, tx, scope.key, messageID, skeleton, details); err != nil {
			return err
		}
		payload, err := json.Marshal(skeleton)
		if err != nil {
			return err
		}
		guidedSource := ""
		if stringFromAny(skeleton["role"]) == "user" {
			guidedSource = strings.TrimSpace(stringFromAny(skeleton["source_round_id"]))
		}
		if _, err = tx.ExecContext(
			ctx,
			`INSERT INTO conversation_history_rows (scope, message_id, round_key, kind, timestamp, guided_source, seq, payload)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(scope, message_id) DO UPDATE SET round_key = excluded.round_key, kind = excluded.kind,
			   timestamp = excluded.timestamp, guided_source = excluded.guided_source, seq = excluded.seq,
			   payload = excluded.payload`,
			scope.key, messageID, roundKey, kind, messageTimestamp(skeleton), guidedSource, seq, payload,
		); err != nil {
			return err
		}
		if kind == conversationHistoryKindMessage {
			touched[roundKey] = struct{}{}
		}
		if previousKey != "" && previousKey != roundKey {
			touched[previousKey] = struct{}{}
		}
	}
	for roundKey := range touched {
		if err := refreshConversationHistoryRound(ctx, tx, scope, roundKey); err != nil {
			return err
		}
	}
	return nil
}

// conversationHistoryRowKind 判断一行是否落表，规则与读路径一致：round marker 由调用方物化成用户行后再写，
// transcript_ref 由调用方换成解析后的消息，控制行与读路径过滤掉的内部行不落表。
func conversationHistoryRowKind(row protocol.Message) (string, bool) {
	switch stringFromAny(row[overlayKindField]) {
	case overlayKindRoundMarker, overlayKindTranscriptRef, overlayKindRoomPublicCursor, "history_rewrite", "room_context_checkpoint":
		return "", false
	case overlayKindExternalDelivery:
		return conversationHistoryKindReceipt, strings.TrimSpace(stringFromAny(row["message_id"])) != ""
	}
	if strings.TrimSpace(stringFromAny(row["message_id"])) == "" || shouldSkipInternalHistoryRow(row) {
		return "", false
	}
	return conversationHistoryKindMessage, true
}

// conversationHistoryReceiptRoundKey 把投递回执归到目标 assistant 所在的轮次；目标还没落表时按回执的 round_id。
func conversationHistoryReceiptRoundKey(
	ctx context.Context,
	tx *sql.Tx,
	scope conversationHistoryScope,
	row protocol.Message,
) (string, error) {
	if target := strings.TrimSpace(stringFromAny(row["assistant_message_id"])); target != "" {
		var key string
		err := tx.QueryRowContext(
			ctx,
			`SELECT round_key FROM conversation_history_rows WHERE scope = ? AND message_id = ? AND kind = ?`,
			scope.key, target, conversationHistoryKindMessage,
		).Scan(&key)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
	if roundID == "" {
		return "", nil
	}
	// 回执不带 agent_id：Room 成员的物理轮次 root:agent 要按 agent 后缀折叠到根轮次，与目标 assistant 同组。
	candidates := []string{historyPageGroupKey(protocol.Message{"round_id": roundID, "agent_id": row["agent_id"]}, scope.collapse)}
	if index := strings.LastIndex(roundID, ":"); scope.collapse && index > 0 {
		candidates = append(candidates, historyPageGroupKey(protocol.Message{"round_id": roundID, "agent_id": roundID[index+1:]}, true))
	}
	for _, key := range candidates {
		var exists int
		err := tx.QueryRowContext(
			ctx,
			`SELECT 1 FROM conversation_history_rounds WHERE scope = ? AND round_key = ?`,
			scope.key, key,
		).Scan(&exists)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return candidates[len(candidates)-1], nil
}

// projectConversationHistoryDetails 把大块工具输出和内联图片拆成 detail，行里只留骨架；判定阈值与旧读模型相同。
// 已经是骨架的块（带 detail_ref 的预览）会原样保留。拆分失败（例如单块超过上限）时整条消息原样落表。
func projectConversationHistoryDetails(scope, messageID string, row protocol.Message) (protocol.Message, []historyReadModelDetail) {
	content, exists := row["content"]
	if !exists {
		return row, nil
	}
	budget := historyToolResultBudget(historyMessageDetailThresholdBytes)
	projected, details, err := projectHistoryDetailValue(content, historyDetailProjectionContext{
		toolBudget: &budget,
		scope:      scope,
		messageID:  messageID,
		path:       "content",
	})
	if err != nil {
		return row, nil
	}
	skeleton := protocol.Clone(row)
	skeleton["content"] = projected
	return skeleton, details
}

func replaceConversationHistoryDetails(
	ctx context.Context,
	tx *sql.Tx,
	scope string,
	messageID string,
	skeleton protocol.Message,
	details []historyReadModelDetail,
) error {
	for _, detail := range details {
		var exists int
		err := tx.QueryRowContext(
			ctx,
			`SELECT 1 FROM conversation_history_details WHERE scope = ? AND detail_ref = ?`,
			scope, detail.Ref,
		).Scan(&exists)
		if err == nil {
			// ref 由消息、位置与内容摘要决定：同一份内容在多次快照间只压缩、存储一次。
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		compressed, err := gzipConversationHistoryPayload(detail.Payload)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(
			ctx,
			`INSERT INTO conversation_history_details (scope, detail_ref, message_id, kind, media_type, byte_size, payload_digest, payload)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			scope, detail.Ref, messageID, detail.Kind, detail.MediaType, len(detail.Payload), detail.Digest, compressed,
		); err != nil {
			return err
		}
	}
	keep := make(map[string]struct{})
	collectConversationHistoryDetailRefs(skeleton["content"], keep)
	rows, err := tx.QueryContext(
		ctx,
		`SELECT detail_ref FROM conversation_history_details WHERE scope = ? AND message_id = ?`,
		scope, messageID,
	)
	if err != nil {
		return err
	}
	stale := make([]string, 0)
	for rows.Next() {
		var ref string
		if err = rows.Scan(&ref); err != nil {
			_ = rows.Close()
			return err
		}
		if _, kept := keep[ref]; !kept {
			stale = append(stale, ref)
		}
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for _, ref := range stale {
		if _, err = tx.ExecContext(
			ctx,
			`DELETE FROM conversation_history_details WHERE scope = ? AND detail_ref = ?`,
			scope, ref,
		); err != nil {
			return err
		}
	}
	return nil
}

func collectConversationHistoryDetailRefs(value any, refs map[string]struct{}) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectConversationHistoryDetailRefs(item, refs)
		}
	case []map[string]any:
		for _, item := range typed {
			collectConversationHistoryDetailRefs(item, refs)
		}
	case map[string]any:
		if ref := strings.TrimSpace(stringFromAny(typed["detail_ref"])); ref != "" {
			refs[ref] = struct{}{}
		}
		for _, key := range []string{"content", "source"} {
			collectConversationHistoryDetailRefs(typed[key], refs)
		}
	}
}

// refreshConversationHistoryRound 重算一个轮次的分页游标与导航条目；成本只取决于这一轮的行数。
func refreshConversationHistoryRound(ctx context.Context, tx *sql.Tx, scope conversationHistoryScope, roundKey string) error {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT timestamp, seq, payload FROM conversation_history_rows WHERE scope = ? AND round_key = ? AND kind = ?
		 ORDER BY timestamp, seq`,
		scope.key, roundKey, conversationHistoryKindMessage,
	)
	if err != nil {
		return err
	}
	entries := make(map[string]*sessionRoundIndexAccumulator)
	count := 0
	first, firstSeq := int64(0), int64(0)
	cursor := ""
	for rows.Next() {
		var timestamp, seq int64
		var payload []byte
		if err = rows.Scan(&timestamp, &seq, &payload); err != nil {
			_ = rows.Close()
			return err
		}
		var indexRow roundIndexOverlayJSONRow
		if err = json.Unmarshal(payload, &indexRow); err != nil {
			_ = rows.Close()
			return err
		}
		// 按 (timestamp, seq) 升序扫描，第一行就是轮次在 canonical 里的起点。
		if count == 0 {
			first, firstSeq = timestamp, seq
		}
		if cursor == "" {
			cursor = historyPageCursorRoundID(protocol.Message{
				"message_id": indexRow.MessageID,
				"round_id":   indexRow.RoundID,
				"agent_id":   indexRow.AgentID,
			}, scope.collapse)
		}
		count++
		indexRow.applyToIndex(entries, nil, scope.collapse, scope.defaultAgentID)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM conversation_history_rounds WHERE scope = ? AND round_key = ?`, scope.key, roundKey)
		return err
	}
	// 没有 round_id 的单消息分组不进导航；列不能为 NULL，存空 BLOB。
	indexItem := []byte{}
	items := buildSessionRoundIndex(entries).Items
	for _, item := range items {
		if "round:"+item.RoundID == roundKey || len(items) == 1 {
			if indexItem, err = json.Marshal(item); err != nil {
				return err
			}
			break
		}
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO conversation_history_rounds (scope, round_key, cursor_round_id, first_timestamp, first_seq, index_item)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(scope, round_key) DO UPDATE SET cursor_round_id = excluded.cursor_round_id,
		   first_timestamp = excluded.first_timestamp, first_seq = excluded.first_seq, index_item = excluded.index_item`,
		scope.key, roundKey, cursor, first, firstSeq, indexItem,
	)
	return err
}

// decodeConversationHistoryPayload 与 canonical 读取 JSONL 的解码方式一致，保证数字等类型同形。
func decodeConversationHistoryPayload(payload []byte) (protocol.Message, error) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	normalized, ok := normalizeDecodedJSONValue(decoded).(map[string]any)
	if !ok {
		return nil, errConversationHistoryNotReady
	}
	return protocol.Message(normalized), nil
}

func gzipConversationHistoryPayload(payload []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(payload); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func gunzipConversationHistoryPayload(payload []byte, limit int64) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrHistoryMessageDetailUnavailable
	}
	return data, nil
}

func (h *conversationHistoryStore) checkpoint(ctx context.Context) {
	if db, err := h.database(ctx); err == nil {
		_, _ = db.ExecContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`)
	}
}

// canonicalHistoryFallback 判断表读取失败后是否退回 canonical：请求自身已取消或超时时照常返回错误；其余失败
// （回填出错、退避窗口内复用上次错误、回填反复被并发写入打断、表读取出错）记一条日志后退回 canonical，
// 读得慢，但会话总能打开。
func canonicalHistoryFallback(ctx context.Context, label string, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	logx.FromContext(ctx).Warn("会话历史读模型不可用，退回 canonical 读取", "scope", label, "err", err)
	return true
}

// ensure 同步执行 key 对应的回填/补齐任务：同一 key 同时只跑一个，后到的读取等待同一结果；失败后在退避窗口内
// 直接返回上次的错误，避免每次读取都重跑一次失败的整会话回填。任务与请求取消解耦：请求放弃等待时任务照常
// 完成，结果留给下一次读取。
func (h *conversationHistoryStore) ensure(ctx context.Context, key string, job func(context.Context) error) error {
	h.jobsMu.Lock()
	flight := h.inflight[key]
	if flight == nil {
		if attempt, failed := h.attempts[key]; failed && time.Since(attempt.at) < conversationHistoryRetryDelay(attempt.failures) {
			h.jobsMu.Unlock()
			return attempt.err
		}
		flight = &conversationHistoryFlight{done: make(chan struct{})}
		h.inflight[key] = flight
		go h.runFlight(key, flight, job)
	}
	h.jobsMu.Unlock()
	select {
	case <-flight.done:
		return flight.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func conversationHistoryRetryDelay(failures int) time.Duration {
	delay := conversationHistoryRetryMin
	for i := 1; i < failures && delay < conversationHistoryRetryMax; i++ {
		delay *= 2
	}
	return min(delay, conversationHistoryRetryMax)
}

func (h *conversationHistoryStore) runFlight(key string, flight *conversationHistoryFlight, job func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), conversationHistoryJobTimeout)
	started := time.Now()
	err := runConversationHistoryJob(ctx, job)
	cancel()

	h.jobsMu.Lock()
	delete(h.inflight, key)
	switch {
	case err == nil || retryableConversationHistoryErr(err):
		delete(h.attempts, key)
	default:
		attempt := h.attempts[key]
		attempt.at = time.Now()
		attempt.failures++
		attempt.err = err
		h.attempts[key] = attempt
	}
	h.jobsMu.Unlock()
	flight.err = err
	close(flight.done)

	logger := logx.FromContext(context.Background())
	args := []any{"scope", conversationHistoryScopeLabel(key), "duration_ms", time.Since(started).Milliseconds()}
	if err != nil {
		logger.Warn("会话历史回填失败", append(args, "err", err)...)
	} else {
		logger.Info("会话历史回填完成", args...)
	}
}

// needsVerification 在本进程第一次读取会话时检查最近几轮：有未收口的轮次、且与上次整会话回填时记下的指纹不同，
// 才需要再回填一次。它补的是「runtime 写完 transcript、表还没提交时进程退出」留下的缺口。
func (h *conversationHistoryStore) needsVerification(ctx context.Context, scope string) bool {
	if _, checked := h.verified.LoadOrStore(scope, struct{}{}); checked {
		return false
	}
	db, err := h.database(ctx)
	if err != nil {
		return false
	}
	state, ok, err := readConversationHistoryState(ctx, db, scope)
	if err != nil || !ok {
		return false
	}
	fingerprint, err := conversationHistoryUnfinishedFingerprint(ctx, db, scope)
	if err != nil {
		return false
	}
	return fingerprint != "" && fingerprint != state.VerifiedUnfinished
}

// conversationHistoryUnfinishedFingerprint 列出最近几轮里没有终态的轮次及其最新行顺序号；全部收口时为空。
func conversationHistoryUnfinishedFingerprint(ctx context.Context, q conversationHistoryQuerier, scope string) (string, error) {
	rows, err := q.QueryContext(
		ctx,
		`SELECT r.round_key, r.index_item,
		   (SELECT COALESCE(MAX(w.seq), 0) FROM conversation_history_rows w WHERE w.scope = r.scope AND w.round_key = r.round_key)
		 FROM conversation_history_rounds r WHERE r.scope = ? AND length(r.index_item) > 0
		 ORDER BY r.first_timestamp DESC, r.first_seq DESC LIMIT ?`,
		scope, conversationHistoryVerifyRounds,
	)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0)
	for rows.Next() {
		var key string
		var payload []byte
		var seq int64
		var item protocol.SessionRoundIndexItem
		if err = rows.Scan(&key, &payload, &seq); err != nil {
			_ = rows.Close()
			return "", err
		}
		if err = json.Unmarshal(payload, &item); err != nil {
			_ = rows.Close()
			return "", err
		}
		if strings.TrimSpace(item.Status) == "" {
			parts = append(parts, fmt.Sprintf("%s@%d", key, seq))
		}
	}
	return strings.Join(parts, "\n"), errors.Join(rows.Err(), rows.Close())
}

func runConversationHistoryJob(ctx context.Context, job func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("conversation history job panic: %v", recovered)
		}
	}()
	return job(ctx)
}

func conversationHistoryScopeLabel(scope string) string {
	return strings.ReplaceAll(scope, "\x00", "|")
}
