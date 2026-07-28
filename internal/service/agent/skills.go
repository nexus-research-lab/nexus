package agent

import (
	"io/fs"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) enrichAgentsWithSkillsCount(agents []protocol.Agent) error {
	for index := range agents {
		if err := s.enrichAgentWithSkillsCount(&agents[index]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enrichAgentWithSkillsCount(agent *protocol.Agent) error {
	if agent == nil {
		return nil
	}
	root, err := s.openAgentWorkspace(*agent, false)
	if os.IsNotExist(err) {
		agent.SkillsCount = selectedSkillCount(agent.Options.SkillIDs)
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	count, err := countDeployedSkillsAt(root, agent.Options.SkillIDs...)
	if err != nil {
		return err
	}
	agent.SkillsCount = count
	return nil
}

func selectedSkillCount(selectedNames []string) int {
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
	return len(skillNames)
}

func countDeployedSkillsAt(
	root *confinedfs.Root,
	selectedNames ...string,
) (int, error) {
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
	for _, parent := range []string{
		".agents/skills",
		".agents",
		".claude/skills",
	} {
		parentRoot, err := root.OpenRootNoSymlink(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		entries, err := fs.ReadDir(parentRoot.FS(), ".")
		if err != nil {
			parentRoot.Close()
			return 0, err
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			skillRoot, openErr := parentRoot.OpenRootNoSymlink(entry.Name())
			if openErr != nil {
				continue
			}
			skillFile, openErr := skillRoot.OpenFileNoSymlink(
				"SKILL.md",
				os.O_RDONLY,
				0,
			)
			skillRoot.Close()
			if openErr != nil {
				continue
			}
			skillFile.Close()
			skillNames[entry.Name()] = struct{}{}
		}
		parentRoot.Close()
	}
	return len(skillNames), nil
}
