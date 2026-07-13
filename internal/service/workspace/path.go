package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var textExtensions = map[string]struct{}{
	"txt": {}, "md": {}, "markdown": {}, "json": {}, "jsonl": {}, "yaml": {}, "yml": {}, "toml": {}, "xml": {},
	"csv": {}, "ts": {}, "tsx": {}, "js": {}, "jsx": {}, "mjs": {}, "cjs": {}, "py": {}, "java": {}, "go": {},
	"rs": {}, "rb": {}, "php": {}, "sh": {}, "bash": {}, "zsh": {}, "sql": {}, "html": {}, "css": {}, "scss": {},
	"less": {}, "log": {}, "ini": {}, "conf": {}, "env": {}, "dockerfile": {}, "makefile": {}, "cmake": {},
	"gradle": {}, "proto": {}, "graphql": {}, "svg": {}, "rst": {}, "adoc": {},
}

func resolveWorkspacePath(workspacePath string, relativePath string) (string, string, error) {
	root := filepath.Clean(workspacePath)
	normalizedPath := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	if normalizedPath == "" {
		return "", "", errors.New("文件路径不能为空")
	}
	if isProtectedWorkspacePath(normalizedPath) {
		return "", "", errors.New("不能直接操作内部运行时目录")
	}
	targetPath := filepath.Clean(filepath.Join(root, normalizedPath))
	rootWithSeparator := root + string(os.PathSeparator)
	if targetPath != root && !strings.HasPrefix(targetPath, rootWithSeparator) {
		return "", "", errors.New("文件路径超出 workspace 范围")
	}
	return targetPath, filepath.ToSlash(normalizedPath), nil
}

func shouldHideWorkspaceEntry(relativePath string) bool {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	baseName := filepath.Base(normalizedPath)
	return hasWorkspacePathSegment(
		normalizedPath,
		".agents",
		".nexus",
		".git",
		".claude",
		"__pycache__",
		"node_modules",
		".pnpm-store",
		".next",
		".turbo",
		".cache",
		"dist",
		"build",
		"coverage",
	) || strings.HasPrefix(baseName, ".DS_")
}

func isProtectedWorkspacePath(relativePath string) bool {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	return hasWorkspacePathSegment(normalizedPath, ".agents", ".claude", ".git", "__pycache__")
}

func hasWorkspacePathSegment(relativePath string, targets ...string) bool {
	if strings.TrimSpace(relativePath) == "" {
		return false
	}
	for _, segment := range strings.Split(filepath.ToSlash(relativePath), "/") {
		if slices.Contains(targets, segment) {
			return true
		}
	}
	return false
}

func normalizeUploadName(filename string) string {
	raw := strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func buildUploadTargetPath(destination string, filename string) string {
	target := strings.TrimSpace(strings.ReplaceAll(destination, "\\", "/"))
	target = strings.TrimPrefix(target, "/")
	if target == "" {
		return filename
	}
	if strings.HasSuffix(target, "/") {
		return target + filename
	}
	lowerBase := strings.ToLower(filepath.Base(target))
	if strings.Contains(lowerBase, ".") {
		return target
	}
	return target + "/" + filename
}

func ensureUniqueWorkspaceFile(targetPath string, normalizedPath string) (string, string, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return normalizedPath, targetPath, nil
	} else if err != nil {
		return "", "", err
	}
	extension := filepath.Ext(normalizedPath)
	base := strings.TrimSuffix(filepath.Base(normalizedPath), extension)
	parent := filepath.ToSlash(filepath.Dir(normalizedPath))
	timestamp := time.Now().Format("20060102-150405")
	for index := 1; index <= 100; index++ {
		suffix := timestamp
		if index > 1 {
			suffix = timestamp + "-" + strconv.Itoa(index)
		}
		nextName := base + "-" + suffix + extension
		nextPath := nextName
		if parent != "." && parent != "" {
			nextPath = parent + "/" + nextName
		}
		nextTargetPath := filepath.Join(filepath.Dir(targetPath), nextName)
		if _, err := os.Stat(nextTargetPath); os.IsNotExist(err) {
			return nextPath, nextTargetPath, nil
		} else if err != nil {
			return "", "", err
		}
	}
	return "", "", errors.New("无法生成唯一文件名")
}

func tryDecodeTextSnapshot(path string, content []byte) (string, bool) {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if _, ok := textExtensions[extension]; ok {
		return string(content), true
	}
	if utf8Text(content) {
		return string(content), true
	}
	return "", false
}

func utf8Text(content []byte) bool {
	for len(content) > 0 {
		if content[0] == 0 {
			return false
		}
		if content[0] < 0x80 {
			content = content[1:]
			continue
		}
		_, size := utf8.DecodeRune(content)
		if size == 1 {
			return false
		}
		content = content[size:]
	}
	return true
}
