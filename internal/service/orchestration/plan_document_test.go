// INPUT: 合法、等价、恶意及越界的 Nexus Plan Document v1 YAML。
// OUTPUT: strict parser、typed canonical document、稳定 digest 与精确错误位置的行为证明。
// POS: proposal sealing 前文本边界的回归测试。
package orchestration

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"gopkg.in/yaml.v3"
)

const validMinimalPlanDocument = `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria:
  - Report is verified
items:
  - logical_key: make
    kind: produce
    subject: Make report
    objective: Produce the report
    deliverable: report.md
`

func TestParseExecutionPlanDocumentCanonicalizesTypedDraft(t *testing.T) {
	t.Parallel()

	source := `nexus_plan: 1
operation: replan
objective: "  ship evidence  "
completion_criteria:
  - " evidence is verified "
revision_reason: "  improve the graph  "
supersede_active_work: true
items:
  - logical_key: source
    existing_work_item_id: " work-source "
    kind: produce
    subject: " Collect sources "
    objective: " Gather primary evidence "
    deliverable: " source notes "
    acceptance_criteria:
      - " sources are cited "
    required: true
    input_refs:
      - " spec.md "
    shared_output_scopes:
      - " semantic:source-index "
  - logical_key: verify
    kind: verify
    subject: Verify report
    objective: Check the report against its evidence
    deliverable: Verification result
    acceptance_criteria: []
    terminal: true
    depends_on:
      - source
    output_scopes:
      - " dir:reports/../reports "
    shared_output_scopes:
      - " semantic:review "
`

	document, draft, err := ParseExecutionPlanDocument(source)
	if err != nil {
		t.Fatalf("ParseExecutionPlanDocument() error = %v", err)
	}
	if document.Version != protocol.ExecutionPlanProposalDocumentVersion ||
		document.Operation != protocol.ExecutionPlanProposalReplan ||
		document.Objective != "ship evidence" ||
		!reflect.DeepEqual(document.CompletionCriteria, []string{"evidence is verified"}) ||
		document.RevisionReason != "improve the graph" ||
		!document.SupersedeActiveWork ||
		len(document.Items) != 2 {
		t.Fatalf("canonical document = %#v", document)
	}
	if draft.RevisionReason != "improve the graph" || len(draft.Items) != 2 {
		t.Fatalf("canonical draft = %#v", draft)
	}

	sourceItem := document.Items[0]
	if sourceItem.ExistingWorkItemID != "work-source" ||
		sourceItem.Subject != "Collect sources" ||
		!reflect.DeepEqual(sourceItem.AcceptanceCriteria, []string{"sources are cited"}) ||
		!reflect.DeepEqual(sourceItem.InputRefs, []string{"spec.md"}) ||
		!reflect.DeepEqual(sourceItem.OutputScopes, []protocol.WorkOutputScope{{
			Scope: "semantic:source-index",
			Mode:  protocol.WorkOutputScopeShared,
		}}) {
		t.Fatalf("source item = %#v", sourceItem)
	}

	verifyItem := document.Items[1]
	if !reflect.DeepEqual(verifyItem.DependsOn, []protocol.ExecutionPlanProposalDependency{{
		LogicalKey: "source",
		Kind:       protocol.WorkDependencyHard,
	}}) || !reflect.DeepEqual(verifyItem.OutputScopes, []protocol.WorkOutputScope{
		{Scope: "dir:reports", Mode: protocol.WorkOutputScopeExclusive},
		{Scope: "semantic:review", Mode: protocol.WorkOutputScopeShared},
	}) {
		t.Fatalf("verify item = %#v", verifyItem)
	}
	if !reflect.DeepEqual(draft.Items[1].DependsOn, []PlanDependencyDraft{{
		LogicalKey: "source",
		Kind:       protocol.WorkDependencyHard,
	}}) || !reflect.DeepEqual(draft.Items[1].OutputScopes, verifyItem.OutputScopes) {
		t.Fatalf("document/draft canonical WorkGraph diverged: %#v / %#v", document.Items[1], draft.Items[1])
	}
}

func TestParseExecutionPlanDocumentDigestIgnoresYAMLPresentation(t *testing.T) {
	t.Parallel()

	left := `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [Report is verified]
items:
  - logical_key: source_a
    kind: produce
    subject: Source A
    objective: Gather A
    deliverable: A notes
  - logical_key: source_b
    kind: produce
    subject: Source B
    objective: Gather B
    deliverable: B notes
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate and verify
    deliverable: report.md
    acceptance_criteria: []
    depends_on: [source_b, source_a]
    input_refs: [brief.md]
    output_scopes: [semantic:z, semantic:b]
    shared_output_scopes: [semantic:a]
`
	right := `
# Presentation and mapping order do not participate in proposal identity.
operation: create
nexus_plan: 1
completion_criteria:
  - "Report is verified"
objective: " Ship report "
revision_reason: "   "
items:
  - deliverable: A notes
    objective: Gather A
    subject: Source A
    kind: produce
    logical_key: source_a
    acceptance_criteria: []
  - subject: Source B
    objective: Gather B
    deliverable: B notes
    logical_key: source_b
    kind: produce
  - input_refs:
      - brief.md
    logical_key: final
    deliverable: report.md
    subject: Final
    objective: Integrate and verify
    kind: integrate
    depends_on: [source_a, source_b]
    shared_output_scopes: [semantic:a]
    output_scopes:
      - semantic:b
      - semantic:z
`

	leftDocument, _, err := ParseExecutionPlanDocument(left)
	if err != nil {
		t.Fatalf("parse left: %v", err)
	}
	rightDocument, _, err := ParseExecutionPlanDocument(right)
	if err != nil {
		t.Fatalf("parse right: %v", err)
	}
	if !reflect.DeepEqual(leftDocument, rightDocument) {
		t.Fatalf("equivalent YAML produced different typed documents:\nleft  = %#v\nright = %#v", leftDocument, rightDocument)
	}
	leftDigest, err := protocol.DigestExecutionPlanProposalDocument(leftDocument)
	if err != nil {
		t.Fatalf("digest left: %v", err)
	}
	rightDigest, err := protocol.DigestExecutionPlanProposalDocument(rightDocument)
	if err != nil {
		t.Fatalf("digest right: %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("equivalent YAML digests differ: %q != %q", leftDigest, rightDigest)
	}

	changed := strings.Replace(right, "subject: Final", "subject: Final report", 1)
	changedDocument, _, err := ParseExecutionPlanDocument(changed)
	if err != nil {
		t.Fatalf("parse changed: %v", err)
	}
	changedDigest, err := protocol.DigestExecutionPlanProposalDocument(changedDocument)
	if err != nil {
		t.Fatalf("digest changed: %v", err)
	}
	if changedDigest == leftDigest {
		t.Fatal("semantic change did not change the typed document digest")
	}
}

func TestParseExecutionPlanDocumentAllowsDraftReviewFinalizeFileHandoff(t *testing.T) {
	t.Parallel()
	const source = `nexus_plan: 1
operation: create
objective: Produce a reviewed demonstration document
completion_criteria:
  - The final document exists
items:
  - logical_key: draft
    kind: produce
    subject: Draft document
    objective: Write the initial document
    deliverable: output/workgraph-demo.md
    output_scopes:
      - file:output/workgraph-demo.md
  - logical_key: review
    kind: review
    subject: Review document
    objective: Review the accepted draft
    deliverable: Review verdict
    depends_on:
      - draft
    output_scopes:
      - semantic:review-verdict
  - logical_key: finalize
    kind: integrate
    subject: Finalize document
    objective: Apply the accepted review
    deliverable: output/workgraph-demo.md
    terminal: true
    depends_on:
      - review
    output_scopes:
      - file:output/workgraph-demo.md
`
	document, _, err := ParseExecutionPlanDocument(source)
	if err != nil {
		t.Fatalf("hard-ordered file handoff rejected: %v", err)
	}
	if len(document.Items) != 3 ||
		document.Items[0].OutputScopes[0].Scope != document.Items[2].OutputScopes[0].Scope {
		t.Fatalf("parsed handoff document = %#v", document)
	}
}

func TestParseExecutionPlanDocumentPreservesReplanBoundaryPresence(t *testing.T) {
	t.Parallel()

	omitted := strings.Replace(
		validMinimalPlanDocument,
		"operation: create\nobjective: Ship report\ncompletion_criteria:\n  - Report is verified\n",
		"operation: replan\nrevision_reason: refresh graph\n",
		1,
	)
	explicitEmpty := strings.Replace(
		omitted,
		"revision_reason: refresh graph\n",
		"revision_reason: refresh graph\ncompletion_criteria: []\n",
		1,
	)
	omittedDocument, _, err := ParseExecutionPlanDocument(omitted)
	if err != nil {
		t.Fatalf("parse omitted boundary: %v", err)
	}
	explicitDocument, _, err := ParseExecutionPlanDocument(explicitEmpty)
	if err != nil {
		t.Fatalf("parse explicit boundary: %v", err)
	}
	if omittedDocument.CompletionCriteria != nil {
		t.Fatalf("omitted completion_criteria = %#v, want nil", omittedDocument.CompletionCriteria)
	}
	if explicitDocument.CompletionCriteria == nil || len(explicitDocument.CompletionCriteria) != 0 {
		t.Fatalf("explicit completion_criteria = %#v, want non-nil empty", explicitDocument.CompletionCriteria)
	}
	omittedDigest, _ := protocol.DigestExecutionPlanProposalDocument(omittedDocument)
	explicitDigest, _ := protocol.DigestExecutionPlanProposalDocument(explicitDocument)
	if omittedDigest == explicitDigest {
		t.Fatal("omitted and explicitly empty replan boundaries must not have the same digest")
	}
}

func TestParseExecutionPlanDocumentRejectsUnsupportedYAMLAndSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      string
		wantPath    string
		wantMessage string
	}{
		{name: "empty", source: " \n\t", wantPath: "$", wantMessage: "must not be empty"},
		{
			name:        "unsupported version",
			source:      strings.Replace(validMinimalPlanDocument, "nexus_plan: 1", "nexus_plan: 2", 1),
			wantPath:    "$.nexus_plan",
			wantMessage: "unsupported version",
		},
		{
			name:        "unsupported operation",
			source:      strings.Replace(validMinimalPlanDocument, "operation: create", "operation: append", 1),
			wantPath:    "$.operation",
			wantMessage: "operation must be create, replan, or replace",
		},
		{
			name:        "unknown root field",
			source:      validMinimalPlanDocument + "mystery: true\n",
			wantPath:    "$.mystery",
			wantMessage: "unknown field",
		},
		{
			name: "unknown item field",
			source: strings.Replace(
				validMinimalPlanDocument,
				"    kind: produce",
				"    mystery: value\n    kind: produce",
				1,
			),
			wantPath:    "$.items[0].mystery",
			wantMessage: "unknown field",
		},
		{
			name: "duplicate key",
			source: strings.Replace(
				validMinimalPlanDocument,
				"operation: create",
				"operation: create\noperation: create",
				1,
			),
			wantPath:    "$.operation",
			wantMessage: "duplicate mapping key",
		},
		{
			name:        "multiple documents",
			source:      validMinimalPlanDocument + "---\nnexus_plan: 1\n",
			wantPath:    "$",
			wantMessage: "multiple YAML documents",
		},
		{
			name: "anchor and alias",
			source: strings.Replace(
				strings.Replace(validMinimalPlanDocument, "objective: Ship report", "objective: &objective Ship report", 1),
				"subject: Make report",
				"subject: *objective",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "anchors are not allowed",
		},
		{
			name: "explicit tag",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: !!str Ship report",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "explicit YAML tags",
		},
		{
			name: "merge key",
			source: `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [verified]
items:
  - <<: {logical_key: make, kind: produce, subject: Make, objective: Ship, deliverable: report.md}
`,
			wantPath:    "$.items[0].<<",
			wantMessage: "merge keys are not allowed",
		},
		{
			name: "null",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: null",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "null values are not allowed",
		},
		{
			name: "timestamp",
			source: strings.Replace(
				validMinimalPlanDocument,
				"objective: Ship report",
				"objective: 2026-08-05",
				1,
			),
			wantPath:    "$.objective",
			wantMessage: "timestamp values are not allowed",
		},
		{
			name:        "float",
			source:      strings.Replace(validMinimalPlanDocument, "nexus_plan: 1", "nexus_plan: 1.0", 1),
			wantPath:    "$.nexus_plan",
			wantMessage: "scalar type !!float is not allowed",
		},
		{
			name: "wrong collection type",
			source: `nexus_plan: 1
operation: create
objective: Ship report
completion_criteria: [verified]
items: {}
`,
			wantPath:    "$.items",
			wantMessage: "must be a sequence",
		},
		{
			name:        "syntax error",
			source:      "nexus_plan: [\n",
			wantPath:    "$",
			wantMessage: "invalid YAML",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseExecutionPlanDocument(test.source)
			assertPlanDocumentError(t, err, test.wantPath, test.wantMessage)
		})
	}
}

func TestParseExecutionPlanDocumentRejectsResourceAndCombinedCollectionLimits(t *testing.T) {
	t.Parallel()

	t.Run("bytes", func(t *testing.T) {
		t.Parallel()
		_, _, err := ParseExecutionPlanDocument(strings.Repeat("x", maxExecutionPlanDocumentBytes+1))
		assertPlanDocumentError(t, err, "$", "byte limit")
	})
	t.Run("items", func(t *testing.T) {
		t.Parallel()
		var source strings.Builder
		source.WriteString("nexus_plan: 1\noperation: create\nobjective: Ship\ncompletion_criteria: [done]\nitems:\n")
		for index := 0; index <= protocol.ExecutionProjectionCollectionLimit; index++ {
			fmt.Fprintf(
				&source,
				"  - logical_key: item_%d\n    kind: produce\n    subject: Item\n    objective: Work\n    deliverable: out-%d\n",
				index,
				index,
			)
		}
		_, _, err := ParseExecutionPlanDocument(source.String())
		assertPlanDocumentError(t, err, "$.items", "maximum is 32")
	})
	t.Run("combined dependencies", func(t *testing.T) {
		t.Parallel()
		hard := strings.TrimSuffix(strings.Repeat("source,", 17), ",")
		soft := strings.TrimSuffix(strings.Repeat("source,", 16), ",")
		source := fmt.Sprintf(`nexus_plan: 1
operation: create
objective: Ship
completion_criteria: [done]
items:
  - logical_key: source
    kind: produce
    subject: Source
    objective: Gather
    deliverable: notes
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate
    deliverable: report
    depends_on: [%s]
    soft_depends_on: [%s]
`, hard, soft)
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.items[1].depends_on", "combined hard and soft dependencies")
	})
	t.Run("combined output scopes", func(t *testing.T) {
		t.Parallel()
		exclusive := make([]string, 17)
		shared := make([]string, 16)
		for index := range exclusive {
			exclusive[index] = fmt.Sprintf("semantic:exclusive-%d", index)
		}
		for index := range shared {
			shared[index] = fmt.Sprintf("semantic:shared-%d", index)
		}
		source := fmt.Sprintf(`nexus_plan: 1
operation: create
objective: Ship
completion_criteria: [done]
items:
  - logical_key: final
    kind: integrate
    subject: Final
    objective: Integrate
    deliverable: report
    output_scopes: [%s]
    shared_output_scopes: [%s]
`, strings.Join(exclusive, ","), strings.Join(shared, ","))
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.items[0].output_scopes", "combined exclusive and shared output scopes")
	})
	t.Run("depth", func(t *testing.T) {
		t.Parallel()
		source := strings.Repeat("[", maxExecutionPlanYAMLDepth+2) + "x" +
			strings.Repeat("]", maxExecutionPlanYAMLDepth+2)
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$*", "nesting depth")
	})
	t.Run("node count", func(t *testing.T) {
		t.Parallel()
		source := "mystery: " + nestedPlanFlowSequence(22, 3) + "\n"
		if len(source) >= maxExecutionPlanDocumentBytes {
			t.Fatalf("node-count fixture is unexpectedly too large: %d bytes", len(source))
		}
		_, _, err := ParseExecutionPlanDocument(source)
		assertPlanDocumentError(t, err, "$.mystery*", "node count")
	})
}

func TestParseExecutionPlanDocumentWrapsDomainErrorWithDocumentLocation(t *testing.T) {
	t.Parallel()

	source := strings.Replace(
		validMinimalPlanDocument,
		"    deliverable: report.md",
		"    deliverable: report.md\n    depends_on: [missing]",
		1,
	)
	_, _, err := ParseExecutionPlanDocument(source)
	typed := assertPlanDocumentError(t, err, "$.items[0].depends_on", "unknown_dependency")
	var domainErr *DomainError
	if !errors.As(typed, &domainErr) || domainErr.Code != ErrorCodeUnknownDependency {
		t.Fatalf("wrapped domain error = %#v, want %s", domainErr, ErrorCodeUnknownDependency)
	}
	result := RejectedResult(nil, typed, nil)
	if result.ReasonCode != ErrorCodeUnknownDependency ||
		!strings.Contains(result.Message, "$.items[0].depends_on") ||
		!strings.Contains(result.Message, "line ") ||
		!strings.Contains(result.Message, "column ") {
		t.Fatalf("projected domain error lost location: %#v", result)
	}
}

func TestValidatePlanYAMLNodeRejectsAliasNodes(t *testing.T) {
	t.Parallel()

	node := &yaml.Node{Kind: yaml.AliasNode, Value: "source", Line: 4, Column: 7}
	count := 0
	err := validatePlanYAMLNode(node, "$.items[0].subject", 0, &count)
	typed := assertPlanDocumentError(t, err, "$.items[0].subject", "aliases are not allowed")
	if typed.Line != 4 || typed.Column != 7 {
		t.Fatalf("alias location = %d:%d, want 4:7", typed.Line, typed.Column)
	}
}

func assertPlanDocumentError(
	t *testing.T,
	err error,
	wantPath string,
	wantMessage string,
) *PlanDocumentError {
	t.Helper()
	if err == nil {
		t.Fatal("ParseExecutionPlanDocument() error = nil")
	}
	var typed *PlanDocumentError
	if !errors.As(err, &typed) {
		t.Fatalf("error type = %T, want *PlanDocumentError: %v", err, err)
	}
	pathMatches := typed.Path == wantPath
	if strings.HasSuffix(wantPath, "*") {
		pathMatches = strings.HasPrefix(typed.Path, strings.TrimSuffix(wantPath, "*"))
	}
	if !pathMatches {
		t.Fatalf("error path = %q, want %q: %v", typed.Path, wantPath, typed)
	}
	if typed.Line <= 0 || typed.Column <= 0 {
		t.Fatalf("error location = %d:%d, want positive one-based coordinates", typed.Line, typed.Column)
	}
	if !strings.Contains(typed.Message, wantMessage) {
		t.Fatalf("error message = %q, want substring %q", typed.Message, wantMessage)
	}
	return typed
}

func nestedPlanFlowSequence(width, depth int) string {
	value := "x"
	for range depth {
		entries := make([]string, width)
		for index := range entries {
			entries[index] = value
		}
		value = "[" + strings.Join(entries, ",") + "]"
	}
	return value
}
