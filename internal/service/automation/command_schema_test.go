package automation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func TestAutomationExactContractAndRawInputAgree(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	contract, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "create")
	if err != nil || len(contract.Operations) != 1 {
		t.Fatalf("contract=%+v err=%v", contract, err)
	}
	schema := contract.Operations["create"].InputSchema
	valid := map[string]any{"name": "test", "instruction": "test", "schedule": map[string]any{"kind": "interval", "interval_value": 5}}
	if err := command.ValidateInput(schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		field string
		value any
	}{
		{"agent_id", "other"}, {"enabled", "true"}, {"context_mode", "unknown"},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 0}},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 5, "extra": true}},
	} {
		input := map[string]any{}
		for k, v := range valid {
			input[k] = v
		}
		input[test.field] = test.value
		if err := ValidateRuntimeCommandInput(fixture.ServerContext, "plan", "create", input); err == nil {
			t.Fatalf("accepted invalid %s=%v", test.field, test.value)
		}
	}
	if _, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "unknown"); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "list", nil); err == nil || !strings.Contains(err.Error(), "action=inspect") {
		t.Fatalf("query correction: %v", err)
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "create", valid); err == nil || !strings.Contains(err.Error(), "action=plan") {
		t.Fatalf("mutation correction: %v", err)
	}
}

func TestAutomationPlanViewCanBeReusedWithoutHiddenFields(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	input := automationdomain.AutomationCommandInput{Name: "test", Instruction: "test", Schedule: &automationdomain.AutomationCommandSchedule{Kind: "interval", IntervalValue: 60}}
	plan, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", input)
	if err != nil {
		t.Fatal(err)
	}
	view, err := RuntimeCommandPlanView(fixture.ServerContext, *plan)
	if err != nil {
		t.Fatal(err)
	}
	raw := view["input"].(map[string]any)
	if _, ok := raw["agent_id"]; ok {
		t.Fatal("hidden agent_id leaked into reusable input")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "apply", "create", raw); err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(raw)
	var reused automationdomain.AutomationCommandInput
	if err := json.Unmarshal(bytes, &reused); err != nil {
		t.Fatal(err)
	}
	replanned, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", reused)
	if err != nil {
		t.Fatal(err)
	}
	request := automationdomain.AutomationCommandRequest{RequestID: "schema-test-001", ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest}
	if err := ValidateRuntimeCommandPlan(request, *replanned); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"request_id", "expected_revision", "plan_digest", "stale_revision", "stale_digest"} {
		bad := request
		switch field {
		case "request_id":
			bad.RequestID = ""
		case "expected_revision":
			bad.ExpectedRevision = ""
		case "plan_digest":
			bad.PlanDigest = ""
		case "stale_revision":
			bad.ExpectedRevision = "old"
		case "stale_digest":
			bad.PlanDigest = "old"
		}
		if err := ValidateRuntimeCommandPlan(bad, *replanned); err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
}
