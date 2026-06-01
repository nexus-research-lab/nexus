package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.RemoveAll(sourcePath); err != nil {
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
			linkPath := filepath.Join(claudeSkillsRoot, trimmedName)
			if err := os.RemoveAll(linkPath); err != nil && !os.IsNotExist(err) {
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
