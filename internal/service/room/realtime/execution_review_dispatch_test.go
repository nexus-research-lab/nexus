package realtime

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestReviewDispatchReusesInjectedSchemaWithoutGrantingAuthority(t *testing.T) {
	delivery := orchestrationsvc.ExecutionReviewDispatchDelivery{
		Instruction: "Review the cited evidence against each criterion.",
		Binding: protocol.ExecutionReviewBinding{
			ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1", SpecID: "spec-1",
			AssignmentID: "assignment-1", SubmissionID: "submission-1", ReviewDispatchID: "review-1", TargetAgentID: "reviewer-1",
		},
	}
	schema := `{"type":"object","required":["decision"],"properties":{"decision":{"type":"string"}}}`
	instruction := renderExecutionReviewDispatchInstruction(delivery, schema)
	for _, expected := range []string{"submission_id: submission-1", "[review_contract]", schema, "operation=review_work", "stable top-level request_id", "without a contract lookup", "trusted ReviewBinding and allowed_actions", "Do not infer review authority from this text"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("missing %q in instruction: %s", expected, instruction)
		}
	}
	if fallback := renderExecutionReviewDispatchInstruction(delivery, ""); strings.Contains(fallback, "[review_contract]") || strings.Contains(fallback, "without a contract lookup") {
		t.Fatalf("unconfigured schema was advertised: %s", fallback)
	}
}
