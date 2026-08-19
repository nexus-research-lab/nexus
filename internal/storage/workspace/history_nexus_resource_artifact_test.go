package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCompactMessagesPreservesDistinctNexusResourceCards(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "assistant-resource",
			"role":       "assistant",
			"content": []map[string]any{
				{
					"id":            "nexus_resource:agent:tool-1:agent-1",
					"type":          protocol.ContentBlockTypeNexusResourceArtifact,
					"resource_kind": protocol.NexusResourceKindAgent,
					"resource_id":   "agent-1",
					"name":          "产品搭档",
				},
			},
		},
		{
			"message_id": "assistant-resource",
			"role":       "assistant",
			"content": []map[string]any{
				{
					"id":              "nexus_resource:room:tool-2:room-1",
					"type":            protocol.ContentBlockTypeNexusResourceArtifact,
					"resource_kind":   protocol.NexusResourceKindRoom,
					"resource_id":     "room-1",
					"conversation_id": "conversation-1",
					"name":            "产品评审室",
				},
			},
		},
	}

	compacted := compactMessages(rows)
	if len(compacted) != 1 {
		t.Fatalf("compacted messages = %d, want 1", len(compacted))
	}
	blocks, _ := compacted[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("distinct Agent and Room cards must both be preserved: %+v", blocks)
	}
}
