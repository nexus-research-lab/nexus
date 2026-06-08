package skills

import (
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readSkillSource(sourceDir string) (string, string, string, error) {
	skillMDPath := filepath.Join(sourceDir, "SKILL.md")
	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		return "", "", "", err
	}
	return string(content), skillMDPath, filepath.Base(sourceDir), nil
}

func copyDirectory(sourceDir string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		sourceFile, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		targetFile, createErr := os.Create(targetPath)
		if createErr != nil {
			_ = sourceFile.Close()
			return createErr
		}
		if _, err = io.Copy(targetFile, sourceFile); err != nil {
			_ = sourceFile.Close()
			_ = targetFile.Close()
			return err
		}
		if err = sourceFile.Close(); err != nil {
			_ = targetFile.Close()
			return err
		}
		if err = targetFile.Close(); err != nil {
			return err
		}
		return os.Chmod(targetPath, info.Mode())
	})
}

func matchSkillQuery(detail Detail, query string) bool {
	fields := []string{
		strings.ToLower(detail.Name),
		strings.ToLower(detail.Title),
		strings.ToLower(detail.Description),
		strings.ToLower(strings.Join(detail.Tags, " ")),
	}
	for _, field := range fields {
		if strings.Contains(field, query) {
			return true
		}
	}
	return false
}

func defaultSkillScope(scope string) string {
	normalized := strings.TrimSpace(scope)
	if normalized == scopeMain {
		return scopeMain
	}
	if normalized == scopeRoom {
		return scopeRoom
	}
	return scopeAny
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptySlice(candidates ...[]string) []string {
	for _, item := range candidates {
		if len(item) > 0 {
			return append(make([]string, 0, len(item)), item...)
		}
	}
	return []string{}
}

func projectRoot() string {
	return appfs.Root()
}
