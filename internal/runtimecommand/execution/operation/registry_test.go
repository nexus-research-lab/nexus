package operation

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestPlanPreparationIsDurableButNotReadOnly(t *testing.T) {
	definitions := BuildAll(nil, contract.Context{})
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

func TestExecutionOperationSchemasHideFencingAndIdempotency(t *testing.T) {
	for _, definition := range BuildAll(nil, contract.Context{}) {
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

func TestPlanOperationSchemasExposeDocumentGoalIntentThenExactSealedReference(t *testing.T) {
	prepare := preparePlanExecution(nil, contract.Context{})
	commit := planExecution(nil, contract.Context{})

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
	if !strings.Contains(planDocumentDescription, "goal_binding is not a YAML field") ||
		!strings.Contains(goalBinding["description"].(string), "Outer command input beside plan_document") {
		t.Fatalf("prepare schema blurs outer command input and Plan YAML: %#v", prepare.InputSchema)
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
	definition := auditExecutionAlignment(nil, contract.Context{})
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

func TestExecutionOperationSchemasExposeProjectionCollectionLimits(t *testing.T) {
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

func TestWorkReferenceSchemasDescribeOnlyTheirActualAuthorityLane(t *testing.T) {
	for name, schema := range map[string]map[string]any{
		"assign_work":    assignWorkSchema(),
		"take_over_work": takeOverWorkSchema(),
	} {
		properties := schema["properties"].(map[string]any)
		for _, field := range []string{"work_item_id", "logical_key"} {
			description := properties[field].(map[string]any)["description"].(string)
			if strings.Contains(description, "WorkBinding") ||
				strings.Contains(description, "ReviewBinding") {
				t.Fatalf("%s.%s invents a binding authority lane: %s", name, field, description)
			}
		}
	}

	for name, schema := range map[string]map[string]any{
		"submit_work": submitWorkSchema(),
		"block_work":  blockWorkSchema(),
		"resume_work": resumeWorkSchema(),
	} {
		properties := schema["properties"].(map[string]any)
		for _, field := range []string{"work_item_id", "logical_key"} {
			description := properties[field].(map[string]any)["description"].(string)
			if !strings.Contains(description, "exact trusted WorkBinding") ||
				!strings.Contains(description, "explicit value must match") {
				t.Fatalf("%s.%s omits trusted-default/fail-closed semantics: %s", name, field, description)
			}
		}
	}

	review := reviewWorkSchema()["properties"].(map[string]any)
	for _, field := range []string{"submission_id", "work_item_id", "logical_key"} {
		description := review[field].(map[string]any)["description"].(string)
		if !strings.Contains(description, "exact trusted ReviewBinding") ||
			!strings.Contains(description, "exact trusted WorkBinding") ||
			!strings.Contains(description, "must match") {
			t.Fatalf("review_work.%s omits exact ReviewBinding consistency: %s", field, description)
		}
	}
}

func TestSubmitAndReviewSurfacesKeepConditionalLocatorsExplicitAndStable(t *testing.T) {
	type toolSurface struct {
		description string
		schema      map[string]any
	}
	unbound := contract.Context{}
	workBound := contract.Context{WorkBinding: &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
	}}
	reviewBound := contract.Context{ReviewBinding: &protocol.ExecutionReviewBinding{
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "reviewer-1",
	}}

	unboundSubmit := submitWork(nil, unbound)
	boundSubmit := submitWork(nil, workBound)
	unboundReview := reviewWork(nil, unbound)
	boundReview := reviewWork(nil, reviewBound)
	selfReview := reviewWork(nil, workBound)
	for _, pair := range []struct {
		name  string
		left  toolSurface
		right toolSurface
	}{
		{
			name:  "submit_work",
			left:  toolSurface{description: unboundSubmit.Description, schema: unboundSubmit.InputSchema},
			right: toolSurface{description: boundSubmit.Description, schema: boundSubmit.InputSchema},
		},
		{
			name:  "review_work",
			left:  toolSurface{description: unboundReview.Description, schema: unboundReview.InputSchema},
			right: toolSurface{description: boundReview.Description, schema: boundReview.InputSchema},
		},
		{
			name:  "review_work_self_binding",
			left:  toolSurface{description: unboundReview.Description, schema: unboundReview.InputSchema},
			right: toolSurface{description: selfReview.Description, schema: selfReview.InputSchema},
		},
	} {
		leftSchema, err := json.Marshal(pair.left.schema)
		if err != nil {
			t.Fatal(err)
		}
		rightSchema, err := json.Marshal(pair.right.schema)
		if err != nil {
			t.Fatal(err)
		}
		if pair.left.description != pair.right.description ||
			string(leftSchema) != string(rightSchema) {
			t.Fatalf("%s model surface changed with trusted binding", pair.name)
		}
	}

	for _, field := range []string{"assigned_work", "current_actor", "not proof"} {
		if !strings.Contains(unboundSubmit.Description, field) ||
			!strings.Contains(unboundReview.Description, field) {
			t.Fatalf("submit/review descriptions do not distinguish projection from capability: missing %q", field)
		}
	}
	if !strings.Contains(unboundSubmit.Description, "DM coordination or any unbound round") ||
		!strings.Contains(unboundSubmit.Description, "provide work_item_id or logical_key") {
		t.Fatalf("submit_work description omits unbound locator rule: %s", unboundSubmit.Description)
	}
	if !strings.Contains(unboundReview.Description, "DM coordination or any unbound round") ||
		!strings.Contains(unboundReview.Description, "provide at least one of submission_id, work_item_id, or logical_key") {
		t.Fatalf("review_work description omits unbound locator rule: %s", unboundReview.Description)
	}

	submitRequired := unboundSubmit.InputSchema["required"].([]string)
	reviewRequired := unboundReview.InputSchema["required"].([]string)
	if !slices.Equal(submitRequired, []string{"result_summary"}) ||
		!slices.Equal(reviewRequired, []string{"decision"}) {
		t.Fatalf("conditional locators changed stable required arrays: submit=%#v review=%#v", submitRequired, reviewRequired)
	}
	submitProperties := unboundSubmit.InputSchema["properties"].(map[string]any)
	for _, field := range []string{"work_item_id", "logical_key"} {
		description := submitProperties[field].(map[string]any)["description"].(string)
		if !strings.Contains(description, "DM coordination or any unbound call") ||
			!strings.Contains(description, "assigned_work/current_actor projections do not establish that binding") {
			t.Fatalf("submit_work.%s omits unbound conditional requirement: %s", field, description)
		}
	}
	assignmentDescription := submitProperties["assignment_id"].(map[string]any)["description"].(string)
	if !strings.Contains(assignmentDescription, "never replaces the required Work Item locator") {
		t.Fatalf("submit_work.assignment_id implies it can locate work: %s", assignmentDescription)
	}
	reviewProperties := unboundReview.InputSchema["properties"].(map[string]any)
	for _, field := range []string{"submission_id", "work_item_id", "logical_key"} {
		description := reviewProperties[field].(map[string]any)["description"].(string)
		if !strings.Contains(description, "DM coordination or any unbound call") ||
			!strings.Contains(description, "at least one of submission_id, work_item_id, or logical_key") ||
			!strings.Contains(description, "assigned_work/current_actor projections do not establish either binding") {
			t.Fatalf("review_work.%s omits unbound conditional requirement: %s", field, description)
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
