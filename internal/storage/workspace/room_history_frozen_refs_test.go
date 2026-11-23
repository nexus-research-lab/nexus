// INPUT: 已发布读模型的长 Room、进程重启、成员 transcript 的追加、无换行末行、原地改写、分支切换与 marker 的改写和重新归属。
// OUTPUT: 成员只追加时重启后也只从成员检查点续读且结果与 canonical 重建一致；已发布消息的投影变化一律回退重建。
// POS: Room 冻结引用检查点的回归；夹具沿用 room_long_session_bench_test.go。
package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

// refreshRoomTailAfterRestart 模拟进程重启后做一次 Room 增量刷新，并报告成员 transcript 是否被完整解析过
// （完整解析会写入解析缓存，从检查点续读不会）。
func refreshRoomTailAfterRestart(t *testing.T, f *roomLongSessionFixture) (err error, parsedWhole bool) {
	t.Helper()
	f.restart(t)
	scope := f.history.historyPageAccess(f.owner, f.conversation).Scope
	err = f.history.tryRefreshRoomHistoryTail(context.Background(), f.owner, f.conversation, scope)
	cache := f.history.agentHistory.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return err, len(cache.entries) > 0
}

// requireRoomTailResumed 要求重启后的刷新走增量路径、没有完整解析成员 transcript，且结果与 canonical 重建一致。
func requireRoomTailResumed(t *testing.T, f *roomLongSessionFixture) {
	t.Helper()
	err, parsedWhole := refreshRoomTailAfterRestart(t, f)
	if err != nil {
		t.Fatalf("append-only member transcript rejected: %v", err)
	}
	if parsedWhole {
		t.Fatal("member transcript parsed in full instead of resuming from its checkpoint")
	}
	assertRoomHistoryPageParity(t, f.history, f.owner, f.conversation, nil, 20, "", 0, "", 0)
}

// requireRoomTailRebuilt 要求重启后的增量刷新被拒绝，随后的读取经完整重建仍与 canonical 一致。
func requireRoomTailRebuilt(t *testing.T, f *roomLongSessionFixture) {
	t.Helper()
	if err, _ := refreshRoomTailAfterRestart(t, f); !errors.Is(err, errHistoryPageIndexInvalid) {
		t.Fatalf("changed published member message accepted: %v", err)
	}
	f.readFirstPage(t)
	assertRoomHistoryPageParity(t, f.history, f.owner, f.conversation, nil, 20, "", 0, "", 0)
}

func readRoomFrozenRefForTest(t *testing.T, f *roomLongSessionFixture) roomFrozenTranscriptRef {
	t.Helper()
	ctx := context.Background()
	access := f.history.historyPageAccess(f.owner, f.conversation)
	db, err := f.history.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(ctx, db, access.Scope)
	if err != nil || !ok {
		t.Fatalf("scope unavailable: %v", err)
	}
	_, state, err := readHistoryReadTail(ctx, db, access.Scope, scope.Generation)
	if err != nil {
		t.Fatal(err)
	}
	var frozen []roomFrozenTranscriptRef
	if err = json.Unmarshal(state, &frozen); err != nil || len(frozen) != 1 {
		t.Fatalf("frozen refs = %s: %v", state, err)
	}
	return frozen[0]
}

// appendPrivateFollowUp 让成员在公区之外再开始一个私有轮次，transcript 因而越过尚未发布的回复。
func (f *roomLongSessionFixture) appendPrivateFollowUp(t *testing.T) {
	t.Helper()
	timestamp := f.roundTimestamp(f.rounds-1) + 5000
	if err := f.private().AppendRoundMarker(f.workspacePath, f.sessionKey, "private-follow-up", "private follow-up", timestamp); err != nil {
		t.Fatal(err)
	}
	f.appendTranscript(t, map[string]any{
		"type": "user", "uuid": "u-follow-up", "parentUuid": f.lastUUID, "sessionId": f.sessionID,
		"timestamp": time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano),
		"message":   map[string]any{"role": "user", "content": "private follow-up"},
	})
	f.lastUUID = "u-follow-up"
}

// appendReplySnapshot 追加一条沿用已有 message id 的助手快照，使该消息的投影发生变化。
func (f *roomLongSessionFixture) appendReplySnapshot(t *testing.T, messageID string) {
	t.Helper()
	f.appendTranscript(t, map[string]any{
		"type": "assistant", "uuid": "a-snapshot", "parentUuid": f.lastUUID, "sessionId": f.sessionID,
		"timestamp": time.UnixMilli(f.roundTimestamp(f.rounds-1) + 4000).UTC().Format(time.RFC3339Nano),
		"message": map[string]any{"id": messageID, "role": "assistant", "stop_reason": "end_turn",
			"content": []map[string]any{{"type": "text", "text": "rewritten"}}},
	})
	f.lastUUID = "a-snapshot"
}

func TestRoomFrozenTranscriptResumesAfterRestart(t *testing.T) {
	t.Run("appended rounds", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		built := readRoomFrozenRefForTest(t, f)
		if built.Checkpoint == nil || built.Checkpoint.UUID != "u-00005" || len(built.MessageIDs) != 5 || len(built.Pending) != 0 {
			t.Fatalf("full rebuild frozen ref = %+v", built)
		}
		// 连续几次“重启、追加、增量刷新”：后几次续接的是增量刷新自己写下的检查点。
		for range 3 {
			f.appendRound(t)
			requireRoomTailResumed(t, f)
		}
		resumed := readRoomFrozenRefForTest(t, f)
		if resumed.Checkpoint == nil || resumed.Checkpoint.UUID != "u-00008" || len(resumed.MessageIDs) != 8 || len(resumed.Pending) != 0 {
			t.Fatalf("resumed frozen ref = %+v", resumed)
		}
	})

	t.Run("streaming round", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		// 公区先出现新提问，成员还没开始：上一轮的回复离开尾轮，但它的投影仍在检查点之后，只能待封存。
		rows := f.startRound(t)
		requireRoomTailResumed(t, f)
		if pending := readRoomFrozenRefForTest(t, f).Pending; len(pending) != 1 || pending[0].ID != f.replyID(3) {
			t.Fatalf("pending = %+v", pending)
		}
		for _, row := range rows {
			f.appendTranscript(t, row)
			requireRoomTailResumed(t, f)
		}
		// 成员的新轮次开始后检查点前移，上一轮的回复被封存。
		if sealed := readRoomFrozenRefForTest(t, f); sealed.Checkpoint.UUID != "u-00004" || len(sealed.MessageIDs) != 4 || len(sealed.Pending) != 0 {
			t.Fatalf("sealed frozen ref = %+v", sealed)
		}
		f.publishReply(t, 4)
		requireRoomTailResumed(t, f)
		f.appendRound(t)
		requireRoomTailResumed(t, f)
	})

	t.Run("reply published after the member moved on", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		f.appendTranscript(t, f.startRound(t)...)
		f.appendPrivateFollowUp(t)
		requireRoomTailResumed(t, f)
		// 检查点已越过这条回复：引用指向检查点之前，必须完整读取并按摘要校验，但仍是增量刷新。
		f.publishReply(t, 4)
		err, parsedWhole := refreshRoomTailAfterRestart(t, f)
		if err != nil || !parsedWhole {
			t.Fatalf("reference before the checkpoint: err=%v parsedWhole=%v", err, parsedWhole)
		}
		assertRoomHistoryPageParity(t, f.history, f.owner, f.conversation, nil, 20, "", 0, "", 0)
		f.appendRound(t)
		assertRoomHistoryPageParity(t, f.history, f.owner, f.conversation, nil, 20, "", 0, "", 0)
	})

	t.Run("reply in an unterminated last line", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		f.appendTranscript(t, f.startRound(t)...)
		// 回复所在的末行还没有换行：续读把这样的行留给下一次，完整读取则接受它，已发布的回复不能因此消失。
		info, err := os.Stat(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.Truncate(f.transcript, info.Size()-1); err != nil {
			t.Fatal(err)
		}
		f.publishReply(t, 4)
		err, parsedWhole := refreshRoomTailAfterRestart(t, f)
		if err != nil || !parsedWhole {
			t.Fatalf("reference into an unterminated line: err=%v parsedWhole=%v", err, parsedWhole)
		}
		f.requireLatestReply(t, f.readFirstPage(t))
		assertRoomHistoryPageParity(t, f.history, f.owner, f.conversation, nil, 20, "", 0, "", 0)
	})
}

func TestRoomFrozenTranscriptRejectsChangedPublishedMessages(t *testing.T) {
	t.Run("prefix rewritten in place", func(t *testing.T) {
		// 工具输出撑大 transcript，被改写的回复落在首尾指纹之外：只有“长度没有增长”能说明这不是追加。
		f := newRoomLongSessionFixture(t, 6, 4096)
		f.readFirstPage(t)
		data, err := os.ReadFile(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		reply := bytes.Index(data, []byte(f.replyID(2)))
		rewritten := append(bytes.Clone(data[:reply]), bytes.Replace(data[reply:], []byte("answer answer"), []byte("ANSWER ANSWER"), 1)...)
		file, err := os.OpenFile(f.transcript, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.WriteAt(rewritten, 0); err != nil {
			t.Fatal(err)
		}
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
		// 确保改写后的 mtime 与发布时的快照不同。
		later := time.Now().Add(time.Second)
		if err = os.Chtimes(f.transcript, later, later); err != nil {
			t.Fatal(err)
		}
		requireRoomTailRebuilt(t, f)
	})

	t.Run("appended branch drops a sealed message", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		// 新条目从第二轮中途分叉：主链改走新分支，其后各轮已发布的回复不再属于投影。
		f.lastUUID = "a1-00001"
		f.appendPrivateFollowUp(t)
		requireRoomTailRebuilt(t, f)
	})

	t.Run("sealed message continued by an appended snapshot", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		f.appendReplySnapshot(t, f.replyID(1))
		requireRoomTailRebuilt(t, f)
	})

	t.Run("pending message continued by an appended snapshot", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		f.startRound(t)
		requireRoomTailResumed(t, f)
		f.lastUUID = "a2-00003"
		f.appendReplySnapshot(t, f.replyID(3))
		requireRoomTailRebuilt(t, f)
	})

	t.Run("marker before the checkpoint edited in place", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		// marker 的数量与位置都没有变，只有第二轮的 round 归属被改掉：对齐下标看不出差别，只能靠 overlay 前缀校验。
		overlay := f.private().paths.SessionOverlayPath(f.workspacePath, f.sessionKey)
		content, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		edited := bytes.Replace(content, []byte("private-00001"), []byte("private-edit1"), 1)
		if bytes.Equal(edited, content) {
			t.Fatal("overlay marker not found")
		}
		if err = os.WriteFile(overlay, edited, 0o600); err != nil {
			t.Fatal(err)
		}
		requireRoomTailRebuilt(t, f)
	})

	t.Run("marker in the middle of a long overlay edited in place", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 120, 64)
		f.readFirstPage(t)
		// 首尾指纹只取样文件两端各 4 KiB：改动落在两端之间、长度不变时指纹照旧，只能靠“长度没变就是改写”拦下。
		overlay := f.private().paths.SessionOverlayPath(f.workspacePath, f.sessionKey)
		content, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		at := bytes.Index(content, []byte("private-00060"))
		if at < 4096 || at+4096 > len(content) {
			t.Fatalf("marker at %d of %d is not between the sampled edges", at, len(content))
		}
		edited := bytes.Replace(content, []byte("private-00060"), []byte("private-edit6"), 1)
		if err = os.WriteFile(overlay, edited, 0o600); err != nil {
			t.Fatal(err)
		}
		// 长度与首尾指纹都没变，快照只剩 mtime 能区分：确保它与发布时的快照不同。
		later := time.Now().Add(time.Second)
		if err = os.Chtimes(overlay, later, later); err != nil {
			t.Fatal(err)
		}
		requireRoomTailRebuilt(t, f)
	})

	t.Run("marker before the checkpoint rewritten", func(t *testing.T) {
		f := newRoomLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		// overlay 仍是纯追加，但新 marker 与第二轮的输入内容相同且时间相同，第二轮重新对齐到它，
		// 已发布回复的 round 归属随之变化。
		if err := f.private().AppendRoundMarker(f.workspacePath, f.sessionKey, "private-rebound", "question root-00001", f.roundTimestamp(1)); err != nil {
			t.Fatal(err)
		}
		requireRoomTailRebuilt(t, f)
	})
}
