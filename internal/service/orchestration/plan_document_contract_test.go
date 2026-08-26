// INPUT: parser-backed Plan Document schema contract 与其模型示例。
// OUTPUT: 字段集合、必填约束、不可变副本和示例可解析性的回归证明。
// POS: 防止 parser、MCP schema 与修复提示再次发生字段漂移。
package orchestration

import (
	"slices"
	"strings"
	"testing"
)

func TestExecutionPlanDocumentSchemaContractMatchesParser(t *testing.T) {
	t.Parallel()

	contract := ExecutionPlanDocumentSchemaContract()
	if correction := contract.CommonAliasCorrections["goal_binding"]; !strings.Contains(correction, "outer command input") {
		t.Fatalf("goal_binding correction = %q", correction)
	}
	assertPlanDocumentFieldSet(t, "root", contract.AllowedRootFields, planDocumentFields)
	assertPlanDocumentFieldSet(t, "item", contract.AllowedItemFields, planDocumentItemFields)
	for _, field := range contract.ParserRequiredRootFields {
		if _, ok := planDocumentFields[field]; !ok {
			t.Fatalf("required root field %q is not parser-allowed", field)
		}
	}
	for _, field := range contract.RequiredItemFields {
		if _, ok := planDocumentItemFields[field]; !ok {
			t.Fatalf("required item field %q is not parser-allowed", field)
		}
	}
	if _, _, err := ParseExecutionPlanDocument(contract.MinimalValidCreateExample); err != nil {
		t.Fatalf("contract example is not parser-valid: %v\n%s", err, contract.MinimalValidCreateExample)
	}
}

func TestExecutionPlanDocumentSchemaContractRequiredItemFieldsAreEnforced(t *testing.T) {
	t.Parallel()

	contract := ExecutionPlanDocumentSchemaContract()
	replacements := map[string][2]string{
		"logical_key": {
			"  - logical_key: produce\n    kind: produce",
			"  - kind: produce",
		},
		"kind": {
			"    kind: produce\n",
			"",
		},
		"subject": {
			"    subject: \"Produce the requested outcome\"\n",
			"",
		},
		"objective": {
			"    objective: \"Create the requested deliverable\"\n",
			"",
		},
		"deliverable": {
			"    deliverable: \"The completed requested outcome\"\n",
			"",
		},
	}
	for _, field := range contract.RequiredItemFields {
		replacement, ok := replacements[field]
		if !ok {
			t.Fatalf("required item field %q has no enforcement fixture", field)
		}
		source := strings.Replace(
			contract.MinimalValidCreateExample,
			replacement[0],
			replacement[1],
			1,
		)
		_, _, err := ParseExecutionPlanDocument(source)
		if err == nil || !strings.Contains(err.Error(), "required field is missing") {
			t.Fatalf("missing %q error = %v", field, err)
		}
	}
}

func TestExecutionPlanDocumentSchemaContractReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	contract := ExecutionPlanDocumentSchemaContract()
	contract.AllowedItemFields[0] = "mutated"
	contract.OperationRequirements["create"] = "mutated"
	contract.CommonAliasCorrections["dependencies"] = "mutated"

	fresh := ExecutionPlanDocumentSchemaContract()
	if fresh.AllowedItemFields[0] == "mutated" ||
		fresh.OperationRequirements["create"] == "mutated" ||
		fresh.CommonAliasCorrections["dependencies"] == "mutated" {
		t.Fatalf("schema contract exposed mutable canonical state: %#v", fresh)
	}
}

func assertPlanDocumentFieldSet(
	t *testing.T,
	name string,
	contractFields []string,
	parserFields map[string]struct{},
) {
	t.Helper()
	want := make([]string, 0, len(parserFields))
	for field := range parserFields {
		want = append(want, field)
	}
	slices.Sort(want)
	got := append([]string{}, contractFields...)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("%s fields = %v, parser allows %v", name, got, want)
	}
}
