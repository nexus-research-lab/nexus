package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func enrichAgentsWithSkillsCount(agents []protocol.Agent) error {
	for index := range agents {
		if err := enrichAgentWithSkillsCount(&agents[index]); err != nil {
			return err
		}
	}
	return nil
}

func enrichAgentWithSkillsCount(agent *protocol.Agent) error {
	if agent == nil {
		return nil
	}
	count, err := countDeployedSkills(agent.WorkspacePath)
	if err != nil {
		return err
	}
	agent.SkillsCount = count
	return nil
}

func countDeployedSkills(workspacePath string) (int, error) {
	root := strings.TrimSpace(workspacePath)
	skillNames := map[string]struct{}{}
	for _, parent := range []string{
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, ".agents"),
		filepath.Join(root, ".claude", "skills"),
	} {
		entries, err := os.ReadDir(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			skillDir := filepath.Join(parent, entry.Name())
			if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
				continue
			}
			skillNames[entry.Name()] = struct{}{}
		}
	}
	return len(skillNames), nil
}
