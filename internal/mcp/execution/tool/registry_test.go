package tool

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestPlanPreparationIsDurableButNotReadOnly(t *testing.T) {
	definitions := BuildAll(nil, contract.ServerContext{})
	readOnlyByName := map[string]bool{}
	for _, definition := range definitions {
		readOnlyByName[definition.Name] = definition.Annotations != nil &&
			(definition.Annotations.ReadOnly || definition.Annotations.ReadOnlyHint)
	}
	for name, wantReadOnly := range map[string]bool{
		"get_execution":          false,
		"prepare_plan_execution": false,
		"plan_execution":         false,
	} {
		if got := readOnlyByName[name]; got != wantReadOnly {
			t.Fatalf("%s read-only annotation = %t, want %t", name, got, wantReadOnly)
		}
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

func TestPlanToolSchemasExposeDocumentGoalIntentThenExactSealedReference(t *testing.T) {
	prepare := preparePlanExecution(nil, contract.ServerContext{})
	commit := planExecution(nil, contract.ServerContext{})

	prepareProperties := prepare.InputSchema["properties"].(map[string]any)
	prepareRequired := prepare.InputSchema["required"].([]string)
	goalBinding := prepareProperties["goal_binding"].(map[string]any)
	if len(prepareProperties) != 2 ||
		prepareProperties["plan_document"].(map[string]any)["type"] != "string" ||
		goalBinding["type"] != "string" ||
		!slices.Equal(goalBinding["enum"].([]string), []string{"none", "current", "inherit"}) ||
		!slices.Equal(prepareRequired, []string{"plan_document"}) {
		t.Fatalf("prepare schema = %#v", prepare.InputSchema)
	}
	planDocumentDescription := prepareProperties["plan_document"].(map[string]any)["description"].(string)
	if !strings.Contains(
		planDocumentDescription,
		orchestration.ExecutionPlanDocumentSchemaContract().OutputScopeRequirements,
	) {
		t.Fatalf("prepare schema omits output scope handoff semantics: %s", planDocumentDescription)
	}
	commitProperties := commit.InputSchema["properties"].(map[string]any)
	commitRequired := commit.InputSchema["required"].([]string)
	if len(commitProperties) != 2 ||
		commitProperties["proposal_id"].(map[string]any)["type"] != "string" ||
		commitProperties["proposal_digest"].(map[string]any)["type"] != "string" ||
		!slices.Equal(commitRequired, []string{"proposal_id", "proposal_digest"}) {
		t.Fatalf("commit schema = %#v", commit.InputSchema)
	}

	for _, definition := range []struct {
		name   string
		schema map[string]any
	}{
		{name: prepare.Name, schema: prepare.InputSchema},
		{name: commit.Name, schema: commit.InputSchema},
	} {
		assertClosedObjectSchemas(t, definition.schema)
		encoded, err := json.Marshal(definition.schema)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			`"items":`,
			`"objective":`,
			`"completion_criteria":`,
			`"work_graph_json":`,
			`"execution_id":`,
		} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("%s schema leaks old or trusted field %s: %s", definition.name, forbidden, encoded)
			}
		}
	}
}

func TestAuditExecutionAlignmentUsesPortableNativeReportSchema(t *testing.T) {
	definition := auditExecutionAlignment(nil, contract.ServerContext{})
	assertPortableSchemaKeywords(t, definition.InputSchema)
	assertClosedObjectSchemas(t, definition.InputSchema)

	required := definition.InputSchema["required"].([]string)
	for _, field := range []string{"decision", "criteria_results", "summary"} {
		if !slices.Contains(required, field) {
			t.Fatalf("alignment required = %#v, missing %s", required, field)
		}
	}
	properties := definition.InputSchema["properties"].(map[string]any)
	criteria := properties["criteria_results"].(map[string]any)
	criterion := criteria["items"].(map[string]any)
	criterionProperties := criterion["properties"].(map[string]any)
	evidence := criterionProperties["evidence"].(map[string]any)
	if criteria["type"] != "array" || criterion["type"] != "object" ||
		evidence["type"] != "array" || evidence["items"].(map[string]any)["type"] != "object" {
		t.Fatalf("alignment report schema = %#v", criteria)
	}
	encoded, err := json.Marshal(definition.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, unsupported := range []string{`"pattern":`, `"minItems":`, `"maxItems":`} {
		if strings.Contains(string(encoded), unsupported) {
			t.Fatalf("portable alignment schema contains %s: %s", unsupported, encoded)
		}
	}
}

func assertPortableSchemaKeywords(t *testing.T, schema map[string]any) {
	t.Helper()
	allowed := map[string]struct{}{
		"type":                 {},
		"properties":           {},
		"required":             {},
		"additionalProperties": {},
		"items":                {},
		"enum":                 {},
		"description":          {},
	}
	for key := range schema {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("portable plan schema contains unsupported keyword %q: %#v", key, schema)
		}
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for propertyName, rawProperty := range properties {
			property, propertyOK := rawProperty.(map[string]any)
			if !propertyOK {
				t.Fatalf("portable plan property %q = %#v, want schema object", propertyName, rawProperty)
			}
			assertPortableSchemaKeywords(t, property)
		}
	}
	if rawItems, ok := schema["items"]; ok {
		items, itemsOK := rawItems.(map[string]any)
		if !itemsOK {
			t.Fatalf("portable plan items = %#v, want schema object", rawItems)
		}
		assertPortableSchemaKeywords(t, items)
	}
}

func assertClosedObjectSchemas(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if typed["type"] == "object" && typed["additionalProperties"] != false {
			t.Fatalf("object schema must reject additional properties: %#v", typed)
		}
		for _, child := range typed {
			assertClosedObjectSchemas(t, child)
		}
	case []any:
		for _, child := range typed {
			assertClosedObjectSchemas(t, child)
		}
	}
}

func TestExecutionToolSchemasExposeProjectionCollectionLimits(t *testing.T) {
	submit := submitWorkSchema()["properties"].(map[string]any)
	review := reviewWorkSchema()["properties"].(map[string]any)
	criterion := review["criteria_results"].(map[string]any)["items"].(map[string]any)
	criterionProperties := criterion["properties"].(map[string]any)
	resume := resumeWorkSchema()["properties"].(map[string]any)

	for name, schema := range map[string]map[string]any{
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

func TestGoalPromotionSchemaAllowsAgentSelectedComplexityReason(t *testing.T) {
	properties := promoteExecutionSchema()["properties"].(map[string]any)
	reasons := properties["activation_reason"].(map[string]any)["enum"].([]string)
	if !slices.Contains(reasons, string(protocol.GoalActivationReasonSubstantialComplexity)) {
		t.Fatalf("Goal promotion reasons = %#v", reasons)
	}
}
