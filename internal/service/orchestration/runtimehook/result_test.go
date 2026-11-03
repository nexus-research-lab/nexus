package runtimehook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type resultRecorder struct {
	calls  int
	result protocol.ExecutionResultDeclaration
}

func (r *resultRecorder) AcceptRuntimeResult(_ context.Context, _ orchestration.ActorContext, result protocol.ExecutionResultDeclaration) (orchestration.RuntimeResultOutcome, error) {
	r.calls++
	r.result = result
	return orchestration.RuntimeResultOutcome{State: "applied"}, nil
}

func TestResultHookRequiresExplicitStrictBoundDeclaration(t *testing.T) {
	actor := orchestration.ActorContext{OwnerUserID: "owner", AgentID: "worker", AgentRoundID: "physical-round", RuntimeRoundID: "root-round", WorkBinding: &protocol.ExecutionWorkBinding{ExecutionID: "execution", PlanID: "plan", WorkItemID: "work", SpecID: "spec", AssignmentID: "assignment", AttemptID: "attempt"}}
	declaration := protocol.ExecutionResultDeclaration{Version: 1, RoundID: actor.AgentRoundID, Delivery: &protocol.ExecutionDeliveryResult{ResultSummary: " Evidence delivered ", Evidence: []string{" checked "}}}
	encoded, _ := json.Marshal(declaration)
	valid := "Delivered the report.\n" + resultMarker + string(encoded) + "\n-->"
	for name, text := range map[string]string{
		"plain_completion": "Everything is done and accepted.",
		"runtime_finished": "finished",
		"quoted_schema":    "```json\n" + string(encoded) + "\n```",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := &resultRecorder{}
			output, err := ResultCallback(recorder, actor, nil)(context.Background(), sdkhook.Input{LastAssistantMessage: text}, "")
			if err != nil || output.Decision != "" || recorder.calls != 0 {
				t.Fatalf("inferred result: %+v %v", output, err)
			}
		})
	}
	for name, text := range map[string]string{
		"unknown_field":    strings.Replace(valid, `"version":1`, `"version":1,"execution_id":"other"`, 1),
		"duplicate_key":    strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1),
		"nested_duplicate": strings.Replace(valid, `"result_summary":`, `"result_summary":"first","result_summary":`, 1),
		"stale_round":      strings.Replace(valid, "physical-round", "old-round", 1),
		"trailing_text":    valid + " not final",
		"multiple":         valid + valid,
		"quoted_comment":   "```html\n" + valid,
		"inline_comment":   "Example: " + resultMarker + string(encoded) + "\n-->",
		"case_alias":       strings.Replace(valid, `"version":1`, `"version":1,"Version":1`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := &resultRecorder{}
			output, err := ResultCallback(recorder, actor, nil)(context.Background(), sdkhook.Input{LastAssistantMessage: text}, "")
			if err != nil || output.Decision != "block" || recorder.calls != 0 {
				t.Fatalf("bad declaration accepted: %+v %v", output, err)
			}
		})
	}
	recorder := &resultRecorder{}
	callback := ResultCallback(recorder, actor, nil)
	if output, err := callback(context.Background(), sdkhook.Input{LastAssistantMessage: valid}, ""); err != nil || output.Decision != "" || recorder.calls != 1 || recorder.result.Delivery.ResultSummary != "Evidence delivered" {
		t.Fatalf("valid=%+v %v %+v", output, err, recorder)
	}
	for _, input := range []sdkhook.Input{{LastAssistantMessage: valid, IsInterrupt: true}, {LastAssistantMessage: valid, AgentID: "child"}, {LastAssistantMessage: valid, Error: "runtime failed"}} {
		if _, err := callback(context.Background(), input, ""); err != nil {
			t.Fatal(err)
		}
	}
	if recorder.calls != 1 {
		t.Fatal("failed/child runtime declared a parent result")
	}
	if !strings.Contains(ResultContract(actor), "round_id") || ResultContract(orchestration.ActorContext{}) != "" {
		t.Fatal("contract leaked to an unbound round")
	}
}
