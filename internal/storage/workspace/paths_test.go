package workspace

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const testRoomOwnerUserID = "user-room-test"

func TestStoreRoomConversationAssetDirUsesOwnerWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	store := New("")
	store.StateRoot = stateRoot
	conversationID := "conversation/assets"
	got := store.RoomConversationAssetDir(testRoomOwnerUserID, conversationID)
	want := filepath.Join(
		appfs.UserRoomAssetsRootAt(stateRoot, testRoomOwnerUserID),
		encodeConversationDirName(conversationID),
	)
	if got != want {
		t.Fatalf("RoomConversationAssetDir() = %q, want %q", got, want)
	}
}

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
