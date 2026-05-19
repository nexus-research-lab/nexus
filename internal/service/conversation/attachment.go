package conversation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AttachmentPathResolver 把应用层附件解析成当前 runtime 可以读取的真实路径。
type AttachmentPathResolver func(context.Context, protocol.ChatAttachment) (string, error)

// RenderRuntimeContentWithAttachments 将结构化附件渲染成当前 Claude Code 兼容运行时可消费的输入。
func RenderRuntimeContentWithAttachments(
	ctx context.Context,
	content string,
	attachments []protocol.ChatAttachment,
	resolver AttachmentPathResolver,
) (string, error) {
	normalizedAttachments := protocol.NormalizeChatAttachments(attachments, "")
	if len(normalizedAttachments) == 0 {
		return strings.TrimSpace(content), nil
	}
	if resolver == nil {
		return "", errors.New("attachment path resolver is required")
	}

	refs := make([]string, 0, len(normalizedAttachments))
	for _, attachment := range normalizedAttachments {
		absolutePath, err := resolver(ctx, attachment)
		if err != nil {
			return "", err
		}
		ref, err := quoteClaudePathReference(absolutePath)
		if err != nil {
			return "", err
		}
		refs = append(refs, ref)
	}

	refText := strings.Join(refs, " ")
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return "请查看这些附件： " + refText, nil
	}
	return refText + " " + trimmedContent, nil
}

// ResolveWorkspaceAttachmentPath 将 workspace 相对路径约束到指定 workspace 内并返回绝对路径。
func ResolveWorkspaceAttachmentPath(workspacePath string, relativePath string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	if root == "" {
		return "", errors.New("workspace_path is required")
	}
	normalizedPath := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	if normalizedPath == "" {
		return "", errors.New("attachment workspace_path is required")
	}
	targetPath := filepath.Clean(filepath.Join(root, normalizedPath))
	rootWithSeparator := root + string(os.PathSeparator)
	if targetPath != root && !strings.HasPrefix(targetPath, rootWithSeparator) {
		return "", errors.New("attachment path escapes workspace")
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("attachment path is a directory: %s", normalizedPath)
	}
	return targetPath, nil
}

func quoteClaudePathReference(path string) (string, error) {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(path))
	if normalizedPath == "" {
		return "", errors.New("attachment path is required")
	}
	if strings.Contains(normalizedPath, "\"") {
		return "", fmt.Errorf("attachment path contains unsupported quote: %s", normalizedPath)
	}
	return "@\"" + normalizedPath + "\"", nil
}
