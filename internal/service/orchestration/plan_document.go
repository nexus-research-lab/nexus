// INPUT: Provider portable 的单个 Nexus Plan Document v1 YAML string。
// OUTPUT: 严格解析并规范化的 typed proposal document 与既有 PlanDraft。
// POS: 非可信文本传输到 proposal sealing 之间的唯一语法、资源与 canonicalization 边界。
package orchestration

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"gopkg.in/yaml.v3"
)

const (
	maxExecutionPlanDocumentBytes = 64 << 10
	maxExecutionPlanYAMLNodes     = 8192
	maxExecutionPlanYAMLDepth     = 16
)

var yamlSyntaxLinePattern = regexp.MustCompile(`\bline ([0-9]+)\b`)

// PlanDocumentError pinpoints a transport, YAML, schema, or WorkGraph error in
// the submitted document. Line and Column are always one-based and Path uses a
// JSONPath-like semantic location independent of YAML formatting.
type PlanDocumentError struct {
	Path    string
	Line    int
	Column  int
	Message string
	Cause   error
}

func (e *PlanDocumentError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf(
		"plan document %s at line %d, column %d: %s",
		e.Path,
		e.Line,
		e.Column,
		e.Message,
	)
}

// Unwrap preserves existing orchestration reason codes for callers that use
// errors.As while retaining the document location needed to repair the YAML.
func (e *PlanDocumentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type planYAMLField struct {
	value *yaml.Node
}

type parsedPlanItem struct {
	draft  PlanWorkItemDraft
	node   *yaml.Node
	fields map[string]planYAMLField
}

var planDocumentFields = map[string]struct{}{
	"nexus_plan":            {},
	"operation":             {},
	"objective":             {},
	"completion_criteria":   {},
	"revision_reason":       {},
	"supersede_active_work": {},
	"replacement_reason":    {},
	"items":                 {},
}

var planDocumentItemFields = map[string]struct{}{
	"logical_key":           {},
	"existing_work_item_id": {},
	"kind":                  {},
	"subject":               {},
	"objective":             {},
	"deliverable":           {},
	"acceptance_criteria":   {},
	"required":              {},
	"terminal":              {},
	"parent_logical_key":    {},
	"depends_on":            {},
	"soft_depends_on":       {},
	"input_refs":            {},
	"output_scopes":         {},
	"shared_output_scopes":  {},
}

// ParseExecutionPlanDocument decodes exactly one strict YAML document. The
// returned protocol document is the canonical typed value used for proposal
// digesting; its WorkGraph arrays no longer retain YAML key or formatting order.
func ParseExecutionPlanDocument(
	source string,
) (protocol.ExecutionPlanProposalDocument, PlanDraft, error) {
	if len(source) > maxExecutionPlanDocumentBytes {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			nil,
			"$",
			fmt.Sprintf("document exceeds the %d-byte limit", maxExecutionPlanDocumentBytes),
			nil,
		)
	}
	if !utf8.ValidString(source) {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			nil,
			"$",
			"document must be valid UTF-8",
			nil,
		)
	}
	if strings.TrimSpace(source) == "" {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			nil,
			"$",
			"document must not be empty",
			nil,
		)
	}

	root, err := decodeSinglePlanYAML(source)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	nodeCount := 0
	if err = validatePlanYAMLNode(root, "$", 0, &nodeCount); err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	if root.Kind != yaml.MappingNode {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			root,
			"$",
			"root must be a mapping",
			nil,
		)
	}

	fields, err := collectPlanYAMLFields(root, "$", planDocumentFields)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	versionNode, err := requirePlanYAMLField(fields, root, "$", "nexus_plan")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	version, err := parsePlanYAMLInteger(versionNode, "$.nexus_plan")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	if version != protocol.ExecutionPlanProposalDocumentVersion {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			versionNode,
			"$.nexus_plan",
			fmt.Sprintf(
				"unsupported version %d; expected %d",
				version,
				protocol.ExecutionPlanProposalDocumentVersion,
			),
			nil,
		)
	}

	operationNode, err := requirePlanYAMLField(fields, root, "$", "operation")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	operationText, err := parsePlanYAMLString(operationNode, "$.operation", true)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	operation := protocol.ExecutionPlanProposalOperation(operationText)
	switch operation {
	case protocol.ExecutionPlanProposalCreate,
		protocol.ExecutionPlanProposalReplan,
		protocol.ExecutionPlanProposalReplace:
	default:
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, newPlanDocumentError(
			operationNode,
			"$.operation",
			"operation must be create, replan, or replace",
			nil,
		)
	}

	objective, err := parseOptionalPlanYAMLString(fields, "objective", "$.objective")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	completionCriteria, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"completion_criteria",
		"$.completion_criteria",
	)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	revisionReason, err := parseOptionalPlanYAMLString(fields, "revision_reason", "$.revision_reason")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	supersedeActiveWork, err := parseOptionalPlanYAMLBool(
		fields,
		"supersede_active_work",
		"$.supersede_active_work",
	)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	replacementReason, err := parseOptionalPlanYAMLString(
		fields,
		"replacement_reason",
		"$.replacement_reason",
	)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}

	itemsNode, err := requirePlanYAMLField(fields, root, "$", "items")
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	parsedItems, err := parsePlanDocumentItems(itemsNode)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, err
	}
	draft := PlanDraft{
		RevisionReason: revisionReason,
		Items:          make([]PlanWorkItemDraft, len(parsedItems)),
	}
	for index := range parsedItems {
		draft.Items[index] = parsedItems[index].draft
	}
	normalizedDraft, err := NormalizeAndValidatePlanDraft(draft)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, PlanDraft{}, planDraftDocumentError(
			err,
			itemsNode,
			parsedItems,
		)
	}
	canonicalizePlanDraft(&normalizedDraft)

	document := protocol.ExecutionPlanProposalDocument{
		Version:             version,
		Operation:           operation,
		Objective:           strings.TrimSpace(objective),
		CompletionCriteria:  cloneOptionalStrings(completionCriteria),
		RevisionReason:      normalizedDraft.RevisionReason,
		SupersedeActiveWork: supersedeActiveWork,
		ReplacementReason:   strings.TrimSpace(replacementReason),
		Items:               proposalItemsFromPlanDraft(normalizedDraft),
	}
	return document, normalizedDraft, nil
}

func decodeSinglePlanYAML(source string) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(source)))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, planYAMLSyntaxError(err)
	}
	if len(document.Content) != 1 {
		return nil, newPlanDocumentError(nil, "$", "document has no root value", nil)
	}
	var trailing yaml.Node
	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, planYAMLSyntaxError(err)
		}
		node := &trailing
		if len(trailing.Content) > 0 {
			node = trailing.Content[0]
		}
		return nil, newPlanDocumentError(node, "$", "multiple YAML documents are not allowed", nil)
	}
	return document.Content[0], nil
}

func planYAMLSyntaxError(err error) error {
	line := 1
	if matches := yamlSyntaxLinePattern.FindStringSubmatch(err.Error()); len(matches) == 2 {
		if parsed, parseErr := strconv.Atoi(matches[1]); parseErr == nil && parsed > 0 {
			line = parsed
		}
	}
	return &PlanDocumentError{
		Path:    "$",
		Line:    line,
		Column:  1,
		Message: "invalid YAML: " + err.Error(),
		Cause:   err,
	}
}

func validatePlanYAMLNode(node *yaml.Node, path string, depth int, count *int) error {
	if node == nil {
		return newPlanDocumentError(nil, path, "missing YAML node", nil)
	}
	(*count)++
	if *count > maxExecutionPlanYAMLNodes {
		return newPlanDocumentError(
			node,
			path,
			fmt.Sprintf("YAML node count exceeds the limit of %d", maxExecutionPlanYAMLNodes),
			nil,
		)
	}
	if depth > maxExecutionPlanYAMLDepth {
		return newPlanDocumentError(
			node,
			path,
			fmt.Sprintf("YAML nesting depth exceeds the limit of %d", maxExecutionPlanYAMLDepth),
			nil,
		)
	}
	if node.Anchor != "" {
		return newPlanDocumentError(node, path, "YAML anchors are not allowed", nil)
	}
	if node.Kind == yaml.AliasNode {
		return newPlanDocumentError(node, path, "YAML aliases are not allowed", nil)
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return newPlanDocumentError(node, path, "explicit YAML tags are not allowed", nil)
	}
	if node.Tag == "!!merge" {
		return newPlanDocumentError(node, path, "YAML merge keys are not allowed", nil)
	}
	switch node.Tag {
	case "!!null":
		return newPlanDocumentError(node, path, "null values are not allowed", nil)
	case "!!timestamp":
		return newPlanDocumentError(node, path, "timestamp values are not allowed", nil)
	}

	switch node.Kind {
	case yaml.MappingNode:
		if node.Tag != "!!map" {
			return newPlanDocumentError(node, path, "mapping must use the standard YAML map type", nil)
		}
		if len(node.Content)%2 != 0 {
			return newPlanDocumentError(node, path, "mapping contains an incomplete key/value pair", nil)
		}
		if err := validatePlanCollectionLimit(node, path, len(node.Content)/2); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			keyPath := appendPlanPath(path, key.Value)
			if err := validatePlanYAMLNode(key, keyPath, depth+1, count); err != nil {
				return err
			}
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return newPlanDocumentError(key, keyPath, "mapping keys must be strings", nil)
			}
			if _, exists := seen[key.Value]; exists {
				return newPlanDocumentError(key, keyPath, "duplicate mapping key", nil)
			}
			seen[key.Value] = struct{}{}
			if err := validatePlanYAMLNode(node.Content[index+1], keyPath, depth+1, count); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		if node.Tag != "!!seq" {
			return newPlanDocumentError(node, path, "sequence must use the standard YAML sequence type", nil)
		}
		if err := validatePlanCollectionLimit(node, path, len(node.Content)); err != nil {
			return err
		}
		for index, child := range node.Content {
			if err := validatePlanYAMLNode(
				child,
				fmt.Sprintf("%s[%d]", path, index),
				depth+1,
				count,
			); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str", "!!int", "!!bool":
			return nil
		default:
			return newPlanDocumentError(
				node,
				path,
				fmt.Sprintf("scalar type %s is not allowed", node.ShortTag()),
				nil,
			)
		}
	default:
		return newPlanDocumentError(node, path, "unsupported YAML node type", nil)
	}
	return nil
}

func validatePlanCollectionLimit(node *yaml.Node, path string, count int) error {
	if count <= protocol.ExecutionProjectionCollectionLimit {
		return nil
	}
	return newPlanDocumentError(
		node,
		path,
		fmt.Sprintf(
			"collection has %d members; maximum is %d",
			count,
			protocol.ExecutionProjectionCollectionLimit,
		),
		nil,
	)
}

func collectPlanYAMLFields(
	node *yaml.Node,
	path string,
	allowed map[string]struct{},
) (map[string]planYAMLField, error) {
	fields := make(map[string]planYAMLField, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		if _, ok := allowed[key.Value]; !ok {
			return nil, newPlanDocumentError(
				key,
				appendPlanPath(path, key.Value),
				"unknown field",
				nil,
			)
		}
		fields[key.Value] = planYAMLField{value: node.Content[index+1]}
	}
	return fields, nil
}

func requirePlanYAMLField(
	fields map[string]planYAMLField,
	parent *yaml.Node,
	parentPath string,
	name string,
) (*yaml.Node, error) {
	field, ok := fields[name]
	if !ok {
		return nil, newPlanDocumentError(
			parent,
			appendPlanPath(parentPath, name),
			"required field is missing",
			nil,
		)
	}
	return field.value, nil
}

func parsePlanYAMLInteger(node *yaml.Node, path string) (int, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, newPlanDocumentError(node, path, "must be an integer", nil)
	}
	var value int
	if err := node.Decode(&value); err != nil {
		return 0, newPlanDocumentError(node, path, "integer is out of range", err)
	}
	return value, nil
}

func parsePlanYAMLString(node *yaml.Node, path string, required bool) (string, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", newPlanDocumentError(node, path, "must be a string", nil)
	}
	value := strings.TrimSpace(node.Value)
	if required && value == "" {
		return "", newPlanDocumentError(node, path, "must not be blank", nil)
	}
	return value, nil
}

func parseOptionalPlanYAMLString(
	fields map[string]planYAMLField,
	name string,
	path string,
) (string, error) {
	field, ok := fields[name]
	if !ok {
		return "", nil
	}
	return parsePlanYAMLString(field.value, path, false)
}

func parseOptionalPlanYAMLBool(
	fields map[string]planYAMLField,
	name string,
	path string,
) (bool, error) {
	field, ok := fields[name]
	if !ok {
		return false, nil
	}
	if field.value.Kind != yaml.ScalarNode || field.value.Tag != "!!bool" {
		return false, newPlanDocumentError(field.value, path, "must be a boolean", nil)
	}
	var value bool
	if err := field.value.Decode(&value); err != nil {
		return false, newPlanDocumentError(field.value, path, "invalid boolean", err)
	}
	return value, nil
}

func parseOptionalPlanYAMLStringSequence(
	fields map[string]planYAMLField,
	name string,
	path string,
) ([]string, bool, error) {
	field, ok := fields[name]
	if !ok {
		return nil, false, nil
	}
	values, err := parsePlanYAMLStringSequence(field.value, path)
	return values, true, err
}

func parsePlanYAMLStringSequence(node *yaml.Node, path string) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, newPlanDocumentError(node, path, "must be a sequence of strings", nil)
	}
	values := make([]string, len(node.Content))
	for index, child := range node.Content {
		value, err := parsePlanYAMLString(child, fmt.Sprintf("%s[%d]", path, index), true)
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

func parsePlanDocumentItems(node *yaml.Node) ([]parsedPlanItem, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, newPlanDocumentError(node, "$.items", "must be a sequence of Work Item mappings", nil)
	}
	items := make([]parsedPlanItem, len(node.Content))
	for index, itemNode := range node.Content {
		itemPath := fmt.Sprintf("$.items[%d]", index)
		if itemNode.Kind != yaml.MappingNode {
			return nil, newPlanDocumentError(itemNode, itemPath, "must be a Work Item mapping", nil)
		}
		fields, err := collectPlanYAMLFields(itemNode, itemPath, planDocumentItemFields)
		if err != nil {
			return nil, err
		}
		item, err := parsePlanDocumentItem(fields, itemNode, itemPath)
		if err != nil {
			return nil, err
		}
		items[index] = parsedPlanItem{draft: item, node: itemNode, fields: fields}
	}
	return items, nil
}

func parsePlanDocumentItem(
	fields map[string]planYAMLField,
	node *yaml.Node,
	path string,
) (PlanWorkItemDraft, error) {
	logicalKey, err := parseRequiredPlanYAMLString(fields, node, path, "logical_key")
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	kind, err := parseRequiredPlanYAMLString(fields, node, path, "kind")
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	subject, err := parseRequiredPlanYAMLString(fields, node, path, "subject")
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	objective, err := parseRequiredPlanYAMLString(fields, node, path, "objective")
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	deliverable, err := parseRequiredPlanYAMLString(fields, node, path, "deliverable")
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	existingWorkItemID, err := parseOptionalPlanYAMLString(
		fields,
		"existing_work_item_id",
		appendPlanPath(path, "existing_work_item_id"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	acceptanceCriteria, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"acceptance_criteria",
		appendPlanPath(path, "acceptance_criteria"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	required, err := parseOptionalPlanYAMLBool(fields, "required", appendPlanPath(path, "required"))
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	terminal, err := parseOptionalPlanYAMLBool(fields, "terminal", appendPlanPath(path, "terminal"))
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	parentLogicalKey, err := parseOptionalPlanYAMLString(
		fields,
		"parent_logical_key",
		appendPlanPath(path, "parent_logical_key"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	dependsOn, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"depends_on",
		appendPlanPath(path, "depends_on"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	softDependsOn, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"soft_depends_on",
		appendPlanPath(path, "soft_depends_on"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	if len(dependsOn)+len(softDependsOn) > protocol.ExecutionProjectionCollectionLimit {
		field := fields["soft_depends_on"].value
		return PlanWorkItemDraft{}, newPlanDocumentError(
			field,
			appendPlanPath(path, "depends_on"),
			fmt.Sprintf(
				"combined hard and soft dependencies have %d members; maximum is %d",
				len(dependsOn)+len(softDependsOn),
				protocol.ExecutionProjectionCollectionLimit,
			),
			nil,
		)
	}
	inputRefs, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"input_refs",
		appendPlanPath(path, "input_refs"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	outputScopes, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"output_scopes",
		appendPlanPath(path, "output_scopes"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	sharedOutputScopes, _, err := parseOptionalPlanYAMLStringSequence(
		fields,
		"shared_output_scopes",
		appendPlanPath(path, "shared_output_scopes"),
	)
	if err != nil {
		return PlanWorkItemDraft{}, err
	}
	if len(outputScopes)+len(sharedOutputScopes) > protocol.ExecutionProjectionCollectionLimit {
		field := fields["shared_output_scopes"].value
		return PlanWorkItemDraft{}, newPlanDocumentError(
			field,
			appendPlanPath(path, "output_scopes"),
			fmt.Sprintf(
				"combined exclusive and shared output scopes have %d members; maximum is %d",
				len(outputScopes)+len(sharedOutputScopes),
				protocol.ExecutionProjectionCollectionLimit,
			),
			nil,
		)
	}

	dependencies := make([]PlanDependencyDraft, 0, len(dependsOn)+len(softDependsOn))
	for _, dependency := range dependsOn {
		dependencies = append(dependencies, PlanDependencyDraft{
			LogicalKey: dependency,
			Kind:       protocol.WorkDependencyHard,
		})
	}
	for _, dependency := range softDependsOn {
		dependencies = append(dependencies, PlanDependencyDraft{
			LogicalKey: dependency,
			Kind:       protocol.WorkDependencySoft,
		})
	}
	scopes := make([]protocol.WorkOutputScope, 0, len(outputScopes)+len(sharedOutputScopes))
	for _, scope := range outputScopes {
		scopes = append(scopes, protocol.WorkOutputScope{
			Scope: scope,
			Mode:  protocol.WorkOutputScopeExclusive,
		})
	}
	for _, scope := range sharedOutputScopes {
		scopes = append(scopes, protocol.WorkOutputScope{
			Scope: scope,
			Mode:  protocol.WorkOutputScopeShared,
		})
	}
	return PlanWorkItemDraft{
		LogicalKey:         logicalKey,
		ExistingWorkItemID: existingWorkItemID,
		Kind:               protocol.WorkItemKind(kind),
		Subject:            subject,
		Objective:          objective,
		Deliverable:        deliverable,
		AcceptanceCriteria: cloneStrings(acceptanceCriteria),
		Required:           required,
		Terminal:           terminal,
		ParentLogicalKey:   parentLogicalKey,
		DependsOn:          dependencies,
		InputRefs:          cloneStrings(inputRefs),
		OutputScopes:       scopes,
	}, nil
}

func parseRequiredPlanYAMLString(
	fields map[string]planYAMLField,
	parent *yaml.Node,
	parentPath string,
	name string,
) (string, error) {
	node, err := requirePlanYAMLField(fields, parent, parentPath, name)
	if err != nil {
		return "", err
	}
	return parsePlanYAMLString(node, appendPlanPath(parentPath, name), true)
}

func canonicalizePlanDraft(draft *PlanDraft) {
	for index := range draft.Items {
		item := &draft.Items[index]
		slices.SortFunc(item.DependsOn, func(left, right PlanDependencyDraft) int {
			if compared := strings.Compare(left.LogicalKey, right.LogicalKey); compared != 0 {
				return compared
			}
			return strings.Compare(string(left.Kind), string(right.Kind))
		})
		slices.SortFunc(item.OutputScopes, func(left, right protocol.WorkOutputScope) int {
			if compared := strings.Compare(left.Scope, right.Scope); compared != 0 {
				return compared
			}
			return strings.Compare(string(left.Mode), string(right.Mode))
		})
	}
}

func proposalItemsFromPlanDraft(draft PlanDraft) []protocol.ExecutionPlanProposalItem {
	items := make([]protocol.ExecutionPlanProposalItem, len(draft.Items))
	for index, draftItem := range draft.Items {
		dependencies := make([]protocol.ExecutionPlanProposalDependency, len(draftItem.DependsOn))
		for dependencyIndex, dependency := range draftItem.DependsOn {
			dependencies[dependencyIndex] = protocol.ExecutionPlanProposalDependency{
				LogicalKey: dependency.LogicalKey,
				Kind:       dependency.Kind,
			}
		}
		items[index] = protocol.ExecutionPlanProposalItem{
			LogicalKey:         draftItem.LogicalKey,
			ExistingWorkItemID: draftItem.ExistingWorkItemID,
			Kind:               draftItem.Kind,
			Subject:            draftItem.Subject,
			Objective:          draftItem.Objective,
			Deliverable:        draftItem.Deliverable,
			AcceptanceCriteria: cloneStrings(draftItem.AcceptanceCriteria),
			Required:           draftItem.Required,
			Terminal:           draftItem.Terminal,
			ParentLogicalKey:   draftItem.ParentLogicalKey,
			DependsOn:          dependencies,
			InputRefs:          cloneStrings(draftItem.InputRefs),
			OutputScopes:       slices.Clone(draftItem.OutputScopes),
		}
	}
	return items
}

func planDraftDocumentError(
	err error,
	itemsNode *yaml.Node,
	items []parsedPlanItem,
) error {
	path := "$.items"
	node := itemsNode
	var domainErr *DomainError
	if !errors.As(err, &domainErr) {
		return newPlanDocumentError(node, path, err.Error(), err)
	}
	itemIndex := -1
	for index := range items {
		if strings.TrimSpace(items[index].draft.LogicalKey) == domainErr.WorkItemKey {
			itemIndex = index
		}
	}
	if itemIndex < 0 {
		return localizedPlanDraftDocumentError(node, path, domainErr)
	}
	item := items[itemIndex]
	path = fmt.Sprintf("$.items[%d]", itemIndex)
	node = item.node
	fieldName := planDraftErrorField(domainErr, item)
	if fieldName != "" {
		path = appendPlanPath(path, fieldName)
		if field, ok := item.fields[fieldName]; ok {
			node = field.value
		}
	}
	return localizedPlanDraftDocumentError(node, path, domainErr)
}

// localizedPlanDraftDocumentError keeps the domain reason code discoverable
// while ensuring callers that project DomainError.Message do not discard the
// YAML repair location carried by the outer error.
func localizedPlanDraftDocumentError(
	node *yaml.Node,
	path string,
	domainErr *DomainError,
) error {
	located := newPlanDocumentError(node, path, domainErr.Error(), nil)
	located.Cause = &DomainError{
		Code:        domainErr.Code,
		Message:     located.Error(),
		WorkItemKey: domainErr.WorkItemKey,
		RelatedKey:  domainErr.RelatedKey,
	}
	return located
}

func planDraftErrorField(domainErr *DomainError, item parsedPlanItem) string {
	if domainErr == nil {
		return ""
	}
	switch domainErr.Code {
	case ErrorCodeDuplicateLogicalKey:
		return "logical_key"
	case ErrorCodeAcceptanceCriteriaEmpty:
		return "acceptance_criteria"
	case ErrorCodeOutputScopeConflict:
		return "output_scopes"
	case ErrorCodeUnknownDependency, ErrorCodeDependencyCycle:
		if strings.TrimSpace(item.draft.ParentLogicalKey) == domainErr.RelatedKey {
			return "parent_logical_key"
		}
		if planYAMLSequenceContains(item.fields["soft_depends_on"].value, domainErr.RelatedKey) {
			return "soft_depends_on"
		}
		return "depends_on"
	case ErrorCodeProjectionLimitExceeded:
		return domainErr.RelatedKey
	case ErrorCodeInvalidInput:
		switch {
		case strings.Contains(domainErr.Message, "work item kind"):
			return "kind"
		case strings.Contains(domainErr.Message, "subject, objective and deliverable"):
			return "subject"
		case strings.Contains(domainErr.Message, "logical_key"):
			return "logical_key"
		case strings.Contains(domainErr.Message, "dependency"):
			return "depends_on"
		case strings.Contains(domainErr.Message, "output scope"),
			strings.Contains(domainErr.Message, "path"),
			strings.Contains(domainErr.Message, "semantic"):
			return "output_scopes"
		}
	}
	return ""
}

func planYAMLSequenceContains(node *yaml.Node, value string) bool {
	if node == nil || node.Kind != yaml.SequenceNode {
		return false
	}
	for _, child := range node.Content {
		if strings.TrimSpace(child.Value) == value {
			return true
		}
	}
	return false
}

func appendPlanPath(parent, field string) string {
	if parent == "" {
		parent = "$"
	}
	return parent + "." + field
}

func newPlanDocumentError(node *yaml.Node, path, message string, cause error) *PlanDocumentError {
	line, column := 1, 1
	if node != nil {
		if node.Line > 0 {
			line = node.Line
		}
		if node.Column > 0 {
			column = node.Column
		}
	}
	if strings.TrimSpace(path) == "" {
		path = "$"
	}
	return &PlanDocumentError{
		Path:    path,
		Line:    line,
		Column:  column,
		Message: strings.TrimSpace(message),
		Cause:   cause,
	}
}

func cloneStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func cloneOptionalStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return cloneStrings(values)
}
