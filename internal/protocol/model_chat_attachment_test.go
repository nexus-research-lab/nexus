package protocol

import "testing"

func TestNormalizeChatAttachmentKeepsRoomConversationScope(t *testing.T) {
	t.Parallel()

	attachment := NormalizeChatAttachment(ChatAttachment{
		WorkspacePath:    "attachments/demo.txt",
		WorkspaceAgentID: "agent-devin",
		ConversationID:   "conversation-1",
		Scope:            ChatAttachmentScopeRoomConversation,
		Kind:             ChatAttachmentKindFile,
	}, "agent-amy")

	if attachment.Scope != ChatAttachmentScopeRoomConversation {
		t.Fatalf("scope = %q, want %q", attachment.Scope, ChatAttachmentScopeRoomConversation)
	}
	if attachment.WorkspaceAgentID != "" {
		t.Fatalf("room attachment should not keep workspace_agent_id: %q", attachment.WorkspaceAgentID)
	}
	if attachment.ConversationID != "conversation-1" {
		t.Fatalf("conversation_id = %q", attachment.ConversationID)
	}
}

func TestNormalizeChatAttachmentDefaultsAgentWorkspaceScope(t *testing.T) {
	t.Parallel()

	attachment := NormalizeChatAttachment(ChatAttachment{
		WorkspacePath: "attachments/demo.txt",
		Kind:          ChatAttachmentKindText,
	}, "agent-amy")

	if attachment.Scope != ChatAttachmentScopeAgentWorkspace {
		t.Fatalf("scope = %q, want %q", attachment.Scope, ChatAttachmentScopeAgentWorkspace)
	}
	if attachment.WorkspaceAgentID != "agent-amy" {
		t.Fatalf("workspace_agent_id = %q", attachment.WorkspaceAgentID)
	}
}
