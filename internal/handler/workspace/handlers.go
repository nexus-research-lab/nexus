// INPUT: 已鉴权的 workspace HTTP 请求、路径、正文与可选 revision 前提。
// OUTPUT: 旧 envelope 兼容响应；文件修改携带可证明的数据影响与恢复动作。
// POS: workspace HTTP 边界；不从传输错误猜测提交结果，也不改变 Agent/path 身份。
package workspace

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	"github.com/go-chi/chi/v5"
)

const (
	workspaceFileDispositionAttachment = "attachment"
	workspaceFileDispositionInline     = "inline"
)

type workspaceMutationOperation string

const (
	workspaceMutationCreate workspaceMutationOperation = "create"
	workspaceMutationDelete workspaceMutationOperation = "delete"
	workspaceMutationRename workspaceMutationOperation = "rename"
	workspaceMutationUpload workspaceMutationOperation = "upload"
)

// Handlers 封装工作区 HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	workspace *workspacepkg.Service
}

// New 创建工作区 handlers。
func New(api *handlershared.API, workspace *workspacepkg.Service) *Handlers {
	return &Handlers{
		api:       api,
		workspace: workspace,
	}
}

// HandleWorkspaceFiles 返回工作区文件列表。
func (h *Handlers) HandleWorkspaceFiles(writer http.ResponseWriter, request *http.Request) {
	items, err := h.workspace.ListFiles(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleWorkspaceMemory 返回 SDK 文件式记忆的工作区投影。
func (h *Handlers) HandleWorkspaceMemory(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.GetMemorySnapshot(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteWorkspaceMemory 删除正文记忆并同步清理 MEMORY.md 索引。
func (h *Handlers) HandleDeleteWorkspaceMemory(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.DeleteMemoryDocument(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		request.URL.Query().Get("path"),
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if handlershared.IsClientMessageError(err) ||
			strings.Contains(err.Error(), "路径") ||
			strings.Contains(err.Error(), "记忆文件") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleWorkspaceFile 返回单个工作区文件。
func (h *Handlers) HandleWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.GetFile(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if handlershared.IsClientMessageError(err) || strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateWorkspaceFile 更新工作区文件内容。
func (h *Handlers) HandleUpdateWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path             string  `json:"path"`
		Content          string  `json:"content"`
		ExpectedRevision *string `json:"expected_revision"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.workspace.UpdateFileIfRevision(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		payload.Path,
		payload.Content,
		payload.ExpectedRevision,
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, workspacepkg.ErrFileRevisionConflict) {
		h.api.WriteError(writer, request, http.StatusConflict, workspaceFileRevisionConflict())
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func workspaceFileRevisionConflict() handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "workspace.file_revision_conflict",
		Category: protocol.FailureCategoryConflict,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "文件已在其他位置更新",
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "workspace.reload_file",
		},
	}
}

// HandleCreateWorkspaceEntry 创建工作区条目。
func (h *Handlers) HandleCreateWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path      string `json:"path"`
		EntryType string `json:"entry_type"`
		Content   string `json:"content"`
	}
	if !h.api.BindJSONError(writer, request, &payload, workspaceMutationRequestFailure(workspaceMutationCreate)) {
		return
	}
	item, err := h.workspace.CreateEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.EntryType, payload.Content)
	if err != nil {
		h.writeWorkspaceMutationError(writer, request, workspaceMutationCreate, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleRenameWorkspaceEntry 重命名工作区条目。
func (h *Handlers) HandleRenameWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if !h.api.BindJSONError(writer, request, &payload, workspaceMutationRequestFailure(workspaceMutationRename)) {
		return
	}
	item, err := h.workspace.RenameEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.NewPath)
	if err != nil {
		h.writeWorkspaceMutationError(writer, request, workspaceMutationRename, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteWorkspaceEntry 删除工作区条目。
func (h *Handlers) HandleDeleteWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.DeleteEntry(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if err != nil {
		h.writeWorkspaceMutationError(writer, request, workspaceMutationDelete, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUploadWorkspaceFile 上传工作区文件。
func (h *Handlers) HandleUploadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	file, header, err := request.FormFile("file")
	if err != nil {
		spec := workspaceMutationRequestFailure(workspaceMutationUpload)
		spec.Detail = "缺少上传文件"
		spec.Cause = err
		h.api.WriteError(writer, request, http.StatusBadRequest, spec)
		return
	}
	defer file.Close()

	item, err := h.workspace.UploadFile(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		header.Filename,
		request.FormValue("path"),
		file,
	)
	if err != nil {
		h.writeWorkspaceMutationError(writer, request, workspaceMutationUpload, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func workspaceMutationRequestFailure(operation workspaceMutationOperation) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "workspace." + string(operation) + "_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "请求参数错误",
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "workspace.review_request",
		},
	}
}

func (h *Handlers) writeWorkspaceMutationError(
	writer http.ResponseWriter,
	request *http.Request,
	operation workspaceMutationOperation,
	err error,
) {
	status, spec := workspaceMutationFailure(operation, err)
	h.api.WriteError(writer, request, status, spec)
}

func workspaceMutationFailure(
	operation workspaceMutationOperation,
	err error,
) (int, handlershared.FailureSpec) {
	prefix := "workspace." + string(operation)
	if errors.Is(err, agentpkg.ErrAgentNotFound) ||
		((operation == workspaceMutationRename || operation == workspaceMutationDelete) &&
			errors.Is(err, workspacepkg.ErrFileNotFound)) {
		return http.StatusNotFound, handlershared.FailureSpec{
			Code:     prefix + "_not_found",
			Category: protocol.FailureCategoryNotFound,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   "资源不存在",
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "workspace.reload_files",
			},
		}
	}
	if errors.Is(err, workspacepkg.ErrMutationInvalid) {
		return http.StatusBadRequest, handlershared.FailureSpec{
			Code:     prefix + "_invalid",
			Category: protocol.FailureCategoryValidation,
			Effect:   protocol.FailureEffectNotApplied,
			Detail:   workspaceMutationInvalidDetail(operation),
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "workspace.review_request",
			},
		}
	}
	return http.StatusInternalServerError, handlershared.FailureSpec{
		Code:     prefix + "_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   workspaceMutationFailureDetail(operation),
		Cause:    err,
		Resolution: &protocol.FailureResolution{
			Actor:  protocol.FailureRecoveryActorUser,
			Action: "workspace.reload_files",
		},
	}
}

func workspaceMutationInvalidDetail(operation workspaceMutationOperation) string {
	switch operation {
	case workspaceMutationCreate:
		return "文件名称或位置不符合要求"
	case workspaceMutationRename:
		return "新的文件名称或位置不符合要求"
	case workspaceMutationDelete:
		return "要删除的文件位置不符合要求"
	case workspaceMutationUpload:
		return "上传位置不符合要求"
	default:
		return "请求内容不符合要求"
	}
}

func workspaceMutationFailureDetail(operation workspaceMutationOperation) string {
	switch operation {
	case workspaceMutationCreate:
		return "创建失败"
	case workspaceMutationRename:
		return "重命名失败"
	case workspaceMutationDelete:
		return "删除失败"
	case workspaceMutationUpload:
		return "上传失败"
	default:
		return "文件操作失败"
	}
}

// HandleDownloadWorkspaceFile 下载工作区文件。
func (h *Handlers) HandleDownloadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	file, fileName, err := h.workspace.OpenFileForDownload(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		request.URL.Query().Get("path"),
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	writer.Header().Set(
		"Content-Disposition",
		buildWorkspaceFileDispositionHeader(fileName, request.URL.Query().Get("disposition")),
	)
	info, err := file.Stat()
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	http.ServeContent(writer, request, fileName, info.ModTime(), file)
}

// HandleRevealWorkspaceFile 在桌面端文件管理器中定位工作区文件。
func (h *Handlers) HandleRevealWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	filePath, err := h.workspace.RevealFileInFolder(request.Context(), chi.URLParam(request, "agent_id"), payload.Path)
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, workspacepkg.ErrLocalFileRevealUnavailable) {
		h.api.WriteFailure(writer, http.StatusBadRequest, "仅桌面端支持在文件夹中显示")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]string{"path": filePath})
}

// 中文注释：预览与下载共用同一路由，但内容处置必须显式分流，避免 PDF/图片预览复用下载语义。
func buildWorkspaceFileDispositionHeader(fileName string, requestedDisposition string) string {
	disposition := workspaceFileDispositionAttachment
	if requestedDisposition == workspaceFileDispositionInline {
		disposition = workspaceFileDispositionInline
	}
	return mime.FormatMediaType(disposition, map[string]string{"filename": fileName})
}
