package protocol

import "testing"

func TestIsTranscriptNativeMessagePersistsNexusResourceArtifactsInOverlay(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    bool
	}{
		{
			name: "plain assistant remains transcript native",
			message: Message{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "done"}},
			},
			want: true,
		},
		{
			name: "typed resource card is persisted by nexus overlay",
			message: Message{
				"role": "assistant",
				"content": []map[string]any{{
					"type":          ContentBlockTypeNexusResourceArtifact,
					"resource_kind": NexusResourceKindRoom,
					"resource_id":   "room-1",
				}},
			},
			want: false,
		},
		{
			name: "decoded resource card is persisted by nexus overlay",
			message: Message{
				"role": "assistant",
				"content": []any{map[string]any{
					"type":          ContentBlockTypeNexusResourceArtifact,
					"resource_kind": NexusResourceKindRoom,
					"resource_id":   "room-1",
				}},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTranscriptNativeMessage(tc.message); got != tc.want {
				t.Fatalf("IsTranscriptNativeMessage() = %v, want %v", got, tc.want)
			}
		})
	}
}
