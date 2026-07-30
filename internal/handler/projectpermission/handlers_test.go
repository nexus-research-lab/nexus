package projectpermission

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	projectsvc "github.com/nexus-research-lab/nexus/internal/service/projectpermission"
)

func TestVisibleProjectsFiltersAndRedactsMembersForRegularUser(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	request = request.WithContext(authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-b",
		Role:   authctx.RoleMember,
	}))
	projects := []projectsvc.Project{
		{
			ProjectID: "shared",
			Members: map[string]string{
				"user-a": "write",
				"user-b": "read",
			},
		},
		{
			ProjectID: "private",
			Members:   map[string]string{"user-a": "write"},
		},
	}

	visible := visibleProjects(request, projects)
	if len(visible) != 1 || visible[0].ProjectID != "shared" {
		t.Fatalf("普通成员可见项目错误: %+v", visible)
	}
	if len(visible[0].Members) != 1 || visible[0].Members["user-b"] != "read" {
		t.Fatalf("普通成员不应看到其他成员: %+v", visible[0].Members)
	}
}

func TestVisibleProjectsKeepsFullViewForAdmin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	request = request.WithContext(authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "admin",
		Role:   authctx.RoleAdmin,
	}))
	projects := []projectsvc.Project{{
		ProjectID: "shared",
		Members:   map[string]string{"user-a": "write", "user-b": "read"},
	}}

	visible := visibleProjects(request, projects)
	if len(visible) != 1 || len(visible[0].Members) != 2 {
		t.Fatalf("管理员应保留完整项目视图: %+v", visible)
	}
}
