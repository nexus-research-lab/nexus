// INPUT: immutable Plan graph、stable Work Item/spec/state fence、显式 active-work opt-in 与 expected versions。
// OUTPUT: 原子 Plan 写入、DAG/claim 校验、active responsibility 收束、Spec-aware lifecycle 切换与审计事件。
// POS: Plan revision 不可变性和 active Plan 唯一性的事务边界。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// WritePlan 原子创建或激活 immutable Plan revision；replacement 默认拒绝 current
// Assignment，只有带 reason 的显式 opt-in 才会同事务收束旧责任链。
func (r *Repository) WritePlan(ctx context.Context, command WritePlanCommand) (*protocol.ExecutionSnapshot, error) {
	plan := command.Plan
	plan.ID = strings.TrimSpace(plan.ID)
	plan.ExecutionID = strings.TrimSpace(plan.ExecutionID)
	if command.ExecutionID == "" {
		command.ExecutionID = plan.ExecutionID
	}
	if command.ExecutionID != plan.ExecutionID || plan.ID == "" || plan.Revision <= 0 {
		return nil, fmt.Errorf("%w: plan identity and revision are invalid", ErrInvariant)
	}
	if plan.Status != protocol.PlanRevisionStatusProposed && plan.Status != protocol.PlanRevisionStatusActive {
		return nil, fmt.Errorf("%w: WritePlan only accepts proposed or active status", ErrInvariant)
	}
	if command.SupersedeActiveWork && strings.TrimSpace(plan.RevisionReason) == "" {
		return nil, fmt.Errorf("%w: revision reason is required to supersede active work", ErrInvariant)
	}
	eventType := protocol.ExecutionEventPlanProposed
	if plan.Status == protocol.PlanRevisionStatusActive {
		eventType = protocol.ExecutionEventPlanActivated
	}
	mutation, err := r.beginMutation(
		ctx, plan.ExecutionID, command.ExpectedExecutionVersion, command.Meta, eventType,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	existing, err := r.getPlan(ctx, mutation.tx, plan.ID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if existing != nil {
		if err = r.activateProposedPlan(ctx, mutation.tx, command, *existing); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
			EntityType:    protocol.ExecutionEntityPlan,
			EntityID:      plan.ID,
			EntityVersion: command.ExpectedPlanVersion + 1,
			PlanID:        plan.ID,
		})
	}
	if command.ExpectedPlanVersion != 0 {
		r.abortMutation(mutation)
		return nil, ErrVersionConflict
	}
	normalized, dependencies, err := normalizeAndValidatePlan(plan, command.WorkItems, command.Dependencies, r.currentTime())
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	active, err := r.getActivePlan(ctx, mutation.tx, plan.ExecutionID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = validatePlanBase(ctx, r, mutation.tx, plan, active); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	now := r.currentTime()
	plan.Version = 1
	plan.CreatedAt = timeOr(plan.CreatedAt, now)
	if plan.Status == protocol.PlanRevisionStatusActive {
		activatedAt := now
		plan.ActivatedAt = &activatedAt
		if active != nil {
			if err = r.replaceActivePlan(
				ctx,
				mutation.tx,
				*active,
				plan,
				command.SupersedeActiveWork,
				command.Meta.CommandID,
				now,
			); err != nil {
				r.abortMutation(mutation)
				return nil, err
			}
		}
	}
	if err = r.insertPlan(ctx, mutation.tx, plan); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	for _, work := range normalized {
		if err = r.ensureWorkItem(ctx, mutation.tx, work.WorkItem); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		if err = r.ensureSpec(ctx, mutation.tx, work.Spec); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	for _, work := range parentFirst(normalized) {
		if err = r.insertPlanItem(ctx, mutation.tx, work.Item); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	for _, dependency := range dependencies {
		if err = r.insertDependency(ctx, mutation.tx, dependency); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	for _, work := range normalized {
		for _, claim := range work.OutputClaims {
			if err = r.insertClaim(ctx, mutation.tx, claim); err != nil {
				r.abortMutation(mutation)
				return nil, err
			}
		}
	}
	for _, work := range normalized {
		activate := plan.Status == protocol.PlanRevisionStatusActive
		if err = r.ensureState(ctx, mutation.tx, work, activate); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityPlan,
		EntityID:      plan.ID,
		EntityVersion: plan.Version,
		PlanID:        plan.ID,
	})
}

func (r *Repository) activateProposedPlan(
	ctx context.Context,
	tx *sql.Tx,
	command WritePlanCommand,
	existing protocol.ExecutionPlanRevision,
) error {
	if command.Plan.Status != protocol.PlanRevisionStatusActive ||
		existing.Status != protocol.PlanRevisionStatusProposed ||
		existing.ExecutionID != command.ExecutionID {
		return fmt.Errorf("%w: existing Plan can only transition proposed to active", ErrInvariant)
	}
	if err := validateExpectedVersion(command.ExpectedPlanVersion, "expected plan version"); err != nil {
		return err
	}
	normalized, dependencies, err := normalizeAndValidatePlan(
		command.Plan, command.WorkItems, command.Dependencies, r.currentTime(),
	)
	if err != nil {
		return err
	}
	if err = r.validatePersistedPlanGraph(ctx, tx, existing.ID, normalized, dependencies); err != nil {
		return err
	}
	active, err := r.getActivePlan(ctx, tx, existing.ExecutionID)
	if err != nil {
		return err
	}
	if err = validatePlanBase(ctx, r, tx, command.Plan, active); err != nil {
		return err
	}
	now := r.currentTime()
	if active != nil && active.ID != existing.ID {
		if err = r.replaceActivePlan(
			ctx,
			tx,
			*active,
			existing,
			command.SupersedeActiveWork,
			command.Meta.CommandID,
			now,
		); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `
UPDATE execution_plan_revisions
SET status = 'active',
    version = version + 1,
    activated_at = `+r.bind(1)+`
WHERE plan_id = `+r.bind(2)+`
  AND execution_id = `+r.bind(3)+`
  AND status = 'proposed'
  AND version = `+r.bind(4),
		r.timestamp(now), existing.ID, existing.ExecutionID, command.ExpectedPlanVersion,
	)
	if err != nil {
		return err
	}
	if err = requireOne(result); err != nil {
		return err
	}
	for _, work := range normalized {
		if err = r.ensureState(ctx, tx, work, true); err != nil {
			return err
		}
	}
	return nil
}

func validatePlanBase(
	ctx context.Context,
	r *Repository,
	tx *sql.Tx,
	plan protocol.ExecutionPlanRevision,
	active *protocol.ExecutionPlanRevision,
) error {
	if active != nil && active.ID != plan.ID {
		if plan.BasePlanID != active.ID {
			return fmt.Errorf("%w: plan base %q does not match active plan %q", ErrInvariant, plan.BasePlanID, active.ID)
		}
		return nil
	}
	if plan.BasePlanID == "" {
		return nil
	}
	base, err := r.getPlan(ctx, tx, plan.BasePlanID)
	if err != nil {
		return err
	}
	if base == nil || base.ExecutionID != plan.ExecutionID {
		return fmt.Errorf("%w: base plan is outside the execution", ErrInvariant)
	}
	return nil
}

func (r *Repository) supersedePlan(
	ctx context.Context,
	tx *sql.Tx,
	plan protocol.ExecutionPlanRevision,
	now time.Time,
) error {
	result, err := tx.ExecContext(ctx, `
UPDATE execution_plan_revisions
SET status = 'superseded',
    version = version + 1,
    superseded_at = `+r.bind(1)+`
WHERE plan_id = `+r.bind(2)+`
  AND status = 'active'
  AND version = `+r.bind(3),
		r.timestamp(now), plan.ID, plan.Version,
	)
	if err != nil {
		return err
	}
	return requireOne(result)
}

func (r *Repository) replaceActivePlan(
	ctx context.Context,
	tx *sql.Tx,
	active protocol.ExecutionPlanRevision,
	replacement protocol.ExecutionPlanRevision,
	supersedeActiveWork bool,
	commandID string,
	now time.Time,
) error {
	var unreviewed int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_submissions submission
LEFT JOIN execution_acceptances acceptance
  ON acceptance.submission_id = submission.submission_id
WHERE submission.plan_id = `+r.bind(1)+`
  AND acceptance.acceptance_id IS NULL`,
		active.ID,
	).Scan(&unreviewed); err != nil {
		return err
	}
	if unreviewed != 0 {
		return fmt.Errorf("%w: active Plan has unreviewed Submission", ErrInvariant)
	}
	var currentAssignments int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_work_assignments
WHERE plan_id = `+r.bind(1)+`
  AND status IN ('assigned', 'active')`,
		active.ID,
	).Scan(&currentAssignments); err != nil {
		return err
	}
	if currentAssignments != 0 && !supersedeActiveWork {
		return fmt.Errorf("%w: active Plan still has current Assignment", ErrInvariant)
	}
	if currentAssignments != 0 {
		if strings.TrimSpace(replacement.RevisionReason) == "" {
			return fmt.Errorf("%w: revision reason is required to supersede active work", ErrInvariant)
		}
		if err := r.releasePlanWork(
			ctx,
			tx,
			active.ExecutionID,
			active.ID,
			commandID,
			strings.TrimSpace(replacement.RevisionReason),
			now,
		); err != nil {
			return err
		}
	}
	return r.supersedePlan(ctx, tx, active, now)
}

func (r *Repository) releasePlanWork(
	ctx context.Context,
	tx *sql.Tx,
	executionID string,
	planID string,
	commandID string,
	revisionReason string,
	now time.Time,
) error {
	reason := "plan superseded: " + revisionReason
	if err := r.enqueueAttemptCancellations(
		ctx,
		tx,
		cancellationAttemptScope{
			ExecutionID: executionID,
			PlanID:      planID,
		},
		commandID,
		reason,
		now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_attempts
SET status = 'interrupted',
    failure_reason = `+r.bind(1)+`,
    version = version + 1,
    finished_at = `+r.bind(2)+`
WHERE plan_id = `+r.bind(3)+`
  AND status IN ('pending', 'running')`,
		reason, r.timestamp(now), planID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'cancelled',
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`,
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE plan_id = `+r.bind(3)+`
  AND status IN ('pending', 'claimed', 'failed')`,
		reason, r.timestamp(now), planID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'released',
    version = version + 1,
    released_at = `+r.bind(1)+`
WHERE plan_id = `+r.bind(2)+`
  AND status IN ('assigned', 'active')`,
		r.timestamp(now), planID,
	)
	return err
}

func normalizeAndValidatePlan(
	plan protocol.ExecutionPlanRevision,
	source []PlanWorkItem,
	dependencies []protocol.ExecutionPlanDependency,
	now time.Time,
) ([]PlanWorkItem, []protocol.ExecutionPlanDependency, error) {
	if err := protocol.ValidateExecutionProjectionLimit("items", len(source)); err != nil {
		return nil, nil, err
	}
	if len(source) == 0 {
		return nil, nil, fmt.Errorf("%w: Plan must contain at least one Work Item", ErrInvariant)
	}
	items := make([]PlanWorkItem, len(source))
	byID := make(map[string]int, len(source))
	for index := range source {
		work := source[index]
		for _, collection := range []struct {
			field string
			count int
		}{
			{field: "acceptance_criteria", count: len(work.Spec.AcceptanceCriteria)},
			{field: "input_refs", count: len(work.Spec.InputRefs)},
			{field: "output_scopes", count: len(work.OutputClaims)},
		} {
			if err := protocol.ValidateExecutionProjectionLimit(
				collection.field,
				collection.count,
			); err != nil {
				return nil, nil, err
			}
		}
		work.OutputClaims = slices.Clone(work.OutputClaims)
		work.WorkItem.ID = strings.TrimSpace(work.WorkItem.ID)
		work.WorkItem.ExecutionID = strings.TrimSpace(work.WorkItem.ExecutionID)
		work.Spec.ID = strings.TrimSpace(work.Spec.ID)
		work.Spec.WorkItemID = strings.TrimSpace(work.Spec.WorkItemID)
		work.Spec.ExecutionID = strings.TrimSpace(work.Spec.ExecutionID)
		if work.WorkItem.ID == "" || work.WorkItem.ExecutionID != plan.ExecutionID ||
			work.Spec.ID == "" || work.Spec.WorkItemID != work.WorkItem.ID ||
			work.Spec.ExecutionID != plan.ExecutionID || work.Spec.Version <= 0 ||
			strings.TrimSpace(work.Spec.SpecHash) == "" {
			return nil, nil, fmt.Errorf("%w: Work Item/spec chain at index %d is incomplete", ErrInvariant, index)
		}
		if _, exists := byID[work.WorkItem.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate Work Item %q", ErrInvariant, work.WorkItem.ID)
		}
		byID[work.WorkItem.ID] = index
		work.WorkItem.CreatedAt = timeOr(work.WorkItem.CreatedAt, now)
		work.Spec.CreatedAt = timeOr(work.Spec.CreatedAt, now)
		work.State.WorkItemID = work.WorkItem.ID
		work.State.ExecutionID = plan.ExecutionID
		work.State.CurrentSpecID = work.Spec.ID
		if work.State.Status == "" {
			work.State.Status = protocol.WorkItemStatusOpen
		}
		work.State.Version = versionOrOne(work.State.Version)
		work.State.UpdatedAt = timeOr(work.State.UpdatedAt, now)
		work.Item.PlanID = plan.ID
		work.Item.ExecutionID = plan.ExecutionID
		work.Item.WorkItemID = work.WorkItem.ID
		work.Item.SpecID = work.Spec.ID
		work.Item.CreatedAt = timeOr(work.Item.CreatedAt, now)
		for claimIndex := range work.OutputClaims {
			claim := &work.OutputClaims[claimIndex]
			claim.PlanID = plan.ID
			claim.ExecutionID = plan.ExecutionID
			claim.WorkItemID = work.WorkItem.ID
			claim.SpecID = work.Spec.ID
			claim.CreatedAt = timeOr(claim.CreatedAt, now)
			normalized, err := protocol.NormalizeWorkOutputScope(protocol.WorkOutputScope{
				Scope: claim.Scope,
				Mode:  claim.Mode,
			})
			if err != nil {
				return nil, nil, fmt.Errorf(
					"%w: invalid output claim for %q: %v",
					ErrInvariant,
					work.WorkItem.ID,
					err,
				)
			}
			claim.Scope = normalized.Scope
			claim.Mode = normalized.Mode
		}
		items[index] = work
	}
	parentEdges := make(map[string]string, len(items))
	for _, work := range items {
		parent := strings.TrimSpace(work.Item.ParentWorkItemID)
		if parent == "" {
			continue
		}
		if parent == work.WorkItem.ID {
			return nil, nil, fmt.Errorf("%w: Work Item cannot parent itself", ErrInvariant)
		}
		if _, exists := byID[parent]; !exists {
			return nil, nil, fmt.Errorf("%w: parent %q is outside Plan", ErrInvariant, parent)
		}
		parentEdges[work.WorkItem.ID] = parent
	}
	if err := validateSingleParentCycles(parentEdges); err != nil {
		return nil, nil, err
	}
	directDependencyCounts := make(map[string]int, len(items))
	for _, dependency := range dependencies {
		workItemID := strings.TrimSpace(dependency.WorkItemID)
		directDependencyCounts[workItemID]++
		if err := protocol.ValidateExecutionProjectionLimit(
			"depends_on",
			directDependencyCounts[workItemID],
		); err != nil {
			return nil, nil, err
		}
	}
	normalizedDependencies := make([]protocol.ExecutionPlanDependency, len(dependencies))
	edges := make(map[string][]string, len(items))
	seenEdge := make(map[string]struct{}, len(dependencies))
	for index, dependency := range dependencies {
		dependency.PlanID = plan.ID
		dependency.ExecutionID = plan.ExecutionID
		dependency.WorkItemID = strings.TrimSpace(dependency.WorkItemID)
		dependency.DependsOnWorkItemID = strings.TrimSpace(dependency.DependsOnWorkItemID)
		if _, exists := byID[dependency.WorkItemID]; !exists {
			return nil, nil, fmt.Errorf("%w: dependency target %q is outside Plan", ErrInvariant, dependency.WorkItemID)
		}
		if _, exists := byID[dependency.DependsOnWorkItemID]; !exists ||
			dependency.WorkItemID == dependency.DependsOnWorkItemID {
			return nil, nil, fmt.Errorf("%w: dependency source %q is invalid", ErrInvariant, dependency.DependsOnWorkItemID)
		}
		if dependency.Kind == "" {
			dependency.Kind = protocol.WorkDependencyHard
		}
		key := dependency.WorkItemID + "\x00" + dependency.DependsOnWorkItemID
		if _, exists := seenEdge[key]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate dependency %q", ErrInvariant, key)
		}
		seenEdge[key] = struct{}{}
		dependency.CreatedAt = timeOr(dependency.CreatedAt, now)
		normalizedDependencies[index] = dependency
		edges[dependency.WorkItemID] = append(edges[dependency.WorkItemID], dependency.DependsOnWorkItemID)
	}
	if err := validateDependencyCycles(byID, edges); err != nil {
		return nil, nil, err
	}
	if err := validateOutputClaims(items, normalizedDependencies); err != nil {
		return nil, nil, err
	}
	return items, normalizedDependencies, nil
}

func validateSingleParentCycles(parent map[string]string) error {
	for start := range parent {
		seen := map[string]struct{}{}
		for current := start; current != ""; current = parent[current] {
			if _, exists := seen[current]; exists {
				return fmt.Errorf("%w: parent cycle reaches %q", ErrInvariant, current)
			}
			seen[current] = struct{}{}
		}
	}
	return nil
}

func validateDependencyCycles(nodes map[string]int, edges map[string][]string) error {
	state := make(map[string]uint8, len(nodes))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("%w: dependency cycle reaches %q", ErrInvariant, id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, dependency := range edges[id] {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range nodes {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func validateOutputClaims(
	items []PlanWorkItem,
	dependencies []protocol.ExecutionPlanDependency,
) error {
	type claimOwner struct {
		workItemID string
		scope      protocol.WorkOutputScope
	}
	hardDependencies := make(map[string][]string, len(items))
	for _, dependency := range dependencies {
		if dependency.Kind == protocol.WorkDependencyHard {
			hardDependencies[dependency.WorkItemID] = append(
				hardDependencies[dependency.WorkItemID],
				dependency.DependsOnWorkItemID,
			)
		}
	}
	claims := make([]claimOwner, 0)
	seen := make(map[string]struct{})
	for _, work := range items {
		for _, claim := range work.OutputClaims {
			comparisonKey, err := protocol.WorkOutputScopeComparisonKey(protocol.WorkOutputScope{
				Scope: claim.Scope,
				Mode:  claim.Mode,
			})
			if err != nil {
				return fmt.Errorf("%w: invalid output claim %q: %v", ErrInvariant, claim.Scope, err)
			}
			key := work.WorkItem.ID + "\x00" + comparisonKey
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: duplicate output claim %q", ErrInvariant, claim.Scope)
			}
			seen[key] = struct{}{}
			scope := protocol.WorkOutputScope{Scope: claim.Scope, Mode: claim.Mode}
			for _, existing := range claims {
				if existing.workItemID == work.WorkItem.ID {
					continue
				}
				conflict, err := protocol.WorkOutputClaimsConflict(
					work.WorkItem.ID,
					scope,
					existing.workItemID,
					existing.scope,
					hardDependencies,
				)
				if err != nil {
					return fmt.Errorf("%w: invalid output claim %q: %v", ErrInvariant, claim.Scope, err)
				}
				if conflict {
					return fmt.Errorf(
						"%w: output %q conflicts between Work Items %q and %q",
						ErrInvariant,
						claim.Scope,
						existing.workItemID,
						work.WorkItem.ID,
					)
				}
			}
			claims = append(claims, claimOwner{workItemID: work.WorkItem.ID, scope: scope})
		}
	}
	return nil
}

func parentFirst(items []PlanWorkItem) []PlanWorkItem {
	byID := make(map[string]PlanWorkItem, len(items))
	for _, item := range items {
		byID[item.WorkItem.ID] = item
	}
	result := make([]PlanWorkItem, 0, len(items))
	added := make(map[string]bool, len(items))
	var appendItem func(string)
	appendItem = func(id string) {
		if added[id] {
			return
		}
		item := byID[id]
		if parent := item.Item.ParentWorkItemID; parent != "" {
			appendItem(parent)
		}
		added[id] = true
		result = append(result, item)
	}
	sortedIDs := make([]string, 0, len(items))
	for id := range byID {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)
	for _, id := range sortedIDs {
		appendItem(id)
	}
	return result
}

func (r *Repository) insertPlan(ctx context.Context, tx *sql.Tx, plan protocol.ExecutionPlanRevision) error {
	metadataJSON, err := marshalMap(plan.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_plan_revisions (
    plan_id, execution_id, revision, status, base_plan_id,
    created_by_agent_id, revision_reason, version, created_at,
    activated_at, superseded_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.jsonBind(12)+`)`,
		plan.ID, plan.ExecutionID, plan.Revision, plan.Status, nullString(plan.BasePlanID),
		nullString(plan.CreatedByAgentID), nullString(plan.RevisionReason), plan.Version,
		r.timestamp(plan.CreatedAt), nullTime(plan.ActivatedAt), nullTime(plan.SupersededAt), metadataJSON,
	)
	return err
}

func (r *Repository) ensureWorkItem(ctx context.Context, tx *sql.Tx, item protocol.WorkItem) error {
	existing, err := scanWorkItem(tx.QueryRowContext(ctx, r.workItemSelect("")+`
FROM execution_work_items WHERE work_item_id = `+r.bind(1), item.ID))
	if err == nil {
		wantMetadata, marshalErr := marshalMap(item.Metadata)
		if marshalErr != nil {
			return marshalErr
		}
		gotMetadata, marshalErr := marshalMap(existing.Metadata)
		if marshalErr != nil {
			return marshalErr
		}
		if existing.ExecutionID != item.ExecutionID || existing.LogicalKey != item.LogicalKey ||
			existing.Kind != item.Kind || gotMetadata != wantMetadata {
			return fmt.Errorf("%w: immutable Work Item %q differs", ErrInvariant, item.ID)
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_work_items (
    work_item_id, execution_id, logical_key, kind, created_at, metadata_json
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.jsonBind(6)+`)`,
		item.ID, item.ExecutionID, item.LogicalKey, item.Kind, r.timestamp(item.CreatedAt), metadataJSON,
	)
	return err
}

func (r *Repository) ensureSpec(ctx context.Context, tx *sql.Tx, spec protocol.WorkItemSpec) error {
	existing, err := scanSpec(tx.QueryRowContext(ctx, r.specSelect("")+`
FROM execution_work_item_specs WHERE spec_id = `+r.bind(1), spec.ID))
	if err == nil {
		if existing.ExecutionID != spec.ExecutionID || existing.WorkItemID != spec.WorkItemID ||
			existing.Version != spec.Version || existing.SpecHash != spec.SpecHash ||
			existing.Subject != spec.Subject || existing.Objective != spec.Objective ||
			existing.Deliverable != spec.Deliverable {
			return fmt.Errorf("%w: immutable Work Item spec %q differs", ErrInvariant, spec.ID)
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	criteriaJSON, err := marshalSlice(spec.AcceptanceCriteria)
	if err != nil {
		return err
	}
	inputRefsJSON, err := marshalSlice(spec.InputRefs)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalMap(spec.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_work_item_specs (
    spec_id, work_item_id, execution_id, spec_version, subject,
    objective, deliverable, acceptance_criteria_json, input_refs_json,
    spec_hash, created_by_agent_id, created_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.jsonBind(8)+`,`+r.jsonBind(9)+`,`+
		r.bind(10)+`,`+r.bind(11)+`,`+r.bind(12)+`,`+r.jsonBind(13)+`)`,
		spec.ID, spec.WorkItemID, spec.ExecutionID, spec.Version, spec.Subject,
		spec.Objective, spec.Deliverable, criteriaJSON, inputRefsJSON, spec.SpecHash,
		nullString(spec.CreatedByAgentID), r.timestamp(spec.CreatedAt), metadataJSON,
	)
	return err
}

func (r *Repository) insertPlanItem(ctx context.Context, tx *sql.Tx, item protocol.ExecutionPlanItem) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO execution_plan_items (
    plan_id, execution_id, work_item_id, spec_id, parent_work_item_id,
    is_required, is_terminal, position, created_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`)`,
		item.PlanID, item.ExecutionID, item.WorkItemID, item.SpecID,
		nullString(item.ParentWorkItemID), item.Required, item.Terminal,
		item.Position, r.timestamp(item.CreatedAt),
	)
	return err
}

func (r *Repository) insertDependency(ctx context.Context, tx *sql.Tx, item protocol.ExecutionPlanDependency) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO execution_plan_dependencies (
    plan_id, execution_id, work_item_id, depends_on_work_item_id,
    dependency_kind, created_at
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`)`,
		item.PlanID, item.ExecutionID, item.WorkItemID, item.DependsOnWorkItemID,
		item.Kind, r.timestamp(item.CreatedAt),
	)
	return err
}

func (r *Repository) insertClaim(ctx context.Context, tx *sql.Tx, item protocol.ExecutionPlanOutputClaim) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO execution_plan_output_claims (
    plan_id, execution_id, work_item_id, spec_id, claim_key, mode, created_at
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`)`,
		item.PlanID, item.ExecutionID, item.WorkItemID, item.SpecID,
		item.Scope, item.Mode, r.timestamp(item.CreatedAt),
	)
	return err
}

func (r *Repository) ensureState(ctx context.Context, tx *sql.Tx, work PlanWorkItem, activate bool) error {
	existing, err := r.getState(ctx, tx, work.WorkItem.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		metadataJSON, marshalErr := marshalMap(work.State.Metadata)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO execution_work_item_states (
    work_item_id, execution_id, current_spec_id, status,
    block_reason, needed_input, version, updated_at, metadata_json
) VALUES (`+
			r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
			r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.jsonBind(9)+`)`,
			work.State.WorkItemID, work.State.ExecutionID, work.State.CurrentSpecID,
			work.State.Status, nullString(work.State.BlockReason), nullString(work.State.NeededInput),
			work.State.Version, r.timestamp(work.State.UpdatedAt), metadataJSON,
		)
		return err
	}
	if !activate || existing.CurrentSpecID == work.Spec.ID {
		return nil
	}
	if err = validateExpectedVersion(work.ExpectedStateVersion, "expected Work Item state version"); err != nil {
		return err
	}
	blockReason := strings.TrimSpace(work.State.BlockReason)
	neededInput := strings.TrimSpace(work.State.NeededInput)
	if work.State.Status != protocol.WorkItemStatusWaitingInput {
		blockReason = ""
		neededInput = ""
	}
	metadataJSON, err := marshalMap(work.State.Metadata)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE execution_work_item_states
SET current_spec_id = `+r.bind(1)+`,
    status = `+r.bind(2)+`,
    block_reason = `+r.bind(3)+`,
    needed_input = `+r.bind(4)+`,
    metadata_json = `+r.jsonBind(5)+`,
    version = version + 1,
    updated_at = `+r.bind(6)+`
WHERE work_item_id = `+r.bind(7)+`
  AND execution_id = `+r.bind(8)+`
  AND version = `+r.bind(9),
		work.Spec.ID, work.State.Status, nullString(blockReason), nullString(neededInput),
		metadataJSON, r.timestamp(r.currentTime()), work.WorkItem.ID,
		work.WorkItem.ExecutionID, work.ExpectedStateVersion,
	)
	if err != nil {
		return err
	}
	return requireOne(result)
}

func (r *Repository) validatePersistedPlanGraph(
	ctx context.Context,
	tx *sql.Tx,
	planID string,
	work []PlanWorkItem,
	dependencies []protocol.ExecutionPlanDependency,
) error {
	items, err := r.listPlanItems(ctx, tx, planID)
	if err != nil {
		return err
	}
	persistedDependencies, err := r.listDependencies(ctx, tx, planID)
	if err != nil {
		return err
	}
	if len(items) != len(work) || len(persistedDependencies) != len(dependencies) {
		return fmt.Errorf("%w: proposed Plan graph differs during activation", ErrInvariant)
	}
	itemByWorkID := make(map[string]protocol.ExecutionPlanItem, len(items))
	for _, item := range items {
		itemByWorkID[item.WorkItemID] = item
	}
	for _, candidate := range work {
		item, exists := itemByWorkID[candidate.WorkItem.ID]
		if !exists || item.SpecID != candidate.Spec.ID ||
			item.ParentWorkItemID != candidate.Item.ParentWorkItemID ||
			item.Required != candidate.Item.Required || item.Terminal != candidate.Item.Terminal ||
			item.Position != candidate.Item.Position {
			return fmt.Errorf("%w: proposed Plan membership differs during activation", ErrInvariant)
		}
	}
	persisted := make(map[string]protocol.WorkDependencyKind, len(persistedDependencies))
	for _, dependency := range persistedDependencies {
		persisted[dependency.WorkItemID+"\x00"+dependency.DependsOnWorkItemID] = dependency.Kind
	}
	for _, dependency := range dependencies {
		if persisted[dependency.WorkItemID+"\x00"+dependency.DependsOnWorkItemID] != dependency.Kind {
			return fmt.Errorf("%w: proposed Plan dependencies differ during activation", ErrInvariant)
		}
	}
	return nil
}
