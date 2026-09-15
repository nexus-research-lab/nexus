// INPUT: 配置的本机 nxs 路径与显式诊断请求上下文。
// OUTPUT: 独立文件可用性及可选命令沙箱诊断，不变更授权。
// POS: Runtime 设置只读服务；诊断失败不否定原引擎可用性。
package nxsruntime

import (
	"context"
	bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
)

// RuntimeStatus 表示 nxs runtime 在当前主机上的可用状态。
type RuntimeStatus struct {
	Available   bool           `json:"available"`
	Path        string         `json:"path,omitempty"`
	Source      string         `json:"source,omitempty"`
	CanDownload bool           `json:"can_download"`
	Message     string         `json:"message,omitempty"`
	Sandbox     *SandboxStatus `json:"sandbox,omitempty"`
}

// Status 只检查本地已存在的 nxs runtime，不触发下载。
func Status() RuntimeStatus {
	status := bridgenxs.NewRuntimeInspector().Status()
	return RuntimeStatus{
		Available:   status.Available,
		Path:        status.Path,
		Source:      string(status.Source),
		CanDownload: status.CanDownload,
		Message:     runtimeStatusMessage(status),
	}
}

func runtimeStatusMessage(status bridgenxs.Status) string {
	switch status.Error {
	case bridgenxs.StatusErrorEnvNotExecutable:
		return "NEXUS_NXS_COMMAND_PATH 指向的 nxs 不可执行，请修正路径。"
	case bridgenxs.StatusErrorNotFound:
		return "未配置 nxs runtime。桌面包会由 sidecar 注入 NEXUS_NXS_COMMAND_PATH；开发环境请设置 NEXUS_NXS_COMMAND_PATH 指向本地 nxs。"
	default:
		return ""
	}
}

// SandboxStatus 独立报告诊断结论，不授予执行或变更审批模式。
type SandboxStatus struct {
	State    string `json:"state"`
	Platform string `json:"platform,omitempty"`
}

// StatusWithSandbox 仅供显式检查调用；旧的 Status 保持无进程启动的文件检查。
func StatusWithSandbox(ctx context.Context) RuntimeStatus {
	status := Status()
	status.Sandbox = &SandboxStatus{State: "unknown"}
	if !status.Available {
		return status
	}
	diagnosis, err := bridgenxs.NewRuntimeInspector().SandboxStatus(ctx)
	status.Sandbox = projectSandboxStatus(diagnosis, err)
	return status
}

// projectSandboxStatus 不把查询失败当成禁用，也不把依赖齐全当成已启用。
func projectSandboxStatus(diagnosis *bridgenxs.SandboxBackendStatus, err error) *SandboxStatus {
	result := &SandboxStatus{State: "unknown"}
	if err != nil || diagnosis == nil {
		return result
	}
	result.Platform = diagnosis.Platform
	switch {
	case !diagnosis.BackendSupported:
		result.State = "unsupported"
	case !diagnosis.DependenciesAvailable:
		result.State = "missing_dependencies"
	default:
		result.State = "dependencies_available"
	}
	return result
}
