// INPUT: workspace、transcript session 标识与可取消的请求上下文。
// OUTPUT: 受 owner 根约束的 transcript 路径，以及必要时查询到的 Git worktree 候选。
// POS: transcript 文件定位与 Git worktree 回退边界。
package workspace

import (
	"context"
	"errors"
	"fmt"
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
	return s.resolveTranscriptPathContext(context.Background(), workspacePath, sessionID)
}

func (s *AgentHistoryStore) resolveTranscriptPathContext(
	ctx context.Context,
	workspacePath string,
	sessionID string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectsRoot := s.transcriptProjectsRootForWorkspace(canonicalPath)
	projectDir, err := s.findTranscriptProjectDirAt(projectsRoot, canonicalPath)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if projectDir != "" {
		path := filepath.Join(projectDir, sessionID+".jsonl")
		exists, err := s.transcriptFileIsNonEmpty(workspacePath, path)
		if err != nil {
			return "", err
		}
		if exists {
			return path, nil
		}
	}

	worktreePaths, err := listTranscriptWorktreePathsContext(ctx, canonicalPath)
	if err != nil {
		return "", err
	}
	for _, worktreePath := range worktreePaths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if worktreePath == canonicalPath {
			continue
		}
		worktreeDir, err := s.findTranscriptProjectDirAt(projectsRoot, worktreePath)
		if err != nil {
			return "", err
		}
		if worktreeDir == "" {
			continue
		}
		path := filepath.Join(worktreeDir, sessionID+".jsonl")
		exists, err := s.transcriptFileIsNonEmpty(workspacePath, path)
		if err != nil {
			return "", err
		}
		if exists {
			return path, nil
		}
	}
	return "", fmt.Errorf(
		"transcript %s 不存在于 projects 根 %s，预期项目目录 %s: %w",
		strings.TrimSpace(sessionID),
		projectsRoot,
		TranscriptProjectDirectoryName(canonicalPath),
		os.ErrNotExist,
	)
}

func (s *AgentHistoryStore) transcriptProjectsRootForWorkspace(
	workspacePath string,
) string {
	if strings.TrimSpace(s.ownerUserID) != "" {
		return filepath.Join(
			appfs.UserRuntimeRootAt(s.paths.StateRoot, s.ownerUserID),
			"projects",
		)
	}
	return transcriptProjectsDirForWorkspace(workspacePath)
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
	// canonical 状态根启用后，旧的全局 projects 即使残留也不能再被回读；
	// 否则一个 owner 的历史目录会重新成为所有用户的共享读取入口。
	if managedStateRootConfigured() {
		return managedRoot
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

// TranscriptProjectsDirForWorkspace 返回 workspace 对应的 transcript projects 根。
//
// canonical owner workspace 使用该 owner 的 runtime/projects；非 canonical
// workspace 使用 system runtime（迁移后的宿主回退根）。
func TranscriptProjectsDirForWorkspace(workspacePath string) string {
	return transcriptProjectsDirForWorkspace(workspacePath)
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
	result, _ := findTranscriptProjectDirInRoot(root, projectsRoot, projectPath)
	return result
}

func findTranscriptProjectDirInRoot(
	root *confinedfs.Root,
	projectsRoot string,
	projectPath string,
) (string, error) {
	sanitized := TranscriptProjectDirectoryName(projectPath)
	exactName := sanitized
	info, statErr := root.Lstat(exactName)
	if statErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return filepath.Join(projectsRoot, exactName), nil
	}
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	if errors.Is(statErr, os.ErrNotExist) {
		if _, absoluteErr := os.Lstat(filepath.Join(projectsRoot, exactName)); absoluteErr == nil {
			return "", errors.New("transcript projects root changed while reading")
		}
	}
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return "", nil
	}
	prefix := sanitized[:maxTranscriptSanitizedLength]
	entries, readErr := fs.ReadDir(root.FS(), ".")
	if readErr != nil {
		return "", readErr
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix+"-") {
			return filepath.Join(projectsRoot, entry.Name()), nil
		}
	}
	return "", nil
}

func (s *AgentHistoryStore) findTranscriptProjectDirAt(
	projectsRoot string,
	projectPath string,
) (string, error) {
	if strings.TrimSpace(s.ownerUserID) == "" {
		return findTranscriptProjectDirAt(projectsRoot, projectPath), nil
	}
	root, err := s.paths.openOwnerTranscriptProjectsRoot(
		s.ownerUserID,
		false,
	)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer root.Close()
	if !sameTranscriptPath(root.Name(), projectsRoot) {
		return "", errors.New("transcript projects root does not match owner root")
	}
	return findTranscriptProjectDirInRoot(root, root.Name(), projectPath)
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
	paths, _ := listTranscriptWorktreePathsContext(context.Background(), cwd)
	return paths
}

type transcriptCommandContext func(context.Context, string, ...string) *exec.Cmd

func listTranscriptWorktreePathsContext(ctx context.Context, cwd string) ([]string, error) {
	return listTranscriptWorktreePathsContextWithCommand(ctx, cwd, exec.CommandContext)
}

func listTranscriptWorktreePathsContextWithCommand(
	ctx context.Context,
	cwd string,
	commandContext transcriptCommandContext,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cwd) == "" {
		return nil, nil
	}
	if !transcriptWorktreeLookupRequired(cwd) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	lookupCtx, cancel := context.WithTimeout(ctx, transcriptSessionSearchTimout)
	defer cancel()

	command := commandContext(lookupCtx, "git", "worktree", "list", "--porcelain")
	command.Dir = cwd
	output, err := command.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// 内部 Git 查询超时和普通 Git 错误沿用旧行为：没有额外候选。
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		results = append(results, norm.NFC.String(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))))
	}
	return results, nil
}

// transcriptWorktreeLookupRequired 先排除最常见的单 worktree 仓库，
// 避免 transcript miss 每次都启动一个 Git 子进程。
func transcriptWorktreeLookupRequired(cwd string) bool {
	if strings.TrimSpace(os.Getenv("GIT_DIR")) != "" ||
		strings.TrimSpace(os.Getenv("GIT_WORK_TREE")) != "" {
		return true
	}
	current := filepath.Clean(cwd)
	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Lstat(gitPath)
		switch {
		case err == nil && !info.IsDir():
			// linked worktree 与 submodule 都使用 .git 文件；保守回退 Git。
			return true
		case err == nil:
			entries, readErr := os.ReadDir(filepath.Join(gitPath, "worktrees"))
			if errors.Is(readErr, os.ErrNotExist) {
				return false
			}
			if readErr != nil {
				return true
			}
			return len(entries) > 0
		case !errors.Is(err, os.ErrNotExist):
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func sanitizeTranscriptPath(path string) string {
	sanitized := transcriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return sanitized
	}
	return sanitized[:maxTranscriptSanitizedLength] + "-" + transcriptProjectHashSuffix(path)
}

func (s *AgentHistoryStore) transcriptFileIsNonEmpty(
	workspacePath string,
	transcriptPath string,
) (bool, error) {
	root, relative, info, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer root.Close()
	return info.Mode().IsRegular() && info.Size() > 0 && relative != "", nil
}
