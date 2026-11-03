package execution

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type recoveryHandlerStub struct {
	executionViewer
	owner, execution string
	input            protocol.RequestExecutionRecovery
	requests, reads  int
	err              error
}

func (s *recoveryHandlerStub) RequestWorkRecovery(_ context.Context, owner, execution string, input protocol.RequestExecutionRecovery) (*protocol.ExecutionRecoveryReceipt, error) {
	s.requests++
	s.owner, s.execution, s.input = owner, execution, input
	return &protocol.ExecutionRecoveryReceipt{ExecutionRecoveryTarget: protocol.ExecutionRecoveryTarget{ExecutionID: execution, WorkItemID: input.WorkItemID, AttemptID: input.AttemptID}, RequestID: input.RequestID, Queued: true}, s.err
}
func (s *recoveryHandlerStub) GetWorkRecoveryReceipt(_ context.Context, owner, execution, session, request string) (*protocol.ExecutionRecoveryReceipt, error) {
	s.reads++
	s.owner, s.execution, s.input = owner, execution, protocol.RequestExecutionRecovery{SessionKey: session, RequestID: request}
	return &protocol.ExecutionRecoveryReceipt{ExecutionRecoveryTarget: protocol.ExecutionRecoveryTarget{ExecutionID: execution}, RequestID: request, Queued: false}, s.err
}

func TestWorkRecoveryHTTPIntakeAndReadOnlyReconciliation(t *testing.T) {
	stub := &recoveryHandlerStub{}
	handler := New(handlershared.NewAPI(nil), stub)
	router := chi.NewRouter()
	router.Post("/executions/{execution_id}/recover", handler.HandleRequestWorkRecovery)
	router.Get("/executions/{execution_id}/recovery-receipt", handler.HandleGetWorkRecoveryReceipt)
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/executions/execution/recover", strings.NewReader(body)))
		return w
	}
	w := post(`{"session_key":" session ","request_id":" request ","work_item_id":" work ","attempt_id":" attempt ","owner_user_id":"forged"}`)
	if w.Code != http.StatusOK || stub.owner != authsvc.SystemUserID || stub.execution != "execution" || stub.input != (protocol.RequestExecutionRecovery{SessionKey: "session", RequestID: "request", WorkItemID: "work", AttemptID: "attempt"}) {
		t.Fatalf("intake identity: %+v status=%d", stub, w.Code)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/executions/execution/recovery-receipt?session_key=%20session%20&request_id=%20request%20", nil))
	var response struct {
		Data protocol.ExecutionRecoveryReceipt `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || stub.requests != 1 || stub.reads != 1 || response.Data.Queued || response.Data.RequestID != "request" || stub.input.SessionKey != "session" {
		t.Fatalf("receipt read enqueued or lost identity: %+v %s", stub, w.Body)
	}
	if w := post(`{`); w.Code != http.StatusBadRequest || stub.requests != 1 {
		t.Fatal("malformed request reached service")
	}
	stub.err = &svc.DomainError{Code: svc.ErrorCodeInvalidInput, Message: "private target"}
	if w := post(`{}`); w.Code != http.StatusConflict || strings.Contains(w.Body.String(), "private target") {
		t.Fatal("domain error was not safely rejected")
	}
	stub.err = errors.New("private transport detail")
	if w := post(`{}`); w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "private transport detail") {
		t.Fatal("unknown outcome was not bounded")
	}
}
