// history_pagination_test.go 覆盖历史分页索引、轮次索引、改写与 Overlay 回放测试。
package workspace

import (
	"bytes"
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
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryPageIndexMatchesCanonicalPagination(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-parity", AgentID: "amy"}
	history := NewAgentHistoryStore(root)
	writeAgentHistoryIndexSession(t, history, workspacePath, session)

	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-1", 1000)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-2", 2000)
	// 同起点的两轮在 canonical 里必然交错（未收口轮次的 synthetic interrupt 落在 +1），canonical 会拆成碎片分组；
	// 那种情况按整轮分页、偏离 canonical，见 TestHistoryPageKeepsInterleavedRoundsWhole。这里用不交错的时间戳。
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-z", 3000)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-a", 3002)
	appendAgentHistoryIndexRound(t, history, workspacePath, session.SessionKey, "round-5", 4000)

	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 2, "", 0, "", 0)
	// 第二次必须走 persisted selected-group seek，JSON wire 仍与 canonical oracle 相同。
	latest := assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 2, "", 0, "", 0)
	assertAgentHistoryPageParity(
		t,
		history,
		workspacePath,
		session,
		nil,
		2,
		derefString(latest.NextBeforeRoundID),
		derefInt64(latest.NextBeforeRoundTimestamp),
		"",
		0,
	)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 0, "", 0, "round-z", 1)
}

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

func TestHistoryPageIndexActiveRoundsSuppressSyntheticInterrupt(t *testing.T) {
	t.Run("dm", func(t *testing.T) {
		root := t.TempDir()
		workspacePath := filepath.Join(root, "Amy")
		session := protocol.Session{SessionKey: "agent:amy:ws:dm:index-active", AgentID: "amy"}
		history := NewAgentHistoryStore(root)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		if err := history.AppendOverlayMessage(workspacePath, session.SessionKey, protocol.Message{
			"message_id": "active-user", "round_id": "dm-active", "role": "user",
			"content": "仍在运行", "timestamp": int64(1000),
		}); err != nil {
			t.Fatal(err)
		}
		active := assertAgentHistoryPageParity(t, history, workspacePath, session, []string{"dm-active"}, 1, "", 0, "", 0)
		if historyPageHasMessage(active, "assistant_interrupt_dm-active") {
			t.Fatalf("active DM 不应显示 synthetic interrupt: %+v", active.Items)
		}
		inactive := assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
		if !historyPageHasMessage(inactive, "assistant_interrupt_dm-active") {
			t.Fatalf("inactive DM 应恢复 synthetic interrupt: %+v", inactive.Items)
		}
	})

	t.Run("room_physical_subround", func(t *testing.T) {
		root := t.TempDir()
		history := newRoomHistoryTestStore(t, root)
		ownerUserID := testRoomOwnerUserID
		conversationID := "room-index-active"
		if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
			"message_id": "room-active-user", "round_id": "root-active:amy", "agent_id": "amy",
			"role": "user", "content": "仍在运行", "timestamp": int64(1000),
		}); err != nil {
			t.Fatal(err)
		}
		active := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			[]string{"root-active:amy"},
			1,
			"",
			0,
			"",
			0,
		)
		if historyPageHasMessage(active, "assistant_interrupt_root-active:amy") {
			t.Fatalf("active Room physical subround 不应显示 synthetic interrupt: %+v", active.Items)
		}
		inactive := assertRoomHistoryPageParity(t, history, ownerUserID, conversationID, nil, 1, "", 0, "", 0)
		if !historyPageHasMessage(inactive, "assistant_interrupt_root-active:amy") {
			t.Fatalf("inactive Room physical subround 应恢复 synthetic interrupt: %+v", inactive.Items)
		}
	})
}

// TestHistoryPageKeepsInterleavedRoundsWhole 固定一个有意偏离 canonical 的语义：Room 里上一轮成员还在回复时
// 用户又发了新消息，两轮的行按时间交错。canonical 先全局排序再按相邻 round 分组，会把两轮拆成碎片，一页里
// 可能只有半轮（只有回复没有提问）；表按根轮次分组，一页总是完整轮次，与前端按 round 驻留和淘汰的窗口一致。
// 同起点的两轮按追加顺序排（root-z 先于 root-a），游标翻页不能因 round_id 字典序跳过 root-z。
func TestHistoryPageKeepsInterleavedRoundsWhole(t *testing.T) {
	root := t.TempDir()
	history := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "room-interleaved"
	appendRoomHistoryIndexRound(t, history, owner, conversation, "root-1", "amy", 1000)
	appendRoomHistoryIndexRound(t, history, owner, conversation, "root-z", "amy", 2000)
	appendRoomHistoryIndexRound(t, history, owner, conversation, "root-a", "amy", 2000)

	latest, err := history.ReadMessagesPageContext(context.Background(), owner, conversation, nil, HistoryPageQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(latest.Items))
	for _, item := range latest.Items {
		if stringFromAny(item["round_id"]) != "root-a" {
			t.Fatalf("最新一页混入了其他轮次: %+v", latest.Items)
		}
		ids = append(ids, stringFromAny(item["message_id"]))
	}
	if len(ids) == 0 || ids[0] != "user-root-a" {
		t.Fatalf("最新一页应从 root-a 的提问开始: %v", ids)
	}
	if !latest.HasMore || derefString(latest.NextBeforeRoundID) != "root-a" || derefInt64(latest.NextBeforeRoundTimestamp) != 2000 {
		t.Fatalf("游标应指向 root-a 的起点: %+v", latest)
	}
	previous, err := history.ReadMessagesPageContext(context.Background(), owner, conversation, nil, HistoryPageQuery{
		Limit: 1, BeforeRoundID: "root-a", BeforeRoundTimestamp: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(previous.Items) == 0 || stringFromAny(previous.Items[0]["message_id"]) != "user-root-z" {
		t.Fatalf("上一页应是完整的 root-z: %+v", previous.Items)
	}
	for _, item := range previous.Items {
		if stringFromAny(item["round_id"]) != "root-z" {
			t.Fatalf("上一页混入了其他轮次: %+v", previous.Items)
		}
	}
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

func historyPageHasMessage(page protocol.MessagePage, messageID string) bool {
	for _, item := range page.Items {
		if stringFromAny(item["message_id"]) == messageID {
			return true
		}
	}
	return false
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

func TestPaginateNormalizedHistoryRowsCollapsesSuffixedRoomMarker(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "room_mention_abc:agent-a",
			"round_id":   "room_mention_abc:agent-a",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "result-agent-a",
			"round_id":   "room_mention_abc:agent-a",
			"agent_id":   "agent-a",
			"role":       "result",
			"timestamp":  2000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, true)
	if page.HasMore {
		t.Fatalf("只有一个归一后的 room round，不应还有更多历史: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("带 agent 后缀的 marker/result 应同页返回: got=%d", len(page.Items))
	}
	for _, item := range page.Items {
		if item["round_id"] != "room_mention_abc" {
			t.Fatalf("返回给前端的 round_id 应已归一: %+v", page.Items)
		}
	}
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

func TestAgentHistoryStoreReadRoundIndexIgnoresLargeResultBody(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Amy")
	sessionKey := "agent:amy:ws:dm:large"
	history := NewAgentHistoryStore(root)

	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-large", "长回复", 1000); err != nil {
		t.Fatalf("写入 marker 失败: %v", err)
	}
	overlayPath := New(root).SessionOverlayPath(workspacePath, sessionKey)
	file, err := os.OpenFile(overlayPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("打开 overlay 失败: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(`{"role":"result","round_id":"round-large","subtype":"success","duration_ms":100,"result":"` + strings.Repeat("x", transcriptScannerBufferBytes+1) + `"}` + "\n"); err != nil {
		t.Fatalf("写入长 result 失败: %v", err)
	}

	index, err := history.ReadRoundIndex(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "amy",
	}, nil)
	if err != nil {
		t.Fatalf("读取长 result round index 失败: %v", err)
	}
	if len(index.Items) != 1 || index.Items[0].RoundID != "round-large" {
		t.Fatalf("长 result 后索引不正确: %+v", index)
	}
	if index.Items[0].Status != "success" || index.Items[0].DurationMS == nil || *index.Items[0].DurationMS != 100 {
		t.Fatalf("长 result 元数据未保留: %+v", index.Items[0])
	}
}

func newRoomHistoryTestStore(t *testing.T, stateRoot string) *RoomHistoryStore {
	t.Helper()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	return NewRoomHistoryStore(filepath.Join(stateRoot, "users"))
}

func TestAgentHistoryStoreRemoveOverlayRounds(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	sessionKey := "agent:nexus:ws:dm:rewrite-prune"
	history := NewAgentHistoryStore(root)

	appendTestRound(t, history, workspacePath, sessionKey, "round-1", "第一问", "第一答", 1000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-2", "旧问题", "旧回答", 2000)
	appendTestRound(t, history, workspacePath, sessionKey, "round-3", "新问题", "新回答", 3100)

	removed, err := history.RemoveOverlayRounds(workspacePath, sessionKey, []string{"round-2"})
	if err != nil {
		t.Fatalf("裁剪 overlay round 失败: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}

	rows, err := history.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	if hasRound(rows, "round-2") {
		t.Fatalf("被裁剪 round 不应进入有效历史: %+v", rows)
	}
	if !hasRound(rows, "round-1") || !hasRound(rows, "round-3") {
		t.Fatalf("裁剪后应保留其他 round: %+v", rows)
	}

	index, err := history.ReadRoundIndex(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "nexus",
	}, nil)
	if err != nil {
		t.Fatalf("读取 round index 失败: %v", err)
	}
	if len(index.Items) != 2 {
		t.Fatalf("round index 数量不正确: %+v", index.Items)
	}
	for _, item := range index.Items {
		if item.RoundID == "round-2" {
			t.Fatalf("round index 不应包含被替换 round: %+v", index.Items)
		}
	}
}

func appendTestRound(
	t *testing.T,
	history *AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	roundID string,
	userContent string,
	assistantContent string,
	timestamp int64,
) {
	t.Helper()
	if err := history.AppendRoundMarker(workspacePath, sessionKey, roundID, userContent, timestamp); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":  "assistant-" + roundID,
		"session_key": sessionKey,
		"agent_id":    "nexus",
		"round_id":    roundID,
		"role":        "assistant",
		"content": []map[string]any{
			{"type": "text", "text": assistantContent},
		},
		"timestamp": timestamp + 100,
	}); err != nil {
		t.Fatalf("写入 assistant overlay 失败: %v", err)
	}
}

func hasRound(rows []protocol.Message, roundID string) bool {
	for _, row := range rows {
		if stringFromAny(row["round_id"]) == roundID {
			return true
		}
	}
	return false
}

func TestAgentHistoryStoreResolveTranscriptRoundTailIncludesInterruptedToolResults(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "runtime"))

	sessionKey := "agent:nexus:ws:dm:rewrite-interrupted-tools"
	sessionID := "session-rewrite-interrupted-tools"
	history := NewAgentHistoryStore(workspaceRoot)
	for index, round := range []struct{ id, prompt string }{
		{"round-before", "只回复准备好了"},
		{"round-tools", "先执行 echo ready，完成后再执行 sleep 60"},
	} {
		if err := history.AppendRoundMarker(workspacePath, sessionKey, round.id, round.prompt, int64(index+1)*1000); err != nil {
			t.Fatal(err)
		}
	}

	// Use persisted transcript UUIDs: tool results are user messages but must
	// stay inside the same round tail as both independent assistant tool calls.
	entries := []struct {
		uuid    string
		role    string
		content any
	}{
		{"user-before", "user", "只回复准备好了"},
		{"assistant-before", "assistant", []map[string]any{{"type": "text", "text": "准备好了"}}},
		{"user-tools", "user", "先执行 echo ready，完成后再执行 sleep 60"},
		{"assistant-echo", "assistant", []map[string]any{{
			"type": "tool_use", "id": "call-echo", "name": "Bash", "input": map[string]any{"command": "echo ready"},
		}}},
		{"result-echo", "user", []map[string]any{{
			"type": "tool_result", "tool_use_id": "call-echo", "content": "ready", "is_error": false,
		}}},
		{"assistant-sleep", "assistant", []map[string]any{{
			"type": "tool_use", "id": "call-sleep", "name": "Bash", "input": map[string]any{"command": "sleep 60"},
		}}},
		{"result-interrupted", "user", []map[string]any{{
			"type": "tool_result", "tool_use_id": "call-sleep", "content": "[Request interrupted by user]", "is_error": true,
		}}},
	}
	rows := make([]map[string]any, 0, len(entries))
	parentUUID := ""
	for index, entry := range entries {
		rows = append(rows, map[string]any{
			"type": entry.role, "uuid": entry.uuid, "sessionId": sessionID,
			"parentUuid": parentUUID, "timestamp": int64(index+1) * 1000,
			"message": map[string]any{"role": entry.role, "content": entry.content},
		})
		parentUUID = entry.uuid
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, rows)

	tail, err := history.ResolveTranscriptRoundTail(workspacePath, sessionKey, sessionID, "round-tools")
	if err != nil {
		t.Fatal(err)
	}
	want := TranscriptRoundTail{
		TargetRoundID:      "round-tools",
		TargetMessageUUID:  "user-tools",
		TargetRoundEndUUID: "result-interrupted",
		MessageUUIDs:       []string{"user-tools", "assistant-echo", "result-echo", "assistant-sleep", "result-interrupted"},
		RoundIDs:           []string{"round-tools"},
	}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("rewrite must remove both tool pairs and preserve the preceding text round: got %#v, want %#v", tail, want)
	}
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

func TestOverlayReplayParsesOnlyAppendedRows(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	for round := 0; round < 50; round++ {
		harness.appendRows(
			map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%03d", round), "content": "hi", "timestamp": 1_700_000_000_000 + round*1000},
			map[string]any{"role": "result", "round_id": fmt.Sprintf("round-%03d", round), "message_id": fmt.Sprintf("result-%03d", round), "subtype": "success", "timestamp": 1_700_000_000_500 + round*1000},
		)
	}
	harness.assertMatchesFullRead(nil)
	info, err := os.Stat(harness.overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-050", "content": "next", "timestamp": 1_700_000_050_000})

	file, err := os.Open(harness.overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entry := harness.replayEntry()
	for name, cursor := range map[string]*overlayReplayCursor{"history": entry.history.cursor, "round index": entry.roundIndex.cursor} {
		span, err := openOverlayReplaySpan(context.Background(), file, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !span.Resumed || span.Reader.Size() >= info.Size() || span.Next.Size-span.Reader.Size() != info.Size() {
			t.Fatalf("%s replay should resume at the previous end %d: resumed=%t span=%d", name, info.Size(), span.Resumed, span.Reader.Size())
		}
	}
	harness.assertMatchesFullRead(nil)
}

func TestOverlayReplayFallsBackToFullRead(t *testing.T) {
	seed := func(harness *overlayReplayHarness) {
		for round := 0; round < 6; round++ {
			harness.appendRows(
				map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%d", round), "content": fmt.Sprintf("prompt %d", round), "timestamp": 1_700_000_000_000 + round*1000},
				map[string]any{"role": "result", "round_id": fmt.Sprintf("round-%d", round), "message_id": fmt.Sprintf("result-%d", round), "subtype": "success", "duration_ms": 900, "timestamp": 1_700_000_000_500 + round*1000},
			)
		}
		harness.assertMatchesFullRead(nil)
	}
	rewrite := func(harness *overlayReplayHarness, edit func(lines []string) []string) {
		harness.t.Helper()
		payload, err := os.ReadFile(harness.overlayPath)
		if err != nil {
			harness.t.Fatal(err)
		}
		lines := edit(strings.SplitAfter(strings.TrimSuffix(string(payload), "\n"), "\n"))
		if err = os.WriteFile(harness.overlayPath, []byte(strings.Join(lines, "")+"\n"), 0o644); err != nil {
			harness.t.Fatal(err)
		}
	}
	cases := map[string]func(harness *overlayReplayHarness){
		"later row overwrites an applied message_id": func(harness *overlayReplayHarness) {
			harness.appendRows(map[string]any{"role": "result", "round_id": "round-9", "message_id": "result-2", "subtype": "error", "timestamp": 1_700_000_090_000})
		},
		"file replaced by a new file": func(harness *overlayReplayHarness) {
			replacement := harness.overlayPath + ".next"
			if err := os.WriteFile(replacement, []byte(`{"nexus_overlay_kind":"round_marker","round_id":"round-new","content":"fresh","timestamp":1700000000000}`+"\n"), 0o644); err != nil {
				harness.t.Fatal(err)
			}
			if err := os.Rename(replacement, harness.overlayPath); err != nil {
				harness.t.Fatal(err)
			}
		},
		"truncated in place": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string { return lines[:4] })
		},
		"rewritten in place with more rows": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string {
				lines[len(lines)-1] = `{"role":"result","round_id":"round-5","message_id":"result-5","subtype":"error","timestamp":1700000005500}`
				return append(lines, "\n"+`{"nexus_overlay_kind":"round_marker","round_id":"round-6","content":"prompt 6","timestamp":1700000006000}`)
			})
		},
		"reordered in place at the same size": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string {
				last := len(lines) - 1
				lines[last-2], lines[last] = strings.TrimSuffix(lines[last], "\n")+"\n", strings.TrimSuffix(lines[last-2], "\n")
				return lines
			})
		},
		"removed": func(harness *overlayReplayHarness) {
			if err := os.Remove(harness.overlayPath); err != nil {
				harness.t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			seed(harness)
			mutate(harness)
			harness.assertMatchesFullRead(nil)
			harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-after", "content": "after", "timestamp": 1_700_000_100_000})
			harness.assertMatchesFullRead([]string{"round-after"})
		})
	}
}

// 边缘指纹只覆盖首尾各 4KB；中段被改写时要靠文件身份与同尺寸下的修改时间发现。
func TestOverlayReplayDetectsRewritesOutsideTheEdgeFingerprint(t *testing.T) {
	cases := map[string]func(t *testing.T, path string, rewritten []byte){
		"same size rewritten in place": func(t *testing.T, path string, rewritten []byte) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, rewritten, 0o644); err != nil {
				t.Fatal(err)
			}
			if err = os.Chtimes(path, info.ModTime().Add(2*time.Second), info.ModTime().Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
		},
		"replaced by a longer file with the same edges": func(t *testing.T, path string, rewritten []byte) {
			rewritten = append(rewritten, `{"nexus_overlay_kind":"round_marker","round_id":"round-new","content":"new","timestamp":1700009000000}`+"\n"...)
			if err := os.WriteFile(path+".next", rewritten, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".next", path); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			for round := 0; round < 200; round++ {
				harness.appendRows(
					map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%03d", round), "content": fmt.Sprintf("prompt %03d", round), "timestamp": 1_700_000_000_000 + round*1000},
					map[string]any{"role": "assistant", "round_id": fmt.Sprintf("round-%03d", round), "message_id": fmt.Sprintf("answer-%03d", round), "timestamp": 1_700_000_000_500 + round*1000},
				)
			}
			harness.assertMatchesFullRead(nil)
			payload, err := os.ReadFile(harness.overlayPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) < 4*historyPageEdgeHashBytes {
				t.Fatalf("fixture too small to hide a rewrite from the edge fingerprint: %d bytes", len(payload))
			}
			rewritten := bytes.Replace(payload, []byte("prompt 100"), []byte("edited 100"), 1)
			rewritten = bytes.Replace(rewritten, []byte("answer-100"), []byte("answer-1xx"), 1)
			if bytes.Equal(rewritten, payload) {
				t.Fatal("fixture rewrite did not change the overlay")
			}
			mutate(t, harness.overlayPath, rewritten)
			harness.assertMatchesFullRead(nil)
		})
	}
}

func TestOverlayReplayKeepsNoStateForPartiallyWrittenLine(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-0", "content": "first", "timestamp": 1_700_000_000_000})
	harness.assertMatchesFullRead(nil)

	complete := `{"nexus_overlay_kind":"round_marker","round_id":"round-1","content":"second","timestamp":1700000001000}`
	harness.appendRaw(complete[:40])
	state, err := harness.history.readOverlayHistoryStateContext(context.Background(), harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RoundMarkers) != 1 {
		t.Fatalf("half-written marker must not be visible: %#v", state.RoundMarkers)
	}
	if entry := harness.replayEntry(); entry.history.cursor != nil {
		t.Fatalf("a read ending mid-line must not leave a resume point: %#v", entry.history.cursor)
	}

	harness.appendRaw(complete[40:] + "\n")
	harness.assertMatchesFullRead(nil)
	state, err = harness.history.readOverlayHistoryStateContext(context.Background(), harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RoundMarkers) != 2 || state.RoundMarkers[1].Content != "second" {
		t.Fatalf("completed marker missing after the line was finished: %#v", state.RoundMarkers)
	}
}

// 追加方把一行写到一半时，导航读取返回已写完的行，不把半行当成读取失败；
// 这一行写完后再读必须出现。
func TestRoundIndexReadSkipsPartiallyWrittenLastRow(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	marker := func(round int) string {
		return fmt.Sprintf(`{"nexus_overlay_kind":"round_marker","round_id":"round-%d","content":"第 %d 轮","timestamp":%d}`,
			round, round, 1_700_000_000_000+round*1000)
	}
	readRounds := func() []string {
		t.Helper()
		index, err := harness.history.ReadRoundIndex(harness.workspacePath, harness.session, nil)
		if err != nil {
			t.Fatalf("round index read: %v", err)
		}
		rounds := make([]string, len(index.Items))
		for i, item := range index.Items {
			rounds[i] = item.RoundID
		}
		return rounds
	}
	harness.appendRaw(marker(0) + "\n")
	want := []string{"round-0"}

	// 断点依次落在行首、多字节字符中间和只差收尾括号处。
	for index, cut := range []func(row string) int{
		func(string) int { return 1 },
		func(row string) int { return strings.Index(row, "轮") + 1 },
		func(row string) int { return len(row) - 1 },
	} {
		if got := readRounds(); !reflect.DeepEqual(got, want) {
			t.Fatalf("complete rows: got %v want %v", got, want)
		}
		round := index + 1
		row := marker(round)
		written := cut(row)
		harness.appendRaw(row[:written])
		if got := readRounds(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%d of %d bytes of the next row written: got %v want %v", written, len(row), got, want)
		}
		if entry := harness.replayEntry(); entry.roundIndex.cursor != nil {
			t.Fatalf("a read ending mid-row must not leave a resume point: %#v", entry.roundIndex.cursor)
		}
		harness.assertMatchesFullRead(nil)

		harness.appendRaw(row[written:] + "\n")
		want = append(want, fmt.Sprintf("round-%d", round))
		harness.assertMatchesFullRead(nil)
	}
	if got := readRounds(); !reflect.DeepEqual(got, want) {
		t.Fatalf("finished rows: got %v want %v", got, want)
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

func TestOverlayReplayResultIsIsolatedFromCallers(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-0", "content": "first", "timestamp": 1_700_000_000_000})
	for index, agentID := range []string{"amy", "bob", "cat"} {
		harness.appendRows(map[string]any{"role": "assistant", "round_id": "round-0", "agent_id": agentID, "message_id": fmt.Sprintf("a-%d", index), "timestamp": 1_700_000_000_100 + index})
	}
	ctx := context.Background()
	state, err := harness.history.readOverlayHistoryStateContext(ctx, harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	index, err := harness.history.readCanonicalRoundIndexContext(ctx, harness.workspacePath, harness.session, []string{"round-0"})
	if err != nil {
		t.Fatal(err)
	}

	harness.appendRows(
		map[string]any{"role": "assistant", "round_id": "round-0", "agent_id": "dan", "message_id": "a-3", "timestamp": 1_700_000_000_200},
		map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-1", "content": "second", "timestamp": 1_700_000_001_000},
	)
	harness.assertMatchesFullRead(nil)

	// 旧结果上的 append 与条目改写都不能进入回放状态。
	_ = append(state.MessageRows, protocol.Message{"message_id": "caller-row"})
	_ = append(state.RoundMarkers, transcriptRoundMarker{RoundID: "caller-marker"})
	_ = append(index.Items[0].AgentIDs, "caller-agent")
	index.Items[0].Title = "caller edit"
	harness.assertMatchesFullRead(nil)
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

func TestOverlayReplayStoreEvictsLeastRecentlyUsedWithinBudget(t *testing.T) {
	var store overlayReplayStore
	retain := func(key string, size int64) {
		if err := store.replay(key, func(entry *overlayReplayEntry) error {
			entry.history.cursor = &overlayReplayCursor{Size: size}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	retain("old", overlayReplayBudgetBytes/2)
	retain("recent", overlayReplayBudgetBytes/2)
	retain("old", overlayReplayBudgetBytes/2)
	retain("new", overlayReplayBudgetBytes/2)
	if _, ok := store.entries["recent"]; ok {
		t.Fatal("least recently used overlay state should be evicted first")
	}
	if len(store.entries) != 2 || store.entries["old"] == nil || store.entries["new"] == nil {
		t.Fatalf("unexpected retained overlays: %v", store.order)
	}
	for index := 0; index < maxOverlayReplaySessions+8; index++ {
		retain(fmt.Sprintf("small-%d", index), 1)
	}
	if len(store.entries) != maxOverlayReplaySessions || len(store.order) != maxOverlayReplaySessions {
		t.Fatalf("retained %d overlays, want %d", len(store.entries), maxOverlayReplaySessions)
	}
}
