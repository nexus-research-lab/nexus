package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var transcriptSessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// IsTranscriptSessionID 判断值是否符合 Claude/nxs transcript session id 形态。
func IsTranscriptSessionID(sessionID string) bool {
	return transcriptSessionIDPattern.MatchString(strings.ToLower(strings.TrimSpace(sessionID)))
}

// TranscriptSessionExists 判断 workspace 下是否存在可恢复的 SDK transcript。
func (s *AgentHistoryStore) TranscriptSessionExists(workspacePath string, sessionID string) (bool, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	normalizedSessionID := strings.ToLower(trimmedSessionID)
	if normalizedSessionID == "" || !IsTranscriptSessionID(normalizedSessionID) {
		return false, nil
	}
	if _, err := s.resolveTranscriptPath(workspacePath, normalizedSessionID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteTranscriptSession 删除单个 SDK transcript 文件。
func (s *AgentHistoryStore) DeleteTranscriptSession(workspacePath string, sessionID string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" {
		return false, nil
	}

	transcriptPath, err := s.resolveTranscriptPath(workspacePath, sessionID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := os.Remove(transcriptPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	s.invalidateTranscriptCache(transcriptPath)
	if err := removeDirectoryIfEmpty(filepath.Dir(transcriptPath)); err != nil {
		return true, err
	}
	return true, nil
}

// DeleteTranscriptProject 删除整个 workspace 对应的 transcript 项目目录。
func (s *AgentHistoryStore) DeleteTranscriptProject(workspacePath string) (bool, error) {
	projectDir := findTranscriptProjectDir(canonicalizeTranscriptPath(workspacePath))
	if strings.TrimSpace(projectDir) == "" {
		return false, nil
	}
	if _, err := os.Stat(projectDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if err := os.RemoveAll(projectDir); err != nil {
		return false, err
	}
	s.invalidateTranscriptCachePrefix(projectDir)
	return true, nil
}
