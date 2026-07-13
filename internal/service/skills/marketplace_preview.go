package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// GetExternalSkillPreview 获取外部技能详情页或 README 的预览。
func (s *Service) GetExternalSkillPreview(ctx context.Context, detailURL string) (*ExternalSkillPreviewResponse, error) {
	targetURL, err := s.validateExternalURL(ctx, detailURL)
	if err != nil {
		return nil, err
	}
	if isSkillsShPreviewURL(targetURL) {
		return &ExternalSkillPreviewResponse{
			DetailURL:      targetURL,
			ReadmeMarkdown: "",
		}, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills 预览加载失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalPreviewBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxExternalPreviewBytes {
		body = body[:maxExternalPreviewBytes]
	}
	if isZipPayload(targetURL, response.Header.Get("Content-Type"), body) {
		markdown, extractErr := extractSkillMarkdownFromZip(body)
		if extractErr != nil {
			return nil, extractErr
		}
		return &ExternalSkillPreviewResponse{
			DetailURL:      targetURL,
			ReadmeMarkdown: markdown,
		}, nil
	}
	return &ExternalSkillPreviewResponse{
		DetailURL:      targetURL,
		ReadmeMarkdown: extractPreviewMarkdown(string(body)),
	}, nil
}

func (s *Service) attachExternalReadmes(ctx context.Context, items []ExternalSkillSearchItem) {
	if len(items) == 0 {
		return
	}
	semaphore := make(chan struct{}, maxExternalPreviewConcurrency)
	var wg sync.WaitGroup
	for index := range items {
		index := index
		if items[index].SourceKind == externalSourceKindSkillsSh || items[index].ImportMode == externalSourceKindSkillsSh {
			continue
		}
		if strings.TrimSpace(items[index].DetailURL) == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			preview, err := s.GetExternalSkillPreview(ctx, items[index].DetailURL)
			if err == nil && preview != nil {
				items[index].ReadmeMarkdown = preview.ReadmeMarkdown
			}
		}()
	}
	wg.Wait()
}

func extractSkillMarkdownFromZip(payload []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return "", errors.New("skill 预览内容不是合法 zip 包")
	}
	bestPath := ""
	var bestFile *zip.File
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || zipEntryIsDir(file.Name) || zipEntryBase(file.Name) != "SKILL.md" {
			continue
		}
		if bestFile == nil || len(file.Name) < len(bestPath) {
			bestPath = file.Name
			bestFile = file
		}
	}
	if bestFile == nil {
		return "", errors.New("skill 预览 zip 中未找到 SKILL.md")
	}
	handle, err := bestFile.Open()
	if err != nil {
		return "", err
	}
	defer handle.Close()
	body, err := io.ReadAll(io.LimitReader(handle, maxExternalPreviewBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxExternalPreviewBytes {
		body = body[:maxExternalPreviewBytes]
	}
	return string(body), nil
}

func extractPreviewMarkdown(html string) string {
	trimmed := strings.TrimSpace(html)
	if isPlainMarkdownPreview(trimmed) {
		return trimmed
	}
	marker := `"dangerouslySetInnerHTML":{"__html":"`
	if !strings.Contains(html, marker) {
		return ""
	}
	best := ""
	bestScore := 0
	remaining := html
	for {
		_, fragment, ok := strings.Cut(remaining, marker)
		if !ok {
			break
		}
		fragment, rest, _ := strings.Cut(fragment, `"}}`)
		candidate := normalizePreviewHTMLFragment(fragment)
		if score := previewMarkdownScore(candidate); score > bestScore {
			best = candidate
			bestScore = score
		}
		remaining = rest
	}
	return strings.TrimSpace(best)
}

func normalizePreviewHTMLFragment(fragment string) string {
	result := strings.ReplaceAll(fragment, `\n`, "\n")
	result = strings.ReplaceAll(result, `\"`, `"`)
	result = strings.ReplaceAll(result, `\u003c`, "<")
	result = strings.ReplaceAll(result, `\u003C`, "<")
	result = strings.ReplaceAll(result, `\u003e`, ">")
	result = strings.ReplaceAll(result, `\u003E`, ">")
	result = strings.ReplaceAll(result, `\u0026`, "&")
	for _, item := range previewMarkdownRules {
		result = item.pattern.ReplaceAllString(result, item.replace)
	}
	result = strings.ReplaceAll(result, "&#x3C;", "<")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	return strings.TrimSpace(result)
}

func previewMarkdownScore(markdown string) int {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" || strings.HasPrefix(trimmed, "{") || strings.Contains(trimmed, `"@context"`) {
		return 0
	}
	score := len(trimmed)
	if strings.Contains(trimmed, "# ") || strings.Contains(trimmed, "## ") {
		score += 200
	}
	return score
}

func isPlainMarkdownPreview(trimmed string) bool {
	if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "# ") || strings.Contains(trimmed, "\n# ") {
		return true
	}
	return !strings.Contains(strings.ToLower(trimmed), "<html") && strings.Contains(trimmed, "\n")
}
