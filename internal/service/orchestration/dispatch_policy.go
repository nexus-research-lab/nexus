// INPUT: Explicit coordinator routing, current Plan and durable policy candidates.
// OUTPUT: Once-only ready assignments and recoverable DM/Room queue handoffs.
// POS: Declarative scheduling reuses existing assignment authorization; no inferred owners or automatic takeover.
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type DispatchPolicyInput struct {
	WorkItemID      string                         `json:"work_item_id,omitempty"`
	LogicalKey      string                         `json:"logical_key,omitempty"`
	TargetAgentID   string                         `json:"target_agent_id"`
	ReturnToAgentID string                         `json:"return_to_agent_id"`
	Instruction     string                         `json:"instruction,omitempty"`
	DispatchKind    protocol.ExecutionDispatchKind `json:"dispatch_kind,omitempty"`
}
type ConfigureDispatchInput struct {
	ExecutionID, CommandID string
	SnapshotRevision       int64
	Policies               []DispatchPolicyInput
}

type dispatchPolicyRepository interface {
	ConfigureDispatch(context.Context, store.ConfigureDispatchCommand) (*protocol.ExecutionSnapshot, error)
	FindAssignmentBatchReceipt(context.Context, string, string) (*protocol.ExecutionEvent, error)
	ListDueDispatchPolicies(context.Context, time.Time, int) ([]protocol.ExecutionDispatchPolicy, error)
	ActivateDispatchPolicy(context.Context, store.ActivateDispatchPolicyCommand) (*protocol.ExecutionSnapshot, error)
	DeferDispatchPolicy(context.Context, string, string, string, string, time.Time) error
}

// ScheduledWorkConsumer owns queue pause and Goal admission. It cannot grant a new
// policy: the persisted policy and exact Assignment remain the scheduling authority.
type ScheduledWorkConsumer interface {
	AutomaticDispatchAllowed(context.Context, protocol.Execution, string) (bool, error)
	QueueScheduledWork(context.Context, protocol.Execution, protocol.ExecutionWorkBinding, string, string) error
}

func (s *Service) SetScheduledWorkConsumer(c ScheduledWorkConsumer) { s.scheduledWork = c }

func (s *Service) ConfigureDispatch(ctx context.Context, actor ActorContext, input ConfigureDispatchInput) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(ctx, actor, input.ExecutionID, input.SnapshotRevision, true, false)
	if err != nil {
		return MutationResult{}, err
	}
	if rejected != nil && (snapshot == nil || rejected.ReasonCode != ErrorCodeStaleExecution) {
		return *rejected, nil
	}
	if rejected != nil {
		if err := s.requireRuntimeCoordination(actor, snapshot); err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
		if err := requireCoordinator(actor, snapshot); err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
	}
	if actor.WorkBinding != nil || actor.ReviewBinding != nil || actor.ObservationOnly || snapshot.Plan == nil || input.CommandID == "" || len(input.Policies) > protocol.ExecutionProjectionCollectionLimit {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "dispatch configuration requires an unbound coordinator, current Plan, request id and at most 32 explicit policies"), nil), nil
	}
	repository, ok := s.repository.(dispatchPolicyRepository)
	if !ok {
		return MutationResult{}, errors.New("dispatch policy repository unavailable")
	}
	payload, _ := json.Marshal(input.Policies)
	digest := sha256.Sum256(payload)
	id := sha256.Sum256([]byte(input.CommandID))
	meta := s.commandMeta(actor, hex.EncodeToString(id[:]), "configure_dispatch")
	receipt, err := repository.FindAssignmentBatchReceipt(ctx, snapshot.Execution.ID, meta.CommandID)
	if err != nil {
		return MutationResult{}, err
	}
	if receipt != nil {
		if receipt.Type != protocol.ExecutionEventDispatchConfigured || receipt.ActorID != actor.AgentID || receipt.Payload["request_digest"] != hex.EncodeToString(digest[:]) {
			return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "request_id reused for different dispatch policy"), nil), nil
		}
		return NoOpResult(snapshot, "dispatch configuration was already committed"), nil
	}
	if rejected != nil {
		return *rejected, nil
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive && snapshot.Execution.Status != protocol.ExecutionStatusWaiting {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "paused Execution cannot configure dispatch"), nil), nil
	}
	command := store.ConfigureDispatchCommand{ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID, ExpectedExecutionVersion: input.SnapshotRevision, Meta: meta, RequestDigest: hex.EncodeToString(digest[:])}
	seen := map[string]bool{}
	for _, intent := range input.Policies {
		work, spec, err := resolvePlanWork(snapshot, intent.WorkItemID, intent.LogicalKey)
		if err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
		if seen[work.ID] || intent.TargetAgentID == "" || intent.ReturnToAgentID == "" || activeAssignmentForWork(snapshot, work.ID) != nil {
			return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "each unassigned Work Item needs one explicit owner and reviewer"), nil), nil
		}
		seen[work.ID] = true
		if snapshot.Execution.ScopeKind == protocol.ExecutionScopeDM && intent.ReturnToAgentID != actor.AgentID {
			return RejectedResult(snapshot, domainError(ErrorCodeWrongReviewer, "DM review must return to its coordinator"), nil), nil
		}
		_, _, _, err = s.authorizedPolicyChain(ctx, actor, snapshot, work, spec, intent, "validate-policy")
		if err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
		command.Policies = append(command.Policies, protocol.ExecutionDispatchPolicy{ID: s.id("policy"), ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID, WorkItemID: work.ID, SpecID: spec.ID, TargetAgentID: intent.TargetAgentID, ReturnToAgentID: intent.ReturnToAgentID, Instruction: intent.Instruction, DispatchKind: intent.DispatchKind})
	}
	updated, err := repository.ConfigureDispatch(ctx, command)
	if err != nil {
		return s.storageMutationResult(snapshot, err, nil)
	}
	return AppliedResult(updated, []string{"dispatch_policy:" + snapshot.Plan.ID}, nextActions(updated, actor)), nil
}

func (s *Service) authorizedPolicyChain(ctx context.Context, actor ActorContext, snapshot *protocol.ExecutionSnapshot, work protocol.WorkItem, spec protocol.WorkItemSpec, intent DispatchPolicyInput, key string) (protocol.WorkAssignment, *protocol.ExecutionDispatch, protocol.WorkAttempt, error) {
	kind := intent.DispatchKind
	if intent.TargetAgentID == actor.AgentID {
		kind = ""
	}
	a, d, attempt, err := s.buildAssignmentChain(actor, snapshot, work, spec, intent.TargetAgentID, intent.ReturnToAgentID, "", "explicit dispatch policy", "", intent.Instruction, kind, key)
	if err == nil {
		err = s.authorizeAssignmentTarget(ctx, actor, snapshot, a, d)
	}
	if err != nil {
		return a, d, attempt, err
	}
	if intent.DispatchKind != "" && intent.DispatchKind != protocol.ExecutionDispatchRoomDirected && intent.DispatchKind != protocol.ExecutionDispatchRoomPublic {
		return a, d, attempt, domainError(ErrorCodeInvalidInput, "unknown dispatch route")
	}
	if snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom && d == nil {
		kind = intent.DispatchKind
		if kind == "" {
			kind = protocol.ExecutionDispatchRoomDirected
		}
		d = &protocol.ExecutionDispatch{ID: s.id("dispatch"), Kind: kind, TargetAgentID: a.OwnerAgentID, DedupeKey: key, Instruction: intent.Instruction}
		if d.Instruction == "" {
			d.Instruction = fmt.Sprintf("Deliver %s. Acceptance criteria: %v", spec.Deliverable, spec.AcceptanceCriteria)
		}
	}
	a.Metadata = map[string]any{"dispatch_policy": true}
	return a, d, attempt, nil
}

func (s *Service) ReconcileDispatchPolicies(ctx context.Context, limit int) (int, error) {
	repository, ok := s.repository.(dispatchPolicyRepository)
	if !ok {
		return 0, nil
	}
	if limit <= 0 || limit > 256 {
		limit = 32
	}
	policies, err := repository.ListDueDispatchPolicies(ctx, s.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	var failures []error
	for _, policy := range policies {
		if err := s.reconcileDispatchPolicy(ctx, repository, policy); err != nil {
			failures = append(failures, err)
		}
	}
	return len(policies), errors.Join(failures...)
}
func (s *Service) reconcileDispatchPolicy(ctx context.Context, repository dispatchPolicyRepository, p protocol.ExecutionDispatchPolicy) error {
	deferPolicy := func(state, reason string) error {
		return repository.DeferDispatchPolicy(ctx, p.ID, p.State, state, reason, s.now().UTC().Add(15*time.Second))
	}
	snapshot, err := s.repository.GetSnapshot(ctx, p.ExecutionID)
	if err != nil {
		return errors.Join(err, deferPolicy(p.State, "snapshot_unavailable"))
	}
	if snapshot == nil || !snapshot.Execution.Status.Current() || snapshot.Plan == nil || snapshot.Plan.ID != p.PlanID {
		return deferPolicy("cancelled", "plan_or_execution_replaced")
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusPaused || rootRecoveryStoppedByUser(snapshot) {
		return deferPolicy(p.State, "paused")
	}
	if s.scheduledWork == nil {
		return deferPolicy(p.State, "admission_unavailable")
	}
	allowed, err := s.scheduledWork.AutomaticDispatchAllowed(ctx, snapshot.Execution, p.TargetAgentID)
	if err != nil {
		return errors.Join(err, deferPolicy(p.State, "admission_unavailable"))
	}
	if !allowed {
		return deferPolicy(p.State, "paused_or_goal_unavailable")
	}
	if p.State == "assigned" {
		assignment := findAssignmentByID(snapshot, p.AssignmentID)
		attempt := findAttemptByID(snapshot, p.AttemptID)
		if assignment != nil && attempt != nil && attempt.AgentRoundID != "" {
			return deferPolicy("queued", "runtime_admission_proven")
		}
		if assignment == nil || !currentAssignment(*assignment) || attempt == nil || attempt.Status != protocol.WorkAttemptStatusPending {
			return deferPolicy("cancelled", "responsibility_changed")
		}
		binding := protocol.ExecutionWorkBinding{ExecutionID: p.ExecutionID, PlanID: p.PlanID, WorkItemID: p.WorkItemID, SpecID: p.SpecID, AssignmentID: p.AssignmentID, AttemptID: p.AttemptID}
		if err := s.scheduledWork.QueueScheduledWork(ctx, snapshot.Execution, binding, p.Instruction, "policy:"+p.ID); err != nil {
			return errors.Join(err, deferPolicy(p.State, "queue_receipt_unknown"))
		}
		return deferPolicy("queued", "")
	}
	if activeAssignmentForWork(snapshot, p.WorkItemID) != nil {
		return deferPolicy("cancelled", "explicit_assignment_exists")
	}
	if !slices.Contains(snapshot.ReadyWorkItemIDs, p.WorkItemID) {
		return deferPolicy("pending", "waiting_for_acceptance")
	}
	actor := ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, ExecutionID: p.ExecutionID, AgentID: snapshot.Execution.CoordinatorAgentID, Role: ExecutionActorCoordinator, ActorKind: protocol.ExecutionActorSystem, ScopeKind: snapshot.Execution.ScopeKind, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID}
	work, spec, err := resolvePlanWork(snapshot, p.WorkItemID, "")
	if err != nil || spec.ID != p.SpecID {
		return deferPolicy("cancelled", "spec_changed")
	}
	a, d, attempt, err := s.authorizedPolicyChain(ctx, actor, snapshot, work, spec, DispatchPolicyInput{TargetAgentID: p.TargetAgentID, ReturnToAgentID: p.ReturnToAgentID, Instruction: p.Instruction, DispatchKind: p.DispatchKind}, "policy:"+p.ID)
	if err != nil {
		return deferPolicy("cancelled", "target_no_longer_authorized")
	}
	updated, err := repository.ActivateDispatchPolicy(ctx, store.ActivateDispatchPolicyCommand{PolicyID: p.ID, Assignment: store.AssignCommand{Assignment: a, Dispatch: d, RootAttempt: &attempt, ExpectedExecutionVersion: snapshot.Execution.Version, Meta: s.commandMeta(actor, "policy:"+p.ID, "assign")}})
	if err != nil {
		return errors.Join(err, deferPolicy(p.State, "reconcile_assignment"))
	}
	s.WakeExecutionDispatch()
	s.invalidateSnapshot(ctx, updated)
	for _, current := range updated.DispatchPolicies {
		if current.ID == p.ID && current.State == "assigned" {
			return s.reconcileDispatchPolicy(ctx, repository, current)
		}
	}
	return nil
}
