package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var transcriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func (s *AgentHistoryStore) resolveTranscriptPath(workspacePath string, sessionID string) (string, error) {
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectDir := findTranscriptProjectDir(canonicalPath)
	if projectDir != "" {
		path := filepath.Join(projectDir, sessionID+".jsonl")
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}

	for _, worktreePath := range listTranscriptWorktreePaths(canonicalPath) {
		if worktreePath == canonicalPath {
			continue
		}
		worktreeDir := findTranscriptProjectDir(worktreePath)
		if worktreeDir == "" {
			continue
		}
		path := filepath.Join(worktreeDir, sessionID+".jsonl")
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func removeDirectoryIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(path)
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
	return filepath.Join(transcriptConfigHomeDir(), "projects")
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
	exact := filepath.Join(transcriptProjectsDir(), sanitizeTranscriptPath(projectPath))
	if isDirectory(exact) {
		return exact
	}
	sanitized := sanitizeTranscriptPath(projectPath)
	if len(sanitized) <= maxTranscriptSanitizedLength {
		return ""
	}
	prefix := sanitized[:maxTranscriptSanitizedLength]
	for _, entry := range readDirectories(transcriptProjectsDir()) {
		if strings.HasPrefix(filepath.Base(entry), prefix+"-") {
			return entry
		}
	}
	return ""
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

func readDirectories(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	results := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, filepath.Join(root, entry.Name()))
		}
	}
	return results
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
