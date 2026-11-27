// INPUT: Room 公区 ledger 追加、transcript_ref 对应的实时消息、公区读取请求与会话删除。
// OUTPUT: 与 ledger 同锁落表的公区历史、同步的整会话回填与崩溃后的 ledger 补齐，以及只查表的读入口。
// POS: Room 公区接入历史读模型（见 conversation_history.go）。
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomConversationHistoryScope(ownerUserID, conversationID string) conversationHistoryScope {
	return conversationHistoryScope{
		key:      "room\x00" + strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(conversationID),
		collapse: true,
	}
}

// snapshotRoomHistoryLedger 返回公区 ledger 的文件身份、大小与首尾指纹；ledger 不存在时返回零值快照。
func (s *RoomHistoryStore) snapshotRoomHistoryLedger(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
) (historyPageSourceSnapshot, error) {
	base := historyPageSourceSnapshot{Kind: historyPageSourceRoomLedger}
	parent, name, err := s.files.openRoomFileParent(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		false,
	)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer parent.Close()
	return snapshotFileAtRoot(ctx, parent, name, base)
}

// appendRoomLedgerRow 追加一行公区 ledger，并在同一把会话锁内把 projected（解析后的完整消息）落表、推进
// ledger 偏移。落表失败不影响追加结果：ledger 是事实来源，会话标记失效后由下一次读取整会话回填。
func (s *RoomHistoryStore) appendRoomLedgerRow(
	ownerUserID string,
	conversationID string,
	row map[string]any,
	projected protocol.Message,
) error {
	target := s.paths.RoomConversationOverlayPath(ownerUserID, conversationID)
	h := s.history
	scope := roomConversationHistoryScope(ownerUserID, conversationID)
	unlock := h.lock(scope.key)
	defer unlock()
	ctx := context.Background()
	before, beforeErr := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
	if err := s.files.appendRoomJSONL(ownerUserID, target, row); err != nil {
		return err
	}
	after, afterErr := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
	err := errors.Join(beforeErr, afterErr)
	if err == nil {
		err = s.projectRoomLedgerAppendLocked(ctx, scope, ownerUserID, conversationID, before, after, projected)
	}
	if err != nil {
		logx.FromContext(ctx).Warn("Room 历史落表失败，等待整会话回填", "conversation_id", conversationID, "err", err)
		h.markBroken(ctx, scope.key)
	}
	return nil
}

func (s *RoomHistoryStore) projectRoomLedgerAppendLocked(
	ctx context.Context,
	scope conversationHistoryScope,
	ownerUserID string,
	conversationID string,
	before historyPageSourceSnapshot,
	after historyPageSourceSnapshot,
	projected protocol.Message,
) error {
	h := s.history
	state, ok, err := h.state(ctx, scope.key)
	if err != nil || !ok || !state.accepting() {
		// 还没回填：这一行已在 ledger 里，回填时一并处理。
		return err
	}
	// 回填时 ledger 还不存在的会话，第一次追加才确定文件身份。
	newLedger := state.Identity == "" && state.LedgerOffset == 0 && before.Size == 0
	if !newLedger && (before.FileIdentity != state.Identity || after.FileIdentity != state.Identity || before.Size < state.LedgerOffset) {
		return errConversationHistoryLedgerChanged
	}
	rows := make([]protocol.Message, 0, 1)
	if before.Size > state.LedgerOffset {
		// 上次落表之后 ledger 还有未投影的行（进程在追加与落表之间退出），先补齐。
		missed, readErr := s.readRoomLedgerRange(ctx, ownerUserID, conversationID, state.LedgerOffset, before.Size)
		if readErr != nil {
			return readErr
		}
		rows = append(rows, missed...)
	}
	rows = append(rows, projected)
	return h.applyRows(ctx, scope, rows, conversationHistoryLiveSeq, func(tx *sql.Tx) error {
		state.Identity = after.FileIdentity
		state.LedgerOffset = after.Size
		return writeConversationHistoryState(ctx, tx, scope.key, state)
	})
}

// readRoomLedgerRange 读取 ledger 的 [start, end) 字节区间并解析 transcript_ref；末尾半行会被跳过。
func (s *RoomHistoryStore) readRoomLedgerRange(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	start int64,
	end int64,
) ([]protocol.Message, error) {
	if end <= start {
		return nil, nil
	}
	parent, name, err := s.files.openRoomFileParent(ownerUserID, s.paths.RoomConversationOverlayPath(ownerUserID, conversationID), false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	file, err := parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := readJSONLReaderWithContext(ctx, io.NewSectionReader(file, start, end-start))
	if err != nil {
		return nil, err
	}
	return s.resolveRoomHistoryRowsWithCache(ctx, ownerUserID, conversationID, raw, newRoomTranscriptIndexCache())
}

// backfillConversationHistory 从 ledger 整会话回填。回填开始后的追加由实时写入直接落表（同一把锁内切换状态），
// 所以回填只处理水位之前的行；表里已有的行比回填快照新，按「已有行在后」合并。
func (s *RoomHistoryStore) backfillConversationHistory(ctx context.Context, ownerUserID, conversationID string) error {
	h := s.history
	scope := roomConversationHistoryScope(ownerUserID, conversationID)
	unlock := h.lock(scope.key)
	watermark, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
	if err == nil {
		err = h.resetScope(ctx, scope.key, watermark.FileIdentity, watermark.Size)
	}
	unlock()
	if err != nil {
		return err
	}
	rows, err := s.readRoomLedgerRange(ctx, ownerUserID, conversationID, 0, watermark.Size)
	if err != nil {
		return err
	}
	if err = h.applyBackfillRows(ctx, scope, compactConversationHistoryBackfillRows(rows)); err != nil {
		return err
	}
	unlock = h.lock(scope.key)
	defer unlock()
	return h.finishBackfill(ctx, scope.key, func(state conversationHistoryState) error {
		current, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
		if err != nil {
			return err
		}
		if current.Size < state.LedgerOffset || (state.Identity != "" && current.FileIdentity != state.Identity) {
			return errConversationHistoryLedgerChanged
		}
		return nil
	})
}

// catchUpConversationHistory 补齐 ledger 里比表新的行：正常情况下是正在落表的写入（拿到锁时已完成，什么也不做），
// 进程在追加与落表之间退出时才真正补行。
func (s *RoomHistoryStore) catchUpConversationHistory(ctx context.Context, ownerUserID, conversationID string) error {
	h := s.history
	scope := roomConversationHistoryScope(ownerUserID, conversationID)
	unlock := h.lock(scope.key)
	defer unlock()
	state, ok, err := h.state(ctx, scope.key)
	if err != nil || !ok || state.Status != conversationHistoryStatusReady {
		return err
	}
	ledger, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
	if err != nil {
		return err
	}
	if ledger.Size < state.LedgerOffset || (state.Identity != "" && ledger.FileIdentity != state.Identity) {
		h.markBroken(ctx, scope.key)
		return errConversationHistoryLedgerChanged
	}
	if ledger.Size == state.LedgerOffset {
		return nil
	}
	rows, err := s.readRoomLedgerRange(ctx, ownerUserID, conversationID, state.LedgerOffset, ledger.Size)
	if err != nil {
		return err
	}
	return h.applyRows(ctx, scope, rows, conversationHistoryLiveSeq, func(tx *sql.Tx) error {
		state.Identity = ledger.FileIdentity
		state.LedgerOffset = ledger.Size
		return writeConversationHistoryState(ctx, tx, scope.key, state)
	})
}

// readyConversationHistory 保证会话可以读表：未回填、已失效或 ledger 被替换时同步整会话回填，ledger 比表新时同步补齐。
// 每次读取比对一次 ledger 身份与大小（只读文件首尾几 KB）。
func (s *RoomHistoryStore) readyConversationHistory(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
) (conversationHistoryScope, error) {
	h := s.history
	scope := roomConversationHistoryScope(ownerUserID, conversationID)
	backfill := func(ctx context.Context) error {
		return s.backfillConversationHistory(ctx, ownerUserID, conversationID)
	}
	for attempt := 0; attempt < 3; attempt++ {
		state, ok, err := h.state(ctx, scope.key)
		if err != nil {
			return scope, err
		}
		if !ok || !state.ready() {
			if err = h.ensure(ctx, scope.key, backfill); err != nil && !retryableConversationHistoryErr(err) {
				return scope, err
			}
			continue
		}
		if h.needsVerification(ctx, scope.key) {
			h.markBroken(ctx, scope.key)
			continue
		}
		ledger, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
		if err != nil {
			return scope, err
		}
		if ledger.Size < state.LedgerOffset || (state.Identity != "" && ledger.FileIdentity != state.Identity) {
			h.markBroken(ctx, scope.key)
			continue
		}
		if ledger.Size > state.LedgerOffset {
			err = h.ensure(ctx, scope.key+"\x00catch-up", func(ctx context.Context) error {
				return s.catchUpConversationHistory(ctx, ownerUserID, conversationID)
			})
			if err != nil {
				if retryableConversationHistoryErr(err) {
					continue
				}
				return scope, err
			}
		}
		return scope, nil
	}
	return scope, errConversationHistoryNotReady
}

// deleteRoomConversationHistory 在删除 Room 会话时清掉表里的数据。
func deleteRoomConversationHistory(h *conversationHistoryStore, ownerUserID, conversationID string) error {
	if _, err := os.Stat(h.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	scope := roomConversationHistoryScope(ownerUserID, conversationID)
	unlock := h.lock(scope.key)
	defer unlock()
	return h.deleteScope(context.Background(), scope.key)
}

// ReadMessageDetailContext 展开 Room 历史页里的大块内容；ref 不在表里时返回 ErrHistoryMessageDetailUnavailable。
func (s *RoomHistoryStore) ReadMessageDetailContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	ref string,
) (HistoryMessageDetail, error) {
	detail, ok, err := s.history.readDetail(ctx, roomConversationHistoryScope(ownerUserID, conversationID), "", ref)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	if !ok {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	return detail, nil
}
