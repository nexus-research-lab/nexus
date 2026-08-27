package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	// 中文注释：初始化会重建托管 skill 目录，同一 workspace 并发执行会互相删除正在复制的文件。
	workspaceInitializationLocks sync.Map
)

var defaultDirs = []string{".agents", ".claude"}

func projectRoot() string {
	return appfs.Root()
}

const (
	workspaceInitializationStateDirectory = "workspace-initialization"
	// 修改模板、托管目录或修复规则时递增，旧 workspace 会在下一次使用前自动升级。
	workspaceInitializationRevision = 1
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
	return ensureInitializedAt(rootFS, agentID, agentName, isMainAgent, createdAt, nil)
}

// EnsureInitializedOnceForAgentAt 使用宿主状态标记，仅在初始化版本或托管状态变化时执行完整初始化。
//
// 标记位于 owner state，和 workspace 结果分离；runtime 即使改写标记也不能
// 绕过结果校验，宿主不会只凭标记跳过初始化。
func EnsureInitializedOnceForAgentAt(
	rootFS *confinedfs.Root,
	agentValue protocol.Agent,
) error {
	stateRoot, markerName, err := openWorkspaceInitializationState(agentValue)
	if err != nil {
		return err
	}
	defer stateRoot.Close()
	return ensureInitializedAt(
		rootFS,
		agentValue.AgentID,
		agentValue.Name,
		agentValue.IsMain,
		agentValue.CreatedAt,
		&workspaceInitializationState{
			root:    stateRoot,
			marker:  markerName,
			version: workspaceInitializationVersion(agentValue),
			isMain:  agentValue.IsMain,
		},
	)
}

type workspaceInitializationState struct {
	root    *confinedfs.Root
	marker  string
	version string
	isMain  bool
}

func ensureInitializedAt(
	rootFS *confinedfs.Root,
	agentID string,
	agentName string,
	isMainAgent bool,
	createdAt time.Time,
	state *workspaceInitializationState,
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
	if state != nil && workspaceInitializationReady(rootFS, state) {
		return nil
	}
	initializer := workspaceInitializer{
		root:    root,
		isMain:  isMainAgent,
		context: buildTemplateContext(agentID, agentName, root, createdAt),
		rootFS:  rootFS,
	}
	if err := initializer.run(); err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	return state.root.WriteFileAtomic(
		state.marker,
		[]byte(state.version+"\n"),
		appfs.RuntimeCollaborativeFileMode(0o600),
	)
}

func openWorkspaceInitializationState(
	agentValue protocol.Agent,
) (*confinedfs.Root, string, error) {
	stateRootPath := appfs.StateRoot()
	if err := os.MkdirAll(stateRootPath, 0o700); err != nil {
		return nil, "", err
	}
	stateRoot, err := confinedfs.Open(stateRootPath)
	if err != nil {
		return nil, "", err
	}
	relative, marker := workspaceInitializationStateLocation(agentValue)
	markers, err := stateRoot.OpenOrCreateRootNoSymlink(
		relative,
		appfs.RuntimeCollaborativeDirectoryMode(0o700),
	)
	closeErr := stateRoot.Close()
	if err != nil {
		return nil, "", err
	}
	if closeErr != nil {
		markers.Close()
		return nil, "", closeErr
	}
	return markers, marker, nil
}

func removeWorkspaceInitializationState(agentValue protocol.Agent) error {
	stateRoot, err := confinedfs.Open(appfs.StateRoot())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer stateRoot.Close()
	relative, marker := workspaceInitializationStateLocation(agentValue)
	markers, err := stateRoot.OpenRootNoSymlink(relative)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer markers.Close()
	err = markers.Remove(marker)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func workspaceInitializationStateLocation(agentValue protocol.Agent) (string, string) {
	return filepath.ToSlash(filepath.Join(
		"users",
		appfs.UserPathSegment(agentValue.OwnerUserID),
		"state",
		workspaceInitializationStateDirectory,
	)), appfs.UserPathSegment(agentValue.AgentID) + ".manifest"
}

func workspaceInitializationVersion(agentValue protocol.Agent) string {
	return fmt.Sprintf(
		"revision=%d;is_main=%t",
		workspaceInitializationRevision,
		agentValue.IsMain,
	)
}

func workspaceInitializationReady(
	rootFS *confinedfs.Root,
	state *workspaceInitializationState,
) bool {
	if state == nil || state.root == nil {
		return false
	}
	payload, err := state.root.ReadFile(state.marker)
	return err == nil &&
		strings.TrimSpace(string(payload)) == state.version &&
		workspaceManagedStateReady(rootFS, state.isMain)
}

func workspaceManagedStateReady(rootFS *confinedfs.Root, isMain bool) bool {
	for _, directory := range defaultDirs {
		info, err := rootFS.Lstat(filepath.ToSlash(directory))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false
		}
	}
	emotion, err := rootFS.Lstat(".agents/emotion.json")
	if err != nil || emotion.Mode()&os.ModeSymlink != 0 || !emotion.Mode().IsRegular() {
		return false
	}
	for _, skillName := range managedSkillNames(isMain) {
		for _, parent := range []string{".agents/skills", ".claude/skills"} {
			_, err = rootFS.Lstat(filepath.ToSlash(filepath.Join(parent, skillName)))
			if !os.IsNotExist(err) {
				return false
			}
		}
	}
	return sharedRuntimeCLIShimsReady()
}

func sharedRuntimeCLIShimsReady() bool {
	for _, name := range []string{"nexusctl", "nexusctl.cmd", "nexuscfg", "nexuscfg.cmd"} {
		info, err := os.Lstat(filepath.Join(appfs.AgentRuntimeBinDir(), name))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false
		}
		if runtime.GOOS != "windows" && !strings.HasSuffix(name, ".cmd") && info.Mode().Perm()&0o111 == 0 {
			return false
		}
	}
	return true
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
	if err := ensureRuntimeCLIShims(appfs.AgentRuntimeBinDir(), i.context); err != nil {
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
