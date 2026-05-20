package conversation

import (
	"context"
	"fmt"
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
	if !strings.HasPrefix(content.PlainText(), "@\"") {
		t.Fatalf("content should begin with quoted attachment ref, got %q", content.PlainText())
	}
	if !strings.HasSuffix(content.PlainText(), " 总结一下") {
		t.Fatalf("content should keep original text, got %q", content.PlainText())
	}
	if payload, ok := content.Payload().(string); !ok || payload != content.PlainText() {
		t.Fatalf("text attachment should keep string payload, got %#v", content.Payload())
	}
}

func TestRenderRuntimeContentWithImageAttachmentUsesImageBlock(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, ".nexus", "attachments", "demo.png")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("mkdir attachment dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("fake-image"), 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	content, err := RenderRuntimeContentWithAttachments(
		context.Background(),
		"描述一下图片",
		[]protocol.ChatAttachment{{
			FileName:      "demo.png",
			WorkspacePath: ".nexus/attachments/demo.png",
			Kind:          protocol.ChatAttachmentKindImage,
			MIMEType:      "image/png",
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (string, error) {
			return ResolveWorkspaceAttachmentPath(workspacePath, attachment.WorkspacePath)
		},
	)
	if err != nil {
		t.Fatalf("render runtime content: %v", err)
	}
	blocks, ok := content.Payload().([]map[string]any)
	if !ok {
		t.Fatalf("image attachment should use structured payload, got %#v", content.Payload())
	}
	if len(blocks) != 2 {
		t.Fatalf("image payload block count = %d, want 2", len(blocks))
	}
	if blocks[0]["type"] != "text" || !strings.Contains(fmt.Sprint(blocks[0]["text"]), "描述一下图片") {
		t.Fatalf("first block should keep user text, got %#v", blocks[0])
	}
	source, ok := blocks[1]["source"].(map[string]any)
	if !ok {
		t.Fatalf("second block should include image source, got %#v", blocks[1])
	}
	if blocks[1]["type"] != "image" ||
		source["type"] != "base64" ||
		source["media_type"] != "image/png" ||
		source["data"] == "" {
		t.Fatalf("second block should be Claude Code image data, got %#v", blocks[1])
	}
	if !strings.Contains(content.PlainText(), "@\"") {
		t.Fatalf("plain text should keep path reference for history, got %q", content.PlainText())
	}
}

func TestRenderRuntimeContentWithUnsupportedImageReturnsError(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, ".nexus", "attachments", "diagram.svg")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("mkdir attachment dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("<svg />"), 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	_, err := RenderRuntimeContentWithAttachments(
		context.Background(),
		"看看这个图",
		[]protocol.ChatAttachment{{
			FileName:      "diagram.svg",
			WorkspacePath: ".nexus/attachments/diagram.svg",
			Kind:          protocol.ChatAttachmentKindImage,
			MIMEType:      "image/svg+xml",
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (string, error) {
			return ResolveWorkspaceAttachmentPath(workspacePath, attachment.WorkspacePath)
		},
	)
	if err == nil {
		t.Fatal("unsupported runtime image should return an error")
	}
}

func TestResolveWorkspaceAttachmentPathRejectsEscape(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	if _, err := ResolveWorkspaceAttachmentPath(workspacePath, "../outside.txt"); err == nil {
		t.Fatal("expected escaping attachment path to be rejected")
	}
}
