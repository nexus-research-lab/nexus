package launcher

import (
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	launcherpkg "github.com/nexus-research-lab/nexus/internal/service/launcher"
)

// Handlers 封装 launcher 域 HTTP handlers。
type Handlers struct {
	api      *handlershared.API
	launcher *launcherpkg.Service
}

// New 创建 launcher handlers。
func New(api *handlershared.API, launcher *launcherpkg.Service) *Handlers {
	return &Handlers{
		api:      api,
		launcher: launcher,
	}
}

// HandleLauncherBootstrap 返回 launcher 启动数据。
func (h *Handlers) HandleLauncherBootstrap(writer http.ResponseWriter, request *http.Request) {
	item, err := h.launcher.Bootstrap(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}
