package computeruse

import (
	"errors"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	computerusesvc "github.com/nexus-research-lab/nexus/internal/service/computeruse"
)

type Handlers struct {
	api     *handlershared.API
	service *computerusesvc.Service
}

func New(api *handlershared.API, service *computerusesvc.Service) *Handlers {
	return &Handlers{api: api, service: service}
}

type statusResponse struct {
	computerusesvc.CUAInstallationStatus
	Permissions *permissionResponse `json:"permissions,omitempty"`
}

type permissionResponse struct {
	ScreenCapture string `json:"screen_capture"`
	Accessibility string `json:"accessibility"`
	InputControl  string `json:"input_control"`
}

func (h *Handlers) HandleStatus(writer http.ResponseWriter, request *http.Request) {
	writeNoStore(writer)
	if h.service == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "Computer Use 当前不可用")
		return
	}
	h.api.WriteSuccess(writer, statusResponse{CUAInstallationStatus: h.service.InstallationStatus()})
}

func (h *Handlers) HandleInstall(writer http.ResponseWriter, request *http.Request) {
	writeNoStore(writer)
	if h.service == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "Computer Use 当前不可用")
		return
	}
	status, err := h.service.Install(request.Context())
	if err != nil {
		h.writeServiceError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, statusResponse{CUAInstallationStatus: status})
}

func (h *Handlers) HandleEnable(writer http.ResponseWriter, request *http.Request) {
	h.handleSetEnabled(writer, request, true)
}

func (h *Handlers) HandleDisable(writer http.ResponseWriter, request *http.Request) {
	h.handleSetEnabled(writer, request, false)
}

func (h *Handlers) handleSetEnabled(writer http.ResponseWriter, request *http.Request, enabled bool) {
	writeNoStore(writer)
	if h.service == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "Computer Use 当前不可用")
		return
	}
	if err := h.service.SetEnabled(request.Context(), enabled); err != nil {
		h.writeServiceError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, statusResponse{CUAInstallationStatus: h.service.InstallationStatus()})
}

func (h *Handlers) HandleCheckPermissions(writer http.ResponseWriter, request *http.Request) {
	writeNoStore(writer)
	if h.service == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "Computer Use 当前不可用")
		return
	}
	permissions, err := h.service.CheckPermissions(request.Context())
	if err != nil {
		h.writeServiceError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, statusResponse{
		CUAInstallationStatus: h.service.InstallationStatus(),
		Permissions: &permissionResponse{
			ScreenCapture: string(permissions.ScreenCapture),
			Accessibility: string(permissions.Accessibility),
			InputControl:  string(permissions.InputControl),
		},
	})
}

func (h *Handlers) writeServiceError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	detail := "Computer Use 操作失败，请稍后重试"
	switch {
	case errors.Is(err, computerusesvc.ErrCUANoRelease):
		status = http.StatusNotFound
		detail = "GitHub 暂无可用的 CUA 安装包"
	case errors.Is(err, computerusesvc.ErrCUANotInstalled):
		status = http.StatusConflict
		detail = "请先安装 CUA"
	case errors.Is(err, computerusesvc.ErrDisabled):
		status = http.StatusConflict
		detail = "请先开启 CUA"
	case errors.Is(err, computerusesvc.ErrCUAReleaseRejected), errors.Is(err, computerusesvc.ErrCUAChecksumMismatch):
		status = http.StatusBadGateway
		detail = "GitHub 的 CUA 安装包未通过校验"
	}
	h.api.WriteFailure(writer, status, detail)
}

func writeNoStore(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
}
