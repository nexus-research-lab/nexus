// INPUT: 真实 workspace HTTP 路由、同一文件的读取 revision 与两次条件写入。
// OUTPUT: 首次提交成功、旧基线 not_applied 冲突和服务器正文保留断言。
// POS: workspace revision HTTP 端到端回归；不运行 Agent runtime。
package workspace_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestWorkspaceRevisionConflictPreservesCommittedContent(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	filePath := "USER.md"
	loaded := workspaceFileRequest(t, server.Router(), http.MethodGet, cfg.DefaultAgentID, filePath, nil)
	if loaded.Revision == "" {
		t.Fatal("文件读取未返回 revision")
	}

	firstBody := map[string]any{
		"path":              filePath,
		"content":           "concurrent writer\n",
		"expected_revision": loaded.Revision,
	}
	first := workspaceFileRequest(t, server.Router(), http.MethodPut, cfg.DefaultAgentID, filePath, firstBody)
	if first.Content != "concurrent writer\n" || first.Revision == loaded.Revision {
		t.Fatalf("首次条件写入未推进 revision: %+v", first)
	}

	staleBody := map[string]any{
		"path":              filePath,
		"content":           "stale draft\n",
		"expected_revision": loaded.Revision,
	}
	recorder := serveWorkspaceFileRequest(
		t,
		server.Router(),
		http.MethodPut,
		cfg.DefaultAgentID,
		filePath,
		staleBody,
	)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("旧 revision 状态码 = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var failure struct {
		Data struct {
			Failure struct {
				Code   string `json:"code"`
				Effect string `json:"effect"`
			} `json:"failure"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil {
		t.Fatalf("解析冲突响应失败: %v", err)
	}
	if failure.Data.Failure.Code != "workspace.file_revision_conflict" ||
		failure.Data.Failure.Effect != "not_applied" {
		t.Fatalf("冲突 FailureCore = %+v", failure.Data.Failure)
	}

	current := workspaceFileRequest(t, server.Router(), http.MethodGet, cfg.DefaultAgentID, filePath, nil)
	if current.Content != first.Content || current.Revision != first.Revision {
		t.Fatalf("冲突覆盖了服务器正文: current=%+v first=%+v", current, first)
	}
}

type workspaceFileResponse struct {
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

func workspaceFileRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	agentID string,
	filePath string,
	body map[string]any,
) workspaceFileResponse {
	t.Helper()
	recorder := serveWorkspaceFileRequest(t, handler, method, agentID, filePath, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("workspace %s 状态码 = %d, body=%s", method, recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data workspaceFileResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析 workspace %s 响应失败: %v", method, err)
	}
	return payload.Data
}

func serveWorkspaceFileRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	agentID string,
	filePath string,
	body map[string]any,
) *httptest.ResponseRecorder {
	t.Helper()
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化 workspace 请求失败: %v", err)
		}
		payload = bytes.NewReader(encoded)
	}
	target := fmt.Sprintf(
		"/nexus/v1/agents/%s/workspace/file?path=%s",
		url.PathEscape(agentID),
		url.QueryEscape(filePath),
	)
	request := httptest.NewRequest(method, target, payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
