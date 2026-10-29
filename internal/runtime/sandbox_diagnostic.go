// INPUT: SDK 返回的沙箱不可用原因。
// OUTPUT: 已知 Windows 环境问题的用户提示；未知原因不投影。
// POS: Runtime 启动和显式诊断共用的错误呈现。
package runtime

import "strings"

// DesktopSandboxUnavailableMessage 只映射已知原因，不暴露任意探测输出。
func DesktopSandboxUnavailableMessage(reason string) string {
	switch {
	case strings.Contains(reason, "sandbox helper nxs-mxc.exe is missing"), strings.Contains(reason, "sandbox helper version mismatch"):
		return "Nexus 的 Windows 沙箱组件缺失或版本不匹配，请重新安装完整安装包，无需自行安装 Rust 或 MXC。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	case strings.Contains(reason, "elevated administrator token"):
		return "Nexus 正以管理员权限运行，桌面沙箱不可用。请以普通用户身份重新启动 Nexus。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	case strings.Contains(reason, "CreateProcessSecurityEnvironment") && strings.Contains(strings.ToLower(reason), "0x80070032"):
		return "Windows 已提供沙箱 API，但不支持当前安全环境创建请求（0x80070032）。这不代表缺少可单独安装的 MXC 组件。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	case strings.Contains(reason, "Windows PSEC contract"), strings.Contains(reason, "Windows PSEC NetworkIngress is not supported"):
		return "当前 Windows 不支持本次桌面沙箱策略所需的 PSEC 能力。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	case strings.Contains(reason, "does not provide the process security environment API"), strings.Contains(reason, "process security environment API is unavailable"):
		return "当前 Windows 未提供桌面沙箱所需的系统 API。请检查 Windows 更新；仅凭版本号不能确认沙箱可用。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	default:
		return ""
	}
}
