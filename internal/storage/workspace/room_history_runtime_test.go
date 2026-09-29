// INPUT: 已消费历史、游标和后续排队输入。
// OUTPUT: runtime 增量窗口与 canonical 内容一致，旧前缀和 UI detail 不污染输入。
// POS: Room runtime 历史读取回归。
package workspace

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomRuntimeHistoryCursorWindow(t *testing.T) {
	h := newRoomHistoryTestStore(t, t.TempDir())
	ctx := context.Background()
	owner, conversation := testRoomOwnerUserID, "runtime-window"
	for i := 0; i < 10; i++ {
		agent := "other"
		if i == 7 {
			agent = "target"
		}
		for _, role := range []string{"user", "assistant"} {
			row := protocol.Message{"message_id": fmt.Sprintf("%s-%d", role, i), "round_id": fmt.Sprint("r-", i), "role": role, "content": "hello", "timestamp": int64(1000 + i*10), "agent_id": agent, "is_complete": true, "stop_reason": "end_turn"}
			if i == 7 && role == "assistant" {
				row["result_summary"] = map[string]any{"subtype": "error", "is_error": true, "terminal_reason": "rate_limit"}
			}
			if err := h.AppendInlineMessage(owner, conversation, row); err != nil {
				t.Fatal(err)
			}
		}
	}
	access := h.historyPageAccess(owner, conversation)
	built, err := access.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.readModel.persist(ctx, access, built); err != nil {
		t.Fatal(err)
	}
	want, err := h.ReadMessages(owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := h.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 已消费的旧前缀不可解码，成功仍能证明游标路径没有读取它。
	if _, err = db.Exec(`UPDATE history_read_groups SET payload='invalid' WHERE scope=? AND sequence=0`, access.Scope); err != nil {
		t.Fatal(err)
	}
	got, err := h.readRuntimeHistoryIndex(ctx, access, "assistant-8", "target", "user-9", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := want[14:19]
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("window differs: got=%+v want=%+v", got, expected)
	}
	if summary, _ := got[1]["result_summary"].(map[string]any); summary["terminal_reason"] != "rate_limit" {
		t.Fatalf("previous failure lost: %+v", got)
	}
	// 找不到游标和冷启动均返回 canonical，不凭页大小丢历史。
	for _, cursor := range []string{"", "missing"} {
		got, err = h.ReadRuntimeHistoryContext(ctx, owner, conversation, cursor, "target", "", nil)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("fallback differs: %v", err)
		}
	}
	// 当前轮已落盘但尚未回复，后续用户输入不能提前被当前轮消费。
	for i := 10; i < 12; i++ {
		if err = h.AppendInlineMessage(owner, conversation, protocol.Message{"message_id": fmt.Sprint("user-", i), "round_id": fmt.Sprint("r-", i), "role": "user", "content": "next", "timestamp": int64(1000 + i*10)}); err != nil {
			t.Fatal(err)
		}
	}
	got, err = h.ReadRuntimeHistoryContext(ctx, owner, conversation, "assistant-9", "target", "user-10", []string{"r-10"})
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1]["message_id"] != "user-10" {
		t.Fatalf("queued input leaked: %+v", got)
	}
	for _, row := range got {
		if row["message_id"] == "assistant_interrupt_r-10" {
			t.Fatal("active round was interrupted")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = h.ReadRuntimeHistoryContext(cancelled, owner, conversation, "assistant-9", "target", "", nil); err != context.Canceled {
		t.Fatalf("cancel ignored: %v", err)
	}
}

func TestRoomRuntimeHistoryDoesNotUseDetailPreview(t *testing.T) {
	h := newRoomHistoryTestStore(t, t.TempDir())
	ctx := context.Background()
	owner, conversation := testRoomOwnerUserID, "runtime-detail"
	text := strings.Repeat("完整正文", historyMessageDetailThresholdBytes/4)
	for _, row := range []protocol.Message{
		{"message_id": "u", "round_id": "r", "role": "user", "content": "question", "timestamp": int64(1000)},
		{"message_id": "a", "round_id": "r", "role": "assistant", "content": []any{map[string]any{"type": "tool_result", "content": text}}, "timestamp": int64(1001), "is_complete": true, "stop_reason": "end_turn"},
	} {
		if err := h.AppendInlineMessage(owner, conversation, row); err != nil {
			t.Fatal(err)
		}
	}
	access := h.historyPageAccess(owner, conversation)
	built, err := access.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.readModel.persist(ctx, access, built); err != nil {
		t.Fatal(err)
	}
	want, err := h.ReadMessages(owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.ReadRuntimeHistoryContext(ctx, owner, conversation, "u", "", "", nil)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime used a truncated preview: %v", err)
	}
}
