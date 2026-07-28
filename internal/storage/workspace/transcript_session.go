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

	root, relative, _, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if err := root.Remove(relative); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	s.invalidateTranscriptCache(transcriptPath)
	// 仅尝试删除空的 project 目录；失败时保留目录，不影响 session 删除。
	_ = root.Remove(filepath.ToSlash(filepath.Dir(relative)))
	return true, nil
}

// DeleteTranscriptProject 删除整个 workspace 对应的 transcript 项目目录。
func (s *AgentHistoryStore) DeleteTranscriptProject(workspacePath string) (bool, error) {
	canonicalPath := canonicalizeTranscriptPath(workspacePath)
	projectsRoot := s.transcriptProjectsRootForWorkspace(canonicalPath)
	projectDir, err := s.findTranscriptProjectDirAt(projectsRoot, canonicalPath)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(projectDir) == "" {
		return false, nil
	}
	if strings.TrimSpace(s.ownerUserID) != "" {
		root, err := s.paths.openOwnerTranscriptProjectsRoot(
			s.ownerUserID,
			false,
		)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		defer root.Close()
		relative, err := filepath.Rel(root.Name(), projectDir)
		if err != nil || relative == "." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false, errors.New("transcript project is outside owner root")
		}
		if err := root.RemoveAll(filepath.ToSlash(relative)); err != nil {
			return false, err
		}
		s.invalidateTranscriptCachePrefix(projectDir)
		return true, nil
	}
	root, relative, err := relativeStorePathWithCreate(projectsRoot, projectDir, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()

	if err := root.RemoveAll(relative); err != nil {
		return false, err
	}
	s.invalidateTranscriptCachePrefix(projectDir)
	return true, nil
}
