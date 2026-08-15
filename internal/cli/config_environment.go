// INPUT: nexusctl / nexuscfg 继承的宿主或 Agent runtime 环境变量。
// OUTPUT: 指向 Nexus 宿主状态根和 workspace 基址的 Config。
// POS: 控制面 CLI 入口与通用 config.Load 之间的目录布局适配层。
package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const (
	nexusConfigDirEnvName = "NEXUS_CONFIG_DIR"
	workspacePathEnvName  = "WORKSPACE_PATH"
)

// LoadConfig 从当前进程环境加载控制面 CLI 使用的宿主配置。
func LoadConfig() config.Config {
	normalizeRuntimeLayoutEnvironment()
	return config.Load()
}

// LoadConfigurationConfig 只在人工终端模式加载宿主配置。
// Agent runtime 的 nexuscfg 由 broker 提供全部配置能力，不能反推或打开宿主状态根。
func LoadConfigurationConfig() config.Config {
	if runtimeConfigurationBrokerConfigured() {
		return config.Config{}
	}
	return LoadConfig()
}

// normalizeRuntimeLayoutEnvironment 把用户 runtime 根还原为控制面 CLI 的宿主视图。
//
// nxs 与 Claude 必须继续把 NEXUS_CONFIG_DIR 指向 users/<owner>/runtime；
// 控制面 CLI 则直接装配宿主服务，不能把该目录误当成 NEXUS_STATE_ROOT，也不能
// 把当前 Agent workspace 误当成所有用户的 workspace 基址。
func normalizeRuntimeLayoutEnvironment() {
	stateRoot, ownerSegment, ok := stateRootFromRuntimeConfigDir(
		os.Getenv(nexusConfigDirEnvName),
	)
	if !ok {
		return
	}

	explicitStateRoot := strings.TrimSpace(os.Getenv(appfs.NexusStateRootEnvName))
	if explicitStateRoot != "" && !sameCLIPath(appfs.StateRoot(), stateRoot) {
		return
	}
	if explicitStateRoot == "" {
		_ = os.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	}

	workspaceBase := workspaceBaseFromRuntimePath(
		os.Getenv(nexusctlWorkspacePathEnvName),
		ownerSegment,
	)
	if workspaceBase == "" {
		workspaceBase = filepath.Join(stateRoot, "users")
	}
	_ = os.Setenv(workspacePathEnvName, workspaceBase)
}

func stateRootFromRuntimeConfigDir(configDir string) (string, string, bool) {
	runtimeRoot := filepath.Clean(strings.TrimSpace(configDir))
	if runtimeRoot == "." || !equalCLIPathSegment(filepath.Base(runtimeRoot), "runtime") {
		return "", "", false
	}
	ownerRoot := filepath.Dir(runtimeRoot)
	ownerSegment := filepath.Base(ownerRoot)
	usersRoot := filepath.Dir(ownerRoot)
	if ownerSegment == "." || ownerSegment == string(filepath.Separator) ||
		!equalCLIPathSegment(filepath.Base(usersRoot), "users") {
		return "", "", false
	}
	stateRoot := filepath.Dir(usersRoot)
	if !sameCLIPath(
		runtimeRoot,
		appfs.UserRuntimeRootAt(stateRoot, ownerSegment),
	) {
		return "", "", false
	}
	return stateRoot, ownerSegment, true
}

func workspaceBaseFromRuntimePath(workspacePath string, ownerSegment string) string {
	current := filepath.Clean(strings.TrimSpace(workspacePath))
	if current == "." || strings.TrimSpace(ownerSegment) == "" {
		return ""
	}
	for {
		workspaceRoot := filepath.Dir(current)
		ownerRoot := filepath.Dir(workspaceRoot)
		if equalCLIPathSegment(filepath.Base(workspaceRoot), "workspace") &&
			equalCLIPathSegment(filepath.Base(ownerRoot), ownerSegment) {
			return filepath.Dir(ownerRoot)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func equalCLIPathSegment(left string, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func sameCLIPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
