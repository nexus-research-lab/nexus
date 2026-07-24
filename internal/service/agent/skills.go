package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
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
	count, err := countDeployedSkills(agent.WorkspacePath, agent.Options.SkillIDs...)
	if err != nil {
		return err
	}
	agent.SkillsCount = count
	return nil
}

func countDeployedSkills(workspacePath string, selectedNames ...string) (int, error) {
	root := strings.TrimSpace(workspacePath)
	skillNames := map[string]struct{}{}
	for _, name := range selectedNames {
		normalized := strings.TrimSpace(name)
		if externalName, ok := protocol.ParseExternalSkillReference(normalized); ok {
			normalized = externalName
		}
		if normalized != "" {
			skillNames[normalized] = struct{}{}
		}
	}
	confinedRoot, err := confinedfs.Open(root)
	if os.IsNotExist(err) {
		return len(skillNames), nil
	}
	if err != nil {
		return 0, err
	}
	defer confinedRoot.Close()
	for _, parent := range []string{
		".agents/skills",
		".agents",
		".claude/skills",
	} {
		entries, err := fs.ReadDir(confinedRoot.FS(), parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			skillFile := filepath.ToSlash(filepath.Join(parent, entry.Name(), "SKILL.md"))
			if _, err := confinedRoot.Stat(skillFile); err != nil {
				continue
			}
			skillNames[entry.Name()] = struct{}{}
		}
	}
	return len(skillNames), nil
}
