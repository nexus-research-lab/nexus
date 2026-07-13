package room

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"
)

func TestWriteConversationSubagentTaskErrorDistinguishesUnsupportedRuntime(t *testing.T) {
	handler := &Handlers{api: handlershared.NewAPI(nil)}
	recorder := httptest.NewRecorder()

	handler.writeConversationSubagentTaskError(recorder, sessionpkg.ErrSubagentOperationUnsupported)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "当前运行时不支持该操作") || strings.Contains(body, "已结束") {
		t.Fatalf("unsupported response = %s", body)
	}
}
