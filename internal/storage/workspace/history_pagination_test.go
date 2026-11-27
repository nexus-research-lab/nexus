// history_pagination_test.go 覆盖历史分页索引、轮次索引、改写与 Overlay 回放测试。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryPageIndexMatchesCanonicalPaginationAndReceipt(t *testing.T) {
	root := t.TempDir()
	history := newRoomHistoryTestStore(t, root)
	ownerUserID := testRoomOwnerUserID
	conversationID := "room-index-parity"

	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-1", "amy", 1000)
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-z", "amy", 2000)
	appendRoomHistoryIndexRound(t, history, ownerUserID, conversationID, "root-a", "amy", 2002)
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id":  "result-only",
		"round_id":    "root-result:amy",
		"agent_id":    "amy",
		"role":        "result",
		"subtype":     "success",
		"duration_ms": int64(50),
		"timestamp":   int64(3000),
	}); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		overlayKindField:              overlayKindExternalDelivery,
		"message_id":                  "delivery-result-only",
		"role":                        overlayKindExternalDelivery,
		"round_id":                    "root-result:amy",
		"channel":                     "feishu",
		"target":                      "chat-1",
		"primary_platform_message_id": "platform-1",
		"timestamp":                   int64(3010),
	}); err != nil {
		t.Fatal(err)
	}

	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
	latest := assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 2, "", 0, "", 0)
	if len(latest.Items) == 0 || latest.Items[len(latest.Items)-1]["external_delivery"] == nil {
		t.Fatalf("result-only Room subround 的 receipt 未挂到 synthetic assistant: %+v", latest.Items)
	}
	assertRoomHistoryPageParity(
		t,
		history,
		ownerUserID,
		conversationID,
		nil,
		2,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 0, "", 0, "root-z", 1)
}

func TestHistoryReadModelRoundIndexMatchesCanonicalProjection(t *testing.T) {
	t.Run("agent", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{
			SessionKey: "agent:amy:ws:dm:round-index-model",
			AgentID:    "amy",
		}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		for index := 1; index <= 4; index++ {
			appendAgentHistoryIndexRound(
				t, history, workspacePath, session.SessionKey,
				fmt.Sprintf("round-%d", index), int64(index*1000),
			)
		}
		active := []string{"round-4"}
		expected, err := history.ReadRoundIndex(workspacePath, session, active)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := history.ReadRoundIndexPageContext(
			context.Background(), workspacePath, session, active,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, actual)
		warm, err := history.ReadRoundIndexPageContext(
			context.Background(), workspacePath, session, active,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, warm)
	})

	t.Run("room", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		conversationID := "round-index-model"
		for index := 1; index <= 4; index++ {
			appendRoomHistoryIndexRound(
				t,
				history,
				testRoomOwnerUserID,
				conversationID,
				fmt.Sprintf("round-%d", index),
				"amy",
				int64(index*1000),
			)
		}
		active := []string{"round-4:amy"}
		expected, err := canonicalRoomRoundIndexForTest(history, testRoomOwnerUserID, conversationID, active)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := history.ReadRoundIndexPageContext(
			context.Background(),
			testRoomOwnerUserID,
			conversationID,
			active,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, actual)
		warm, err := history.ReadRoundIndexPageContext(
			context.Background(),
			testRoomOwnerUserID,
			conversationID,
			active,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertSessionRoundIndexJSONEqual(t, expected, warm)
	})
}

func appendAgentHistoryIndexRound(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	roundID string,
	timestamp int64,
) {
	t.Helper()
	for _, row := range []protocol.Message{
		{
			"message_id": "user-" + roundID,
			"round_id":   roundID,
			"role":       "user",
			"content":    "question " + roundID,
			"timestamp":  timestamp,
		},
		{
			"message_id":  "assistant-" + roundID,
			"round_id":    roundID,
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "answer " + roundID}},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   timestamp + 1,
		},
	} {
		if err := history.AppendOverlayMessage(workspacePath, sessionKey, row); err != nil {
			t.Fatal(err)
		}
	}
}

func writeAgentHistoryIndexSession(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
) {
	t.Helper()
	if _, err := history.files.UpsertSession(workspacePath, session); err != nil {
		t.Fatalf("写入 history index session meta 失败: %v", err)
	}
}

func appendRoomHistoryIndexRound(
	t *testing.T,
	history *RoomHistoryStore,
	ownerUserID string,
	conversationID string,
	roundID string,
	agentID string,
	timestamp int64,
) {
	t.Helper()
	for _, row := range []protocol.Message{
		{
			"message_id": "user-" + roundID,
			"round_id":   roundID,
			"role":       "user",
			"content":    "question " + roundID,
			"timestamp":  timestamp,
		},
		{
			"message_id":  "assistant-" + roundID,
			"round_id":    roundID + ":" + agentID,
			"agent_id":    agentID,
			"role":        "assistant",
			"content":     []map[string]any{{"type": "text", "text": "answer " + roundID}},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   timestamp + 1,
		},
	} {
		if err := history.AppendInlineMessage(ownerUserID, conversationID, row); err != nil {
			t.Fatal(err)
		}
	}
}

func assertAgentHistoryPageParity(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	session protocol.Session,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) protocol.MessagePage {
	t.Helper()
	raw, err := history.readHistoryRows(workspacePath, session)
	if err != nil {
		t.Fatal(err)
	}
	expected := canonicalHistoryPageOracle(
		raw,
		active,
		limit,
		beforeRoundID,
		beforeTimestamp,
		aroundRoundID,
		aroundLimit,
		false,
	)
	actual, err := history.ReadMessagesPageContext(
		context.Background(),
		workspacePath,
		session,
		active,
		HistoryPageQuery{
			Limit:                limit,
			BeforeRoundID:        beforeRoundID,
			BeforeRoundTimestamp: beforeTimestamp,
			AroundRoundID:        aroundRoundID,
			AroundLimit:          aroundLimit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryPageJSONEqual(t, expected, actual)
	return actual
}

func assertRoomHistoryPageParity(
	t *testing.T,
	history *RoomHistoryStore,
	ownerUserID string,
	conversationID string,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) protocol.MessagePage {
	t.Helper()
	raw, err := history.readResolvedRows(ownerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	expected := canonicalHistoryPageOracle(
		raw,
		active,
		limit,
		beforeRoundID,
		beforeTimestamp,
		aroundRoundID,
		aroundLimit,
		true,
	)
	actual, err := history.ReadMessagesPageContext(
		context.Background(),
		ownerUserID,
		conversationID,
		active,
		HistoryPageQuery{
			Limit:                limit,
			BeforeRoundID:        beforeRoundID,
			BeforeRoundTimestamp: beforeTimestamp,
			AroundRoundID:        aroundRoundID,
			AroundLimit:          aroundLimit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryPageJSONEqual(t, expected, actual)
	return actual
}

func canonicalHistoryPageOracle(
	rows []protocol.Message,
	active []string,
	limit int,
	beforeRoundID string,
	beforeTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
	collapseRoomRounds bool,
) protocol.MessagePage {
	normalized := normalizeHistoryRows(rows, normalizeActiveRoundIDs(active))
	if aroundRoundID != "" {
		return paginateNormalizedHistoryRowsAround(normalized, aroundRoundID, aroundLimit, collapseRoomRounds)
	}
	return paginateNormalizedHistoryRows(
		normalized,
		limit,
		beforeRoundID,
		beforeTimestamp,
		collapseRoomRounds,
	)
}

func assertHistoryPageJSONEqual(t *testing.T, expected protocol.MessagePage, actual protocol.MessagePage) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if string(expectedJSON) != string(actualJSON) {
		t.Fatalf("history page 与 canonical oracle 不一致\nexpected=%s\nactual=%s", expectedJSON, actualJSON)
	}
}

func assertSessionRoundIndexJSONEqual(
	t *testing.T,
	expected protocol.SessionRoundIndex,
	actual protocol.SessionRoundIndex,
) {
	t.Helper()
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if string(expectedJSON) != string(actualJSON) {
		t.Fatalf("round index 与 canonical 不一致\nexpected=%s\nactual=%s", expectedJSON, actualJSON)
	}
}

// canonicalRoomRoundIndexForTest 直接从公区 ledger 计算导航，作为 Room 读模型导航的 oracle。
func canonicalRoomRoundIndexForTest(
	history *RoomHistoryStore,
	ownerUserID string,
	conversationID string,
	active []string,
) (protocol.SessionRoundIndex, error) {
	return history.readCanonicalRoundIndexContext(context.Background(), ownerUserID, conversationID, active)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func paginateNormalizedHistoryRows(
	rows []protocol.Message,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
	collapseRoomAgentRounds bool,
) protocol.MessagePage {
	if len(rows) == 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	pageLimit := normalizeRoundPageLimit(limit)
	groups := buildHistoryPageGroups(rows, collapseRoomAgentRounds)
	endGroupIndex := findHistoryPageEndGroupIndex(
		groups,
		strings.TrimSpace(beforeRoundID),
		beforeRoundTimestamp,
	)
	if endGroupIndex <= 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	startGroupIndex := endGroupIndex - pageLimit
	if startGroupIndex < 0 {
		startGroupIndex = 0
	}

	pageItems := make([]protocol.Message, 0)
	for _, group := range groups[startGroupIndex:endGroupIndex] {
		pageItems = append(pageItems, group.Items...)
	}

	page := protocol.MessagePage{
		Items:   pageItems,
		HasMore: startGroupIndex > 0,
	}
	if page.HasMore && len(pageItems) > 0 {
		oldestGroup := groups[startGroupIndex]
		if strings.TrimSpace(oldestGroup.CursorRoundID) != "" {
			page.NextBeforeRoundID = stringPointer(oldestGroup.CursorRoundID)
		}
		timestamp := oldestGroup.CursorRoundTimestamp
		page.NextBeforeRoundTimestamp = &timestamp
	}
	return page
}

func paginateNormalizedHistoryRowsAround(
	rows []protocol.Message,
	aroundRoundID string,
	aroundLimit int,
	collapseRoomAgentRounds bool,
) protocol.MessagePage {
	if len(rows) == 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	aroundRoundID = strings.TrimSpace(aroundRoundID)
	if aroundRoundID == "" {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: false,
		}
	}

	groups := buildHistoryPageGroups(rows, collapseRoomAgentRounds)
	targetIndex := -1
	for index, group := range groups {
		if group.CursorRoundID == aroundRoundID {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return protocol.MessagePage{
			Items:   []protocol.Message{},
			HasMore: len(groups) > 0,
		}
	}

	radius := normalizeRoundAroundLimit(aroundLimit)
	startIndex := targetIndex - radius
	if startIndex < 0 {
		startIndex = 0
	}
	endIndex := targetIndex + radius + 1
	if endIndex > len(groups) {
		endIndex = len(groups)
	}

	pageItems := make([]protocol.Message, 0)
	for _, group := range groups[startIndex:endIndex] {
		pageItems = append(pageItems, group.Items...)
	}
	page := protocol.MessagePage{
		Items:   pageItems,
		HasMore: startIndex > 0 || endIndex < len(groups),
	}
	if startIndex > 0 {
		oldestGroup := groups[startIndex]
		if strings.TrimSpace(oldestGroup.CursorRoundID) != "" {
			page.NextBeforeRoundID = stringPointer(oldestGroup.CursorRoundID)
		}
		timestamp := oldestGroup.CursorRoundTimestamp
		page.NextBeforeRoundTimestamp = &timestamp
	}
	return page
}

func buildHistoryPageGroups(
	rows []protocol.Message,
	collapseRoomAgentRounds bool,
) []historyPageGroup {
	if len(rows) == 0 {
		return nil
	}

	groups := make([]historyPageGroup, 0, len(rows))
	currentGroupKey := ""
	currentGroup := historyPageGroup{}

	flushCurrentGroup := func() {
		if len(currentGroup.Items) == 0 {
			return
		}
		groups = append(groups, currentGroup)
		currentGroup = historyPageGroup{}
	}

	for _, row := range rows {
		groupKey := historyPageGroupKey(row, collapseRoomAgentRounds)
		if groupKey == "" {
			continue
		}
		if groupKey != currentGroupKey {
			flushCurrentGroup()
			currentGroupKey = groupKey
			currentGroup = historyPageGroup{
				CursorRoundID:        historyPageCursorRoundID(row, collapseRoomAgentRounds),
				CursorRoundTimestamp: messageTimestamp(row),
				Items:                make([]protocol.Message, 0, 1),
			}
		}
		currentGroup.Items = append(
			currentGroup.Items,
			normalizeHistoryPageRow(row, collapseRoomAgentRounds),
		)
	}
	flushCurrentGroup()
	return groups
}

func newRoomHistoryTestStore(t *testing.T, stateRoot string) *RoomHistoryStore {
	t.Helper()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	return NewRoomHistoryStore(filepath.Join(stateRoot, "users"))
}

// overlayReplayHarness 让同一个 store 走增量回放，再与整份重读的结果逐字段比较。
type overlayReplayHarness struct {
	t             *testing.T
	history       *AgentHistoryStore
	workspacePath string
	session       protocol.Session
	overlayPath   string
}

func newOverlayReplayHarness(t *testing.T) *overlayReplayHarness {
	t.Helper()
	return newOverlayReplayHarnessFor(t, newLongSessionFixture(t, 0, 16))
}

func newOverlayReplayHarnessFor(t *testing.T, fixture *longSessionFixture) *overlayReplayHarness {
	t.Helper()
	overlayPath := fixture.history.paths.SessionOverlayPath(fixture.workspacePath, fixture.session.SessionKey)
	if err := os.MkdirAll(filepath.Dir(overlayPath), 0o755); err != nil {
		t.Fatal(err)
	}
	return &overlayReplayHarness{
		t:             t,
		history:       fixture.history,
		workspacePath: fixture.workspacePath,
		session:       fixture.session,
		overlayPath:   overlayPath,
	}
}

func (h *overlayReplayHarness) appendRaw(raw string) {
	h.t.Helper()
	file, err := os.OpenFile(h.overlayPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		h.t.Fatal(err)
	}
	defer file.Close()
	if _, err = file.WriteString(raw); err != nil {
		h.t.Fatal(err)
	}
}

func (h *overlayReplayHarness) appendRows(rows ...map[string]any) {
	h.t.Helper()
	var builder strings.Builder
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			h.t.Fatal(err)
		}
		builder.Write(payload)
		builder.WriteByte('\n')
	}
	h.appendRaw(builder.String())
}

// assertMatchesFullRead 比较增量回放与整份重读；整份重读用不带缓存的原始读取函数。
func (h *overlayReplayHarness) assertMatchesFullRead(active []string) {
	h.t.Helper()
	ctx := context.Background()
	gotState, err := h.history.readOverlayHistoryStateContext(ctx, h.workspacePath, h.session.SessionKey)
	if err != nil {
		h.t.Fatalf("replayed overlay state: %v", err)
	}
	rows, err := h.readAllRows(ctx)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.t.Fatalf("full overlay read: %v", err)
	}
	if wantState := overlayHistoryStateFromRows(rows); !reflect.DeepEqual(gotState, wantState) {
		h.t.Fatalf("overlay state diverged from full read\n got: %#v\nwant: %#v", gotState, wantState)
	}

	gotIndex, gotErr := h.history.readCanonicalRoundIndexContext(ctx, h.workspacePath, h.session, active)
	root, err := h.history.files.openWorkspaceRoot(h.workspacePath, false)
	if err != nil {
		h.t.Fatal(err)
	}
	defer root.Close()
	relative, err := filepath.Rel(h.workspacePath, h.overlayPath)
	if err != nil {
		h.t.Fatal(err)
	}
	wantIndex, wantErr := readRoundIndexFromRootContext(
		ctx, root, filepath.ToSlash(relative), normalizeActiveRoundIDs(active), false, h.session.AgentID,
	)
	if (gotErr == nil) != (wantErr == nil) {
		h.t.Fatalf("round index error diverged: got %v want %v", gotErr, wantErr)
	}
	if !reflect.DeepEqual(gotIndex, wantIndex) {
		h.t.Fatalf("round index diverged from full read\n got: %#v\nwant: %#v", gotIndex, wantIndex)
	}
}

func (h *overlayReplayHarness) readAllRows(ctx context.Context) ([]map[string]any, error) {
	file, err := openWorkspaceFileAt(h.history.files, h.workspacePath, h.overlayPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readJSONLFileWithContext(ctx, file)
}

func (h *overlayReplayHarness) replayEntry() *overlayReplayEntry {
	return h.history.cache.overlays.entry(h.history.ownerUserID + "\x00" + h.overlayPath)
}

// randomOverlayRow 覆盖 round index 与历史行会区分的全部行形态：可见/隐藏 marker、
// 用户行、assistant/result 行（含重复 message_id）、result_summary 与控制行。
func randomOverlayRow(random *rand.Rand, sequence int) map[string]any {
	roundID := fmt.Sprintf("round-%02d", random.Intn(24))
	timestamp := int64(0)
	if random.Intn(8) != 0 {
		timestamp = int64(1_700_000_000_000 + random.Intn(40)*1000)
	}
	row := map[string]any{"round_id": roundID, "timestamp": timestamp}
	if random.Intn(3) == 0 {
		row["message_id"] = fmt.Sprintf("msg-%02d", random.Intn(30))
	}
	switch random.Intn(8) {
	case 0, 1:
		row[overlayKindField] = overlayKindRoundMarker
		row["content"] = fmt.Sprintf("prompt %d", sequence)
		row["hidden_from_user"] = random.Intn(5) == 0
		row["user_message_id"] = fmt.Sprintf("user-%d", sequence)
	case 2:
		row["role"] = "user"
		row["content"] = fmt.Sprintf("typed %d", sequence)
	case 3:
		row["role"] = "assistant"
		row["agent_id"] = []string{"", "amy", "bob"}[random.Intn(3)]
		row["content"] = []map[string]any{{"type": "text", "text": "answer"}}
	case 4, 5:
		row["role"] = "result"
		row["subtype"] = []string{"success", "error", "interrupted", ""}[random.Intn(4)]
		row["duration_ms"] = random.Intn(3) * 1500
	case 6:
		row["role"] = "system"
		row["result_summary"] = map[string]any{"subtype": "success", "duration_ms": 1200}
	default:
		row[overlayKindField] = []string{"history_rewrite", overlayKindRoomPublicCursor, "room_context_checkpoint"}[random.Intn(3)]
	}
	return row
}

func TestOverlayReplayMatchesFullReadAcrossRandomAppends(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			random := rand.New(rand.NewSource(seed))
			harness.assertMatchesFullRead(nil)
			sequence := 0
			for step := 0; step < 60; step++ {
				rows := make([]map[string]any, 1+random.Intn(4))
				for index := range rows {
					rows[index] = randomOverlayRow(random, sequence)
					sequence++
				}
				harness.appendRows(rows...)
				var active []string
				if random.Intn(2) == 0 {
					active = []string{fmt.Sprintf("round-%02d", random.Intn(24)), "round-missing"}
				}
				harness.assertMatchesFullRead(active)
			}
		})
	}
}

// 一行中断在等值的位置后又有完整行追加：后一行会被解码成前一行的字段值，文件以换行结尾。
// 这不是写到一半的行，读取必须报错，不能悄悄丢掉后一轮并把续读位置推到它之后。
func TestRoundIndexReadRejectsIncompleteRowBeforeFinishedAppend(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	read := func() (protocol.SessionRoundIndex, error) {
		return harness.history.ReadRoundIndex(harness.workspacePath, harness.session, nil)
	}
	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-0","content":"第 0 轮","timestamp":1700000000000}` + "\n")
	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-1","content":`)
	index, err := read()
	if err != nil || len(index.Items) != 1 {
		t.Fatalf("interrupted last row: got %#v, %v", index.Items, err)
	}

	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-2","content":"第 2 轮","timestamp":1700000002000}` + "\n")
	if index, err = read(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("finished append after an interrupted row: got %#v, %v", index.Items, err)
	}
	if entry := harness.replayEntry(); entry.roundIndex.cursor != nil {
		t.Fatalf("a failed read must not leave a resume point: %#v", entry.roundIndex.cursor)
	}
}

// 回放状态里的行会被多次读取共用：真实读取链路并发读、同时有新行追加时，
// 任何读取方都不能改写共用的行，race detector 与事后的整份比对共同把关。
// 行里带上读取链路会改写的嵌套形态：超出内联预算的 tool_result、已有的投递记录与投递回执。
func TestOverlayReplaySharedRowsSurviveConcurrentHistoryReads(t *testing.T) {
	fixture := newLongSessionFixture(t, 12, 64)
	harness := newOverlayReplayHarnessFor(t, fixture)
	largeToolOutput := strings.Repeat("x", historyMessageDetailThresholdBytes)
	appendOverlayRows := func(round int) {
		roundID := fmt.Sprintf("round-%05d", round)
		for _, message := range []protocol.Message{
			{"message_id": "overlay-assistant-" + roundID, "role": "assistant", "round_id": roundID, "agent_id": "amy",
				"content": []map[string]any{
					{"type": "text", "text": "overlay answer"},
					{"type": "tool_result", "tool_use_id": "tool-" + roundID, "content": largeToolOutput},
				},
				"external_deliveries": []map[string]any{{"channel": "im", "target": "earlier"}},
				"timestamp":           1_700_000_000_000 + round*10_000 + 4000},
			{"message_id": "overlay-result-" + roundID, "role": "result", "round_id": roundID, "subtype": "success",
				"duration_ms": 1200, "timestamp": 1_700_000_000_000 + round*10_000 + 5000},
		} {
			if err := fixture.history.AppendOverlayMessage(fixture.workspacePath, fixture.session.SessionKey, message); err != nil {
				t.Fatal(err)
			}
		}
		if err := fixture.history.AppendExternalDeliveryReceipt(fixture.workspacePath, fixture.session.SessionKey, ExternalDeliveryReceipt{
			RoundID: roundID, MessageID: "overlay-assistant-" + roundID, Channel: "im", Target: "later",
			PrimaryPlatformMessageID: "platform-" + roundID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < fixture.rounds; round++ {
		appendOverlayRows(round)
	}
	fixture.readFirstPage(t)

	ctx := context.Background()
	readers := []func() error{
		func() error {
			_, err := fixture.history.ReadMessagesPageContext(ctx, fixture.workspacePath, fixture.session, nil, HistoryPageQuery{Limit: 20})
			return err
		},
		func() error {
			_, err := fixture.history.ReadMessages(fixture.workspacePath, fixture.session, []string{"round-00003"})
			return err
		},
		func() error {
			_, err := fixture.history.ReadRoundIndex(fixture.workspacePath, fixture.session, []string{"round-00003"})
			return err
		},
		func() error {
			_, err := fixture.history.HasCanonicalUserInput(fixture.workspacePath, fixture.session.SessionKey)
			return err
		},
	}
	var group sync.WaitGroup
	failures := make(chan error, len(readers)*8)
	for _, read := range readers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 8 {
				if err := read(); err != nil {
					failures <- err
					return
				}
			}
		}()
	}
	for range 6 {
		round := fixture.rounds
		fixture.appendRound(t)
		appendOverlayRows(round)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	harness.assertMatchesFullRead([]string{"round-00003"})
}
