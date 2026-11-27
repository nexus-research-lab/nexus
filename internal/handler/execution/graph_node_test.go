// INPUT: graph-node 读取的 session/execution/node 身份与服务端节点详情结果。
// OUTPUT: 参数校验、认证 owner 透传、节点不存在 404 与成功信封回归。
// POS: Execution HTTP 边界的按需节点详情测试；骨架/详情投影本身由 orchestration 与 storage 测试覆盖。
package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

type graphNodeViewerStub struct {
	executionViewer
	detail                          *protocol.ExecutionGraphNodeDetailView
	owner, session, execution, node string
	calls                           int
}

func (s *graphNodeViewerStub) GetGraphNodeDetail(
	_ context.Context,
	owner, session, execution, node string,
) (*protocol.ExecutionGraphNodeDetailView, error) {
	s.calls++
	s.owner, s.session, s.execution, s.node = owner, session, execution, node
	return s.detail, nil
}

func TestGraphNodeDetailHTTPBoundary(t *testing.T) {
	stub := &graphNodeViewerStub{}
	router := chi.NewRouter()
	router.Get("/executions/{execution_id}/graph-node", New(handlershared.NewAPI(nil), stub).HandleGetExecutionGraphNode)
	get := func(query string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/executions/execution/graph-node?"+query, nil))
		return recorder
	}

	for _, query := range []string{"node_id=lead", "session_key=session", "session_key=%20&node_id=lead"} {
		if recorder := get(query); recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("missing identity must be rejected: query=%q status=%d", query, recorder.Code)
		}
	}
	if stub.calls != 0 {
		t.Fatalf("invalid requests reached the service: %d", stub.calls)
	}

	if recorder := get("session_key=session&node_id=missing"); recorder.Code != http.StatusNotFound {
		t.Fatalf("missing node status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	stub.detail = &protocol.ExecutionGraphNodeDetailView{
		ExecutionID: "execution",
		Node:        protocol.ExecutionGraphNodeView{ID: "lead", ErrorSummary: "Upstream timed out"},
		Children:    []protocol.ExecutionGraphNodeView{{ID: "read", ParentNodeID: "lead"}},
	}
	recorder := get("session_key=%20session%20&node_id=%20lead%20")
	if recorder.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.owner != authsvc.SystemUserID || stub.session != "session" || stub.execution != "execution" || stub.node != "lead" {
		t.Fatalf("identity not trimmed or owner not from auth context: %+v", stub)
	}
	var payload struct {
		Data protocol.ExecutionGraphNodeDetailView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Node.ErrorSummary != "Upstream timed out" || len(payload.Data.Children) != 1 {
		t.Fatalf("detail envelope mismatch: %+v", payload.Data)
	}
}
