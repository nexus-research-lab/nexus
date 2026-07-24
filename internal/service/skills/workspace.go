package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func undeployWorkspaceLocalSkill(workspacePath string, record catalogRecord) error {
	workspaceRoot := filepath.Clean(strings.TrimSpace(workspacePath))
	sourcePath := filepath.Clean(strings.TrimSpace(record.SourcePath))
	if workspaceRoot == "." || sourcePath == "." {
		return errors.New("workspace skill path is empty")
	}
	agentsRoot := filepath.Join(workspaceRoot, ".agents")
	claudeSkillsRoot := filepath.Join(workspaceRoot, ".claude", "skills")
	sourceUnderAgents := pathIsChildOf(sourcePath, agentsRoot)
	sourceUnderClaudeSkills := pathIsChildOf(sourcePath, claudeSkillsRoot)
	if !sourceUnderAgents && !sourceUnderClaudeSkills {
		return errors.New("workspace skill path is outside supported skill directories")
	}
	root, err := confinedfs.Open(workspaceRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	sourceRelative, err := filepath.Rel(workspaceRoot, sourcePath)
	if err != nil {
		return err
	}
	if err = root.RemoveAll(filepath.ToSlash(sourceRelative)); err != nil {
		return err
	}
	skillNames := []string{record.Detail.Name, filepath.Base(sourcePath)}
	seen := map[string]struct{}{}
	for _, skillName := range skillNames {
		trimmedName := strings.TrimSpace(skillName)
		if trimmedName == "" {
			continue
		}
		if _, ok := seen[trimmedName]; ok {
			continue
		}
		seen[trimmedName] = struct{}{}
		if sourceUnderAgents {
			linkPath, relativeErr := filepath.Rel(workspaceRoot, filepath.Join(claudeSkillsRoot, trimmedName))
			if relativeErr != nil {
				return relativeErr
			}
			if err := root.RemoveAll(filepath.ToSlash(linkPath)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func pathIsChildOf(path string, root string) bool {
	relativePath, err := filepath.Rel(root, path)
	return err == nil &&
		relativePath != "." &&
		relativePath != ".." &&
		!strings.HasPrefix(relativePath, ".."+string(os.PathSeparator))
}
