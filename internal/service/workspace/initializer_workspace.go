package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	// 中文注释：初始化会重建托管 skill 目录，同一 workspace 并发执行会互相删除正在复制的文件。
	workspaceInitializationLocks sync.Map
)

// EnsureInitialized 保证 workspace 模板与系统技能已经落地。
func EnsureInitialized(
	agentID string,
	agentName string,
	workspacePath string,
	isMainAgent bool,
	createdAt time.Time,
) error {
	root := strings.TrimSpace(workspacePath)
	if root == "" {
		return fmt.Errorf("workspace_path 不能为空")
	}
	lock := workspaceInitializationLock(root)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, dir := range defaultDirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	if err := agentsvc.EnsureRuntimeEmotionState(root); err != nil {
		return err
	}

	context := buildTemplateContext(agentID, agentName, root, createdAt)
	if err := ensureNexusctlShim(appfs.AgentRuntimeBinDir(), context); err != nil {
		return err
	}
	if err := removeWorkspaceBinShim(root); err != nil {
		return err
	}
	for key, relativePath := range workspaceFiles {
		targetPath := filepath.Join(root, relativePath)
		if isMainAgent && key == "agents" {
			if err := removeGeneratedMainAgentsPrompt(targetPath); err != nil {
				return err
			}
			if _, err := os.Stat(targetPath); err == nil {
				if err := repairAgentsScheduleGuidance(targetPath); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if isMainAgent && (key == "soul" || key == "tools") {
			if err := removeGeneratedMainWorkspaceFile(targetPath); err != nil {
				return err
			}
			continue
		}
		content := renderTemplate(workspaceTemplate(key, isMainAgent), context)
		if err := ensureWorkspaceTemplateFile(targetPath, key, content); err != nil {
			return err
		}
	}

	memoryReadmePath := filepath.Join(root, "memory", "README.md")
	if _, err := os.Stat(memoryReadmePath); os.IsNotExist(err) {
		if err = os.WriteFile(memoryReadmePath, []byte("# memory/\n\nDaily notes, summaries, research fragments, temporary conclusions, and reusable memory assets live here.\n"), 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	for _, skillName := range retiredBaseSkillNames {
		if err := UndeploySkill(root, skillName); err != nil {
			return err
		}
	}
	for _, skillName := range managedSkillNames(isMainAgent) {
		if err := deployManagedSkill(skillName, root, context); err != nil {
			return err
		}
	}
	return nil
}

func workspaceInitializationLock(workspacePath string) *sync.Mutex {
	key := filepath.Clean(strings.TrimSpace(workspacePath))
	value, _ := workspaceInitializationLocks.LoadOrStore(key, &sync.Mutex{})
	return value.(*sync.Mutex)
}
