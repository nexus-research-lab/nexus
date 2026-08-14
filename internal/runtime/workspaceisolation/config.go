// INPUT: 部署级 runtime isolation 配置与当前 Agent 身份。
// OUTPUT: 可注入 bridge 的 launcher 元数据和统一 workspace policy。
// POS: nxs/Claude 共用的宿主隔离装配入口。
package workspaceisolation

import (
	"fmt"
	"strings"
)

const (
	// LauncherTicketEnvName 把 root-owned launcher 生成的策略票据交给实际启动进程。
	LauncherTicketEnvName = "NEXUS_RUNTIME_ISOLATION_TICKET"
	// LauncherModeEnvName 只用于 runtime 诊断；launcher 会在 exec 前移除。
	LauncherModeEnvName = "NEXUS_RUNTIME_ISOLATION_MODE"
	// ScriptRuntimeKind 是 launcher 内置的受限自动化脚本执行型。
	ScriptRuntimeKind = "script"
)

// Mode 表示宿主 workspace policy 的执行级别。
type Mode string

const (
	ModeOff     Mode = "off"
	ModeAudit   Mode = "audit"
	ModeEnforce Mode = "enforce"
)

// Config 描述一次 runtime 启动使用的宿主隔离配置。
type Config struct {
	Mode         Mode
	LauncherPath string
}

// NormalizeMode 解析部署配置；未知值直接报错，避免拼写错误静默关闭边界。
func NormalizeMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case "", ModeOff:
		return ModeOff, nil
	case ModeAudit:
		return ModeAudit, nil
	case ModeEnforce:
		return ModeEnforce, nil
	default:
		return "", fmt.Errorf("不支持的 runtime isolation mode: %q", value)
	}
}

// Input 是当前 runtime 会话不可由模型修改的身份与路径快照。
type Input struct {
	OwnerUserID      string
	RuntimeKind      string
	CWD              string
	ReadRoots        []string
	WriteRoots       []string
	EnvironmentNames []string
	// IsMainAgent 表示当前 runtime 是否属于 Nexus 主智能体。
	// 主智能体是 owner-scoped 控制面主体；普通 Agent 永远不继承该能力。
	IsMainAgent bool
}
