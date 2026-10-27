// INPUT: SDK 返回的沙箱不可用原因。
// OUTPUT: 已知 Windows 环境问题的用户提示；未知原因不投影。
// POS: Runtime 启动和显式诊断共用的错误呈现。
package runtime

import "strings"

// DesktopSandboxUnavailableMessage 只映射已知原因，不暴露任意探测输出。
func DesktopSandboxUnavailableMessage(reason string) string {
	switch {
	case strings.Contains(reason, "elevated administrator token"):
		return "Nexus 正以管理员权限运行，桌面沙箱不可用。请以普通用户身份重新启动 Nexus。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	case strings.Contains(reason, "does not provide the process security environment API"), strings.Contains(reason, "process security environment API is unavailable"):
		return "当前 Windows 版本不支持桌面沙箱，需要 Windows 11 24H2（build 26100.9278）或更新版本。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。"
	default:
		return ""
	}
}
