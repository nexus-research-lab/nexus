package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	// 中文注释：初始化会重建托管 skill 目录，同一 workspace 并发执行会互相删除正在复制的文件。
	workspaceInitializationLocks sync.Map
)

// EnsureInitialized 保证 workspace 模板就绪，并确保平台 Skill 不落入 Agent workspace。
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
	if err := os.MkdirAll(root, workspaceDirectoryMode()); err != nil {
		return err
	}
	rootFS, err := confinedfs.Open(root)
	if err != nil {
		return err
	}
	defer rootFS.Close()
	return EnsureInitializedAt(rootFS, agentID, agentName, isMainAgent, createdAt)
}

// EnsureInitializedAt 在已验证的 workspace 根中完成初始化。
func EnsureInitializedAt(
	rootFS *confinedfs.Root,
	agentID string,
	agentName string,
	isMainAgent bool,
	createdAt time.Time,
) error {
	if rootFS == nil {
		return fmt.Errorf("workspace 根句柄不能为空")
	}
	root := strings.TrimSpace(rootFS.Name())
	if root == "" {
		return fmt.Errorf("workspace 根路径不能为空")
	}
	lock := workspaceInitializationLock(root)
	lock.Lock()
	defer lock.Unlock()
	initializer := workspaceInitializer{
		root:    root,
		isMain:  isMainAgent,
		context: buildTemplateContext(agentID, agentName, root, createdAt),
		rootFS:  rootFS,
	}
	return initializer.run()
}

type workspaceInitializer struct {
	root    string
	isMain  bool
	context map[string]string
	rootFS  *confinedfs.Root
}

type mainWorkspaceFileInitializer func(*workspaceInitializer, string) error

var mainWorkspaceFileInitializers = map[string]mainWorkspaceFileInitializer{
	"agents": (*workspaceInitializer).ensureMainAgentsFile,
	"soul":   (*workspaceInitializer).removeGeneratedMainFile,
	"tools":  (*workspaceInitializer).removeGeneratedMainFile,
}

func (i *workspaceInitializer) run() error {
	if i.rootFS == nil {
		if err := i.ensureDirectories(); err != nil {
			return err
		}
		rootFS, err := confinedfs.Open(i.root)
		if err != nil {
			return err
		}
		i.rootFS = rootFS
		defer rootFS.Close()
	} else if err := i.ensureDirectoriesAt(); err != nil {
		return err
	}
	if err := agentsvc.EnsureRuntimeEmotionStateAt(i.rootFS); err != nil {
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
	if err := os.MkdirAll(i.root, workspaceDirectoryMode()); err != nil {
		return err
	}
	root, err := confinedfs.Open(i.root)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, dir := range defaultDirs {
		directoryRoot, err := root.OpenOrCreateRootNoSymlink(
			filepath.ToSlash(dir),
			workspaceDirectoryMode(),
		)
		if err != nil {
			return err
		}
		_ = directoryRoot.Close()
	}
	return nil
}

func (i *workspaceInitializer) ensureDirectoriesAt() error {
	for _, dir := range defaultDirs {
		directoryRoot, err := i.rootFS.OpenOrCreateRootNoSymlink(
			filepath.ToSlash(dir),
			workspaceDirectoryMode(),
		)
		if err != nil {
			return err
		}
		if err = directoryRoot.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (i *workspaceInitializer) ensureRuntimeTools() error {
	if err := ensureNexusctlShim(appfs.AgentRuntimeBinDir(), i.context); err != nil {
		return err
	}
	return removeWorkspaceBinShim(i.rootFS)
}

func (i *workspaceInitializer) ensureTemplateFiles() error {
	for key, relativePath := range workspaceFiles {
		if err := i.ensureTemplateFile(key, relativePath); err != nil {
			return err
		}
	}
	return nil
}

func (i *workspaceInitializer) ensureTemplateFile(key string, relativePath string) error {
	if i.isMain {
		if initializer := mainWorkspaceFileInitializers[key]; initializer != nil {
			return initializer(i, relativePath)
		}
	}
	content := renderTemplate(workspaceTemplate(key, i.isMain), i.context)
	return ensureWorkspaceTemplateFile(i.rootFS, relativePath, key, content)
}

func (i *workspaceInitializer) ensureMainAgentsFile(relativePath string) error {
	if err := removeGeneratedMainAgentsPrompt(i.rootFS, relativePath); err != nil {
		return err
	}
	if _, err := i.rootFS.Lstat(relativePath); err == nil {
		return repairAgentsScheduleGuidance(i.rootFS, relativePath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (i *workspaceInitializer) removeGeneratedMainFile(relativePath string) error {
	return removeGeneratedMainWorkspaceFile(i.rootFS, relativePath)
}

func (i *workspaceInitializer) ensureSkills() error {
	for _, skillName := range managedSkillNames(i.isMain) {
		if err := UndeploySkillAt(i.rootFS, skillName); err != nil {
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
