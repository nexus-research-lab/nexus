package workspace

import (
	"errors"
	"net/http"
	"strings"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	"github.com/go-chi/chi/v5"
)

const (
	workspaceFileDispositionAttachment = "attachment"
	workspaceFileDispositionInline     = "inline"
)

// Handlers 封装工作区 HTTP handlers。
type Handlers struct {
	api       *gatewayshared.API
	workspace *workspacepkg.Service
}

// New 创建工作区 handlers。
func New(api *gatewayshared.API, workspace *workspacepkg.Service) *Handlers {
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

// HandleWorkspaceFile 返回单个工作区文件。
func (h *Handlers) HandleWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.GetFile(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) || strings.Contains(err.Error(), "文件路径") || strings.Contains(err.Error(), "目录") {
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
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.workspace.UpdateFile(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.Content)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
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

// HandleCreateWorkspaceEntry 创建工作区条目。
func (h *Handlers) HandleCreateWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Path      string `json:"path"`
		EntryType string `json:"entry_type"`
		Content   string `json:"content"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.workspace.CreateEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.EntryType, payload.Content)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "存在") || strings.Contains(err.Error(), "仅支持") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
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
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.workspace.RenameEntry(request.Context(), chi.URLParam(request, "agent_id"), payload.Path, payload.NewPath)
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "相同") || strings.Contains(err.Error(), "存在") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteWorkspaceEntry 删除工作区条目。
func (h *Handlers) HandleDeleteWorkspaceEntry(writer http.ResponseWriter, request *http.Request) {
	item, err := h.workspace.DeleteEntry(request.Context(), chi.URLParam(request, "agent_id"), request.URL.Query().Get("path"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) || errors.Is(err, workspacepkg.ErrFileNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUploadWorkspaceFile 上传工作区文件。
func (h *Handlers) HandleUploadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	file, header, err := request.FormFile("file")
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, "缺少上传文件")
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
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") || strings.Contains(err.Error(), "限制") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDownloadWorkspaceFile 下载工作区文件。
func (h *Handlers) HandleDownloadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	filePath, fileName, err := h.workspace.GetFileForDownload(
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
	writer.Header().Set(
		"Content-Disposition",
		buildWorkspaceFileDispositionHeader(fileName, request.URL.Query().Get("disposition")),
	)
	http.ServeFile(writer, request, filePath)
}

// 中文注释：预览与下载共用同一路由，但内容处置必须显式分流，避免 PDF/图片预览复用下载语义。
func buildWorkspaceFileDispositionHeader(fileName string, requestedDisposition string) string {
	if requestedDisposition == workspaceFileDispositionInline {
		return `inline; filename="` + fileName + `"`
	}
	return `attachment; filename="` + fileName + `"`
}
