package shared

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestWriteFailureKeepsLegacyEnvelope(t *testing.T) {
	api := newFailureTestAPI()
	recorder := httptest.NewRecorder()

	api.WriteFailure(recorder, http.StatusConflict, "database conflict")

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析旧失败响应: %v", err)
	}
	if payload["code"] != "409" || payload["message"] != "failed" || payload["success"] != false {
		t.Fatalf("旧失败 envelope 被改变: %#v", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["detail"] != "请求冲突" {
		t.Fatalf("旧失败 detail 被改变: %#v", payload["data"])
	}
	if len(data) != 1 {
		t.Fatalf("旧 WriteFailure 不应自动增加新字段: %#v", data)
	}
}

func TestWriteFailureKeepsLegacyCancellationAndSanitization(t *testing.T) {
	api := newFailureTestAPI()

	canceled := httptest.NewRecorder()
	api.WriteFailure(canceled, http.StatusInternalServerError, "context canceled")
	if canceled.Code != 499 || !strings.Contains(canceled.Body.String(), "请求已取消") {
		t.Fatalf("旧取消请求投影被改变: status=%d body=%s", canceled.Code, canceled.Body.String())
	}

	internal := httptest.NewRecorder()
	api.WriteFailure(internal, http.StatusInternalServerError, "sqlite secret")
	if strings.Contains(internal.Body.String(), "sqlite secret") ||
		!strings.Contains(internal.Body.String(), "服务内部错误") {
		t.Fatalf("旧内部错误脱敏被改变: %s", internal.Body.String())
	}
}

func TestWriteErrorAddsOptionalFailureCoreWithoutChangingEnvelope(t *testing.T) {
	api := newFailureTestAPI()
	handler := RequestContextMiddleware(api.BaseLogger())(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			api.WriteError(writer, request, http.StatusConflict, FailureSpec{
				Code:     "workgraph.revision_conflict",
				Category: protocol.FailureCategoryConflict,
				Effect:   protocol.FailureEffectNotApplied,
				Detail:   "工作图已被其他操作更新",
				Resolution: &protocol.FailureResolution{
					Actor:  protocol.FailureRecoveryActorUser,
					Action: "workgraph.refresh_editor",
				},
			})
		},
	))

	request := httptest.NewRequest(http.MethodPost, "/workgraphs/one", nil)
	request.Header.Set("X-Request-ID", "http-attempt-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got != "http-attempt-1" {
		t.Fatalf("响应头 request ID 不一致: %q", got)
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Success bool   `json:"success"`
		Data    struct {
			Detail    string               `json:"detail"`
			RequestID string               `json:"request_id"`
			Failure   protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析结构化失败响应: %v", err)
	}
	if payload.Code != "409" || payload.Message != "failed" || payload.Success {
		t.Fatalf("结构化失败改变了旧 envelope: %#v", payload)
	}
	if payload.Data.RequestID != "http-attempt-1" ||
		payload.Data.Failure.TransportRequestID != "http-attempt-1" {
		t.Fatalf("诊断 request ID 未保持一致: %#v", payload.Data)
	}
	if payload.Data.Failure.Code != "workgraph.revision_conflict" ||
		payload.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("FailureCore 事实不正确: %#v", payload.Data.Failure)
	}
}

func TestWriteErrorDoesNotExposeCauseOrInventRequestID(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/workgraphs/one", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Cause: errors.New("sqlite secret path"),
	})

	body := recorder.Body.String()
	if strings.Contains(body, "sqlite secret path") {
		t.Fatalf("内部 Cause 泄露到响应: %s", body)
	}
	if strings.Contains(body, "transport_request_id") || strings.Contains(body, `"request_id"`) {
		t.Fatalf("缺少 middleware 时不应另造 request ID: %s", body)
	}
	if !strings.Contains(body, `"code":"common.request_failed"`) ||
		!strings.Contains(body, `"effect":"unknown"`) {
		t.Fatalf("空结构化事实没有安全回退: %s", body)
	}
}

func TestWriteErrorKeepsCancellationStatusAndCategoryConsistent(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodGet, "/runs", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Code:     "automation.run_history_unavailable",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectNotApplicable,
		Cause:    context.Canceled,
	})

	if recorder.Code != 499 ||
		!strings.Contains(recorder.Body.String(), `"category":"canceled"`) ||
		!strings.Contains(recorder.Body.String(), `"effect":"not_applicable"`) {
		t.Fatalf("取消投影不一致: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestWriteErrorOnlyPublishesRetryAfterForExplicitTransientStatus(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodGet, "/loops", nil)

	limited := httptest.NewRecorder()
	api.WriteError(limited, request, http.StatusTooManyRequests, FailureSpec{
		Code:       "provider.rate_limited",
		Category:   protocol.FailureCategoryRateLimited,
		Effect:     protocol.FailureEffectNotApplicable,
		RetryAfter: 1500 * time.Millisecond,
	})
	if limited.Header().Get("Retry-After") != "2" ||
		!strings.Contains(limited.Body.String(), `"retry_after_ms":1500`) {
		t.Fatalf("429 Retry-After 不正确: headers=%v body=%s", limited.Header(), limited.Body.String())
	}

	conflict := httptest.NewRecorder()
	api.WriteError(conflict, request, http.StatusConflict, FailureSpec{
		Code:       "workgraph.revision_conflict",
		Category:   protocol.FailureCategoryConflict,
		Effect:     protocol.FailureEffectNotApplied,
		RetryAfter: time.Minute,
	})
	if conflict.Header().Get("Retry-After") != "" ||
		strings.Contains(conflict.Body.String(), "retry_after_ms") {
		t.Fatalf("非瞬时状态不应发布 Retry-After: headers=%v body=%s", conflict.Header(), conflict.Body.String())
	}
}

func TestHTTPDiagnosticRequestIDNeverMakesPostIdempotent(t *testing.T) {
	api := newFailureTestAPI()
	calls := 0
	handler := RequestContextMiddleware(api.BaseLogger())(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			calls++
			writer.WriteHeader(http.StatusNoContent)
		},
	))

	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
		request.Header.Set("X-Request-ID", "same-http-diagnostic-id")
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	if calls != 2 {
		t.Fatalf("HTTP 诊断 ID 不得改变 POST 语义，调用次数=%d", calls)
	}
}

func TestRequestIDContextAccessorIsDiagnosticOnly(t *testing.T) {
	if got := requestID(context.Background()); got != "" {
		t.Fatalf("空 context 不应有 request ID: %q", got)
	}
	ctx := withRequestID(context.Background(), " request-one ")
	if got := requestID(ctx); got != "request-one" {
		t.Fatalf("request ID context 读取错误: %q", got)
	}
}

func TestRequestContextMiddlewareGeneratesOneConsistentDiagnosticID(t *testing.T) {
	api := newFailureTestAPI()
	captured := ""
	handler := RequestContextMiddleware(api.BaseLogger())(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			captured = requestID(request.Context())
			writer.WriteHeader(http.StatusNoContent)
		},
	))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/loops", nil))

	responseID := recorder.Header().Get("X-Request-ID")
	if captured == "" || responseID == "" || captured != responseID {
		t.Fatalf("middleware 诊断 ID 不一致: context=%q header=%q", captured, responseID)
	}
}

func newFailureTestAPI() *API {
	return NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
}
