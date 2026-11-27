// room_history_test.go 覆盖 Room 历史、分页对齐、资源、定向消息与公开交接存储测试。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryStoreRejectsCrossOwnerStateSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	ownerBRoomRoot := appfs.UserRoomRootAt(stateRoot, "user-b")
	if err := os.MkdirAll(ownerBRoomRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerADataRoot := appfs.UserDataRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerADataRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join("..", "user-b", "state"),
		filepath.Join(ownerADataRoot, "state"),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	history := NewRoomHistoryStore("")
	err := history.AppendInlineMessage("user-a", "conversation-a", protocol.Message{
		"message_id": "message-a",
		"content":    "must stay with owner a",
	})
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("跨 owner state symlink 应被拒绝: %v", err)
	}
	foreignTarget := filepath.Join(
		ownerBRoomRoot,
		filepath.Base(history.paths.RoomConversationDir("user-b", "conversation-a")),
		"overlay.jsonl",
	)
	if _, statErr := os.Stat(foreignTarget); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("不能借 symlink 写入 owner B Room ledger: %v", statErr)
	}
}

func TestRoomHistoryRepairsTranscriptPermissions(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		t.Skip("需要以普通用户运行 Linux 文件权限回归")
	}
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("NEXUS_RUNTIME_ISOLATION_MODE", "enforce")
	owner := "room-permission-owner"
	workspacePath := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, owner), "agent")
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{{
		"type": "assistant", "uuid": "assistant-1", "sessionId": sessionID,
		"message": map[string]any{
			"id": "assistant-1", "role": "assistant", "stop_reason": "end_turn",
			"content": []map[string]any{{"type": "text", "text": "任务完成"}},
		},
	}})
	history := NewRoomHistoryStore("")
	transcriptPath, err := history.agentHistory.ForOwner(owner).resolveTranscriptPath(workspacePath, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err = history.AppendTranscriptReference(owner, "conversation-1", workspacePath,
		"agent:agent:ws:group:conversation-1", protocol.Message{
			"message_id": "assistant-1", "session_id": sessionID, "agent_id": "agent",
			"role": "assistant", "round_id": "round-1", "is_complete": true, "timestamp": int64(1000),
		}); err != nil {
		t.Fatal(err)
	}
	// 用受限替身模拟 launcher 修复；精确校验 owner，其他参数不能触发 chmod。
	launcher := filepath.Join(stateRoot, "repair-launcher")
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = repair-user ] && [ \"$2\" = --owner ] && [ \"$3\" = %s ] || exit 1\nchmod 600 %q\n", owner, transcriptPath)
	if err = os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_RUNTIME_LAUNCHER_PATH", launcher)
	for _, paged := range []bool{false, true} {
		t.Run(fmt.Sprintf("paged=%t", paged), func(t *testing.T) {
			if err := os.Chmod(transcriptPath, 0); err != nil {
				t.Fatal(err)
			}
			if file, openErr := os.Open(transcriptPath); openErr == nil {
				file.Close()
				t.Fatal("未复现 transcript 权限拒绝")
			}
			var rows []protocol.Message
			if paged {
				var page protocol.MessagePage
				page, err = history.ReadMessagesPageContext(context.Background(), owner, "conversation-1", nil, HistoryPageQuery{Limit: 10})
				rows = page.Items
			} else {
				rows, err = history.ReadMessages(owner, "conversation-1", nil)
			}
			if err == nil && (len(rows) != 1 || rows[0]["message_id"] != "assistant-1") {
				t.Fatalf("恢复后必须保留原消息，实际 %+v", rows)
			}
			if err != nil {
				t.Fatalf("Room 读取应修复权限并成功重试: %v", err)
			}
		})
	}
}

// roomRoundPageNormalize 是新读路径的参考实现：一轮的原始行先去掉已被 durable 引导用户行覆盖的
// guided_input 系统行（这是唯一跨轮次的规则，靠一次 guided_source 查询完成），再走现有规范化。
func roomRoundPageNormalize(rows []protocol.Message, guidedSources map[string]struct{}) []protocol.Message {
	kept := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		if stringFromAny(row["role"]) == "system" {
			metadata, _ := row["metadata"].(map[string]any)
			if stringFromAny(metadata["subtype"]) == message.SystemMessageSubtypeGuidedInput {
				if _, covered := guidedSources[stringFromAny(metadata["source_round_id"])]; covered {
					continue
				}
			}
		}
		kept = append(kept, protocol.Clone(row))
	}
	return normalizeHistoryRows(kept, nil)
}

type roundPageStream struct {
	round int
	user  bool
	rows  []protocol.Message
}

// roundPageDiffRows 生成一个会话的公区原始行（时间戳在交错时分配）。
func roundPageDiffRows(r *rand.Rand) []protocol.Message {
	agents := []string{"amy", "bob", "cid"}[:1+r.Intn(3)]
	roundCount := 2 + r.Intn(5)
	streams := make([]*roundPageStream, 0)
	for k := 0; k < roundCount; k++ {
		root := fmt.Sprintf("round-%d", k)
		user := &roundPageStream{round: k, user: true, rows: []protocol.Message{
			{"message_id": "user-" + root, "round_id": root, "role": "user", "content": "question " + root},
		}}
		if r.Intn(3) == 0 {
			// durable 引导用户行；对应的 guided_input 系统行可能落在任意一轮（含本轮）的成员执行里。
			user.rows = append(user.rows, protocol.Message{
				"message_id": "guide-" + root, "round_id": root, "role": "user", "content": "guide " + root,
				"source_round_id": "src-" + root,
			})
		}
		streams = append(streams, user)
		for index, agent := range agents {
			if k > 0 && r.Intn(3) == 0 {
				continue // 这一轮该成员没有参与
			}
			physical := root + ":" + agent
			id := fmt.Sprintf("assistant-%s-%s", root, agent)
			stream := &roundPageStream{round: k}
			snapshots := 1 + r.Intn(3)
			for n := 1; n <= snapshots; n++ {
				row := protocol.Message{
					"message_id": id, "round_id": physical, "agent_id": agent, "role": "assistant",
					"content":     []any{map[string]any{"type": "text", "text": fmt.Sprintf("answer %s part %d", id, n)}},
					"is_complete": n == snapshots,
				}
				if n == snapshots {
					row["stop_reason"] = "end_turn"
				}
				if r.Intn(2) == 0 {
					row["display_order"] = int64(index + 1)
				}
				stream.rows = append(stream.rows, row)
				if n == 1 && r.Intn(5) == 0 {
					stream.rows = append(stream.rows, protocol.Message{
						"message_id": "retry-" + physical, "round_id": physical, "agent_id": agent, "role": "system",
						"content": "retrying", "metadata": map[string]any{"subtype": "api_retry"},
					})
				}
			}
			if r.Intn(4) == 0 {
				// 引用某一轮的引导来源；有时来源不存在（不应被压掉）。
				source := fmt.Sprintf("src-round-%d", r.Intn(roundCount+1))
				stream.rows = append(stream.rows, protocol.Message{
					"message_id": "guided-" + physical, "round_id": physical, "agent_id": agent, "role": "system",
					"content": "guided", "metadata": map[string]any{"subtype": message.SystemMessageSubtypeGuidedInput, "source_round_id": source},
				})
			}
			if r.Intn(10) < 7 {
				stream.rows = append(stream.rows, protocol.NewHostResultMessage("result-"+physical, "", agent, physical, "success", "done "+id, false))
			} // 否则该成员轮次未收口，规范化会补一条中断
			streams = append(streams, stream)
		}
	}

	// 交错：每一轮的用户流按轮次顺序开始；成员流要等本轮用户问题写入后才开始；不同轮次可以重叠。
	userDone := make([]bool, roundCount)
	clock := int64(1000)
	var rows []protocol.Message
	for {
		ready := make([]*roundPageStream, 0)
		for _, stream := range streams {
			if len(stream.rows) == 0 {
				continue
			}
			if stream.user && (stream.round == 0 || userDone[stream.round-1]) {
				ready = append(ready, stream)
			}
			if !stream.user && userDone[stream.round] {
				ready = append(ready, stream)
			}
		}
		if len(ready) == 0 {
			break
		}
		stream := ready[r.Intn(len(ready))]
		row := stream.rows[0]
		stream.rows = stream.rows[1:]
		if r.Intn(6) != 0 {
			clock += int64(1 + r.Intn(3)) // 约 1/6 的行与上一行同毫秒
		}
		row["timestamp"] = clock
		rows = append(rows, row)
		if stream.user {
			userDone[stream.round] = true
		}
	}
	return rows
}

func roundPageJSON(t *testing.T, rows []protocol.Message) string {
	t.Helper()
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestRoomRoundPageNormalizationMatchesFullHistory(t *testing.T) {
	for seed := int64(1); seed <= 400; seed++ {
		r := rand.New(rand.NewSource(seed))
		raw := roundPageDiffRows(r)

		// 基准：整段历史一次规范化，再按 root round 归组（与分页分组用同一个 key）。
		full := normalizeHistoryRows(cloneHistoryRows(raw), nil)
		want := make(map[string][]protocol.Message)
		for _, row := range full {
			if key := historyPageGroupKey(row, true); key != "" {
				want[key] = append(want[key], row)
			}
		}

		// 新方案：写入时跳过内部行、按 round_key 落表；读取时逐轮规范化。
		guided := make(map[string]struct{})
		byRound := make(map[string][]protocol.Message)
		for _, row := range raw {
			if stringFromAny(row["role"]) == "user" && stringFromAny(row["source_round_id"]) != "" {
				guided[stringFromAny(row["source_round_id"])] = struct{}{}
			}
			if shouldSkipInternalHistoryRow(row) {
				continue
			}
			if key := historyPageGroupKey(row, true); key != "" {
				byRound[key] = append(byRound[key], row)
			}
		}
		keys := make([]string, 0, len(byRound))
		for key := range byRound {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		seen := make(map[string]struct{})
		for _, key := range keys {
			got := roomRoundPageNormalize(byRound[key], guided)
			if len(got) == 0 {
				if len(want[key]) != 0 {
					t.Fatalf("seed %d %s: 按轮规范化为空，整段历史有 %s", seed, key, roundPageJSON(t, want[key]))
				}
				continue
			}
			seen[key] = struct{}{}
			if w, g := roundPageJSON(t, want[key]), roundPageJSON(t, got); w != g {
				t.Fatalf("seed %d %s: 按轮规范化与整段历史不一致\n full  %s\n round %s\n raw   %s", seed, key, w, g, roundPageJSON(t, raw))
			}
			// 设计 §4：轮次排序键 first_timestamp 取本轮已落表（按 message_id 合并后）行的最小时间，
			// 必须等于整段历史里该轮第一行的时间。
			stored := compactMessages(byRound[key])
			minStored := messageTimestamp(stored[0])
			for _, row := range stored {
				minStored = min(minStored, messageTimestamp(row))
			}
			if first := messageTimestamp(want[key][0]); first != minStored {
				t.Errorf("seed %d %s: 本轮最小写入时间 %d ≠ 整段历史该轮首行时间 %d，first_timestamp 需改为按规范化结果计算", seed, key, minStored, first)
			}
		}
		for key := range want {
			if _, ok := seen[key]; !ok {
				t.Fatalf("seed %d: 整段历史有轮次 %s，按轮读取缺失", seed, key)
			}
		}
	}
}

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
	want, err := h.ReadMessages(owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 第一次读取同步回填；之后的成员唤醒只查表。
	if _, err = h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	db, err := h.history.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 已消费的旧前缀不可解码，成功仍能证明游标路径没有读取它。
	if _, err = db.Exec(`UPDATE conversation_history_rows SET payload='invalid' WHERE scope=? AND round_key='round:r-0'`,
		roomConversationHistoryScope(owner, conversation).key); err != nil {
		t.Fatal(err)
	}
	got, err := h.ReadRuntimeHistoryContext(ctx, owner, conversation, "assistant-8", "target", "user-9", nil)
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

func TestRoomDirectedMessageStoreDeduplicatesStableMessageID(t *testing.T) {
	root := t.TempDir()
	store := NewRoomDirectedMessageStore(root)
	store.paths.StateRoot = root
	message := protocol.RoomDirectedMessageRecord{
		MessageID: "stable-directed-message", RoomID: "room-idempotent",
		ConversationID: "conversation-idempotent", SourceAgentID: "agent-source",
		Recipients: []string{"agent-target"}, Content: "deliver once",
		ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
		Timestamp:  100,
	}
	first, inserted, err := store.AppendMessageIfAbsent(testRoomOwnerUserID, message)
	if err != nil || !inserted || first.MessageID != message.MessageID {
		t.Fatalf("first append = %+v inserted=%t err=%v", first, inserted, err)
	}
	retry := message
	retry.Timestamp = 200
	replayed, inserted, err := store.AppendMessageIfAbsent(testRoomOwnerUserID, retry)
	if err != nil || inserted || replayed.Timestamp != 100 {
		t.Fatalf("retry append = %+v inserted=%t err=%v", replayed, inserted, err)
	}
	rows, err := store.ReadMessages(testRoomOwnerUserID, message.ConversationID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("deduplicated rows = %+v err=%v", rows, err)
	}
	conflict := message
	conflict.Content = "different payload"
	if _, _, err = store.AppendMessageIfAbsent(testRoomOwnerUserID, conflict); err == nil {
		t.Fatal("same message id with different intent must fail closed")
	}
}

func TestRoomDirectedMessageWakeStoreRestoresOnlyPendingWakes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomDirectedMessageWakeStore(root)
	wake := RoomDirectedMessageWake{
		WakeID:      "wake-1",
		OwnerUserID: "owner-1",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "message-1",
			RoomID:         "room-1",
			ConversationID: "conversation-1",
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.Schedule(wake); err != nil {
		t.Fatalf("写入延迟唤醒失败: %v", err)
	}
	pending, err := NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 1 || pending[0].WakeID != wake.WakeID {
		t.Fatalf("延迟唤醒恢复不正确: pending=%+v err=%v", pending, err)
	}
	if err = store.Complete(wake.OwnerUserID, wake.WakeID); err != nil {
		t.Fatalf("完成延迟唤醒失败: %v", err)
	}
	pending, err = NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("已完成唤醒不应在重启后恢复: pending=%+v err=%v", pending, err)
	}
	inserted, err := store.ScheduleIfAbsent(wake)
	if err != nil || inserted {
		t.Fatalf("已完成 wake 的工具重试必须保持终态: inserted=%t err=%v", inserted, err)
	}
	pending, err = NewRoomDirectedMessageWakeStore(root).Pending(wake.OwnerUserID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("工具重试不应复活已完成 wake: pending=%+v err=%v", pending, err)
	}
}

func TestRoomDirectedMessageWakeStorePendingAllRejectsMismatchedOwner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomDirectedMessageWakeStore(root)
	const pathOwnerUserID = "owner-a"
	forgedWake := RoomDirectedMessageWake{
		WakeID:      "wake-forged-owner",
		OwnerUserID: "owner-b",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "message-forged-owner",
			RoomID:         "room-forged-owner",
			ConversationID: "conversation-forged-owner",
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.files.appendRoomJSONL(
		pathOwnerUserID,
		store.paths.RoomDirectedMessageWakesPath(pathOwnerUserID),
		map[string]any{
			"action": roomWakeActionSchedule,
			"wake":   forgedWake,
		},
	); err != nil {
		t.Fatalf("写入伪造 owner 延迟唤醒失败: %v", err)
	}

	pending, err := NewRoomDirectedMessageWakeStore(root).PendingAll()
	if err != nil {
		t.Fatalf("扫描全部延迟唤醒失败: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("目录与记录 owner 不一致时必须拒绝恢复: pending=%+v", pending)
	}
}

func TestRoomPublicHandoffRejectsInvalidGoalCollaborationBinding(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID:       "rh_invalid_goal_binding",
		ConversationID:  "conversation-handoff",
		SourceMessageID: "message-invalid",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-room",
		},
	})
	if err == nil {
		t.Fatal("incomplete Goal collaboration binding must be rejected")
	}
}

func TestRoomPublicHandoffGoalCollaborationInFlightUsesExactRevision(t *testing.T) {
	root := t.TempDir()
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	conversationID := "conversation-goal-fence"
	ownerUserID := testRoomOwnerUserID
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_goal_fence",
		ConversationID:  conversationID,
		SourceMessageID: "message-goal-fence",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-peer",
		QueueSource:     protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID:            "goal-room",
			ObjectiveRevision: 2,
		},
	}
	if _, _, err := store.Detect(ownerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	inFlight, err := store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		*handoff.GoalCollaborationBinding,
	)
	if err != nil || !inFlight {
		t.Fatalf("inFlight = %v err=%v, want exact revision fenced", inFlight, err)
	}
	inFlight, err = store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		protocol.GoalCollaborationBinding{GoalID: "goal-room", ObjectiveRevision: 3},
	)
	if err != nil || inFlight {
		t.Fatalf("inFlight = %v err=%v, want newer revision independent", inFlight, err)
	}
	if err := store.MarkTerminal(ownerUserID, conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	inFlight, err = store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		*handoff.GoalCollaborationBinding,
	)
	if err != nil || inFlight {
		t.Fatalf("inFlight = %v err=%v, want terminal edge released", inFlight, err)
	}
}

func TestRoomPublicHandoffRejectsIncompleteResponsibilityBinding(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID:       "execution_dispatch_incomplete",
		ConversationID:  "conversation-handoff",
		SourceMessageID: "execution_dispatch_incomplete",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		Content:         "deliver the result",
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			DispatchID:   "dispatch-1",
		},
	})
	if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		t.Fatalf("error = %v, want capability envelope error", err)
	}
}

func TestRoomPublicHandoffStoreReleasesClaimAfterRestartWindow(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-retry"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_test-retry",
		ConversationID:  conversationID,
		SourceMessageID: "message-retry",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil || !ok {
		t.Fatalf("首次 claim 失败: ok=%v err=%v", ok, err)
	}
	// 手工追加一个过期 claim，验证重启扫描会重新暴露它。
	value, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID)
	if err != nil || ok {
		t.Fatalf("未过期 claim 不应重复领取: value=%+v ok=%v err=%v", value, ok, err)
	}
	value.ClaimedAt = 1
	value.UpdatedAt = 1
	if err := store.appendLocked(testRoomOwnerUserID, conversationID, roomPublicHandoffActionClaimed, value); err != nil {
		t.Fatal(err)
	}
	reloaded := NewRoomPublicHandoffStore(root)
	reloaded.paths.StateRoot = root
	pending, err := reloaded.Pending(testRoomOwnerUserID, conversationID)
	if err != nil || len(pending) != 1 || pending[0].HandoffID != handoff.HandoffID {
		t.Fatalf("过期 claim 应进入恢复列表: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffStoreListsAndCancelsRoot(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-root-cancel"
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	for _, handoff := range []RoomPublicHandoff{
		{
			HandoffID: "rh-root-a", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-a", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
		},
		{
			HandoffID: "rh-root-b", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-b", SourceAgentID: "agent-b", TargetAgentID: "agent-c",
		},
		{
			HandoffID: "rh-other", ConversationID: conversationID, RootRoundID: "root-2",
			SourceMessageID: "message-c", SourceAgentID: "agent-a", TargetAgentID: "agent-d",
		},
	} {
		if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	edges, err := store.ListRoot(testRoomOwnerUserID, conversationID, "root-1")
	if err != nil || len(edges) != 2 {
		t.Fatalf("root snapshot 不正确: edges=%+v err=%v", edges, err)
	}
	if err := store.CancelForRoot(testRoomOwnerUserID, conversationID, "root-1", "interrupted"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(testRoomOwnerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].HandoffID != "rh-other" {
		t.Fatalf("取消 root 后只应保留其他 root: %+v", pending)
	}
}
