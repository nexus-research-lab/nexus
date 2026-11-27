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
