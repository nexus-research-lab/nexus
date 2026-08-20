// INPUT: 已认证 owner、session_key/exact Workflow 查询参数。
// OUTPUT: 当前/历史安全 WorkGraph JSON 投影，以及 owner Workflow 目录读取/删除。
// POS: Web/桌面端 WorkGraph 管理 HTTP 边界；Workflow 创建只允许 Skill + CLI。
package execution

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionViewer interface {
	GetLatestView(context.Context, string, string) (*protocol.ExecutionView, error)
}

type executionHistoryViewer interface {
	ListHistoryViews(context.Context, string, string, int) ([]protocol.ExecutionView, error)
}

type workflowManager interface {
	List(context.Context, string) ([]protocol.WorkGraphWorkflow, error)
	Delete(context.Context, string, string) (bool, error)
}

// Handlers 封装 Execution WorkGraph 只读接口。
type Handlers struct {
	api        *handlershared.API
	executions executionViewer
	workflows  workflowManager
}

// New 创建 Execution handlers。
func New(
	api *handlershared.API,
	executions executionViewer,
	workflows ...workflowManager,
) *Handlers {
	handler := &Handlers{api: api, executions: executions}
	if len(workflows) > 0 {
		handler.workflows = workflows[0]
	}
	return handler
}

// HandleListExecutionHistory 返回 session 最近的 managed WorkGraph 历史。
func (h *Handlers) HandleListExecutionHistory(
	writer http.ResponseWriter,
	request *http.Request,
) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "session_key 不能为空")
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	history, ok := h.executions.(executionHistoryViewer)
	if !ok {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "Execution 历史服务不可用")
		return
	}
	views, err := history.ListHistoryViews(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		sessionKey,
		limit,
	)
	if err != nil {
		h.writeExecutionError(writer, err, "Execution 历史读取失败")
		return
	}
	h.api.WriteSuccess(writer, views)
}

// HandleListWorkGraphWorkflows 返回当前 owner 的命名 Workflow 目录。
func (h *Handlers) HandleListWorkGraphWorkflows(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "WorkGraph Workflow 服务不可用")
		return
	}
	items, err := h.workflows.List(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "WorkGraph Workflow 读取失败")
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleDeleteWorkGraphWorkflow 删除命名 Workflow 及其 Slash command。
func (h *Handlers) HandleDeleteWorkGraphWorkflow(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "WorkGraph Workflow 服务不可用")
		return
	}
	deleted, err := h.workflows.Delete(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		chi.URLParam(request, "workflow_id"),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "WorkGraph Workflow 删除失败")
		return
	}
	if !deleted {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"deleted": true})
}

func (h *Handlers) writeExecutionError(
	writer http.ResponseWriter,
	err error,
	fallback string,
) {
	var domainErr *orchestrationsvc.DomainError
	if errors.As(err, &domainErr) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "Execution 查询参数无效")
		return
	}
	h.api.WriteFailure(writer, http.StatusInternalServerError, fallback)
}

// HandleGetLatestExecution 返回 session 当前或最近一次 managed WorkGraph。
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
