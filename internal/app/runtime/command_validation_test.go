package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

func TestAutomationMalformedCommandRejectedBeforeServiceAndConfirmation(t *testing.T) {
	// No database or permission context: invalid model input must not reach either.
	for _, test := range []struct {
		request command.Request
		want    string
	}{
		{command.Request{Action: "apply", Operation: "create", Input: map[string]any{"name": "test", "instruction": "test", "schedule": map[string]any{"kind": "interval", "interval_value": 0}}}, "$.schedule.interval_value"},
		{command.Request{Action: "apply", Operation: "delete", Input: map[string]any{"job_id": "job", "instruction": "unrelated"}}, "$.instruction"},
		{command.Request{Action: "invoke", Operation: "get", Input: map[string]any{"job_id": "job"}}, "action=inspect"},
	} {
		_, err := handleAutomationCommand(context.Background(), &automationsvc.Service{}, nil, command.Actor{SourceContextType: "agent"}, test.request)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %s, got %v", test.want, err)
		}
	}
}
