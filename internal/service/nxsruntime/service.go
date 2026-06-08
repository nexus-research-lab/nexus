package nxsruntime

import (
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"

	bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
)

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
	Ensure() (bridgenxs.Status, error)
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
	return runtimeStatusFromBridge(s.withDefaults().inspector().Status(), nil)
}

// Download 通过 bridge resolver 下载并缓存 nxs runtime。
func (s *Service) Download() (RuntimeStatus, error) {
	status, err := s.withDefaults().inspector().Ensure()
	return runtimeStatusFromBridge(status, err), err
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
	return bridgenxs.NewRuntimeInspector(bridgenxs.WithAppRoot(appfs.Root()))
}

func runtimeStatusFromBridge(status bridgenxs.Status, err error) RuntimeStatus {
	return RuntimeStatus{
		Available:   status.Available,
		Path:        status.Path,
		Source:      string(status.Source),
		CanDownload: status.CanDownload,
		Message:     runtimeStatusMessage(status, err),
	}
}

func runtimeStatusMessage(status bridgenxs.Status, err error) string {
	switch status.Error {
	case bridgenxs.StatusErrorEnvNotExecutable:
		return "NEXUS_NXS_COMMAND_PATH 指向的 nxs 不可执行，请修正或清空后再下载。"
	case bridgenxs.StatusErrorAppRootNotExecutable:
		return "Nexus 桌面包内置的 nxs runtime 不可执行，请重新安装或更新应用。"
	case bridgenxs.StatusErrorDownloadedNotExecutable:
		return "nxs runtime 下载完成但文件不可执行。"
	case bridgenxs.StatusErrorDownloadFailed:
		if err != nil {
			return fmt.Sprintf("nxs runtime 下载失败：%v", err)
		}
		return "nxs runtime 下载失败。"
	case bridgenxs.StatusErrorNotFound:
		return "当前未找到可用 nxs runtime，可以下载后再切换。"
	default:
		if !status.Available && status.CanDownload {
			return "当前未找到可用 nxs runtime，可以下载后再切换。"
		}
		return ""
	}
}
