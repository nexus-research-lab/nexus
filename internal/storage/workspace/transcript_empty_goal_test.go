package workspace

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreEmptyGoalTurnKeepsReportInItsOwnRound(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Nova")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "runtime"))
	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:nova:ws:dm:empty-goal"
	sessionID := "empty-goal-transcript"
	const goalRound = "goal-continuation"
	const report = "M3 report delivered"
	const formula = `Formula rendered: $$K_D = \frac{[A][B]}{[AB]}$$`
	for _, marker := range []struct {
		id      string
		content string
		ts      int64
		options RoundMarkerOptions
	}{
		{"old-round", "Render the formula", 1000, RoundMarkerOptions{}},
		{"goal-command", "/goal Research M3", 86401000, RoundMarkerOptions{ControlOnly: true, Purpose: "goal_command"}},
		{goalRound, "", 86401100, RoundMarkerOptions{HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}},
		{"next-round", "Thanks", 86405000, RoundMarkerOptions{}},
	} {
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, marker.id, marker.content, marker.ts, marker.options); err != nil {
			t.Fatal(err)
		}
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{"type": "user", "uuid": "old-user", "sessionId": sessionID, "timestamp": "1970-01-01T00:00:01.005Z", "message": map[string]any{"role": "user", "content": "Render the formula"}},
		{"type": "assistant", "uuid": "old-answer", "parentUuid": "old-user", "sessionId": sessionID, "timestamp": "1970-01-01T00:00:02Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": formula}}}},
		// nxs extracts the hidden Goal reminder before writing this empty user turn.
		{"type": "user", "uuid": "goal-user", "parentUuid": "old-answer", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:01.105Z", "message": map[string]any{"role": "user", "content": ""}},
		{"type": "assistant", "uuid": "goal-progress", "parentUuid": "goal-user", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:02Z", "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "Researching M3"}}}},
		{"type": "assistant", "uuid": "goal-answer", "parentUuid": "goal-progress", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:03Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": report}}}},
		{"type": "user", "uuid": "next-user", "parentUuid": "goal-answer", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:05.005Z", "message": map[string]any{"role": "user", "content": "Thanks"}},
		{"type": "assistant", "uuid": "next-answer", "parentUuid": "next-user", "sessionId": sessionID, "timestamp": "1970-01-02T00:00:06Z", "message": map[string]any{"role": "assistant", "stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": "Welcome"}}}},
	})
	for _, row := range []protocol.Message{
		{"message_id": "goal-result", "round_id": goalRound, "role": "result", "subtype": "success", "result": report, "timestamp": int64(86404000)},
		{"message_id": "goal-answer", "round_id": goalRound, "role": "assistant", "content": []map[string]any{{"type": "text", "text": report}}, "timestamp": int64(86403000), protocol.GoalCompletionReceiptField: protocol.GoalCompletionReceipt{GoalID: "goal-1", RoundID: goalRound}},
	} {
		if err := history.AppendOverlayMessage(workspacePath, sessionKey, row); err != nil {
			t.Fatal(err)
		}
	}
	session := protocol.Session{SessionKey: sessionKey, AgentID: "Nova", SessionID: &sessionID, Options: map[string]any{}}
	assertRows := func(t *testing.T, rows []protocol.Message) {
		t.Helper()
		reports := 0
		formulas := 0
		for _, row := range rows {
			id := stringFromAny(row["message_id"])
			wantRound := map[string]string{"old-answer": "old-round", "goal-progress": goalRound, "goal-answer": goalRound, "next-answer": "next-round"}[id]
			if wantRound != "" && row["round_id"] != wantRound {
				t.Errorf("%s round = %v, want %s", id, row["round_id"], wantRound)
			}
			if row["role"] == "user" && row["round_id"] == goalRound {
				t.Error("hidden Goal input must not become a visible user message")
			}
			for _, block := range normalizeMessageContentBlocks(row["content"]) {
				if block["text"] == report {
					reports++
				}
				if block["text"] == formula && row["round_id"] == "old-round" {
					formulas++
				}
			}
			if id == "goal-answer" && (row["result_summary"] == nil || row[protocol.GoalCompletionReceiptField] == nil) {
				t.Error("Goal result and completion receipt must attach to the original report")
			}
		}
		if reports != 1 {
			t.Errorf("report copies = %d, want 1", reports)
		}
		if formulas != 1 {
			t.Errorf("formula replies in the original round = %d, want 1", formulas)
		}
	}
	rows, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, rows)
	for range 2 { // Both freshly built and cached pagination must preserve the boundary.
		page, err := history.ReadMessagesPageContext(context.Background(), workspacePath, session, nil, HistoryPageQuery{Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		assertRows(t, page.Items)
	}
	tail, err := history.ResolveTranscriptRoundTail(workspacePath, sessionKey, sessionID, goalRound)
	if err != nil {
		t.Fatal(err)
	}
	if tail.TargetMessageUUID != "goal-user" || tail.TargetRoundEndUUID != "goal-answer" {
		t.Fatalf("Goal rewrite boundary = %+v", tail)
	}
}

func TestEmptyTranscriptGoalMarkerMatchingBoundaries(t *testing.T) {
	goalMarker := transcriptRoundMarker{RoundID: "goal", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}
	for _, tc := range []struct {
		name       string
		timestamps []string
		markers    []transcriptRoundMarker
		wantIndex  int
	}{
		{"empty_goal", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 0},
		{"ordinary_empty_without_marker", []string{"1970-01-02T00:00:01.005Z"}, nil, -1},
		{"visible_marker", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "visible", Timestamp: 86401000}}, -1},
		{"other_hidden_input", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "echo", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "echo_followup"}}, -1},
		{"missing_timestamp", []string{""}, []transcriptRoundMarker{goalMarker}, -1},
		{"stale_marker", []string{"1970-01-03T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"future_marker", []string{"1970-01-01T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"ambiguous_markers", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker, goalMarker}, -1},
		{"incidental_empty_before_goal", []string{"1970-01-02T00:00:00Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 1},
		{"ambiguous_empty_inputs", []string{"1970-01-02T00:00:01.005Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := make([]transcriptEntry, len(tc.timestamps))
			for i, timestamp := range tc.timestamps {
				chain[i] = transcriptEntry{Index: i, Data: map[string]any{"type": "user", "timestamp": timestamp, "message": map[string]any{"role": "user", "content": ""}}}
			}
			aligned := alignTranscriptRoundMarkers(chain, tc.markers)
			for i := range chain {
				got := i < len(aligned) && transcriptRoundMarkerPresent(aligned[i])
				if got != (i == tc.wantIndex) {
					t.Errorf("empty input %d matched = %v, want %v", i, got, i == tc.wantIndex)
				}
			}
		})
	}
}

func TestHistoryReadModelDiscardsOldGoalRoundProjection(t *testing.T) {
	model := &historyReadModel{path: filepath.Join(t.TempDir(), historyReadModelFileName)}
	db, err := model.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`INSERT INTO history_read_scopes
		(scope, schema_version, generation, group_count, sources_json, round_index_json, accessed_at_ms)
		VALUES ('old-goal-rounds', 4, 'old-generation', 0, '[]', '[]', 0);
		PRAGMA user_version = 4;`); err != nil {
		t.Fatal(err)
	}
	if err := initializeHistoryReadModel(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_read_scopes`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale Goal round projections retained = %d", count)
	}
}
