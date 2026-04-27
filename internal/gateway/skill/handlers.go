package skill

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装技能域 HTTP handlers。
type Handlers struct {
	api    *gatewayshared.API
	skills *skillspkg.Service
}

// New 创建技能域 handlers。
func New(api *gatewayshared.API, skills *skillspkg.Service) *Handlers {
	return &Handlers{
		api:    api,
		skills: skills,
	}
}

// HandleListSkills 返回技能列表。
func (h *Handlers) HandleListSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := h.skills.ListSkills(request.Context(), skillspkg.Query{
		AgentID:     request.URL.Query().Get("agent_id"),
		CategoryKey: request.URL.Query().Get("category_key"),
		SourceType:  request.URL.Query().Get("source_type"),
		Q:           request.URL.Query().Get("q"),
	})
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

// HandleGetSkillDetail 返回单个技能详情。
func (h *Handlers) HandleGetSkillDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.GetSkillDetail(request.Context(), chi.URLParam(request, "skill_name"), request.URL.Query().Get("agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) || strings.Contains(strings.ToLower(gatewayshared.ErrString(err)), "not found") {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleAgentSkills 返回 agent 已安装技能。
func (h *Handlers) HandleAgentSkills(writer http.ResponseWriter, request *http.Request) {
	items, err := h.skills.GetAgentSkills(request.Context(), chi.URLParam(request, "agent_id"))
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

// HandleInstallAgentSkill 安装技能到 agent。
func (h *Handlers) HandleInstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		SkillName string `json:"skill_name"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.skills.InstallSkill(request.Context(), chi.URLParam(request, "agent_id"), payload.SkillName)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(err.Error(), "仅允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUninstallAgentSkill 卸载 agent 技能。
func (h *Handlers) HandleUninstallAgentSkill(writer http.ResponseWriter, request *http.Request) {
	err := h.skills.UninstallSkill(request.Context(), chi.URLParam(request, "agent_id"), chi.URLParam(request, "skill_name"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不能") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleImportLocalSkill 导入本地技能。
func (h *Handlers) HandleImportLocalSkill(writer http.ResponseWriter, request *http.Request) {
	filePayload, filename, localPath, err := h.parseLocalSkillImportRequest(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, "请求参数错误")
		return
	}
	var item *skillspkg.Detail
	if len(filePayload) > 0 {
		item, err = h.skills.ImportUploadedArchive(filename, filePayload)
	} else {
		item, err = h.skills.ImportLocalPath(localPath)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "SKILL.md") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteSkill 删除技能。
func (h *Handlers) HandleDeleteSkill(writer http.ResponseWriter, request *http.Request) {
	err := h.skills.DeleteSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		if strings.Contains(err.Error(), "不允许") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleImportGitSkill 导入 Git 技能。
func (h *Handlers) HandleImportGitSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.skills.ImportGit(request.Context(), payload.URL, payload.Branch)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSearchExternalSkills 搜索外部技能。
func (h *Handlers) HandleSearchExternalSkills(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.SearchExternalSkills(
		request.Context(),
		request.URL.Query().Get("q"),
		strings.EqualFold(request.URL.Query().Get("include_readme"), "true"),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandlePreviewExternalSkill 预览外部技能。
func (h *Handlers) HandlePreviewExternalSkill(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.GetExternalSkillPreview(request.Context(), request.URL.Query().Get("detail_url"))
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleImportSkillsShSkill 导入 skills.sh 技能。
func (h *Handlers) HandleImportSkillsShSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		PackageSpec string `json:"package_spec"`
		SkillSlug   string `json:"skill_slug"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.skills.ImportSkillsSh(request.Context(), payload.PackageSpec, payload.SkillSlug)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateImportedSkills 更新全部导入技能。
func (h *Handlers) HandleUpdateImportedSkills(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.UpdateImportedSkills(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateSingleSkill 更新单个技能。
func (h *Handlers) HandleUpdateSingleSkill(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.UpdateSingleSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) parseLocalSkillImportRequest(request *http.Request) ([]byte, string, string, error) {
	contentType := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, header, err := request.FormFile("file")
		if err == nil {
			defer file.Close()
			payload, readErr := io.ReadAll(file)
			return payload, header.Filename, "", readErr
		}
		localPath := strings.TrimSpace(request.FormValue("local_path"))
		return nil, "", localPath, nil
	}
	var payload struct {
		LocalPath string `json:"local_path"`
	}
	if err := gatewayshared.DecodeJSONBody(request.Body, &payload, false); err != nil {
		return nil, "", "", err
	}
	return nil, "", payload.LocalPath, nil
}
