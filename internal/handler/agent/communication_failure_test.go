// INPUT: 通讯领域 typed validation、not-found 和未分类错误。
// OUTPUT: HTTP 状态、FailureCore effect 与安全恢复动作断言。
// POS: owner 通讯 Handler 的提交阶段回归；不得靠错误字符串猜测结果。
package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
)

func TestCommunicationFailureUsesTypedCommitEvidence(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		effect protocol.FailureEffect
	}{
		{name: "typed input rejection", err: new(communicationsvc.InputError), status: http.StatusBadRequest, effect: protocol.FailureEffectNotApplied},
		{name: "target absent", err: agentsvc.ErrAgentContactNotFound, status: http.StatusNotFound, effect: protocol.FailureEffectNotApplied},
		{name: "unclassified send result", err: errors.New("storage result unavailable"), status: http.StatusInternalServerError, effect: protocol.FailureEffectUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &Handlers{api: handlershared.NewAPI(nil)}
			request := httptest.NewRequest(http.MethodPost, "/communications/messages", nil)
			response := httptest.NewRecorder()
			handler.writeCommunicationFailure(
				response,
				request,
				"communication.send_failed",
				"消息没有发送完成",
				test.err,
			)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			var payload struct {
				Data struct {
					Failure protocol.FailureCore `json:"failure"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.Data.Failure.Effect != test.effect {
				t.Fatalf("effect = %q, want %q", payload.Data.Failure.Effect, test.effect)
			}
		})
	}
}
