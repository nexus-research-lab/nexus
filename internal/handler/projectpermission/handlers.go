package projectpermission

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	projectsvc "github.com/nexus-research-lab/nexus/internal/service/projectpermission"
)

// Handlers 封装共享项目 ACL 控制面。
type Handlers struct {
	api      *shared.API
	projects *projectsvc.Service
}

// New 创建共享项目 ACL handlers。
func New(api *shared.API, projects *projectsvc.Service) *Handlers {
	return &Handlers{api: api, projects: projects}
}

// HandleListProjects 返回当前宿主 registry 中的项目及成员关系。
func (h *Handlers) HandleListProjects(writer http.ResponseWriter, request *http.Request) {
	projects, err := h.projects.List(request.Context())
	if errors.Is(err, projectsvc.ErrUnavailable) {
		h.api.WriteFailure(writer, http.StatusNotImplemented, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, visibleProjects(request, projects))
}

// visibleProjects 将共享项目列表按请求主体收窄。
//
// 管理员和 owner 需要看到完整成员关系来执行运维操作；普通成员只能看
// 自己已经加入的项目，并且不泄露同项目中的其他用户标识。
func visibleProjects(request *http.Request, projects []projectsvc.Project) []projectsvc.Project {
	principal := authctx.PrincipalFromContext(request.Context())
	if principal == nil {
		// 未启用认证时只有本地单用户，保留完整视图以兼容桌面端。
		return projects
	}
	role := strings.TrimSpace(principal.Role)
	if role == authctx.RoleOwner || role == authctx.RoleAdmin {
		return projects
	}

	ownerUserID := authsvc.OwnerUserID(request.Context())
	result := make([]projectsvc.Project, 0, len(projects))
	for _, project := range projects {
		access := strings.TrimSpace(project.Members[ownerUserID])
		if access != "read" && access != "write" {
			continue
		}
		project.Members = map[string]string{ownerUserID: access}
		result = append(result, project)
	}
	return result
}

// HandleEnsureProject 创建项目并将当前用户授予 write。
func (h *Handlers) HandleEnsureProject(writer http.ResponseWriter, request *http.Request) {
	if !canManageProject(request) {
		h.api.WriteFailure(writer, http.StatusForbidden, "project admin access required")
		return
	}
	var payload struct {
		ProjectID string `json:"project_id"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	result, err := h.projects.Ensure(request.Context(), payload.ProjectID)
	if errors.Is(err, projectsvc.ErrInvalidProjectID) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, projectsvc.ErrUnavailable) {
		h.api.WriteFailure(writer, http.StatusNotImplemented, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	ownerUserID := authsvc.OwnerUserID(request.Context())
	if access := result.Project.Members[ownerUserID]; access != "read" && access != "write" {
		// launcher 只会给首次创建者原子授予 write；已存在项目绝不借
		// ensure 自动加成员，避免通过猜项目 ID 扩大共享目录权限。
		h.api.WriteFailure(writer, http.StatusForbidden, "project membership required")
		return
	}
	h.api.WriteSuccess(writer, result.Project)
}

// HandleGrantProjectMember 更新项目成员 ACL。
func (h *Handlers) HandleGrantProjectMember(writer http.ResponseWriter, request *http.Request) {
	if !canManageProjectMember(request) {
		h.api.WriteFailure(writer, http.StatusForbidden, "project member admin access required")
		return
	}
	var payload struct {
		Access string `json:"access"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	result, err := h.projects.Grant(
		request.Context(),
		chi.URLParam(request, "project_id"),
		chi.URLParam(request, "owner_user_id"),
		payload.Access,
	)
	if errors.Is(err, projectsvc.ErrInvalidProjectID) ||
		errors.Is(err, projectsvc.ErrInvalidOwnerUserID) ||
		errors.Is(err, projectsvc.ErrInvalidAccess) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, projectsvc.ErrUnavailable) {
		h.api.WriteFailure(writer, http.StatusNotImplemented, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

func canManageProject(request *http.Request) bool {
	principal := authctx.PrincipalFromContext(request.Context())
	if principal == nil {
		return true
	}
	switch strings.TrimSpace(principal.Role) {
	case authctx.RoleOwner, authctx.RoleAdmin:
		return true
	default:
		return false
	}
}

func canManageProjectMember(request *http.Request) bool {
	principal := authctx.PrincipalFromContext(request.Context())
	if principal == nil {
		return true
	}
	return strings.TrimSpace(principal.Role) == authctx.RoleAdmin
}
