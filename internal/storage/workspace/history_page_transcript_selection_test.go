// INPUT: 仍存在的低优先级 transcript fallback 与后出现的 canonical transcript。
// OUTPUT: DM/Room persisted history page generation 按 resolver selection identity 失效。
// POS: history page transcript source 优先级切换的回归测试。
package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestHistoryPageIndexInvalidatesFallbackWhenCanonicalTranscriptAppears(t *testing.T) {
	t.Run("dm", func(t *testing.T) {
		configRoot := t.TempDir()
		t.Setenv("NEXUS_STATE_ROOT", "")
		t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))
		workspaceRoot := filepath.Join(configRoot, "workspace")
		workspacePath := longHistoryPageTranscriptWorkspace(workspaceRoot)
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			t.Fatal(err)
		}

		sessionID := "7210e741-b09d-4714-a9c8-adc235129001"
		session := protocol.Session{
			SessionKey: "agent:amy:ws:dm:transcript-selection",
			AgentID:    "amy",
			SessionID:  &sessionID,
			Options:    map[string]any{},
		}
		history := NewAgentHistoryStore(workspaceRoot)
		writeAgentHistoryIndexSession(t, history, workspacePath, session)
		if err := history.AppendRoundMarker(
			workspacePath,
			session.SessionKey,
			"round-selection",
			"选择 transcript",
			1000,
		); err != nil {
			t.Fatal(err)
		}
		fallbackPath, canonicalPath := historyPageTranscriptSelectionPaths(t, workspacePath, sessionID)
		writeHistoryPageSelectionTranscript(t, fallbackPath, sessionID, "fallback answer")

		fallbackPage := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			nil,
			1,
			"",
			0,
			"",
			0,
		)
		assertHistoryPageSelectionText(t, fallbackPage, "fallback answer", "canonical answer")
		access := history.historyPageAccess(workspacePath, session)
		waitHistoryPageIndexFuture(t, access.Scope)

		writeHistoryPageSelectionTranscript(t, canonicalPath, sessionID, "canonical answer")
		canonicalPage := assertAgentHistoryPageParity(
			t,
			history,
			workspacePath,
			session,
			nil,
			1,
			"",
			0,
			"",
			0,
		)
		assertHistoryPageSelectionText(t, canonicalPage, "canonical answer", "fallback answer")
		if _, err := os.Stat(fallbackPath); err != nil {
			t.Fatalf("旧 fallback 应仍存在以证明 selection identity 失效: %v", err)
		}
		waitHistoryPageIndexFuture(t, access.Scope)
	})

	t.Run("room", func(t *testing.T) {
		stateRoot := t.TempDir()
		history := newRoomHistoryTestStore(t, stateRoot)
		ownerUserID := testRoomOwnerUserID
		conversationID := "room-transcript-selection"
		workspacePath := longHistoryPageTranscriptWorkspace(
			appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		)
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			t.Fatal(err)
		}

		sessionID := "7210e741-b09d-4714-a9c8-adc235129002"
		privateSessionKey := "agent:amy:ws:group:" + conversationID
		ownerHistory := history.agentHistory.ForOwner(ownerUserID)
		writeAgentHistoryIndexSession(t, ownerHistory, workspacePath, protocol.Session{
			SessionKey: privateSessionKey,
			AgentID:    "amy",
			SessionID:  &sessionID,
		})
		if err := ownerHistory.AppendRoundMarker(
			workspacePath,
			privateSessionKey,
			"private-selection",
			"选择 transcript",
			1000,
		); err != nil {
			t.Fatal(err)
		}
		fallbackPath, canonicalPath := historyPageTranscriptSelectionPaths(t, workspacePath, sessionID)
		writeHistoryPageSelectionTranscript(t, fallbackPath, sessionID, "fallback answer")
		if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
			"message_id": "room-selection-user", "round_id": "root-selection", "role": "user",
			"content": "选择 transcript", "timestamp": int64(1000),
		}); err != nil {
			t.Fatal(err)
		}
		if err := history.AppendTranscriptReference(
			ownerUserID,
			conversationID,
			workspacePath,
			privateSessionKey,
			protocol.Message{
				"message_id": "selection-assistant", "session_id": sessionID,
				"conversation_id": conversationID, "agent_id": "amy",
				"round_id": "root-selection:amy", "agent_round_id": "private-selection",
				"role": "assistant", "is_complete": true, "timestamp": int64(1100),
			},
		); err != nil {
			t.Fatal(err)
		}

		fallbackPage := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			nil,
			1,
			"",
			0,
			"",
			0,
		)
		assertHistoryPageSelectionText(t, fallbackPage, "fallback answer", "canonical answer")
		access := history.historyPageAccess(ownerUserID, conversationID)
		waitHistoryPageIndexFuture(t, access.Scope)

		writeHistoryPageSelectionTranscript(t, canonicalPath, sessionID, "canonical answer")
		canonicalPage := assertRoomHistoryPageParity(
			t,
			history,
			ownerUserID,
			conversationID,
			nil,
			1,
			"",
			0,
			"",
			0,
		)
		assertHistoryPageSelectionText(t, canonicalPage, "canonical answer", "fallback answer")
		if _, err := os.Stat(fallbackPath); err != nil {
			t.Fatalf("旧 Room fallback 应仍存在以证明 selection identity 失效: %v", err)
		}
		waitHistoryPageIndexFuture(t, access.Scope)
	})
}

func longHistoryPageTranscriptWorkspace(root string) string {
	// owner workspace 必须是 workspace/ 的直接子目录；单个合法长目录名已足以
	// 触发 transcript project 的 legacy prefix fallback 路径。
	return filepath.Join(root, strings.Repeat("a", 220))
}

func historyPageTranscriptSelectionPaths(
	t *testing.T,
	workspacePath string,
	sessionID string,
) (string, string) {
	t.Helper()
	projectName := TranscriptProjectDirectoryName(workspacePath)
	if len(projectName) <= maxTranscriptSanitizedLength {
		t.Fatalf("fixture 未触发 transcript fallback 目录: len=%d", len(projectName))
	}
	projectsRoot := transcriptProjectsDirForWorkspace(workspacePath)
	fallbackDir := filepath.Join(
		projectsRoot,
		projectName[:maxTranscriptSanitizedLength]+"-legacy-selection",
	)
	canonicalDir := filepath.Join(projectsRoot, projectName)
	return filepath.Join(fallbackDir, sessionID+".jsonl"),
		filepath.Join(canonicalDir, sessionID+".jsonl")
}

func writeHistoryPageSelectionTranscript(
	t *testing.T,
	path string,
	sessionID string,
	answer string,
) {
	t.Helper()
	writeTranscriptArtifactRows(t, path, []map[string]any{
		{
			"type": "user", "uuid": "selection-user", "sessionId": sessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"message":   map[string]any{"role": "user", "content": "选择 transcript"},
		},
		{
			"type": "assistant", "uuid": "selection-assistant-entry", "sessionId": sessionID,
			"parentUuid": "selection-user", "timestamp": "2026-08-17T00:00:01Z",
			"message": map[string]any{
				"id": "selection-assistant", "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": answer}},
			},
		},
	})
}

func assertHistoryPageSelectionText(
	t *testing.T,
	page protocol.MessagePage,
	want string,
	unwanted string,
) {
	t.Helper()
	foundWant := false
	for _, row := range page.Items {
		if messageContentContainsText(row["content"], want) {
			foundWant = true
		}
		if messageContentContainsText(row["content"], unwanted) {
			t.Fatalf("page 仍包含低优先级 transcript 内容 %q: %+v", unwanted, page.Items)
		}
	}
	if !foundWant {
		t.Fatalf("page 缺少选中 transcript 内容 %q: %+v", want, page.Items)
	}
}
