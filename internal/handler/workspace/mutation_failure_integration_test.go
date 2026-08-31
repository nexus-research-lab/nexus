// INPUT: 真实 workspace 修改路由，以及可在进入文件修改前证明的失败请求。
// OUTPUT: 兼容 HTTP envelope 中稳定的 not_applied FailureCore 断言。
// POS: workspace 修改失败协议集成回归；不把内部/传输失败伪装成未执行。
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

func TestWorkspaceMutationKnownRejectionsAreNotApplied(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	tests := []struct {
		body       string
		method     string
		path       string
		wantCode   string
		wantStatus int
	}{
		{
			body:       `{"path":`,
			method:     http.MethodPost,
			path:       "entry",
			wantCode:   "workspace.create_request_invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			body:       `{"path":"USER.md","entry_type":"file"}`,
			method:     http.MethodPost,
			path:       "entry",
			wantCode:   "workspace.create_invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			method:     http.MethodDelete,
			path:       "entry?path=" + url.QueryEscape("missing.txt"),
			wantCode:   "workspace.delete_not_found",
			wantStatus: http.StatusNotFound,
		},
		{
			method:     http.MethodPost,
			path:       "upload",
			wantCode:   "workspace.upload_request_invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.wantCode, func(t *testing.T) {
			target := fmt.Sprintf(
				"/nexus/v1/agents/%s/workspace/%s",
				url.PathEscape(cfg.DefaultAgentID),
				testCase.path,
			)
			request := httptest.NewRequest(
				testCase.method,
				target,
				bytes.NewBufferString(testCase.body),
			)
			if testCase.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("状态码 = %d, body=%s", recorder.Code, recorder.Body.String())
			}

			var payload struct {
				Data struct {
					Failure struct {
						Code   string `json:"code"`
						Effect string `json:"effect"`
					} `json:"failure"`
				} `json:"data"`
			}
			if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("解析失败响应: %v", err)
			}
			if payload.Data.Failure.Code != testCase.wantCode ||
				payload.Data.Failure.Effect != "not_applied" {
				t.Fatalf("FailureCore = %+v", payload.Data.Failure)
			}
		})
	}
}
