// INPUT: Nexus Plan Document v1 的 parser 字段与稳定 transport 约束。
// OUTPUT: parser 与模型工具共用、区分外层 command/内层 YAML、明确 current Execution operation 选择且不可变的 schema contract 和有效 create 示例。
// POS: strict YAML parser 前的单一字段真相源；具体 operation authority 仍由 proposal service 校验。
package orchestration

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var planDocumentAllowedRootFields = []string{
	"nexus_plan",
	"operation",
	"objective",
	"completion_criteria",
	"revision_reason",
	"supersede_active_work",
	"replacement_reason",
	"items",
}

var planDocumentRequiredRootFields = []string{
	"nexus_plan",
	"operation",
	"items",
}

var planDocumentAllowedItemFields = []string{
	"logical_key",
	"existing_work_item_id",
	"kind",
	"subject",
	"objective",
	"deliverable",
	"acceptance_criteria",
	"required",
	"terminal",
	"parent_logical_key",
	"depends_on",
	"soft_depends_on",
	"input_refs",
	"output_scopes",
	"shared_output_scopes",
}

var planDocumentRequiredItemFields = []string{
	"logical_key",
	"kind",
	"subject",
	"objective",
	"deliverable",
}

const planDocumentMinimalCreateExample = `nexus_plan: 1
operation: create
objective: "Deliver the requested outcome"
completion_criteria:
  - "The requested outcome is delivered and verified"
items:
  - logical_key: produce
    kind: produce
    subject: "Produce the requested outcome"
    objective: "Create the requested deliverable"
    deliverable: "The completed requested outcome"
    acceptance_criteria:
      - "The deliverable satisfies the requested scope"
    required: true
    terminal: true
    depends_on: []
    output_scopes:
      - "semantic:requested-outcome"`

// PlanDocumentSchemaContract is the stable, parser-backed shape consumers need
// to author or repair a complete document. Conditional root requirements remain
// operation-specific and are described by the tool that knows current context.
type PlanDocumentSchemaContract struct {
	Version                   int               `json:"version"`
	ParserRequiredRootFields  []string          `json:"parser_required_root_fields"`
	AllowedRootFields         []string          `json:"allowed_root_fields"`
	RequiredItemFields        []string          `json:"required_item_fields"`
	AllowedItemFields         []string          `json:"allowed_item_fields"`
	OperationRequirements     map[string]string `json:"operation_requirements"`
	OutputScopeRequirements   string            `json:"output_scope_requirements"`
	CommonAliasCorrections    map[string]string `json:"common_alias_corrections"`
	MinimalValidCreateExample string            `json:"minimal_valid_create_example"`
}

// ExecutionPlanDocumentSchemaContract returns fresh collections so the parser's
// canonical field sets cannot be mutated by MCP or other consumers.
func ExecutionPlanDocumentSchemaContract() PlanDocumentSchemaContract {
	return PlanDocumentSchemaContract{
		Version:                  protocol.ExecutionPlanProposalDocumentVersion,
		ParserRequiredRootFields: cloneStrings(planDocumentRequiredRootFields),
		AllowedRootFields:        cloneStrings(planDocumentAllowedRootFields),
		RequiredItemFields:       cloneStrings(planDocumentRequiredItemFields),
		AllowedItemFields:        cloneStrings(planDocumentAllowedItemFields),
		OperationRequirements: map[string]string{
			"create":  "required when execution inspect returns no current Execution, including the first successor Plan after Goal reset or retarget; completion_criteria is required; objective may be omitted only when an active Goal supplies and the service inherits the exact Goal objective; existing_work_item_id is forbidden",
			"replan":  "allowed only when execution inspect returns the current Execution and this proposal adds an immutable Plan revision to that same objective boundary; revision_reason is required; objective and completion_criteria inherit the current Execution boundary; existing_work_item_id may reuse an unchanged Work Item",
			"replace": "allowed only for a current transient Goal-free Execution that must be replaced as a whole; objective, completion_criteria, and replacement_reason are required; a Goal-bound Execution cannot be replaced",
		},
		OutputScopeRequirements: "each scope must be exactly file:<workspace-relative-path>, dir:<workspace-relative-path>, or semantic:<stable-key>; file/dir paths must use forward slashes, cannot be absolute, cannot be '.', and cannot escape with '..'; never copy an owner or Agent absolute workspace path into a scope—use a workspace-relative path when the graph coordinates that shared location, otherwise use semantic:<stable-key> for a member-produced artifact. output_scopes are orchestration scheduling/review declarations, not filesystem-enforced write locks, and are exclusive by default; overlapping exclusive scopes are allowed only when one Work Item reaches the other through an all-hard depends_on path, which transfers ownership after upstream Acceptance; parallel, sibling, unrelated, and soft-only overlap must use distinct scopes, or shared_output_scopes only when concurrent writing is genuinely safe",
		CommonAliasCorrections: map[string]string{
			"dependencies": "depends_on or soft_depends_on",
			"description":  "subject plus objective plus deliverable",
			"acceptance":   "acceptance_criteria",
			"goal_binding": "outer command input beside plan_document; never a Plan Document YAML root field",
			"scopes":       "output_scopes or shared_output_scopes",
		},
		MinimalValidCreateExample: strings.TrimSpace(planDocumentMinimalCreateExample),
	}
}

func planDocumentFieldSet(fields []string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}
