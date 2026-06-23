package nxsruntime

import bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"

// RuntimeStatus 表示 nxs runtime 在当前主机上的可用状态。
type RuntimeStatus struct {
	Available   bool   `json:"available"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source,omitempty"`
	CanDownload bool   `json:"can_download"`
	Message     string `json:"message,omitempty"`
}

type runtimeInspector interface {
	Status() bridgenxs.Status
}

// Service 负责探测和拉取 nxs runtime。
type Service struct {
	inspector func() runtimeInspector
}

// NewService 创建 nxs runtime 服务。
func NewService() *Service {
	return &Service{
		inspector: defaultInspector,
	}
}

// Status 只检查本地已存在的 nxs runtime，不触发下载。
func (s *Service) Status() RuntimeStatus {
	status := s.withDefaults().inspector().Status()
	return RuntimeStatus{
		Available:   status.Available,
		Path:        status.Path,
		Source:      string(status.Source),
		CanDownload: status.CanDownload,
		Message:     runtimeStatusMessage(status),
	}
}

func (s *Service) withDefaults() *Service {
	if s == nil {
		return NewService()
	}
	result := *s
	if result.inspector == nil {
		result.inspector = defaultInspector
	}
	return &result
}

func defaultInspector() runtimeInspector {
	return bridgenxs.NewRuntimeInspector()
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
