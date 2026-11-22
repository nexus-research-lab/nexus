// INPUT: 带受理游标的 DM round marker / Room inline 用户行、实时用户行构造器、追加与进程重启。
// OUTPUT: 用户行的 input_seq 在实时行、分页读模型与完整读取之间一致，刷新和重启后不变。
// POS: 用户输入行受理游标的持久化合同测试。
package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// userRowInputSeqs 按 message_id 收集用户行的受理游标；缺游标的用户行直接判失败。
func userRowInputSeqs(t *testing.T, rows []protocol.Message) map[string]int64 {
	t.Helper()
	cursors := map[string]int64{}
	for _, row := range rows {
		if row["role"] != "user" {
			continue
		}
		inputSeq := protocol.Int64FromAny(row[protocol.InputSeqField])
		if inputSeq <= 0 {
			t.Fatalf("用户行缺少受理游标: %+v", row)
		}
		cursors[stringFromAny(row["message_id"])] = inputSeq
	}
	return cursors
}

func assertInputSeqs(t *testing.T, stage string, got map[string]int64, live []protocol.Message) {
	t.Helper()
	if len(got) != len(live) {
		t.Fatalf("%s: 用户行数量 = %d，期望 %d: %v", stage, len(got), len(live), got)
	}
	var previous int64
	for _, row := range live {
		messageID := stringFromAny(row["message_id"])
		want := protocol.Int64FromAny(row[protocol.InputSeqField])
		if got[messageID] != want {
			t.Fatalf("%s: %s 的游标 = %d，实时行是 %d", stage, messageID, got[messageID], want)
		}
		if want <= previous {
			t.Fatalf("%s: %s 的游标 %d 没有大于上一条输入的 %d", stage, messageID, want, previous)
		}
		previous = want
	}
}

func TestDMUserRowsKeepInputSeqAcrossRefreshAndRestart(t *testing.T) {
	f := newLongSessionFixture(t, 0, 0)
	var live []protocol.Message
	// accept 复刻 DM 受理：铸造一次游标，写进 round marker，并用实时构造器生成广播给前端的用户行。
	accept := func() {
		index := len(live)
		roundID := fmt.Sprintf("cursor-round-%d", index)
		userMessageID := "msg_user_cursor_" + fmt.Sprint(index)
		prompt := fmt.Sprintf("question %d", index)
		at := int64(1_700_000_000_000 + index*10_000)
		inputSeq := protocol.NewInputSeq()
		if err := f.history.AppendRoundMarkerWithOptions(f.workspacePath, f.session.SessionKey, roundID, prompt, at, RoundMarkerOptions{
			UserMessageID: userMessageID,
			InputSeq:      inputSeq,
		}); err != nil {
			t.Fatal(err)
		}
		userUUID, assistantUUID := "cursor-u-"+fmt.Sprint(index), "cursor-a-"+fmt.Sprint(index)
		transcriptTime := func(offsetSeconds int64) string {
			return time.UnixMilli(at + offsetSeconds*1000).UTC().Format(time.RFC3339Nano)
		}
		f.appendTranscript(t,
			map[string]any{"type": "user", "uuid": userUUID, "parentUuid": nilIfEmpty(f.lastUUID), "sessionId": *f.session.SessionID,
				"timestamp": transcriptTime(0), "message": map[string]any{"role": "user", "content": prompt}},
			map[string]any{"type": "assistant", "uuid": assistantUUID, "parentUuid": userUUID, "sessionId": *f.session.SessionID,
				"timestamp": transcriptTime(1), "message": map[string]any{"id": "cursor-m-" + fmt.Sprint(index), "role": "assistant",
					"stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": "answer"}}}},
		)
		f.lastUUID = assistantUUID
		live = append(live, dmdomain.BuildUserRoundMarker(f.session, roundID, "", userMessageID, inputSeq, prompt, "", nil))
	}
	readAll := func() []protocol.Message {
		rows, err := f.history.ReadMessages(f.workspacePath, f.session, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	accept()
	accept()
	assertInputSeqs(t, "首次分页读取", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
	assertInputSeqs(t, "首次完整读取", userRowInputSeqs(t, readAll()), live)

	accept()
	assertInputSeqs(t, "追加一轮后刷新", userRowInputSeqs(t, f.readFirstPage(t).Items), live)

	simulateHistoryProcessRestart(t, f)
	assertInputSeqs(t, "重启后分页读取", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
	assertInputSeqs(t, "重启后完整读取", userRowInputSeqs(t, readAll()), live)

	accept()
	assertInputSeqs(t, "重启后再受理一轮", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
}

func TestRoomUserRowsKeepInputSeqAcrossRefreshAndRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	history := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "room-input-cursor"
	var live []protocol.Message
	// accept 复刻 Room 受理：同一个带游标的用户行既写入公区账本，也是广播给前端的实时行。
	accept := func() {
		index := len(live)
		row := protocol.Message{
			"message_id":           "msg_user_room_cursor_" + fmt.Sprint(index),
			"conversation_id":      conversation,
			"round_id":             fmt.Sprintf("room-cursor-round-%d", index),
			"role":                 "user",
			"content":              fmt.Sprintf("room question %d", index),
			"timestamp":            int64(1_700_000_000_000 + index*10_000),
			protocol.InputSeqField: protocol.NewInputSeq(),
		}
		if err := history.AppendInlineMessage(owner, conversation, row); err != nil {
			t.Fatal(err)
		}
		live = append(live, row)
	}
	readPage := func(store *RoomHistoryStore) []protocol.Message {
		page, err := store.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		waitHistoryPageIndexFutureTB(t, store.historyPageAccess(owner, conversation).Scope)
		return page.Items
	}
	readAll := func(store *RoomHistoryStore) []protocol.Message {
		rows, err := store.ReadMessages(owner, conversation, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	accept()
	accept()
	assertInputSeqs(t, "首次分页读取", userRowInputSeqs(t, readPage(history)), live)
	assertInputSeqs(t, "首次完整读取", userRowInputSeqs(t, readAll(history)), live)

	accept()
	assertInputSeqs(t, "追加一条后刷新", userRowInputSeqs(t, readPage(history)), live)

	// 新的 store 与读模型连接模拟进程重启：进程内缓存全部丢弃，只剩落盘的账本与读模型。
	reopened := NewRoomHistoryStore(filepath.Join(root, "users"))
	reopened.readModel = &historyReadModel{path: history.readModel.path}
	t.Cleanup(func() {
		if reopened.readModel.db != nil {
			_ = reopened.readModel.db.Close()
		}
	})
	assertInputSeqs(t, "重启后分页读取", userRowInputSeqs(t, readPage(reopened)), live)
	assertInputSeqs(t, "重启后完整读取", userRowInputSeqs(t, readAll(reopened)), live)
}

// 同一条用户消息的后一份快照（如 transcript 副本或 reparent 后的重写）不带游标时，合并结果沿用受理时的值。
func TestCompactKeepsInputSeqWhenLaterUserSnapshotOmitsIt(t *testing.T) {
	accepted := protocol.Message{"message_id": "msg_user_1", "round_id": "round-1", "role": "user", "content": "hi",
		"timestamp": int64(1000), protocol.InputSeqField: int64(1_700_000_000_000_001)}
	later := protocol.Message{"message_id": "msg_user_1", "round_id": "round-1", "role": "user", "content": "hi", "timestamp": int64(2000)}
	for name, rows := range map[string][]protocol.Message{
		"带游标的快照在前": {accepted, later},
		"带游标的快照在后": {later, accepted},
	} {
		compacted := compactMessages(rows)
		if len(compacted) != 1 || protocol.Int64FromAny(compacted[0][protocol.InputSeqField]) != 1_700_000_000_000_001 {
			t.Fatalf("%s: 合并后的用户行 = %+v", name, compacted)
		}
	}
}
