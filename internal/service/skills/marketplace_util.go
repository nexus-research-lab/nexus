package skills

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func unzipArchive(payload []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return errors.New("上传文件不是合法 zip 包")
	}
	for _, file := range reader.File {
		targetPath := filepath.Join(targetDir, file.Name)
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return errors.New("zip 包含非法路径")
		}
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(cleanTarget, 0o755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
			return err
		}
		readerHandle, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		writer, createErr := os.OpenFile(cleanTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			_ = readerHandle.Close()
			return createErr
		}
		if _, err = io.Copy(writer, readerHandle); err != nil {
			_ = readerHandle.Close()
			_ = writer.Close()
			return err
		}
		if err = readerHandle.Close(); err != nil {
			_ = writer.Close()
			return err
		}
		if err = writer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func isZipPayload(targetURL string, contentType string, payload []byte) bool {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(targetURL)), ".zip") {
		return true
	}
	if strings.Contains(strings.ToLower(contentType), "zip") {
		return true
	}
	return len(payload) >= 4 && bytes.Equal(payload[:4], []byte{'P', 'K', 0x03, 0x04})
}

func findSkillSourceDir(root string) (string, error) {
	bestMatch := ""
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "SKILL.md" {
			return nil
		}
		sourceDir := filepath.Dir(path)
		if bestMatch == "" || len(sourceDir) < len(bestMatch) {
			bestMatch = sourceDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if bestMatch == "" {
		return "", errors.New("未找到 SKILL.md")
	}
	return bestMatch, nil
}

func skillNameFromSourceURL(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "external-skill"
	}
	name := strings.TrimSuffix(filepath.Base(parsed.Path), ".git")
	name = strings.TrimSuffix(name, ".zip")
	name = strings.TrimSuffix(name, ".md")
	if strings.EqualFold(name, "SKILL") || name == "." || name == "/" || name == "" {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i := len(segments) - 1; i >= 0; i-- {
			segment := normalizeSkillNameFallback(segments[i])
			if segment != "" && !strings.EqualFold(segment, "skill") && !strings.EqualFold(segment, "skills") {
				return segment
			}
		}
		return parsed.Host
	}
	return name
}

func repairClaudePluginsText(text string) string {
	if !looksLikeMojibake(text) {
		return text
	}
	bytes := make([]byte, 0, len(text))
	for _, character := range text {
		if character > 255 {
			return text
		}
		bytes = append(bytes, byte(character))
	}
	if !utf8.Valid(bytes) {
		return text
	}
	repaired := string(bytes)
	if strings.ContainsRune(repaired, '\uFFFD') || mojibakeScore(repaired) >= mojibakeScore(text) {
		return text
	}
	return repaired
}

func looksLikeMojibake(text string) bool {
	return mojibakeScore(text) >= 2
}

func mojibakeScore(text string) int {
	score := 0
	for _, character := range text {
		switch {
		case character >= 0x80 && character <= 0x9f:
			score += 2
		case strings.ContainsRune("ÂÃâÄäÅåÆæÇçÉéÊêËëÎîÏïÐðÑñÒòÓóÔôÕõÖöØøÙùÚúÛûÜüÝýÞþ", character):
			score++
		}
	}
	return score
}

func externalSkillSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return limit
}

func anyMap(value any) map[string]any {
	item, _ := value.(map[string]any)
	if item == nil {
		return map[string]any{}
	}
	return item
}

func anyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func anyStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return normalizeStringSlice(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, raw := range typed {
			if text, ok := raw.(string); ok {
				items = append(items, text)
			}
		}
		return normalizeStringSlice(items)
	case string:
		if strings.TrimSpace(typed) == "" {
			return []string{}
		}
		return normalizeStringSlice(strings.Split(typed, ","))
	default:
		return []string{}
	}
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func anyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func cleanSkillSubdirPath(skillPath string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(skillPath), "/")
	if trimmed == "" {
		return "", nil
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == "." {
		return "", nil
	}
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", errors.New("skill 子目录非法")
	}
	return cleaned, nil
}
