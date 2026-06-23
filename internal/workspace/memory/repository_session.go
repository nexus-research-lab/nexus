package memory

import (
	"os"
	"path/filepath"
	"strings"
)

func (r *Repository) AppendSessionSummary(sessionKey string, content string) (string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	content = strings.TrimSpace(content)
	if sessionKey == "" || content == "" {
		return "", nil
	}
	relativePath := filepath.ToSlash(filepath.Join("memory", "sessions", safeMemoryFilename(sessionKey)+".md"))
	targetPath := filepath.Join(r.workspacePath, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	existing := ""
	if current, err := os.ReadFile(targetPath); err == nil {
		existing = strings.TrimRight(string(current), "\n")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	next := strings.TrimSpace(content) + "\n"
	if existing != "" {
		next = existing + "\n\n" + next
	}
	return relativePath, os.WriteFile(targetPath, []byte(next), 0o644)
}

func (r *Repository) ReadSessionSummary(sessionKey string) (string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return "", nil
	}
	targetPath := filepath.Join(r.workspacePath, "memory", "sessions", safeMemoryFilename(sessionKey)+".md")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func safeMemoryFilename(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	builder := strings.Builder{}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_' || char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "session"
	}
	if len(result) > 96 {
		result = result[:96]
	}
	return result
}
