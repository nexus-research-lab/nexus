package skills

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func validateSkillName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("skill name 不能为空")
	}
	if trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, "/\\\x00") {
		return errors.New("skill name 不能包含路径分隔符")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "external:") {
		return errors.New("skill name 不能使用保留引用前缀")
	}
	return nil
}

func readSkillSource(sourceDir string) (string, string, string, error) {
	skillMDPath := filepath.Join(sourceDir, "SKILL.md")
	info, err := os.Lstat(skillMDPath)
	if err != nil {
		return "", "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", "", errors.New("SKILL.md 必须是普通文件，不能是符号链接")
	}
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
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("skill 源不能包含符号链接")
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("skill 源只能包含普通文件和目录")
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
	return slices.ContainsFunc(fields, func(field string) bool {
		return strings.Contains(field, query)
	})
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptySlice(candidates ...[]string) []string {
	for _, item := range candidates {
		if len(item) > 0 {
			return slices.Clone(item)
		}
	}
	return []string{}
}

func projectRoot() string {
	return appfs.Root()
}
