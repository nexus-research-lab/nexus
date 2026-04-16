// =====================================================
// @File   ：service_skills.go
// @Date   ：2026/04/16 13:44:49
// @Author ：leemysw
// 2026/04/16 13:44:49   Create
// =====================================================

package agent

import (
	"os"
	"path/filepath"
	"strings"
)

func enrichAgentsWithSkillsCount(agents []Agent) error {
	for index := range agents {
		if err := enrichAgentWithSkillsCount(&agents[index]); err != nil {
			return err
		}
	}
	return nil
}

func enrichAgentWithSkillsCount(agent *Agent) error {
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
