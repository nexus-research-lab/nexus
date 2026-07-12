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
	initializer := workspaceInitializer{
		root:    root,
		isMain:  isMainAgent,
		context: buildTemplateContext(agentID, agentName, root, createdAt),
	}
	return initializer.run()
}

type workspaceInitializer struct {
	root    string
	isMain  bool
	context map[string]string
}

type mainWorkspaceFileInitializer func(*workspaceInitializer, string) error

var mainWorkspaceFileInitializers = map[string]mainWorkspaceFileInitializer{
	"agents": (*workspaceInitializer).ensureMainAgentsFile,
	"soul":   (*workspaceInitializer).removeGeneratedMainFile,
	"tools":  (*workspaceInitializer).removeGeneratedMainFile,
}

func (i *workspaceInitializer) run() error {
	if err := i.ensureDirectories(); err != nil {
		return err
	}
	if err := agentsvc.EnsureRuntimeEmotionState(i.root); err != nil {
		return err
	}
	if err := i.ensureRuntimeTools(); err != nil {
		return err
	}
	if err := i.ensureTemplateFiles(); err != nil {
		return err
	}
	return i.ensureSkills()
}

func (i *workspaceInitializer) ensureDirectories() error {
	if err := os.MkdirAll(i.root, 0o755); err != nil {
		return err
	}
	for _, dir := range defaultDirs {
		if err := os.MkdirAll(filepath.Join(i.root, dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (i *workspaceInitializer) ensureRuntimeTools() error {
	if err := ensureNexusctlShim(appfs.AgentRuntimeBinDir(), i.context); err != nil {
		return err
	}
	return removeWorkspaceBinShim(i.root)
}

func (i *workspaceInitializer) ensureTemplateFiles() error {
	for key, relativePath := range workspaceFiles {
		if err := i.ensureTemplateFile(key, filepath.Join(i.root, relativePath)); err != nil {
			return err
		}
	}
	return nil
}

func (i *workspaceInitializer) ensureTemplateFile(key string, targetPath string) error {
	if i.isMain {
		if initializer := mainWorkspaceFileInitializers[key]; initializer != nil {
			return initializer(i, targetPath)
		}
	}
	content := renderTemplate(workspaceTemplate(key, i.isMain), i.context)
	return ensureWorkspaceTemplateFile(targetPath, key, content)
}

func (i *workspaceInitializer) ensureMainAgentsFile(targetPath string) error {
	if err := removeGeneratedMainAgentsPrompt(targetPath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		return repairAgentsScheduleGuidance(targetPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (i *workspaceInitializer) removeGeneratedMainFile(targetPath string) error {
	return removeGeneratedMainWorkspaceFile(targetPath)
}

func (i *workspaceInitializer) ensureSkills() error {
	for _, skillName := range managedSkillNames(i.isMain) {
		if err := deployManagedSkill(skillName, i.root, i.context); err != nil {
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
