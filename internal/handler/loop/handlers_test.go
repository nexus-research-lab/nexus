package loop

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	loopsvc "github.com/nexus-research-lab/nexus/internal/service/loops"
)

func TestHandleGetLoopDetailAddsReadOnlyFailureWithoutChanging404(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := New(api, loopsvc.NewService())
	router := chi.NewRouter()
	router.Use(handlershared.RequestContextMiddleware(api.BaseLogger()))
	router.Get("/capability/loops/{slug}", handler.HandleGetLoopDetail)

	request := httptest.NewRequest(http.MethodGet, "/capability/loops/missing-loop", nil)
	request.Header.Set("X-Request-ID", "loop-http-attempt")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("loop 404 状态被改变: status=%d body=%s", recorder.Code, recorder.Body.String())
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
		t.Fatalf("解析 loop 404: %v", err)
	}
	if payload.Code != "404" || payload.Message != "failed" || payload.Success ||
		payload.Data.Detail != "资源不存在" {
		t.Fatalf("loop 旧 404 envelope 被改变: %#v", payload)
	}
	if payload.Data.RequestID != "loop-http-attempt" ||
		payload.Data.Failure.TransportRequestID != "loop-http-attempt" ||
		payload.Data.Failure.Code != "loop.not_found" ||
		payload.Data.Failure.Effect != protocol.FailureEffectNotApplicable {
		t.Fatalf("loop FailureCore 不正确: %#v", payload.Data)
	}
}

func TestHandleGetLoopDetailKeepsSuccessEnvelope(t *testing.T) {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := New(api, loopsvc.NewService())
	router := chi.NewRouter()
	router.Get("/capability/loops/{slug}", handler.HandleGetLoopDetail)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/capability/loops/test-until-green", nil),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("loop success 状态被改变: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析 loop success: %v", err)
	}
	if payload["code"] != "0000" || payload["message"] != "success" || payload["success"] != true {
		t.Fatalf("loop success envelope 被改变: %#v", payload)
	}
}
