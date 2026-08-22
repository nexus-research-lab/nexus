// INPUT: typed ExecutionPlanProposal immutable envelope and complete logical-key WorkGraph document.
// OUTPUT: canonical sealed proposal or a storage-boundary invariant rejection.
// POS: proposal creation's single normalization/validation boundary; materialization never reparses model JSON.
package orchestration

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const proposalCollectionLimit = 32

var proposalLogicalKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

func normalizeAndValidatePlanProposal(
	input protocol.ExecutionPlanProposal,
	now time.Time,
) (protocol.ExecutionPlanProposal, string, error) {
	item := input.Normalized()
	if item.RootRoundID == "" {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: sealed proposal requires root round provenance",
			ErrInvariant,
		)
	}
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{name: "root round id", value: item.RootRoundID, limit: 128},
		{name: "runtime round id", value: item.RuntimeRoundID, limit: 128},
		{name: "agent round id", value: item.AgentRoundID, limit: 128},
		{name: "target execution id", value: item.TargetExecutionID, limit: 64},
		{name: "base plan id", value: item.BasePlanID, limit: 64},
		{name: "Goal id", value: item.GoalID, limit: 64},
		{name: "Goal-reserved execution id", value: item.GoalReservedExecutionID, limit: 64},
		{name: "replaces execution id", value: item.ReplacesExecutionID, limit: 64},
	} {
		if widthErr := validatePlanProposalStringWidth(field.name, field.value, field.limit); widthErr != nil {
			return protocol.ExecutionPlanProposal{}, "", widthErr
		}
	}

	access, err := normalizePlanProposalAccess(planProposalAccessFor(item))
	if err != nil {
		return protocol.ExecutionPlanProposal{}, "", err
	}
	item.ID = access.ProposalID
	item.OwnerUserID = access.OwnerUserID
	item.SessionKey = access.SessionKey
	item.ScopeKind = access.ScopeKind
	item.RoomID = access.RoomID
	item.ConversationID = access.ConversationID
	item.CoordinatorAgentID = access.CoordinatorAgentID

	if item.TargetExecutionVersion < 0 || item.GoalObjectiveRevision < 0 {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: proposal target and Goal revisions cannot be negative",
			ErrInvariant,
		)
	}
	if err := validateSealedProposalGoal(item); err != nil {
		return protocol.ExecutionPlanProposal{}, "", err
	}
	document, err := normalizeAndValidateProposalDocument(item.Document)
	if err != nil {
		return protocol.ExecutionPlanProposal{}, "", err
	}
	item.Document = document
	if err := validateProposalOperationFence(item); err != nil {
		return protocol.ExecutionPlanProposal{}, "", err
	}

	if item.Status == "" {
		item.Status = protocol.ExecutionPlanProposalStatusSealed
	}
	if item.Status != protocol.ExecutionPlanProposalStatusSealed {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: a new proposal must be sealed",
			ErrInvariant,
		)
	}
	if item.Version == 0 {
		item.Version = 1
	}
	if item.Version != 1 {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: a new proposal must have version 1",
			ErrInvariant,
		)
	}
	if item.ConfirmationState == "" {
		item.ConfirmationState = protocol.ExecutionPlanProposalConfirmationNone
	}
	if item.ConfirmationState != protocol.ExecutionPlanProposalConfirmationNone ||
		item.AttemptCount != 0 || item.NextAttemptAt != nil || item.LastError != "" ||
		item.ReservedExecutionID != "" || item.MaterializationCommandID != "" ||
		item.MaterializedExecutionID != "" || item.MaterializedPlanID != "" ||
		item.MaterializedAt != nil {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: a sealed proposal cannot carry lifecycle or materialization receipt state",
			ErrInvariant,
		)
	}
	// Creation time is repository evidence, never caller-controlled proposal content.
	item.CreatedAt = now.UTC()
	item.UpdatedAt = item.CreatedAt
	digest, err := protocol.DigestExecutionPlanProposalImmutable(item)
	if err != nil {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf("digest immutable proposal: %w", err)
	}
	if item.ContentDigest != "" && item.ContentDigest != digest {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf(
			"%w: supplied proposal digest does not match the canonical immutable envelope",
			ErrInvariant,
		)
	}
	item.ContentDigest = digest
	documentJSON, err := protocol.MarshalExecutionPlanProposalDocument(item.Document)
	if err != nil {
		return protocol.ExecutionPlanProposal{}, "", fmt.Errorf("encode proposal document: %w", err)
	}
	return item, string(documentJSON), nil
}

func validateSealedProposalGoal(item protocol.ExecutionPlanProposal) error {
	if item.GoalID == "" {
		if item.GoalObjectiveRevision != 0 || item.GoalActivationOrigin != "" ||
			item.GoalActivationReason != "" || item.GoalReservedExecutionID != "" {
			return fmt.Errorf("%w: Goal-free proposal carries a partial Goal fence", ErrInvariant)
		}
		return nil
	}
	if item.GoalObjectiveRevision <= 0 {
		return fmt.Errorf("%w: Goal proposal requires a positive objective revision", ErrInvariant)
	}
	if item.GoalActivationOrigin == "" || item.GoalActivationReason == "" {
		return fmt.Errorf("%w: Goal proposal requires sealed activation origin and reason", ErrInvariant)
	}
	if !validProposalGoalActivationOrigin(item.GoalActivationOrigin) {
		return fmt.Errorf("%w: invalid Goal activation origin %q", ErrInvariant, item.GoalActivationOrigin)
	}
	if !validProposalGoalActivationReason(item.GoalActivationReason) {
		return fmt.Errorf("%w: invalid Goal activation reason %q", ErrInvariant, item.GoalActivationReason)
	}
	return nil
}

func validateProposalOperationFence(item protocol.ExecutionPlanProposal) error {
	switch item.Document.Operation {
	case protocol.ExecutionPlanProposalCreate:
		if item.TargetExecutionID != "" || item.TargetExecutionVersion != 0 || item.BasePlanID != "" ||
			item.Document.RevisionReason != "" || item.Document.ReplacementReason != "" ||
			item.Document.SupersedeActiveWork {
			return fmt.Errorf("%w: create proposal cannot carry a target or revision fence", ErrInvariant)
		}
		if item.GoalReservedExecutionID != "" && item.GoalID == "" {
			return fmt.Errorf("%w: Goal-free create cannot carry a reserved Execution", ErrInvariant)
		}
		if item.ReplacesExecutionID != "" &&
			(item.GoalReservedExecutionID == "" ||
				item.ReplacesExecutionID == item.GoalReservedExecutionID) {
			return fmt.Errorf(
				"%w: create predecessor requires a distinct Goal-reserved successor",
				ErrInvariant,
			)
		}
	case protocol.ExecutionPlanProposalReplan:
		if item.TargetExecutionID == "" || item.TargetExecutionVersion <= 0 ||
			item.Document.ReplacementReason != "" ||
			item.ReplacesExecutionID != "" || item.GoalReservedExecutionID != "" {
			return fmt.Errorf("%w: replan proposal requires an exact target/version/base Plan fence", ErrInvariant)
		}
		if item.Document.SupersedeActiveWork && item.Document.RevisionReason == "" {
			return fmt.Errorf("%w: superseding active work requires a revision reason", ErrInvariant)
		}
		if item.BasePlanID == "" {
			for _, work := range item.Document.Items {
				if work.ExistingWorkItemID != "" {
					return fmt.Errorf(
						"%w: first Plan for an existing Execution cannot reuse Work Item %q",
						ErrInvariant,
						work.ExistingWorkItemID,
					)
				}
			}
		}
	case protocol.ExecutionPlanProposalReplace:
		if item.TargetExecutionID == "" || item.TargetExecutionVersion <= 0 || item.BasePlanID == "" ||
			item.Document.ReplacementReason == "" || item.Document.RevisionReason != "" ||
			item.Document.SupersedeActiveWork || item.ReplacesExecutionID != item.TargetExecutionID ||
			item.GoalReservedExecutionID != "" {
			return fmt.Errorf("%w: replace proposal requires target/version/base/replacement reason only", ErrInvariant)
		}
	default:
		return fmt.Errorf("%w: unknown proposal operation %q", ErrInvariant, item.Document.Operation)
	}
	for _, work := range item.Document.Items {
		if item.Document.Operation != protocol.ExecutionPlanProposalReplan && work.ExistingWorkItemID != "" {
			return fmt.Errorf(
				"%w: %s proposal cannot reuse Work Item %q",
				ErrInvariant,
				item.Document.Operation,
				work.ExistingWorkItemID,
			)
		}
	}
	return nil
}

func normalizeAndValidateProposalDocument(
	input protocol.ExecutionPlanProposalDocument,
) (protocol.ExecutionPlanProposalDocument, error) {
	document := input
	if document.Version == 0 {
		document.Version = protocol.ExecutionPlanProposalDocumentVersion
	}
	if document.Version != protocol.ExecutionPlanProposalDocumentVersion {
		return protocol.ExecutionPlanProposalDocument{}, fmt.Errorf(
			"%w: unsupported nexus_plan version %d",
			ErrInvariant,
			document.Version,
		)
	}
	document.Objective = strings.TrimSpace(document.Objective)
	document.RevisionReason = strings.TrimSpace(document.RevisionReason)
	document.ReplacementReason = strings.TrimSpace(document.ReplacementReason)
	criteria, err := normalizeProposalStrings("completion_criteria", document.CompletionCriteria, true)
	if err != nil {
		return protocol.ExecutionPlanProposalDocument{}, err
	}
	document.CompletionCriteria = criteria
	if document.Objective == "" {
		return protocol.ExecutionPlanProposalDocument{}, fmt.Errorf("%w: proposal objective is required", ErrInvariant)
	}
	if len(document.Items) == 0 || len(document.Items) > proposalCollectionLimit {
		return protocol.ExecutionPlanProposalDocument{}, fmt.Errorf(
			"%w: proposal must contain 1..%d Work Items",
			ErrInvariant,
			proposalCollectionLimit,
		)
	}
	document.Items = append([]protocol.ExecutionPlanProposalItem(nil), document.Items...)
	keys := make(map[string]struct{}, len(document.Items))
	existingWorkItemIDs := make(map[string]string, len(document.Items))
	for index := range document.Items {
		item, normalizeErr := normalizeProposalItem(document.Items[index])
		if normalizeErr != nil {
			return protocol.ExecutionPlanProposalDocument{}, normalizeErr
		}
		if _, duplicate := keys[item.LogicalKey]; duplicate {
			return protocol.ExecutionPlanProposalDocument{}, fmt.Errorf(
				"%w: duplicate proposal logical_key %q",
				ErrInvariant,
				item.LogicalKey,
			)
		}
		keys[item.LogicalKey] = struct{}{}
		if item.ExistingWorkItemID != "" {
			if previous, duplicate := existingWorkItemIDs[item.ExistingWorkItemID]; duplicate {
				return protocol.ExecutionPlanProposalDocument{}, fmt.Errorf(
					"%w: Work Items %q and %q reuse existing_work_item_id %q",
					ErrInvariant,
					previous,
					item.LogicalKey,
					item.ExistingWorkItemID,
				)
			}
			existingWorkItemIDs[item.ExistingWorkItemID] = item.LogicalKey
		}
		document.Items[index] = item
	}
	if err := validateProposalReferences(document.Items, keys); err != nil {
		return protocol.ExecutionPlanProposalDocument{}, err
	}
	if err := validateProposalOutputScopes(document.Items); err != nil {
		return protocol.ExecutionPlanProposalDocument{}, err
	}
	return document, nil
}

func normalizeProposalItem(
	input protocol.ExecutionPlanProposalItem,
) (protocol.ExecutionPlanProposalItem, error) {
	item := input
	item.LogicalKey = strings.TrimSpace(item.LogicalKey)
	item.ExistingWorkItemID = strings.TrimSpace(item.ExistingWorkItemID)
	item.Subject = strings.TrimSpace(item.Subject)
	item.Objective = strings.TrimSpace(item.Objective)
	item.Deliverable = strings.TrimSpace(item.Deliverable)
	item.ParentLogicalKey = strings.TrimSpace(item.ParentLogicalKey)
	if err := validatePlanProposalStringWidth("existing Work Item id", item.ExistingWorkItemID, 64); err != nil {
		return protocol.ExecutionPlanProposalItem{}, err
	}
	if !proposalLogicalKeyPattern.MatchString(item.LogicalKey) {
		return protocol.ExecutionPlanProposalItem{}, fmt.Errorf(
			"%w: invalid proposal logical_key %q",
			ErrInvariant,
			item.LogicalKey,
		)
	}
	if !validProposalWorkItemKind(item.Kind) {
		return protocol.ExecutionPlanProposalItem{}, fmt.Errorf(
			"%w: invalid Work Item kind %q for %q",
			ErrInvariant,
			item.Kind,
			item.LogicalKey,
		)
	}
	if item.Subject == "" || item.Objective == "" || item.Deliverable == "" {
		return protocol.ExecutionPlanProposalItem{}, fmt.Errorf(
			"%w: subject, objective and deliverable are required for %q",
			ErrInvariant,
			item.LogicalKey,
		)
	}
	criteria, err := normalizeProposalStrings("acceptance_criteria", item.AcceptanceCriteria, false)
	if err != nil {
		return protocol.ExecutionPlanProposalItem{}, proposalItemError(item.LogicalKey, err)
	}
	inputRefs, err := normalizeProposalStrings("input_refs", item.InputRefs, false)
	if err != nil {
		return protocol.ExecutionPlanProposalItem{}, proposalItemError(item.LogicalKey, err)
	}
	item.AcceptanceCriteria = criteria
	item.InputRefs = inputRefs
	if len(item.DependsOn) > proposalCollectionLimit || len(item.OutputScopes) > proposalCollectionLimit {
		return protocol.ExecutionPlanProposalItem{}, fmt.Errorf(
			"%w: Work Item %q exceeds proposal collection limit %d",
			ErrInvariant,
			item.LogicalKey,
			proposalCollectionLimit,
		)
	}
	item.DependsOn = append([]protocol.ExecutionPlanProposalDependency(nil), item.DependsOn...)
	for index := range item.DependsOn {
		dependency := &item.DependsOn[index]
		dependency.LogicalKey = strings.TrimSpace(dependency.LogicalKey)
		if dependency.Kind == "" {
			dependency.Kind = protocol.WorkDependencyHard
		}
		if dependency.LogicalKey == "" || !validProposalDependencyKind(dependency.Kind) {
			return protocol.ExecutionPlanProposalItem{}, fmt.Errorf(
				"%w: invalid dependency on Work Item %q",
				ErrInvariant,
				item.LogicalKey,
			)
		}
	}
	sort.Slice(item.DependsOn, func(left, right int) bool {
		if item.DependsOn[left].LogicalKey != item.DependsOn[right].LogicalKey {
			return item.DependsOn[left].LogicalKey < item.DependsOn[right].LogicalKey
		}
		return item.DependsOn[left].Kind < item.DependsOn[right].Kind
	})
	item.OutputScopes = append([]protocol.WorkOutputScope(nil), item.OutputScopes...)
	for index := range item.OutputScopes {
		normalized, normalizeErr := protocol.NormalizeWorkOutputScope(item.OutputScopes[index])
		if normalizeErr != nil {
			return protocol.ExecutionPlanProposalItem{}, proposalItemError(item.LogicalKey, normalizeErr)
		}
		item.OutputScopes[index] = normalized
	}
	sort.Slice(item.OutputScopes, func(left, right int) bool {
		if item.OutputScopes[left].Scope != item.OutputScopes[right].Scope {
			return item.OutputScopes[left].Scope < item.OutputScopes[right].Scope
		}
		return item.OutputScopes[left].Mode < item.OutputScopes[right].Mode
	})
	return item, nil
}

func normalizeProposalStrings(field string, input []string, required bool) ([]string, error) {
	if len(input) > proposalCollectionLimit {
		return nil, fmt.Errorf("%w: %s exceeds limit %d", ErrInvariant, field, proposalCollectionLimit)
	}
	if required && len(input) == 0 {
		return nil, fmt.Errorf("%w: %s must not be empty", ErrInvariant, field)
	}
	result := make([]string, len(input))
	seen := make(map[string]struct{}, len(input))
	for index, raw := range input {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%w: %s contains an empty entry", ErrInvariant, field)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("%w: %s contains duplicate %q", ErrInvariant, field, value)
		}
		seen[value] = struct{}{}
		result[index] = value
	}
	return result, nil
}

func validateProposalReferences(
	items []protocol.ExecutionPlanProposalItem,
	keys map[string]struct{},
) error {
	dependencyGraph := make(map[string][]string, len(items))
	parentGraph := make(map[string][]string, len(items))
	for _, item := range items {
		if item.ParentLogicalKey != "" {
			if _, exists := keys[item.ParentLogicalKey]; !exists {
				return fmt.Errorf("%w: unknown parent %q for %q", ErrInvariant, item.ParentLogicalKey, item.LogicalKey)
			}
			if item.ParentLogicalKey == item.LogicalKey {
				return fmt.Errorf("%w: Work Item %q cannot parent itself", ErrInvariant, item.LogicalKey)
			}
			parentGraph[item.LogicalKey] = []string{item.ParentLogicalKey}
		}
		seen := make(map[string]struct{}, len(item.DependsOn))
		for _, dependency := range item.DependsOn {
			if _, exists := keys[dependency.LogicalKey]; !exists {
				return fmt.Errorf("%w: unknown dependency %q for %q", ErrInvariant, dependency.LogicalKey, item.LogicalKey)
			}
			if dependency.LogicalKey == item.LogicalKey {
				return fmt.Errorf("%w: Work Item %q cannot depend on itself", ErrInvariant, item.LogicalKey)
			}
			if _, duplicate := seen[dependency.LogicalKey]; duplicate {
				return fmt.Errorf("%w: duplicate dependency %q for %q", ErrInvariant, dependency.LogicalKey, item.LogicalKey)
			}
			seen[dependency.LogicalKey] = struct{}{}
			dependencyGraph[item.LogicalKey] = append(dependencyGraph[item.LogicalKey], dependency.LogicalKey)
		}
	}
	if cycle := firstProposalCycle(parentGraph, keys); len(cycle) > 0 {
		return fmt.Errorf("%w: proposal parent graph contains cycle %s", ErrInvariant, strings.Join(cycle, " -> "))
	}
	if cycle := firstProposalCycle(dependencyGraph, keys); len(cycle) > 0 {
		return fmt.Errorf("%w: proposal dependency graph contains cycle %s", ErrInvariant, strings.Join(cycle, " -> "))
	}
	return nil
}

func firstProposalCycle(graph map[string][]string, keys map[string]struct{}) []string {
	const (
		proposalUnseen = iota
		proposalVisiting
		proposalVisited
	)
	state := make(map[string]int, len(keys))
	stack := make([]string, 0, len(keys))
	var visit func(string) []string
	visit = func(node string) []string {
		switch state[node] {
		case proposalVisited:
			return nil
		case proposalVisiting:
			start := 0
			for index, value := range stack {
				if value == node {
					start = index
					break
				}
			}
			return append(append([]string(nil), stack[start:]...), node)
		}
		state[node] = proposalVisiting
		stack = append(stack, node)
		for _, dependency := range graph[node] {
			if cycle := visit(dependency); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = proposalVisited
		return nil
	}
	nodes := make([]string, 0, len(keys))
	for key := range keys {
		nodes = append(nodes, key)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		if cycle := visit(node); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func validateProposalOutputScopes(items []protocol.ExecutionPlanProposalItem) error {
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
			key, err := protocol.WorkOutputScopeComparisonKey(scope)
			if err != nil {
				return proposalItemError(item.LogicalKey, err)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("%w: duplicate output scope %q for %q", ErrInvariant, scope.Scope, item.LogicalKey)
			}
			seen[key] = struct{}{}
			for _, existing := range claims {
				conflict, conflictErr := protocol.WorkOutputClaimsConflict(
					item.LogicalKey,
					scope,
					existing.logicalKey,
					existing.scope,
					hardDependencies,
				)
				if conflictErr != nil {
					return proposalItemError(item.LogicalKey, conflictErr)
				}
				if conflict {
					return fmt.Errorf(
						"%w: output scopes conflict between %q and %q",
						ErrInvariant,
						item.LogicalKey,
						existing.logicalKey,
					)
				}
			}
			claims = append(claims, claim{logicalKey: item.LogicalKey, scope: scope})
		}
	}
	return nil
}

func proposalItemError(logicalKey string, err error) error {
	return fmt.Errorf("%w: Work Item %q: %v", ErrInvariant, logicalKey, err)
}

func validProposalWorkItemKind(kind protocol.WorkItemKind) bool {
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

func validProposalDependencyKind(kind protocol.WorkDependencyKind) bool {
	return kind == protocol.WorkDependencyHard || kind == protocol.WorkDependencySoft
}

func validProposalGoalActivationOrigin(origin protocol.GoalActivationOrigin) bool {
	switch origin {
	case protocol.GoalActivationOriginUserExplicit,
		protocol.GoalActivationOriginAdaptiveInitial,
		protocol.GoalActivationOriginAdaptivePromoted:
		return true
	default:
		return false
	}
}

func validProposalGoalActivationReason(reason protocol.GoalActivationReason) bool {
	switch reason {
	case protocol.GoalActivationReasonPersistenceRequested,
		protocol.GoalActivationReasonObservedBoundary,
		protocol.GoalActivationReasonRoomDependencyChain,
		protocol.GoalActivationReasonExternalWait,
		protocol.GoalActivationReasonScheduledRetry,
		protocol.GoalActivationReasonContextBoundary,
		protocol.GoalActivationReasonRecoveryRequired,
		protocol.GoalActivationReasonSubstantialComplexity:
		return true
	default:
		return false
	}
}
