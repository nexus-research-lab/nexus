package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	legacyGatewayTimeout := httptest.NewRecorder()
	api.WriteFailure(legacyGatewayTimeout, http.StatusGatewayTimeout, "upstream timeout")
	if !strings.Contains(legacyGatewayTimeout.Body.String(), "服务内部错误") {
		t.Fatalf("旧 WriteFailure 的 504 文案不应被新协议改写: %s", legacyGatewayTimeout.Body.String())
	}
}

func TestWriteFailureDoesNotCopyRawDetailIntoSharedLogs(t *testing.T) {
	var logOutput bytes.Buffer
	api := NewAPI(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	recorder := httptest.NewRecorder()

	api.WriteFailure(recorder, http.StatusInternalServerError, "provider-token private/path sql statement")

	logs := logOutput.String()
	if strings.Contains(logs, "provider-token") || strings.Contains(logs, "private/path") {
		t.Fatalf("旧失败日志泄露了未经分类的 detail: %s", logs)
	}
	if !strings.Contains(logs, `"has_detail":true`) {
		t.Fatalf("旧失败日志应保留非敏感的 detail 存在性: %s", logs)
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
	if payload.Data.Detail != "工作图已被其他操作更新" {
		t.Fatalf("显式安全 detail 不应被压成通用 HTTP 文案: %q", payload.Data.Detail)
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

func TestWriteErrorDoesNotCopyClientDetailIntoLogs(t *testing.T) {
	var logOutput bytes.Buffer
	api := NewAPI(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, nil, http.StatusBadRequest, FailureSpec{
		Code:     "agent.create_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "资源名称包含用户私密内容",
	})

	logs := logOutput.String()
	if strings.Contains(logs, "资源名称包含用户私密内容") {
		t.Fatalf("用户文案不得复制到结构化日志: %s", logs)
	}
	if !strings.Contains(logs, `"has_client_detail":true`) ||
		!strings.Contains(logs, `"failure_code":"agent.create_request_invalid"`) {
		t.Fatalf("日志应保留非敏感失败事实: %s", logs)
	}
}

func TestWriteErrorDoesNotCopyRawCauseIntoSharedLogs(t *testing.T) {
	var logOutput bytes.Buffer
	api := NewAPI(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, nil, http.StatusInternalServerError, FailureSpec{
		Code:     "provider.connection_failed",
		Category: protocol.FailureCategoryInternal,
		Effect:   protocol.FailureEffectUnknown,
		Cause:    errors.New("provider-token private/path sql statement"),
	})

	logs := logOutput.String()
	if strings.Contains(logs, "provider-token") || strings.Contains(logs, "private/path") {
		t.Fatalf("共享失败日志泄露了未经脱敏的 cause: %s", logs)
	}
	if !strings.Contains(logs, `"has_cause":true`) || !strings.Contains(logs, `"cause_type"`) {
		t.Fatalf("共享失败日志应保留 cause 存在性和类型: %s", logs)
	}
}

func TestWriteErrorRejectsUnstableCode(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusConflict, FailureSpec{
		Code:     "https://internal.example/secret",
		Category: protocol.FailureCategoryConflict,
		Effect:   protocol.FailureEffectUnknown,
		Detail:   "请求结果尚未确认",
	})

	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"common.request_failed"`) {
		t.Fatalf("非稳定 code 没有安全降级: %s", body)
	}
	if strings.Contains(body, "internal.example") {
		t.Fatalf("非稳定 code 泄露到 wire: %s", body)
	}
}

func TestNormalizeFailureSemanticKeyRequiresDomainReasonShape(t *testing.T) {
	tests := []struct {
		value string
		want  string
		valid bool
	}{
		{value: " workgraph.refresh_editor ", want: "workgraph.refresh_editor", valid: true},
		{value: "automation.delivery_retry_2", want: "automation.delivery_retry_2", valid: true},
		{value: "", want: "", valid: true},
		{value: "request_failed", valid: false},
		{value: "WorkGraph.refresh", valid: false},
		{value: "workgraph..refresh", valid: false},
		{value: "workgraph.-refresh", valid: false},
		{value: "workgraph.refresh/editor", valid: false},
		{value: strings.Repeat("a", maxFailureSemanticKeyLength+1) + ".reason", valid: false},
	}
	for _, test := range tests {
		got, valid := normalizeFailureSemanticKey(test.value)
		if got != test.want || valid != test.valid {
			t.Fatalf("normalizeFailureSemanticKey(%q)=(%q,%t) want=(%q,%t)", test.value, got, valid, test.want, test.valid)
		}
	}
}

func TestWriteErrorDegradesUnknownEffectWithoutPublishingIt(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Code:   "automation.task_update_failed",
		Effect: protocol.FailureEffect("partially_maybe_saved"),
	})

	body := recorder.Body.String()
	if !strings.Contains(body, `"effect":"unknown"`) || strings.Contains(body, "partially_maybe_saved") {
		t.Fatalf("未知 effect 没有安全降级: %s", body)
	}
}

func TestWriteErrorDegradesUnknownCategoryFromHTTPStatus(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusConflict, FailureSpec{
		Code:     "automation.task_update_failed",
		Category: protocol.FailureCategory("provider_secret_category"),
		Effect:   protocol.FailureEffectNotApplied,
	})

	body := recorder.Body.String()
	if !strings.Contains(body, `"category":"conflict"`) || strings.Contains(body, "provider_secret_category") {
		t.Fatalf("未知 category 没有按 HTTP 语义安全降级: %s", body)
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

func TestWriteErrorKeepsDeadlineDistinctFromClientCancellation(t *testing.T) {
	api := newFailureTestAPI()
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, request, http.StatusInternalServerError, FailureSpec{
		Code:   "automation.task_update_failed",
		Effect: protocol.FailureEffectUnknown,
		Cause:  context.DeadlineExceeded,
	})

	if recorder.Code != http.StatusGatewayTimeout ||
		!strings.Contains(recorder.Body.String(), `"category":"timeout"`) ||
		!strings.Contains(recorder.Body.String(), `"detail":"请求超时"`) ||
		strings.Contains(recorder.Body.String(), `"category":"canceled"`) {
		t.Fatalf("截止时间不得投影为客户端取消: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestFailureEffectBeforeHandlerUsesOnlyRequestSemantics(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		request := httptest.NewRequest(method, "/resource", nil)
		if got := failureEffectBeforeHandler(request); got != protocol.FailureEffectNotApplicable {
			t.Fatalf("%s pre-handler effect=%q", method, got)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		request := httptest.NewRequest(method, "/resource", nil)
		if got := failureEffectBeforeHandler(request); got != protocol.FailureEffectNotApplied {
			t.Fatalf("%s pre-handler effect=%q", method, got)
		}
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

func TestDiagnosticRequestIDRejectsUnsafeOrOversizedValuesWithoutRejectingRequest(t *testing.T) {
	for _, raw := range []string{
		"request id with spaces",
		"request/id",
		strings.Repeat("a", maxDiagnosticRequestIDLength+1),
	} {
		if got := normalizeDiagnosticRequestID(raw); got != "" {
			t.Fatalf("不安全 request ID %q 被接受为 %q", raw, got)
		}
	}
	if got := normalizeDiagnosticRequestID(" trace_01:attempt-2 "); got != "trace_01:attempt-2" {
		t.Fatalf("合法 request ID 被改变: %q", got)
	}

	api := newFailureTestAPI()
	called := false
	handler := RequestContextMiddleware(api.BaseLogger())(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			called = true
			writer.WriteHeader(http.StatusNoContent)
		},
	))
	request := httptest.NewRequest(http.MethodPost, "/scheduled-tasks", nil)
	request.Header.Set("X-Request-ID", strings.Repeat("a", maxDiagnosticRequestIDLength+1))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("无效诊断 ID 不得拒绝请求: called=%v status=%d", called, recorder.Code)
	}
	generated := recorder.Header().Get("X-Request-ID")
	if generated == "" || generated == request.Header.Get("X-Request-ID") {
		t.Fatalf("无效诊断 ID 应静默替换: %q", generated)
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
