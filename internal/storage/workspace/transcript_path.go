package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"golang.org/x/text/unicode/norm"
)

var transcriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func (s *AgentHistoryStore) resolveTranscriptPath(workspacePath string, sessionID string) (string, error) {
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectsRoot := transcriptProjectsDirForWorkspace(canonicalPath)
	projectDir := findTranscriptProjectDirAt(projectsRoot, canonicalPath)
	if projectDir != "" {
		path := filepath.Join(projectDir, sessionID+".jsonl")
		if transcriptFileIsNonEmpty(canonicalPath, path) {
			return path, nil
		}
	}

	for _, worktreePath := range listTranscriptWorktreePaths(canonicalPath) {
		if worktreePath == canonicalPath {
			continue
		}
		worktreeDir := findTranscriptProjectDirAt(projectsRoot, worktreePath)
		if worktreeDir == "" {
			continue
		}
		path := filepath.Join(worktreeDir, sessionID+".jsonl")
		if transcriptFileIsNonEmpty(worktreePath, path) {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func transcriptConfigHomeDir() string {
	if value := strings.TrimSpace(os.Getenv("NEXUS_CONFIG_DIR")); value != "" {
		return norm.NFC.String(value)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return norm.NFC.String(filepath.Join(".", ".nexus"))
	}
	return norm.NFC.String(filepath.Join(homeDir, ".nexus"))
}

func transcriptProjectsDir() string {
	legacyRoot := filepath.Join(transcriptConfigHomeDir(), "projects")
	managedRoot := filepath.Join(
		appfs.UserRuntimeRoot(authctx.SystemUserID),
		"projects",
	)
	if sameTranscriptPath(legacyRoot, managedRoot) {
		return legacyRoot
	}
	// 迁移后旧的全局 projects 会落在 system runtime。只有在宿主明确
	// 使用 managed state root 且旧目录已经不存在时才切换，保留自定义
	// NEXUS_CONFIG_DIR 测试/部署的原有语义。
	if managedStateRootConfigured() {
		if _, err := os.Stat(legacyRoot); errors.Is(err, os.ErrNotExist) {
			return managedRoot
		}
	}
	return legacyRoot
}

func managedStateRootConfigured() bool {
	if strings.TrimSpace(os.Getenv(appfs.NexusStateRootEnvName)) != "" {
		return true
	}
	configRoot := filepath.Clean(transcriptConfigHomeDir())
	stateRoot := filepath.Clean(appfs.StateRoot())
	return sameTranscriptPath(configRoot, stateRoot) ||
		sameTranscriptPath(configRoot, filepath.Join(stateRoot, "app"))
}

func sameTranscriptPath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func canonicalizeTranscriptPath(path string) string {
	if path == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = path
	}
	resolved, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		resolved = absolutePath
	}
	return norm.NFC.String(resolved)
}

func findTranscriptProjectDir(projectPath string) string {
	return findTranscriptProjectDirAt(transcriptProjectsDirForWorkspace(projectPath), projectPath)
}

// TranscriptProjectDirectoryName 返回 Claude/nxs transcript 使用的项目目录名。
//
// 迁移层需要用同一套规范计算旧 workspace 对应的项目目录，避免迁移后
// 历史记录变成“目录还在但宿主找不到”的孤儿数据。
func TranscriptProjectDirectoryName(workspacePath string) string {
	return sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath))
}

// TranscriptProjectDirectoryNames 返回迁移场景下可能出现的项目目录名。
//
// macOS 等平台可能在 workspace 搬迁前后才解析 `/var` 这类符号链接；
// 同时保留规范化路径和未解析绝对路径，才能无损接住历史目录。
func TranscriptProjectDirectoryNames(workspacePath string) []string {
	absolutePath, err := filepath.Abs(workspacePath)
	if err != nil {
		absolutePath = filepath.Clean(workspacePath)
	}
	candidates := []string{
		TranscriptProjectDirectoryName(workspacePath),
		sanitizeTranscriptPath(norm.NFC.String(filepath.Clean(absolutePath))),
	}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		seen := false
		for _, existing := range result {
			if existing == candidate {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, candidate)
		}
	}
	return result
}

func findTranscriptProjectDirAt(projectsRoot string, projectPath string) string {
	root, err := confinedfs.Open(projectsRoot)
	if err != nil {
		return ""
	}
	defer root.Close()

	sanitized := TranscriptProjectDirectoryName(projectPath)
	exactName := sanitized
	if info, statErr := root.Lstat(exactName); statErr == nil &&
		info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return filepath.Join(projectsRoot, exactName)
	}
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return ""
	}
	prefix := sanitized[:maxTranscriptSanitizedLength]
	entries, readErr := fs.ReadDir(root.FS(), ".")
	if readErr != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix+"-") {
			return filepath.Join(projectsRoot, entry.Name())
		}
	}
	return ""
}

func transcriptProjectsDirForWorkspace(workspacePath string) string {
	canonicalWorkspace := canonicalizeTranscriptPath(workspacePath)
	canonicalUsersRoot := canonicalizeTranscriptPath(appfs.UsersRoot())
	relative, err := filepath.Rel(canonicalUsersRoot, canonicalWorkspace)
	if err != nil || relative == "." || relative == "" {
		return transcriptProjectsDir()
	}
	parts := strings.Split(filepath.Clean(relative), string(os.PathSeparator))
	if len(parts) < 3 || parts[0] == ".." || parts[1] != "workspace" {
		return transcriptProjectsDir()
	}
	return filepath.Join(canonicalUsersRoot, parts[0], "runtime", "projects")
}

func listTranscriptWorktreePaths(cwd string) []string {
	if strings.TrimSpace(cwd) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), transcriptSessionSearchTimout)
	defer cancel()

	command := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	command.Dir = cwd
	output, err := command.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		results = append(results, norm.NFC.String(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))))
	}
	return results
}

func sanitizeTranscriptPath(path string) string {
	sanitized := transcriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return sanitized
	}
	return sanitized[:maxTranscriptSanitizedLength] + "-" + transcriptProjectHashSuffix(path)
}

func transcriptFileIsNonEmpty(workspacePath string, path string) bool {
	root, relative, info, err := openTranscriptPath(workspacePath, path)
	if err != nil {
		return false
	}
	defer root.Close()
	return info.Mode().IsRegular() && info.Size() > 0 && relative != ""
}
