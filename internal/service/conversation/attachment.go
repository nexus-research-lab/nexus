package conversation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ResolvedAttachment 表示已经通过目录边界校验并固定 inode 的附件。
//
// File 的所有权交给 RenderRuntimeContentWithAttachments；resolver 返回后不能
// 再关闭或复用，避免路径校验与图片读取之间重新按绝对路径打开文件。
type ResolvedAttachment struct {
	AbsolutePath string
	File         *os.File
}

// AttachmentPathResolver 把应用层附件解析成当前 runtime 可以读取的真实文件。
type AttachmentPathResolver func(context.Context, protocol.ChatAttachment) (ResolvedAttachment, error)

// RuntimeContent 是 Nexus 应用层投递给 SDK runtime 的用户输入。
type RuntimeContent struct {
	text   string
	blocks []map[string]any
}

const (
	runtimeImageMaxBase64Size      = 5 * 1024 * 1024
	runtimeMaxImageBlocksPerSubmit = 100
)

// NewRuntimeTextContent 构造纯文本 runtime 输入。
func NewRuntimeTextContent(text string) RuntimeContent {
	return RuntimeContent{text: strings.TrimSpace(text)}
}

// IsEmpty 判断 runtime 输入是否为空。
func (c RuntimeContent) IsEmpty() bool {
	return strings.TrimSpace(c.text) == "" && len(c.blocks) == 0
}

// PlainText 返回适合持久化、日志和非多模态通道使用的文本表示。
func (c RuntimeContent) PlainText() string {
	if text := strings.TrimSpace(c.text); text != "" {
		return text
	}
	return strings.TrimSpace(runtimeBlocksPlainText(c.blocks))
}

// Payload 返回 SDK runtime 可消费的实际消息 content。
func (c RuntimeContent) Payload() any {
	if len(c.blocks) == 0 {
		return strings.TrimSpace(c.text)
	}
	return cloneRuntimeBlocks(c.blocks)
}

// AppendText 将运行时动态上下文追加到本轮用户输入尾部。
func (c RuntimeContent) AppendText(suffix string) RuntimeContent {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return c
	}
	text := strings.TrimSpace(c.text)
	if text == "" {
		c.text = suffix
	} else {
		c.text = text + "\n\n" + suffix
	}
	if len(c.blocks) > 0 {
		blocks := cloneRuntimeBlocks(c.blocks)
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": suffix,
		})
		c.blocks = blocks
	}
	return c
}

// RenderRuntimeContentWithAttachments 将结构化附件渲染成 SDK runtime 可消费的输入。
func RenderRuntimeContentWithAttachments(
	ctx context.Context,
	content string,
	attachments []protocol.ChatAttachment,
	resolver AttachmentPathResolver,
) (RuntimeContent, error) {
	normalizedAttachments := protocol.NormalizeChatAttachments(attachments, "")
	if len(normalizedAttachments) == 0 {
		return NewRuntimeTextContent(content), nil
	}
	if resolver == nil {
		return RuntimeContent{}, errors.New("attachment path resolver is required")
	}

	refs := make([]string, 0, len(normalizedAttachments))
	textRefs := make([]string, 0, len(normalizedAttachments))
	imageRefs := make([]string, 0, len(normalizedAttachments))
	imageBlocks := make([]map[string]any, 0, len(normalizedAttachments))
	for _, attachment := range normalizedAttachments {
		resolved, err := resolver(ctx, attachment)
		if err != nil {
			return RuntimeContent{}, err
		}
		if resolved.File == nil {
			return RuntimeContent{}, errors.New("attachment resolver returned no file")
		}
		ref, err := quoteRuntimePathReference(resolved.AbsolutePath)
		if err != nil {
			_ = resolved.File.Close()
			return RuntimeContent{}, err
		}
		refs = append(refs, ref)
		if attachment.Kind != protocol.ChatAttachmentKindImage {
			textRefs = append(textRefs, ref)
			_ = resolved.File.Close()
			continue
		}
		if len(imageBlocks) >= runtimeMaxImageBlocksPerSubmit {
			_ = resolved.File.Close()
			return RuntimeContent{}, fmt.Errorf("image attachment count exceeds runtime limit: %d", runtimeMaxImageBlocksPerSubmit)
		}
		block, err := imageAttachmentBlock(attachment, resolved.AbsolutePath, resolved.File)
		_ = resolved.File.Close()
		if err != nil {
			return RuntimeContent{}, err
		}
		imageRefs = append(imageRefs, ref)
		imageBlocks = append(imageBlocks, block)
	}

	refText := strings.Join(refs, " ")
	trimmedContent := strings.TrimSpace(content)
	plainText := refText
	if trimmedContent == "" {
		plainText = "Please review these attachments: " + refText
	} else {
		plainText = refText + " " + trimmedContent
	}
	if len(imageBlocks) == 0 {
		return NewRuntimeTextContent(plainText), nil
	}

	blocks := make([]map[string]any, 0, 1+len(imageBlocks))
	if text := runtimeTextBlockForAttachments(trimmedContent, textRefs, imageRefs); text != "" {
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": text,
		})
	}
	blocks = append(blocks, imageBlocks...)
	return RuntimeContent{
		text:   plainText,
		blocks: blocks,
	}, nil
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

func quoteRuntimePathReference(path string) (string, error) {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(path))
	if normalizedPath == "" {
		return "", errors.New("attachment path is required")
	}
	if strings.Contains(normalizedPath, "\"") {
		return "", fmt.Errorf("attachment path contains unsupported quote: %s", normalizedPath)
	}
	return "@\"" + normalizedPath + "\"", nil
}

func imageAttachmentBlock(
	attachment protocol.ChatAttachment,
	absolutePath string,
	file *os.File,
) (map[string]any, error) {
	// SDK runtime 使用 Anthropic ContentBlockParam 形状，media_type 必须位于 source 内。
	mimeType, ok := runtimeImageBlockMIMEType(attachment, absolutePath)
	if !ok {
		return nil, fmt.Errorf("unsupported runtime image attachment: %s", filepath.Base(absolutePath))
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	if len(encoded) > runtimeImageMaxBase64Size {
		return nil, fmt.Errorf("image attachment base64 size exceeds runtime limit: %d > %d", len(encoded), runtimeImageMaxBase64Size)
	}
	block := map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": mimeType,
			"data":       encoded,
		},
	}
	return block, nil
}

func runtimeImageBlockMIMEType(attachment protocol.ChatAttachment, absolutePath string) (string, bool) {
	mimeType := strings.ToLower(strings.TrimSpace(attachment.MIMEType))
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return mimeType, true
	case "image/jpg":
		return "image/jpeg", true
	default:
		if strings.HasPrefix(mimeType, "image/") {
			return "", false
		}
	}
	switch strings.ToLower(filepath.Ext(absolutePath)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	default:
		return "", false
	}
}

func runtimeTextBlockForAttachments(content string, textRefs []string, imageRefs []string) string {
	parts := make([]string, 0, 4)
	if len(imageRefs) > 0 {
		parts = append(parts, "Review these image attachments first, then respond to the user's request.")
		parts = append(parts, "Image source files: "+strings.Join(imageRefs, " "))
	}
	if len(textRefs) > 0 {
		parts = append(parts, "Also review these file attachments: "+strings.Join(textRefs, " "))
	}
	if strings.TrimSpace(content) != "" {
		parts = append(parts, strings.TrimSpace(content))
	}
	return strings.Join(parts, "\n\n")
}

func runtimeBlocksPlainText(blocks []map[string]any) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(block["type"])) == "text" {
			if text := strings.TrimSpace(fmt.Sprint(block["text"])); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func cloneRuntimeBlocks(blocks []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		result = append(result, cloneRuntimeMap(block))
	}
	return result
}

func cloneRuntimeMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = cloneRuntimeValue(item)
	}
	return result
}

func cloneRuntimeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneRuntimeMap(typed)
	case []map[string]any:
		return cloneRuntimeBlocks(typed)
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, cloneRuntimeValue(item))
		}
		return result
	default:
		return typed
	}
}
