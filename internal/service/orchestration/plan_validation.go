// INPUT: 模型一次性提交的完整 Plan draft。
// OUTPUT: 规范化 WorkGraph 或带稳定 reason code 的拒绝。
// POS: Plan Document preparation/materialization 在生成持久 ID 和开启事务前的纯领域校验。
package orchestration

import (
	"regexp"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var planLogicalKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// PlanDraft 是一次 immutable Plan revision 的完整模型意图。
type PlanDraft struct {
	RevisionReason string
	Items          []PlanWorkItemDraft
}

// PlanWorkItemDraft 使用 logical key 表达本次 Plan 内的引用，服务端负责 mint opaque ID。
type PlanWorkItemDraft struct {
	LogicalKey         string
	ExistingWorkItemID string
	Kind               protocol.WorkItemKind
	Subject            string
	Objective          string
	Deliverable        string
	AcceptanceCriteria []string
	Required           bool
	Terminal           bool
	ParentLogicalKey   string
	DependsOn          []PlanDependencyDraft
	InputRefs          []string
	OutputScopes       []protocol.WorkOutputScope
}

// PlanDependencyDraft 是 logical-key dependency edge。
type PlanDependencyDraft struct {
	LogicalKey string
	Kind       protocol.WorkDependencyKind
}

// ValidatePlanDraft 检查结构完整性、DAG 与已声明 output scope 冲突。
func ValidatePlanDraft(draft PlanDraft) error {
	_, err := NormalizeAndValidatePlanDraft(draft)
	return err
}

// NormalizeAndValidatePlanDraft 返回服务端可安全持久化的规范化副本，不修改调用方输入。
func NormalizeAndValidatePlanDraft(draft PlanDraft) (PlanDraft, error) {
	if err := newProjectionLimitError("items", len(draft.Items), ""); err != nil {
		return PlanDraft{}, err
	}
	normalized := PlanDraft{
		RevisionReason: strings.TrimSpace(draft.RevisionReason),
		Items:          make([]PlanWorkItemDraft, len(draft.Items)),
	}
	for index, raw := range draft.Items {
		for _, collection := range []struct {
			field string
			count int
		}{
			{field: "acceptance_criteria", count: len(raw.AcceptanceCriteria)},
			{field: "depends_on", count: len(raw.DependsOn)},
			{field: "input_refs", count: len(raw.InputRefs)},
			{field: "output_scopes", count: len(raw.OutputScopes)},
		} {
			if err := newProjectionLimitError(
				collection.field,
				collection.count,
				strings.TrimSpace(raw.LogicalKey),
			); err != nil {
				return PlanDraft{}, err
			}
		}
		raw.DependsOn = slices.Clone(raw.DependsOn)
		raw.OutputScopes = slices.Clone(raw.OutputScopes)
		item, err := normalizePlanWorkItemDraft(raw)
		if err != nil {
			return PlanDraft{}, err
		}
		normalized.Items[index] = item
	}
	if err := validateNormalizedPlanDraft(normalized); err != nil {
		return PlanDraft{}, err
	}
	return normalized, nil
}

func validateNormalizedPlanDraft(draft PlanDraft) error {
	if len(draft.Items) == 0 {
		return newDomainError(
			ErrorCodePlanItemsEmpty,
			"Plan Document items must be a non-empty YAML sequence describing the complete WorkGraph; empty or placeholder entries are invalid",
			"",
			"",
		)
	}
	items := make(map[string]PlanWorkItemDraft, len(draft.Items))
	for _, item := range draft.Items {
		if planWorkItemDraftIsPlaceholder(item) {
			return newDomainError(
				ErrorCodePlanItemsEmpty,
				"Plan Document items must contain populated Work Items; empty mappings are invalid",
				"",
				"",
			)
		}
		if !planLogicalKeyPattern.MatchString(item.LogicalKey) {
			return newDomainError(
				ErrorCodeInvalidInput,
				"logical_key must start with an ASCII letter and contain only letters, digits, _ or -",
				item.LogicalKey,
				"",
			)
		}
		if _, exists := items[item.LogicalKey]; exists {
			return newDomainError(
				ErrorCodeDuplicateLogicalKey,
				"logical_key must be unique within one Plan revision",
				item.LogicalKey,
				"",
			)
		}
		if !validWorkItemKind(item.Kind) {
			return newDomainError(
				ErrorCodeInvalidInput,
				"unknown work item kind; expected produce, review, verify, or integrate",
				item.LogicalKey,
				"",
			)
		}
		if item.Subject == "" || item.Objective == "" || item.Deliverable == "" {
			return newDomainError(
				ErrorCodeInvalidInput,
				"subject, objective and deliverable are required",
				item.LogicalKey,
				"",
			)
		}
		if slices.Contains(item.AcceptanceCriteria, "") {
			return newDomainError(
				ErrorCodeAcceptanceCriteriaEmpty,
				"acceptance criteria must be non-empty when provided",
				item.LogicalKey,
				"",
			)
		}
		items[item.LogicalKey] = item
	}

	graph := make(map[string][]string, len(items))
	for _, item := range draft.Items {
		if item.ParentLogicalKey != "" {
			if _, exists := items[item.ParentLogicalKey]; !exists {
				return newDomainError(
					ErrorCodeUnknownDependency,
					"parent logical_key does not exist in this Plan",
					item.LogicalKey,
					item.ParentLogicalKey,
				)
			}
			if item.ParentLogicalKey == item.LogicalKey {
				return newDomainError(
					ErrorCodeDependencyCycle,
					"work item cannot be its own parent",
					item.LogicalKey,
					item.ParentLogicalKey,
				)
			}
		}
		seenDependencies := make(map[string]struct{}, len(item.DependsOn))
		for _, dependency := range item.DependsOn {
			if _, exists := items[dependency.LogicalKey]; !exists {
				return newDomainError(
					ErrorCodeUnknownDependency,
					"dependency logical_key does not exist in this Plan",
					item.LogicalKey,
					dependency.LogicalKey,
				)
			}
			if _, duplicate := seenDependencies[dependency.LogicalKey]; duplicate {
				return newDomainError(
					ErrorCodeInvalidInput,
					"dependency appears more than once",
					item.LogicalKey,
					dependency.LogicalKey,
				)
			}
			seenDependencies[dependency.LogicalKey] = struct{}{}
			graph[item.LogicalKey] = append(graph[item.LogicalKey], dependency.LogicalKey)
		}
	}
	if cycle := firstDependencyCycle(graph); len(cycle) > 0 {
		return newDomainError(
			ErrorCodeDependencyCycle,
			"dependency graph contains a cycle: "+strings.Join(cycle, " -> "),
			cycle[0],
			cycle[len(cycle)-1],
		)
	}
	if err := validateOutputScopes(draft.Items); err != nil {
		return err
	}
	return nil
}

func planWorkItemDraftIsPlaceholder(item PlanWorkItemDraft) bool {
	return item.LogicalKey == "" &&
		item.ExistingWorkItemID == "" &&
		item.Kind == "" &&
		item.Subject == "" &&
		item.Objective == "" &&
		item.Deliverable == "" &&
		len(item.AcceptanceCriteria) == 0 &&
		!item.Required &&
		!item.Terminal &&
		item.ParentLogicalKey == "" &&
		len(item.DependsOn) == 0 &&
		len(item.InputRefs) == 0 &&
		len(item.OutputScopes) == 0
}

func normalizePlanWorkItemDraft(item PlanWorkItemDraft) (PlanWorkItemDraft, error) {
	item.LogicalKey = strings.TrimSpace(item.LogicalKey)
	item.ExistingWorkItemID = strings.TrimSpace(item.ExistingWorkItemID)
	item.Subject = strings.TrimSpace(item.Subject)
	item.Objective = strings.TrimSpace(item.Objective)
	item.Deliverable = strings.TrimSpace(item.Deliverable)
	item.ParentLogicalKey = strings.TrimSpace(item.ParentLogicalKey)
	item.AcceptanceCriteria = normalizeNonEmptyStrings(item.AcceptanceCriteria)
	item.InputRefs = normalizeNonEmptyStrings(item.InputRefs)
	for index := range item.DependsOn {
		item.DependsOn[index].LogicalKey = strings.TrimSpace(item.DependsOn[index].LogicalKey)
		if item.DependsOn[index].Kind == "" {
			item.DependsOn[index].Kind = protocol.WorkDependencyHard
		}
	}
	for index := range item.OutputScopes {
		normalized, err := protocol.NormalizeWorkOutputScope(item.OutputScopes[index])
		if err != nil {
			return PlanWorkItemDraft{}, newDomainError(
				ErrorCodeInvalidInput,
				err.Error(),
				item.LogicalKey,
				item.OutputScopes[index].Scope,
			)
		}
		item.OutputScopes[index] = normalized
	}
	return item, nil
}

func normalizeNonEmptyStrings(input []string) []string {
	result := make([]string, 0, len(input))
	for _, value := range input {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func validWorkItemKind(kind protocol.WorkItemKind) bool {
	switch kind {
	case protocol.WorkItemKindProduce,
		protocol.WorkItemKindReview,
		protocol.WorkItemKindVerify,
		protocol.WorkItemKindIntegrate:
		return true
	default:
		return false
	}
}

func firstDependencyCycle(graph map[string][]string) []string {
	const (
		unseen   = 0
		visiting = 1
		visited  = 2
	)
	state := make(map[string]int, len(graph))
	stack := make([]string, 0, len(graph))
	var visit func(string) []string
	visit = func(node string) []string {
		switch state[node] {
		case visited:
			return nil
		case visiting:
			index := slices.Index(stack, node)
			if index < 0 {
				return []string{node, node}
			}
			return append(append([]string(nil), stack[index:]...), node)
		}
		state[node] = visiting
		stack = append(stack, node)
		dependencies := append([]string(nil), graph[node]...)
		slices.Sort(dependencies)
		for _, dependency := range dependencies {
			if cycle := visit(dependency); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = visited
		return nil
	}
	nodes := make([]string, 0, len(graph))
	for node := range graph {
		nodes = append(nodes, node)
	}
	slices.Sort(nodes)
	for _, node := range nodes {
		if cycle := visit(node); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func validateOutputScopes(items []PlanWorkItemDraft) error {
	type claim struct {
		logicalKey string
		scope      protocol.WorkOutputScope
	}
	hardDependencies := make(map[string][]string, len(items))
	for _, item := range items {
		for _, dependency := range item.DependsOn {
			if dependency.Kind == protocol.WorkDependencyHard {
				hardDependencies[item.LogicalKey] = append(
					hardDependencies[item.LogicalKey],
					dependency.LogicalKey,
				)
			}
		}
	}
	claims := make([]claim, 0)
	for _, item := range items {
		seen := make(map[string]struct{}, len(item.OutputScopes))
		for _, scope := range item.OutputScopes {
			comparisonKey, keyErr := protocol.WorkOutputScopeComparisonKey(scope)
			if keyErr != nil {
				return newDomainError(
					ErrorCodeInvalidInput,
					keyErr.Error(),
					item.LogicalKey,
					scope.Scope,
				)
			}
			if _, duplicate := seen[comparisonKey]; duplicate {
				return newDomainError(
					ErrorCodeInvalidInput,
					"output scope appears more than once on the same Work Item",
					item.LogicalKey,
					scope.Scope,
				)
			}
			seen[comparisonKey] = struct{}{}
			for _, existing := range claims {
				if existing.logicalKey == item.LogicalKey {
					continue
				}
				conflict, err := protocol.WorkOutputClaimsConflict(
					item.LogicalKey,
					scope,
					existing.logicalKey,
					existing.scope,
					hardDependencies,
				)
				if err != nil {
					return newDomainError(
						ErrorCodeInvalidInput,
						err.Error(),
						item.LogicalKey,
						scope.Scope,
					)
				}
				if conflict {
					return newDomainError(
						ErrorCodeOutputScopeConflict,
						"Work Items cannot overlap an exclusive output scope",
						item.LogicalKey,
						existing.logicalKey,
					)
				}
			}
			claims = append(claims, claim{
				logicalKey: item.LogicalKey,
				scope:      scope,
			})
		}
	}
	return nil
}
