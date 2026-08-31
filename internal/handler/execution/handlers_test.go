// INPUT: WorkGraph editor Apply 的既有 owner、editor_id、revision 与失败结果。
// OUTPUT: 成功链路兼容性、原身份透传和 FailureCore 写入边界回归。
// POS: Execution HTTP 边界的低风险失败协议样板测试。
package execution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type applyWorkflowStub struct {
	applyErr     error
	applyResult  *protocol.WorkGraphWorkflowPreview
	applyOwner   string
	applyRequest protocol.ApplyWorkGraphWorkflowEditorRequest
	applyCalls   int
}

func (stub *applyWorkflowStub) PreviewFromExecution(
	context.Context,
	string,
	protocol.PreviewWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflowPreview, error) {
	return nil, errors.New("unexpected PreviewFromExecution call")
}

func (stub *applyWorkflowStub) PreviewSavedWorkflow(
	context.Context,
	string,
	string,
	string,
) (*protocol.WorkGraphWorkflowPreview, error) {
	return nil, errors.New("unexpected PreviewSavedWorkflow call")
}

func (stub *applyWorkflowStub) List(context.Context, string) ([]protocol.WorkGraphWorkflow, error) {
	return nil, errors.New("unexpected List call")
}

func (stub *applyWorkflowStub) ListLocalized(
	context.Context,
	string,
	string,
) ([]protocol.WorkGraphWorkflow, error) {
	return nil, errors.New("unexpected ListLocalized call")
}

func (stub *applyWorkflowStub) Delete(context.Context, string, string) (bool, error) {
	return false, errors.New("unexpected Delete call")
}

func (stub *applyWorkflowStub) StartMetadataEditor(
	context.Context,
	string,
	protocol.StartWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	return nil, errors.New("unexpected StartMetadataEditor call")
}

func (stub *applyWorkflowStub) GetMetadataEditor(
	string,
	protocol.GetWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	return nil, errors.New("unexpected GetMetadataEditor call")
}

func (stub *applyWorkflowStub) ApplyMetadataEditor(
	ownerUserID string,
	request protocol.ApplyWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowPreview, error) {
	stub.applyCalls++
	stub.applyOwner = ownerUserID
	stub.applyRequest = request
	return stub.applyResult, stub.applyErr
}

func (stub *applyWorkflowStub) SelectMetadataEditorVersion(
	context.Context,
	string,
	protocol.SelectWorkGraphWorkflowEditorVersionRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	return nil, errors.New("unexpected SelectMetadataEditorVersion call")
}

func (stub *applyWorkflowStub) CloseMetadataEditor(
	context.Context,
	string,
	string,
	string,
) (bool, error) {
	return false, errors.New("unexpected CloseMetadataEditor call")
}

func TestApplyWorkGraphWorkflowEditorKeepsExistingIdentityAndSuccessEnvelope(t *testing.T) {
	stub := &applyWorkflowStub{applyResult: &protocol.WorkGraphWorkflowPreview{PreviewID: "preview-a"}}
	router := newApplyWorkflowTestRouter(stub)

	for range 2 {
		request := httptest.NewRequest(
			http.MethodPost,
			"/workgraph/editors/editor-a/apply",
			strings.NewReader(`{"source_session_key":"session-a","editor_id":"ignored-body-id","revision":7}`),
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Request-ID", "same-http-attempt-label")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("Apply success status changed: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode Apply success: %v", err)
		}
		if payload["code"] != "0000" || payload["message"] != "success" || payload["success"] != true {
			t.Fatalf("Apply success envelope changed: %#v", payload)
		}
	}

	if stub.applyCalls != 2 {
		t.Fatalf("HTTP diagnostic ID must not deduplicate Apply: calls=%d", stub.applyCalls)
	}
	if stub.applyOwner != authsvc.SystemUserID ||
		stub.applyRequest.SourceSessionKey != "session-a" ||
		stub.applyRequest.EditorID != "editor-a" ||
		stub.applyRequest.Revision != 7 {
		t.Fatalf("existing Apply identity changed: owner=%q request=%#v", stub.applyOwner, stub.applyRequest)
	}
}

func TestApplyWorkGraphWorkflowEditorMapsOnlyProvenFailureEffects(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       string
		effect     protocol.FailureEffect
		action     string
		wantDetail string
	}{
		{
			name: "revision conflict", err: workgraphworkflowsvc.ErrRevisionConflict,
			status: http.StatusUnprocessableEntity, code: "workgraph.revision_conflict",
			effect: protocol.FailureEffectNotApplied, action: "workgraph.refresh_editor",
			wantDetail: "工作图编辑请求无效或版本已变化",
		},
		{
			name: "editor not found", err: workgraphworkflowsvc.ErrNotFound,
			status: http.StatusNotFound, code: "workgraph.editor_not_found",
			effect: protocol.FailureEffectNotApplied, action: "workgraph.reopen_editor",
			wantDetail: "工作图编辑会话不存在或已过期",
		},
		{
			name: "invalid request", err: workgraphworkflowsvc.ErrInvalidInput,
			status: http.StatusUnprocessableEntity, code: "workgraph.editor_invalid_request",
			effect:     protocol.FailureEffectNotApplied,
			wantDetail: "工作图编辑请求无效或版本已变化",
		},
		{
			name: "unclassified failure", err: errors.New("sqlite secret"),
			status: http.StatusInternalServerError, code: "workgraph.editor_apply_failed",
			effect:     protocol.FailureEffectUnknown,
			wantDetail: "工作图编辑失败",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &applyWorkflowStub{applyErr: test.err}
			router := newApplyWorkflowTestRouter(stub)
			request := httptest.NewRequest(
				http.MethodPost,
				"/workgraph/editors/editor-a/apply",
				strings.NewReader(`{"source_session_key":"session-a","revision":7}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-ID", "workgraph-http-attempt")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.status, recorder.Body.String())
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
				t.Fatalf("decode Apply failure: %v", err)
			}
			if payload.Code != strconv.Itoa(test.status) {
				t.Fatalf("legacy status code=%q want=%q", payload.Code, strconv.Itoa(test.status))
			}
			if payload.Message != "failed" || payload.Success || payload.Data.Detail != test.wantDetail {
				t.Fatalf("failure envelope or safe detail changed: %#v", payload)
			}
			if payload.Data.RequestID != "workgraph-http-attempt" ||
				payload.Data.Failure.TransportRequestID != "workgraph-http-attempt" ||
				payload.Data.Failure.Code != test.code ||
				payload.Data.Failure.Effect != test.effect {
				t.Fatalf("FailureCore mismatch: %#v", payload.Data)
			}
			if test.action == "" {
				if payload.Data.Failure.Resolution != nil {
					t.Fatalf("unexpected recovery action: %#v", payload.Data.Failure.Resolution)
				}
			} else if payload.Data.Failure.Resolution == nil ||
				payload.Data.Failure.Resolution.Action != test.action {
				t.Fatalf("recovery action mismatch: %#v", payload.Data.Failure.Resolution)
			}
			if strings.Contains(recorder.Body.String(), "sqlite secret") {
				t.Fatalf("internal cause leaked: %s", recorder.Body.String())
			}
		})
	}
}

func TestApplyWorkGraphWorkflowEditorUsesStructuredFailureBeforeServiceCall(t *testing.T) {
	stub := &applyWorkflowStub{}
	router := newApplyWorkflowTestRouter(stub)
	request := httptest.NewRequest(
		http.MethodPost,
		"/workgraph/editors/editor-a/apply",
		strings.NewReader(`{"source_session_key":`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "invalid-json-attempt")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.applyCalls != 0 {
		t.Fatalf("invalid JSON must fail before service call: calls=%d", stub.applyCalls)
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
		t.Fatalf("decode invalid JSON failure: %v", err)
	}
	if payload.Code != "400" || payload.Message != "failed" || payload.Success ||
		payload.Data.Detail != "请求参数错误" ||
		payload.Data.RequestID != "invalid-json-attempt" ||
		payload.Data.Failure.Code != "workgraph.editor_invalid_request" ||
		payload.Data.Failure.Category != protocol.FailureCategoryValidation ||
		payload.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("invalid JSON FailureCore mismatch: %#v", payload)
	}
	if strings.Contains(recorder.Body.String(), "unexpected end") {
		t.Fatalf("JSON decoder detail leaked: %s", recorder.Body.String())
	}
}

func newApplyWorkflowTestRouter(stub *applyWorkflowStub) http.Handler {
	api := handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := New(api, nil, stub)
	router := chi.NewRouter()
	router.Use(handlershared.RequestContextMiddleware(api.BaseLogger()))
	router.Post("/workgraph/editors/{editor_id}/apply", handler.HandleApplyWorkGraphWorkflowEditor)
	return router
}
