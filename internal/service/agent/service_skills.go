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
	skillRoot := filepath.Join(strings.TrimSpace(workspacePath), ".agents", "skills")
	entries, err := os.ReadDir(skillRoot)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count, nil
}
