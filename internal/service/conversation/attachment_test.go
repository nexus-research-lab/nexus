package conversation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRenderRuntimeContentWithAttachments(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, "tmp", "attachments", "demo.txt")
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
			WorkspacePath: "tmp/attachments/demo.txt",
			Kind:          protocol.ChatAttachmentKindText,
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (ResolvedAttachment, error) {
			return openWorkspaceAttachment(workspacePath, attachment.WorkspacePath)
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
	attachmentPath := filepath.Join(workspacePath, "tmp", "attachments", "demo.png")
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
			WorkspacePath: "tmp/attachments/demo.png",
			Kind:          protocol.ChatAttachmentKindImage,
			MIMEType:      "image/png",
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (ResolvedAttachment, error) {
			return openWorkspaceAttachment(workspacePath, attachment.WorkspacePath)
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
		t.Fatalf("second block should be runtime image data, got %#v", blocks[1])
	}
	if !strings.Contains(content.PlainText(), "@\"") {
		t.Fatalf("plain text should keep path reference for history, got %q", content.PlainText())
	}
}

func TestRenderRuntimeContentWithImageOnlyCanAppendContext(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, "tmp", "attachments", "demo.png")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("mkdir attachment dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("fake-image"), 0o644); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	content, err := RenderRuntimeContentWithAttachments(
		context.Background(),
		"",
		[]protocol.ChatAttachment{{
			FileName:      "demo.png",
			WorkspacePath: "tmp/attachments/demo.png",
			Kind:          protocol.ChatAttachmentKindImage,
			MIMEType:      "image/png",
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (ResolvedAttachment, error) {
			return openWorkspaceAttachment(workspacePath, attachment.WorkspacePath)
		},
	)
	if err != nil {
		t.Fatalf("render runtime content: %v", err)
	}
	if content.IsEmpty() {
		t.Fatal("纯图片 runtime content 不应被判定为空")
	}
	appended := content.AppendText("动态上下文")
	if !strings.Contains(appended.PlainText(), "动态上下文") {
		t.Fatalf("纯图片输入应能追加动态上下文: %q", appended.PlainText())
	}
	blocks, ok := appended.Payload().([]map[string]any)
	if !ok {
		t.Fatalf("纯图片输入应保持结构化 payload: %#v", appended.Payload())
	}
	lastBlock := blocks[len(blocks)-1]
	if lastBlock["type"] != "text" || lastBlock["text"] != "动态上下文" {
		t.Fatalf("动态上下文应追加到图片 payload 尾部: %#v", blocks)
	}
}

func TestRenderRuntimeContentWithUnsupportedImageReturnsError(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	attachmentPath := filepath.Join(workspacePath, "tmp", "attachments", "diagram.svg")
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
			WorkspacePath: "tmp/attachments/diagram.svg",
			Kind:          protocol.ChatAttachmentKindImage,
			MIMEType:      "image/svg+xml",
		}},
		func(_ context.Context, attachment protocol.ChatAttachment) (ResolvedAttachment, error) {
			return openWorkspaceAttachment(workspacePath, attachment.WorkspacePath)
		},
	)
	if err == nil {
		t.Fatal("unsupported runtime image should return an error")
	}
}

// ResolveWorkspaceAttachmentPath 将 workspace 相对路径约束到指定 workspace 内并返回绝对路径。
func ResolveWorkspaceAttachmentPath(workspacePath string, relativePath string) (string, error) {
	resolved, err := openWorkspaceAttachment(workspacePath, relativePath)
	if err != nil {
		return "", err
	}
	_ = resolved.File.Close()
	return resolved.AbsolutePath, nil
}

func openWorkspaceAttachment(workspacePath string, relativePath string) (ResolvedAttachment, error) {
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	if root == "" {
		return ResolvedAttachment{}, errors.New("workspace_path is required")
	}
	normalizedPath := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	if normalizedPath == "" {
		return ResolvedAttachment{}, errors.New("attachment workspace_path is required")
	}
	targetPath := filepath.Clean(filepath.Join(root, normalizedPath))
	rootWithSeparator := root + string(os.PathSeparator)
	if targetPath != root && !strings.HasPrefix(targetPath, rootWithSeparator) {
		return ResolvedAttachment{}, errors.New("attachment path escapes workspace")
	}
	rootFS, err := confinedfs.Open(root)
	if err != nil {
		return ResolvedAttachment{}, err
	}
	relative := filepath.ToSlash(normalizedPath)
	parent, err := rootFS.OpenRootNoSymlink(path.Dir(relative))
	rootFS.Close()
	if err != nil {
		return ResolvedAttachment{}, err
	}
	defer parent.Close()
	name := path.Base(relative)
	file, err := parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return ResolvedAttachment{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return ResolvedAttachment{}, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		if info.IsDir() {
			return ResolvedAttachment{}, fmt.Errorf("attachment path is a directory: %s", normalizedPath)
		}
		return ResolvedAttachment{}, fmt.Errorf("attachment path is not a regular file: %s", normalizedPath)
	}
	return ResolvedAttachment{
		AbsolutePath: targetPath,
		File:         file,
	}, nil
}
