// session_and_paths_test.go 覆盖会话文件、Owner 路径与工作区路径解析测试。
package workspace

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSessionDeletionLifecycleAbortRestoresWriterAdmission(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:delete-abort"
	now := time.Now().UTC()
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "keep me",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.BeginSessionDeletion(
		workspacePath,
		sessionKey,
		created.ConfigurationVersion,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbortSessionDeletion(lease); err != nil {
		t.Fatal(err)
	}
	next := *created
	next.MessageCount++
	updated, err := store.UpsertSession(workspacePath, next)
	if err != nil || updated == nil || updated.MessageCount != 1 {
		t.Fatalf("abort did not restore writer admission: item=%+v err=%v", updated, err)
	}
}

const testRoomOwnerUserID = "user-room-test"

func TestTranscriptProjectHashSuffixMatchesBunHashFixtures(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: "27k1wwwhf13t"},
		{name: "ascii", input: "abc", expected: "1g45uqqks6lu"},
		{name: "unicode", input: "/Users/foo/my_project-测试", expected: "2a16ot6asyzsy"},
		{name: "emoji", input: strings.Repeat("😀", 101), expected: "1wlro20j1vo13"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := transcriptProjectHashSuffix(test.input); got != test.expected {
				t.Fatalf("transcriptProjectHashSuffix() = %q, want %q", got, test.expected)
			}
		})
	}
}
