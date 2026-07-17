package operation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	operationpkg "github.com/nexus-research-lab/nexus/internal/service/operation"
)

func TestHandleGetStageSnapshotReturnsEmptyPayloadWhenMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := operationpkg.NewService(config.Config{CacheFileDir: filepath.Join(root, "cache")})
	handler := New(handlershared.NewAPI(nil), service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/operation/stage/snapshot?key=session:test", nil)
	handler.HandleGetStageSnapshot(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("缺失舞台快照应返回成功空结果，实际状态码: %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Key       string          `json:"key"`
			Snapshot  json.RawMessage `json:"snapshot"`
			UpdatedAt string          `json:"updated_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("响应 JSON 无法解析: %v", err)
	}
	if !payload.Success || payload.Data.Key != "session:test" || string(payload.Data.Snapshot) != "null" || payload.Data.UpdatedAt != "" {
		t.Fatalf("缺失舞台快照响应不正确: %+v", payload)
	}
}

func TestHandleStagePresenceLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := operationpkg.NewService(config.Config{CacheFileDir: filepath.Join(root, "cache")})
	handler := New(handlershared.NewAPI(nil), service)
	sessionKey := "room:group:conversation-stage"

	touchRecorder := httptest.NewRecorder()
	touchRequest := httptest.NewRequest(
		http.MethodPut,
		"/nexus/v1/operation/stage/presence",
		strings.NewReader(`{"session_key":"room:group:conversation-stage","client_id":"browser-1"}`),
	)
	touchRequest.Header.Set("Content-Type", "application/json")
	handler.HandleTouchStagePresence(touchRecorder, touchRequest)

	if touchRecorder.Code != http.StatusOK {
		t.Fatalf("建立舞台在线租约失败: status=%d body=%s", touchRecorder.Code, touchRecorder.Body.String())
	}
	if !service.IsStageActive(sessionKey) {
		t.Fatal("建立在线租约后舞台应处于活跃状态")
	}

	closeRecorder := httptest.NewRecorder()
	closeRequest := httptest.NewRequest(
		http.MethodDelete,
		"/nexus/v1/operation/stage/presence",
		strings.NewReader(`{"session_key":"room:group:conversation-stage","client_id":"browser-1"}`),
	)
	closeRequest.Header.Set("Content-Type", "application/json")
	handler.HandleCloseStagePresence(closeRecorder, closeRequest)

	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("关闭舞台在线租约失败: status=%d body=%s", closeRecorder.Code, closeRecorder.Body.String())
	}
	if service.IsStageActive(sessionKey) {
		t.Fatal("关闭最后一个在线租约后舞台不应继续活跃")
	}
}

func TestHandleBrowserPageReturnsCleanInAppError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := operationpkg.NewService(config.Config{CacheFileDir: filepath.Join(root, "cache")})
	handler := New(handlershared.NewAPI(nil), service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/operation/browser/page?url=http%3A%2F%2F127.0.0.1%2Fadmin",
		nil,
	)
	handler.HandleBrowserPage(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("本机地址应被拒绝: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(recorder.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("Navi 错误页响应头不完整: %+v", recorder.Header())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "这个地址不可访问") || !strings.Contains(body, "nexus-navi-proxy") {
		t.Fatalf("Navi 错误页内容不正确: %s", body)
	}
	if strings.Contains(body, `"success"`) || strings.Contains(body, "browser page address denied") {
		t.Fatalf("错误页不应暴露 API 包裹或内部错误: %s", body)
	}
}
