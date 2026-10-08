package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
