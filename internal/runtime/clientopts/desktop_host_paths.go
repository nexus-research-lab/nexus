// INPUT: 宿主 canonical 状态根、已装配的桌面后端与权限模式。
// OUTPUT: 受限 macOS 会话对 app 整树写入及私有目录读取的后端拒绝规则。
// POS: 统一 DM/Room/后台 options 装配；Full Access 是明确的无隔离例外。
package clientopts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// applyDesktopHostPaths 只从宿主输入生成保护，不读取任务 Env 中的状态根。
// 这是命令/文件工具策略，不宣称整个 SDK 进程、IPC 或第三方服务被隔离。
func applyDesktopHostPaths(options agentclient.Options, input AgentClientOptionsInput, platform, appRoot string) (agentclient.Options, error) {
	if platform != "darwin" || !strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop") || options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
		return options, nil
	}
	if options.Sandbox == nil || options.Sandbox.Filesystem == nil {
		return agentclient.Options{}, errors.New("desktop host protection requires backend filesystem policy")
	}
	roots, err := desktopHostRootAliases(appRoot)
	if err != nil {
		return agentclient.Options{}, err
	}
	filesystem := *options.Sandbox.Filesystem
	var privateRoots []string
	// 与 canonical app 布局对应；Skill 投影仍须可读，但整个 app 树不可写。
	for _, root := range roots {
		for _, name := range []string{"data", "config", "cache", "logs", "rooms", "processes", ".migrations", ".agents", "sidecar.lock"} {
			aliases, err := desktopHostRootAliases(filepath.Join(root, name))
			if err != nil {
				return agentclient.Options{}, err
			}
			privateRoots = appendHostPaths(privateRoots, aliases...)
		}
	}
	filesystem.DenyRead = appendHostPaths(filesystem.DenyRead, privateRoots...)
	writeRoots := appendHostPaths(roots, privateRoots...)
	filesystem.DenyWrite = appendHostPaths(filesystem.DenyWrite, writeRoots...)
	settings := *options.Sandbox
	settings.Filesystem = &filesystem
	options.Sandbox = &settings
	if options.Runtime.Kind == agentclient.RuntimeClaude {
		for _, root := range privateRoots {
			options.Tools.Deny = appendDistinctStrings(options.Tools.Deny, "Read(/"+filepath.ToSlash(root)+")", "Read(/"+filepath.ToSlash(root)+"/**)")
		}
		for _, root := range writeRoots {
			// Claude 的绝对路径规则用双斜杠；Edit 同时约束原生 Write。
			options.Tools.Deny = appendDistinctStrings(options.Tools.Deny, "Edit(/"+filepath.ToSlash(root)+")", "Edit(/"+filepath.ToSlash(root)+"/**)")
		}
	}
	return options, nil
}

// desktopHostRootAliases 固定词法/物理两种拼写，允许新状态根尚未创建。
func desktopHostRootAliases(root string) ([]string, error) {
	if root == "" || strings.ContainsRune(root, 0) {
		return nil, errors.New("invalid desktop host directory")
	}
	// 老宿主配置可使用相对状态根；只按宿主 cwd 解析，不按任务 workspace。
	absolute, err := filepath.Abs(root)
	if err != nil || absolute == string(filepath.Separator) {
		return nil, errors.New("invalid desktop host directory")
	}
	root = absolute
	current := root
	var missing []string
	for {
		physical, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				physical = filepath.Join(physical, missing[i])
			}
			return appendHostPaths([]string{root}, physical), nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("resolve desktop host directory: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, errors.New("desktop host directory has no resolvable parent")
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// appendHostPaths 保留合法路径中的空格，不复用会 TrimSpace 的工具名合并函数。
func appendHostPaths(base []string, extra ...string) []string {
	result := slices.Clone(base)
	for _, path := range extra {
		if !slices.Contains(result, path) {
			result = append(result, path)
		}
	}
	return result
}
