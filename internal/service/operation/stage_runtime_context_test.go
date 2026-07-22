package operation

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestStageRuntimeContextFollowsLiveConversationPresence(t *testing.T) {
	t.Parallel()

	service := NewService(config.Config{CacheFileDir: t.TempDir()})
	runtimeSessionKey := "agent:agent-1:ws:group:conversation-1"
	if blocks := service.StageRuntimeContext(runtimeSessionKey); len(blocks) != 0 {
		t.Fatalf("inactive Stage context = %#v, want none", blocks)
	}

	if _, err := service.TouchStagePresence(
		context.Background(),
		"room:group:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatal(err)
	}
	blocks := service.StageRuntimeContext(runtimeSessionKey)
	if len(blocks) != 1 {
		t.Fatalf("active Stage context = %#v, want one block", blocks)
	}
	block := blocks[0]
	if block.Name != stageRuntimeContextName || block.Metadata["session_key"] != runtimeSessionKey {
		t.Fatalf("Stage context identity = %#v", block)
	}
	for _, required := range []string{
		"Terminal is an observable execution surface",
		"self-contained HTML/CSS/JavaScript artifact",
		"one standalone host-open command",
		"Never claim that an app",
	} {
		if !strings.Contains(block.Content, required) {
			t.Fatalf("Stage context missing %q:\n%s", required, block.Content)
		}
	}

	if _, err := service.CloseStagePresence(
		context.Background(),
		"room:group:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatal(err)
	}
	if blocks = service.StageRuntimeContext(runtimeSessionKey); len(blocks) != 0 {
		t.Fatalf("closed Stage context = %#v, want none", blocks)
	}
}
