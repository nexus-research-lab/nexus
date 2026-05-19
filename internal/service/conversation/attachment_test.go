package conversation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRenderRuntimeContentWithAttachments(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, ".nexus", "attachments", "demo.txt")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("mkdir attachment dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	content, err := RenderRuntimeContentWithAttachments(
		context.Background(),
		"总结一下",
		[]protocol.ChatAttachment{{
			FileName:      "demo.txt",
			WorkspacePath: ".nexus/attachments/demo.txt",
			Kind:          protocol.ChatAttachmentKindText,
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (string, error) {
			return ResolveWorkspaceAttachmentPath(workspacePath, attachment.WorkspacePath)
		},
	)
	if err != nil {
		t.Fatalf("render runtime content: %v", err)
	}
	if !strings.HasPrefix(content, "@\"") {
		t.Fatalf("content should begin with quoted attachment ref, got %q", content)
	}
	if !strings.HasSuffix(content, " 总结一下") {
		t.Fatalf("content should keep original text, got %q", content)
	}
}

func TestResolveWorkspaceAttachmentPathRejectsEscape(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if _, err := ResolveWorkspaceAttachmentPath(workspacePath, "../outside.txt"); err == nil {
		t.Fatal("expected escaping attachment path to be rejected")
	}
}
