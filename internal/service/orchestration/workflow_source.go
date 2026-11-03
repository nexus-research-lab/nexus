// INPUT: Optional exact saved template version and typed parameter values in a Plan Document.
// OUTPUT: Owner-validated provenance sealed with the Plan, never runtime authority.
// POS: Consuming-side template validation port and strict YAML data parser.
package orchestration

import (
	"context"
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"gopkg.in/yaml.v3"
)

type WorkflowSourceValidator interface {
	ValidateWorkflowSource(context.Context, string, protocol.WorkGraphWorkflowSource) (*protocol.WorkGraphWorkflowSource, error)
}

func (s *Service) SetWorkflowSourceValidator(v WorkflowSourceValidator) { s.workflowSources = v }
func workflowSourceMetadata(source *protocol.WorkGraphWorkflowSource) map[string]any {
	if source == nil {
		return nil
	}
	return map[string]any{"workflow_source": source}
}
func parsePlanWorkflowSource(fields map[string]planYAMLField, operation protocol.ExecutionPlanProposalOperation) (*protocol.WorkGraphWorkflowSource, error) {
	field, exists := fields["workflow"]
	if !exists {
		return nil, nil
	}
	node := field.value
	bad := func(message string) error { return newPlanDocumentError(node, "$.workflow", message, nil) }
	if operation == protocol.ExecutionPlanProposalReplan {
		return nil, bad("workflow provenance is fixed at create/replace, not replan")
	}
	if node.Kind != yaml.MappingNode {
		return nil, bad("workflow must be a mapping")
	}
	values, err := collectPlanYAMLFields(node, "$.workflow", planDocumentFieldSet([]string{"id", "version", "parameters"}))
	if err != nil {
		return nil, err
	}
	idNode, err := requirePlanYAMLField(values, node, "$.workflow", "id")
	if err != nil {
		return nil, err
	}
	id, err := parsePlanYAMLString(idNode, "$.workflow.id", true)
	if err != nil {
		return nil, err
	}
	versionNode, err := requirePlanYAMLField(values, node, "$.workflow", "version")
	if err != nil {
		return nil, err
	}
	version, err := parsePlanYAMLInteger(versionNode, "$.workflow.version")
	if err != nil {
		return nil, err
	}
	if version < 1 {
		return nil, bad("version must be positive")
	}
	source := &protocol.WorkGraphWorkflowSource{ID: id, Version: int64(version)}
	if p, ok := values["parameters"]; ok {
		if p.value.Kind != yaml.MappingNode || len(p.value.Content) > 64 {
			return nil, bad("parameters must contain at most 32 entries")
		}
		if err = p.value.Decode(&source.Parameters); err != nil {
			return nil, bad("parameters must be JSON-compatible values")
		}
		raw, err := json.Marshal(source.Parameters)
		if err != nil || len(raw) > 16384 {
			return nil, bad("parameters must be finite JSON data within 16 KiB")
		}
		_ = json.Unmarshal(raw, &source.Parameters)
	}
	return source, nil
}
