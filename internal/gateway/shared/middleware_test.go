package shared

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareWritesRequestIDAndAccessLog(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte("ok"))
		})),
	)

	request := httptest.NewRequest(http.MethodPost, "/agent/v1/agents?limit=10", nil)
	request.RemoteAddr = "127.0.0.1:9000"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if requestID := recorder.Result().Header.Get("X-Request-ID"); requestID == "" {
		t.Fatal("响应头未写入 X-Request-ID")
	}

	output := buffer.String()
	if !strings.Contains(output, "\"msg\":\"HTTP 请求完成\"") {
		t.Fatalf("未写入 access log: %s", output)
	}
	if !strings.Contains(output, "\"status\":201") {
		t.Fatalf("access log 状态码不正确: %s", output)
	}
	if !strings.Contains(output, "\"request_id\"") {
		t.Fatalf("access log 缺少 request_id: %s", output)
	}
	if strings.Contains(output, "\"ok\"") {
		t.Fatalf("access log 不应包含响应体: %s", output)
	}
}

func TestRecoverMiddlewareReturnsInternalError(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	api := NewAPI(logger)

	handler := RequestContextMiddleware(logger)(
		AccessLogMiddleware()(
			RecoverMiddleware(api)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic("boom")
			})),
		),
	)

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/panic", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic 后状态码不正确: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "服务内部错误") {
		t.Fatalf("panic 后返回体不正确: %s", recorder.Body.String())
	}

	output := buffer.String()
	if !strings.Contains(output, "\"msg\":\"HTTP 请求 panic\"") {
		t.Fatalf("未记录 panic 日志: %s", output)
	}
}
