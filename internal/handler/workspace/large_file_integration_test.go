// INPUT: 超过整文件与内置预览上限的真实 workspace 文件。
// OUTPUT: 正文读取/非流式预览被拒绝，下载与 PDF Range 仍保持分段响应。
// POS: 大文件 HTTP 内存边界回归；不启动 Agent runtime。
package workspace_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func TestWorkspaceLargeFilesUseBoundedTransferPaths(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	workspacePath := agentsvc.ResolveWorkspacePath(cfg, authctx.SystemUserID, cfg.DefaultAgentID)
	if err = os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 失败: %v", err)
	}
	for _, fileName := range []string{"large.csv", "large.pdf"} {
		filePath := filepath.Join(workspacePath, fileName)
		if err = os.WriteFile(filePath, []byte("large file\n"), 0o644); err != nil {
			t.Fatalf("创建大文件失败: %v", err)
		}
		if err = os.Truncate(filePath, 15*1024*1024+1); err != nil {
			t.Fatalf("扩展大文件失败: %v", err)
		}
	}

	whole := serveWorkspaceFileRequest(
		t,
		server.Router(),
		http.MethodGet,
		cfg.DefaultAgentID,
		"large.csv",
		nil,
	)
	assertWorkspaceFailure(t, whole, http.StatusRequestEntityTooLarge, "workspace.file_too_large")

	inline := serveWorkspaceDownloadRequest(t, server.Router(), cfg.DefaultAgentID, "large.csv", "inline")
	assertWorkspaceFailure(t, inline, http.StatusRequestEntityTooLarge, "workspace.inline_preview_too_large")

	for _, testCase := range []struct {
		disposition string
		fileName    string
	}{
		{disposition: "attachment", fileName: "large.csv"},
		{disposition: "inline", fileName: "large.pdf"},
	} {
		recorder := serveWorkspaceDownloadRequest(
			t,
			server.Router(),
			cfg.DefaultAgentID,
			testCase.fileName,
			testCase.disposition,
		)
		if recorder.Code != http.StatusPartialContent || recorder.Body.Len() != 1024 {
			t.Fatalf(
				"%s %s Range 响应 = %d/%d, body=%s",
				testCase.disposition,
				testCase.fileName,
				recorder.Code,
				recorder.Body.Len(),
				recorder.Body.String(),
			)
		}
		if got := recorder.Header().Get("Content-Range"); got == "" {
			t.Fatalf("%s %s 缺少 Content-Range", testCase.disposition, testCase.fileName)
		}
	}
}

func serveWorkspaceDownloadRequest(
	t *testing.T,
	handler http.Handler,
	agentID string,
	filePath string,
	disposition string,
) *httptest.ResponseRecorder {
	t.Helper()
	target := fmt.Sprintf(
		"/nexus/v1/agents/%s/workspace/download?path=%s&disposition=%s",
		url.PathEscape(agentID),
		url.QueryEscape(filePath),
		url.QueryEscape(disposition),
	)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Range", "bytes=0-1023")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func assertWorkspaceFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("状态码 = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Failure struct {
				Code string `json:"code"`
			} `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析失败响应: %v", err)
	}
	if payload.Data.Failure.Code != wantCode {
		t.Fatalf("FailureCore code = %q, want %q", payload.Data.Failure.Code, wantCode)
	}
}
