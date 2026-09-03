package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestDesktopPersonalProfileAllowsLocalAvatar(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.AppMode = "desktop"
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()

	status := getAuthStatus(t, httpServer.URL)
	if status.AuthRequired || !status.Authenticated || status.Username != "local" {
		t.Fatalf("desktop auth 状态不正确: %+v", status)
	}
	profile := getPersonalProfile(t, httpServer.URL)
	if profile.User.UserID != authsvc.SystemUserID ||
		profile.User.Username != "local" ||
		profile.User.AuthMethod != authsvc.AuthMethodLocal ||
		profile.CanChangePassword ||
		!profile.CanUpdateProfile {
		t.Fatalf("desktop 个人资料不正确: %+v", profile)
	}

	updated := updatePersonalAvatar(t, httpServer.URL, "15")
	if updated.User.Avatar != "15" {
		t.Fatalf("desktop 本地头像更新未生效: %+v", updated.User)
	}
	status = getAuthStatus(t, httpServer.URL)
	if status.Avatar != "15" {
		t.Fatalf("desktop auth status 未返回最新头像: %+v", status)
	}
}

type authStatusResponse struct {
	AuthRequired  bool   `json:"auth_required"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
}

type personalProfileResponse struct {
	User struct {
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
		Avatar     string `json:"avatar"`
		AuthMethod string `json:"auth_method"`
	} `json:"user"`
	CanChangePassword bool `json:"can_change_password"`
	CanUpdateProfile  bool `json:"can_update_profile"`
}

type apiEnvelope[T any] struct {
	Data T `json:"data"`
}

func getAuthStatus(t *testing.T, baseURL string) authStatusResponse {
	t.Helper()
	return getJSON[authStatusResponse](t, http.MethodGet, baseURL+"/nexus/v1/auth/status", nil)
}

func getPersonalProfile(t *testing.T, baseURL string) personalProfileResponse {
	t.Helper()
	return getJSON[personalProfileResponse](t, http.MethodGet, baseURL+"/nexus/v1/settings/profile", nil)
}

func updatePersonalAvatar(t *testing.T, baseURL string, avatar string) personalProfileResponse {
	t.Helper()
	body, err := json.Marshal(map[string]string{"avatar": avatar})
	if err != nil {
		t.Fatal(err)
	}
	return getJSON[personalProfileResponse](
		t,
		http.MethodPatch,
		baseURL+"/nexus/v1/settings/profile",
		bytes.NewReader(body),
	)
}

func getJSON[T any](t *testing.T, method string, endpoint string, body io.Reader) T {
	t.Helper()
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s %s status = %d", method, endpoint, response.StatusCode)
	}
	var payload apiEnvelope[T]
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data
}
