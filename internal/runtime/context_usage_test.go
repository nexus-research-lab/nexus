package runtime

import (
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeContextUsagePrefersRawWindow(t *testing.T) {
	usage, valid := normalizeContextUsage(agentclient.ContextUsageResponse{
		TotalTokens:  196_000,
		MaxTokens:    245_000,
		RawMaxTokens: 258_000,
		Percentage:   99,
		Model:        " glm-5.2 ",
	})

	if !valid {
		t.Fatal("normalizeContextUsage() valid = false, want true")
	}
	if usage.TotalTokens != 196_000 ||
		usage.MaxTokens != 258_000 ||
		usage.Percentage != 76 ||
		usage.Model != "glm-5.2" {
		t.Fatalf("usage = %#v, want normalized raw window snapshot", usage)
	}
}

func TestNormalizeContextUsageRejectsMissingWindow(t *testing.T) {
	if _, valid := normalizeContextUsage(agentclient.ContextUsageResponse{
		TotalTokens: 100,
	}); valid {
		t.Fatal("normalizeContextUsage() valid = true, want false")
	}
}

func TestManagerContextUsageSnapshotsKeepPerAgentLatestValue(t *testing.T) {
	manager := NewManager()
	sessionKey := "room:group:conversation-a"
	manager.RecordContextUsage(sessionKey, "agent-b", protocol.ContextUsageData{
		TotalTokens: 20,
		MaxTokens:   100,
		Percentage:  20,
		Model:       "model-b-old",
	})
	manager.RecordContextUsage(sessionKey, "agent-a", protocol.ContextUsageData{
		TotalTokens: 10,
		MaxTokens:   100,
		Percentage:  10,
		Model:       "model-a",
	})
	manager.RecordContextUsage(sessionKey, "agent-b", protocol.ContextUsageData{
		TotalTokens: 30,
		MaxTokens:   100,
		Percentage:  30,
		Model:       "model-b",
	})

	snapshots := manager.ContextUsageSnapshots(sessionKey)
	if len(snapshots) != 2 {
		t.Fatalf("ContextUsageSnapshots() len = %d, want 2", len(snapshots))
	}
	if snapshots[0].AgentID != "agent-a" ||
		snapshots[0].Usage.Model != "model-a" ||
		snapshots[1].AgentID != "agent-b" ||
		snapshots[1].Usage.TotalTokens != 30 ||
		snapshots[1].Usage.Model != "model-b" {
		t.Fatalf("ContextUsageSnapshots() = %#v, want sorted latest values", snapshots)
	}
}

func TestManagerRecordContextUsageAfterRuntimeReplacement(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:agent-a:ws:dm:session-a"
	manager.mu.Lock()
	manager.ensureStateLocked(sessionKey).ContextUsageByAgent = nil
	manager.mu.Unlock()

	manager.RecordContextUsage(sessionKey, "agent-a", protocol.ContextUsageData{
		TotalTokens: 10,
		MaxTokens:   100,
		Percentage:  10,
	})

	if snapshots := manager.ContextUsageSnapshots(sessionKey); len(snapshots) != 1 {
		t.Fatalf("ContextUsageSnapshots() = %#v, want replacement runtime snapshot", snapshots)
	}
}

func TestManagerContextUsageSnapshotsIgnoreInvalidIdentity(t *testing.T) {
	manager := NewManager()
	manager.RecordContextUsage("", "agent-a", protocol.ContextUsageData{MaxTokens: 100})
	manager.RecordContextUsage("agent:agent-a:ws:dm:session-a", "", protocol.ContextUsageData{MaxTokens: 100})
	manager.RecordContextUsage(
		"agent:agent-a:ws:dm:session-a",
		"agent-a",
		protocol.ContextUsageData{},
	)

	if snapshots := manager.ContextUsageSnapshots("agent:agent-a:ws:dm:session-a"); len(snapshots) != 0 {
		t.Fatalf("ContextUsageSnapshots() = %#v, want empty", snapshots)
	}
}
