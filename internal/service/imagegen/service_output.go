package imagegen

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeFileNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (s *Service) writeImage(input GenerateInput, payload []byte, mimeType string) (string, error) {
	ext := extensionFor(mimeType, input.OutputFormat)
	name := strings.TrimSpace(input.FileName)
	if name == "" {
		name = fmt.Sprintf("%s-%s", s.now().Format("20060102-150405"), promptSlug(input.Prompt))
	}
	name = strings.TrimSuffix(sanitizeFileName(name), filepath.Ext(name))
	if name == "" {
		name = "generated-image"
	}
	relativePath := filepath.ToSlash(filepath.Join("output", "imagegen", name+ext))
	fullPath := filepath.Join(input.WorkspacePath, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
		return "", err
	}
	return relativePath, nil
}

func detectMIMEType(payload []byte, outputFormat string) string {
	if len(payload) > 0 {
		detected := http.DetectContentType(payload)
		if strings.HasPrefix(detected, "image/") {
			return detected
		}
	}
	switch outputFormat {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func extensionFor(mimeType string, outputFormat string) string {
	if exts, err := mime.ExtensionsByType(strings.TrimSpace(mimeType)); err == nil && len(exts) > 0 {
		return exts[0]
	}
	switch outputFormat {
	case "jpeg", "jpg":
		return ".jpg"
	case "webp":
		return ".webp"
	default:
		return ".png"
	}
}

func promptSlug(prompt string) string {
	words := strings.Fields(strings.ToLower(prompt))
	if len(words) == 0 {
		return "image"
	}
	joined := strings.Join(words, "-")
	if len(joined) > 40 {
		joined = joined[:40]
	}
	return sanitizeFileName(joined)
}

func sanitizeFileName(name string) string {
	cleaned := safeFileNamePattern.ReplaceAllString(strings.TrimSpace(name), "-")
	cleaned = strings.Trim(cleaned, ".-_")
	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}
	return cleaned
}
