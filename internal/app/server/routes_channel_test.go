// INPUT: 完整 Server 路由表与空的 Channel 登录会话存储。
// OUTPUT: 当前登录 GET 与既有启动 POST 在同一路径独立分发的回归证明。
// POS: app 路由组合测试；防止只读对账入口覆盖或重用启动写入口。
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestChannelCurrentLoginGETAndStartPOSTRemainSeparateRoutes(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	handlertest.CloseServer(t, server)

	path := cfg.APIPrefix + "/capability/channels/feishu/login"
	getRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(
		getRecorder,
		httptest.NewRequest(http.MethodGet, path, nil),
	)
	assertChannelRouteFailure(
		t,
		getRecorder,
		http.StatusNotFound,
		"channel.read_login_not_found",
		protocol.FailureEffectNotApplicable,
	)

	postRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(
		postRecorder,
		httptest.NewRequest(http.MethodPost, path, nil),
	)
	assertChannelRouteFailure(
		t,
		postRecorder,
		http.StatusConflict,
		"channel.start_login_config_required",
		protocol.FailureEffectNotApplied,
	)
}

func assertChannelRouteFailure(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantEffect protocol.FailureEffect,
) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if payload.Data.Failure.Code != wantCode || payload.Data.Failure.Effect != wantEffect {
		t.Fatalf("failure = %+v", payload.Data.Failure)
	}
}
