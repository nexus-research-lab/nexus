// INPUT: Owner-scoped Agent identity and an explicit list of completed deliverables.
// OUTPUT: Validated canonical paths in that Agent's workspace; no directory-wide inference.
// POS: File delivery validation shared by runtime tools; the message owns persistence and provenance.
package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// ValidateDeliverables checks the entire batch before returning any file evidence.
// It does not claim that filesystem existence proves authorship: delivery is explicit.
func (s *Service) ValidateDeliverables(ctx context.Context, agentID string, paths []string) ([]string, error) {
	if len(paths) == 0 || len(paths) > 32 {
		return nil, errors.New("deliver between 1 and 32 files")
	}
	agent, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	root, err := s.openAgentWorkspace(agent, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return validateDeliverablePaths(ctx, root, agent.WorkspacePath, paths)
}

func validateDeliverablePaths(ctx context.Context, root *confinedfs.Root, workspacePath string, paths []string) ([]string, error) {
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path = strings.TrimSpace(path)
		if filepath.IsAbs(path) {
			var err error
			path, err = filepath.Rel(workspacePath, path)
			if err != nil {
				return nil, errors.New("file is outside the current workspace")
			}
		}
		_, relative, err := resolveWorkspacePath(workspacePath, path)
		if err != nil {
			return nil, err
		}
		relative = filepath.ToSlash(filepath.Clean(relative))
		file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
		if err != nil {
			return nil, errors.New("deliverable must be an existing regular file in the current workspace")
		}
		info, statErr := file.Stat()
		_ = file.Close()
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, errors.New("deliverable must be a regular file")
		}
		if !seen[relative] {
			seen[relative] = true
			result = append(result, relative)
		}
	}
	return result, nil
}
