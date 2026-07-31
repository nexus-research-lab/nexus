// INPUT: 已认证 owner 与 session_key 查询参数。
// OUTPUT: 当前或最近一次安全 WorkGraph JSON 投影。
// POS: Web/桌面端读取 Execution UI 状态的只读 HTTP 边界。
package execution

import (
	"context"
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionViewer interface {
	GetLatestView(context.Context, string, string) (*protocol.ExecutionView, error)
}

// Handlers 封装 Execution WorkGraph 只读接口。
type Handlers struct {
	api        *handlershared.API
	executions executionViewer
}

// New 创建 Execution handlers。
func New(api *handlershared.API, executions executionViewer) *Handlers {
	return &Handlers{api: api, executions: executions}
}

// HandleGetLatestExecution 返回 session 当前或最近一次 WorkGraph。
func (h *Handlers) HandleGetLatestExecution(
	writer http.ResponseWriter,
	request *http.Request,
) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "session_key 不能为空")
		return
	}
	if h.executions == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "Execution 服务不可用")
		return
	}
	view, err := h.executions.GetLatestView(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		sessionKey,
	)
	if err != nil {
		var domainErr *orchestrationsvc.DomainError
		if errors.As(err, &domainErr) {
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "Execution 查询参数无效")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, "Execution 状态读取失败")
		return
	}
	h.api.WriteSuccess(writer, view)
}
