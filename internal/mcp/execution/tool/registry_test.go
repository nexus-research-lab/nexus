package tool

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildAllExposesOnlySemanticExecutionTools(t *testing.T) {
	definitions := BuildAll(nil, contract.ServerContext{})
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	want := []string{
		"get_execution",
		"plan_execution",
		"abandon_execution",
		"assign_work",
		"submit_work",
		"review_work",
		"block_work",
		"resume_work",
		"take_over_work",
		"promote_execution_to_goal",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("tool names = %#v, want %#v", names, want)
	}
	if slices.Contains(names, "start_work") {
		t.Fatal("machine bookkeeping leaked into the model tool registry")
	}
}

func TestExecutionToolSchemasHideFencingAndIdempotency(t *testing.T) {
	for _, definition := range BuildAll(nil, contract.ServerContext{}) {
		encoded, err := json.Marshal(definition.InputSchema)
		if err != nil {
			t.Fatalf("%s schema: %v", definition.Name, err)
		}
		text := string(encoded)
		for _, forbidden := range []string{"snapshot_revision", "command_id"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s schema exposes %s: %s", definition.Name, forbidden, text)
			}
		}
		properties, ok := definition.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s properties = %#v", definition.Name, definition.InputSchema["properties"])
		}
		if maps.Keys(properties) == nil {
			t.Fatalf("%s has invalid properties", definition.Name)
		}
		required, ok := definition.InputSchema["required"].([]string)
		if !ok {
			t.Fatalf("%s required = %#v, want []string", definition.Name, definition.InputSchema["required"])
		}
		if required == nil {
			t.Fatalf("%s required must marshal as an array, not null", definition.Name)
		}
		if definition.InputSchema["additionalProperties"] != false {
			t.Fatalf("%s additionalProperties = %#v", definition.Name, definition.InputSchema["additionalProperties"])
		}
	}
}

func TestRoomAssignmentDescriptionMakesStructuredHandoffPrimary(t *testing.T) {
	definition := assignWork(nil, contract.ServerContext{})
	for _, required := range []string{"structured", "backend dispatches", "do not duplicate", "@ message"} {
		if !strings.Contains(definition.Description, required) {
			t.Fatalf("description missing %q: %s", required, definition.Description)
		}
	}
}

func TestPlanExecutionSchemaExplainsInitialCriterionWithoutBurdeningReplan(t *testing.T) {
	definition := planExecution(nil, contract.ServerContext{})
	for _, requiredText := range []string{
		"at least one nonblank top-level",
		"same-objective replan may omit",
		"never rewrites",
	} {
		if !strings.Contains(strings.ToLower(definition.Description), strings.ToLower(requiredText)) {
			t.Fatalf("description missing %q: %s", requiredText, definition.Description)
		}
	}
	properties := definition.InputSchema["properties"].(map[string]any)
	criteria := properties["completion_criteria"].(map[string]any)
	items := criteria["items"].(map[string]any)
	if criteria["minItems"] != 1 ||
		criteria["maxItems"] != protocol.ExecutionProjectionCollectionLimit ||
		items["pattern"] != `\S` {
		t.Fatalf("completion_criteria schema = %#v", criteria)
	}
	required := definition.InputSchema["required"].([]string)
	if slices.Contains(required, "completion_criteria") ||
		slices.Contains(required, "objective") {
		t.Fatalf("context-dependent creation fields became mandatory for replan: %#v", required)
	}
	if !slices.Contains(required, "work_graph_json") {
		t.Fatalf("required = %#v, want work_graph_json", required)
	}
	if _, exposesNestedItems := properties["items"]; exposesNestedItems {
		t.Fatalf("model schema still exposes provider-fragile nested items: %#v", properties)
	}
	workGraph := properties["work_graph_json"].(map[string]any)
	if workGraph["type"] != "string" || workGraph["pattern"] != `\S` {
		t.Fatalf("work_graph_json schema = %#v", workGraph)
	}
	for _, requiredText := range []string{
		"work_graph_json",
		"json array serialized inside one string",
		"do not send nested work item objects as a tool argument array",
	} {
		if !strings.Contains(strings.ToLower(definition.Description), strings.ToLower(requiredText)) {
			t.Fatalf("description missing %q: %s", requiredText, definition.Description)
		}
	}
}

func TestExecutionToolSchemasExposeProjectionCollectionLimits(t *testing.T) {
	plan := planExecutionSchema()["properties"].(map[string]any)
	submit := submitWorkSchema()["properties"].(map[string]any)
	review := reviewWorkSchema()["properties"].(map[string]any)
	criterion := review["criteria_results"].(map[string]any)["items"].(map[string]any)
	criterionProperties := criterion["properties"].(map[string]any)
	resume := resumeWorkSchema()["properties"].(map[string]any)

	for name, schema := range map[string]map[string]any{
		"completion_criteria": plan["completion_criteria"].(map[string]any),
		"result_refs":         submit["result_refs"].(map[string]any),
		"submission_evidence": submit["evidence"].(map[string]any),
		"criteria_results":    review["criteria_results"].(map[string]any),
		"criterion_evidence":  criterionProperties["evidence"].(map[string]any),
		"resume_evidence":     resume["evidence"].(map[string]any),
	} {
		if schema["maxItems"] != protocol.ExecutionProjectionCollectionLimit {
			t.Fatalf("%s maxItems = %#v", name, schema["maxItems"])
		}
	}
}
