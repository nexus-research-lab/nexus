// INPUT: 已认证 owner、session_key/exact Execution/草图确认与命名工作图查询参数。
// OUTPUT: 当前/历史安全 WorkGraph、durable Draft/版本编辑、隐藏后台保存调度及命名工作图目录读取/删除。
// POS: Web/桌面端 WorkGraph 管理 HTTP 边界；草图持久化只允许后台 Agent 的 Skill + CLI。
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
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type executionViewer interface {
	GetLatestView(context.Context, string, string) (*protocol.ExecutionView, error)
	GetView(context.Context, string, string, string) (*protocol.ExecutionView, error)
}

type executionHistoryViewer interface {
	ListHistoryViews(context.Context, string, string, int) ([]protocol.ExecutionView, error)
}

type workflowManager interface {
	PreviewFromExecution(context.Context, string, protocol.PreviewWorkGraphWorkflowRequest) (*protocol.WorkGraphWorkflowPreview, error)
	PreviewSavedWorkflow(context.Context, string, string, string) (*protocol.WorkGraphWorkflowPreview, error)
	List(context.Context, string) ([]protocol.WorkGraphWorkflow, error)
	ListLocalized(context.Context, string, string) ([]protocol.WorkGraphWorkflow, error)
	Delete(context.Context, string, string) (bool, error)
}

type workflowSaveScheduler interface {
	ScheduleSave(context.Context, string, protocol.ScheduleWorkGraphWorkflowSaveRequest) (*protocol.WorkGraphWorkflowSaveReceipt, error)
}

type workflowSlashNameChecker interface {
	CheckSlashNameAvailability(context.Context, string, string, string) (*protocol.WorkGraphWorkflowSlashNameAvailability, error)
}

type workflowMetadataEditor interface {
	StartMetadataEditor(context.Context, string, protocol.StartWorkGraphWorkflowEditorRequest) (*protocol.WorkGraphWorkflowEditorSession, error)
	GetMetadataEditor(string, protocol.GetWorkGraphWorkflowEditorRequest) (*protocol.WorkGraphWorkflowEditorSession, error)
	ApplyMetadataEditor(string, protocol.ApplyWorkGraphWorkflowEditorRequest) (*protocol.WorkGraphWorkflowPreview, error)
	SelectMetadataEditorVersion(context.Context, string, protocol.SelectWorkGraphWorkflowEditorVersionRequest) (*protocol.WorkGraphWorkflowEditorSession, error)
	CloseMetadataEditor(context.Context, string, string, string) (bool, error)
}

// HandleSelectWorkGraphWorkflowEditorVersion 选择不可变草图版本作为当前编辑基线。
func (h *Handlers) HandleSelectWorkGraphWorkflowEditorVersion(writer http.ResponseWriter, request *http.Request) {
	editor, ok := h.workflows.(workflowMetadataEditor)
	if !ok {
		h.writeWorkflowEditorError(writer, errors.New("workgraph editor is unavailable"))
		return
	}
	var payload protocol.SelectWorkGraphWorkflowEditorVersionRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	payload.EditorID = strings.TrimSpace(chi.URLParam(request, "editor_id"))
	session, err := editor.SelectMetadataEditorVersion(
		request.Context(), authsvc.OwnerUserID(request.Context()), payload,
	)
	if err != nil {
		h.writeWorkflowEditorError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, session)
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

// HandleListWorkGraphWorkflows 返回当前 owner 的命名工作图目录。
func (h *Handlers) HandleListWorkGraphWorkflows(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图服务不可用")
		return
	}
	items, err := h.workflows.ListLocalized(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		request.URL.Query().Get("locale"),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "工作图读取失败")
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleCheckWorkGraphWorkflowSlashName 返回 owner/exact Draft 视角下的命名 Slash 可用性。
func (h *Handlers) HandleCheckWorkGraphWorkflowSlashName(
	writer http.ResponseWriter,
	request *http.Request,
) {
	checker, ok := h.workflows.(workflowSlashNameChecker)
	if !ok || checker == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图命名检查服务不可用")
		return
	}
	availability, err := checker.CheckSlashNameAvailability(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		request.URL.Query().Get("slash_name"),
		request.URL.Query().Get("preview_id"),
	)
	if err != nil {
		if errors.Is(err, workgraphworkflowsvc.ErrInvalidInput) {
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "斜杠命令格式无效")
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, "斜杠命令检查失败")
		return
	}
	h.api.WriteSuccess(writer, availability)
}

// HandlePreviewSavedWorkGraphWorkflow 恢复能力目录命名图的关联 Draft 以继续编辑。
func (h *Handlers) HandlePreviewSavedWorkGraphWorkflow(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图服务不可用")
		return
	}
	var payload protocol.PreviewSavedWorkGraphWorkflowRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	preview, err := h.workflows.PreviewSavedWorkflow(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		chi.URLParam(request, "workflow_id"),
		payload.OutputLanguage,
	)
	if err != nil {
		switch {
		case errors.Is(err, workgraphworkflowsvc.ErrInvalidInput):
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "工作图无法继续编辑")
		case errors.Is(err, workgraphworkflowsvc.ErrNotFound):
			h.api.WriteFailure(writer, http.StatusNotFound, "工作图不存在")
		default:
			h.api.WriteFailure(writer, http.StatusInternalServerError, "工作图草图恢复失败")
		}
		return
	}
	h.api.WriteSuccess(writer, preview)
}

// HandlePreviewWorkGraphWorkflow 使用默认对话模型从 exact 完成图生成或复用 durable Draft。
func (h *Handlers) HandlePreviewWorkGraphWorkflow(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图服务不可用")
		return
	}
	var payload protocol.PreviewWorkGraphWorkflowRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	preview, err := h.workflows.PreviewFromExecution(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		payload,
	)
	if err != nil {
		switch {
		case errors.Is(err, workgraphworkflowsvc.ErrInvalidInput):
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "只能从完成态工作图生成草图")
		case errors.Is(err, workgraphworkflowsvc.ErrNotFound):
			h.api.WriteFailure(writer, http.StatusNotFound, "工作图不存在")
		default:
			h.api.WriteFailure(writer, http.StatusInternalServerError, "工作图草图生成失败")
		}
		return
	}
	h.api.WriteSuccess(writer, preview)
}

// HandleScheduleWorkGraphWorkflowSave 将确认过的 preview 交给不进入聊天时间线的内部 Agent round。
func (h *Handlers) HandleScheduleWorkGraphWorkflowSave(
	writer http.ResponseWriter,
	request *http.Request,
) {
	scheduler, ok := h.workflows.(workflowSaveScheduler)
	if !ok || scheduler == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图后台保存服务不可用")
		return
	}
	var payload protocol.ScheduleWorkGraphWorkflowSaveRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	payload.PreviewID = strings.TrimSpace(chi.URLParam(request, "preview_id"))
	receipt, err := scheduler.ScheduleSave(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		payload,
	)
	if err != nil {
		switch {
		case errors.Is(err, workgraphworkflowsvc.ErrInvalidInput):
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "工作图草图保存请求无效")
		case errors.Is(err, workgraphworkflowsvc.ErrNotFound):
			h.api.WriteFailure(writer, http.StatusNotFound, "工作图草图不存在或已过期")
		case errors.Is(err, workgraphworkflowsvc.ErrNameConflict):
			h.api.WriteFailure(writer, http.StatusConflict, "斜杠命令已存在或不可使用")
		default:
			h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图后台保存启动失败")
		}
		return
	}
	h.api.WriteSuccess(writer, receipt)
}

// HandleStartWorkGraphWorkflowEditor 为 exact Draft 创建或恢复不进入普通会话目录的隐藏编辑会话。
func (h *Handlers) HandleStartWorkGraphWorkflowEditor(writer http.ResponseWriter, request *http.Request) {
	editor, ok := h.workflows.(workflowMetadataEditor)
	if !ok || editor == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图编辑服务不可用")
		return
	}
	var payload protocol.StartWorkGraphWorkflowEditorRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	payload.PreviewID = strings.TrimSpace(chi.URLParam(request, "preview_id"))
	session, err := editor.StartMetadataEditor(request.Context(), authsvc.OwnerUserID(request.Context()), payload)
	if err != nil {
		h.writeWorkflowEditorError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, session)
}

// HandleGetWorkGraphWorkflowEditor 读取隐藏 DM 当前选中草图及不可变版本目录。
func (h *Handlers) HandleGetWorkGraphWorkflowEditor(writer http.ResponseWriter, request *http.Request) {
	editor, ok := h.workflows.(workflowMetadataEditor)
	if !ok || editor == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图编辑服务不可用")
		return
	}
	payload := protocol.GetWorkGraphWorkflowEditorRequest{
		SourceSessionKey: request.URL.Query().Get("source_session_key"),
		EditorID:         strings.TrimSpace(chi.URLParam(request, "editor_id")),
	}
	session, err := editor.GetMetadataEditor(authsvc.OwnerUserID(request.Context()), payload)
	if err != nil {
		h.writeWorkflowEditorError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, session)
}

// HandleApplyWorkGraphWorkflowEditor 把 exact 选中版本投影回当前 preview。
func (h *Handlers) HandleApplyWorkGraphWorkflowEditor(writer http.ResponseWriter, request *http.Request) {
	editor, ok := h.workflows.(workflowMetadataEditor)
	if !ok || editor == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图编辑服务不可用")
		return
	}
	var payload protocol.ApplyWorkGraphWorkflowEditorRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	payload.EditorID = strings.TrimSpace(chi.URLParam(request, "editor_id"))
	preview, err := editor.ApplyMetadataEditor(authsvc.OwnerUserID(request.Context()), payload)
	if err != nil {
		h.writeWorkflowEditorError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, preview)
}

// HandleCloseWorkGraphWorkflowEditor 显式丢弃隐藏编辑会话；普通关闭 UI 不调用。
func (h *Handlers) HandleCloseWorkGraphWorkflowEditor(writer http.ResponseWriter, request *http.Request) {
	editor, ok := h.workflows.(workflowMetadataEditor)
	if !ok || editor == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图编辑服务不可用")
		return
	}
	deleted, err := editor.CloseMetadataEditor(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		request.URL.Query().Get("source_session_key"),
		chi.URLParam(request, "editor_id"),
	)
	if err != nil {
		h.writeWorkflowEditorError(writer, err)
		return
	}
	if !deleted {
		h.api.WriteFailure(writer, http.StatusNotFound, "工作图编辑会话不存在或已过期")
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"deleted": true})
}

func (h *Handlers) writeWorkflowEditorError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workgraphworkflowsvc.ErrInvalidInput):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "工作图编辑请求无效或版本已变化")
	case errors.Is(err, workgraphworkflowsvc.ErrNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "工作图编辑会话不存在或已过期")
	case errors.Is(err, workgraphworkflowsvc.ErrNameConflict):
		h.api.WriteFailure(writer, http.StatusConflict, "斜杠命令已存在或不可使用")
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, "工作图编辑失败")
	}
}

// HandleDeleteWorkGraphWorkflow 删除命名工作图及其 Slash command。
func (h *Handlers) HandleDeleteWorkGraphWorkflow(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if h.workflows == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "工作图服务不可用")
		return
	}
	deleted, err := h.workflows.Delete(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		chi.URLParam(request, "workflow_id"),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, "工作图删除失败")
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

// HandleGetExecution 按 exact owner/session/execution 读取来源 WorkGraph，供草图对照使用。
func (h *Handlers) HandleGetExecution(
	writer http.ResponseWriter,
	request *http.Request,
) {
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	executionID := strings.TrimSpace(chi.URLParam(request, "execution_id"))
	if sessionKey == "" || executionID == "" {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "session_key 和 execution_id 不能为空")
		return
	}
	if h.executions == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "Execution 服务不可用")
		return
	}
	view, err := h.executions.GetView(
		request.Context(),
		authsvc.OwnerUserID(request.Context()),
		sessionKey,
		executionID,
	)
	if err != nil {
		h.writeExecutionError(writer, err, "Execution 状态读取失败")
		return
	}
	if view == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "Execution 不存在")
		return
	}
	h.api.WriteSuccess(writer, view)
}
