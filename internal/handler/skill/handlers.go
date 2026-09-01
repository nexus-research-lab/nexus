// INPUT: 已认证 owner 的 Skill HTTP 请求、路由身份与上传/远端来源参数。
// OUTPUT: Skill 查询/写响应；Marketplace 写失败显式携带 FailureCore 提交事实。
// POS: Skill HTTP 适配层；业务写由 service 持有，Handler 只投影已知阶段和安全文案。
package skill

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装技能域 HTTP handlers。
type Handlers struct {
	api    *handlershared.API
	skills *skillspkg.Service
}

// New 创建技能域 handlers。
func New(api *handlershared.API, skills *skillspkg.Service) *Handlers {
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
		Scope:       request.URL.Query().Get("scope"),
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
	if errors.Is(err, agentpkg.ErrAgentNotFound) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleListSkillAgents 返回 Skill 在各 Agent 上的启用状态。
func (h *Handlers) HandleListSkillAgents(writer http.ResponseWriter, request *http.Request) {
	items, err := h.skills.ListSkillAgents(
		request.Context(),
		chi.URLParam(request, "skill_name"),
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) ||
		(err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleAgentSkills 返回 Agent 可见 Skill 及其启用状态。
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

// HandleInstallAgentSkill 保留旧启用入口。
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

// HandleSetAgentSkillEnabled 更新 Agent 的技能启用开关。
func (h *Handlers) HandleSetAgentSkillEnabled(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Enabled     *bool  `json:"enabled"`
		TargetScope string `json:"target_scope"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if payload.Enabled == nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, "enabled 不能为空")
		return
	}
	targetScope := skillspkg.AgentSkillTargetScope(strings.TrimSpace(payload.TargetScope))
	if targetScope == "" {
		h.api.WriteFailure(writer, http.StatusBadRequest, "target_scope 不能为空")
		return
	}
	item, err := h.skills.SetAgentSkillEnabledInScope(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		chi.URLParam(request, "skill_name"),
		*payload.Enabled,
		targetScope,
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "skill not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		if strings.Contains(err.Error(), "不能") ||
			strings.Contains(err.Error(), "仅允许") ||
			strings.Contains(err.Error(), "target_scope") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUninstallAgentSkill 保留旧停用入口。
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
		h.api.WriteError(writer, request, http.StatusBadRequest, skillRequestFailure(
			"skill.import_local_request_invalid",
			"请选择有效的 Skill 压缩包或本地路径",
		))
		return
	}
	var item *skillspkg.Detail
	if len(filePayload) > 0 {
		item, err = h.skills.ImportUploadedArchive(request.Context(), filename, filePayload)
	} else {
		item, err = h.skills.ImportLocalPath(request.Context(), localPath)
	}
	if err != nil {
		status := http.StatusInternalServerError
		category := protocol.FailureCategoryInternal
		detail := "技能没有导入"
		if !skillspkg.SkillMutationNeedsReconcile(err) {
			switch {
			case errors.Is(err, skillspkg.ErrLocalPathImportUnavailable):
				status = http.StatusForbidden
				category = protocol.FailureCategoryAuthorization
				detail = "当前环境不允许从本地路径导入"
			case errors.Is(err, os.ErrNotExist), strings.Contains(err.Error(), "SKILL.md"):
				status = http.StatusBadRequest
				category = protocol.FailureCategoryValidation
				detail = "没有找到可导入的 SKILL.md"
			}
		}
		status, spec := skillMutationFailure(
			err,
			"skill.import_local_failed",
			status,
			category,
			detail,
		)
		h.api.WriteError(writer, request, status, spec)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteSkill 删除技能。
func (h *Handlers) HandleDeleteSkill(writer http.ResponseWriter, request *http.Request) {
	err := h.skills.DeleteSkill(request.Context(), chi.URLParam(request, "skill_name"))
	if err != nil {
		status := http.StatusInternalServerError
		category := protocol.FailureCategoryInternal
		detail := "技能没有删除"
		if !skillspkg.SkillMutationNeedsReconcile(err) &&
			(strings.Contains(err.Error(), "不允许") ||
				strings.Contains(strings.ToLower(err.Error()), "not found")) {
			status = http.StatusBadRequest
			category = protocol.FailureCategoryValidation
			detail = "这个技能不能删除或已经不存在"
		}
		status, spec := skillMutationFailure(
			err,
			"skill.delete_failed",
			status,
			category,
			detail,
		)
		h.api.WriteError(writer, request, status, spec)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleImportGitSkill 导入 Git 技能。
func (h *Handlers) HandleImportGitSkill(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
		Path   string `json:"path"`
	}
	if !h.api.BindJSONError(writer, request, &payload, skillRequestFailure(
		"skill.import_git_request_invalid",
		"Git 导入信息格式不正确",
	)) {
		return
	}
	item, err := h.skills.ImportGitPath(request.Context(), payload.URL, payload.Branch, payload.Path)
	if err != nil {
		status, spec := skillMutationFailure(
			err,
			"skill.import_git_failed",
			http.StatusBadRequest,
			protocol.FailureCategoryValidation,
			"无法从这个 Git 地址导入技能",
		)
		h.api.WriteError(writer, request, status, spec)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSearchExternalSkills 搜索外部技能。
func (h *Handlers) HandleSearchExternalSkills(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.SearchExternalSkillsFromSource(
		request.Context(),
		request.URL.Query().Get("q"),
		strings.EqualFold(request.URL.Query().Get("include_readme"), "true"),
		request.URL.Query().Get("source_id"),
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

// HandleImportSkillsShSkill 导入社区技能，兼容历史 skills.sh 接口路径。
func (h *Handlers) HandleImportSkillsShSkill(writer http.ResponseWriter, request *http.Request) {
	var payload skillspkg.ExternalSkillSearchItem
	if !h.api.BindJSONError(writer, request, &payload, skillRequestFailure(
		"skill.import_external_request_invalid",
		"外部 Skill 信息格式不正确",
	)) {
		return
	}
	item, err := h.skills.ImportExternalSkill(request.Context(), payload)
	if err != nil {
		status, spec := skillMutationFailure(
			err,
			"skill.import_external_failed",
			http.StatusBadRequest,
			protocol.FailureCategoryValidation,
			"无法导入这个外部技能",
		)
		h.api.WriteError(writer, request, status, spec)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleImportPrivateSkill 从私有来源安全导入指定 Skill。
func (h *Handlers) HandleImportPrivateSkill(writer http.ResponseWriter, request *http.Request) {
	var payload skillspkg.ImportPrivateSkillRequest
	if !h.api.BindJSONError(writer, request, &payload, skillRequestFailure(
		"skill.import_external_request_invalid",
		"私有来源 Skill 信息格式不正确",
	)) {
		return
	}
	item, err := h.skills.ImportPrivateSkillFromSource(request.Context(), payload)
	if err != nil {
		status, spec := skillMutationFailure(
			err,
			"skill.import_external_failed",
			http.StatusBadRequest,
			protocol.FailureCategoryValidation,
			"无法导入这个外部技能",
		)
		h.api.WriteError(writer, request, status, spec)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleListExternalSkillSources 返回社区与私有 skill 来源。
func (h *Handlers) HandleListExternalSkillSources(writer http.ResponseWriter, request *http.Request) {
	items, err := h.skills.ListExternalSkillSources(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleCreateExternalSkillSource 新增一个私有 skill 来源。
func (h *Handlers) HandleCreateExternalSkillSource(writer http.ResponseWriter, request *http.Request) {
	var payload skillspkg.CreateExternalSkillSourceRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.skills.CreateExternalSkillSource(request.Context(), payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateExternalSkillSource 更新 skill 来源配置。
func (h *Handlers) HandleUpdateExternalSkillSource(writer http.ResponseWriter, request *http.Request) {
	var payload skillspkg.ExternalSkillSourceRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.skills.UpdateExternalSkillSource(request.Context(), chi.URLParam(request, "source_id"), payload)
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

// HandleDeleteExternalSkillSource 删除一个用户私有 skill 来源。
func (h *Handlers) HandleDeleteExternalSkillSource(writer http.ResponseWriter, request *http.Request) {
	err := h.skills.DeleteExternalSkillSource(request.Context(), chi.URLParam(request, "source_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleCheckSkillUpdates 检查外部导入技能是否有更新。
func (h *Handlers) HandleCheckSkillUpdates(writer http.ResponseWriter, request *http.Request) {
	item, err := h.skills.CheckImportedSkillUpdates(request.Context())
	if err != nil {
		status, spec := skillMutationFailure(
			err,
			"skill.check_updates_failed",
			http.StatusInternalServerError,
			protocol.FailureCategoryInternal,
			"无法检查技能更新",
		)
		h.api.WriteError(writer, request, status, spec)
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
		if !skillspkg.SkillMutationNeedsReconcile(err) &&
			strings.Contains(strings.ToLower(err.Error()), "not found") {
			status, spec := skillNotFoundMutationFailure(
				err,
				"skill.update_failed",
			)
			h.api.WriteError(writer, request, status, spec)
			return
		}
		status, spec := skillMutationFailure(
			err,
			"skill.update_failed",
			http.StatusBadRequest,
			protocol.FailureCategoryValidation,
			"无法更新这个技能",
		)
		h.api.WriteError(writer, request, status, spec)
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
	if err := handlershared.DecodeJSONBody(request.Body, &payload, false); err != nil {
		return nil, "", "", err
	}
	return nil, "", payload.LocalPath, nil
}
