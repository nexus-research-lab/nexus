package conversation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AttachmentPathResolver 把应用层附件解析成当前 runtime 可以读取的真实路径。
type AttachmentPathResolver func(context.Context, protocol.ChatAttachment) (string, error)

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

// PrependText 将系统动态上下文放在用户输入前面。
func (c RuntimeContent) PrependText(prefix string) RuntimeContent {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return c
	}
	text := strings.TrimSpace(c.text)
	if text == "" {
		c.text = prefix
	} else {
		c.text = prefix + "\n\n" + text
	}
	if len(c.blocks) > 0 {
		blocks := make([]map[string]any, 0, len(c.blocks)+1)
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": prefix,
		})
		blocks = append(blocks, cloneRuntimeBlocks(c.blocks)...)
		c.blocks = blocks
	}
	return c
}

// RenderRuntimeContentWithAttachments 将结构化附件渲染成 Claude Code 运行时可消费的输入。
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
		absolutePath, err := resolver(ctx, attachment)
		if err != nil {
			return RuntimeContent{}, err
		}
		ref, err := quoteClaudePathReference(absolutePath)
		if err != nil {
			return RuntimeContent{}, err
		}
		refs = append(refs, ref)
		if attachment.Kind != protocol.ChatAttachmentKindImage {
			textRefs = append(textRefs, ref)
			continue
		}
		if len(imageBlocks) >= runtimeMaxImageBlocksPerSubmit {
			return RuntimeContent{}, fmt.Errorf("image attachment count exceeds runtime limit: %d", runtimeMaxImageBlocksPerSubmit)
		}
		block, err := imageAttachmentBlock(attachment, absolutePath)
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
		plainText = "请查看这些附件： " + refText
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

func imageAttachmentBlock(attachment protocol.ChatAttachment, absolutePath string) (map[string]any, error) {
	// Claude Code 入口使用 Anthropic ContentBlockParam 形状，media_type 必须位于 source 内。
	mimeType, ok := runtimeImageBlockMIMEType(attachment, absolutePath)
	if !ok {
		return nil, fmt.Errorf("unsupported runtime image attachment: %s", filepath.Base(absolutePath))
	}

	data, err := os.ReadFile(absolutePath)
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
		parts = append(parts, "请先阅读这些图片附件，再根据用户问题处理。")
		parts = append(parts, "图片源文件："+strings.Join(imageRefs, " "))
	}
	if len(textRefs) > 0 {
		parts = append(parts, "请同时查看这些文件附件："+strings.Join(textRefs, " "))
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
