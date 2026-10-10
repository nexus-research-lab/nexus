package shared

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func newFailureTestAPI() *API {
	return NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
}
