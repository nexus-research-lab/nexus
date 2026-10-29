// INPUT: 宿主 canonical 状态根、宿主用户目录、已装配的桌面后端与权限模式。
// OUTPUT: 受限 macOS/Windows 会话对 app 整树写入、Skill 投影以外 app 条目、Control 目录、既有 sidecar 身份记录及用户凭据目录的拒绝规则。
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
func applyDesktopHostPaths(options agentclient.Options, input AgentClientOptionsInput, platform, appRoot, home string) (agentclient.Options, error) {
	if (platform != "darwin" && platform != "windows") || !input.DesktopSandboxEnabled || !strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop") || options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
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
	// Skill 投影仍须可读，但整个 app 树不可写；其余 app 条目一律不可读。
	for _, root := range roots {
		names, err := desktopHostPrivateAppEntries(root)
		if err != nil {
			return agentclient.Options{}, err
		}
		// 原生宿主 sidecar 登记与本机 Control 密钥位于状态根；禁止任务读取或改写。
		paths := []string{filepath.Join(filepath.Dir(root), "NexusSidecar.pid.json"), filepath.Join(filepath.Dir(root), "control")}
		for _, name := range names {
			paths = append(paths, filepath.Join(root, name))
		}
		for _, path := range paths {
			aliases, err := desktopHostRootAliases(path)
			if err != nil {
				return agentclient.Options{}, err
			}
			privateRoots = appendHostPaths(privateRoots, aliases...)
		}
	}
	writeRoots := appendHostPaths(roots, privateRoots...)
	secretRoots, err := desktopUserSecretPaths(home)
	if err != nil {
		return agentclient.Options{}, err
	}
	privateRoots = appendHostPaths(privateRoots, secretRoots...)
	if platform == "windows" {
		privateRoots, err = windowsProtectionRoots(privateRoots)
		if err != nil {
			return agentclient.Options{}, err
		}
		writeRoots, err = windowsProtectionRoots(writeRoots)
		if err != nil {
			return agentclient.Options{}, err
		}
	}
	filesystem.DenyRead = appendHostPaths(filesystem.DenyRead, privateRoots...)
	filesystem.DenyWrite = appendHostPaths(filesystem.DenyWrite, writeRoots...)
	settings := *options.Sandbox
	settings.Filesystem = &filesystem
	options.Sandbox = &settings
	if platform == "darwin" && options.Runtime.Kind == agentclient.RuntimeClaude {
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

// desktopHostPublicAppEntries 是 app 根下受限会话唯一可读的 Skill 投影。
var desktopHostPublicAppEntries = []string{"platform-skills", "host-skills"}

// desktopHostPrivateAppNames 覆盖尚未创建的 canonical 私有条目。
var desktopHostPrivateAppNames = []string{"data", "config", "cache", "logs", "rooms", "backups", ".migrations", ".agents", "sidecar.lock"}

// desktopHostPrivateAppEntries 拒绝固定私有条目与 app 根下现有的全部非 Skill 条目，
// 使备份、调试输出等未登记目录不会因默认可读而暴露。
func desktopHostPrivateAppEntries(root string) ([]string, error) {
	names := slices.Clone(desktopHostPrivateAppNames)
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("list desktop app directory: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !slices.Contains(desktopHostPublicAppEntries, name) && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names, nil
}

// desktopUserSecretNames 是受限会话默认不可读的用户凭据位置（相对宿主用户目录）。
// Seatbelt 默认放开全盘读取，这些内容一旦被命令读出就会进入模型上下文。
// 需要它们的命令（如 ssh 推送）应走单次沙箱外执行审批。
var desktopUserSecretNames = []string{
	".ssh",
	".aws",
	".gnupg",
	".netrc",
	".git-credentials",
	".config/gh",
	".kube",
	".docker/config.json",
	".config/gcloud",
	".azure",
	".pypirc",
	".cargo/credentials",
	".cargo/credentials.toml",
	"Library/Keychains",
	"Library/Cookies",
	"Library/Application Support/Google/Chrome",
	"Library/Application Support/Firefox",
}

// desktopUserSecretPaths 只从宿主进程的用户目录生成，不读取任务 Env 中的 HOME。
func desktopUserSecretPaths(home string) ([]string, error) {
	if home == "" {
		return nil, nil
	}
	var result []string
	for _, name := range desktopUserSecretNames {
		aliases, err := desktopHostRootAliases(filepath.Join(home, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		result = appendHostPaths(result, aliases...)
	}
	return result, nil
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

// windowsProtectionRoots preserves denial when PSEC cannot resolve a missing
// intermediate directory: deny the first absent ancestor instead of dropping
// the rule or creating directories in the user's home.
func windowsProtectionRoots(paths []string) ([]string, error) {
	var result []string
	for _, path := range paths {
		target := path
		for {
			parent := filepath.Dir(target)
			if parent == target {
				break
			}
			if _, err := os.Lstat(parent); err == nil {
				break
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("resolve Windows protection parent: %w", err)
			}
			target = parent
		}
		result = appendHostPaths(result, target)
	}
	return result, nil
}
